# Phương án tạo trước (pre-create) Transport dẫn dắt bởi vòng đời MQTT

## Mục tiêu

Khi thiết bị kết nối / ngắt kết nối khỏi `mqtt_server`, `mqtt_server` sẽ thông qua callback để đăng (publish) một thông điệp vòng đời (lifecycle message) lên MQTT topic mà chương trình chính đã lắng nghe (subscribe) sẵn. Sau khi chương trình chính nhận được:

1. Khi thiết bị lên mạng (online), tạo trước `mqtt udp transport`
2. Khi thiết bị lên mạng, làm nóng (warm-up) MCP theo kiểu best-effort
3. Khi thiết bị xuống mạng (offline), ánh xạ (map) ngay trạng thái offline của thiết bị
4. Sau khi thiết bị offline, giữ lại transport trong một khoảng thời gian, tránh việc tạo/hủy liên tục khi kết nối lại trong thời gian ngắn
5. Không thay đổi ngữ nghĩa tín hiệu hiện có của `hello` / `listen` / `abort` / `goodbye`

## Thiết kế Topic

Không thêm tiền tố gốc mới, tái sử dụng tiền tố hiện có `"/p2p/device_public/"`.

Bổ sung topic vòng đời mới:

`/p2p/device_public/_server/lifecycle`

Đề xuất hằng số tương ứng trong code:

- `MDeviceLifecycleTopic = MDevicePubTopicPrefix + "_server/lifecycle"`

## Định dạng thông điệp vòng đời

Nội dung thông điệp sử dụng JSON:

```json
{
  "type": "mqtt_lifecycle",
  "device_id": "ba:8f:17:de:94:94",
  "state": "online",
  "client_id": "GID_test@@@ba_8f_17_de_94_94@@@uuid",
  "ts": 1710000000000
}
```

Giải thích các trường:

- `type`: cố định là `mqtt_lifecycle`
- `device_id`: ID thiết bị đã được chuẩn hóa, thống nhất dùng định dạng dấu hai chấm
- `state`: `online` / `offline`
- `client_id`: client id MQTT gốc, thuận tiện cho việc dò lỗi (troubleshooting)
- `ts`: timestamp của sự kiện, tính bằng mili-giây

## Luồng xử lý đầu-cuối (end-to-end)

### 1. mqtt_server đăng thông điệp vòng đời

Trong `DeviceHook`, tại các hàm:

- `OnSessionEstablished`
- `OnDisconnect`

sẽ thông qua callback để đăng sự kiện vòng đời lên `/p2p/device_public/_server/lifecycle`.

Về mặt triển khai, việc đăng (publish) vẫn do `mqtt_server` đảm nhiệm, chỉ là hành động đăng được thu gọn lại thành lời gọi callback bên trong hook, tránh việc logic ghép topic bị phân tán ở nhiều nơi.

### 2. Chương trình chính tái sử dụng subscription hiện có

`MqttUdpAdapter` tiếp tục chỉ subscribe topic hiện có:

`/p2p/device_public/#`

Khi nhận được tin nhắn, trước tiên phán đoán topic:

- Nếu là `/p2p/device_public/_server/lifecycle`, đi vào nhánh xử lý vòng đời
- Nếu không, tiếp tục đi theo nhánh xử lý tin nhắn nghiệp vụ thiết bị hiện có

Cách làm này sẽ không ảnh hưởng đến việc phân tích các tín hiệu bình thường phía sau như `hello` / `listen`.

### 3. Tạo trước transport khi thiết bị lên mạng

Sau khi nhận được thông điệp vòng đời `online`:

1. Thực hiện chống dội (debounce) cho vòng đời trước
2. Nếu transport chưa tồn tại, thì tạo ngay `MqttUdpConn + UdpSession`
3. Kích hoạt `onNewConnection`, để chương trình chính tạo `ChatManager`
4. Đánh dấu broker online
5. Kích hoạt một lần làm nóng MCP theo kiểu best-effort
6. Ánh xạ trạng thái online của thiết bị

Lưu ý:

- Những gì được tạo ở đây là `transport` và `ChatManager`
- `ChatSession` vẫn giữ nguyên cơ chế tạo lười (lazy) sau `hello`

