# Hướng dẫn về file cấu hình

## Danh sách file cấu hình

- `config.json` - File cấu hình mặc định
- `config.dev.json` - Cấu hình môi trường phát triển (development)
- `config.prod.json` - Cấu hình môi trường production
- `config.example.json` - File cấu hình mẫu (ví dụ)

## Cấu trúc file cấu hình

```json
{
  "server": {
    "port": "8080", // Cổng server
    "mode": "debug" // Chế độ chạy: debug/release
  },
  "database": {
    "host": "localhost", // Host của database
    "port": "3306", // Cổng database
    "username": "root", // Tên đăng nhập database
    "password": "password", // Mật khẩu database
    "database": "milestones_admin" // Tên database
  },
  "jwt": {
    "secret": "your_secret_key", // Khóa bí mật (secret key) để ký JWT
    "expire_hour": 24 // Thời gian hết hạn của Token (tính bằng giờ)
  }
}
```

## Cách sử dụng

### 1. Tham số dòng lệnh (command line)

```bash
# Sử dụng file cấu hình mặc định
go run main.go

# Chỉ định file cấu hình
go run main.go -config=config/config.dev.json
go run main.go -c config/config.prod.json
```

### 2. Script khởi động

**Windows:**

```cmd
start.bat                    # Cấu hình mặc định
start.bat dev                # Môi trường phát triển
start.bat prod               # Môi trường production
start.bat custom my.json     # Cấu hình tùy chỉnh
start.bat help               # Hiển thị trợ giúp
```

**Linux/Mac:**

```bash
./start.sh                    # Cấu hình mặc định
./start.sh dev                # Môi trường phát triển
./start.sh prod               # Môi trường production
./start.sh custom my.json     # Cấu hình tùy chỉnh
./start.sh help               # Hiển thị trợ giúp
```

## Khuyến nghị cấu hình theo môi trường

### Môi trường phát triển (config.dev.json)

- Sử dụng chế độ debug
- Tên database nên thêm hậu tố `_dev`
- Khóa bí mật JWT có thể dùng chuỗi đơn giản
- Thời gian hết hạn Token có thể đặt dài hơn

### Môi trường production (config.prod.json)

- Sử dụng chế độ release
- Sử dụng database production riêng biệt
- Khóa bí mật JWT bắt buộc phải dùng mật khẩu mạnh
- Thời gian hết hạn Token nên đặt ngắn hơn
- Quyền của tài khoản database nên được giới hạn tối thiểu (nguyên tắc đặc quyền tối thiểu)

## Lưu ý về bảo mật

1. **Không được commit file cấu hình môi trường production vào hệ thống quản lý phiên bản (version control)**
2. **Khóa bí mật JWT phải được giữ kín và đủ phức tạp**
3. **Mật khẩu database nên được thay đổi định kỳ**
4. **Ở môi trường production, khuyến nghị sử dụng biến môi trường để ghi đè các cấu hình nhạy cảm**

## Thứ tự ưu tiên của file cấu hình

1. File cấu hình được chỉ định qua tham số dòng lệnh
2. File cấu hình mặc định (config.json)

## Xử lý sự cố (Troubleshooting)

### File cấu hình không tồn tại

```
Lỗi: không thể mở file cấu hình config/missing.json: no such file or directory
```

**Cách khắc phục**: Kiểm tra lại đường dẫn file cấu hình có chính xác không

### File cấu hình sai định dạng

```
Lỗi: phân tích (parse) file cấu hình thất bại config/config.json: invalid character '}' looking for beginning of object key string
```

**Cách khắc phục**: Kiểm tra lại định dạng JSON có đúng không, có thể dùng công cụ kiểm tra (validate) JSON để hỗ trợ

### Kết nối database thất bại

```
Lỗi: kết nối database thất bại: Error 1045: Access denied for user 'root'@'localhost'
```

**Cách khắc phục**: Kiểm tra lại thông tin cấu hình database có chính xác không
