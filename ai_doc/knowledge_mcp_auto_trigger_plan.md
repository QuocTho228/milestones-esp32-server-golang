# Phương án kích hoạt truy hồi knowledge base tự động, không cảm nhận (v2, dành cho chat trên thiết bị)

## Bối cảnh

- Công cụ truy hồi hiện tại `search_knowledge` chủ yếu phụ thuộc vào việc mô hình có chủ động gọi hay không.
- Cần hỗ trợ "truy hồi có định hướng theo ID knowledge base trúng", nhằm giảm bớt các yêu cầu knowledge base không liên quan.

## Thay đổi cốt lõi

1. Nâng cấp tham số đầu vào của công cụ

- `search_knowledge` bổ sung trường tùy chọn `knowledge_base_ids: number[]`.
- Vẫn giữ `query`, `top_k`.
- Logic tương thích: khi không truyền `knowledge_base_ids`, sẽ truy hồi trên toàn bộ knowledge base khả dụng của agent hiện tại.

2. Ngữ nghĩa truy hồi có định hướng

- Khi truyền `knowledge_base_ids`, chỉ truy hồi trong phạm vi các knowledge base này.
- Các ID không hợp lệ (chưa liên kết/không tồn tại/inactive/thiếu `external_kb_id`) sẽ tự động bị bỏ qua (theo kiểu best effort).

3. Chiến lược thực thi đồng thời (concurrency)

- Thực hiện request song song theo "chiều knowledge base", mỗi knowledge base trúng sẽ độc lập phát khởi một yêu cầu truy hồi riêng.
- Provider vẫn do cấu hình riêng của từng knowledge base quyết định (dify/ragflow).
- Sau khi tổng hợp toàn bộ kết quả trúng, sắp xếp toàn cục theo score, sau đó cắt còn `top_k`.

4. Chiến lược timeout (đã xác nhận giá trị mặc định)

- Timeout cho từng knowledge base riêng lẻ: `2500ms`
- Timeout tổng: `2500ms`
- Timeout/thất bại một phần không chặn luồng chính; nếu toàn bộ đều thất bại thì trả về lỗi.

5. Nâng cấp gợi ý định tuyến cho LLM

- System Prompt sẽ cung cấp danh sách "id:tên" của các knowledge base khả dụng.
- Hướng dẫn mô hình truyền `knowledge_base_ids` khi có thể xác định được, và có thể không truyền khi không chắc chắn.

## Các bước thực hiện

1. Thêm `knowledge_base_ids` vào cấu trúc tham số của `search_knowledge`.
2. Truyền tiếp `knowledge_base_ids` xuyên suốt chuỗi gọi `ChatSessionOperator -> LocalMcpSearchKnowledge -> rag.Search`.
3. `rag.Search` bổ sung cơ chế lọc theo ID và kiểm soát timeout tổng.
4. `dify_searcher` và `ragflow_searcher` chuyển sang truy hồi song song theo từng knowledge base, đồng thời bổ sung kiểm soát timeout cho từng knowledge base riêng lẻ.
5. Điều chỉnh quy tắc truy hồi knowledge base trong system prompt, hỗ trợ hướng dẫn sử dụng `knowledge_base_ids`.

## Tính tương thích và rollback

- Các lệnh gọi cũ không truyền `knowledge_base_ids` sẽ không bị ảnh hưởng.
- Nếu bất kỳ provider nào thất bại cục bộ, chỉ ghi log và bỏ qua, vẫn giữ lại kết quả từ các provider khác.

## Tiêu chí nghiệm thu

- Công cụ có thể nhận và áp dụng hiệu lực với `knowledge_base_ids`.
- Trong kịch bản nhiều knowledge base, có thể truy hồi song song và trả về kết quả đã tổng hợp.
- Timeout của từng knowledge base riêng lẻ và timeout tổng đều mặc định là 2500ms.
- Luồng gọi cũ (không truyền `knowledge_base_ids`) vẫn hoạt động bình thường.
