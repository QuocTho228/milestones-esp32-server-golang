# Phương án đánh dấu ngắt (interrupt) LLM và bổ sung nội dung lịch sử (chờ xác nhận)

## 1. Mục tiêu

Trong dự án hiện tại, cần triển khai hai việc sau:

1. Khi quá trình xử lý streaming của LLM bị ngắt giữa chừng, ghi cờ đánh dấu ngắt (interrupt) vào trường `Extra` của tin nhắn lịch sử assistant tương ứng.
2. Khi lắp ráp (assemble) lịch sử để gửi request cho LLM ở các lần sau, nếu phát hiện cờ này, sẽ nối thêm chuỗi `" [用户打断]"` ("[Người dùng đã ngắt]") vào cuối `content` của tin nhắn đó trước khi gửi cho mô hình.

Ghi chú: Phương án này chỉ mô tả đường hướng triển khai, chưa trực tiếp sửa code.

---

## 2. Các đường code hiện trạng (điểm mấu chốt)

- Nơi kích hoạt ngắt: `/Users/shijingbo/git/milestones-esp32-server-golang/internal/app/server/chat/common.go:3`
  Hàm `StopSpeaking()` sẽ hủy (cancel) `SessionCtx`, khiến ngữ cảnh xử lý LLM/TTS kết thúc.

- Xử lý streaming LLM và ghi xuống lịch sử: `/Users/shijingbo/git/milestones-esp32-server-golang/internal/app/server/chat/llm.go:323`
  Hiện tại `handleLLMResponse()` chỉ lưu tin nhắn assistant khi `llmResponse.IsEnd=true`;
  nhánh `ctx.Done()` sẽ return ngay lập tức, không lưu lại assistant "đã xuất ra nhưng bị ngắt".

- Điểm vào lắp ráp lịch sử: `/Users/shijingbo/git/milestones-esp32-server-golang/internal/app/server/chat/llm.go:1050`
  Hiện tại `GetMessages()` chỉ thêm trực tiếp `msg` lịch sử vào request, chưa xử lý bổ sung nội dung dựa trên `Extra`.

---

## 3. Nguyên tắc thiết kế

1. **Can thiệp tối thiểu**: Chỉ sửa logic lưu lịch sử và lắp ráp lịch sử trong `llm.go`.
2. **Không làm ô nhiễm lịch sử gốc**: Khi lắp ráp request, sao chép (copy) tin nhắn ra rồi mới sửa `content`, không sửa trực tiếp trên đối tượng lịch sử đang nằm trong bộ nhớ.
3. **Tránh ghi trùng lặp**: Việc ghi lịch sử khi bị ngắt chỉ thực hiện một lần, và loại trừ lẫn nhau với nhánh `IsEnd` bình thường.
4. **Tương thích ngược**: Các bản ghi lịch sử không có `Extra.interrupt` vẫn giữ nguyên hành vi cũ.

---

## 4. Chi tiết phương án

### 4.1 Ghi vào Extra khi bị ngắt (giai đoạn LLM)

Vị trí thay đổi: hàm `handleLLMResponse()` tại `/Users/shijingbo/git/milestones-esp32-server-golang/internal/app/server/chat/llm.go:324`

Logic mới bổ sung:

1. Thêm biến trạng thái cục bộ trong hàm:
   - `assistantSaved bool`: để tránh lưu trùng lặp trong cùng một lần xử lý.

2. Tách ra một helper nội bộ (closure cục bộ trong hàm hoặc một phương thức private), được thực thi khi `ctx.Done()` được kích hoạt:
   - Lấy văn bản đã tích lũy hiện tại từ `fullText.String()`;
   - Nếu văn bản rỗng, bỏ qua;
   - Khởi tạo `assistantMsg := schema.AssistantMessage(text, nil)`;
   - Thiết lập:
     - `assistantMsg.Extra["interrupt"] = true`
     - `assistantMsg.Extra["interrupt_by"] = "user"`
     - `assistantMsg.Extra["interrupt_stage"] = "llm"`
   - Gọi `AddLlmMessage(ctx, assistantMsg)` để lưu vào lịch sử.

3. Ở tất cả các điểm return trong nhánh `ctx.Done()`, gọi helper này trước khi return.

Ghi chú:

- Nhánh `IsEnd` bình thường vẫn giữ nguyên (không thêm cờ interrupt).
- Chỉ lưu khi thực sự có văn bản đã tích lũy, tránh lưu bản ghi assistant rỗng.

---

### 4.2 Bổ sung nội dung content dựa theo Extra khi lắp ráp lịch sử

Vị trí thay đổi: hàm `GetMessages()` tại `/Users/shijingbo/git/milestones-esp32-server-golang/internal/app/server/chat/llm.go:1050`

Logic mới bổ sung:

