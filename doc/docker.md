# Môi trường vận hành

#### 1. Triển khai funasr

Xem [tài liệu triển khai funasr bằng docker](https://github.com/modelscope/FunASR/blob/main/runtime/docs/SDK_advanced_guide_online_zh.md)

#### 2. Clone mã nguồn

> git clone 'https://github.com/quoctho228/milestones-esp32-server-golang'

#### 3. Cấu hình config/config.yaml, xem chi tiết tại [giải thích cấu hình config](config.md)

Các mục cần sửa chính như sau:

```yaml
# 1. Nhận dạng giọng nói asr
asr:
  provider: 'funasr'
  funasr:
    host: '127.0.0.1' # IP của dịch vụ websocket funasr đã triển khai
    port: '10096' # Cổng của websocket funasr đã triển khai
    mode: 'offline' # Chế độ, dùng offline là được
    # ...

# 2. tts
tts:
  provider: 'milestones' # Loại tts sử dụng, khuyến nghị doubao_ws, hoặc có thể chọn edge miễn phí
  doubao_ws:
    appid: '6886011847' # appid của bạn
    access_token: 'access_token' # access token của bạn
    cluster: 'volcano_tts'
    voice: 'zh_female_wanwanxiaohe_moon_bigtts' # Âm sắc, mặc định là giọng "Loan Loan Tiểu Hà"
    ws_host: 'openspeech.bytedance.com'
    use_stream: true
  edge:
    voice: 'zh-CN-XiaoxiaoNeural'
    rate: '+0%'
    volume: '+0%'
    pitch: '+0Hz'
    connect_timeout: 10
    receive_timeout: 60
  # ....

# 3. llm mô hình lớn
llm:
  provider: 'deepseek' # Nhà cung cấp, tương ứng với key bên dưới
  deepseek:
    type: 'openai' # Loại API tương thích ở phía server
    model_name: 'Pro/deepseek-ai/DeepSeek-V3' # Tên mô hình
    api_key: 'api_key' # api key
    base_url: 'https://api.siliconflow.cn/v1' # Địa chỉ API, mặc định dùng SiliconFlow
    max_tokens: 500
  # ...
```

#### 4. Khởi động docker

Tại thư mục gốc dự án, khởi động docker và mount thư mục config cùng các cổng (http/websocket: 8989, các cổng khác mount theo nhu cầu)

```
docker run -itd --name milestones_server -v $(pwd)/config:/workspace/config -p 8989:8989 quoctho228/milestones_server:latest

Nếu không kết nối được từ trong nước (Trung Quốc), dùng nguồn thay thế sau:

docker run -itd --name milestones_server -v $(pwd)/config:/workspace/config -p 8989:8989 docker.jsdelivr.fyi/quoctho228/milestones_server:latest
```

**Giải thích về hỗ trợ ten_vad:**

- Image Docker đã tự động bao gồm sẵn các file thư viện ten_vad, không cần mount thêm
- Nếu dùng ten_vad làm provider cho VAD, chỉ cần thiết lập `vad.provider: "ten_vad"` trong file cấu hình

Bây giờ bạn có thể kết nối tới

> ws://IP_máy:8989/milestones/v1/

để bắt đầu trò chuyện.

# Môi trường phát triển

```
docker run -itd --name milestones_server_golang -v $(pwd):/workspace/ -p 8989:8989 quoctho228/milestones_golang:0.1
Nếu không kết nối được từ trong nước (Trung Quốc), dùng nguồn thay thế sau:
docker run -itd --name milestones_server_golang -v $(pwd):/workspace/ -p 8989:8989 docker.jsdelivr.fyi/quoctho228/milestones_golang:0.1

go build -o milestones_server cmd/server/*.go
```

**Giải thích về ten_vad trong môi trường phát triển:**

- Image cho môi trường phát triển đã bao gồm sẵn các phụ thuộc biên dịch (compile) và runtime của ten_vad
- Nếu cần dùng ten_vad trong môi trường phát triển, hãy đảm bảo thư mục `lib/ten-vad` tồn tại tại thư mục gốc của dự án
- Khi biên dịch, hệ thống sẽ tự động dùng file header và thư viện của ten_vad
