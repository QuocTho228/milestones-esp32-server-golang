# Hỗ trợ biên dịch cục bộ bằng Docker

Đã thêm mới file `docker-compose.local.yml`, hỗ trợ biên dịch cục bộ và triển khai đa kiến trúc (multi-architecture).

## File mới thêm

- `docker/docker-composer/docker-compose.local.yml` - File cấu hình biên dịch cục bộ

## Cách biên dịch

### Biên dịch mặc định (AMD64)

```bash
cd docker/docker-composer
docker-compose -f docker-compose.local.yml up --build
```

### Biên dịch ARM64 (Apple Silicon)

```bash
cd docker/docker-composer
TARGETARCH=arm64 docker-compose -f docker-compose.local.yml up --build
```

## Cách chạy

Sau khi biên dịch xong, các dịch vụ sẽ tự động khởi động, bao gồm:

- Server chính (cổng 8989)
- Backend quản trị (cổng 8081)
- Giao diện frontend (cổng 8080)
- Cơ sở dữ liệu MySQL (cổng 23306)

Truy cập http://<IP máy chủ hoặc domain>:8080 để xem giao diện frontend.

## 🏗️ Hỗ trợ đa kiến trúc

### Tự động phát hiện kiến trúc (khuyến nghị)

`docker-compose.local.yml` hỗ trợ tự động phát hiện kiến trúc của hệ thống hiện tại:

```bash
# Tự động phát hiện kiến trúc và build (hành vi mặc định)
docker-compose -f docker-compose.local.yml up --build
```

### Chỉ định kiến trúc thủ công

Nếu cần build cho một kiến trúc cụ thể:

```bash
# Build cho kiến trúc ARM64
TARGETARCH=arm64 docker-compose -f docker-compose.local.yml up --build

# Build cho kiến trúc AMD64
TARGETARCH=amd64 docker-compose -f docker-compose.local.yml up --build
```

### Các kiến trúc được hỗ trợ

- **AMD64/x86_64**: Bộ xử lý Intel/AMD (mặc định)
- **ARM64**: Apple Silicon (M1/M2), máy chủ ARM

## 📁 Giải thích các file cấu hình

### docker-compose.yml

Sử dụng image chính thức đã được build sẵn, phù hợp với môi trường production:

```yaml
services:
  mysql:
    image: docker.jsdelivr.fyi/mysql:8.0
  main-server:
    image: docker.jsdelivr.fyi/quoctho228/milestones_golang:0.1
  backend:
    image: docker.jsdelivr.fyi/quoctho228/milestones_backend:0.1
  frontend:
    image: docker.jsdelivr.fyi/quoctho228/milestones_frontend:0.1
```

### docker-compose.local.yml

Phiên bản build cục bộ, hỗ trợ sửa đổi code và đa kiến trúc:

```yaml
services:
  main-server:
    build:
      context: ../..
      dockerfile: docker/Dockerfile.main
      args:
        TARGETARCH: ${TARGETARCH:-amd64}
```

## 🔧 Cấu hình biến môi trường

### Liên quan đến kiến trúc

| Tên biến     | Giá trị mặc định | Ghi chú                      |
| ------------ | ---------------- | ---------------------------- |
| `TARGETARCH` | `amd64`          | Kiến trúc đích (amd64/arm64) |

## 🛠️ Các thao tác thường dùng

### Xem trạng thái dịch vụ

```bash
# Xem trạng thái tất cả các dịch vụ
docker-compose ps

# Xem log dịch vụ
docker-compose logs -f main-server
docker-compose logs -f backend
docker-compose logs -f frontend
```

### Dừng và khởi động lại dịch vụ

```bash
# Dừng tất cả các dịch vụ
docker-compose down

# Khởi động lại một dịch vụ cụ thể
docker-compose restart main-server

# Build lại và khởi động
docker-compose up --build
```
