# 🚀 milestones-esp32-server-golang

> **Backend AI Milestones cho thiết bị ESP32**

---

## Giới thiệu dự án | Project Overview

milestones-esp32-server-golang là một dịch vụ backend AI hiệu năng cao, hỗ trợ xử lý streaming toàn trình (full-streaming), được thiết kế chuyên biệt cho các kịch bản IoT và tương tác thoại thông minh. Dự án được phát triển bằng ngôn ngữ Go, tích hợp các năng lực lõi như ASR (nhận dạng giọng nói tự động), LLM (mô hình ngôn ngữ lớn), TTS (tổng hợp giọng nói), hỗ trợ xử lý đồng thời (concurrency) quy mô lớn và kết nối đa giao thức, giúp các thiết bị đầu cuối thông minh và thiết bị biên (edge devices) thực hiện tương tác thoại AI.

---

## ✨ Tính năng chính | Key Features

- ⚡ **Chuỗi thoại AI full-streaming end-to-end**: Toàn bộ quy trình ASR → LLM → TTS đều xử lý streaming, tương tác thời gian thực với độ trễ thấp
- 🎙️ **Nhận dạng vân giọng và chuyển đổi TTS động**: Tự động chuyển đổi âm sắc TTS theo danh tính người nói, mang lại trải nghiệm giọng nói cá nhân hóa
- 🔌 **Trừu tượng hóa tầng giao diện Transport**: Trừu tượng hóa thống nhất WebSocket / MQTT UDP, có thể linh hoạt tiêm (inject) logic nghiệp vụ chính, thuận tiện mở rộng giao thức
- 📬 **Xử lý theo hàng đợi tin nhắn (message queue)**: LLM và TTS sử dụng hàng đợi tin nhắn để xử lý bất đồng bộ, hỗ trợ tiêm linh hoạt logic nghiệp vụ
- 🌐 **Kết nối đa giao thức, đồng thời cao**: Hỗ trợ kết nối và đẩy tin nhắn cho số lượng lớn thiết bị đồng thời
- ♻️ **Pool tài nguyên và tái sử dụng kết nối hiệu quả**: Cơ chế connection pool cho tài nguyên bên ngoài, giảm thời gian phản hồi, nâng cao thông lượng hệ thống
- 🤖 **Tích hợp năng lực AI đa engine**: Dựa trên framework Eino, hỗ trợ nhiều engine như FunASR, tương thích OpenAI, Ollama, Doubao, EdgeTTS, CosyVoice...
- 🧩 **Kiến trúc module hóa, dễ mở rộng**: Các module lõi VAD/ASR/LLM/TTS/MCP/thị giác... độc lập và có thể cắm-rút (pluggable)
- 🎵 **MCP Audio Server**: Lấy tài nguyên âm thanh theo trang (pagination) và xử lý streaming, phát nhạc và điều khiển âm lượng
- 🦞 **Tích hợp agent OpenClaw**: Sinh endpoint OpenClaw riêng theo từng agent, hỗ trợ xem trạng thái kết nối, kiểm thử phiên (session), định tuyến theo từ khóa vào/ra (mặc định là "打开龙虾/进入龙虾" để bật và "关闭龙虾/退出龙虾" để tắt)
- 🖥️ **Bảng điều khiển quản trị Web đầy đủ tính năng**: Trình hướng dẫn cấu hình trực quan, kiểm thử khả dụng toàn chuỗi VAD/ASR/LLM/TTS, quản lý thiết bị và tiêm tin nhắn, giám sát độ trễ thời gian thực và xác thực OTA
- 🧠 **Tính năng nghiệp vụ nâng cao**: Tổng hợp và nhập MCP Marketplace, nhân bản giọng nói (voice clone), knowledge base (Dify/RAGFlow/WeKnora), debug gọi MCP từ xa theo chiều thiết bị/agent
- 📦 **Giải pháp triển khai một-cú-nhấp dễ dùng**: Gói aio đã biên dịch sẵn, dùng ngay (chương trình chính + control panel + dịch vụ vân giọng), triển khai Docker một-cú-nhấp, hỗ trợ biên dịch cục bộ trên Linux/Windows/macOS
- 🔐 **Hệ thống bảo mật và phân quyền** (đang lên kế hoạch): Đã dự trù interface xác thực người dùng và quản lý quyền

