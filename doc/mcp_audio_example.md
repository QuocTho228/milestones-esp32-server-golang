# Hướng dẫn sử dụng MCP Audio Server (Repository độc lập)

## Tổng quan

MCP Audio Server đã được tách thành một repository độc lập, khuyến nghị sử dụng trực tiếp dự án độc lập này để chạy, debug và phát triển thêm.

Tên dự án độc lập:

- `mcp_audio_server`
- `github.com/quoctho228/mcp_audio_server`

Mục tiêu cốt lõi nhằm minh họa:

- Cách công cụ `musicPlayer` trả về `ResourceLink`
- Cách đọc dữ liệu âm thanh theo trang (phân trang) thông qua `resource/read`
- Cách sử dụng `BlobResourceContents` để trả về đoạn dữ liệu âm thanh được mã hóa base64

Repository độc lập này vừa có thể chạy trực tiếp, vừa phù hợp làm template để tích hợp.

## Cách sử dụng được khuyến nghị

Khuyến nghị sử dụng MCP Audio Server trong repository độc lập.

Khuyến nghị clone repository độc lập trước, sau đó vào thư mục dự án:

```bash
git clone https://github.com/quoctho228/mcp_audio_server.git
cd mcp_audio_server
```

## Các năng lực mà dịch vụ cung cấp

Hiện tại dịch vụ chỉ cung cấp hai loại năng lực:

1. Công cụ (tool) `musicPlayer`
2. Tài nguyên (resource) `resource://read_from_http`

### `musicPlayer`

- Chức năng: Tìm kiếm nhạc theo tên bài hát người dùng nhập và trả về tài nguyên có thể phát
- Tham số đầu vào: `query`
- Giá trị trả về: `ResourceLink`

Ý nghĩa các trường quan trọng trong `ResourceLink` trả về như sau:

- `URI`: `resource://read_from_http`
- `Name`: tên bài hát thực tế
- `Description`: URL âm thanh thực tế
- `MIMEType`: `audio/mpeg`

### `resource://read_from_http`

- Chức năng: Đọc dữ liệu âm thanh từ xa theo trang (phân trang)
- Cách gọi: thông qua `resource/read`
- Tham số được truyền qua `Arguments`

Định dạng tham số yêu cầu:

```json
{
  "url": "URL âm thanh thực tế",
  "start": 0,
  "end": 102400
}
```

Giải thích tham số:

- `url`: địa chỉ âm thanh thực tế, lấy từ `ResourceLink.Description`
- `start`: vị trí byte bắt đầu
- `end`: vị trí byte kết thúc, không bao gồm vị trí này

Nội dung trả về là `BlobResourceContents`:

- `MIMEType`: `audio/mpeg`
- `Blob`: dữ liệu nhị phân âm thanh đã được mã hóa base64

Khi đọc hết dữ liệu, server sẽ trả về `[DONE]` (đã mã hóa base64) làm cờ đánh dấu kết thúc.

## Quy trình gọi

Quy trình đầy đủ như sau:

1. Client gọi `musicPlayer`
2. Công cụ tìm kiếm bài hát và trả về `ResourceLink`
3. Client gửi yêu cầu `resource/read` tới `resource://read_from_http`
4. Mỗi lần truyền `url`, `start`, `end` qua `Arguments`
5. Server trả về `BlobResourceContents` đã mã hóa base64
6. Client giải mã và phát liên tục dưới dạng luồng âm thanh, cho đến khi nhận được `[DONE]`

## Cách chạy

Repository độc lập hỗ trợ hai phương thức truyền tải (transport):

- Mặc định: `stdio`
- Tùy chọn: HTTP Streamable MCP

### Chế độ stdio

Khởi động trực tiếp:

```bash
git clone https://github.com/quoctho228/mcp_audio_server.git
cd mcp_audio_server
go run .
```

### Chế độ HTTP

Chỉ định rõ transport HTTP:

```bash
cd mcp_audio_server
go run . -t http
```

Hoặc:

```bash
cd mcp_audio_server
go run . --transport http
```

Thông tin lắng nghe ở chế độ HTTP:

- Port: `3001`
- Đường dẫn: `/mcp`
- Địa chỉ đầy đủ: `http://localhost:3001/mcp`

## Lưu ý khi sử dụng hiện tại

Repository độc lập có thể build và chạy trực tiếp, trước khi sử dụng nên lưu ý một số điểm sau:

- Việc tìm kiếm bài hát và lấy URL thực tế phụ thuộc vào `github.com/scroot/music-sd/pkg/netease` và `github.com/scroot/music-sd/pkg/qq`
- Kết quả tìm kiếm nhạc và độ ổn định của link phát phụ thuộc vào năng lực của trang web bên ngoài
- Nếu chuyển dự án độc lập này sang dự án khác, thường cần bổ sung đồng bộ các dependency và logic tìm kiếm nêu trên

Nếu mục tiêu của bạn là nhanh chóng tích hợp công cụ âm thanh của riêng mình, khuyến nghị nên ưu tiên tái sử dụng giao thức và luồng dữ liệu, thay vì tái sử dụng trực tiếp phần triển khai tìm kiếm nhạc.

## Các phần cần giữ nguyên khi dùng làm template

Nếu muốn cải tạo dự án độc lập này thành MCP Server âm thanh của riêng bạn, nên giữ lại các quy ước giao thức sau:

- Công cụ trả về `ResourceLink`
- `resource/read` sử dụng `Arguments` để đọc phân trang
- Dữ liệu âm thanh được trả về qua `BlobResourceContents.Blob`
- Nội dung `Blob` giữ nguyên dạng mã hóa base64
- Loại MIME của âm thanh phải khớp với dữ liệu thực tế; repository độc lập hiện tại dùng `audio/mpeg`
- Khi kết thúc luồng, trả về `[DONE]`

Làm như vậy sẽ đảm bảo tương thích với logic tiêu thụ (consume) âm thanh trong dịch vụ chính hiện tại.

## Tính tương thích với dịch vụ chính hiện tại

Logic tiêu thụ công cụ MCP loại âm thanh trong dịch vụ chính hiện tại đã được xử lý theo cách sau:

- Nhận diện `ResourceLink`
- Sử dụng phương thức `Arguments` để gọi phân trang `resource/read`
- Giải mã `BlobResourceContents.Blob`
- Phân tích định dạng âm thanh theo loại MIME
- Phát liên tục cho đến khi đọc xong

Do đó, hình thái giao thức của dự án độc lập này có thể tiếp tục được dùng làm template tham khảo cho các công cụ MCP loại âm thanh.
