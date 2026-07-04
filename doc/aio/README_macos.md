# Hướng dẫn sử dụng dịch vụ Milestones trên macOS

Chào mừng bạn đến với gói cài đặt Milestones Service (AIO) dành cho macOS. Tài liệu này bao gồm hướng dẫn cài đặt các thư viện phụ thuộc, khởi động dịch vụ và cấu hình hệ thống.

## Cấu trúc thư mục

```
milestones_server-macos-<arch>-<version>/
├── milestones_server              # Chương trình chính
├── ten-vad/
│   └── lib/macOS/
│       ├── ten_vad.framework/  # Framework VAD (phát hiện giọng nói)
│       ├── libonnxruntime.*.dylib
│       └── libsherpa-onnx-*.dylib
├── main_config.yaml            # File cấu hình chính
├── manager.json                # Cấu hình trang quản trị
├── asr_server.json             # Cấu hình dịch vụ ASR (nhận dạng giọng nói)
├── models/                     # Thư mục chứa các file mô hình
├── data/                       # Thư mục dữ liệu
└── logs/                       # Thư mục nhật ký (log)
```

> **Lưu ý**: Phiên bản macOS được chia thành hai loại **amd64** (Intel) và **arm64** (Apple Silicon), vui lòng tải đúng phiên bản phù hợp với dòng máy Mac của bạn.

## Yêu cầu vận hành

### Yêu cầu hệ thống

- **Phiên bản macOS**: macOS 11 (Big Sur) trở lên
- **Kiến trúc**: Intel (x86_64) hoặc Apple Silicon (arm64)

### Cài đặt các thư viện phụ thuộc

Sử dụng Homebrew để cài đặt các thư viện cần thiết:

```bash
# Cài đặt Homebrew (nếu chưa có)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Cài đặt thư viện phụ thuộc
brew install pkg-config
```

## Khởi động nhanh

```bash
# Cấp quyền thực thi
chmod +x milestones_server

# Nếu đây là bản build bạn tự đóng gói, hãy sửa rpath trước
./build/macos/fix_rpath.sh ./milestones_server

# Khởi động dịch vụ
./milestones_server
```

Ghi chú:

- Đối với gói phát hành chính thức đã được đóng gói sẵn, thông thường không cần chạy lại `fix_rpath.sh`
- Chỉ khi bạn tự build gói phân phối macOS từ mã nguồn thì mới cần thực hiện thêm bước này
- Bước này sẽ thay đổi đường dẫn tuyệt đối (`rpath`) của máy phát triển bên trong file thực thi thành `@executable_path/ten-vad/lib/macOS`

### Cảnh báo bảo mật khi chạy lần đầu

Khi chạy lần đầu, macOS có thể hiện cảnh báo bảo mật vì chương trình chưa được Apple xác thực (chứng thực chữ ký). Vui lòng:

1. Mở "Cài đặt hệ thống" (System Settings) → "Quyền riêng tư & Bảo mật" (Privacy & Security)
2. Tìm mục cảnh báo liên quan đến `milestones_server`
3. Nhấn "Vẫn mở" (Open Anyway) hoặc "Cho phép" (Allow)

Hoặc dùng lệnh sau để gỡ bỏ thuộc tính cách ly (quarantine):

```bash
xattr -cr milestones_server
```

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
- **Truy cập trong mạng LAN**: `http://<IP máy này>:8080/`

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

## Chạy nền (background)

### Sử dụng nohup

```bash
nohup ./milestones_server > logs/output.log 2>&1 &
```

### Tạo dịch vụ launchd (khuyến nghị)

Tạo file `~/Library/LaunchAgents/com.milestones.server.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.milestones.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>/path/to/milestones_server</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/path/to/milestones_server-macos-<arch>-<version></string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/path/to/logs/output.log</string>
    <key>StandardErrorPath</key>
    <string>/path/to/logs/error.log</string>
</dict>
</plist>
```

Nạp và chạy dịch vụ:

```bash
# Nạp dịch vụ
launchctl load ~/Library/LaunchAgents/com.milestones.server.plist

# Khởi động dịch vụ
launchctl start com.milestones.server

# Xem trạng thái
launchctl list | grep milestones

# Dừng dịch vụ
launchctl stop com.milestones.server

# Gỡ dịch vụ
launchctl unload ~/Library/LaunchAgents/com.milestones.server.plist
```

## Cấu hình tường lửa (Firewall)

Nếu máy có bật tường lửa, cần cho phép `milestones_server` nhận kết nối vào (inbound):

1. Mở "Cài đặt hệ thống" (System Settings) → "Mạng" (Network) → "Tường lửa" (Firewall)
2. Nhấn "Tùy chọn" (Options)
3. Tìm `milestones_server`, thiết lập thành "Cho phép kết nối vào" (Allow incoming connections)

Hoặc dùng lệnh trong Terminal:

```bash
# Thêm ngoại lệ tường lửa (cần quyền sudo)
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add /path/to/milestones_server
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblock /path/to/milestones_server
```

## Các vấn đề thường gặp

### Cảnh báo "đã bị hỏng" (đã bị lỗi/damaged)

Nếu bị báo ứng dụng đã bị hỏng, hãy chạy lệnh sau:

```bash
xattr -cr milestones_server
```

### Lỗi tải thư viện động (dylib)

Nếu gặp lỗi tải `dylib` thất bại, hãy kiểm tra:

```bash
# Xem các thư viện phụ thuộc
otool -L milestones_server

# Xem rpath
otool -l milestones_server | grep -A2 LC_RPATH

# Đảm bảo các thư viện động nằm đúng vị trí
ls -la ten-vad/lib/macOS/
```

Nếu `LC_RPATH` vẫn là đường dẫn tuyệt đối của máy phát triển (không phải `@executable_path/ten-vad/lib/macOS`), hãy chạy:

```bash
./build/macos/fix_rpath.sh ./milestones_server
```

Nếu bạn đang debug trong thư mục tạm của IDE, hoặc đã di chuyển file thực thi khiến cấu trúc thư mục không còn khớp, có thể tạm thời dùng:

```bash
DYLD_FRAMEWORK_PATH="$PWD/ten-vad/lib/macOS" ./milestones_server
```

### Cổng đã bị chiếm dụng

```bash
# Kiểm tra cổng đang được sử dụng
lsof -i :số_cổng

# Kết thúc tiến trình đang chiếm cổng hoặc sửa cổng trong file cấu hình
```

### Chạy phiên bản Intel trên Apple Silicon (M1/M2/M3)

Để chạy phiên bản Intel trên máy Apple Silicon, cần có Rosetta 2:

```bash
# Cài đặt Rosetta 2
softwareupdate --install-rosetta
```

Tuy nhiên, khuyến nghị bạn nên tải phiên bản arm64 tương ứng để có hiệu năng tốt nhất.
