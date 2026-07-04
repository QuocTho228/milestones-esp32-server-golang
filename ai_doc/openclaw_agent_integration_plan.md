# Phương án triển khai tích hợp OpenClaw Agent theo chiều Agent

## 1. Mục tiêu

Dựa trên `MILESTONES_OPENCLAW_PROTOCOL.md`, triển khai tích hợp OpenClaw trong `milestones-esp32-server-golang`, với các yêu cầu sau:

1. Không thêm trang cấu hình OpenClaw độc lập trên control panel.
2. Cách sinh OpenClaw endpoint giống với cách sinh MCP endpoint (sinh theo agent, token chứa `user_id`, `agent_id`).
3. Chương trình chính quản lý kết nối OpenClaw WebSocket theo `agent_id`.
4. Cấu hình thiết bị được đẩy xuống qua struct chứa cấu hình OpenClaw (cờ cho phép + từ khóa vào/ra).
5. Sau ASR, hỗ trợ vào/ra chế độ OpenClaw; trong chế độ này, tin nhắn bỏ qua LLM, đi thẳng qua OpenClaw, rồi tới TTS.
6. Khi phản hồi OpenClaw đến trễ và thiết bị đang offline, sẽ vào hàng đợi offline trong bộ nhớ; khi thiết bị online lần sau sẽ gửi bù.
7. Chiến lược hàng đợi offline: tối đa 20 tin nhắn/thiết bị, TTL 24 giờ.

## 2. Cải tạo control panel (manager backend)

### 2.1 Mở rộng trường của agent

Mở rộng `models.Agent`:

- `OpenclawEnabled bool`
- `OpenclawEnterKeywords string` (chuỗi JSON array)
- `OpenclawExitKeywords string` (chuỗi JSON array)

Lưu ý:

- Không thêm bảng cấu hình endpoint OpenClaw độc lập.
- Logic lấy endpoint được đồng bộ với MCP: sinh động và trả về qua interface của agent.

### 2.2 Interface Endpoint

Thêm interface mới (cho cả người dùng và trang quản trị):

- `GET /api/user/agents/:id/openclaw-endpoint`
- `GET /api/admin/agents/:id/openclaw-endpoint`

Hành vi:

1. Kiểm tra agent tồn tại và thuộc quyền hợp lệ.
2. Đọc URL WebSocket ngoại mạng (external) của OTA.
3. Sinh JWT token ổn định (có hiệu lực dài hạn, không đặt `exp`/`iat`).
4. Trả về `ws(s)://host/ws/openclaw?token=<token>`.

### 2.3 Claims của Token

Thêm các claims mới cho OpenClaw:

- `user_id`
- `agent_id`
- `endpoint_id` (`agent_<agentID>`)
- `purpose=openclaw-endpoint`

## 3. Đẩy và phân tích cấu hình thiết bị

### 3.1 Struct

Trong `UConfig` của chương trình chính, thêm mới:

```go
type OpenClawConfig struct {
    Allowed       bool     `json:"allowed"`
    EnterKeywords []string `json:"enter_keywords"`
    ExitKeywords  []string `json:"exit_keywords"`
}
```

Và thêm vào `UConfig`:

```go
OpenClaw OpenClawConfig `json:"openclaw"`
```

### 3.2 Response của /api/configs

`GetDeviceConfigs` phía quản trị thêm:

```json
"openclaw": {
  "allowed": true,
  "enter_keywords": ["进入爪子模式", "openclaw"],
  "exit_keywords": ["退出爪子模式", "退出openclaw"]
}
```

Quy tắc điền dữ liệu:

1. `allowed = agent.openclaw_enabled`
2. `enter_keywords/exit_keywords` được phân tích từ JSON array trong trường của agent
3. Nếu trường rỗng hoặc phân tích thất bại, sẽ trả về mảng rỗng

### 3.3 Lấy cấu hình phía chương trình chính

`ConfigManager.GetUserConfig` phân tích object `openclaw`, ghi vào `types.UConfig.OpenClaw`.

## 4. Dịch vụ OpenClaw WebSocket phía chương trình chính (server-side)

### 4.1 Route

Thêm route mới:

- `/ws/openclaw`

### 4.2 Chiều quản lý kết nối

Quản lý pool kết nối theo `agent_id`:

- key: `agentID`
- value: OpenClaw session (một kết nối duy nhất)

Khi có kết nối mới được thiết lập, sẽ thay thế kết nối cũ, đảm bảo mỗi agent chỉ có một kết nối OpenClaw WS đang hoạt động.

