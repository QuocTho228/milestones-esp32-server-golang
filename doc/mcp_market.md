# Hướng dẫn chức năng Chợ MCP (MCP Market)

Tài liệu này giới thiệu chức năng **Chợ MCP (MCP Market)** trong trang quản trị: cách tích hợp chợ MCP bên thứ ba, tổng hợp dịch vụ khám phá (discovery), nhập (import) cấu hình dịch vụ và đưa vào danh sách dịch vụ MCP toàn cục của hệ thống.

Tài liệu liên quan:

- [Giải thích kiến trúc MCP](./mcp.md)
- [Hướng dẫn sử dụng trang quản trị](./manager_console_guide.md)

---

## 1. Định vị chức năng

Chợ MCP dùng để giải quyết vấn đề "hiệu quả tích hợp dịch vụ MCP từ xa còn thấp", hỗ trợ:

- Cấu hình nhiều kết nối chợ MCP (ví dụ: ModelScope...)
- Tổng hợp danh mục dịch vụ từ nhiều chợ
- Xem chi tiết dịch vụ (endpoint, giao thức truyền tải...)
- Nhập cấu hình dịch vụ vào hệ thống chỉ với một thao tác
- Bật/tắt/sửa/xóa các dịch vụ đã nhập

Dịch vụ sau khi nhập sẽ tham gia vào việc hợp nhất cấu hình dịch vụ MCP toàn cục của hệ thống (cùng có hiệu lực với các dịch vụ MCP được cấu hình thủ công).

---

## 2. Phân quyền vai trò và lối vào

Phân quyền vai trò:

- Chỉ quản trị viên (admin) mới có thể thao tác

Lối vào trang quản trị:

- `Quản trị viên -> Chợ MCP`

Trang gồm hai tab:

- `Khám phá chợ`
- `Dịch vụ đã nhập`

---

## 3. Khái niệm cốt lõi

### 3.1 Chợ MCP (Market)

Đại diện cho một "nguồn danh mục chợ MCP có thể truy cập", bao gồm:

- Tên chợ
- Định danh nhà cung cấp (provider)
- URL danh mục (catalog_url)
- Mẫu URL chi tiết (detail_url_template, tùy chọn)
- Token xác thực (tùy chọn)
- Trạng thái bật/tắt

### 3.2 Danh sách dịch vụ tổng hợp

Hệ thống sẽ lấy danh mục dịch vụ từ các chợ đã bật và hiển thị tổng hợp, hỗ trợ:

- Tìm kiếm theo tên dịch vụ/mô tả/Service ID
- Xem chi tiết
- Nhập cấu hình

Khi một số chợ lấy dữ liệu thất bại, trang sẽ hiển thị danh sách cảnh báo "một số chợ lấy dữ liệu thất bại", không ảnh hưởng đến việc hiển thị kết quả của các chợ khác.

### 3.3 Dịch vụ đã nhập

Sau khi nhập, dịch vụ sẽ hình thành một mục cấu hình độc lập trong hệ thống, có thể trực tiếp tham gia vào kết nối dịch vụ MCP khi vận hành (runtime). Hỗ trợ cấu hình:

- Tên
- Loại truyền tải (`sse` / `streamablehttp`)
- URL
- Headers (JSON)
- Chợ nguồn và định danh provider (metadata, tùy chọn)
- Trạng thái bật/tắt

---

## 4. Quy trình thao tác thường dùng (Quản trị viên)

## 4.1 Thêm mới kết nối Chợ MCP

Tại tab `Khám phá chợ`, nhấn `Thêm kết nối mới`, điền:

- `Nhà cung cấp`: ưu tiên chọn preset provider có sẵn (sẽ tự động điền mẫu URL danh mục)
- `Tên`
- `URL danh mục`
- `Mẫu URL chi tiết` (tùy chọn)
- `Bật`
- `Token` (nếu chợ yêu cầu)

Khuyến nghị thực hiện kiểm tra kết nối (xem bên dưới) trước khi lưu sử dụng.

## 4.2 Kiểm tra kết nối chợ

Trong menu thao tác của danh sách chợ, nhấn `Kiểm tra`:

- Thành công sẽ trả về "số lượng dịch vụ có thể khám phá"
- Thất bại sẽ báo lỗi kết nối danh mục/xác thực

Phù hợp để xử lý các trường hợp:

- Token không hợp lệ
- URL danh mục sai
- Chợ tạm thời không khả dụng

## 4.3 Duyệt và tìm kiếm dịch vụ tổng hợp

Tại khu vực `Danh sách dịch vụ tổng hợp` có thể:

- Nhập từ khóa để tìm dịch vụ
- Xem kết quả tổng hợp theo trang
- Nhấn `Chi tiết` để xem thông tin endpoint của dịch vụ

Trang chi tiết dịch vụ thường bao gồm:

