# Hướng dẫn tính năng nhân bản giọng nói (Voice Clone)

Tài liệu này giới thiệu tính năng **Nhân bản giọng nói (Voice Clone)** trong dự án, bao gồm quy trình tạo/nghe thử/thử lại dành cho người dùng thông thường, cũng như quản lý hạn mức nhân bản dành cho quản trị viên.

Các trang và tài liệu liên quan:

- Quản trị viên `Quản lý cấu hình TTS` (cung cấp cấu hình TTS khả dụng cho người dùng)
- Quản trị viên `Quản lý người dùng -> Hạn mức nhân bản`
- Người dùng thông thường `Nhân bản giọng nói`
- [Hướng dẫn sử dụng trang quản trị](./manager_console_guide.md)

---

## 1. Tổng quan tính năng

Tính năng nhân bản giọng nói cho phép người dùng tải lên âm thanh (hoặc ghi âm trực tiếp trên trình duyệt), tạo "giọng đọc nhân bản" trên các nhà cung cấp TTS được hỗ trợ, sau đó chọn giọng đọc này khi phát trong agent/nhân vật.

Các nhà cung cấp dịch vụ nhân bản hiện đã được hỗ trợ ở cả frontend và backend:

- `minimax`
- `cosyvoice`
- `aliyun_qwen` (Qwen của Alibaba)

Các nhà cung cấp TTS không nằm trong danh sách trên, dù có thể dùng để tổng hợp TTS thông thường, cũng không thể dùng cho nhân bản giọng nói.

---

## 2. Vai trò và quyền hạn

### 2.1 Người dùng thông thường

Có thể:

- Tạo giọng đọc nhân bản
- Xem trạng thái tác vụ nhân bản
- Nghe thử âm thanh gốc và âm thanh nhân bản
- Chỉnh sửa tên bản nhân bản
- Thử lại đối với tác vụ thất bại

### 2.2 Quản trị viên

Có thể:

- Cấu hình và bật các nhà cung cấp TTS hỗ trợ nhân bản
- Thiết lập hạn mức nhân bản cho từng người dùng theo từng `Cấu hình TTS` (tuỳ chọn)

---

## 3. Điều kiện tiên quyết

Trước khi sử dụng, vui lòng xác nhận:

1. Quản trị viên đã tạo và bật ít nhất một cấu hình TTS (provider là `minimax` / `cosyvoice` / `aliyun_qwen`)
2. Người dùng thông thường có thể thấy cấu hình TTS đó trên trang "Nhân bản giọng nói"
3. (Tuỳ chọn) Quản trị viên đã cấp hạn mức nhân bản cho người dùng đó

Lưu ý:

- Nếu chưa cấu hình hạn mức, mặc định sẽ tương thích với hành vi cũ, thường được coi là "không giới hạn"

---

## 4. Quy trình sử dụng cho người dùng thông thường

Lối vào:

- `Người dùng thông thường -> Nhân bản giọng nói`

## 4.1 Tạo giọng đọc nhân bản

Bấm `Tạo giọng đọc nhân bản`, điền các thông tin:

- `Tên bản nhân bản` (tuỳ chọn, nếu không điền sẽ dùng tên file)
- `Cấu hình TTS` (bắt buộc chọn cấu hình hỗ trợ nhân bản)
- `Nguồn âm thanh` (tải file lên / ghi âm trên trình duyệt)
- `Văn bản tương ứng với âm thanh` (bắt buộc điền hay không tùy vào khả năng của provider)
- `Ngôn ngữ văn bản` (ví dụ `zh-CN` / `en-US`)

Sau khi gửi có thể xảy ra hai kết quả:

- Thành công ngay lập tức (hiếm gặp)
- Trả về "Đã gửi tác vụ nhân bản, đang xử lý ở nền" (thường gặp, xử lý bất đồng bộ)

## 4.2 Xem trạng thái tác vụ

Danh sách sẽ hiển thị:

- Nhà cung cấp (provider)
- Cấu hình TTS liên quan
- ID giọng đọc nhân bản
- Trạng thái tác vụ
- Lý do thất bại (nếu có)
- Thời gian tạo

Các trạng thái thường gặp có thể hiểu như sau:

- Đang chờ xử lý / Đang xử lý
- Đã hoàn thành (có thể nghe thử)
- Thất bại (có thể xem lý do thất bại và thử lại)

## 4.3 Nghe thử và quản lý

Mỗi bản ghi nhân bản hỗ trợ các thao tác sau:

- `Âm thanh gốc`: Phát âm thanh mẫu do người dùng đã gửi
- `Nghe thử bản nhân bản`: Phát giọng đọc nhân bản do provider trả về (chỉ hiển thị khi trạng thái thành công)
- `Chỉnh sửa`: Sửa tên bản nhân bản
- `Nhân bản lại`: Gửi lại tác vụ đã thất bại (chỉ hiển thị khi trạng thái thất bại)

---

## 5. Sự khác biệt giữa các Provider và lưu ý

