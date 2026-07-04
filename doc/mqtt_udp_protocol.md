# 🚦 Luồng dữ liệu

1. **Gọi API OTA**
   - Lấy địa chỉ **MQTT**, **WebSocket**

2. **Kết nối MQTT**
   - `mqtt_server` tích hợp sẵn sẽ publish một sự kiện lifecycle tới `/p2p/device_public/_server/lifecycle`
   - Chương trình chính dựa vào `device_id` để tạo hoặc tái sử dụng MQTT transport, và cố gắng làm nóng (prewarm) MCP phía thiết bị theo khả năng tốt nhất

3. **Gửi thông điệp `hello`**
   - Lấy được:
     - 🎵 `audio_params`
     - 🌐 Địa chỉ UDP server
     - 🔑 `aes_key`
     - 🧩 `nonce`

4. **Kết nối tới UDP server**
   - Thực hiện gửi và nhận dữ liệu giọng nói

5. **Gửi các tín hiệu tiếp theo như `listen`, `abort`**
   - Ý nghĩa của các tín hiệu này giữ nguyên không đổi, vẫn dựa trên việc khởi tạo cấp độ chat sau khi `hello` đã hoàn tất

---

# 🧭 Topic Lifecycle

- **Topic**: `/p2p/device_public/_server/lifecycle`
- **Mục đích**: chỉ dùng nội bộ ở phía server, để truyền tải sự kiện thiết bị online/offline qua MQTT
- **Ví dụ nội dung thông điệp**:

  ```json
  {
    "type": "mqtt_lifecycle",
    "device_id": "11:22:33:44:55:66",
    "state": "online",
    "client_id": "GID_test@@@11_22_33_44_55_66@@@uuid",
    "ts": 1710000000000
  }
  ```

- **Định nghĩa trạng thái**
  - `online`: thiết bị vừa kết nối tới `mqtt_server`, chương trình chính có thể chuẩn bị trước transport và MCP
  - `offline`: thiết bị ngắt kết nối khỏi `mqtt_server`, chương trình chính lập tức ánh xạ trạng thái offline, nhưng transport sẽ được giữ lại trong một khoảng thời gian để tái sử dụng nếu kết nối lại trong thời gian ngắn

- **Giải thích ranh giới**
  - Sự kiện lifecycle không thay thế cho `hello`
  - Sự kiện lifecycle chỉ duy trì tài nguyên ở cấp độ kết nối, không mang theo các thông tin cấp độ chat như `audio_params`, thỏa thuận UDP

---

# 🛠️ Quy trình xử lý ở phía server

| Bước                         | Giải thích                                                                                                                   |
| :--------------------------- | :--------------------------------------------------------------------------------------------------------------------------- |
| 1. Lắng nghe lifecycle MQTT  | Khi nhận được sự kiện `online`, tạo hoặc tái sử dụng transport, và cố gắng làm nóng MCP phía thiết bị theo khả năng tốt nhất |
| 2. Xử lý `hello`             | Trả về `audio_params`, địa chỉ UDP, khóa và `nonce`, đồng thời chuẩn bị trạng thái phiên cấp độ chat                         |
| 3. Lắng nghe thông điệp MQTT | Khi nhận được `type: listen, state: start`, khởi tạo cấu trúc `clientState`, trạng thái là `start`                           |
| 4. Dịch vụ UDP               | Khi nhận được gói tin, parse `nonce`, tìm `clientState` tương ứng, điền địa chỉ remote, trạng thái là `recv`                 |
| 5. Dừng nhận                 | Khi nhận được `type: listen, state: stop` hoặc tự động phát hiện không có âm thanh, dừng nhận dữ liệu                        |
| 6. Lifecycle MQTT offline    | Khi nhận được sự kiện `offline`, lập tức ánh xạ trạng thái offline, và thu hồi transport sau khi hết thời gian giữ lại       |

---

# 🔗 Các mối quan hệ liên kết

- OTA xác thực **địa chỉ MAC** và **clientId**, và liên kết với **uid**
- **Địa chỉ MQTT** và **mqtt_clientId** mà OTA cấp phát sẽ liên kết với **địa chỉ MAC** và **clientId**
- Thông qua **thông điệp lifecycle kết nối MQTT** có thể liên kết trước **địa chỉ MAC**, `device_id`, `client_id`
- Thông qua **thông điệp `hello` của MQTT** có thể liên kết tới `audio_params`, `aes_key`, `nonce`
- Thông qua **thông điệp âm thanh UDP** có thể liên kết tới `nonce`

---

> **Giải thích:**
>
> - Cấu trúc `clientState` dùng để duy trì trạng thái phiên cấp độ chat và tài nguyên của từng client.
> - Transport và MCP có thể được chuẩn bị trước ở giai đoạn thiết bị online qua MQTT, nhưng việc thỏa thuận thực sự ở cấp độ chat vẫn phải dựa trên `hello`.
> - `nonce` là định danh duy nhất giữa client và server, dùng để liên kết bảo mật và định tuyến dữ liệu.