### 4.3 Xử lý giao thức

1. Sau khi kết nối thành công, gửi `handshake_ack` trước
2. Khi nhận `ping`, phản hồi `pong`
3. Khi nhận `response`:
   - Tìm route thiết bị theo `correlation_id`
   - Nếu thiết bị online thì đẩy TTS
   - Nếu thiết bị offline thì ghi vào hàng đợi offline
4. Khi nhận `error/close`, ghi log và dọn dẹp theo session

## 5. Cải tạo luồng Chat

### 5.1 Trạng thái session

`ClientState` thêm trạng thái vận hành OpenClaw:

- `OpenClawMode bool`

### 5.2 Phân luồng sau ASR

Trong `ChatSession.actionDoChat` thêm luồng xử lý:

1. Nhận diện từ khóa thoát (ưu tiên cao nhất)
2. Nhận diện từ khóa vào
3. Nếu hiện đang ở chế độ OpenClaw:
   - Gửi thẳng văn bản cho OpenClaw
   - Không gọi LLM
4. Nếu không ở chế độ OpenClaw:
   - Giữ nguyên luồng LLM cũ

Cách khớp từ khóa: chuẩn hóa văn bản trước (loại bỏ khoảng trắng đầu/cuối, các dấu câu thông thường), sau đó khớp theo kiểu `contains`.

## 6. Hàng đợi tin nhắn offline

Thêm bộ quản lý hàng đợi offline trong bộ nhớ:

- key: `deviceID`
- value: `[]OfflineMessage`
- Mỗi bản ghi gồm các trường: `Text`, `CreatedAt`, `CorrelationID`

Chiến lược:

1. Mỗi thiết bị tối đa 20 tin nhắn (khi vượt quá, loại bỏ tin cũ nhất)
2. TTL 24 giờ (dọn dẹp kép cả khi ghi và khi đọc)
3. Khi thiết bị online, tự động phát lại (replay) và xóa các tin nhắn đã gửi thành công

Điểm kích hoạt online:

- Trong `App.OnNewConnection`, sau khi thiết bị online, kích hoạt việc phát lại tin nhắn offline cho thiết bị đó

## 7. Các điểm thay đổi code chính

1. `manager/backend/models/models.go`
2. `manager/backend/controllers/admin.go`
3. `manager/backend/controllers/user.go`
4. `manager/backend/router/router.go`
5. `internal/domain/config/types/types.go`
6. `internal/domain/config/manager/manager.go`
7. `internal/app/server/websocket/websocket_server.go`
8. `internal/app/server/websocket/openclaw.go` (mới)
9. `internal/domain/openclaw/*` (mới: connection pool, message model, hàng đợi offline)
10. `internal/data/client/client.go`
11. `internal/app/server/chat/session.go`
12. `internal/app/server/app.go`

## 8. Các bước cài đặt (bao gồm cấu hình channel)

1. Cài đặt plugin milestones OpenClaw:
   `openclaw plugins install @milestones_openclaw/milestones`
2. Trong control panel, mở cấu hình OpenClaw của agent, sao chép endpoint kết nối OpenClaw của agent đó (`ws(s)://.../ws/openclaw?token=...`).
3. Trong phiên OpenClaw, thực hiện "cấu hình channel":
   - Gửi trực tiếp endpoint ở bước trên cho OpenClaw
   - Nói rõ với nó: `配置milestones渠道插件` ("cấu hình plugin channel milestones")
4. Sau khi cấu hình xong, dùng phiên kiểm thử gửi một tin nhắn, xác nhận có thể nhận được phản hồi từ OpenClaw.

## 9. Danh sách nghiệm thu

1. Control panel có thể lấy được OpenClaw endpoint (logic đồng bộ với MCP).
2. `/api/configs` trả về trường `openclaw` có cấu trúc.
3. Client OpenClaw có thể thiết lập kết nối qua `/ws/openclaw` và bắt tay (handshake).
4. Từ khóa vào có thể chuyển sang chế độ OpenClaw, từ khóa ra có thể thoát khỏi chế độ.
5. Trong chế độ OpenClaw, tin nhắn không đi qua LLM, phản hồi có thể chuyển sang TTS.
6. Khi thiết bị offline, phản hồi vào hàng đợi offline; khi online sẽ gửi bù.
7. Hàng đợi offline đáp ứng giới hạn 20 tin nhắn và TTL 24 giờ.
