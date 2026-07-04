# Hướng dẫn sử dụng dịch vụ Milestones trên Linux

Chào mừng bạn đến với gói cài đặt Milestones Service (AIO) dành cho Linux. Tài liệu này bao gồm hướng dẫn cài đặt các thư viện phụ thuộc, khởi động dịch vụ và cấu hình hệ thống.

## Cấu trúc thư mục

```
milestones_server-linux-amd64-<version>/
├── milestones_server              # Chương trình chính
├── ten-vad/
│   └── lib/Linux/x64/
│       ├── libten_vad.so       # Thư viện phụ thuộc VAD (phát hiện giọng nói)
│       ├── libsherpa-onnx-c-api.so
│       ├── libsherpa-onnx-cxx-api.so
│       └── libonnxruntime.so   # Thư viện phụ thuộc ONNX Runtime
├── main_config.yaml            # File cấu hình chính
├── manager.json                # Cấu hình trang quản trị
├── asr_server.json             # Cấu hình dịch vụ ASR (nhận dạng giọng nói)
├── models/                     # Thư mục chứa các file mô hình
├── data/                       # Thư mục dữ liệu
└── logs/                       # Thư mục nhật ký (log)
```

## Yêu cầu vận hành

### Yêu cầu hệ thống

| Hệ điều hành  | Phiên bản tối thiểu | Trạng thái kiểm thử                   |
| ------------- | ------------------- | ------------------------------------- |
| Ubuntu        | 18.04 LTS           | ✅ Đã kiểm thử                        |
| Debian        | 10 (Buster)         | ⚠️ Dự kiến tương thích, chưa kiểm thử |
| CentOS / RHEL | 8                   | ⚠️ Dự kiến tương thích, chưa kiểm thử |

**Yêu cầu về môi trường chạy**:

- **Kiến trúc**: x86_64 (amd64)

### Cài đặt các thư viện phụ thuộc

#### Debian / Ubuntu

```bash
sudo apt update
sudo apt install -y libc++1 libc++abi1
```

#### CentOS / RHEL / Fedora

```bash
sudo dnf install -y libcxx libcxxabi
# hoặc
sudo yum install -y libcxx libcxxabi
```

#### Các bản phân phối khác

Vui lòng cài đặt các gói tương ứng với những thư viện sau:

- `libc++.so.1` — Thư viện chuẩn C++ của LLVM
- `libc++abi.so.1` — Thư viện ABI C++ của LLVM

## Khởi động nhanh

```bash
# Cấp quyền thực thi
chmod +x milestones_server

# Khởi động dịch vụ
./milestones_server
```

### Chạy nền (background)

Sử dụng nohup:

```bash
nohup ./milestones_server > logs/output.log 2>&1 &
```

Hoặc sử dụng systemd (khuyến nghị cho môi trường production), xem phần bên dưới.

## Cổng (Port) và dịch vụ

| Cổng     | Nguồn cấu hình                        | Mô tả                                                 |
| -------- | ------------------------------------- | ----------------------------------------------------- |
| **8080** | `manager.json` → `server.port`        | **Trang quản trị**: Web console + HTTP API            |
| **8989** | `main_config.yaml` → `websocket.port` | **WebSocket dịch vụ chính**: kết nối thiết bị/client  |
| **9000** | `asr_server.json` → `server.port`     | **Dịch vụ ASR/nhận dạng giọng nói**: giao diện nội bộ |
| **2883** | Cấu hình qua console                  | **Dịch vụ MQTT**: kết nối MQTT của thiết bị           |
| **8990** | Cấu hình qua console                  | **Dịch vụ UDP**: giao tiếp UDP của thiết bị           |
| **6060** | Cấu hình qua console                  | **pprof**: phân tích hiệu năng (mặc định tắt)         |

## Địa chỉ truy cập

### Trang quản trị

- **Truy cập nội bộ (local)**: `http://localhost:8080/`
- **Truy cập trong mạng LAN**: `http://<IP máy chủ>:8080/`

### Kết nối thiết bị/client

- **WebSocket**: `ws://<IP máy chủ>:8989/`
- **MQTT**: `<IP máy chủ>:2883`
- **UDP**: `<IP máy chủ>:8990`

## Thay đổi cấu hình

### Các cổng cần sửa trực tiếp trong file cấu hình

Sau khi sửa các cổng dưới đây, cần khởi động lại dịch vụ để có hiệu lực:

| Cổng | File cấu hình      | Mục cấu hình     |
| ---- | ------------------ | ---------------- |
| 8080 | `manager.json`     | `server.port`    |
| 8989 | `main_config.yaml` | `websocket.port` |
| 9000 | `asr_server.json`  | `server.port`    |

### Cấu hình qua trang quản trị (console)

Các cổng dưới đây cùng toàn bộ cấu hình khác đều được thay đổi thông qua trang quản trị:

- **Cấu hình cổng**: MQTT (2883), UDP (8990), pprof (6060)
- **Cấu hình chức năng**: LLM, TTS, ASR, nhận dạng giọng nói (声纹识别), v.v.
- Truy cập `http://localhost:8080/` để vào trang quản trị
- Thay đổi cấu hình có hiệu lực ngay lập tức, không cần khởi động lại dịch vụ

## Triển khai môi trường production (systemd)

Tạo file dịch vụ `/etc/systemd/system/milestones.service`:

```ini
[Unit]
Description=Milestones Server
After=network.target

[Service]
Type=simple
User=YOUR_USER
WorkingDirectory=/path/to/milestones_server-linux-amd64
ExecStart=/path/to/milestones_server-linux-amd64/milestones_server
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

Khởi động dịch vụ:

```bash
# Nạp lại cấu hình systemd
sudo systemctl daemon-reload

# Bật tự khởi động cùng hệ thống
sudo systemctl enable milestones

# Khởi động dịch vụ
sudo systemctl start milestones

# Xem trạng thái
sudo systemctl status milestones

# Xem log
sudo journalctl -u milestones -f
```

## Cấu hình tường lửa (Firewall)

Nếu máy chủ có bật tường lửa, cần mở các cổng tương ứng:

```bash
# Ubuntu/Debian (ufw)
sudo ufw allow 8080/tcp  # Trang quản trị
sudo ufw allow 8989/tcp  # WebSocket
sudo ufw allow 2883/tcp  # MQTT
sudo ufw allow 8990/udp  # UDP

# CentOS/RHEL (firewalld)
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=8989/tcp
sudo firewall-cmd --permanent --add-port=2883/tcp
sudo firewall-cmd --permanent --add-port=8990/udp
sudo firewall-cmd --reload
```

## Các vấn đề thường gặp

### Báo thiếu thư viện dùng chung (shared library)

Dùng lệnh `ldd` để kiểm tra thư viện bị thiếu:

```bash
ldd milestones_server
ldd ten-vad/lib/Linux/x64/libten_vad.so
```

Dựa vào kết quả trả về để cài đặt gói hệ thống tương ứng.

### Phiên bản glibc quá thấp

Nếu xuất hiện lỗi `version 'GLIBC_2.xx' not found`, có nghĩa là phiên bản glibc của hệ thống quá cũ. Khuyến nghị:

- Nâng cấp hệ điều hành lên phiên bản mới hơn
- Hoặc chạy chương trình bằng container Docker

### Cổng đã bị chiếm dụng

```bash
# Kiểm tra cổng đang được sử dụng
sudo lsof -i :số_cổng
# hoặc
sudo netstat -tulpn | grep số_cổng

# Sửa cổng trong file cấu hình hoặc kết thúc tiến trình đang chiếm cổng đó
```
