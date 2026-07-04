# Hướng dẫn sử dụng trang quản trị (Manager Console)

## Truy cập trang quản trị

- Địa chỉ: http://<IP máy chủ hoặc domain>:8080

---

## I. Trình hướng dẫn cấu hình (Configuration Wizard)

Sau lần đăng nhập đầu tiên, hệ thống sẽ tự động vào trình hướng dẫn cấu hình, gồm 5 bước.

### Bước 1: Cấu hình OTA

Cấu hình thông tin server OTA, dùng để đẩy xuống địa chỉ websocket và mqtt cho phần cứng Tiểu Trí (小智).

<!-- Vị trí ảnh chụp màn hình: Giao diện cấu hình OTA -->

> Hình: Giao diện trình hướng dẫn cấu hình OTA

| Mục cấu hình | Giải thích                |
| ------------ | ------------------------- |
| MQTT Broker  | Địa chỉ server MQTT       |
| MQTT Port    | Cổng MQTT (mặc định 1883) |
| UDP Port     | Cổng UDP                  |
| ...          | ...                       |

**Kiểm tra kết nối**: Bấm "Kiểm tra cấu hình hiện tại" để xác minh kết nối MQTT/UDP.

---

### Bước 2: Cấu hình VAD

Chọn engine phát hiện hoạt động giọng nói:

<!-- Vị trí ảnh chụp màn hình: Giao diện cấu hình VAD -->

> Hình: Giao diện trình hướng dẫn cấu hình VAD

| Engine     | Giải thích           | Kịch bản khuyến nghị  |
| ---------- | -------------------- | --------------------- |
| Silero VAD | Độ chính xác cao     | Môi trường production |
| WebRTC VAD | Nhẹ                  | Tài nguyên hạn chế    |
| ten_vad    | Phiên bản C++ cục bộ | Yêu cầu hiệu năng cao |

---

### Bước 3: Cấu hình ASR

Chọn engine nhận dạng giọng nói:

<!-- Vị trí ảnh chụp màn hình: Giao diện cấu hình ASR -->

> Hình: Giao diện trình hướng dẫn cấu hình ASR

| Engine     | Giải thích                      |
| ---------- | ------------------------------- |
| FunASR     | Nhận dạng cục bộ, cần tải model |
| Doubao ASR | API trên cloud                  |

---

### Bước 4: Cấu hình LLM

Chọn mô hình ngôn ngữ lớn:

<!-- Vị trí ảnh chụp màn hình: Giao diện cấu hình LLM -->

> Hình: Giao diện trình hướng dẫn cấu hình LLM

| Engine             | Giải thích            |
| ------------------ | --------------------- |
| Tương thích OpenAI | Hỗ trợ nhiều loại API |
| Ollama             | Triển khai cục bộ     |
| Doubao             | Doubao của ByteDance  |

---

### Bước 5: Cấu hình TTS

Chọn engine tổng hợp giọng nói:

<!-- Vị trí ảnh chụp màn hình: Giao diện cấu hình TTS -->

> Hình: Giao diện trình hướng dẫn cấu hình TTS

| Engine     | Giải thích                  |
| ---------- | --------------------------- |
| Doubao TTS | API trên cloud              |
| EdgeTTS    | TTS miễn phí của Microsoft  |
| CosyVoice  | Chất lượng cao, chạy cục bộ |

---

## II. Kiểm thử cấu hình

### Kiểm thử một cấu hình riêng lẻ

Ở từng trang cấu hình, bấm nút "Kiểm thử" bên cạnh mục cấu hình:

<!-- Vị trí ảnh chụp màn hình: Nút kiểm thử cấu hình đơn -->

> Hình: Nút kiểm thử cấu hình

Giải thích kết quả kiểm thử:

| Trường                  | Giải thích                             |
| ----------------------- | -------------------------------------- |
| Trạng thái              | Thành công/Thất bại                    |
| Độ trễ gói tin đầu tiên | Thời gian phản hồi tính theo mili-giây |
| Thông báo               | Chi tiết lỗi (nếu thất bại)            |

<!-- Vị trí ảnh chụp màn hình: Popup kết quả kiểm thử -->

> Hình: Popup kết quả kiểm thử cấu hình

### Kiểm thử hàng loạt

Trong trang quản lý cấu hình, bấm "Kiểm thử tất cả" để kiểm thử hàng loạt toàn bộ cấu hình:

<!-- Vị trí ảnh chụp màn hình: Giao diện kiểm thử hàng loạt -->

> Hình: Giao diện kiểm thử hàng loạt

### Các loại kiểm thử được hỗ trợ

| Loại kiểm thử | Giải thích                                                               |
| ------------- | ------------------------------------------------------------------------ |
| VAD           | Khả năng kết nối và thời gian phản hồi của phát hiện hoạt động giọng nói |
| ASR           | Khả năng kết nối và độ trễ gói tin đầu tiên của nhận dạng giọng nói      |
| LLM           | Khả năng kết nối và độ trễ gói tin đầu tiên của suy luận mô hình lớn     |
| TTS           | Khả năng kết nối và độ trễ gói tin đầu tiên của tổng hợp giọng nói       |
| OTA           | Kiểm thử khả năng kết nối MQTT/UDP                                       |

---

## III. Giám sát độ trễ

Xem thống kê độ trễ gói tin đầu tiên của từng module trong hệ thống:

<!-- Vị trí ảnh chụp màn hình: Giao diện giám sát độ trễ -->

> Hình: Giao diện giám sát độ trễ

### Đề xuất tối ưu độ trễ

| Module | Hướng tối ưu                                       |
| ------ | -------------------------------------------------- |
| ASR    | Dùng model cục bộ hoặc node API ở gần              |
| LLM    | Chọn model nhỏ hơn hoặc dùng output dạng streaming |
| TTS    | Dùng TTS biên (edge) hoặc model cục bộ             |

---

## IV. Quản lý cấu hình

### Sửa cấu hình

Vào "Quản lý cấu hình" → module tương ứng → sửa mục cấu hình

<!-- Vị trí ảnh chụp màn hình: Giao diện quản lý cấu hình -->

> Hình: Giao diện quản lý cấu hình

### Bật/tắt cấu hình

Dùng công tắc để điều khiển cấu hình có hiệu lực hay không.

### Đặt cấu hình mặc định

Mỗi module có thể đặt một cấu hình mặc định, khi thiết bị không chỉ định cụ thể sẽ dùng cấu hình mặc định.

---

## Các câu hỏi thường gặp

### Q1: Kiểm thử cấu hình thất bại?

1. Kiểm tra kết nối mạng
2. Xác minh API key có chính xác không
3. Xem log console của chương trình chính

### Q2: Làm thế nào để khôi phục cấu hình mặc định?

Xóa file cấu hình trong thư mục `config/`, khởi động lại dịch vụ.

### Q3: Sau khi sửa cấu hình có cần khởi động lại không?

Đa số các cấu hình sau khi sửa sẽ có hiệu lực ngay lập tức (real-time); một số cấu hình module có thể cần khởi động lại kết nối của thiết bị.
