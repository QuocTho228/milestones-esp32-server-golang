# Hướng dẫn triển khai gói khởi động nhanh (One-click Bundle)

## Tải xuống

Truy cập [trang Release](https://github.com/quoctho228/milestones-esp32-server-golang/releases) để tải về phiên bản phù hợp với nền tảng của bạn:

| Nền tảng | Tên file                             |
| -------- | ------------------------------------ |
| Windows  | `milestones-server-windows-xxx.zip`  |
| Linux    | `milestones-server-linux-xxx.tar.gz` |
| macOS    | `milestones-server-macos-xxx.tar.gz` |

---

## Giải nén và cấu trúc thư mục

Sau khi giải nén, cấu trúc thư mục sẽ như sau:

```
milestones-aio/
├── milestones_server          # Chương trình chính
├── config/                 # Thư mục chứa file cấu hình
├── models/                 # Thư mục chứa file model (nếu dùng ASR/TTS cục bộ)
└── data/                   # Thư mục dữ liệu
```

---

## Khởi động dịch vụ

### Windows

Nhấp đúp vào `start.bat`

### Linux

```bash
# Thư viện phụ thuộc runtime cho ten_vad
sudo apt install -y libc++1 libc++abi1

chmod +x milestones_server
LD_LIBRARY_PATH="$PWD/ten-vad/lib/Linux/x64:${LD_LIBRARY_PATH:-}" ./milestones_server
```

### macOS

```bash
chmod +x milestones_server
./build/macos/fix_rpath.sh ./milestones_server
./milestones_server
```

Nếu cấu trúc thư mục được giữ nguyên như sau:

```text
./milestones_server
./ten-vad/lib/macOS/ten_vad.framework
```

thì sau khi gói macOS đã được chạy qua `fix_rpath.sh`, mặc định bạn không cần thiết lập thủ công biến `DYLD_FRAMEWORK_PATH` nữa.

Nếu bạn đang debug từ thư mục tạm của IDE, hoặc đã di chuyển file thực thi khiến cấu trúc thư mục tương đối bị phá vỡ, có thể dùng cách dự phòng sau:

```bash
DYLD_FRAMEWORK_PATH="$PWD/ten-vad/lib/macOS" ./milestones_server
```

Nếu bạn tự đóng gói bản phân phối macOS trong repo mã nguồn, trước khi phát hành cần chạy thêm một lần lệnh sau:

```bash
./build/macos/fix_rpath.sh ./milestones_server
```

Bước này sẽ chỉnh sửa `rpath` bên trong file thực thi, từ đường dẫn mã nguồn trên máy dev sang `@executable_path/ten-vad/lib/macOS`, giúp gói phát hành chạy được ngay khi cấu trúc thư mục đúng.

---

## Bước tiếp theo

### 1. Truy cập bảng điều khiển Web

Mở trình duyệt và truy cập: **http://<IP hoặc domain của server>:8080**

<!-- Vị trí ảnh chụp màn hình: giao diện đăng nhập -->

> Hình: Giao diện đăng nhập bảng điều khiển Web

### 2. Cấu hình dịch vụ

Lần đầu sử dụng, vui lòng làm theo hướng dẫn cấu hình để hoàn tất thiết lập, xem chi tiết tại:

**[Hướng dẫn sử dụng trang quản trị →](manager_console_guide.md)**

---

## Dịch vụ nhận dạng giọng nói (Speaker Identification) (tuỳ chọn)

Chương trình đã tích hợp sẵn dịch vụ nhận dạng giọng nói (voice service).

---

## Câu hỏi thường gặp

### Q1: Sau khi khởi động không truy cập được bảng điều khiển Web?

Kiểm tra thiết lập tường lửa (firewall), đảm bảo cổng 8080 có thể truy cập được.

### Q2: Làm sao để khởi động lại dịch vụ?

Chỉ cần tắt chương trình rồi chạy lại. File cấu hình được lưu trong thư mục `config/`.

### Q3: Làm sao để xem log?

Console sẽ hiển thị log theo thời gian thực, nếu cần lưu lại có thể redirect ra file:

```bash
./milestones_server > server.log 2>&1
```
