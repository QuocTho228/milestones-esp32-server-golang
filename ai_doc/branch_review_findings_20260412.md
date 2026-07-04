# Ghi nhận review code nhánh (2026-04-12)

Nhánh: `codex/optimize-tool-invocation-for-concurrency`

## 1. [P1] Phát media thất bại nhưng vẫn bị đánh dấu là đã có media output

- Vị trí:
  - `internal/app/server/chat/tool.go:257`
  - `internal/app/server/chat/tool.go:271`
  - `internal/app/server/chat/tool.go:275`
  - `internal/app/server/chat/llm.go:873`
- Mô tả vấn đề:
  - Khi gọi `handleAudioContent` / `handleResourceLink` bị thất bại, logic hiện tại vẫn đặt:
    - `execResult.hasMediaOutput = true`
    - `execResult.shouldStopLLMProcessing = true`
  - Tầng trên dựa vào đó để cho rằng "media đã được xuất ra", dẫn đến đi theo nhánh xử lý ức chế `tts_stop` / không tiếp tục xử lý LLM.
- Ảnh hưởng rủi ro:
  - Trên thực tế media chưa được phát thành công, nhưng luồng hội thoại lại kết thúc theo hướng "media output đã thành công", có thể gây ra hiện tượng client im lặng (không phản hồi), trạng thái không nhất quán, hoặc không có phản hồi tiếp theo.

## 2. [P2] Cơ chế loại trùng (dedupe) đối với ToolCall có ID rỗng có thể vô tình loại bỏ nhầm các lệnh gọi trùng hợp lệ

- Vị trí:
  - `internal/app/server/chat/tool.go:154`
  - `internal/app/server/chat/tool.go:160`
  - `internal/app/server/chat/tool.go:44`
  - `internal/app/server/chat/tool.go:67`
- Mô tả vấn đề:
  - Hiện tại, đối với `ToolCall.ID` rỗng, hệ thống dùng `auto_<name>_<arguments>` để sinh ra định danh và thực hiện loại trùng (dedupe).
  - Nếu mô hình sinh ra hợp lệ hai lệnh gọi "không có ID và tham số giống nhau", lệnh gọi thứ hai sẽ bị bỏ qua.
- Ảnh hưởng rủi ro:
  - Có thể dẫn đến số lượng/tương ứng giữa `tool_calls` trong lịch sử assistant và `tool_result` phía sau không khớp nhau, ảnh hưởng đến ngữ cảnh của các lượt hội thoại sau và độ tin cậy của việc gọi tool.
