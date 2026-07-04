# Hướng dẫn triển khai bằng Docker Compose

## Tổng quan

Dự án này sử dụng Docker Compose để triển khai dạng container hóa, bao gồm các dịch vụ cốt lõi sau:

- **Dịch vụ cơ sở dữ liệu MySQL**: lưu trữ dữ liệu
- **Dịch vụ chương trình chính**: xử lý logic nghiệp vụ cốt lõi
- **Dịch vụ backend quản trị**: dịch vụ API
- **Dịch vụ frontend quản trị**: giao diện quản trị web

## Hướng dẫn nhanh (bổ sung)

Phần này là bổ sung cho `doc/docker.md`, giúp bạn nhanh chóng chọn và triển khai theo cách phù hợp.

### 1. Chọn phương án triển khai

- Khuyến nghị: Docker Compose (bao gồm trang quản trị và đầy đủ các dịch vụ)
- Đơn giản hóa: Docker đơn container (không có trang quản trị hoặc ở chế độ tối giản)

### 2. Lộ trình nhanh với Docker Compose

1. Clone mã nguồn hoặc chuẩn bị sẵn file `docker-compose.yml`
2. Tham khảo phần "Chuẩn bị file cấu hình" và "Khởi động dịch vụ" ở phần sau của tài liệu này để hoàn tất cấu hình
3. Khởi động:

```bash
docker compose up -d
```

4. Địa chỉ mặc định của trang quản trị: `http://<IP máy chủ hoặc tên miền>:8080/`

### 3. Docker đơn container (bổ sung)

Sau khi build hoặc pull image theo `doc/docker.md`, chạy container. Một số khuyến nghị thường gặp:

- Mount các thư mục `config/`, `logs/`, `storage/` thành volume dữ liệu
- Mở các cổng WebSocket / MQTT / UDP ra bên ngoài
- Khi cần dùng trang quản trị, hãy bật tham số tương ứng hoặc dùng Compose

### 4. Wizard cấu hình và kiểm thử

Sau khi khởi động, có thể dùng wizard cấu hình trên trang quản trị để hoàn thành cấu hình engine, đồng thời dùng công cụ kiểm thử để kiểm tra khả năng dùng được và độ trễ của VAD/ASR/LLM/TTS, cũng như xác thực toàn bộ quy trình OTA.

### 5. Các vấn đề thường gặp

- Xung đột cổng: kiểm tra xem các cổng 8080/8989/2883/8990 có đang bị chiếm dụng không
- Cấu hình không có hiệu lực: xác nhận đường dẫn mount volume đã đúng, khởi động lại container để áp dụng
- Vấn đề về quyền: trên Linux cần lưu ý quyền của thư mục mount và các giới hạn của SELinux

## Kiến trúc dịch vụ

### 1. Dịch vụ cơ sở dữ liệu MySQL (milestones-mysql)

**Thông tin cấu hình:**

- Image: `docker.jsdelivr.fyi/mysql:8.0`
- Port mapping: `23306:3306`
- Tên database: `milestones_admin`
- Username: `root`
- Password: `password`

**Đặc điểm:**

- Sử dụng MySQL 8.0
- Có cấu hình health check
- Dữ liệu được lưu trữ bền vững (persistent)

### 2. Dịch vụ chương trình chính (milestones-main-server)

**Thông tin cấu hình:**

- Image: `docker.jsdelivr.fyi/quoctho228/milestones_server:0.5`
- Port mapping:
  - `8989:8989` - Dịch vụ WebSocket
  - `2882:2883` - Dịch vụ MQTT
  - `8888:8888/udp` - Dịch vụ UDP

**Quan hệ phụ thuộc:**

- Phụ thuộc vào trạng thái health của dịch vụ MySQL
- Phụ thuộc vào việc dịch vụ backend đã khởi động xong

**Hỗ trợ file cấu hình:**

