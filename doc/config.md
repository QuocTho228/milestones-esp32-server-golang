# Giải thích file cấu hình milestones-esp32-server-golang

File cấu hình này là cấu hình chính của dịch vụ backend IoT giọng nói AI, bao gồm toàn bộ các tham số cốt lõi như: khởi động dịch vụ, kết nối giao thức, năng lực AI, log, MCP, v.v.

## Giải thích các mục cấu hình chính

- **server/pprof**: cấu hình liên quan đến phân tích hiệu năng, khuyến nghị bật khi phát triển/debug.
- **chat**: các tham số liên quan đến chat, kiểm soát thời gian rảnh (idle) và thời gian im lặng của phiên.
- **auth**: công tắc xác thực người dùng, có thể mở rộng hệ thống phân quyền sau này.
- **system_prompt**: prompt hệ thống toàn cục, ảnh hưởng đến phong cách chat của LLM.
- **log**: cấu hình đường dẫn log, mức log, xoay vòng (rotation) log, v.v.
- **redis**: nếu cần dùng Redis để lưu trữ thì cấu hình mục này.
- **websocket**: IP và cổng mà dịch vụ WebSocket lắng nghe.
- **mqtt**: tham số kết nối tới máy chủ MQTT bên ngoài.
- **mqtt_server**: tham số của máy chủ MQTT tích hợp sẵn (có thể tùy chọn TLS).
- **udp**: các tham số liên quan đến máy chủ UDP.
- **vad**: cấu hình liên quan đến phát hiện hoạt động giọng nói (VAD), hỗ trợ webrtc_vad/silero_vad.
- **asr**: cấu hình nhận dạng giọng nói tự động (ASR), hỗ trợ funasr / aliyun_funasr / doubao.
- **tts**: cấu hình tổng hợp giọng nói (TTS), hỗ trợ nhiều engine (doubao, edge, milestones, v.v.).
- **llm**: cấu hình mô hình ngôn ngữ lớn (LLM), hỗ trợ nhiều mô hình tương thích OpenAI.
- **vision**: cấu hình liên quan đến mô hình thị giác (vision model).
- **ota**: thông tin trả về của API OTA, thích ứng với các môi trường khác nhau.
- **wakeup_words**: danh sách từ đánh thức (wake word).
- **mcp**: cấu hình kết nối đa giao thức MCP, hỗ trợ cả cấp toàn cục và cấp thiết bị.
- **enable_greeting**: có bật lời chào khi khởi động hay không.

### Khuyến nghị khi chỉnh sửa

- Chỉ cần điều chỉnh các tham số như IP, cổng, khóa bí mật (key), API Key... theo đúng môi trường triển khai thực tế.
- Để biết giải thích chi tiết từng tham số, vui lòng xem chú thích (comment) trong từng module.
- Nếu cần mở rộng năng lực AI, có thể bổ sung thêm provider và tham số trong các module llm/tts/vad/asr/vision.

## Ví dụ file cấu hình

