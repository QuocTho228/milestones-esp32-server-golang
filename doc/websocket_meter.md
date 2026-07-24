### Kiểm thử tải (Load Testing / Stress Test)

```
root@quoctho228-System-Product-Name:~# docker run -itd --name websocket_meter docker.jsdelivr.fyi/quoctho228/milestones_websocket_client
87311584e5fef592f32e0b7d7062d9053e956d5e0d50edb220370ff37d2293ac
root@quoctho228-System-Product-Name:~#
root@quoctho228-System-Product-Name:~# docker exec -it websocket_meter /bin/bash
root@87311584e5fe:/workspace#
root@87311584e5fe:/workspace# ./ws_multi  -h
Usage of ./ws_multi:
  -count int
        Số lượng client (default 10)
  -device string
        ID thiết bị
  -server string
        Địa chỉ server (default "ws://localhost:8989/milestones/v1/")
  -text string
        Nội dung chat, nhiều câu cách nhau bằng dấu phẩy sẽ được gửi lần lượt (default "你好")
root@87311584e5fe:/workspace# ./ws_multi -count 1 -server wss://joeyzhou.chat/ws/milestones/v1/ -text "你好,在干什么,一起出去玩吧"
Đang chạy client Milestones
Server: wss://joeyzhou.chat/ws/milestones/v1/
Số lượng client: 1
Nội dung gửi: 你好,在干什么,一起出去玩吧
2025-05-27 09:54:51.095 [info] [audio_utils.go:199] Thời gian tới frame đầu tiên của TTS trên cloud: 532 ms
2025-05-27 09:54:51.098 [info] [audio_utils.go:269] TTS cloud -> hoàn tất giải mã frame đầu tiên: 535 ms
2025-05-27 09:54:51.401 [info] [cosyvoice.go:306] Thời gian TTS: từ lúc nhập đến khi lấy xong dữ liệu MP3: 838 ms
2025-05-27 09:54:51.748 [info] [audio_utils.go:199] Thời gian tới frame đầu tiên của TTS trên cloud: 344 ms
2025-05-27 09:54:51.752 [info] [audio_utils.go:269] TTS cloud -> hoàn tất giải mã frame đầu tiên: 347 ms
2025-05-27 09:54:51.901 [info] [cosyvoice.go:306] Thời gian TTS: từ lúc nhập đến khi lấy xong dữ liệu MP3: 497 ms
2025-05-27 09:54:52.292 [info] [audio_utils.go:199] Thời gian tới frame đầu tiên của TTS trên cloud: 387 ms
2025-05-27 09:54:52.296 [info] [audio_utils.go:269] TTS cloud -> hoàn tất giải mã frame đầu tiên: 391 ms
2025-05-27 09:54:52.628 [info] [cosyvoice.go:306] Thời gian TTS: từ lúc nhập đến khi lấy xong dữ liệu MP3: 723 ms
Client 0 bắt đầu chạy
Client 0 đã kết nối tới server: wss://joeyzhou.chat/ws/milestones/v1/
Nhận được message: {Type:hello Text: State: SessionID:cafd2800-1979-06d5-19cf-b8bf53bb55dc Transport:websocket AudioFormat:<nil>}
Đã gửi frame Opus: 20
Đã gửi frame Opus: 50
Đã gửi frame Opus: 59
```

#### Giải thích tổng quan

    1. Chương trình sẽ dựa trên văn bản người dùng nhập vào, gọi interface TTS để sinh dữ liệu âm thanh, sau đó lần lượt gửi đến server
    2. Việc tính thời gian (đo hiệu năng) bắt đầu tính từ khi type: listen, state: stop, cho đến khi nhận được frame âm thanh đầu tiên từ server thì dừng lại

#### Giải thích tham số

    -count: Số lượng kết nối đồng thời (concurrency)
    -device: Mặc định sẽ tự sinh ngẫu nhiên deviceId, nếu dùng tham số này để chỉ định thiết bị thì -count bắt buộc phải là 1
    -server: Địa chỉ WebSocket server
    -text: Nội dung cần gửi, cách nhau bằng dấu ",", sẽ gửi lặp lại theo vòng lặp

#### Giải thích đầu ra (output)

    Có thể redirect output ra file log, sau đó dùng lệnh tail -f xx.log | grep 'thời gian phản hồi trung bình'
