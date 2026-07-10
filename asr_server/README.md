# 🎤 Máy chủ Nhận dạng Giọng nói VAD ASR

Dịch vụ nhận dạng giọng nói hiệu năng cao dựa trên Sherpa-ONNX, hỗ trợ VAD (phát hiện hoạt động giọng nói) theo thời gian thực, nhận dạng đa ngôn ngữ và nhận dạng giọng nói người dùng (speaker recognition / voiceprint).

## ✨ Tính năng

- Nhận dạng giọng nói đa ngôn ngữ theo thời gian thực (Trung/Anh/Nhật/Hàn/Quảng Đông...)
- VAD phân đoạn thông minh, tự động lọc bỏ khoảng lặng
- Nhận dạng giọng nói người dùng (speaker/voiceprint recognition)
- Giao tiếp thời gian thực qua WebSocket, độ trễ thấp
- Kiểm tra sức khỏe (health check), giám sát trạng thái, tắt dịch vụ an toàn (graceful shutdown)

## 🚀 Bắt đầu nhanh

### Cách 1: Triển khai bằng Docker (khuyến nghị)

> **Khuyến nghị: Image Docker đã tự động bao gồm các file model chính (vad, asr, speaker) và thư mục lib, không cần mount thủ công thư mục models hoặc lib.**

#### Build image

```bash
docker build -t voice_server .
```

#### Chạy container (giả sử cổng 8080)

```bash
docker run -d -p 8080:8080 --name voice_server voice_server
```

#### Cấu hình Qdrant bằng biến môi trường (tùy chọn)

Nếu sử dụng cơ sở dữ liệu vector Qdrant, bạn có thể cấu hình thông tin kết nối thông qua biến môi trường (được ưu tiên hơn file cấu hình):

```bash
docker run -d -p 8080:8080 \
  -e QDRANT_HOST=qdrant-server \
  -e QDRANT_PORT=6334 \
  -e QDRANT_COLLECTION_NAME=speaker_embeddings \
  --name voice_server voice_server
```

#### Cổng và truy cập

- Trang thử nghiệm: http://localhost:8080/
- Kiểm tra sức khỏe: http://localhost:8080/health
- WebSocket: ws://localhost:8080/ws

---

### Cách 2: Triển khai từ mã nguồn (nâng cao/dành cho nhà phát triển)

#### Yêu cầu hệ thống

- Go 1.21+
- Linux/macOS/Windows
- Khuyến nghị RAM từ 4GB trở lên

#### Cài đặt và chuẩn bị phụ thuộc (dependencies)

```bash
# Clone dự án
git clone https://github.com/bbeyondllove/voice_server.git
cd voice_server
# Cài đặt các gói phụ thuộc Go
go mod tidy
# Sao chép thư viện động vào thư mục thư viện hệ thống (Linux)
cp lib/*.so /usr/lib/
cp lib/ten-vad/lib/Linux/x64/libten_vad.so /usr/lib/
# Cài đặt thư viện runtime C++ (nếu chưa có)
sudo apt install libc++1
```

#### Chuẩn bị model

```bash
sudo apt install git-lfs
git-lfs install
# Tải model ASR
mkdir -p models/asr
# Khuyến nghị dùng mirror Hugging Face để tải nhanh hơn
git clone https://huggingface.co/csukuangfj/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17 models/asr/

# Tải model nhận dạng giọng nói người dùng (speaker recognition)
mkdir -p models/speaker
wget -O models/speaker/3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx \
  https://huggingface.co/csukuangfj/speaker-embedding-models/resolve/main/3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx
```

#### Build cục bộ trên Windows (bắt buộc đọc)

Dự án này phụ thuộc vào thư viện `sherpa-onnx-go` cho các chức năng ASR/nhận dạng giọng nói, thư viện này gọi thư viện gốc (native library) thông qua CGO, **trên Windows bắt buộc phải bật CGO** thì mới build được. Nếu không bật, sẽ gặp lỗi: `build constraints exclude all Go files in ... sherpa-onnx-go-windows`.

Trên Windows (PowerShell hoặc CMD), hãy thiết lập biến môi trường trước khi build:

```bash
# PowerShell
$env:CGO_ENABLED=1
go build -o main.exe .

# CMD
set CGO_ENABLED=1
go build -o main.exe .
```

Hoặc dùng trực tiếp script: `scripts\build_windows.bat`. Cần cài sẵn MinGW (gcc) hoặc MSVC, và đặt các file DLL của sherpa-onnx vào đường dẫn có thể được nạp (ví dụ: cùng thư mục với file exe hoặc trong PATH).

#### Chạy dịch vụ

```bash
# Khởi động với cấu hình mặc định
go run main.go
# Hoặc build xong rồi chạy (Linux/macOS)
go build -o voice_server
./voice_server
```

#### Kiểm tra truy cập

- Trang thử nghiệm: http://localhost:8080/
- Kiểm tra sức khỏe: http://localhost:8080/health
- WebSocket: ws://localhost:8080/ws

---

