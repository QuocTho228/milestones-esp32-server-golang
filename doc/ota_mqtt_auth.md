# Cấu hình xác thực MQTT cho API OTA

## Tổng quan

API OTA hiện đã hỗ trợ cơ chế xác thực mật khẩu MQTT dựa trên chữ ký HMAC-SHA256, cung cấp phương thức xác thực an toàn hơn. Đồng thời, MQTT server cũng hỗ trợ logic xác thực tương ứng.

## Cấu trúc cấu hình

### File cấu hình (config/config.yaml)

```yaml
mqtt_server:
  signature_key: 'your_ota_signature_key_here'
ota:
  signature_key: 'your_ota_signature_key_here'
  test:
    websocket:
      url: 'ws://192.168.1.97:8989/milestones/v1/'
    mqtt:
      enable: false
      endpoint: '192.168.1.97'
  external:
    websocket:
      url: 'wss://www.tb263.cn:55555/go_ws/milestones/v1/'
    mqtt:
      enable: false
      endpoint: 'www.youdomain.cn'
```

### Giải thích cấu hình

- `mqtt_server.signature_key`: khóa chữ ký MQTT, dùng để tạo chữ ký cho mật khẩu MQTT
- `ota.signature_key`: khóa (key) được dùng khi OTA cấp phát mật khẩu MQTT, cần tương ứng với `mqtt_server.signature_key`
- `ota.test`: cấu hình môi trường test (dùng cho IP nội bộ)
- `ota.external`: cấu hình môi trường bên ngoài (dùng cho IP công cộng)

### Tích hợp với milestones-mqtt-gateway

