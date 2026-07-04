# Tài liệu tính năng nhận dạng giọng nói (Speaker Identification)

> Nhận dạng giọng nói (Speaker Identification) là một tính năng cốt lõi trong dự án milestones-esp32-server-golang, dùng để nhận diện danh tính người dùng ở phía thiết bị, và chuyển đổi giọng đọc TTS tương ứng dựa trên kết quả nhận diện.

---

## I. Tổng quan tính năng

Nhận dạng giọng nói hoạt động bằng cách trích xuất đặc trưng giọng nói (embedding) từ âm thanh của người dùng, sau đó so khớp với dữ liệu giọng nói đã đăng ký trước đó để xác định danh tính người nói.

### Các khả năng cốt lõi

| Khả năng                   | Mô tả                                                                          |
| -------------------------- | ------------------------------------------------------------------------------ |
| 🎤 **Đăng ký giọng nói**   | Tải lên mẫu âm thanh của người dùng, trích xuất đặc trưng giọng nói và lưu trữ |
| 🔍 **Nhận dạng giọng nói** | Nhận diện danh tính người nói theo thời gian thực                              |
| ✅ **Xác thực giọng nói**  | Xác minh âm thanh có thuộc về người dùng chỉ định hay không                    |
| 📡 **Nhận dạng streaming** | Nhận dạng giọng nói streaming theo thời gian thực qua WebSocket                |
| 🔊 **Chuyển đổi TTS động** | Tự động chuyển đổi giọng đọc TTS tương ứng dựa trên kết quả nhận diện          |

---

## II. Kiến trúc hệ thống

### 2.1 Kiến trúc tổng thể

```
┌──────────────────┐     ┌──────────────────────┐     ┌──────────────────┐
│    Thiết bị ESP32   │────▶│ milestones-esp32-server │────▶│   voice-server   │
│  (thu âm thanh)     │     │     (dịch vụ chính)   │     │ (dịch vụ nhận dạng giọng nói) │
└──────────────────┘     └──────────────────────┘     └──────────────────┘
                                                              │
                                                              ▼
                                                      ┌──────────────────┐
                                                      │   Qdrant Vector DB   │
                                                      │ (lưu đặc trưng giọng nói) │
                                                      └──────────────────┘
```

### 2.2 Mô tả các thành phần

| Thành phần                      | Nhiệm vụ                                                                                                     |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| **milestones-esp32-server**     | Dịch vụ chính, chịu trách nhiệm kết nối thiết bị, quản lý phiên (session), xử lý kết quả nhận dạng giọng nói |
| **voice-server (asr_server)**   | Dịch vụ nhận dạng giọng nói, chịu trách nhiệm trích xuất đặc trưng, đăng ký, nhận dạng, xác thực             |
| **Manager (quản trị hệ thống)** | Trang quản trị Web, cung cấp API và giao diện quản lý nhóm giọng nói, quản lý mẫu                            |
| **Qdrant**                      | Cơ sở dữ liệu vector, lưu trữ vector đặc trưng giọng nói                                                     |

---

## III. Mô tả toàn bộ luồng xử lý

### 3.1 Luồng đăng ký giọng nói

```
Người dùng tải lên âm thanh → Manager API → Interface đăng ký của voice-server → Trích xuất embedding → Lưu vào Qdrant
                  │
                  ▼
            Lưu vào file cục bộ + bản ghi database
```

**Các bước chi tiết:**

1. Người dùng tải lên file âm thanh (định dạng WAV) trên giao diện Web của Manager
2. Backend Manager sinh ra một UUID duy nhất, lưu file âm thanh vào bộ nhớ cục bộ
3. Gọi interface `/api/v1/speaker/register` của voice-server
4. voice-server sử dụng model sherpa-onnx để trích xuất đặc trưng giọng nói (vector 192 chiều)
5. Đặc trưng giọng nói được lưu vào cơ sở dữ liệu vector Qdrant
6. Manager tạo bản ghi database `SpeakerSample`

### 3.2 Luồng nhận dạng giọng nói theo thời gian thực

```
ESP32 thu âm → VAD phát hiện giọng nói → Đồng thời gửi đến ASR và dịch vụ nhận dạng giọng nói
                                        │
                                        ▼
                              Nhận dạng streaming qua WebSocket
                                        │
                                        ▼
                              Lấy kết quả nhận dạng khi giọng nói kết thúc
                                        │
                                        ▼
                              Chuyển đổi giọng đọc TTS theo kết quả nhận dạng
```

**Các bước chi tiết:**