- Tên dịch vụ
- Chợ nguồn
- Service ID
- Mô tả
- Danh sách endpoint (giao thức truyền tải + URL)

## 4.4 Nhập cấu hình dịch vụ chỉ với một thao tác (khuyến nghị)

Trong popup chi tiết dịch vụ, nhấn `Nhập cấu hình dịch vụ và cập nhật nóng`:

- Hệ thống sẽ tạo một hoặc nhiều cấu hình dịch vụ nhập dựa trên chi tiết dịch vụ
- Sau khi nhập thành công, danh sách "Dịch vụ đã nhập" sẽ được làm mới
- Trang sẽ tự động chuyển sang tab `Dịch vụ đã nhập`

"Cập nhật nóng" nghĩa là sau khi nhập xong cấu hình, dịch vụ có thể tham gia ngay vào tập hợp dịch vụ runtime mà không cần khởi động lại backend.

## 4.5 Thêm mới/sửa dịch vụ đã nhập thủ công

Tại tab `Dịch vụ đã nhập` có thể nhấn `Thêm dịch vụ mới` để nhập thủ công, cũng có thể sửa mục đã nhập.

Giải thích các trường quan trọng:

- `Truyền tải`: hiện hỗ trợ `SSE`, `StreamableHTTP`
- `URL`: điểm vào (entry) của dịch vụ MCP từ xa
- `Headers (JSON)`: dùng để mang thông tin xác thực, ví dụ `Authorization`
- `Bật`: nếu tắt sẽ không tham gia vào tập hợp dịch vụ khả dụng khi runtime

`Headers (JSON)` bắt buộc phải là một object JSON, ví dụ:

```json
{
  "Authorization": "Bearer <token>"
}
```

---

## 5. Mối quan hệ với cấu hình MCP toàn cục

Chợ MCP không phải để thay thế trang `Cấu hình MCP`, mà là nguồn bổ sung.

Tập hợp dịch vụ MCP toàn cục khả dụng khi runtime được hợp nhất từ hai phần:

- Dịch vụ toàn cục do quản trị viên cấu hình thủ công tại trang `Cấu hình MCP`
- Dịch vụ đã nhập từ Chợ MCP và đang được bật

Do đó, cách làm được khuyến nghị là:

1. Sử dụng Chợ MCP để khám phá và nhập nhanh
2. Tại `Cấu hình MCP` / trong agent, bật và chọn dịch vụ theo nhu cầu

---

## 6. API (Giao diện backend)

Dưới đây là các API liên quan phía quản trị (yêu cầu quyền admin):

### 6.1 Quản lý kết nối chợ

- `GET /admin/mcp-markets`
- `POST /admin/mcp-markets`
- `PUT /admin/mcp-markets/:id`
- `DELETE /admin/mcp-markets/:id`
- `POST /admin/mcp-markets/:id/test`

### 6.2 Khám phá chợ và chi tiết

- `GET /admin/mcp-market/providers`
- `GET /admin/mcp-market/services`
- `GET /admin/mcp-market/services/:market_id/*service_id`
- `POST /admin/mcp-market/import`

### 6.3 Quản lý dịch vụ đã nhập

- `GET /admin/mcp-market/imported-services`
- `POST /admin/mcp-market/imported-services`
- `PUT /admin/mcp-market/imported-services/:id`
- `DELETE /admin/mcp-market/imported-services/:id`

---

## 7. Câu hỏi thường gặp và xử lý sự cố

### 7.1 Danh sách tổng hợp trống

Thứ tự kiểm tra:

1. Kiểm tra kết nối chợ có được bật hay không
2. Thực hiện "Kiểm tra" đối với chợ đó
3. Kiểm tra Token có hợp lệ hay không
4. Kiểm tra URL danh mục / mẫu URL chi tiết có đúng hay không

### 7.2 Nhập thành công nhưng khi vận hành không thấy dịch vụ

Các nguyên nhân thường gặp:

- Dịch vụ đã nhập bị tắt
- Công tắc tổng của MCP toàn cục đang tắt
- Agent đã cấu hình `mcp_service_names` nhưng không bao gồm tên dịch vụ đó

### 7.3 Khi sửa chợ, để trống Token thì sao?

Trong popup chỉnh sửa, để trống Token thường có nghĩa là "không thay đổi Token hiện tại" (giao diện sẽ hiển thị gợi ý về trạng thái Token đã được ẩn/che).

---

## 8. Đề xuất sử dụng

- Ưu tiên sử dụng preset provider có sẵn để giảm thiểu vấn đề do khác biệt trường dữ liệu giữa các giao diện danh mục
- Đặt tên thống nhất cho các dịch vụ cần sử dụng ổn định lâu dài sau khi nhập, để agent dễ dàng chọn theo tên
- Đối với dịch vụ từ xa trong môi trường production, nên cấu hình xác thực qua `Headers (JSON)`, và thực hiện tốt việc xoay vòng (rotation) token