## 5.1 Minimax

Frontend và backend sẽ kiểm tra ràng buộc đối với âm thanh, các quy tắc thường gặp:

- Định dạng âm thanh thường yêu cầu `WAV`
- Thời lượng âm thanh nên/phải không dưới `10 giây`

Trang sẽ hiển thị thông báo ở khu vực tải lên/ghi âm, và chặn việc gửi nếu thời lượng không đủ.

## 5.2 CosyVoice

Đặc điểm:

- Hỗ trợ nhân bản
- Trong các trường hợp thường gặp, yêu cầu điền "văn bản tương ứng với âm thanh" (do interface khả năng của provider trả về)

Việc có bắt buộc điền hay không sẽ tùy theo thông báo về khả năng của provider hiện tại trên trang.

## 5.3 Qwen (`aliyun_qwen`)

Đặc điểm:

- Hỗ trợ nhân bản
- Hỗ trợ nhiều định dạng âm thanh hơn (như `WAV/MP3/M4A`, tùy theo thông báo trên trang)
- Sau khi chọn giọng đọc nhân bản loại này, khi chạy hệ thống sẽ tự động chuyển sang model nhân bản tương ứng (frontend sẽ hiển thị thông báo)

---

## 6. Quản lý hạn mức nhân bản (dành cho quản trị viên)

Lối vào:

- `Quản trị viên -> Quản lý người dùng -> Hạn mức nhân bản`

Quản trị viên có thể cấu hình hạn mức nhân bản cho một người dùng thông thường theo từng `ID cấu hình TTS`:

- `-1`: Không giới hạn số lần
- `0`: Cấm tạo
- `Số nguyên dương`: Số lần nhân bản tối đa

Việc thống kê hạn mức thường được tính theo "số lần gửi tác vụ nhân bản" (việc thử lại khi thất bại cũng nên được tính vào, vui lòng áp dụng theo quy tắc nghiệp vụ hiện hành).

---

## 7. Mô tả interface (phía người dùng)

### 7.1 Dò khả năng (capability probing)

- `GET /user/voice-clone/capabilities?provider=<provider>`

Công dụng:

- Lấy thông tin provider có được bật hay không
- Có yêu cầu điền transcript hay không
- Phạm vi độ dài văn bản
- Danh sách ngôn ngữ được hỗ trợ

### 7.2 Bản ghi nhân bản và các thao tác với tác vụ

- `POST /user/voice-clones` (tạo bản nhân bản, `multipart/form-data`)
- `GET /user/voice-clones` (danh sách)
- `PUT /user/voice-clones/:id` (sửa tên)
- `POST /user/voice-clones/:id/retry` (thử lại khi thất bại)
- `GET /user/voice-clones/:id/preview` (nghe thử giọng đọc nhân bản)

### 7.3 Quản lý âm thanh gốc

- `GET /user/voice-clones/:id/audios`
- `GET /user/voice-clones/audios/:audio_id/file`

---

## 8. Mô tả interface (hạn mức của quản trị viên)

- `GET /admin/users/:id/voice-clone-quotas`
- `PUT /admin/users/:id/voice-clone-quotas`

---

## 9. Câu hỏi thường gặp và cách xử lý

### 9.1 Trang không hiển thị cấu hình TTS để chọn

Kiểm tra:

1. Quản trị viên đã bật cấu hình TTS chưa
2. Provider của TTS có thuộc danh sách hỗ trợ nhân bản không (`minimax/cosyvoice/aliyun_qwen`)
3. Người dùng hiện tại có quyền truy cập cấu hình đó không

### 9.2 Khi gửi báo lỗi "nhà cung cấp này yêu cầu điền văn bản tương ứng với âm thanh"

Điều này có nghĩa là provider yêu cầu bắt buộc phải điền transcript, vui lòng bổ sung văn bản tương ứng với âm thanh rồi gửi lại.

### 9.3 Khi gửi báo lỗi hạn mức không đủ

Quản trị viên cần vào `Quản lý người dùng -> Hạn mức nhân bản` để tăng hạn mức hoặc đặt thành `-1` cho người dùng đó ứng với `ID cấu hình TTS` tương ứng.

### 9.4 Nhân bản thành công nhưng không nghe thử được

Kiểm tra:

1. Trạng thái tác vụ đã hoàn thành chưa
2. Interface preview của provider có hoạt động bình thường không
3. Trình duyệt có chặn tự động phát âm thanh không (thử bấm nút phát thủ công lại)

---

## 10. Đề xuất khi sử dụng

- Chuẩn bị cấu hình TTS riêng cho từng kịch bản (thuận tiện cho việc kiểm soát hạn mức và tính phí theo từng nguồn)
- Âm thanh gửi lên nên sử dụng giọng nói sạch, môi trường ít nhiễu
- Transcript nên khớp với nội dung âm thanh càng nhiều càng tốt, giúp cải thiện hiệu quả và độ ổn định của bản nhân bản
