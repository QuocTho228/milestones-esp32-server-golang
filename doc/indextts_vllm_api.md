# Tài liệu hướng dẫn kết nối interface IndexTTS vLLM

Tài liệu này dùng để giải thích các yêu cầu về interface phía server khi dự án này tích hợp với `indextts_vllm`, áp dụng cho:

- Suy luận TTS của chương trình chính (`/audio/speech`)
- Giao diện quản trị viên lấy danh sách âm sắc (voice) (`/audio/voices`)
- Nhân bản giọng nói của người dùng (`/audio/clone`, dùng cho luồng nhân bản của dự án này)

## 1. Danh sách tương thích nhanh

Dịch vụ IndexTTS của bạn tối thiểu cần thỏa mãn ba điểm sau:

- Cung cấp `POST /audio/speech`, tham số đầu vào tương thích theo phong cách OpenAI TTS: `input`, `voice`, `model`
- Cung cấp `GET /audio/voices`, trả về danh sách âm sắc có thể liệt kê (JSON object)
- Nếu sử dụng năng lực "nhân bản giọng nói" của dự án này, cần cung cấp `POST /audio/clone` (`multipart/form-data`)

Định dạng âm thanh trả về được khuyến nghị: `audio/wav` (16-bit PCM).

## 2. Ánh xạ mục cấu hình (Quản trị viên -> Cấu hình TTS -> IndexTTS(vLLM))

| Trường phía quản trị | Công dụng                | Vị trí gửi đi                                         |
| -------------------- | ------------------------ | ----------------------------------------------------- |
| `api_url`            | Địa chỉ dịch vụ IndexTTS | Dùng làm URL gốc, ghép với endpoint                   |
| `api_key`            | Xác thực (tùy chọn)      | `Authorization: Bearer <api_key>`                     |
| `model`              | Tên model                | Trường `model` trong request body của `/audio/speech` |
| `voice`              | Âm sắc mặc định          | Trường `voice` trong request body của `/audio/speech` |
| `frame_duration`     | Độ dài frame (ms)        | Tham số chia frame âm thanh cục bộ                    |

Giải thích:

- Khi giao diện quản trị viên bấm vào dropdown "âm sắc", hệ thống sẽ dùng giá trị `api_url` mới nhất đang có trong ô input để lấy dữ liệu từ `/audio/voices`.
- `api_url` hỗ trợ điền địa chỉ gốc (ví dụ `http://127.0.0.1:7860`), cũng tương thích với việc điền tới một đường dẫn cụ thể (ví dụ `/audio/speech`).

## 3. Yêu cầu về interface

### 3.1 `GET /audio/voices`

Công dụng: dropdown "âm sắc" ở trang cấu hình quản trị viên, tùy chọn âm sắc phía người dùng.

Header của request:

- `Accept: application/json`
- `Authorization: Bearer <api_key>` (tùy chọn)

Ví dụ dữ liệu trả về (khuyến nghị):

```json
{
  "demo_speaker": ["assets/speaker/demo.wav"],
  "narrator_cn_female": ["assets/speaker/narrator_cn_female.wav"]
}
```

Yêu cầu:

- Kiểu dữ liệu trả về nên là JSON object (tên key sẽ được dùng làm ID âm sắc).
- Dự án này sẽ lọc bỏ các âm sắc hệ thống có tiền tố `indextts_vllm`, sau đó thêm vào các âm sắc nhân bản của người dùng.

### 3.2 `POST /audio/speech`

Công dụng: tổng hợp TTS của chương trình chính, nghe thử sau khi nhân bản.

Header của request:

- `Content-Type: application/json`
- `Accept: audio/wav,application/octet-stream,*/*`
- `Authorization: Bearer <api_key>` (tùy chọn)

Ví dụ request body:

```json
{
  "model": "indextts-vllm",
  "input": "Xin chào, chào mừng bạn đến với IndexTTS.",
  "voice": "demo_speaker"
}
```

Trả về:

- Thành công: luồng âm thanh dạng nhị phân (binary) (khuyến nghị `audio/wav`)
- Thất bại: HTTP 4xx/5xx, và trả về thông tin lỗi có thể đọc được

### 3.3 `POST /audio/clone` (cần thiết cho chức năng nhân bản của dự án này)

Công dụng: được gọi khi `/user/voice-clones` gửi yêu cầu (task) nhân bản.

Kiểu request: `multipart/form-data`

Các trường trong form:

- `voice`: ID âm sắc mong muốn được tạo ra
- `audio`: file âm thanh tham chiếu (wav/mp3/m4a, v.v.)

Ví dụ dữ liệu trả về:

```json
{
  "voice": "demo_speaker_clone_001",
  "ok": true
}
```

Yêu cầu:

- Khuyến nghị response nên chứa trường `voice`; nếu thiếu, dự án này sẽ lùi về (fallback) dùng giá trị trường `voice` trong request.

## 4. Tham khảo tương thích (api_server.py)

Có thể tham khảo phong cách triển khai sau:

- `POST /audio/speech`: đọc `input`, `voice`, `model`
- `GET /audio/voices`: trả về dictionary các âm sắc khả dụng

Link tham khảo:

- https://github.com/quoctho228/index-tts-vllm/blob/master/api_server.py

## 5. Xử lý sự cố thường gặp

### 5.1 Khi bấm dropdown âm sắc ở phía quản trị bị báo lỗi

Ưu tiên kiểm tra:

- `api_url` có truy cập được không (giá trị nhập mới nhất)
- `/audio/voices` có trả về đúng JSON object không
- Có cần `api_key` hay không

### 5.2 Tổng hợp thành công nhưng phát bị lỗi

Ưu tiên kiểm tra:

- Phía server có trả về đúng chuẩn WAV không (PCM16, sample rate đúng)
- Chuỗi xử lý ở giữa có bị chuyển mã (transcode) hoặc cắt xén (truncate) không
- Header `Content-Type` trong response có đúng không

### 5.3 Task nhân bản thất bại

Ưu tiên kiểm tra:

- `/audio/clone` có chấp nhận request multipart với `voice + audio` không
- JSON trả về có phân tích được không, có chứa `voice` khả dụng không