---

[Phân tích kiến trúc trên deepwiki](https://deepwiki.com/quoctho228/milestones-esp32-server-golang)

## 🚀 Bắt đầu nhanh | Quick Start

### Cách 1: Gói khởi động một-cú-nhấp (khuyến nghị)

Tải gói nén phù hợp với nền tảng của bạn, giải nén rồi chạy trực tiếp:

- **Trang Release**: <https://github.com/quoctho228/milestones-esp32-server-golang/releases>
- **Hướng dẫn sử dụng**: [doc/quickstart_bundle_tutorial.md](doc/quickstart_bundle_tutorial.md)

Sau khi khởi động, truy cập **http://<IP máy chủ hoặc domain>:8080** để vào Web Console và cấu hình.

### Cách 2: Triển khai bằng Docker

- [Docker Compose (kèm control panel)](doc/docker_compose.md)
- [Docker (không kèm control panel)](doc/docker.md)

### Cách 3: Biên dịch cục bộ

Phù hợp với môi trường phát triển hoặc các trường hợp cần tùy biến khi biên dịch.

**Cài đặt phụ thuộc** (lấy ví dụ trên Ubuntu)

```bash
# Go 1.24+
# Bộ codec Opus
sudo apt-get install -y pkg-config libopus0 libopusfile-dev

# ONNX Runtime (1.21.0)
wget https://github.com/microsoft/onnxruntime/releases/download/v1.21.0/onnxruntime-linux-x64-1.21.0.tgz
tar -xzf onnxruntime-linux-x64-1.21.0.tgz
sudo cp -r onnxruntime-linux-x64-1.21.0/include/* /usr/local/include/onnxruntime/
sudo cp -r onnxruntime-linux-x64-1.21.0/lib/* /usr/local/lib/
sudo ldconfig

# Các phụ thuộc runtime của ten_vad
sudo apt install -y libc++1 libc++abi1
```

> 📖 Mô tả đầy đủ về phụ thuộc và cấu hình cho Windows/macOS, xem tại [config.md](doc/config.md)

Quy trình biên dịch tách rời cho chương trình chính, frontend/backend của control panel, dịch vụ vân giọng, và đóng gói AIO, xem tại [doc/compile_deploy.md](doc/compile_deploy.md)

Tham khảo [tài liệu chính thức của FunASR](https://github.com/modelscope/FunASR/blob/main/runtime/docs/SDK_advanced_guide_online_zh.md) để triển khai.

**Biên dịch và khởi động**

```bash
# Biên dịch
go build -o milestones_server ./cmd/server/

# Khởi động (chi tiết file cấu hình xem tại config/config.yaml)
./milestones_server -c config/config.yaml
```

---

## 📚 Điều hướng tài liệu | Docs

### Liên quan đến triển khai

- [Hướng dẫn gói khởi động một-cú-nhấp](doc/quickstart_bundle_tutorial.md)
- [Triển khai Docker Compose](doc/docker_compose.md)
- [Triển khai Docker](doc/docker.md)
- [Hướng dẫn biên dịch và triển khai](doc/compile_deploy.md)
- [Giải thích chi tiết cấu hình](doc/config.md)

### Hướng dẫn sử dụng

- [Hướng dẫn sử dụng trang quản trị](doc/manager_console_guide.md)
- [Dịch vụ WebSocket và cấu hình OTA](doc/websocket_server.md)
- [Cấu hình MQTT + UDP](doc/mqtt_udp.md)
- [Giao thức MQTT UDP](doc/mqtt_udp_protocol.md)

### Các module chức năng

- [Năng lực thị giác](doc/vision.md)
- [Nhận dạng vân giọng](doc/speaker_identification.md)
- [Kiến trúc MCP](doc/mcp.md)
- [Tài nguyên âm thanh MCP](doc/mcp_resource.md)
- [MCP Marketplace (khám phá/nhập/cập nhật nóng)](doc/mcp_market.md)
- [Tích hợp agent OpenClaw (Endpoint/định tuyến từ khóa/kiểm thử phiên)](doc/openclaw_integration.md)
- [Nhân bản giọng nói (thao tác người dùng và hạn mức quản trị viên)](doc/voice_clone.md)
- [Knowledge base (cấu hình provider/đồng bộ/kiểm thử truy hồi/RAG)](doc/knowledge_base.md)
- [Gọi MCP từ xa theo chiều thiết bị/agent (Endpoint/Tools/Call)](doc/mcp_remote_call_agent_device.md)

### Kết nối thiết bị

- [Hướng dẫn kết nối backend Milestones cho ESP32](doc/esp32_milestones_backend_guide.md)
- [Giải thích xác thực OTA MQTT](doc/ota_mqtt_auth.md)

---

## 🧩 Tổng quan kiến trúc module | Module Overview

| Module               | Mô tả chức năng                                                                                              | Công nghệ sử dụng                                                                                    |
| -------------------- | ------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------- |
| VAD                  | Phát hiện hoạt động giọng nói (Voice Activity Detection)                                                     | Silero VAD / WebRTC VAD / ten_vad                                                                    |
| ASR                  | Nhận dạng giọng nói                                                                                          | FunASR / Doubao ASR                                                                                  |
| LLM                  | Suy luận mô hình lớn                                                                                         | Tương thích framework Eino, OpenAI, Ollama, v.v.                                                     |
| TTS                  | Tổng hợp giọng nói                                                                                           | Doubao / EdgeTTS / CosyVoice                                                                         |
| MCP                  | Kết nối đa giao thức, khám phá/nhập MCP Marketplace, debug gọi từ xa theo chiều thiết bị/agent               | MCP Server / điểm kết nối / MCP Market / SSE / StreamableHTTP / WebSocket Controller / MCP Tool Call |
| OpenClaw             | Điểm kết nối theo chiều agent, chuyển đổi chế độ bằng từ khóa vào/ra, chuyển tiếp và kiểm thử tin nhắn phiên | OpenClaw WebSocket / Agent Endpoint / Chat Router                                                    |
| Thị giác             | Xử lý thị giác                                                                                               | Doubao / Thị giác Alibaba Cloud                                                                      |
| Nhận dạng vân giọng  | Nhận dạng người nói                                                                                          | sherpa-onnx + cơ sở dữ liệu vector                                                                   |
| Nhân bản giọng nói   | Tạo và nghe thử âm sắc nhân bản phía người dùng                                                              | Minimax / CosyVoice / Qwen                                                                           |
| Knowledge base (RAG) | Đồng bộ tài liệu, kiểm thử truy hồi và tìm kiếm trong hội thoại                                              | Dify / RAGFlow / WeKnora                                                                             |

---

## 📈 Hiệu năng & Kiểm thử | Performance & Testing

- [Báo cáo kiểm thử độ trễ](doc/delay_test.md)
- Trang quản trị cung cấp cổng kiểm thử khả dụng và độ trễ cho VAD/ASR/LLM/TTS

---

## 🛠️ Đang lên kế hoạch | Roadmap

- AI chủ động (proactive AI)

---

## 🤝 Đóng góp | Contributing

Hoan nghênh gửi Issue, PR hoặc góp ý!

---

## 📄 Giấy phép

Giấy phép MIT

---

> © 2026 milestones-esp32-server-golang
