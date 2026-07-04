# Hướng dẫn sử dụng dịch vụ Milestones trên Windows

Chào mừng bạn đến với gói cài đặt Milestones Service (AIO) dành cho Windows. Tài liệu này bao gồm hướng dẫn khởi động, cấu hình và thông tin về cổng (port).

## Cấu trúc thư mục

```
milestones_server-windows-amd64-<version>/
├── milestones_server.exe          # Chương trình chính
├── onnxruntime.dll             # Thư viện phụ thuộc ONNX Runtime
├── sherpa-onnx-c-api.dll       # Thư viện phụ thuộc Sherpa-ONNX
├── sherpa-onnx-cxx-api.dll     # Thư viện phụ thuộc Sherpa-ONNX C++
├── ten_vad.dll                 # Thư viện phụ thuộc VAD (phát hiện giọng nói)
├── start.bat                   # Script khởi động
├── main_config.yaml            # File cấu hình chính
├── manager.json                # Cấu hình trang quản trị
├── asr_server.json             # Cấu hình dịch vụ ASR (nhận dạng giọng nói)
├── models/                     # Thư mục chứa các file mô hình
├── data/                       # Thư mục dữ liệu
└── logs/                       # Thư mục nhật ký (log)
```

## Khởi động nhanh

Nhấp đúp vào `start.bat` để khởi động dịch vụ. Sau khi khởi động, bạn có thể xem log tại thư mục `logs/`.

> Lưu ý: Trong lần khởi động đầu tiên, chương trình sẽ tự động tải về các file mô hình cần thiết (nếu thư mục models đang trống).

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

## Các vấn đề thường gặp

### Cảnh báo tường lửa (Firewall)

Khi chạy lần đầu, Windows có thể hiện cảnh báo tường lửa, vui lòng cho phép chương trình truy cập mạng.

### Cổng đã bị chiếm dụng

Nếu khởi động thất bại và báo cổng đã bị chiếm dụng, vui lòng:

1. Dùng lệnh `netstat -ano | findstr :số_cổng` để kiểm tra tiến trình đang chiếm cổng
2. Sửa số cổng trong file cấu hình
3. Hoặc kết thúc tiến trình đang chiếm cổng đó

### Thiếu file DLL

Nếu báo thiếu file DLL, hãy đảm bảo các file sau nằm cùng thư mục với `milestones_server.exe`:

- `onnxruntime.dll`
- `sherpa-onnx-c-api.dll`
- `sherpa-onnx-cxx-api.dll`
- `ten_vad.dll`

## Dừng dịch vụ

Trong cửa sổ khởi động, nhấn `Ctrl + C` hoặc đóng trực tiếp cửa sổ để dừng dịch vụ.
