# Hướng dẫn quy trình kết nối WebSocket

## Tổng quan

Tài liệu này mô tả quy trình kết nối và giao tiếp WebSocket giữa `internal/domain/config/manager/websocket_client.go` và `websocket.go`.

## Thiết kế kiến trúc

### Định nghĩa vai trò

1. **`internal/domain/config/manager/websocket_client.go`** - WebSocket client của server chính
   - Đóng vai trò client kết nối đến Manager Backend
   - Có thể gửi request và nhận response
   - Hỗ trợ giao tiếp hai chiều (bidirectional)

2. **`websocket.go`** - WebSocket server của Manager Backend
   - Đóng vai trò server nhận kết nối WebSocket từ server chính
   - Xử lý các request được gửi từ server chính
   - **Chỉ giữ lại kết nối hợp lệ cuối cùng** (kết nối mới sẽ ngắt kết nối cũ)
   - Hỗ trợ chủ động đẩy (push) message

### Quy trình kết nối

```
Server chính (internal/domain/config/manager/websocket_client.go)  →  Manager Backend (websocket.go)
        Client                          Server (kết nối đơn - single connection)
```

## Chi tiết quy trình

### 1. Thiết lập kết nối

#### Manager Backend khởi động WebSocket server

```go
// Trong websocket.go
controller := NewWebSocketController(db)
// Đăng ký trong router
router.GET("/ws", controller.HandleWebSocket)
```

#### Server chính kết nối đến Manager Backend

```go
// Trong internal/domain/config/manager/websocket_client.go
client := manager.NewWebSocketClient()
err := client.Connect(ctx)
```

Định dạng URL kết nối:

- Nếu cấu hình là `http://localhost:8080`
- Sẽ kết nối thực tế đến `ws://localhost:8080/ws`

**Quan trọng**: Nếu có yêu cầu kết nối mới, Manager Backend sẽ tự động ngắt kết nối hiện có, chỉ giữ lại kết nối mới nhất.

### 2. Quy trình yêu cầu danh sách công cụ (tool list)

#### Server chính yêu cầu danh sách công cụ MCP

```go
// Trong internal/domain/config/manager/websocket_client.go
response, err := client.SendRequest(ctx, "GET", "/api/mcp/tools", map[string]interface{}{
    "agent_id": "some_agent_id",
})
```

#### Manager Backend xử lý request

```go
// Trong websocket.go
func (client *WebSocketClient) handleMcpToolListRequest(request *WebSocketRequest) {
    agentID := request.Body["agent_id"].(string)

    // Logic lấy danh sách công cụ
    response := map[string]interface{}{
        "agent_id": agentID,
        "tools":    []string{"tool1", "tool2", "tool3"},
        "count":    3,
    }

    client.sendResponse(request.ID, 200, response, "")
}
```

### 3. Hỗ trợ giao tiếp hai chiều

### Client → Server (chức năng có sẵn)

#### Server chính yêu cầu danh sách công cụ MCP

```go
// Trong internal/domain/config/manager/websocket_client.go
response, err := client.SendRequest(ctx, "GET", "/api/mcp/tools", map[string]interface{}{
    "agent_id": "some_agent_id",
})
```

#### Manager Backend xử lý request

```go
// Trong websocket.go
func (client *WebSocketClient) handleMcpToolListRequest(request *WebSocketRequest) {
    agentID := request.Body["agent_id"].(string)

    // Logic lấy danh sách công cụ
    response := map[string]interface{}{
        "agent_id": agentID,
        "tools":    []string{"tool1", "tool2", "tool3"},
        "count":    3,
    }

    client.sendResponse(request.ID, 200, response, "")
}
```

### Server → Client (chức năng mới bổ sung)

#### Manager Backend chủ động gửi yêu cầu tới client

```go
// Trong websocket.go
func (ctrl *WebSocketController) RequestMcpToolsFromClient(ctx context.Context, agentID string) (*WebSocketResponse, error) {
    body := map[string]interface{}{
        "agent_id": agentID,
    }
    return ctrl.SendRequestToClient(ctx, "GET", "/api/mcp/tools", body)
}

// Yêu cầu thông tin server từ client
func (ctrl *WebSocketController) RequestServerInfoFromClient(ctx context.Context) (*WebSocketResponse, error) {
    return ctrl.SendRequestToClient(ctx, "GET", "/api/server/info", nil)
}

// Yêu cầu ping từ client
func (ctrl *WebSocketController) RequestPingFromClient(ctx context.Context) (*WebSocketResponse, error) {
    return ctrl.SendRequestToClient(ctx, "GET", "/api/server/ping", nil)
}
```

#### Client xử lý request từ server

