# Hướng dẫn gọi từ xa MCP theo chiều Thiết bị/Agent

Tài liệu này giới thiệu **năng lực debug gọi từ xa MCP** trong trang điều khiển (console) quản trị, bao gồm:

- Tạo Endpoint (điểm truy cập) MCP theo chiều Agent
- Lấy danh sách công cụ và gọi từ xa theo chiều Agent
- Lấy danh sách công cụ và gọi từ xa theo chiều Thiết bị
- Sự khác biệt về quyền giữa quản trị viên và người dùng thông thường

Tài liệu liên quan:

- [Giải thích kiến trúc MCP](./mcp.md)
- [Hướng dẫn chức năng Chợ MCP](./mcp_market.md)
- [Hướng dẫn sử dụng trang quản trị](./manager_console_guide.md)

---

## 1. Định vị chức năng

Chức năng này chủ yếu dùng để "debug và kiểm chứng":

- Nhanh chóng xem agent/thiết bị hiện tại đang cung cấp những công cụ MCP nào
- Trực tiếp trong console tạo tham số và gọi công cụ
- Lấy MCP endpoint theo chiều agent, cung cấp cho MCP client bên ngoài kết nối để test

Phù hợp với các tình huống:

- Kiểm chứng xem một dịch vụ MCP từ xa nào đó đã có hiệu lực hay chưa
- Kiểm tra schema/tham số mẫu của công cụ
- Xử lý sự khác biệt về hành vi MCP giữa agent và thiết bị

---

## 2. Sự khác biệt giữa hai chiều gọi

## 2.1 Chiều Agent

Đặc điểm:

- Hướng theo góc nhìn "cấu hình agent"
- Hỗ trợ lấy MCP endpoint của agent (kèm token)
- Hỗ trợ lấy danh sách công cụ, trực tiếp thực hiện gọi công cụ
- Bị ảnh hưởng bởi cấu hình agent (như `mcp_service_names`)

Công dụng thường gặp:

- Kiểm chứng tập hợp công cụ MCP khả dụng sau khi agent đã lọc
- Copy endpoint để cung cấp cho client debug bên ngoài sử dụng

## 2.2 Chiều Thiết bị (Device)

Đặc điểm:

- Hướng theo góc nhìn "kết nối thiết bị cụ thể"
- Trực tiếp yêu cầu chi tiết công cụ/gọi công cụ thông qua ngữ cảnh kết nối hiện tại của thiết bị
- Thường phụ thuộc vào việc thiết bị có online và bộ điều khiển (controller) WebSocket có khả dụng hay không

Công dụng thường gặp:

- Xử lý tình huống "cùng một agent nhưng hiển thị công cụ khác nhau trên các thiết bị khác nhau"
- Kiểm chứng năng lực MCP ở phía phiên (session) đang online hiện tại của thiết bị

---

## 3. Lối vào trang (Quản trị viên / Người dùng thông thường)

### 3.1 Quản trị viên

- `Quản trị viên -> Quản lý Agent` (endpoint/tools/call theo chiều Agent)
- `Quản trị viên -> Quản lý thiết bị` (tools/call theo chiều Thiết bị)

### 3.2 Người dùng thông thường

- `Agent của tôi` (tools/call theo chiều Agent)
- `Thiết bị của tôi` / `Thiết bị của Agent` (tools/call theo chiều Thiết bị)
- `Chỉnh sửa Agent` (cấu hình `mcp_service_names`, ảnh hưởng đến phạm vi dịch vụ khả kiến theo chiều Agent)

---

## 4. Chiều Agent: Quy trình debug đầy đủ

## 4.1 Cấu hình dịch vụ MCP khả dụng cho Agent (tùy chọn nhưng khuyến nghị)

Tại trang chỉnh sửa Agent có thể thiết lập `mcp_service_names` (danh sách tên dịch vụ, phân cách bằng dấu phẩy):

- Để trống: sử dụng toàn bộ dịch vụ MCP toàn cục đang được bật
- Điền vào: chỉ sử dụng các dịch vụ được chỉ định theo tên (phải là dịch vụ đã tồn tại và đang được bật trong hệ thống)

Hệ thống sẽ thực hiện các bước sau đối với trường này:

- Loại bỏ trùng lặp
- Loại bỏ khoảng trắng
- Kiểm tra tính hợp lệ (tên dịch vụ phải tồn tại trong tập hợp dịch vụ toàn cục đang được bật)

## 4.2 Lấy MCP Endpoint của Agent

Console có thể lấy URL điểm truy cập MCP riêng của agent, có định dạng tương tự:

```text
ws(s)://<host>/mcp?token=<jwt>
```

Giải thích:

- Endpoint được suy ra dựa trên `external.websocket.url` trong cấu hình OTA mặc định để xác định domain và giao thức
- Token chứa ngữ cảnh người dùng và agent hiện tại (dùng cho kiểm tra quyền/mục đích liên kết)
- Phù hợp để MCP client bên ngoài debug tạm thời, không khuyến nghị chia sẻ công khai

## 4.3 Lấy danh sách công cụ

Console sẽ yêu cầu chi tiết công cụ MCP theo chiều agent, nội dung trả về thường bao gồm:

- `name`
- Mô tả công cụ
- Schema tham số
- Tham số mẫu (nếu phía thiết bị/server cung cấp)

Nếu không lấy được (ví dụ controller chưa khởi tạo hoặc client tạm thời không thể truy cập), backend sẽ trả về danh sách rỗng thay vì báo lỗi, để trang có thể tiếp tục thao tác.

