# Dịch vụ Mock ASR/LLM/TTS bên ngoài độc lập (không sửa chương trình chính)

Phương án này cung cấp một tiến trình dịch vụ mock **chạy độc lập**, dùng để thay thế các dịch vụ cloud ASR/LLM/TTS thực tế khi thực hiện kiểm thử tải (load test/stress test).

## 1. Khởi động

```bash
go run ./cmd/mock_ai_server \
  -addr :18080 \
  -asr-text "Xin chào, đây là kết quả nhận dạng mock khi stress test" \
  -llm-reply "Đây là phản hồi mock của llm" \
  -tts-mode silence
```

Kiểm tra sức khỏe (health check):

```bash
curl http://127.0.0.1:18080/healthz
```

## 2. Các giao diện được cung cấp

- `ws://127.0.0.1:18080/asr/`
  - Tương thích với input dạng ws kiểu FunASR (nhận các frame nhị phân âm thanh)
  - Sau khi nhận được `{"is_speaking": false}` sẽ trả về kết quả nhận dạng cuối cùng

- `POST http://127.0.0.1:18080/v1/chat/completions`
  - Giao diện tương thích với OpenAI Chat Completions
  - Hỗ trợ `stream=false/true`

- `POST http://127.0.0.1:18080/v1/audio/speech`
  - Giao diện tương thích với OpenAI TTS
  - Trả về `audio/wav` (dạng im lặng hoặc tiếng bíp)

## 3. Đề xuất cấu hình chương trình chính (chỉ sửa cấu hình, không sửa code)

### ASR (FunASR)

- `host=127.0.0.1`
- `port=18080`
- Đường dẫn giao thức theo triển khai hiện tại sử dụng `ws://host:port/`, nếu tầng cấu hình của bạn yêu cầu có path, vui lòng dùng `/asr/`.

> Nếu adapter ASR hiện tại của bạn phụ thuộc chặt (strongly depend) vào path gốc `ws://host:port/`, cũng có thể ở tầng gateway chuyển tiếp (forward) `/` sang `/asr/`.

### LLM (Tương thích OpenAI)

- Chọn provider là `eino` (`type=openai`)
- `base_url=http://127.0.0.1:18080/v1`
- `api_key` bất kỳ giá trị nào khác rỗng
- `model_name` bất kỳ giá trị nào (ví dụ `mock-gpt`)

### TTS (Tương thích OpenAI)

- Chọn provider là `openai`
- `api_url=http://127.0.0.1:18080/v1/audio/speech`
- `response_format=wav`
- `api_key` bất kỳ giá trị nào khác rỗng

## 4. Các tham số có thể điều chỉnh

```bash
-asr-delay-ms         # Độ trễ trả kết quả cuối cùng của ASR
-llm-first-delay-ms   # Độ trễ token đầu tiên của LLM
-llm-chunk-delay-ms   # Độ trễ giữa các chunk khi LLM stream
-tts-first-delay-ms   # Độ trễ gói tin đầu tiên của TTS
-tts-mode             # silence|beep (im lặng|tiếng bíp)
-tts-duration-ms      # Thời lượng audio trả về
```

## 5. Đề xuất khi kiểm thử tải (stress test)

1. Trước tiên chạy thử với một kết nối đơn tại local để xác nhận (đảm bảo thiết bị có thể đi hết toàn bộ luồng liên kết và nhận được âm thanh).
2. Sau đó dùng `ws_multi` để tăng dần số lượng kết nối song song (ví dụ 50/100/200/500).
3. Sử dụng các tổ hợp delay khác nhau để mô phỏng biến động thực tế của các dependency bên ngoài, quan sát P95/P99 và tỷ lệ lỗi.

## 6. Đánh giá về việc có cần tối ưu/thay đổi `ws_multi` hay không

Kết luận: **Khuyến nghị tối ưu ở mức nhỏ, không bắt buộc phải tái cấu trúc lớn**. Hiện tại có thể dùng trực tiếp để stress test, nhưng để đo lường "hiệu năng của dịch vụ chính" một cách chân thực hơn thay vì "nút thắt cổ chai (bottleneck) của client stress test", nên bổ sung các năng lực sau:

1. **Bổ sung chế độ phát lại âm thanh thuần túy (khuyến nghị ưu tiên)**
   - Cách làm phổ biến hiện nay là chạy TTS local trước rồi mới đẩy âm thanh, điều này sẽ khiến thời gian xử lý TTS của client bị trộn lẫn vào kết quả đo.
   - Khuyến nghị thêm `-audio_file`/`-audio_dir`, gửi trực tiếp các frame opus đã mã hóa sẵn hoặc wav đã chuyển sang opus.

2. **Xuất kết quả thống kê độ trễ theo cấu trúc**
   - Bổ sung thống kê RT (response time) của frame đầu tiên, RT hoàn tất toàn bộ luồng, phân loại mã lỗi.
   - Khuyến nghị xuất định dạng JSONL, thuận tiện cho việc xử lý và tổng hợp P95/P99 sau này.

3. **Kiểm soát điều tiết (throttle) kết nối và gửi dữ liệu**
   - Bổ sung tính năng tạo kết nối theo lô (ví dụ mỗi giây khởi tạo N client), tránh việc tạo kết nối đồng loạt tức thời làm phóng đại độ rung (jitter) phía client.
   - Bổ sung tham số jitter khi gửi gói tin, để mô phỏng mạng thực tế của thiết bị.

4. **Chính sách thử lại và timeout khi thất bại có thể cấu hình**
   - Ví dụ `-dial_timeout`, `-read_timeout`, `-retry`, nâng cao độ ổn định khi stress test dài hạn.

5. **Thu thập chỉ số tài nguyên (tùy chọn)**
   - Ghi lại CPU/bộ nhớ của bản thân client, thuận tiện để phân biệt "nút thắt phía server" và "nút thắt phía máy chạy stress test".

Trong phương án "dịch vụ mock độc lập" của bạn, `ws_multi` **không sửa vẫn có thể chạy**, nhưng khuyến nghị nên làm ít nhất mục 1 và mục 2, để kết luận stress test đáng tin cậy hơn rõ rệt.