1. **Phát hiện VAD**: Âm thanh do ESP32 thu được sẽ đi qua bước phát hiện hoạt động giọng nói VAD (Voice Activity Detection)
2. **Gửi song song hai kênh**: Khi phát hiện có giọng nói, dữ liệu âm thanh được gửi đồng thời đến:
   - Dịch vụ ASR (chuyển giọng nói thành văn bản)
   - Dịch vụ nhận dạng giọng nói (nhận dạng streaming qua WebSocket)
3. **Xử lý streaming**: Dịch vụ nhận dạng giọng nói liên tục nhận các khối âm thanh (audio chunk)
4. **Lấy kết quả**: Khi phát hiện giọng nói kết thúc (im lặng), gọi `FinishAndIdentify` để lấy kết quả nhận dạng
5. **Chuyển đổi TTS**: Dựa trên kết quả nhận dạng, tự động chuyển sang giọng đọc TTS đã được cấu hình cho người dùng tương ứng

### 3.3 Điều kiện kích hoạt

Tính năng nhận dạng giọng nói cần đồng thời thỏa mãn các điều kiện sau mới được kích hoạt:

- `voice_identify.enable = true`: Bật tính năng nhận dạng giọng nói trong cấu hình toàn cục
- Trong cấu hình thiết bị có tồn tại cấu hình nhóm giọng nói (speaker group)
- `speakerManager` đã được khởi tạo thành công

---

## IV. Hướng dẫn cấu hình

### 4.1 Cấu hình chương trình chính (config.yaml)

Thêm cấu hình sau vào `config.yaml`:

```yaml
# Cấu hình nhận dạng giọng nói
voice_identify:
  enable: true # Có bật tính năng nhận dạng giọng nói hay không
  base_url: 'http://voice-server:8080' # Địa chỉ dịch vụ voice-server
  threshold: 0.6 # Ngưỡng nhận dạng giọng nói, phạm vi 0.0-1.0
```

| Tham số cấu hình | Kiểu dữ liệu | Giá trị mặc định | Mô tả                                                               |
| ---------------- | ------------ | ---------------- | ------------------------------------------------------------------- |
| `enable`         | bool         | false            | Có bật tính năng nhận dạng giọng nói hay không                      |
| `base_url`       | string       | -                | Địa chỉ HTTP của dịch vụ voice-server                               |
| `threshold`      | float        | 0.6              | Ngưỡng nhận dạng, giá trị càng cao yêu cầu độ khớp càng nghiêm ngặt |

### 4.2 Cấu hình Docker Compose

#### Biến môi trường của dịch vụ Backend

```yaml
backend:
  environment:
    - SPEAKER_SERVICE_URL=http://voice-server:8080
```

#### Biến môi trường của dịch vụ voice-server

```yaml
voice-server:
  environment:
    - VAD_ASR_SPEAKER_ENABLED=true
    - VAD_ASR_SPEAKER_VECTOR_DB_HOST=qdrant
    - VAD_ASR_SPEAKER_VECTOR_DB_PORT=6334
    - VAD_ASR_SPEAKER_VECTOR_DB_COLLECTION_NAME=speaker_embeddings
    - VAD_ASR_SPEAKER_THRESHOLD=0.6
    - VAD_ASR_LOGGING_LEVEL=info
```

| Biến môi trường                             | Mô tả                                          |
| ------------------------------------------- | ---------------------------------------------- |
| `VAD_ASR_SPEAKER_ENABLED`                   | Có bật tính năng nhận dạng giọng nói hay không |
| `VAD_ASR_SPEAKER_VECTOR_DB_HOST`            | Địa chỉ dịch vụ Qdrant                         |
| `VAD_ASR_SPEAKER_VECTOR_DB_PORT`            | Cổng gRPC của Qdrant                           |
| `VAD_ASR_SPEAKER_VECTOR_DB_COLLECTION_NAME` | Tên Collection trong Qdrant                    |
| `VAD_ASR_SPEAKER_THRESHOLD`                 | Ngưỡng nhận dạng giọng nói                     |
| `VAD_ASR_LOGGING_LEVEL`                     | Mức độ log                                     |

---

## V. Mô tả API

### 5.1 API trang quản trị Manager

#### Quản lý nhóm giọng nói (Speaker Group)

| Phương thức | Đường dẫn                        | Mô tả                        |
| ----------- | -------------------------------- | ---------------------------- |
| POST        | `/api/speaker-groups`            | Tạo nhóm giọng nói           |
| GET         | `/api/speaker-groups`            | Lấy danh sách nhóm giọng nói |
| GET         | `/api/speaker-groups/:id`        | Lấy chi tiết nhóm giọng nói  |
| PUT         | `/api/speaker-groups/:id`        | Cập nhật nhóm giọng nói      |
| DELETE      | `/api/speaker-groups/:id`        | Xóa nhóm giọng nói           |
| POST        | `/api/speaker-groups/:id/verify` | Xác thực giọng nói           |