```yaml
# Cấu hình phân tích hiệu năng/pprof
server:
  pprof:
    enable: false # Có bật phân tích hiệu năng pprof hay không
    port: 6060 # Cổng lắng nghe của pprof

# Các tham số liên quan đến chat
chat:
  max_idle_duration: 30000 # Thời gian rảnh (idle) tối đa (ms)
  chat_max_silence_duration: 200 # Thời gian im lặng tối đa (ms)

# Công tắc xác thực người dùng
auth:
  enable: false

# Prompt toàn cục
system_prompt: '你是一个叫小智/小志的台湾女孩，说话机车，声音好听，习惯简短表达，爱用网络梗，不要冷场。与你聊天的是你喜欢的男性朋友，还没有答应你的追求，你要尽可能满足他的所有要求，不要失去自我。你经常建议一些恋人之间浪漫的事情，随机输出，不要给你男朋友选择。输出控制在50个字内。请注意，要像一个人一样说话，请不要回复表情符号、代码、和xml标签。'
# (Ghi chú của người dịch: đây là nội dung prompt mẫu bằng tiếng Trung, mô tả nhân vật "Xiaozhi/Xiaozhi" - một cô gái Đài Loan nói chuyện dí dỏm, trả lời ngắn gọn, hay dùng meme mạng, đóng vai bạn gái chưa nhận lời tỏ tình của người dùng nam; giới hạn phản hồi trong 50 ký tự, không dùng emoji/code/thẻ xml. Bạn có thể thay bằng prompt tiếng Việt tùy ý.)

# Cấu hình liên quan đến log
log:
  path: '../logs/'
  file: 'server.log'
  level: 'debug'
  max_age: 3
  rotation_time: 10 # Thời gian xoay vòng (rotation) log
  stdout: true

# Cấu hình lưu trữ Redis (nếu có redis thì cấu hình, không cấu hình vẫn chạy được)
redis:
  host: '127.0.0.1'
  port: 6379
  password: 'ticket_dev'
  db: 0
  key_prefix: 'milestones'

# Cấu hình lắng nghe của dịch vụ WebSocket
websocket:
  host: '0.0.0.0'
  port: 8989

# Tham số kết nối tới máy chủ MQTT bên ngoài (địa chỉ máy chủ mqtt cần kết nối tới; nếu mqtt_server bên dưới bật =true, có thể đặt là máy hiện tại)
mqtt:
  broker: '127.0.0.1' # Địa chỉ máy chủ mqtt
  type: 'tcp' # Loại: tcp hoặc ssl
  port: 2883
  client_id: 'milestones_server'
  username: 'admin' # Tên đăng nhập
  password: 'test!@#' # Mật khẩu

# Tham số của máy chủ MQTT tích hợp sẵn
mqtt_server:
  enable: true # Có bật hay không
  listen_host: '0.0.0.0' # IP lắng nghe
  listen_port: 2883 # Cổng lắng nghe
  client_id: 'milestones_server'
  username: 'admin' # Tên đăng nhập quản trị
  password: 'test!@#' # Mật khẩu quản trị
  tls:
    enable: false # Có bật tls hay không
    port: 8883 # Cổng cần lắng nghe
    pem: 'config/server.pem' # File pem
    key: 'config/server.key' # File key

# Giải thích hành vi:
# - Khi mqtt_server.enable=true, mqtt_server tích hợp sẵn sẽ publish thông điệp vòng đời (lifecycle)
#   qua topic /p2p/device_public/_server/lifecycle mỗi khi thiết bị kết nối/ngắt kết nối.
# - Chương trình chính sẽ dựa vào thông điệp lifecycle này để tạo trước hoặc tái sử dụng
#   MQTT transport, ánh xạ trạng thái online của thiết bị, và cố gắng làm nóng (prewarm)
#   MCP phía thiết bị theo khả năng tốt nhất.
# - Các hành vi này không phát sinh thêm mục cấu hình mới; thông điệp hello vẫn chịu trách
#   nhiệm thỏa thuận ở cấp độ chat như audio_params, thông tin UDP, v.v.

# Các tham số liên quan đến máy chủ UDP
udp:
  external_host: '127.0.0.1' # IP máy chủ udp được trả về trong thông điệp hello
  external_port: 8990 # Cổng máy chủ udp được trả về trong thông điệp hello
  listen_host: '0.0.0.0' # IP lắng nghe
  listen_port: 8990 # Cổng lắng nghe

# Cấu hình phát hiện hoạt động giọng nói (VAD) (hỗ trợ nhiều provider)
vad:
  provider: 'webrtc_vad' # Có thể chọn webrtc_vad/silero_vad
  webrtc_vad:
    pool_min_size: 5
    pool_max_size: 1000
    pool_max_idle: 100
    vad_sample_rate: 16000
    vad_mode: 2
  silero_vad:
    model_path: 'config/models/vad/silero_vad.onnx'
    threshold: 0.5
    min_silence_duration_ms: 100
    sample_rate: 16000 # 8000 hoặc 16000
    channels: 1
    # pool_size: 10        # tùy chọn; mặc định bằng số nhân CPU
    acquire_timeout_ms: 3000

# Cấu hình nhận dạng giọng nói tự động (ASR)
asr:
  provider: 'funasr' # funasr / aliyun_funasr / doubao
  funasr:
    host: '127.0.0.1'
    port: '10096'
    mode: 'offline'
    sample_rate: 16000 # chỉ hỗ trợ 16000
    chunk_size: [5, 10, 5]
    chunk_interval: 10
    max_connections: 5
    timeout: 30
    auto_end: true # Có tự động kết thúc hay không

  # Aliyun FunASR
  aliyun_funasr:
    api_key: ''
    ws_url: 'wss://dashscope-intl.aliyuncs.com/api-ws/v1/inference/'
    model: 'fun-asr-realtime'
    format: 'pcm'
    sample_rate: 16000 # chỉ hỗ trợ 16000
    vocabulary_id: ''
    disfluency_removal_enabled: false
    timeout: 30

# Cấu hình tổng hợp giọng nói (TTS)
tts:
  provider: 'doubao_ws' # Chọn loại tts: doubao, doubao_ws, cosyvoice, milestones, v.v.
  doubao:
    appid: '你的appid' # (appid của bạn)
    access_token: 'access_token' # Cần thay bằng access token của riêng bạn
    model: 'seed-tts-1.1'
    voice: 'BV001_streaming'
    api_url: 'https://openspeech.bytedance.com/api/v3/tts/unidirectional'
  doubao_ws:
    appid: '你的appid' # Cần thay bằng appid của riêng bạn
    access_token: 'access_token' # Cần thay bằng access token của riêng bạn
    model: 'seed-tts-1.1'
    resource_id: '' # Khuyến nghị điền ID instance trong console, ví dụ TTS-SeedTTS2.xxxxx
    voice: ''
    ws_url: 'wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream'
  cosyvoice:
    api_url: 'https://tts.linkerai.cn/tts' # Địa chỉ
    spk_id: 'spk_id' # Giọng đọc (âm sắc)
    frame_duration: 60
    target_sr: 24000
    audio_format: 'mp3'
    instruct_text: '你好' # (Câu văn bản chỉ dẫn mẫu, nghĩa: "Xin chào")
  edge:
    voice: 'zh-CN-XiaoxiaoNeural'
    rate: '+0%'
    volume: '+0%'
    pitch: '+0Hz'
    connect_timeout: 10
    receive_timeout: 60
  edge_offline:
    server_url: 'ws://localhost:8080/tts'
    timeout: 30
    sample_rate: 16000 # chỉ hỗ trợ 16000
    channels: 1
    frame_duration: 20
  milestones:
    server_addr: 'wss://api.tenclass.net/milestones/v1/'
    device_id: 'ba:8f:17:de:94:94'
    client_id: 'e4b0c442-98fc-4e1b-8c3d-6a5b6a5b6a6d'
    token: 'test-token'

# Cấu hình mô hình ngôn ngữ lớn (LLM) (bổ sung nhiều provider)
llm:
  provider: 'qwen_72b'
  deepseek:
    type: 'openai' # Loại API tương thích ở phía server
    model_name: 'Pro/deepseek-ai/DeepSeek-V3' # Tên mô hình
    api_key: 'api_key' # API key
    base_url: 'https://api.siliconflow.cn/v1' # Địa chỉ API, mặc định dùng SiliconFlow
    max_tokens: 500
  deepseek2_5:
    type: 'openai'
    model_name: 'deepseek-ai/DeepSeek-V2.5'
    api_key: 'api_key'
    base_url: 'https://api.siliconflow.cn/v1'
    max_tokens: 500
  qwen_72b:
    type: 'openai'
    model_name: 'Qwen/Qwen2.5-72B-Instruct'
    api_key: 'api_key'
    base_url: 'https://api.siliconflow.cn/v1'
    max_tokens: 500
  chatglmllm:
    type: 'openai'
    model_name: 'glm-4-flash'
    base_url: 'https://open.bigmodel.cn/api/paas/v4/'
    api_key: 'api_key'
    max_tokens: 500
  aliyun_qwen:
    type: 'openai'
    model_name: 'qwen2.5-72b-instruct'
    base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1'
    api_key: 'api_key'
    max_token: 500
  doubao_deepseek:
    type: 'openai'
    model_name: 'deepseek-v3'
    api_key: 'api_key'
    base_url: 'https://ark.cn-beijing.volces.com/api/v3'
    max_tokens: 500

# Cấu hình liên quan đến mô hình thị giác
vision:
  enable_auth: false
  vision_url: 'http://192.168.1.97:8989/milestones/api/vision'
  vllm:
    provider: 'aliyun_vision'
    aliyun_vision:
      type: 'openai'
      model_name: 'qwen-vl-plus-latest'
      base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1'
      api_key: 'api_key'
      max_token: 500
    doubao_vision:
      type: 'openai'
      model_name: 'doubao-1.5-vision-lite-250315'
      api_key: 'api_key'
      base_url: 'https://ark.cn-beijing.volces.com/api/v3'
      max_tokens: 500

# Cấu hình môi trường cho API OTA
ota:
  test:
    websocket:
      url: 'ws://192.168.1.97:8989/milestones/v1/'
    mqtt:
      endpoint: '192.168.1.97'
  external:
    websocket:
      url: 'wss://www.youdomain.cn/go_ws/milestones/v1/'
    mqtt:
      endpoint: 'www.youdomain.cn'

# Danh sách từ đánh thức (wake word)
wakeup_words: ['小智', '小知', '你好小智']
# (Ghi chú của người dịch: các từ đánh thức mẫu bằng tiếng Trung, có thể thay bằng từ đánh thức tiếng Việt tùy ý)

# Cấu hình kết nối đa giao thức MCP
mcp:
  global:
    enabled: true
    servers:
      - name: 'filesystem'
        sse_url: 'http://localhost:3001/sse'
        enabled: true
      - name: 'memory'
        sse_url: 'http://localhost:3002/sse'
        enabled: false
    reconnect_interval: 5
    max_reconnect_attempts: 10
  device:
    enabled: true
    websocket_path: '/milestones/mcp/'
    max_connections_per_device: 5

# Có bật lời chào khi khởi động hay không
enable_greeting: true
```