- Import file cấu hình tùy chỉnh thông qua mount volume
- Đường dẫn file cấu hình: `../../config:/workspace/config`

**Hỗ trợ ten_vad:**

- Image Docker đã bao gồm sẵn thư viện ten_vad (`/workspace/lib/ten-vad/`)
- Đường dẫn thư viện runtime đã được tự động cấu hình thông qua `LD_LIBRARY_PATH`

### 3. Dịch vụ backend quản trị (milestones-backend)

**Thông tin cấu hình:**

- Image: `docker.jsdelivr.fyi/quoctho228/milestones_manager_backend:0.5`
- Port mapping: `8081:8080`

**Chức năng:**

- Cung cấp RESTful API
- Quản lý thiết bị và người dùng

**Hỗ trợ file cấu hình:**

- Import file cấu hình tùy chỉnh thông qua mount volume
- Đường dẫn file cấu hình: `../../manager/backend/config:/root/config`

### 4. Dịch vụ frontend quản trị (milestones-frontend)

**Thông tin cấu hình:**

- Image: `docker.jsdelivr.fyi/quoctho228/milestones_manager_frontend:0.5`
- Port mapping: `8080:80`

**Chức năng:**

- Giao diện quản trị Web (cổng vào quản lý nội bộ)
- Quản lý trạng thái thiết bị và cấu hình hệ thống

## Quy trình triển khai

### 1. Chuẩn bị môi trường

Đảm bảo hệ thống đã cài đặt Docker và Docker Compose:

```bash
docker --version
docker compose version
```

### 2. Chuẩn bị file cấu hình

Đảm bảo các thư mục và file sau đã tồn tại:

```
milestones-esp32-server-golang/
├─ docker/docker-composer/
│  └─ docker-compose.yml
├─ config/
│  ├─ config.yaml
│  ├─ config.json
│  └─ (các file cấu hình khác)
├─ logs/
│  └─ (thư mục log)
└─ manager/backend/config/
   ├─ config.yaml
   └─ (các file cấu hình khác)
```

**Giải thích về việc import file cấu hình:**

- File cấu hình của chương trình chính được import thông qua mount volume `../../config:/workspace/config`
- File cấu hình của backend được import thông qua mount volume `../../manager/backend/config:/root/config`

### 3. Khởi động dịch vụ

**Bắt buộc phải vào thư mục `docker/docker-composer/` trước khi chạy lệnh:**

```bash
cd docker/docker-composer/
docker compose up -d

docker compose ps
docker compose logs -f
```

### 4. Truy cập dịch vụ

- Giao diện quản trị frontend: `http://<IP máy chủ hoặc tên miền>:8080`
- API backend: `http://localhost:8081`
- WebSocket: `ws://localhost:8989`
- MQTT: `localhost:2882`
- UDP: `localhost:8888`
- MySQL: `localhost:23306`

## Các thao tác thường dùng

```bash
cd docker/docker-composer/

docker compose ps

docker compose logs

docker compose logs -f main-server

docker compose restart

docker compose down

docker compose down -v

docker compose pull

docker compose up -d
```

## Cấu hình mạng

Dự án sử dụng mạng tùy chỉnh `milestones-network`:

- MySQL: `mysql:3306`
- Backend: `backend:8080`
- Frontend: `frontend:80`
- Chương trình chính: `main-server:8989` (WebSocket) / `main-server:2883` (MQTT) / `main-server:8888` (UDP)

**Tổng hợp port mapping:**

- 8080 → Giao diện quản trị frontend
- 8081 → API backend
- 8989 → WebSocket
- 2882 → MQTT
- 8888 → UDP
- 23306 → MySQL

## Lưu trữ dữ liệu bền vững

### Dữ liệu MySQL

Được lưu trữ bền vững thông qua Docker volume `mysql_data`, dữ liệu không bị mất khi container khởi động lại.

### File cấu hình

