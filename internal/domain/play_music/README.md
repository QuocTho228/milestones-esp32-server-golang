# Chức năng phát nhạc

Module này cung cấp chức năng phát nhạc dạng streaming từ URL, hỗ trợ lấy file âm thanh từ URL trên mạng và giải mã thời gian thực thành luồng khung âm thanh (audio frame stream).

## Đặc điểm chức năng

- ✅ **Phát dạng streaming**: Hỗ trợ tải và phát nhạc theo thời gian thực từ URL
- ✅ **Hỗ trợ định dạng**: Chủ yếu hỗ trợ định dạng MP3, tự động giải mã thành khung âm thanh Opus
- ✅ **Giải mã âm thanh**: Dựa trên bộ giải mã âm thanh (decoder) đã hoàn thiện, hiệu quả và ổn định
- ✅ **Kiểm soát context**: Hỗ trợ hủy và kiểm soát timeout thông qua context
- ✅ **Tối ưu connection pool**: Sử dụng connection pool HTTP, nâng cao hiệu năng mạng
- ✅ **Cấu hình linh hoạt**: Có thể cấu hình thời lượng khung (frame duration) và định dạng âm thanh
- ✅ **Thông tin thống kê**: Cung cấp thống kê phát và giám sát trạng thái

## Bắt đầu nhanh

### 1. Sử dụng cơ bản

```go
package main

import (
    "context"
    "fmt"

    "milestones-esp32-server-golang/internal/domain/play_music"
)

func main() {
    // Tạo trình phát nhạc
    config := play_music.DefaultMusicPlayerConfig()
    player := play_music.NewMusicPlayer(config.ToMap())

    // Bắt đầu phát nhạc
    ctx := context.Background()
    audioChan, err := player.PlayMusicStream(ctx, "https://example.com/music.mp3")
    if err != nil {
        panic(err)
    }

    // Xử lý khung âm thanh
    for audioFrame := range audioChan {
        fmt.Printf("Đã nhận khung âm thanh: %d byte\n", len(audioFrame))
        // Ở đây có thể gửi khung âm thanh tới thiết bị phát hoặc xử lý khác
    }
}
```

### 2. Cấu hình tùy chỉnh

```go
// Tạo cấu hình tùy chỉnh
config := &play_music.MusicPlayerConfig{
    FrameDuration: 20,   // Thời lượng khung 20ms
}

player := play_music.NewMusicPlayer(config.ToMap())

// Hoặc truyền trực tiếp bản đồ cấu hình
player := play_music.NewMusicPlayer(map[string]interface{}{
    "frame_duration": 20,
    "audio_format":   "mp3",
})
```

### 3. Ví dụ đầy đủ kèm thông tin thống kê

```go
package main

import (
    "context"
    "fmt"
    "time"

    "milestones-esp32-server-golang/internal/domain/play_music"
)

func main() {
    config := play_music.DefaultMusicPlayerConfig()
    player := play_music.NewMusicPlayer(config.ToMap())

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    audioChan, err := player.PlayMusicStream(ctx, "https://example.com/music.mp3")
    if err != nil {
        panic(err)
    }

    // Thông tin thống kê
    stats := &play_music.StreamingStats{
        StartTime: time.Now().UnixMilli(),
    }

    frameCount := 0
    for audioFrame := range audioChan {
        frameCount++
        stats.FramesGenerated = int64(frameCount)
        stats.BytesDecoded += int64(len(audioFrame))

        if frameCount == 1 {
            stats.FirstFrameTime = time.Now().UnixMilli()
            fmt.Printf("Độ trễ khung đầu tiên: %d ms\n", stats.FirstFrameTime - stats.StartTime)
        }

        // Xử lý khung âm thanh...
    }

    fmt.Printf("Phát hoàn tất, tổng số khung: %d\n", frameCount)
}
```

## Tham khảo API

### MusicPlayer

Struct trình phát nhạc chính.

#### Các phương thức

##### `NewMusicPlayer(config map[string]interface{}) *MusicPlayer`

Tạo một instance trình phát nhạc mới.

**Tham số:**

- `config`: Bản đồ tham số cấu hình

**Các tùy chọn cấu hình:**

- `frame_duration` (int): Thời lượng khung (ms), mặc định 20
- `audio_format` (string): Định dạng âm thanh, mặc định "mp3"

##### `PlayMusicStream(ctx context.Context, url string) (chan []byte, error)`

Bắt đầu phát nhạc dạng streaming từ URL.

**Tham số:**

