# Triển khai MCP Host

MCP (Model Context Protocol) Host được triển khai dựa trên [Eino framework](https://github.com/cloudwego/eino), hỗ trợ quản lý công cụ (tool) ở cả cấp độ toàn cục và cấp độ thiết bị.

## Đặc điểm chức năng

### 🌐 Quản lý công cụ MCP toàn cục

- Kết nối tới nhiều MCP Server thông qua SSE
- Tự động khám phá và đăng ký công cụ
- Giám sát trạng thái kết nối và tự động kết nối lại
- Proxy gọi công cụ

### 📱 Quản lý MCP theo thiết bị

- Mỗi thiết bị có kết nối MCP độc lập
- Hỗ trợ giao thức WebSocket
- Đăng ký công cụ riêng cho từng thiết bị
- Giới hạn số lượng kết nối và dọn dẹp

### 🔧 Tích hợp Eino Framework

- Triển khai interface `tool.InvokableTool`
- Hỗ trợ gọi công cụ gốc của Eino
- Đảm bảo an toàn kiểu dữ liệu (type safety) hoàn toàn
- Hỗ trợ xử lý theo luồng (streaming)

## Thiết kế kiến trúc

```
┌─────────────────────────────────────────────────────────────┐
│                    WebSocket Server                        │
│  /milestones/mcp/{deviceId} - Kết nối MCP của thiết bị        │
│  /milestones/api/mcp/tools/{deviceId} - API danh sách công cụ  │
└─────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    ▼                   ▼
┌─────────────────────────┐  ┌─────────────────────────┐
│   GlobalMCPManager      │  │   DeviceMCPManager      │
│   • Quản lý kết nối SSE │  │   • Quản lý kết nối WS   │
│   • Đăng ký công cụ     │  │   • Đăng ký công cụ      │
│     toàn cục            │  │     thiết bị             │
│   • Tự động kết nối lại │  │   • Dọn dẹp kết nối      │
└─────────────────────────┘  └─────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Eino Tool Interface                     │
│  tool.InvokableTool - Interface gọi công cụ thống nhất      │
└─────────────────────────────────────────────────────────────┘
```

## Hướng dẫn cấu hình

### Cấu hình config.json

```json
{
  "mcp": {
    "global": {
      "enabled": true,
      "servers": [
        {
          "name": "filesystem",
          "sse_url": "http://localhost:3001/sse",
          "enabled": true
        },
        {
          "name": "memory",
          "sse_url": "http://localhost:3002/sse",
          "enabled": false
        }
      ],
      "reconnect_interval": 5,
      "max_reconnect_attempts": 10
    },
    "device": {
      "enabled": true,
      "websocket_path": "/milestones/mcp/",
      "max_connections_per_device": 5
    }
  }
}
```

### Giải thích tham số cấu hình

| Tham số                                 | Kiểu   | Giải thích                                 |
| --------------------------------------- | ------ | ------------------------------------------ |
| `mcp.global.enabled`                    | bool   | Có bật MCP Manager toàn cục hay không      |
| `mcp.global.servers`                    | array  | Danh sách MCP Server                       |
| `mcp.global.reconnect_interval`         | int    | Khoảng thời gian kết nối lại (giây)        |
| `mcp.global.max_reconnect_attempts`     | int    | Số lần kết nối lại tối đa                  |
| `mcp.device.enabled`                    | bool   | Có bật MCP Manager theo thiết bị hay không |
| `mcp.device.websocket_path`             | string | Tiền tố đường dẫn WebSocket                |
| `mcp.device.max_connections_per_device` | int    | Số kết nối tối đa cho mỗi thiết bị         |

## Giao diện API

### Endpoint WebSocket

#### Kết nối MCP thiết bị

```
ws://localhost:8989/milestones/mcp/{deviceId}
```

**Quy trình kết nối:**

1. Client kết nối tới endpoint WebSocket
2. Server gửi thông điệp khởi tạo (initialize)
3. Client phản hồi danh sách công cụ
4. Thiết lập giao tiếp hai chiều

**Định dạng thông điệp:**

```json
{
  "jsonrpc": "2.0",
  "method": "tools/list",
  "id": 1,
  "params": {}
}
```

### REST API

#### Lấy danh sách công cụ của thiết bị

```http
GET /milestones/api/mcp/tools/{deviceId}
```

**Ví dụ phản hồi:**

```json
{
  "deviceId": "device123",
  "tools": {
    "filesystem_read_file": {
      "name": "read_file",
      "description": "Đọc nội dung file",
      "type": "global"
    },
    "device_sensor_data": {
      "name": "sensor_data",
      "description": "Lấy dữ liệu cảm biến",
      "type": "device"
    }
  },
  "globalCount": 5,
  "deviceCount": 3,
  "totalCount": 8,
  "timestamp": 1704067200
}
```

## Ví dụ sử dụng

### 1. Khởi động Server

```go
package main

import (
    "milestones-esp32-server-golang/internal/app/server/websocket"
)

func main() {
    server := websocket.NewWebSocketServer(8989)
    server.Start()
}
```

### 2. Kết nối tới MCP Server

MCP Server cần cung cấp endpoint SSE, hỗ trợ các sự kiện sau:

- `tools` - Cập nhật danh sách công cụ
- `status` - Cập nhật trạng thái kết nối

### 3. Ví dụ kết nối thiết bị

```javascript
// Kết nối WebSocket phía thiết bị
const ws = new WebSocket('ws://localhost:8989/milestones/mcp/device123');

ws.onopen = function () {
  console.log('Kết nối MCP đã được thiết lập');
};

ws.onmessage = function (event) {
  const message = JSON.parse(event.data);
  if (message.method === 'initialize') {
    // Phản hồi khởi tạo
    ws.send(
      JSON.stringify({
        jsonrpc: '2.0',
        id: message.id,
        result: {
          protocolVersion: '2024-11-05',
          serverInfo: {
            name: 'device-mcp-server',
            version: '1.0.0',
          },
        },
      }),
    );
  }
};
```

### 4. Ví dụ gọi công cụ

```go
// Lấy danh sách công cụ toàn cục
globalManager := mcp.GetGlobalMCPManager()
tools := globalManager.GetAllTools()

// Gọi công cụ
for name, tool := range tools {
    result, err := tool.InvokableRun(
        context.Background(),
        `{"path": "/tmp/test.txt"}`,
    )
    if err != nil {
        log.Errorf("Gọi công cụ thất bại: %v", err)
        continue
    }
    log.Infof("Kết quả công cụ %s: %s", name, result)
}
```

## Hướng dẫn phát triển

### Triển khai công cụ MCP tùy chỉnh

```go
type customTool struct {
    name        string
    description string
}

func (t *customTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: t.name,
        Desc: t.description,
        ParamsOneOf: &schema.ParamsOneOf{
            // Định nghĩa tham số
        },
    }, nil
}

func (t *customTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
    // Logic triển khai công cụ
    return "result", nil
}
```

### Mở rộng giao thức MCP

1. Thêm trường mới trong struct `MCPMessage`
2. Thêm xử lý thông điệp mới trong phương thức `handleMessage`
3. Triển khai hàm xử lý tương ứng

## Giám sát và Debug

### Cấp độ log

- `INFO` - Các sự kiện quan trọng như thiết lập kết nối, đăng ký công cụ...
- `ERROR` - Kết nối thất bại, lỗi khi gọi công cụ...
- `DEBUG` - Thông tin tương tác giao thức chi tiết

### Kiểm tra sức khỏe (Health check)

```bash
# Kiểm tra công cụ toàn cục
curl http://localhost:8989/milestones/api/mcp/tools/health_check

# Kiểm tra công cụ của thiết bị cụ thể
curl http://localhost:8989/milestones/api/mcp/tools/device123
```

## Xử lý sự cố

### Các vấn đề thường gặp

1. **Kết nối SSE thất bại**
   - Kiểm tra MCP Server có đang chạy hay không
   - Xác nhận cấu hình URL của SSE
   - Kiểm tra kết nối mạng

2. **Kết nối WebSocket bị ngắt**
   - Kiểm tra cơ chế heartbeat
   - Xác nhận định dạng Device ID
   - Kiểm tra giới hạn số lượng kết nối

3. **Gọi công cụ thất bại**
   - Xác nhận định dạng tham số của công cụ
   - Kiểm tra công cụ đã được đăng ký hay chưa
   - Xem log lỗi

### Tối ưu hiệu năng

- Điều chỉnh khoảng thời gian và số lần kết nối lại
- Thiết lập giới hạn số lượng kết nối phù hợp
- Bật cơ chế tái sử dụng connection pool
- Định kỳ dọn dẹp các kết nối đã hết hạn

## Tài liệu tham khảo

- [Tài liệu Eino Framework](https://www.cloudwego.io/docs/eino/)
- [Đặc tả giao thức MCP](https://github.com/mark3labs/mcp-go)
- [Đặc tả SSE](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
- [Giao thức WebSocket](https://tools.ietf.org/html/rfc6455)