```go
// Trong internal/domain/config/manager/websocket_client.go
client.SetRequestHandler(func(request *WebSocketRequest) {
    // Xử lý request nhận được
    switch request.Path {
    case "/api/mcp/tools":
        // Xử lý request danh sách công cụ MCP
        c.handleMcpToolListRequest(request)
    case "/api/server/info":
        // Xử lý request thông tin server
        c.handleServerInfoRequest(request)
    case "/api/server/ping":
        // Xử lý request ping
        c.handlePingRequest(request)
    }
})
```

### Ví dụ giao tiếp hai chiều hoàn chỉnh

```go
// 1. Client kết nối đến server
client := manager.NewWebSocketClient()
err := client.Connect(ctx)

// 2. Client thiết lập bộ xử lý request (request handler)
client.SetRequestHandler(func(request *WebSocketRequest) {
    // Xử lý request đến từ server
    // Và gửi lại response
})

// 3. Client chủ động gửi request đến server
response, err := client.SendRequest(ctx, "GET", "/api/mcp/tools", map[string]interface{}{
    "agent_id": "agent_123",
})

// 4. Server chủ động gửi request đến client
serverResponse, err := websocketController.RequestMcpToolsFromClient(ctx, "agent_456")

// 5. Hoàn tất giao tiếp hai chiều
```

## Định dạng message

### Message request (WebSocketRequest)

```json
{
  "id": "uuid-string",
  "method": "GET",
  "path": "/api/mcp/tools",
  "body": {
    "agent_id": "agent_123"
  }
}
```

### Message response (WebSocketResponse)

```json
{
  "id": "uuid-string",
  "status": 200,
  "body": {
    "agent_id": "agent_123",
    "tools": ["tool1", "tool2", "tool3"],
    "count": 3
  },
  "error": ""
}
```

### Message Ping/Pong

```json
// Ping
{"ping": 1640995200}

// Pong
{"pong": 1640995200}
```

## Quản lý kết nối

### Chiến lược kết nối đơn (single connection)

- **Chỉ giữ lại kết nối hợp lệ cuối cùng**
- Kết nối mới sẽ tự động ngắt kết nối hiện có
- Đơn giản hóa logic quản lý kết nối
- Phù hợp với các kịch bản giao tiếp một-một (one-to-one)

### Giám sát trạng thái kết nối

```go
// Kiểm tra xem có client nào đang kết nối không
func (ctrl *WebSocketController) HasConnectedClient() bool

// Lấy client đang kết nối hiện tại
func (ctrl *WebSocketController) GetCurrentClient() *WebSocketClient
```

### Logic chuyển đổi kết nối

```go
// Trong HandleWebSocket
if ctrl.currentClient != nil && ctrl.currentClient.isConnected {
    log.Printf("Ngắt kết nối hiện có: %s", ctrl.currentClient.ID)
    ctrl.currentClient.conn.Close()
    ctrl.currentClient.isConnected = false
}

// Thiết lập kết nối mới làm client hiện tại
ctrl.currentClient = client
```

## Xử lý lỗi

### Lỗi kết nối

- Tự động phát hiện heartbeat (nhịp tim kết nối)
- Tự động ngắt khi kết nối timeout
- Tự động dọn dẹp khi kết nối bất thường
- Kết nối mới tự động thay thế kết nối cũ

### Lỗi message

- Kiểm tra định dạng message
- Trả về response lỗi
- Ghi log

## Yêu cầu cấu hình

### Cấu hình server chính

```yaml
manager:
  backend_url: 'http://localhost:8080'
```

### Cấu hình Manager Backend

```go
// Đăng ký endpoint WebSocket trong router
router.GET("/ws", websocketController.HandleWebSocket)
```

## Đề xuất kiểm thử (testing)

1. **Kiểm thử kết nối**
   - Xác nhận việc thiết lập kết nối WebSocket
   - Kiểm tra kết nối mới ngắt kết nối cũ
   - Kiểm tra ngắt kết nối và kết nối lại

2. **Kiểm thử chức năng**
   - Kiểm thử request danh sách công cụ MCP
   - Xác nhận giao tiếp hai chiều
   - Kiểm thử push message

3. **Kiểm thử lỗi**
   - Ngắt mạng và kết nối lại
   - Xử lý message không hợp lệ
   - Xử lý timeout
   - Timeout heartbeat
   - Chuyển đổi kết nối

## Lưu ý

1. **Giới hạn kết nối đơn**
   - Chỉ có thể có một kết nối hoạt động tại một thời điểm
   - Kết nối mới sẽ bắt buộc ngắt kết nối cũ
   - Phù hợp với kiến trúc chủ-tớ (master-slave), không phù hợp với kịch bản nhiều client

2. **An toàn khi xử lý đồng thời (concurrency)**
   - Sử dụng read-write lock để bảo vệ tham chiếu đến client hiện tại
   - Chuyển đổi client an toàn
   - Gửi message an toàn theo luồng (thread-safe)

3. **Quản lý tài nguyên**
   - Dọn dẹp kịp thời các kết nối đã ngắt
   - Đóng kết nối WebSocket đúng cách
   - Tránh rò rỉ bộ nhớ (memory leak)