- Cấu hình chương trình chính: `../../config:/workspace/config`
- Cấu hình backend: `../../manager/backend/config:/root/config`

Sau khi sửa cấu hình, cần khởi động lại dịch vụ tương ứng để áp dụng:

```bash
cd docker/docker-composer/
docker compose restart main-server

docker compose restart backend
```

### File log

- Log chương trình chính: `../../logs:/workspace/logs`

## Cách import file cấu hình

### 1. Cấu hình chương trình chính

**Vị trí:**

```
milestones-esp32-server-golang/config/
├─ config.yaml
├─ config.json
├─ mqtt_config.json
└─ (các file cấu hình khác)
```

**Import:**

1. Đặt file cấu hình vào thư mục `config/`
2. Sau khi khởi động, sẽ tự động được mount vào container tại `/workspace/config/`
3. Sau khi sửa, khởi động lại dịch vụ chương trình chính:

```bash
cd docker/docker-composer/
docker compose restart main-server
```

### 2. Cấu hình backend quản trị

**Vị trí:**

```
milestones-esp32-server-golang/manager/backend/config/
├─ config.yaml
└─ (các file cấu hình khác)
```

**Import:**

1. Đặt file cấu hình vào thư mục `manager/backend/config/`
2. Sau khi khởi động, sẽ tự động được mount vào container tại `/root/config/`
3. Sau khi sửa, khởi động lại dịch vụ backend:

```bash
cd docker/docker-composer/
docker compose restart backend
```

### 3. File thư viện ten_vad

**Giải thích:**

- Image Docker đã bao gồm sẵn thư viện ten_vad (`/workspace/lib/ten-vad/`)
- Đường dẫn thư viện runtime đã được tự động cấu hình thông qua `LD_LIBRARY_PATH`
- Khi dùng ten_vad, không cần mount thêm gì

## Health Check

Dịch vụ MySQL được cấu hình health check như sau:

```yaml
healthcheck:
  test: ['CMD', 'mysqladmin', 'ping', '-h', 'localhost', '-u', 'root', '-ppassword']
  timeout: 20s
  retries: 10
  interval: 10s
  start_period: 30s
```

## Xử lý sự cố

### 1. Dịch vụ khởi động thất bại

```bash
cd docker/docker-composer/

docker compose logs [tên_dịch_vụ]

# Kiểm tra cổng bị chiếm dụng (Linux)
netstat -tulpn | grep [cổng]
```

### 2. Kết nối database thất bại

```bash
cd docker/docker-composer/

docker compose ps mysql

docker compose logs mysql

docker compose exec mysql mysql -u root -ppassword
```

### 3. Vấn đề kết nối mạng

```bash
cd docker/docker-composer/

docker network ls
docker network inspect milestones-network

docker compose exec main-server ping mysql
```

## Khuyến nghị tối ưu hiệu năng

1. Ở môi trường production, hãy thiết lập giới hạn tài nguyên (resource limit) cho từng dịch vụ
2. Cấu hình xoay vòng (rotation) log để tránh log phình quá to
3. Định kỳ backup dữ liệu MySQL
4. Tích hợp hệ thống giám sát (monitoring)

## Lưu ý về bảo mật

1. Ở môi trường production, hãy đổi mật khẩu database mặc định
2. Chỉ mở cổng theo nhu cầu thực tế
3. Cấu hình tường lửa và kiểm soát truy cập
4. Sử dụng nguồn image đáng tin cậy

---

## Bước tiếp theo

### Truy cập trang quản trị

Sau khi dịch vụ đã khởi động, truy cập http://<IP máy chủ hoặc tên miền>:8080 để vào trang quản trị.

**[Hướng dẫn sử dụng trang quản trị →](manager_console_guide.md)**

### Cấu hình thiết bị ESP32

Tham khảo [Hướng dẫn kết nối thiết bị ESP32](esp32_milestones_backend_guide.md) để hoàn tất việc kết nối thiết bị.
