# Phương án Mock để kiểm thử tải (pressure test) toàn chuỗi VAD/ASR/LLM/TTS (chờ xác nhận)

> Mục tiêu: trên tiền đề không gọi đến các dịch vụ ASR/LLM/TTS thật (mất phí), vẫn giữ nguyên hành vi của toàn chuỗi WebSocket hiện có, hỗ trợ kiểm thử tải đồng thời cao, tiêm độ trễ (latency) có kiểm soát, và thống kê có thể quan sát được.

## 1. Mục tiêu thiết kế

1. **Chuỗi xử lý đầy đủ**: Giữ nguyên luồng chính "đầu vào âm thanh từ thiết bị -> VAD -> ASR -> LLM -> TTS -> gửi âm thanh xuống thiết bị".
2. **Không tốn chi phí bên ngoài**: ASR/LLM/TTS đều trả về dữ liệu mock cục bộ, không truy cập dịch vụ cloud bên thứ ba.
3. **Ít can thiệp**: Mở rộng provider `mock` dựa trên cơ chế factory provider hiện có, cố gắng không thay đổi luồng nghiệp vụ chính.
4. **Có thể kiểm thử tải, có thể tái lập**: Hỗ trợ trả về cố định, trả về theo template, tiêm lỗi theo xác suất, tiêm độ trễ theo cấu hình.
5. **Có thể đối chiếu với dịch vụ thật**: Thông qua việc chuyển đổi cấu hình, về sau có thể khôi phục provider thật bất cứ lúc nào để so sánh hiệu năng.

## 2. Phương án tổng thể

Áp dụng phương án **Mock ở cấp Provider + tái sử dụng client kiểm thử tải**:

- Thêm mới ba provider:
  - `asr/mock`
  - `llm/mock`
  - `tts/mock`
- Thêm mục cấu hình tương ứng trong cấu hình phía backend (`type=asr|llm|tts`, `provider=mock`).
- Gắn cấu hình mock theo role/agent, để thực hiện mock toàn chuỗi trong phạm vi một session.
- Phía kiểm thử tải tiếp tục dùng công cụ kiểm thử tải websocket hiện có (`ws_multi`) để bơm âm thanh đồng thời.

Cách này đảm bảo được:

- Giao thức WebSocket, state machine của session, logic sắp xếp tin nhắn đều đi qua đường code thật.
- Chỉ thay thế việc gọi đến dịch vụ cloud bên ngoài, chi phí thấp nhất, rủi ro nhỏ nhất.

## 3. Thiết kế hành vi Mock

### 3.1 ASR Mock

Đầu vào: luồng frame âm thanh (giữ nguyên interface hiện có).
Đầu ra: văn bản nhận dạng (cố định/luân phiên/theo quy tắc).

Đề xuất cấu hình:

- `mode`: `fixed` | `sequence` | `echo_hint`
- `fixed_text`: trả về cố định, ví dụ "Xin chào, đây là văn bản kiểm thử tải"
- `sequence_texts`: mảng văn bản, luân phiên theo request
- `first_token_delay_ms`: mô phỏng độ trễ của gói tin đầu tiên
- `final_delay_ms`: mô phỏng độ trễ của gói tin kết thúc
- `error_rate`: xác suất 0~1 để tiêm lỗi nhận dạng thất bại

### 3.2 LLM Mock

Đầu vào: văn bản ASR + tin nhắn ngữ cảnh (context).
Đầu ra: văn bản trả lời (có thể mang theo thông tin độ dài ngữ cảnh).

Đề xuất cấu hình:

- `mode`: `fixed` | `template` | `echo`
- `fixed_answer`: câu trả lời cố định
- `template`: template, ví dụ `"Đã nhận: {{input}}"`
- `first_token_delay_ms`: độ trễ token đầu tiên
- `stream_chunk_chars`: số ký tự mỗi đoạn khi trả về streaming
- `total_delay_ms`: mô phỏng tổng thời gian hoàn thành
- `error_rate`: xác suất thất bại

### 3.3 TTS Mock

Đầu vào: văn bản từ LLM.
Đầu ra: frame Opus/PCM có thể phát được (đề xuất ưu tiên Opus, tương thích với chuỗi xử lý hiện tại).

Đề xuất cấu hình:

- `audio_source`: `builtin_silence` | `builtin_beep` | `file`
- `file_path`: đường dẫn âm thanh đặt sẵn (wav/opus cục bộ)
- `frame_duration_ms`: độ dài mỗi frame khi chia (ví dụ 20ms)
- `first_frame_delay_ms`: độ trễ frame đầu tiên
- `inter_frame_delay_ms`: độ trễ giữa các frame
- `error_rate`: xác suất thất bại

> Để giảm độ phức tạp, phiên bản đầu tiên đề xuất: trước tiên trả về "frame im lặng + độ trễ cố định", sau đó mới bổ sung "phát lại beep/file".

## 4. Ma trận kịch bản kiểm thử tải

### Kịch bản A: Chuỗi thành công thuần túy (baseline)

- ASR trả văn bản cố định
- LLM trả câu trả lời ngắn cố định
- TTS trả frame im lặng
- Mục tiêu: đo mức đồng thời ổn định tối đa, thời gian phản hồi (RT) trung bình, P95/P99