### 4. Thu hồi trễ (delayed cleanup) transport khi thiết bị xuống mạng

Sau khi nhận được thông điệp vòng đời `offline`:

1. Trước tiên đánh dấu broker offline
2. Ánh xạ ngay trạng thái offline của thiết bị
3. Khởi động timer dọn dẹp trễ
4. Trong khoảng thời gian ân hạn (grace period), vẫn giữ lại `transport + udp session`
5. Nếu trong grace period nhận lại được `online`, hủy timer cleanup và tái sử dụng transport cũ

Thời gian giữ lại mặc định đề xuất là `2 phút`, có thể cấu hình được ở các phiên bản sau.

## Ngữ nghĩa trạng thái online

Trạng thái online của thiết bị MQTT-UDP sẽ được đổi sang dẫn dắt bởi vòng đời MQTT, thay vì do việc tạo/hủy `ChatManager` quyết định.

Cụ thể là:

- MQTT `online` -> thiết bị online
- MQTT `offline` -> thiết bị offline

Để tránh thông báo trùng lặp:

- `App.OnNewConnection()` đối với `websocket` giữ nguyên logic cũ
- Đối với `mqtt udp`, `DeviceOnline / DeviceOffline` được đổi sang do callback vòng đời của `MqttUdpAdapter` kích hoạt

## Mối quan hệ với hello / listen

Logic tín hiệu chat hiện có không thay đổi:

- Transport có thể tồn tại trước, ngay sau khi kết nối MQTT được thiết lập
- `ChatManager` có thể tồn tại trước
- `ChatSession` vẫn được tạo sau khi `hello` thành công
- `listen` vẫn yêu cầu `hello` đã hoàn tất

Nhờ vậy có thể đạt được "tạo trước transport" mà không thay đổi ngữ nghĩa ở tầng session.

## Chiến lược làm nóng MCP

Sau khi sự kiện vòng đời `online` đến, kích hoạt một lần làm nóng MCP theo kiểu best-effort.

Đồng thời vẫn giữ lại logic khởi tạo MCP hiện có trong `hello` như phương án dự phòng.

Khi hai luồng cùng tồn tại song song, dựa vào năng lực idempotent (đảm bảo tính bất biến khi gọi lại) và state machine sẵn có của nhánh hiện tại để tránh khởi tạo trùng lặp:

- Khi lên mạng, ưu tiên làm nóng trước, nâng cao khả năng hiển thị của công cụ trên control panel
- Khi `hello`, vẫn tiếp tục làm dự phòng, tránh việc thiếu làm nóng gây ảnh hưởng đến nghiệp vụ

## Đồng thời cao và chống dội (debounce)

Duy trì trạng thái vòng đời theo từng thiết bị:

- `brokerOnline`
- `lastEventTs`
- `cleanupTimer`
- `cleanupVersion`

Quy tắc chống dội:

- Sự kiện có timestamp cũ sẽ bị bỏ qua trực tiếp
- `online` lặp lại sẽ không thông báo online lặp lại
- `offline` lặp lại chỉ làm mới cleanup timer, không thông báo offline lặp lại
- Khi callback của timer thực thi, kiểm tra `cleanupVersion` để tránh timer cũ xóa nhầm kết nối mới

## Các điểm cần sửa kèm theo

Do sau khi offline, transport sẽ được giữ lại trong thời gian ngắn, nên việc phân giải "transport đang online hiện tại" không thể chỉ dựa vào việc `ChatManager` có tồn tại hay không.

Cần cho `MqttUdpConn` expose trạng thái broker online, và để `ChatManager.GetTransportType()` trả về chuỗi rỗng khi MQTT transport đã offline. Nhờ vậy, việc truy vấn/gọi MCP theo chiều thiết bị vẫn phụ thuộc nghiêm ngặt vào "transport đang online hiện tại".

## Các file liên quan

- `internal/data/msg/message_types.go`
- `internal/app/mqtt_server/device_hook.go`
- `internal/app/mqtt_server/mqtt_server.go`
- `internal/app/server/mqtt_udp/mqtt_udp_adapter.go`
- `internal/app/server/mqtt_udp/mqtt_udp_conn.go`
- `internal/app/server/app.go`
- `internal/app/server/chat/chat.go`
