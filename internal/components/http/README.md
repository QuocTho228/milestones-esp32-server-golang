# Thành phần HTTP

Thành phần HTTP client thống nhất, dùng để quản lý toàn bộ các lệnh gọi HTTP đến backend Manager.

## Cấu trúc thư mục

```
internal/components/http/
├── client.go          # HTTP client dùng chung (hỗ trợ retry, xác thực, v.v.)
├── manager_client.go  # Client chuyên dụng cho backend Manager
├── types.go           # Định nghĩa kiểu dữ liệu
└── README.md          # Tài liệu này
```

## Giải thích thiết kế

### Client (HTTP client dùng chung)

Cung cấp các chức năng HTTP request cơ bản:

- Hỗ trợ cơ chế thử lại (retry) (dùng exponential backoff)
- Hỗ trợ Token xác thực (Bearer Token)
- Hỗ trợ tùy chỉnh thời gian timeout
- Xử lý lỗi thống nhất
- Tự động serialize/deserialize JSON

### ManagerClient (Client chuyên dụng cho backend Manager)

Được xây dựng dựa trên client dùng chung, chuyên dùng để gọi API của backend Manager.

## Ví dụ sử dụng

### Tạo Manager client

```go
import "milestones-esp32-server-golang/internal/components/http"

client := http.NewManagerClient(http.ManagerClientConfig{
    BaseURL:    "http://localhost:8080",
    AuthToken:  "your-token",  // Tùy chọn
    Timeout:    10 * time.Second,
    MaxRetries: 3,
})
```

### Gửi request GET

```go
var response MyResponse
err := client.DoRequest(ctx, http.RequestOptions{
    Method: "GET",
    Path:   "/api/configs",
    QueryParams: map[string]string{
        "device_id": "device123",
    },
    Response: &response,
})
```

### Gửi request POST

```go
request := MyRequest{
    Field1: "value1",
    Field2: "value2",
}

err := client.DoRequest(ctx, http.RequestOptions{
    Method: "POST",
    Path:   "/api/internal/history/messages",
    Body:   request,
})
```

### Lấy response gốc

```go
body, err := client.DoRequestRaw(ctx, http.RequestOptions{
    Method: "GET",
    Path:   "/api/system/configs",
})
```

## Giải thích về việc tái cấu trúc (refactor)

### Trước khi tái cấu trúc

- `HistoryClient` và `ConfigManager` mỗi cái tự triển khai logic gọi HTTP riêng
- Code bị trùng lặp, chi phí bảo trì cao
- Logic retry, xác thực... bị phân tán rải rác

### Sau khi tái cấu trúc

- Thành phần HTTP thống nhất, quản lý tập trung
- Code được tái sử dụng, dễ bảo trì
- Cơ chế xử lý lỗi và retry thống nhất

## Các module đã được tái cấu trúc

1. **internal/data/history/client.go** - Client lịch sử chat
2. **internal/domain/config/manager/manager.go** - Trình quản lý cấu hình
3. **internal/domain/config/manager/auth.go** - API liên quan đến xác thực

## Lưu ý

- Tất cả các lệnh gọi HTTP đến backend Manager đều nên sử dụng `ManagerClient`
- Nếu cần gọi đến các backend service khác, có thể xây dựng client chuyên dụng mới dựa trên `Client`
- Cơ chế retry mặc định tối đa 3 lần, có thể điều chỉnh qua cấu hình