## ⚙️ Cấu hình

### File cấu hình

Vui lòng tham khảo chi tiết trong file `config.json`.

### Cấu hình lưu trữ giọng nói người dùng (speaker storage)

Chức năng nhận dạng giọng nói người dùng hỗ trợ hai phương thức lưu trữ, có thể chọn thông qua cấu hình `speaker.storage_type`:

| Loại lưu trữ | Mô tả                             | Trường hợp sử dụng                                              |
| ------------ | --------------------------------- | --------------------------------------------------------------- |
| `json`       | Lưu trữ bằng file JSON (mặc định) | Triển khai nhỏ, phát triển/kiểm thử, không cần dịch vụ bổ sung  |
| `qdrant`     | Cơ sở dữ liệu vector Qdrant       | Môi trường production, triển khai quy mô lớn, cần hiệu năng cao |

**Ví dụ cấu hình lưu trữ JSON:**

```jsonc
"speaker": {
  "storage_type": "json",
  "json_storage": {
    "file_path": "data/speaker/speaker_embeddings.json"
  }
}
```

**Ví dụ cấu hình lưu trữ Qdrant:**

```jsonc
"speaker": {
  "storage_type": "qdrant",
  "vector_db": {
    "host": "localhost",
    "port": 6334,
    "collection_name": "speaker_embeddings"
  }
}
```

### Cấu hình bằng biến môi trường (khuyến nghị khi triển khai Docker)

Để hỗ trợ triển khai Docker, các mục cấu hình sau đây sẽ được ưu tiên đọc từ biến môi trường; nếu biến môi trường không tồn tại thì sẽ dùng giá trị trong file cấu hình:

| Biến môi trường          | Mô tả                     | Đường dẫn tương ứng trong file cấu hình | Giá trị mặc định     |
| ------------------------ | ------------------------- | --------------------------------------- | -------------------- |
| `QDRANT_HOST`            | Địa chỉ máy chủ Qdrant    | `speaker.vector_db.host`                | `localhost`          |
| `QDRANT_PORT`            | Cổng máy chủ Qdrant       | `speaker.vector_db.port`                | `6334`               |
| `QDRANT_COLLECTION_NAME` | Tên collection của Qdrant | `speaker.vector_db.collection_name`     | `speaker_embeddings` |

**Ví dụ:**

```bash
# Cấu hình Qdrant bằng biến môi trường
export QDRANT_HOST=qdrant-server
export QDRANT_PORT=6334
export QDRANT_COLLECTION_NAME=speaker_embeddings

# Chạy dịch vụ
./voice_server
```

