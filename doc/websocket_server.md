# Hướng dẫn cấu hình WebSocket Server và OTA

Tài liệu này dành cho người dùng chưa có kinh nghiệm, hướng dẫn chi tiết cách cấu hình WebSocket server và các tham số liên quan đến OTA (cập nhật firmware).

---

## 1. Vị trí file cấu hình

Tất cả các cấu hình chính đều nằm tại:

- `config/config.yaml`

Nếu không tìm thấy file này, có thể tham khảo thêm `config/config.json.git`.

---

## 2. Cấu hình WebSocket Server

### 2.1 Vai trò

WebSocket server dùng cho việc giao tiếp thời gian thực giữa thiết bị và server.

### 2.2 Các tham số cấu hình quan trọng

Trong file `config/config.yaml`, tìm nội dung sau:

```yaml
websocket:
  host: '0.0.0.0'
  port: 8989
```

- `host`: Địa chỉ lắng nghe, thông thường giữ nguyên `0.0.0.0` là được.
- `port`: Cổng lắng nghe, mặc định là `8989`, có thể thay đổi theo nhu cầu.

### 2.3 Cách chỉnh sửa

Nếu cần đổi cổng thành 9000:

```yaml
websocket:
  host: '0.0.0.0'
  port: 9000
```

---

## 3. Cấu hình OTA (cập nhật firmware)

### 3.1 Vai trò

OTA dùng để thiết bị tự động lấy các tham số kết nối WebSocket/MQTT và thông tin cập nhật firmware do server cấp phát.

### 3.2 Các tham số cấu hình quan trọng

Trong file `config/config.yaml`, tìm mục `ota`:

```yaml
ota:
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

- `test`: Các tham số mà thiết bị nhận được trong môi trường mạng nội bộ (LAN); điều kiện phán đoán trong chương trình là địa chỉ IP bắt đầu bằng 192.168 hoặc 127.0.
- `external`: Các tham số dành cho môi trường mạng ngoài (internet, WAN).
- `websocket.url`: Địa chỉ WebSocket server mà thiết bị sẽ kết nối tới.
- `mqtt.enable`: Nếu được bật, interface OTA sẽ trả về địa chỉ MQTT đã cấu hình, thiết bị sẽ ưu tiên chọn phương thức mqtt+udp.
- `mqtt.endpoint`: Địa chỉ MQTT server; phía thiết bị mặc định dùng cổng 8883 (kết nối TLS), nếu chỉ định cổng khác 8883 thì sẽ dùng kết nối TCP không mã hoá.

### 3.3 Ví dụ chỉnh sửa thường gặp

- Chỉnh sửa địa chỉ WebSocket nội bộ:
  ```yaml
  ota:
    test:
      websocket:
        url: 'ws://192.168.1.100:8989/milestones/v1/'
  ```
- Chỉnh sửa địa chỉ WebSocket bên ngoài:
  ```yaml
  ota:
    external:
      websocket:
        url: 'wss://yourdomain.com:55555/go_ws/milestones/v1/'
  ```

---

## 4. Mô tả interface OTA (cách thiết bị lấy cấu hình)

1. Thiết bị gửi HTTP POST request đến `http://địa-chỉ-server:cổng/milestones/ota/`.
2. Header của request cần bao gồm:
   - `Device-Id`: ID duy nhất của thiết bị (ví dụ địa chỉ MAC)
   - `Client-Id`: ID duy nhất của client
3. Server sẽ tự động chọn cấu hình `test` hoặc `external` dựa trên địa chỉ IP của thiết bị, và trả về các tham số WebSocket/MQTT, v.v.
4. Thiết bị phân tích (parse) nội dung trả về, và kết nối tới WebSocket server theo `websocket.url`.

---

## 5. Câu hỏi thường gặp

- **Cổng bị chiếm dụng?**
  - Thay đổi `websocket.port`, sau đó khởi động lại dịch vụ.
- **Thiết bị không kết nối được server?**
  - Kiểm tra `websocket.url` trong cấu hình `ota` có chính xác không, cổng của server đã mở chưa.
- **Cần dùng MQTT?**
  - Đặt `mqtt.enable` thành `true`, và cấu hình `endpoint`.

---

Nếu có thắc mắc, khuyến nghị nên kiểm tra các mục cấu hình trong `config/config.yaml` trước, sau đó tham khảo lại hướng dẫn này.