#### Quản lý mẫu giọng nói (Speaker Sample)

| Phương thức | Đường dẫn                         | Mô tả                     |
| ----------- | --------------------------------- | ------------------------- |
| POST        | `/api/speaker-groups/:id/samples` | Thêm mẫu giọng nói        |
| GET         | `/api/speaker-groups/:id/samples` | Lấy danh sách mẫu         |
| GET         | `/api/speaker-samples/:id/audio`  | Lấy file âm thanh của mẫu |
| DELETE      | `/api/speaker-samples/:id`        | Xóa mẫu                   |

### 5.2 API của voice-server

#### Interface HTTP

| Phương thức | Đường dẫn                  | Mô tả                          |
| ----------- | -------------------------- | ------------------------------ |
| POST        | `/api/v1/speaker/register` | Đăng ký giọng nói              |
| POST        | `/api/v1/speaker/identify` | Nhận dạng giọng nói            |
| POST        | `/api/v1/speaker/verify`   | Xác thực giọng nói             |
| GET         | `/api/v1/speaker/list`     | Lấy danh sách tất cả người nói |
| DELETE      | `/api/v1/speaker/:id`      | Xóa người nói                  |
| GET         | `/api/v1/speaker/stats`    | Lấy thông tin thống kê         |

#### Nhận dạng streaming qua WebSocket

**Địa chỉ kết nối:** `ws://voice-server:8080/api/v1/speaker/stream`

**Luồng gửi/nhận thông điệp:**

1. Client gửi các khối âm thanh (PCM float32, little-endian)
2. Client gửi lệnh hoàn tất: `{"action": "finish"}`
3. Server trả về kết quả nhận dạng

---

## VI. Cơ sở dữ liệu vector (Qdrant)

### 6.1 Cấu trúc lưu trữ dữ liệu

```json
{
  "uid": "ID người dùng",
  "agent_id": "ID agent (trợ lý ảo)",
  "speaker_id": "ID người nói (khóa chính của nhóm giọng nói)",
  "speaker_name": "Tên người nói (tên nhóm giọng nói)",
  "uuid": "Định danh duy nhất của mẫu",
  "sample_index": 0,
  "created_at": 1704672000,
  "updated_at": 1704672000
}
```

### 6.2 Cấu hình vector

| Cấu hình                   | Giá trị                                 |
| -------------------------- | --------------------------------------- |
| Số chiều vector            | 192                                     |
| Phương pháp đo khoảng cách | Cosine (độ tương đồng cosine)           |
| Tên Collection             | `speaker_embeddings` (có thể tùy chỉnh) |

### 6.3 Cách ly dữ liệu

Hỗ trợ cách ly dữ liệu theo nhiều chiều:

- **UID**: Cách ly ở cấp độ người dùng
- **Agent ID**: Cách ly ở cấp độ agent (trợ lý ảo)
- Các agent khác nhau của cùng một người dùng có thể có dữ liệu giọng nói độc lập với nhau

---

## VII. Cấu trúc bảng cơ sở dữ liệu

### 7.1 SpeakerGroup (Bảng nhóm giọng nói)

