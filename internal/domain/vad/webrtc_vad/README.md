# Hiện thực Resource Pool cho WebRTC VAD

Đây là hiện thực quản lý resource pool (bể tài nguyên) cho WebRTC VAD (Voice Activity Detection - Phát hiện hoạt động giọng nói), giúp cải thiện hiệu năng và mức độ sử dụng tài nguyên trong các tình huống xử lý đồng thời (concurrency).

## Các thành phần chính

### 1. WebRTCVAD

Hiện thực VAD cơ bản, hiện đã hiện thực interface `Resource`:

- `IsValid()`: kiểm tra resource có hợp lệ hay không
- `Close()`: đóng và giải phóng resource
- Các thao tác an toàn với đa luồng (thread-safe)

### 2. WebRTCVADFactory

Factory (nhà máy tạo instance) resource, hiện thực interface `ResourceFactory`:

- `Create()`: tạo thực thể VAD mới
- `Validate()`: kiểm tra tính hợp lệ của resource
- `Reset()`: đặt lại trạng thái resource

### 3. WebRTCVADPool

Bộ quản lý resource pool cho VAD:

- `AcquireVAD()`: lấy một thực thể VAD
- `ReleaseVAD()`: giải phóng thực thể VAD
- `Stats()`: lấy thông tin thống kê
- `Close()`: đóng resource pool

### 4. VADManager

Lớp bọc (wrapper) cấp cao, cung cấp interface sử dụng tiện lợi:

- `ProcessAudio()`: xử lý một đoạn dữ liệu audio
- `ProcessAudioBatch()`: xử lý hàng loạt dữ liệu audio
- `WithVAD()`: sử dụng callback để xử lý VAD

## Cách sử dụng

### Sử dụng cơ bản

```go
// Tạo cấu hình VAD
config := WebRTCVADConfig{
    SampleRate: 16000,
    Mode:       2, // Độ nhạy trung bình
}

// Tạo VAD manager
manager, err := NewVADManager(config)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// Xử lý dữ liệu audio
audioData := make([]float32, 320) // 16kHz, 20ms
isActive, err := manager.ProcessAudio(audioData)
if err != nil {
    log.Printf("VAD processing failed: %v", err)
    return
}

if isActive {
    fmt.Println("Voice activity detected!")
}
```

### Sử dụng nâng cao - Dùng trực tiếp resource pool

```go
// Tạo resource pool
vadConfig := WebRTCVADConfig{
    SampleRate: 16000,
    Mode:       2,
}

poolConfig := &util.PoolConfig{
    MaxSize:          5,               // Số thực thể tối đa
    MinSize:          1,               // Số thực thể được tạo sẵn
    MaxIdle:          3,               // Số thực thể rảnh (idle) tối đa
    AcquireTimeout:   5 * time.Second, // Thời gian timeout khi lấy resource
    IdleTimeout:      2 * time.Minute, // Thời gian timeout khi rảnh
    ValidateOnBorrow: true,            // Kiểm tra khi lấy resource
}

pool, err := NewWebRTCVADPool(vadConfig, poolConfig)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()

// Lấy thực thể VAD
vad, err := pool.AcquireVAD()
if err != nil {
    log.Fatal(err)
}

// Sử dụng VAD
isActive, err := vad.IsVAD(audioData)

// Giải phóng thực thể VAD
pool.ReleaseVAD(vad)
```

### Sử dụng đồng thời (concurrency)

```go
manager, err := NewVADManager(config)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// Nhiều goroutine xử lý đồng thời
for i := 0; i < numWorkers; i++ {
    go func(workerID int) {
        audioData := generateAudioData() // Tạo dữ liệu audio

        active, err := manager.ProcessAudio(audioData)
        if err != nil {
            log.Printf("Worker %d failed: %v", workerID, err)
            return
        }

        fmt.Printf("Worker %d: active = %v\n", workerID, active)
    }(i)
}
```

## Tham số cấu hình

### WebRTCVADConfig

- `SampleRate`: tốc độ lấy mẫu (8000, 16000, 32000, 48000)
- `Mode`: chế độ độ nhạy của VAD (0: kém nhạy nhất, 3: nhạy nhất)

### PoolConfig

- `MaxSize`: số lượng resource tối đa
- `MinSize`: số lượng resource tối thiểu (được tạo sẵn)
- `MaxIdle`: số lượng resource rảnh (idle) tối đa
- `AcquireTimeout`: thời gian timeout khi lấy resource
- `IdleTimeout`: thời gian timeout khi resource ở trạng thái rảnh
- `ValidateOnBorrow`: có kiểm tra resource khi lấy hay không
- `ValidateOnReturn`: có kiểm tra resource khi trả về hay không

## Ưu điểm

1. **Tái sử dụng resource**: tránh việc tạo và hủy thực thể VAD liên tục
2. **An toàn khi đồng thời**: hỗ trợ nhiều goroutine sử dụng cùng lúc
3. **Quản lý tự động**: tự động dọn dẹp các resource rảnh quá thời gian timeout
4. **Giám sát hiệu năng**: cung cấp thông tin thống kê chi tiết
5. **Cấu hình linh hoạt**: hỗ trợ tùy chỉnh kích thước pool và các tham số timeout

## Thống kê hiệu năng

Sử dụng phương thức `GetStats()` để lấy thông tin thống kê của resource pool:

```go
stats := manager.GetStats()
fmt.Printf("Pool stats: %+v\n", stats)
// Ví dụ đầu ra:
// {
//   "total_resources": 3,
//   "available_resources": 2,
//   "in_use_resources": 1,
//   "max_size": 5,
//   "min_size": 1,
//   "max_idle": 3,
//   "is_closed": false
// }
```

## Xử lý lỗi

Các loại lỗi chính:

- Timeout khi lấy resource: `acquire timeout after 5s`
- Resource pool đã đóng: `pool is closed`
- Kiểu resource không hợp lệ: `invalid resource type`
- Khởi tạo VAD thất bại: `failed to initialize WebRTC VAD`

## Thực hành tốt nhất

1. **Thiết lập kích thước pool hợp lý**: dựa theo nhu cầu xử lý đồng thời để thiết lập `MaxSize`
2. **Giải phóng resource kịp thời**: dùng `defer` để đảm bảo resource được giải phóng
3. **Giám sát thông tin thống kê**: thường xuyên kiểm tra tình trạng sử dụng của pool
4. **Đóng một cách an toàn (graceful shutdown)**: gọi phương thức `Close()` khi chương trình thoát
5. **Xử lý lỗi**: xử lý các trường hợp ngoại lệ như timeout khi lấy resource

## Mã nguồn ví dụ

Xem file `example_usage.go` để có ví dụ đầy đủ:

- Ví dụ sử dụng cơ bản
- Ví dụ xử lý hàng loạt
- Ví dụ sử dụng callback function
- Ví dụ sử dụng đồng thời

## Kiểm thử (Testing)

Chạy kiểm thử:

```bash
go test -v ./internal/domain/vad/webrtc_vad/
```

Chạy kiểm thử hiệu năng (benchmark):

```bash
go test -bench=. ./internal/domain/vad/webrtc_vad/
```
