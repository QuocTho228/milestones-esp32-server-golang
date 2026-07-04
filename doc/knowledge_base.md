# Giải thích tính năng Knowledge Base

Tài liệu này giới thiệu tính năng **Knowledge Base (RAG)** trong dự án, bao gồm cấu hình provider phía quản trị viên, quản lý knowledge base và tài liệu phía người dùng thường, kiểm thử recall, cũng như việc tích hợp truy hồi knowledge base trong luồng chat của chương trình chính.

Tài liệu liên quan:

- [Hướng dẫn sử dụng trang quản trị](./manager_console_guide.md)
- [Giải thích kiến trúc MCP](./mcp.md) (công cụ truy hồi knowledge base `search_knowledge` sẽ được kích hoạt thông qua chuỗi công cụ cục bộ)

---

## 1. Tổng quan tính năng

Tính năng knowledge base dùng để cung cấp cho agent năng lực "trả lời có căn cứ dựa trên tài liệu", bao gồm ba tầng:

1. Quản trị viên cấu hình provider truy hồi knowledge base (Dify / RAGFlow / WeKnora)
2. Người dùng thường tạo knowledge base và tài liệu, đồng bộ bất đồng bộ (async) lên provider
3. Agent liên kết với knowledge base, khi hội thoại sẽ kích hoạt công cụ `search_knowledge` cục bộ để thực hiện recall

Các provider hiện đã được trang quản trị frontend hỗ trợ:

- `dify`
- `ragflow`
- `weknora`

---

## 2. Phân công vai trò

## 2.1 Quản trị viên

Chịu trách nhiệm:

- Cấu hình provider truy hồi knowledge base (toàn cục)
- Duy trì tham số kết nối provider và ngưỡng mặc định
- (Tùy chọn) Quản lý knowledge base thay cho người dùng

Vị trí truy cập:

- `Quản trị viên -> Cấu hình truy hồi knowledge base`

## 2.2 Người dùng thường

Chịu trách nhiệm:

- Tạo/sửa/xóa knowledge base của riêng mình
- Quản lý tài liệu trong knowledge base (nhập văn bản / upload file)
- Phát khởi đồng bộ thủ công và retry
- Dùng "kiểm thử recall" để xác minh hiệu quả trúng từ khóa
- Chọn liên kết knowledge base trong agent

Vị trí truy cập:

- `Người dùng thường -> Knowledge base của tôi`
- `Người dùng thường -> Sửa agent (liên kết knowledge base)`

---

## 3. Quản trị viên: Cấu hình truy hồi knowledge base (Cấu hình Provider)

Phía quản trị hỗ trợ duy trì nhiều cấu hình provider, và chỉ định provider mặc định.

Các mục cấu hình thường gặp (khác nhau tùy theo provider):

- `Base URL`
- `API Key / Token`
- Ngưỡng truy hồi mặc định
- Tham số riêng của provider (như ngưỡng tương đồng của RAGFlow, tham số chia chunk của WeKnora, v.v.)

### 3.1 Dify

Các mục cấu hình điển hình:

- `base_url`
- `api_key`
- `score_threshold`
- Các tham số khác của provider

### 3.2 RAGFlow

Các mục cấu hình điển hình:

- `base_url`
- `api_key`
- `similarity_threshold`

### 3.3 WeKnora

Các mục cấu hình điển hình:

- `base_url`
- `api_key`
- `score_threshold`
- Tham số chia chunk (`chunk_size` / `chunk_overlap` / `separators`)
- Tham số polling khi phân tích (`parse_poll_interval_ms` / `parse_timeout_ms`)

Trang quản trị còn hỗ trợ lấy danh sách model của WeKnora (embedding / llm / rerank) để hỗ trợ điền cấu hình.

---

## 4. Người dùng thường: Knowledge base của tôi (Quản lý KB)

Vị trí truy cập:

- `Người dùng thường -> Knowledge base của tôi`

Các thao tác được hỗ trợ:

- Thêm mới/sửa knowledge base
- Đặt trạng thái (`active` / `inactive`)
- Đặt ngưỡng truy hồi (có thể kế thừa từ cấu hình toàn cục)
- Quản lý tài liệu
- Retry đồng bộ thủ công
- Kiểm thử recall
- Xóa knowledge base

### 4.1 Các trường của knowledge base (người dùng nhìn thấy)

Các cột hiển thị thường gặp:

- ID
- Tên
- Mô tả
- Nhà cung cấp (provider)
- Trạng thái
- Trạng thái đồng bộ
- Thời gian đồng bộ gần nhất
- Thao tác

Giải thích:

- Khi đồng bộ thất bại, thông tin lỗi sẽ hiển thị dưới dạng "tooltip" trong cột "trạng thái đồng bộ", tránh làm bảng bị quá rộng theo chiều ngang

### 4.2 Trạng thái đồng bộ (thường gặp)

Cả knowledge base và tài liệu đều có thể xuất hiện các trạng thái tương tự:

- Đang chờ đồng bộ
- Đang upload / Đã upload / Đang phân tích
- Đã đồng bộ
- Thất bại (bao gồm upload thất bại, phân tích thất bại, v.v.)

Nếu thất bại có thể bấm `Retry đồng bộ` để đưa lại vào hàng đợi tác vụ bất đồng bộ.

---

## 5. Quản lý tài liệu (trong knowledge base)

Mỗi knowledge base có thể chứa nhiều tài liệu, hỗ trợ:

- Tài liệu dạng văn bản (chỉnh sửa trực tuyến)
- Tạo tài liệu bằng upload file (định dạng bị giới hạn tùy theo provider)

Chức năng của trang:

- Thêm mới tài liệu
- Sửa tài liệu (tài liệu dạng file thường không hỗ trợ chỉnh sửa trực tuyến)
- Xóa tài liệu
- Retry đồng bộ
- Upload file

### 5.1 Định dạng file khi upload

Frontend sẽ hiển thị gợi ý `accept` và mô tả upload khác nhau tùy theo provider của knowledge base hiện tại:

- Dify: hỗ trợ các định dạng văn bản/tài liệu thông dụng (như txt/md/pdf/html/xlsx/docx/csv, v.v.)
- RAGFlow: hỗ trợ nhiều loại file hơn (bao gồm hình ảnh, log, file cấu hình, v.v.)
- WeKnora: hỗ trợ khá nhiều loại file (bao gồm Office, hình ảnh, email, v.v.)

Định dạng có thể upload cụ thể, xin xem theo gợi ý hiển thị trên trang.

---

## 6. Kiểm thử recall (phía người dùng)

Trong danh sách knowledge base, có thể thực hiện `Kiểm thử recall` cho từng knowledge base riêng lẻ, dùng để trực tiếp xác minh hiệu quả truy hồi của provider.

Các mục kiểm thử:

- `query`: từ khóa hoặc câu hỏi kiểm thử
- `top_k`
- `threshold` (chỉ có hiệu lực trong lần kiểm thử này, có thể để trống)

Nội dung trả về:

- Số lượng trúng
- Nguồn trúng (title)
- score
- Đoạn văn bản trúng
- Thời gian phản hồi

### 6.1 Thứ tự ưu tiên của ngưỡng (giải thích logic)

Thông thường, ngưỡng được lấy theo thứ tự ưu tiên sau:

1. Ngưỡng của request kiểm thử lần này (nếu có điền)
2. Ngưỡng riêng của knowledge base
3. Ngưỡng mặc định toàn cục của provider

### 6.2 Giải thích tham số của WeKnora (quan trọng)

Hiện tại kiểm thử recall của WeKnora đã sử dụng theo chiều knowledge base:

- `knowledge_base_ids` (danh sách ID knowledge base)

Dùng để giới hạn chính xác phạm vi truy hồi vào knowledge base hiện tại.

---

## 7. Agent liên kết knowledge base

Trong trang sửa agent, có thể chọn nhiều knowledge base cho agent (chọn nhiều - multi-select).

Giải thích hành vi:

- Hỗ trợ liên kết nhiều knowledge base
- Khi hội thoại, hệ thống sẽ dựa vào phán đoán của mô hình để quyết định có kích hoạt truy hồi knowledge base hay không
- Nếu có thể xác định được knowledge base cụ thể, lệnh gọi công cụ (tool call) sẽ truyền `knowledge_base_ids`
- Khi truy hồi thất bại sẽ giảm cấp (fallback) về hội thoại LLM thông thường (frontend có văn bản gợi ý)

---

## 8. Truy hồi knowledge base trong luồng hội thoại của chương trình chính

Chương trình chính thực hiện truy hồi knowledge base thông qua công cụ cục bộ `search_knowledge`.

