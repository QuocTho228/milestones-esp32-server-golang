# Hướng dẫn sử dụng Trình quản lý cấu hình (Configuration Manager)

## Tổng quan

Package này cung cấp hai trình quản lý (manager) chính:

1. **ConfigManager** - Trình quản lý cấu hình, cung cấp các chức năng quản lý cấu hình ở tầng cao
2. **AuthManager** - Trình quản lý xác thực, chuyên xử lý các chức năng liên quan đến kích hoạt thiết bị và xác thực

## Tính năng cốt lõi

### ConfigManager - Trình quản lý cấu hình

- ✅ Cơ chế cache cấu hình, giúp tăng hiệu suất truy cập
- ✅ Chức năng kiểm tra tính hợp lệ (validate) của cấu hình
- ✅ Quản lý toàn cục theo mô hình Singleton
- ✅ Cơ chế dọn dẹp (clear) và vô hiệu hóa (invalidate) cache
- ✅ Truy cập đồng thời an toàn với luồng (thread-safe)

### AuthManager - Trình quản lý xác thực

- ✅ Kiểm tra trạng thái kích hoạt thiết bị (thông qua giao diện HTTP)
- ✅ Lấy thông tin kích hoạt theo thời gian thực (không sử dụng cache)
- ✅ Xác minh mã thử thách (challenge code) và xác thực bảo mật bằng HMAC
- ✅ Gọi trực tiếp đến giao diện backend, đảm bảo tính thời gian thực của dữ liệu
- ✅ **Tích hợp giao diện HTTP** - Gọi đến giao diện kích hoạt của hệ thống quản lý backend

## Tích hợp giao diện HTTP

AuthManager hiện gọi đến hệ thống quản lý backend thông qua giao diện HTTP, hỗ trợ các giao diện sau:

### 1. Kiểm tra trạng thái kích hoạt thiết bị

```http
GET /api/internal/device/check-activation?device_id=xxx&client_id=xxx
```

### 2. Lấy thông tin kích hoạt thiết bị

```http
GET /api/internal/device/activation-info?device_id=xxx&client_id=xxx
```

### 3. Kích hoạt thiết bị

```http
POST /api/internal/device/activate
Content-Type: application/json

{
  "device_id": "xxx",
  "client_id": "xxx",
  "code": "123456",
  "challenge": "uuid",
  "algorithm": "hmac-sha256",
  "serial_number": "ABC123",
  "hmac": "signature"
}
```

## Hướng dẫn cấu hình

Thêm cấu hình sau vào file cấu hình (config.yaml):

```yaml
manager:
  backend_url: 'http://localhost:8080' # URL gốc (base URL) của hệ thống quản lý backend
```

Nếu không cấu hình, hệ thống sẽ mặc định sử dụng `http://localhost:8080`.

## Ví dụ sử dụng

```go
package main

import (
    "context"
    "milestones-esp32-server-golang/internal/domain/config/manager"
)

func main() {
    ctx := context.Background()

    // Khởi tạo trình quản lý
    err := manager.Init()
    if err != nil {
        panic(err)
    }

    err = manager.InitAuth()
    if err != nil {
        panic(err)
    }

    // Lấy instance của trình quản lý
    configManager := manager.GetInstance()
    authManager := manager.GetAuthInstance()

    // Sử dụng trình quản lý cấu hình
    config, err := configManager.GetUserConfig(ctx, "device_001")
    if err != nil {
        // Xử lý lỗi
    }

    // Sử dụng trình quản lý xác thực (thông qua giao diện HTTP)
    activated, err := authManager.IsDeviceActivated(ctx, "device_001", "client_001")
    if err != nil {
        // Xử lý lỗi
    }

    if !activated {
        // Lấy thông tin kích hoạt
        code, challenge, message, timeout := authManager.GetActivationInfo(ctx, "device_001", "client_001")
        // Hiển thị mã kích hoạt cho người dùng...

        // Xác minh sau khi người dùng nhập mã kích hoạt
        activationPayload := types.ActivationPayload{
            Algorithm:    "hmac-sha256",
            SerialNumber: "ABC123",
            Challenge:    challenge,
            HMAC:         "calculated_hmac",
        }

        success, err := authManager.VerifyChallenge(ctx, "device_001", "client_001", fmt.Sprintf("%d", code), activationPayload)
        // Xử lý kết quả kích hoạt...
    }
}
```

## Ưu điểm kiến trúc

### Tích hợp hệ thống frontend

- Thiết bị ESP32 hoặc các hệ thống frontend khác gọi trực tiếp đến AuthManager
- AuthManager gọi nội bộ đến hệ thống quản lý backend thông qua HTTP
- Thực hiện tách rời (decouple) giữa hệ thống frontend và hệ thống quản lý backend

### Dữ liệu thời gian thực

- Gọi trực tiếp giao diện HTTP để lấy trạng thái mới nhất
- Thiết kế không dùng cache, đảm bảo tính thời gian thực của dữ liệu
- Đơn giản hóa kiến trúc, giảm độ phức tạp

### Xử lý lỗi

- Xử lý lỗi và ghi log hoàn thiện
- Xử lý dự phòng (fallback/degradation) khi request HTTP thất bại
- Thông tin lỗi chi tiết và log debug

### Bảo mật

- Hỗ trợ xác thực HMAC
- Quy trình kích hoạt an toàn
- Xác thực trạng thái theo thời gian thực

## Lưu ý

1. Đảm bảo hệ thống quản lý backend đang chạy và có thể truy cập được
2. Cấu hình đúng `manager.backend_url`
3. Timeout mặc định của HTTP client là 10 giây
4. Chế độ không cache, mỗi lần gọi đều sẽ gửi request đến giao diện backend
5. Đảm bảo kết nối mạng ổn định, tránh việc gọi giao diện thất bại liên tục