- `ctx`: Đối tượng context, dùng để kiểm soát hủy và timeout
- `url`: Địa chỉ URL của file nhạc

**Trả về:**

- `chan []byte`: Channel dữ liệu khung âm thanh
- `error`: Thông tin lỗi

##### `GetPlayerInfo() map[string]interface{}`

Lấy thông tin cấu hình của trình phát.

##### `Stop() error`

Dừng trình phát và dọn dẹp tài nguyên.

### Các kiểu cấu hình

#### `MusicPlayerConfig`

```go
type MusicPlayerConfig struct {
    FrameDuration int    `json:"frame_duration"` // Thời lượng khung (ms)
    AudioFormat   string `json:"audio_format"`   // Định dạng âm thanh, mặc định "mp3"
}
```

#### `StreamingStats`

Struct thông tin thống kê phát, dùng để giám sát trạng thái phát.

```go
type StreamingStats struct {
    BytesDownloaded int64         `json:"bytes_downloaded"`
    BytesDecoded    int64         `json:"bytes_decoded"`
    FramesGenerated int64         `json:"frames_generated"`
    StartTime       int64         `json:"start_time"`
    FirstFrameTime  int64         `json:"first_frame_time"`
    Status          PlaybackStatus `json:"status"`
    ErrorCount      int           `json:"error_count"`
}
```

## Kiểm thử (Testing)

Chạy ví dụ test:

```bash
cd test/music_player
go run main.go "https://example.com/music.mp3"
```

## Các định dạng âm thanh được hỗ trợ

Hiện tại chủ yếu hỗ trợ:

- **MP3**: Hỗ trợ hoàn toàn, khuyến nghị sử dụng
- **WAV**: Hỗ trợ một phần (thông qua bộ giải mã chung)

## Xử lý lỗi

Trình phát cung cấp cơ chế xử lý lỗi gọn gàng:

1. **Tối ưu connection pool**: Sử dụng connection pool HTTP để nâng cao độ ổn định mạng
2. **Kiểm soát context**: Hỗ trợ hủy thao tác thông qua context
3. **Thoát an toàn (graceful exit)**: Đóng channel một cách an toàn khi gặp lỗi

## Đề xuất tối ưu hiệu năng

1. **Thiết lập thời lượng khung hợp lý**: Mặc định 20ms phù hợp với hầu hết các kịch bản
2. **Tối ưu mạng**: Sử dụng kết nối mạng ổn định, trình phát đã được tối ưu connection pool HTTP
3. **Quản lý bộ nhớ**: Xử lý kịp thời dữ liệu khung âm thanh, tránh tắc nghẽn channel
4. **Kiểm soát đồng thời**: Tránh phát quá nhiều luồng âm thanh cùng lúc

## Ví dụ tích hợp

### Tích hợp với WebSocket

```go
func streamToWebSocket(audioChan <-chan []byte, ws *websocket.Conn) {
    for frame := range audioChan {
        err := ws.WriteMessage(websocket.BinaryMessage, frame)
        if err != nil {
            log.Errorf("Gửi tin nhắn WebSocket thất bại: %v", err)
            return
        }
    }
}
```

### Lưu vào file

```go
func saveToFile(audioChan <-chan []byte, filename string) error {
    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    for frame := range audioChan {
        _, err := file.Write(frame)
        if err != nil {
            return err
        }
    }
    return nil
}
```

## Lưu ý

1. **Tính hợp lệ của URL**: Đảm bảo URL âm thanh có thể truy cập được và trả về file âm thanh hợp lệ
2. **Sử dụng bộ nhớ**: Cần chú ý tình trạng sử dụng bộ nhớ khi phát trong thời gian dài
3. **Độ ổn định mạng**: Sử dụng kết nối mạng ổn định để có trải nghiệm phát tốt nhất
4. **Quản lý context**: Hủy kịp thời các tác vụ phát không cần thiết

## Xử lý sự cố

### Các câu hỏi thường gặp

**Hỏi: Phát không có âm thanh**
Đáp: Kiểm tra URL có hợp lệ hay không, định dạng âm thanh có được hỗ trợ hay không

**Hỏi: Độ trễ khi phát rất cao**
Đáp: Kiểm tra kết nối mạng, đảm bảo URL phản hồi đủ nhanh

**Hỏi: Sử dụng bộ nhớ quá cao**
Đáp: Kiểm tra việc xử lý khung âm thanh có kịp thời hay không, tránh việc dữ liệu bị dồn ứ trong channel

## Giấy phép (License)

MIT License