```sql
CREATE TABLE `speaker_groups` (
  `id` INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` INT UNSIGNED NOT NULL COMMENT 'ID người dùng sở hữu',
  `agent_id` INT UNSIGNED NOT NULL COMMENT 'ID agent liên kết',
  `name` VARCHAR(100) NOT NULL COMMENT 'Tên nhóm giọng nói',
  `prompt` TEXT COMMENT 'Prompt vai trò (role prompt)',
  `description` TEXT COMMENT 'Thông tin mô tả',
  `tts_config_id` VARCHAR(100) COMMENT 'ID cấu hình TTS',
  `voice` VARCHAR(200) COMMENT 'Giá trị giọng đọc',
  `status` VARCHAR(20) NOT NULL DEFAULT 'active',
  `sample_count` INT NOT NULL DEFAULT 0 COMMENT 'Số lượng mẫu',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
);
```

### 7.2 SpeakerSample (Bảng mẫu giọng nói)

```sql
CREATE TABLE `speaker_samples` (
  `id` INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `speaker_group_id` INT UNSIGNED NOT NULL COMMENT 'ID nhóm giọng nói liên kết',
  `user_id` INT UNSIGNED NOT NULL COMMENT 'ID người dùng sở hữu',
  `uuid` VARCHAR(36) NOT NULL COMMENT 'Định danh duy nhất UUID',
  `file_path` VARCHAR(500) NOT NULL COMMENT 'Đường dẫn lưu trữ file âm thanh cục bộ',
  `file_name` VARCHAR(255) COMMENT 'Tên file gốc',
  `file_size` BIGINT COMMENT 'Kích thước file (byte)',
  `duration` FLOAT COMMENT 'Thời lượng âm thanh (giây)',
  `status` VARCHAR(20) NOT NULL DEFAULT 'active',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE INDEX `idx_uuid` (`uuid`)
);
```

---

## VIII. Hướng dẫn sử dụng

### 8.1 Triển khai voice-server

Tham khảo cấu hình triển khai đầy đủ tại [docker_compose.md](docker_compose.md), đảm bảo các dịch vụ sau đã được khởi động:

- **Qdrant**: Cơ sở dữ liệu vector
- **voice-server**: Dịch vụ nhận dạng giọng nói

### 8.2 Cấu hình chương trình chính

Thêm cấu hình nhận dạng giọng nói vào file `config.yaml` của chương trình chính:

```yaml
voice_identify:
  enable: true
  base_url: 'http://voice-server:8080'
  threshold: 0.6
```

### 8.3 Tạo nhóm giọng nói

1. Đăng nhập vào trang quản trị Web Manager
2. Vào mục "Agent (Trợ lý ảo)" → chọn agent mục tiêu → "Quản lý giọng nói"
3. Bấm "Tạo nhóm giọng nói mới", điền tên, mô tả, v.v.
4. Cấu hình giọng đọc TTS tương ứng (tuỳ chọn)

### 8.4 Tải lên mẫu giọng nói

1. Tại trang chi tiết nhóm giọng nói, bấm "Thêm mẫu"
2. Tải lên file âm thanh định dạng WAV (khuyến nghị 3-10 giây, giọng nói rõ ràng)
3. Hệ thống tự động trích xuất đặc trưng giọng nói và lưu trữ

### 8.5 Kiểm tra nhận dạng giọng nói

1. Tại trang chi tiết nhóm giọng nói, bấm "Xác thực"
2. Tải lên âm thanh kiểm tra
3. Xem kết quả nhận dạng và độ tin cậy (confidence)

---

## IX. Các điểm kỹ thuật quan trọng

### 9.1 Trích xuất đặc trưng giọng nói

- Sử dụng model **sherpa-onnx** để trích xuất đặc trưng giọng nói
- Đầu ra là vector embedding 192 chiều
- Hỗ trợ đầu vào với bất kỳ tần số lấy mẫu (sample rate) nào, tự động resample

### 9.2 Tính toán độ tương đồng

- Sử dụng **độ tương đồng Cosine** (Cosine Similarity) để tính mức độ khớp giọng nói
- Phạm vi độ tương đồng: [-1, 1]
- Ngưỡng mặc định 0.6, có thể điều chỉnh tùy theo tình huống thực tế

### 9.3 Tiền xử lý VAD

- Sử dụng TEN-VAD để lọc khoảng lặng (silence)
- Khi đăng ký, giữ lại 100ms khoảng lặng ở đầu và cuối
- Khi nhận dạng thời gian thực, chỉ gửi đoạn âm thanh được VAD phát hiện là có hoạt động giọng nói

---

## X. Câu hỏi thường gặp

### Q1: Nhận dạng giọng nói không hoạt động?

Kiểm tra các cấu hình sau:

1. `voice_identify.enable` có được đặt là `true` không
2. `voice_identify.base_url` có chính xác không
3. Thiết bị đã được cấu hình nhóm giọng nói chưa
4. Dịch vụ voice-server có đang chạy bình thường không

### Q2: Độ chính xác nhận dạng thấp?

- Nâng cao chất lượng mẫu giọng nói (rõ ràng, không nhiễu, 3-10 giây)
- Tăng số lượng mẫu giọng nói (khuyến nghị 3-5 mẫu)
- Điều chỉnh ngưỡng nhận dạng

### Q3: Giọng đọc TTS không chuyển đổi?

Kiểm tra xem trường `tts_config_id` hoặc `voice` trong cấu hình nhóm giọng nói đã được cấu hình chính xác chưa.

---

## XI. Tài liệu liên quan

- [Triển khai Docker Compose](docker_compose.md)
- [Tài liệu cấu hình](config.md)
- [Nhận dạng thị giác (Vision)](vision.md)