4. **Cơ chế heartbeat**
   - Gửi ping mỗi 30 giây
   - Tự động ngắt nếu 60 giây không có phản hồi
   - Hỗ trợ message ping/pong

5. **Ghi log**
   - Ghi lại thay đổi trạng thái kết nối
   - Ghi lại việc chuyển đổi kết nối
   - Ghi lại thông tin request và response
   - Ghi lại lỗi và các trường hợp bất thường

## Ví dụ sử dụng hoàn chỉnh

### Mã kiểm thử giao tiếp hai chiều

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "milestones-esp32-server-golang/internal/domain/config/manager"
)

func main() {
    ctx := context.Background()

    // 1. Tạo client và kết nối
    client := manager.NewWebSocketClient()
    if err := client.Connect(ctx); err != nil {
        log.Fatalf("Kết nối thất bại: %v", err)
    }
    defer client.Disconnect()

    // 2. Thiết lập bộ xử lý request (xử lý request đến từ server)
    client.SetRequestHandler(func(request *manager.WebSocketRequest) {
        log.Printf("Nhận được request từ server: %s %s", request.Method, request.Path)

        switch request.Path {
        case "/api/mcp/tools":
            // Xử lý request danh sách công cụ MCP
            agentID := ""
            if request.Body != nil {
                if id, ok := request.Body["agent_id"].(string); ok {
                    agentID = id
                }
            }

            response := map[string]interface{}{
                "agent_id": agentID,
                "tools":    []string{"client_tool_1", "client_tool_2"},
                "count":    2,
            }

            client.SendResponse(request.ID, 200, response, "")

        case "/api/server/info":
            response := map[string]interface{}{
                "server_name": "milestones-client",
                "version":     "1.0.0",
                "uptime":      time.Now().Format(time.RFC3339),
            }
            client.SendResponse(request.ID, 200, response, "")

        case "/api/server/ping":
            response := map[string]interface{}{
                "message": "pong from client",
                "time":    time.Now().Format(time.RFC3339),
            }
            client.SendResponse(request.ID, 200, response, "")
        }
    })

    // 3. Client chủ động gửi request đến server
    fmt.Println("=== Client gửi request đến server ===")
    response, err := client.SendRequest(ctx, "GET", "/api/mcp/tools", map[string]interface{}{
        "agent_id": "client_agent_123",
    })
    if err != nil {
        log.Printf("Request từ client thất bại: %v", err)
    } else {
        fmt.Printf("Response từ server: %+v\n", response)
    }

    // 4. Chờ một khoảng thời gian để server có cơ hội gửi request
    fmt.Println("Đang chờ request từ server...")
    time.Sleep(5 * time.Second)

    fmt.Println("Kiểm thử giao tiếp hai chiều hoàn tất!")
}
```

### Mã kiểm thử phía server

```go
// Trong Manager Backend
func testBidirectionalCommunication() {
    ctx := context.Background()

    // 1. Kiểm tra trạng thái kết nối của client
    status := websocketController.GetClientConnectionStatus()
    fmt.Printf("Trạng thái client: %+v\n", status)

    // 2. Server chủ động gửi request đến client
    fmt.Println("=== Server gửi request đến client ===")

    // Yêu cầu danh sách công cụ MCP
    response, err := websocketController.RequestMcpToolsFromClient(ctx, "server_agent_456")
    if err != nil {
        log.Printf("Yêu cầu danh sách công cụ MCP thất bại: %v", err)
    } else {
        fmt.Printf("Response danh sách công cụ MCP từ client: %+v\n", response)
    }

    // Yêu cầu thông tin server
    infoResponse, err := websocketController.RequestServerInfoFromClient(ctx)
    if err != nil {
        log.Printf("Yêu cầu thông tin server thất bại: %v", err)
    } else {
        fmt.Printf("Thông tin server từ client: %+v\n", infoResponse)
    }

    // Yêu cầu ping
    pingResponse, err := websocketController.RequestPingFromClient(ctx)
    if err != nil {
        log.Printf("Yêu cầu ping thất bại: %v", err)
    } else {
        fmt.Printf("Response ping từ client: %+v\n", pingResponse)
    }
}
```

## Lưu ý

1. **Yêu cầu về giao tiếp hai chiều**
   - Client bắt buộc phải thiết lập request handler
   - Cả server và client đều phải triển khai phương thức xử lý request tương ứng
   - Request ID phải khớp nhau, đảm bảo response được định tuyến (route) chính xác

2. **Xử lý lỗi**
   - Khi mạng bị ngắt, giao tiếp hai chiều sẽ thất bại
   - Việc xử lý timeout rất quan trọng
   - Kiểm tra trạng thái kết nối là điều không thể thiếu

3. **Cân nhắc về hiệu năng**
   - Tránh gửi request hai chiều quá thường xuyên
   - Thiết lập thời gian timeout hợp lý
   - Giám sát trạng thái kết nối