## 4.4 Gọi công cụ trực tiếp

Trong console, điền:

- `tool_name`
- `arguments` (JSON)

Sau khi gọi, có thể xem toàn bộ nội dung trả về (định dạng JSON) trong khung kết quả.

---

## 5. Chiều Thiết bị: Quy trình debug đầy đủ

## 5.1 Lấy danh sách công cụ của thiết bị

Sau khi chọn thiết bị, console sẽ sử dụng định danh thiết bị (nội bộ sẽ ánh xạ sang tên thiết bị) để yêu cầu bộ điều khiển WebSocket cung cấp chi tiết công cụ MCP.

Các trường hợp thất bại thường gặp:

- Thiết bị không online
- Thiết bị không thuộc về người dùng hiện tại (góc nhìn người dùng)
- Bộ điều khiển WebSocket tạm thời không khả dụng

Trong các trường hợp này, giao diện thường trả về danh sách công cụ rỗng hoặc lỗi về quyền.

## 5.2 Gọi công cụ MCP của thiết bị

Tương tự chiều agent, điền:

- `tool_name`
- `arguments` (JSON)

Điểm khác biệt là ngữ cảnh gọi sử dụng `device_id` (thực tế backend sẽ truyền tên thiết bị), do đó gần với "môi trường thực thi thực tế theo phiên thiết bị hiện tại" hơn.

---

## 6. Sự khác biệt về quyền và giao diện (Quản trị viên vs Người dùng thông thường)

### 6.1 Giao diện cho người dùng thông thường

Theo chiều Agent:

- `GET /user/agents/:id/mcp-endpoint`
- `GET /user/agents/:id/mcp-tools`
- `POST /user/agents/:id/mcp-call`

Theo chiều Thiết bị:

- `GET /user/devices/:id/mcp-tools`
- `POST /user/devices/:id/mcp-call`

Hỗ trợ lọc dịch vụ MCP của Agent:

- `GET /user/agents/:id/mcp-services/options`

Người dùng thông thường chỉ có thể thao tác với agent/thiết bị thuộc về mình.

### 6.2 Giao diện cho quản trị viên

Theo chiều Agent:

- `GET /admin/agents/:id/mcp-endpoint`
- `GET /admin/agents/:id/mcp-tools`
- `POST /admin/agents/:id/mcp-call`

Theo chiều Thiết bị:

- `GET /admin/devices/:id/mcp-tools`
- `POST /admin/devices/:id/mcp-call`

Quản trị viên có thể debug bất kỳ agent/thiết bị nào của bất kỳ người dùng nào (với điều kiện bản ghi tồn tại và chuỗi liên kết kết nối bình thường).

---

## 7. Logic tạo Endpoint (theo chiều Agent)

Việc tạo endpoint của agent phụ thuộc vào:

1. Cấu hình OTA mặc định (`type=ota` và `is_default=true`)
2. Trường `external.websocket.url` trong cấu hình OTA
3. Token ổn định được tạo dựa trên User ID + Agent ID hiện tại

Kết quả tạo ra sẽ sử dụng:

- Cùng giao thức (`ws` / `wss`)
- Cùng host (domain/IP + port)
- Đường dẫn cố định `/mcp`

Do đó, nếu không thể tạo endpoint, hãy ưu tiên kiểm tra cấu hình WebSocket ra bên ngoài (external) của OTA.

---

## 8. Câu hỏi thường gặp và xử lý sự cố

### 8.1 Danh sách công cụ trống

Các nguyên nhân có thể:

- Thiết bị không online (chiều Thiết bị)
- Bộ điều khiển WebSocket chưa khởi tạo
- Client không trả về chi tiết công cụ
- Không còn dịch vụ khả dụng sau khi bị `mcp_service_names` lọc (chiều Agent)

Thứ tự xử lý sự cố được khuyến nghị:

1. Xác nhận trạng thái online của thiết bị
2. Kiểm tra dịch vụ MCP toàn cục có được bật hay không
3. Kiểm tra cấu hình `mcp_service_names` của agent
4. Thử lấy lại danh sách công cụ trong console

### 8.2 Khi gọi báo lỗi tham số JSON

Khu vực tham số trong console yêu cầu JSON hợp lệ dạng object, ví dụ:

```json
{
  "query": "hello"
}
```

Các lỗi thường gặp:

- Dùng dấu nháy đơn
- Có dấu phẩy thừa ở cuối
- Cấp cao nhất không phải là object

### 8.3 Lấy endpoint của Agent thất bại

Thường là do thiếu cấu hình OTA mặc định hoặc chưa cấu hình `external.websocket.url`.

### 8.4 Đã nhập dịch vụ MCP rõ ràng, nhưng agent gọi lại không thấy

Kiểm tra:

1. Dịch vụ đã nhập có được bật hay không
2. Công tắc tổng cấu hình MCP toàn cục và trạng thái bật của dịch vụ
3. Agent có loại trừ dịch vụ đó thông qua `mcp_service_names` hay không

---

## 9. Thực hành tốt nhất (Best Practices)

- Trước tiên hãy kiểm chứng công cụ khả dụng theo "chiều Thiết bị" ở phía quản trị viên, sau đó mới kiểm chứng kết quả lọc công cụ theo "chiều Agent"
- Đối với agent trong môi trường production, khuyến nghị cấu hình rõ ràng `mcp_service_names`, tránh việc để lộ các công cụ không liên quan cho mô hình
- Coi endpoint là điểm debug nhạy cảm, tránh lan truyền URL có kèm token trên các kênh công khai