Các trường tham số cốt lõi khi gọi công cụ:

- `query`
- `top_k`
- `knowledge_base_ids` (tùy chọn, danh sách ID knowledge base)

Giải thích hành vi:

- Không truyền `knowledge_base_ids`: truy hồi trong toàn bộ knowledge base khả dụng đang liên kết với agent hiện tại
- Truyền `knowledge_base_ids`: chỉ truy hồi trong phạm vi các knowledge base được chỉ định

Điều này giúp mô hình có thể thu hẹp phạm vi truy hồi khi đã biết câu hỏi thuộc về đâu, nâng cao độ liên quan và giảm việc trúng những kết quả không liên quan.

### 8.1 Tham số truy hồi của chương trình chính đối với WeKnora

Hiện tại request truy hồi của chương trình chính đối với WeKnora đã sử dụng:

- `knowledge_base_ids`

Giữ nhất quán với kiểm thử recall trên control panel.

---

## 9. Danh sách interface (phía người dùng)

### 9.1 CRUD knowledge base

- `GET /user/knowledge-bases`
- `POST /user/knowledge-bases`
- `GET /user/knowledge-bases/:id`
- `PUT /user/knowledge-bases/:id`
- `DELETE /user/knowledge-bases/:id`
- `POST /user/knowledge-bases/:id/sync`

### 9.2 Kiểm thử recall

- `POST /user/knowledge-bases/:id/test-search`

### 9.3 Quản lý tài liệu

- `GET /user/knowledge-bases/:id/documents`
- `POST /user/knowledge-bases/:id/documents`
- `POST /user/knowledge-bases/:id/documents/upload`
- `PUT /user/knowledge-bases/:id/documents/:doc_id`
- `DELETE /user/knowledge-bases/:id/documents/:doc_id`
- `POST /user/knowledge-bases/:id/documents/:doc_id/sync`

### 9.4 Agent liên kết knowledge base

- `GET /user/agents/:id/knowledge-bases`
- `PUT /user/agents/:id/knowledge-bases`

---

## 10. Danh sách interface (phía quản trị viên)

### 10.1 Quản lý cấu hình provider

- `GET /admin/knowledge-search-configs`
- `POST /admin/knowledge-search-configs`
- `PUT /admin/knowledge-search-configs/:id`
- `DELETE /admin/knowledge-search-configs/:id`

### 10.2 Lấy model của WeKnora (hỗ trợ cấu hình)

- `POST /admin/knowledge-search-configs/weknora/models`

### 10.3 Quản trị viên quản lý knowledge base thay cho người dùng (theo chiều người dùng)

- `GET /admin/users/:id/knowledge-bases`
- `POST /admin/users/:id/knowledge-bases`
- `PUT /admin/users/:id/knowledge-bases/:kb_id`
- `DELETE /admin/users/:id/knowledge-bases/:kb_id`

---

## 11. Các vấn đề thường gặp và cách xử lý

### 11.1 Sau khi tạo knowledge base, mãi không trúng kết quả nào

Ưu tiên kiểm tra:

1. Knowledge base/tài liệu đã đồng bộ thành công chưa
2. Provider bên ngoài đã hoàn tất việc xây dựng index chưa
3. Ngưỡng truy hồi có đang đặt quá cao không
4. `query` có quá chung chung hoặc lệch khỏi nội dung tài liệu không

### 11.2 Sau khi upload file, tài liệu không thể chỉnh sửa

Tài liệu được tạo bằng upload file thường được xử lý như "tài liệu dạng file", frontend sẽ giới hạn không cho chỉnh sửa trực tuyến, khuyến nghị xóa rồi upload lại.

### 11.3 Phạm vi truy hồi của WeKnora không đúng

Xác nhận:

- Kiểm thử recall trên control panel có đang dùng knowledge base hiện tại để phát khởi kiểm thử không
- Lệnh gọi công cụ (tool call) của agent có truyền đúng `knowledge_base_ids` không

---

## 12. Đề xuất sử dụng

- Tách nhiều knowledge base khác nhau theo từng mảng nghiệp vụ (như hậu mãi, sản phẩm, hợp đồng)
- Dùng "kiểm thử recall" để tinh chỉnh ngưỡng trước, rồi mới kết nối vào agent
- Trong phần mô tả của agent, nói rõ khi nào cần dùng knowledge base để trả lời, có thể nâng cao chất lượng kích hoạt