1. Khi duyệt qua các tin nhắn lịch sử, không append trực tiếp `msg` gốc, mà tạo một bản sao nông (shallow copy) trước (sao chép các trường cần thiết).
2. Nếu thỏa mãn đồng thời:
   - `msg.Role == schema.Assistant`
   - `msg.Extra != nil`
   - `msg.Extra["interrupt"] == true`
   - `msg.Content` khác rỗng

   thì sửa nội dung phía request thành:
   - `newMsg.Content = msg.Content + " [用户打断]"`

3. Để tránh nối thêm trùng lặp, nếu nội dung đã kết thúc bằng `" [用户打断]"` thì không nối thêm nữa.

Lưu ý:

- Chỉ sửa `content` trong "bản sao lắp ráp request", không sửa lịch sử gốc.

---

### 4.3 Lọc bỏ user ở cuối lịch sử (tránh làm ô nhiễm user của lượt hiện tại)

Vị trí thay đổi: hàm `GetMessages()` tại `/Users/shijingbo/git/milestones-esp32-server-golang/internal/app/server/chat/llm.go:1050`

Logic mới bổ sung:

1. Sau dòng `messageList := l.clientState.GetMessages(count)`, trước tiên kiểm tra "tin nhắn cuối cùng trong lịch sử":
   - Nếu tin nhắn cuối cùng có `Role == schema.User`, thì loại bỏ tin nhắn đó ra khỏi `messageList`.

2. Chỉ lọc "chuỗi user liên tiếp ở cuối":
   - Khuyến nghị dùng vòng lặp lùi từ cuối lên, xóa các tin nhắn `user` liên tiếp, cho tới khi phần cuối không còn là `user` hoặc danh sách rỗng.

Mục đích:

- Ngăn văn bản user còn sót lại từ lượt trước bị lẫn với `userMessage` của lượt hiện tại, gây nhiễu ý định của phiên hội thoại hiện tại.

Lưu ý:

- Đây là "lọc khi lắp ráp request", không sửa dữ liệu lịch sử gốc trong bộ nhớ.

---

## 5. Các hàm phụ trợ đề xuất bổ sung

Đề xuất đặt trong khu vực phương thức private của `llm.go`:

1. `isInterruptedMessage(msg *schema.Message) bool`
   Thống nhất việc kiểm tra `Extra.interrupt` (hỗ trợ dung sai cho cả kiểu bool/chuỗi `"true"`).

2. `decorateInterruptedContent(content string) string`
   Thống nhất logic nối thêm, tránh lặp lại `" [用户打断]"`.

3. `cloneMessageForRequest(msg *schema.Message) *schema.Message`
   Sao chép `Role/Content/Name/ToolCalls/ToolCallID/Extra/ResponseMeta` (tối thiểu phải đảm bảo `Content` và `Extra` có thể sửa an toàn).

---

## 6. Tính tương thích và rủi ro

1. `Extra` có phát huy hiệu lực trực tiếp với mô hình hay không:
   Tầng thích ứng (adapter) OpenAI hiện tại khi lắp ráp request không truyền tiếp `Extra`, vì vậy hành vi của mô hình chủ yếu do chuỗi `" [用户打断]"` mà chúng ta nối thêm vào `content` quyết định.

2. Sự khác biệt trong lưu trữ lịch sử:
   - Ở chế độ `redis`, `schema.Message` được lưu trực tiếp dưới dạng JSON vào DB, `Extra` có thể được giữ lại.
   - Ở chế độ `manager`, hiện tại chỉ lưu `role/content/tool_calls`, `Extra` có thể bị mất.
     Do đó nếu sau này cần năng lực này ở chế độ manager, cần đồng bộ mở rộng giao thức lịch sử của manager.

3. Ảnh hưởng về văn bản:
   `" [用户打断]"` là một gợi ý hiển thị tường minh, sẽ ảnh hưởng đến phong cách viết tiếp của mô hình; đây là hành vi dự kiến của yêu cầu này.

---

## 7. Tiêu chí nghiệm thu (thực hiện sau khi xác nhận)

1. Kịch bản: user đã vào lịch sử, assistant bị ngắt giữa chừng khi đang streaming
   - Trong lịch sử xuất hiện thêm một bản ghi assistant, với `Extra.interrupt=true`.

2. Xem tin nhắn request trước khi gửi LLM ở lượt tiếp theo
   - Nội dung assistant tương ứng trở thành `"<đoạn văn bản gốc> [用户打断]"`.

3. Tin nhắn hoàn thành bình thường (không bị ngắt)
   - `Extra.interrupt` không tồn tại, `content` không bị thêm tiền tố/hậu tố.

4. Khi tin nhắn cuối trong lịch sử là user
   - User ở cuối đó bị lọc bỏ trong request, không bị trùng/lẫn với user của lượt hiện tại.

5. Không xuất hiện đánh dấu trùng lặp, không xuất hiện bản ghi assistant rỗng.

---

## 8. Danh sách file cần triển khai (thực hiện sau khi xác nhận)

- `/Users/shijingbo/git/milestones-esp32-server-golang/internal/app/server/chat/llm.go`
- (Tùy chọn) `/Users/shijingbo/git/milestones-esp32-server-golang/test/interrupt_history/main.go` dùng để kiểm chứng minh họa