## 🔌 Ví dụ WebSocket API

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onopen = () => ws.send(audioBuffer);
ws.onmessage = (e) => console.log('Kết quả nhận dạng:', e.data);
```

## 🏛️ Kiến trúc hệ thống

```
┌────────────────────┐    ┌──────────────────────┐    ┌────────────────────┐
│  Client WebSocket   │    │  Pool VAD phát hiện   │    │  Module nhận dạng   │
│                    │    │  hoạt động giọng nói  │    │  ASR                │
│  ┌──────────────┐  │    │  ┌──────────────┐    │    │ (tạo stream động)   │
│  │ Luồng âm thanh│◄─┼───►│  │  Instance VAD│◄──┼───►│  ┌──────────────┐  │
│  │ đầu vào       │  │    │  └──────────────┘    │    │  │ Recognizer   │  │
│  └──────────────┘  │    │  ┌──────────────┐    │    │  └──────────────┘  │
│  ┌──────────────┐  │    │  │ Hàng đợi bộ  │    │    │                  │
│  │ Nhận kết quả  │  │    │  │ đệm          │    │    │                  │
│  │ nhận dạng     │  │    │  └──────────────┘    │    └────────────────────┘
│  └──────────────┘  │    └──────────────────────┘             │
└────────────────────┘                                          ▼
┌────────────────────┐    ┌──────────────────────┐    ┌────────────────────┐
│  Trình quản lý     │    │  Module nhận dạng     │    │  Kiểm tra sức khỏe/│
│  phiên (session)    │    │  giọng nói (tùy chọn) │    │  Giám sát          │
│  ┌──────────────┐  │    │  ┌──────────────┐    │    │                    │
│  │ Quản lý trạng │  │    │  │ Đăng ký       │    │    │  Giao diện giám   │
│  │ thái kết nối  │  │    │  │ người nói     │    │    │  sát/trạng thái   │
│  └──────────────┘  │    │  └──────────────┘    │    └────────────────────┘
│  ┌──────────────┐  │    │  ┌──────────────┐    │
│  │ Cấp phát/giải │  │    │  │ Trích xuất    │    │
│  │ phóng tài     │  │    │  │ đặc trưng     │    │
│  │ nguyên        │  │    │  │ giọng nói     │    │
│  └──────────────┘  │    │  └──────────────┘    │
└────────────────────┘    └──────────────────────┘
```

## 🎛️ Giải thích các tham số quan trọng

| Tham số                               | Mô tả                                      | Giá trị khuyến nghị |
| ------------------------------------- | ------------------------------------------ | ------------------- |
| `vad.provider`                        | Loại VAD (silero_vad hoặc ten_vad)         | ten_vad             |
| `vad.pool_size`                       | Số lượng instance trong pool VAD           | 200                 |
| `vad.threshold`                       | Ngưỡng phát hiện VAD                       | 0.5                 |
| `vad.silero_vad.min_silence_duration` | silero_vad: thời lượng im lặng tối thiểu   | 0.1                 |
| `vad.silero_vad.min_speech_duration`  | silero_vad: thời lượng giọng nói tối thiểu | 0.25                |
| `vad.silero_vad.max_speech_duration`  | silero_vad: thời lượng giọng nói tối đa    | 8.0                 |
| `vad.silero_vad.window_size`          | silero_vad: kích thước cửa sổ (window)     | 512                 |
| `vad.silero_vad.buffer_size_seconds`  | silero_vad: thời lượng bộ đệm (buffer)     | 10.0                |
| `vad.ten_vad.hop_size`                | ten-vad: bước nhảy khung (hop size)        | 512                 |
| `vad.ten_vad.min_speech_frames`       | ten-vad: số khung giọng nói tối thiểu      | 12                  |
| `vad.ten_vad.max_silence_frames`      | ten-vad: số khung im lặng tối đa           | 5                   |
| `recognition.num_threads`             | Số luồng (thread) xử lý ASR                | 8-16                |
| `audio.sample_rate`                   | Tần số lấy mẫu (sample rate)               | 16000               |
| `server.port`                         | Cổng dịch vụ                               | 8080                |

### Ví dụ cấu hình VAD

```jsonc
"vad": {
  "provider": "ten_vad",      // Chọn ten_vad hoặc silero_vad
  "pool_size": 200,
  "threshold": 0.5,
  "silero_vad": {
    "model_path": "models/vad/silero_vad/silero_vad.onnx",
    "min_silence_duration": 0.1,
    "min_speech_duration": 0.25,
    "max_speech_duration": 8.0,
    "window_size": 512,
    "buffer_size_seconds": 10.0
  },
  "ten_vad": {
    "hop_size": 512,
    "min_speech_frames": 12,
    "max_silence_frames": 5
  }
}
```

## 🧪 Ví dụ kiểm thử (test)

Dự án đi kèm sẵn các script kiểm thử trong thư mục test/asr/:

- `audiofile_test.py`: kiểm thử nhận dạng một file đơn lẻ, hỗ trợ file wav đa ngôn ngữ.
- `stress_test.py`: kiểm thử tải (stress test) đồng thời, mô phỏng nhiều kết nối nhận dạng cùng lúc.

Ví dụ cách dùng:

```bash
python stress_test.py --connections 100 --audio-per-connection 2
```

- `--connections`: số kết nối đồng thời (ví dụ 100 nghĩa là mô phỏng 100 client cùng lúc)
- `--audio-per-connection`: số file âm thanh mỗi kết nối cần gửi (ví dụ 2 nghĩa là mỗi kết nối gửi 2 file âm thanh)

Ví dụ trên sẽ mô phỏng 100 kết nối đồng thời, mỗi kết nối gửi 2 file âm thanh riêng, tổng cộng 200 yêu cầu nhận dạng.

## 🤝 Đóng góp

Rất hoan nghênh đóng góp mã nguồn! Quy trình như sau:

1. Fork dự án
2. Tạo nhánh tính năng (`git checkout -b feature/AmazingFeature`)
3. Commit thay đổi (`git commit -m 'Add some AmazingFeature'`)
4. Push lên nhánh (`git push origin feature/AmazingFeature`)
5. Mở Pull Request

## 📄 Giấy phép (License)

Toàn bộ dự án sử dụng giấy phép MIT. Tuy nhiên xin lưu ý:

- Nếu bạn sử dụng các tính năng liên quan đến ten-vad (tức đặt `vad.provider` là `ten_vad`), bạn cần tuân thủ [Giấy phép của ten-vad](https://github.com/ten-framework/ten-vad/blob/main/LICENSE).
- Nếu chỉ sử dụng silero-vad (tức đặt `vad.provider` là `silero_vad`), bạn có thể tuân theo giấy phép MIT trực tiếp.

Vui lòng tuân thủ giấy phép mã nguồn mở tương ứng theo loại VAD thực tế bạn sử dụng.

## 🙏 Lời cảm ơn

- [Sherpa-ONNX](https://github.com/k2-fsa/sherpa-onnx) - Engine nhận dạng giọng nói cốt lõi
- [SenseVoice](https://github.com/FunAudioLLM/SenseVoice) - Model nhận dạng giọng nói đa ngôn ngữ
- [Silero VAD](https://github.com/snakers4/silero-vad) - Model phát hiện hoạt động giọng nói
- [ten-vad](https://github.com/zhenghuatan/ten-vad) - Thuật toán phát hiện điểm cuối (endpoint detection) hiệu quả
