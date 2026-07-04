# Phương án cải tổ tạo lười (lazy creation) và tái sử dụng dài hạn cho ChatSession

## Mục tiêu

- Chuyển `ChatSession` từ mô hình "tạo ngay khi kết nối được thiết lập" sang "tạo lười (lazy) sau khi `hello` thành công, và tái sử dụng lâu dài trong suốt vòng đời kết nối".
- `ChatSession.Close()` chỉ giải phóng tài nguyên thuộc phạm vi chat, không còn đóng `serverTransport`, cũng không còn đóng IoT-over-MCP phía thiết bị.
- Tách `hello` thành hai giai đoạn theo trách nhiệm:
  - Cấp độ transport: trả về thông tin bắt tay như `transport/udp(server, port, key, nonce)`.
  - Cấp độ chat: ghi `audio_params`, khởi tạo hoặc tái sử dụng `SessionID`, làm mới cấu hình thiết bị, kích hoạt tạo session.
- Tách `mcp/iot/goodbye` ra khỏi luồng chính vận hành của `ChatSession`, giao cho `ChatManager` xử lý.

## Ranh giới thiết kế

### ChatManager

- Là chủ sở hữu (owner) ở cấp độ kết nối.
- Nắm giữ `transport`, `serverTransport`, `clientState`, `mcpTransport`, `hookHub`, `transformRegistry`.
- Chịu trách nhiệm khởi động và nắm giữ vòng lặp lệnh (command loop), vòng lặp âm thanh (audio loop).
- Chịu trách nhiệm xử lý:
  - `hello`
  - `mcp`
  - `iot`
  - `goodbye`
- Thực hiện định tuyến (routing) cho `listen/abort`, khi cần thiết sẽ gọi `ensureSession()`.

### ChatSession

- Chỉ chịu trách nhiệm cho phạm vi chat:
  - `listen`
  - `abort`
  - ASR/VAD
  - LLM/TTS
  - Phát media ở cấp độ session
- `Start()` không còn khởi động `CmdMessageLoop/AudioMessageLoop` ở cấp độ kết nối.
- `Start()` sẽ khởi động, với điều kiện tiền đề là định dạng âm thanh đầu vào đã sẵn sàng:
  - Vòng lặp nền VAD/ASR
  - `processChatText`
  - `llmManager.Start`
  - `ttsManager.Start`

## Quy ước về vòng đời

- `hello` lần đầu tiên:
  - Ghi `clientState.InputAudioFormat`
  - Tạo `SessionID`
  - Có thể tùy chọn khởi tạo MCP phía thiết bị
  - Gọi `ensureSession()`
  - Phản hồi `hello`
- `hello` lặp lại (các lần sau):
  - Cập nhật `audio_params`
  - Làm mới cấu hình thiết bị
  - Nếu hiện tại không có `ChatSession` nào đang hoạt động, gọi lại `ensureSession()`
  - Có thể tùy chọn kích hoạt lại việc khởi tạo MCP phía thiết bị
- `mqtt_udp`:
  - Khi hội thoại kết thúc bình thường, không đóng transport
  - Chỉ khi thoát tường minh (explicit exit) hoặc gặp lỗi nghiêm trọng (fatal error) mới hủy `ChatSession`
  - Sau đó vẫn có thể tiếp tục tái sử dụng kết nối và tạo lại `ChatSession`
- `websocket`:
  - Sau khi thoát tường minh hoặc gặp lỗi nghiêm trọng, `ChatManager` sẽ đóng transport sau khi việc dọn dẹp session hoàn tất

## Các điểm thay đổi trong code

- `internal/app/server/chat/chat.go`
  - `ChatManager` nắm giữ tài nguyên cấp kết nối và định tuyến tin nhắn
  - Bổ sung `ensureSession()`, `HandleHelloMessage()`, vòng lặp `cmd/audio` ở cấp độ kết nối
- `internal/app/server/chat/session.go`
  - `Start()` chỉ giữ lại các tác vụ nền thuộc phạm vi chat
  - `Close()` được đổi thành chỉ giải phóng tài nguyên chat thuần túy
  - Bổ sung callback đóng session, phục vụ cho việc `ChatManager` xử lý khác biệt theo từng giao thức
- `internal/app/server/chat/server_transport.go`
  - Bổ sung "đường dẫn đóng không đóng transport tầng dưới", dùng cho tình huống đầu xa (remote) đã ngắt kết nối
- `internal/app/server/event_handle.go`
  - Sự kiện thoát chat được chuyển sang do `ChatManager` thực thi, thay vì lấy trực tiếp `ChatSession`

## Các điểm cần xác minh

- Sau khi kết nối mới được thiết lập, sẽ không tạo `ChatSession` ngay lập tức.
- Sau `hello` lần đầu, `ChatSession` được tạo và có thể tiếp tục `listen/start`.
- Trong `mqtt_udp`, sau khi `ChatSession.Close()`, transport vẫn có thể tiếp tục gửi/nhận lệnh.
- Trong `websocket`, sau khi thoát tường minh, kết nối sẽ bị đóng.
- Việc tìm kiếm công cụ MCP tiếp tục duy trì tính "nhận biết theo transport" (transport-aware), không rơi về trạng thái không có transport.