### Kịch bản B: Chuỗi độ trễ cao

- ASR/LLM/TTS lần lượt được tiêm độ trễ 100~500ms
- Mục tiêu: đo ngưỡng timeout, tình trạng dồn ứ hàng đợi (queue backlog)

### Kịch bản C: Chuỗi tiêm lỗi

- Đặt error_rate là 1%/5%/10%
- Mục tiêu: đo khả năng phục hồi sau lỗi, độ ổn định kết nối, chiến lược retry

### Kịch bản D: Chuỗi văn bản dài

- LLM xuất ra văn bản siêu dài (ví dụ 500~1500 chữ)
- Mục tiêu: đo việc chia frame của TTS, backpressure khi gửi và độ ổn định bộ nhớ

## 5. Chỉ số và tiêu chí nghiệm thu (đề xuất)

Chỉ số cốt lõi:

- Tỷ lệ thành công của session (trả về được giọng nói thành công)
- Độ trễ frame đầu tiên end-to-end (từ khi listen stop -> gói âm thanh đầu tiên)
- Độ trễ hoàn thành end-to-end (từ khi listen stop -> tts finish)
- Số session đang hoạt động mỗi giây / mức đồng thời đỉnh
- Tỷ lệ lỗi (phân theo từng giai đoạn ASR/LLM/TTS)
- Tài nguyên dịch vụ: CPU, bộ nhớ, số lượng Goroutine, số lần GC

Đề xuất nghiệm thu (có thể điều chỉnh về sau):

- Tỷ lệ thành công >= 99%
- Ở mức đồng thời mục tiêu, độ trễ frame đầu tiên P95 < 1.5s
- Chạy liên tục 30 phút không có hiện tượng rò rỉ bộ nhớ rõ rệt (biến động RSS trong tầm kiểm soát)

## 6. Các bước thực hiện (chia làm hai giai đoạn)

### Giai đoạn 1 (khả dụng tối thiểu, 1~2 ngày)

1. Thêm đăng ký ba mock provider cho ASR/LLM/TTS.
2. Mỗi provider hỗ trợ trả về cố định + độ trễ cố định + tỷ lệ lỗi.
3. Thêm ba cấu hình mock ở phía backend và có thể đặt làm mặc định.
4. Chạy thử `ws_multi` và xuất ra kết quả kiểm thử tải baseline.

### Giai đoạn 2 (tăng cường, 1~2 ngày)

1. Thêm trả lời theo template, trả lời theo chuỗi (sequence), phát lại file âm thanh.
2. Thêm log chỉ số chi tiết hơn (thời gian xử lý theo từng giai đoạn).
3. Thêm script kiểm thử tải (chạy hàng loạt kịch bản + báo cáo tổng hợp).

## 7. Rủi ro và cách phòng tránh

1. **Định dạng âm thanh không khớp**: định dạng output của mock tts cần khớp với bộ giải mã (decode) ở tầng downstream hiện tại.
   - Cách phòng tránh: phiên bản đầu tiên sử dụng theo đường mã hóa (encoding) thông dụng hiện có và thêm log kiểm tra định dạng.
2. **Log quá lớn khi đồng thời cao**: log chi tiết ở mức đồng thời cao sẽ ảnh hưởng đến hiệu năng.
   - Cách phòng tránh: hạ mức độ log ở chế độ kiểm thử tải, xuất ra dạng tổng hợp cho các chỉ số quan trọng.
3. **Cấu hình lỡ chuyển nhầm sang dịch vụ thật**: dẫn đến vẫn gọi đến interface bên ngoài.
   - Cách phòng tránh: chặn mạng ở môi trường kiểm thử tải hoặc thêm cơ chế kiểm tra whitelist provider (nếu không phải mock thì từ chối khởi động).

## 8. Nội dung sẽ triển khai sau khi bạn xác nhận

Sau khi xác nhận, tôi sẽ trực tiếp sửa code theo danh sách sau:

1. Thêm mới `internal/domain/asr/mock`, `internal/domain/llm/mock`, `internal/domain/tts/mock`.
2. Gắn provider `mock` vào điểm đăng ký của provider factory / pool.
3. Bổ sung mẫu cấu hình mặc định (có thể chọn trực tiếp mock trên trang quản trị).
4. Thêm unit test tối thiểu (ít nhất là test hành vi của provider).
5. Đưa ra danh sách lệnh thực thi kiểm thử tải (các nấc đồng thời + thu thập chỉ số).

---

## Các lựa chọn cần bạn xác nhận

Xin xác nhận 4 điểm sau, tôi sẽ bắt đầu triển khai chính thức:

1. **Độ chi tiết (granularity) của Mock**: bạn có đồng ý mock ở cấp provider không (khuyến nghị)?
2. **Output của TTS**: phiên bản đầu tiên có chấp nhận "frame im lặng" làm âm thanh mock không (nhanh nhất)?
3. **Mức đồng thời mục tiêu của kiểm thử tải**: trước tiên nhắm tới mức đồng thời bao nhiêu (ví dụ 100/300/500)?
4. **Ngưỡng nghiệm thu**: có thực hiện theo tiêu chí nghiệm thu mặc định trong tài liệu này không?
