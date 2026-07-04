#### Kết quả kiểm tra độ trễ

Có thể đạt mức phản hồi trong 1-1.3 giây; nếu dùng mô hình nhỏ hơn thì có thể còn nhanh hơn nữa.

asr: funasr
llm: API của Alibaba Cloud, qwen2.5-72b-instruct
tts: cosyvoice

```
time="2025-05-22 19:33:09.940" level=debug msg="从接收音频结束 asr->llm->tts首帧 整体 耗时: 1394 ms" caller="client.go:428"
time="2025-05-22 19:33:33.458" level=debug msg="从接收音频结束 asr->llm->tts首帧 整体 耗时: 1237 ms" caller="client.go:428"
time="2025-05-22 19:33:52.596" level=debug msg="从接收音频结束 asr->llm->tts首帧 整体 耗时: 1190 ms" caller="client.go:428"
time="2025-05-22 19:34:12.272" level=debug msg="从接收音频结束 asr->llm->tts首帧 整体 耗时: 1361 ms" caller="client.go:428"
time="2025-05-22 19:34:31.598" level=debug msg="从接收音频结束 asr->llm->tts首帧 整体 耗时: 1347 ms" caller="client.go:428"
time="2025-05-22 19:35:00.281" level=debug msg="从接收音频结束 asr->llm->tts首帧 整体 耗时: 1194 ms" caller="client.go:428"
time="2025-05-22 19:35:24.418" level=debug msg="从接收音频结束 asr->llm->tts首帧 整体 耗时: 975 ms" caller="client.go:428"
time="2025-05-22 19:35:49.868" level=debug msg="从接收音频结束 asr->llm->tts首帧 整体 耗时: 1150 ms" caller="client.go:428"
```

_(Ghi chú của người dịch: nội dung log trên giữ nguyên tiếng Trung do là log kỹ thuật gốc từ hệ thống; ý nghĩa dòng log là "Tổng thời gian từ lúc nhận âm thanh xong đến khi có khung TTS đầu tiên qua chuỗi asr->llm->tts".)_

---

## Kiểm thử trên trang quản trị

Cả gói khởi động nhanh (one-click) và triển khai bằng Docker đều đã tích hợp sẵn trang quản trị Web, cung cấp giao diện kiểm thử trực quan.

Hỗ trợ các loại kiểm thử sau:

| Loại kiểm thử | Mô tả                                                                    |
| ------------- | ------------------------------------------------------------------------ |
| VAD           | Kiểm tra kết nối và thời gian phản hồi của phát hiện hoạt động giọng nói |
| ASR           | Kiểm tra kết nối và độ trễ khung đầu tiên của nhận dạng giọng nói        |
| LLM           | Kiểm tra kết nối và độ trễ khung đầu tiên của suy luận mô hình lớn       |
| TTS           | Kiểm tra kết nối và độ trễ khung đầu tiên của tổng hợp giọng nói         |
| OTA           | Kiểm tra kết nối MQTT/UDP                                                |

Để biết cách sử dụng chi tiết, vui lòng xem: **[Hướng dẫn sử dụng trang quản trị →](manager_console_guide.md)**