Hệ thống này được thiết kế để phối hợp sử dụng cùng dự án [milestones-mqtt-gateway](https://github.com/78/milestones-mqtt-gateway) chính thức của Xiage, nhằm hiện thực đầy đủ quy trình xác thực MQTT:

1. **Yêu cầu về tính nhất quán cấu hình**: `ota.signature_key` bắt buộc phải hoàn toàn giống với khóa chữ ký trong dự án milestones-mqtt-gateway
2. **Quy trình xác thực**:
   - milestones-mqtt-gateway chịu trách nhiệm tạo thông tin đăng nhập (credentials) kết nối MQTT
   - Hệ thống này chịu trách nhiệm xác thực thông tin đăng nhập kết nối MQTT
   - Cả hai bên dùng chung thuật toán chữ ký và khóa để đảm bảo xác thực thành công
3. **Khuyến nghị triển khai**: nên triển khai cả hai dự án trong cùng một môi trường mạng, đảm bảo cấu hình được đồng bộ cập nhật

## Các hàm tiện ích (utility functions)

### 1. Tạo chữ ký mật khẩu

```go
// Tạo chữ ký mật khẩu HMAC-SHA256
password := util.GeneratePasswordSignature(data, key)
```

### 2. Tạo thông tin đăng nhập MQTT

```go
// Tạo đầy đủ thông tin đăng nhập kết nối MQTT
credentials, err := util.GenerateMqttCredentials(deviceId, clientId, ip, signatureKey)
if err != nil {
    // Xử lý lỗi
}
// credentials bao gồm: ClientId, Username, Password
```

### 3. Xác thực thông tin đăng nhập MQTT

```go
// Xác thực thông tin đăng nhập kết nối MQTT
credentialInfo, err := util.ValidateMqttCredentials(clientId, username, password, signatureKey)
if err != nil {
    // Xác thực thất bại
}
// credentialInfo bao gồm: GroupId, MacAddress, UUID, UserData
```

## Logic xác thực MQTT

### 1. Định dạng Client ID

```
GID_test@@@{deviceId}@@@{clientId}
```

Ví dụ:

```
GID_test@@@02_4A_7D_E3_89_BF@@@e3b0c442-98fc-4e1a-8c3d-6a5b6a5b6a5b
```

### 2. Định dạng Username

Là chuỗi JSON được mã hóa Base64, chứa thông tin IP của client:

```yaml
ip: '1.202.193.194'
```

Sau khi mã hóa Base64:

```
eyJpcCI6IjEuMjAyLjE5My4xOTQifQ==
```

### 3. Tạo Password

Dùng thuật toán HMAC-SHA256 để tạo chữ ký mật khẩu:

```go
signatureData := clientId + "|" + username
password := HMAC-SHA256(signatureData, signature_key)
```

### 4. Logic xác thực

Khi client xác thực, cần thực hiện:

1. Parse clientId, trích xuất groupId, macAddress, uuid
2. Decode username, lấy thông tin IP
3. Dùng cùng khóa chữ ký và thuật toán để xác thực mật khẩu

## Xác thực ở MQTT server

### Quy trình xác thực

1. **Xác thực super admin**
   - Username: `admin` (có thể cấu hình)
   - Password: `shijingbo!@#` (có thể cấu hình)

2. **Xác thực người dùng thông thường**
   - Ưu tiên dùng xác thực bằng chữ ký HMAC-SHA256
   - Nếu chưa cấu hình khóa chữ ký, sẽ dùng lại phương thức xác thực AES

### Hiện thực hook xác thực

```go
func (h *AuthHook) OnConnectAuthenticate(cl *mqttServer.Client, pk packets.Packet) bool {
    username := string(pk.Connect.Username)
    password := string(pk.Connect.Password)
    clientId := string(pk.Connect.ClientIdentifier)

    // Xác thực super admin
    if username == adminUsername && password == adminPassword {
        return true
    }

    // Xác thực người dùng thông thường - dùng logic xác thực chữ ký mới
    signatureKey := viper.GetString("mqtt_server.signature_key")
    if signatureKey != "" {
        credentialInfo, err := util.ValidateMqttCredentials(clientId, username, password, signatureKey)
        if err != nil {
            return false
        }
        return true
    }

    // Dùng lại logic xác thực AES
    return h.validateWithAes(username, password)
}
```

## Khả năng tương thích

- Nếu chưa cấu hình `mqtt_server.signature_key`, hệ thống sẽ tự động dùng lại cách tạo mật khẩu SHA256/AES cũ
- Vẫn giữ được khả năng tương thích ngược, không ảnh hưởng tới các tính năng hiện có
- MQTT server hỗ trợ nhiều phương thức xác thực cùng tồn tại song song

## Khuyến nghị bảo mật

1. Sử dụng chuỗi ký tự ngẫu nhiên đủ mạnh làm khóa chữ ký
2. Định kỳ xoay vòng (rotate) khóa chữ ký
3. Sử dụng kết nối HTTPS/WSS trong môi trường production
4. Giám sát các lần đăng nhập bất thường
5. Bật ghi log để theo dõi các trường hợp xác thực thành công/thất bại
6. **Đảm bảo khóa chữ ký giữa milestones-mqtt-gateway và hệ thống này luôn được cập nhật đồng bộ**

## Cấu trúc dữ liệu

### MqttCredentials

```go
type MqttCredentials struct {
    ClientId string `json:"client_id"`
    Username string `json:"username"`
    Password string `json:"password"`
}
```

### MqttCredentialInfo

```go
type MqttCredentialInfo struct {
    GroupId    string                 `json:"groupId"`
    MacAddress string                 `json:"macAddress"`
    UUID       string                 `json:"uuid"`
    UserData   map[string]interface{} `json:"userData"`
}
```

# Hướng dẫn sử dụng milestones-mqtt-gateway chính thức của Xiage

Hệ thống này có thể được dùng phối hợp cùng dự án [milestones-mqtt-gateway](https://github.com/78/milestones-mqtt-gateway) chính thức của Xiage.

Chỉ cần username/password MQTT trong API OTA xác thực thành công với milestones-mqtt-gateway; để đảm bảo việc xác thực MQTT hoạt động bình thường, **cấu hình `ota.signature_key` bắt buộc phải giống với khóa chữ ký trong milestones-mqtt-gateway**.

Cấu hình như sau:

1. Không bật mqtt server (dùng milestones-mqtt-gateway)
2. Cấu hình `ota.signature_key` bắt buộc phải giống với khóa chữ ký trong milestones-mqtt-gateway
3. Cấu hình backend websocket của milestones-mqtt-gateway trỏ về địa chỉ của dự án này

```yaml
mqtt_server:
  enable: false
ota:
  signature_key: 'your_ota_signature_key_here'
  test: # Kết quả trả về khi test trong mạng nội bộ
    websocket:
      url: 'ws://192.168.1.97:8989/milestones/v1/'
    mqtt:
      enable: true
      endpoint: '192.168.1.97:1883' # Địa chỉ mqtt server trong milestones-mqtt-gateway
  external: # Kết quả trả về cho mạng bên ngoài
    websocket:
      url: 'wss://www.tb263.cn:55555/go_ws/milestones/v1/'
    mqtt:
      enable: true
      endpoint: 'mqtt.youdomain.com:1883' # Địa chỉ mqtt server trong milestones-mqtt-gateway
```
