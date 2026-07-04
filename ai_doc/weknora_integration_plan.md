# Phương án tích hợp knowledge base WeKnora (chờ xác nhận)

## 1. Mục tiêu và ràng buộc

- Trên nền tảng `dify/ragflow` hiện có, thêm provider thứ ba: `weknora`.
- Quản trị viên có thể cấu hình tham số kết nối WeKnora; các thao tác CRUD knowledge base/tài liệu phía người dùng thường vẫn đi theo hướng đồng bộ bất đồng bộ (async sync).
- Luồng truy hồi RAG cục bộ của chương trình chính hỗ trợ `weknora` (không gọi ngược lại interface truy hồi của control panel).
- Giữ nguyên cấu trúc dữ liệu hiện có: không thêm cột database mới, việc đồng bộ tài liệu chỉ dùng `sync_status + sync_error`.
- Tiếp tục áp dụng chiến lược xóa hiện có: xóa tài liệu; nếu knowledge base được tạo tự động bởi hệ thống và phía remote đã rỗng thì xóa cả knowledge base.

## 2. Căn cứ theo API chính thức

- Tổng quan API và xác thực (`X-API-Key`, Base URL `/api/v1`):
  - <https://github.com/Tencent/WeKnora/blob/main/docs/api/README.md>
- Quản lý knowledge base:
  - <https://github.com/Tencent/WeKnora/blob/main/docs/api/knowledge-base.md>
- Quản lý knowledge (file/URL/knowledge thủ công, trạng thái phân tích):
  - <https://github.com/Tencent/WeKnora/blob/main/docs/api/knowledge.md>
- Tìm kiếm knowledge:
  - <https://github.com/Tencent/WeKnora/blob/main/docs/api/knowledge-search.md>
- Quản lý model (dùng để lấy model Embedding mặc định):
  - <https://github.com/Tencent/WeKnora/blob/main/docs/api/model.md>

## 3. Ánh xạ interface (hành động hệ thống -> API WeKnora)

1. Tạo knowledge base ở phía remote (external_kb_id)
   `POST /api/v1/knowledge-bases`

2. Cập nhật metadata knowledge base ở phía remote (tên/mô tả/cấu hình chunk)
   `PUT /api/v1/knowledge-bases/:id`

3. Xóa knowledge base ở phía remote
   `DELETE /api/v1/knowledge-bases/:id`

4. Tạo tài liệu (tải file lên)
   `POST /api/v1/knowledge-bases/:id/knowledge/file` (multipart)

5. Truy vấn trạng thái phân tích tài liệu
   `GET /api/v1/knowledge/:id` (dùng `parse_status`, `pending/processing/failed/completed`)

6. Xóa tài liệu
   `DELETE /api/v1/knowledge/:id`

7. Kiểm tra knowledge base có rỗng hay không (dùng để tự động xóa knowledge base)
   `GET /api/v1/knowledge-bases/:id/knowledge?page=1&page_size=1`

8. Truy hồi (kiểm thử recall + RAG của chương trình chính)
   `POST /api/v1/knowledge-search`

## 4. Ánh xạ dữ liệu và trạng thái

1. Ánh xạ trường

- `knowledge_bases.external_kb_id` cục bộ <-> `knowledge_base.id` của WeKnora
- `knowledge_base_documents.external_doc_id` cục bộ <-> `knowledge.id` của WeKnora
- `sync_provider = "weknora"`

2. Trạng thái đồng bộ tài liệu (một trường duy nhất)

- Sau khi vào hàng đợi: `pending`
- Bắt đầu upload: `uploading`
- Upload thành công: `uploaded`
- Đang phân tích: `parsing`
- Phân tích thành công: `synced`
- Upload thất bại: `upload_failed`
- Phân tích thất bại: `parse_failed`
- Thất bại nội bộ khi vào hàng đợi, v.v.: `failed`

3. Chiến lược polling trạng thái phân tích (đề xuất)

- `parse_poll_interval_ms`: mặc định `1000`
- `parse_timeout_ms`: mặc định `120000`
- Khi timeout, xử lý thành `parse_failed` và ghi vào `sync_error`

## 5. Phương án cải tạo backend (manager/backend)

1. `manager/backend/controllers/knowledge_sync.go`

- Thêm `weknoraKnowledgeSyncConfig` và `parseWeknoraKnowledgeSyncConfig`.
- Các hàm `syncKnowledgeBaseWithProvider/syncKnowledgeBaseDeleteBestEffort/syncKnowledgeDocumentBestEffort/syncKnowledgeDocumentDeleteBestEffort` thêm nhánh `weknora`.
- Thêm wrapper HTTP cho WeKnora (header xác thực `X-API-Key`, định dạng log request/response giống với hiện tại).
- Việc upload tài liệu thống nhất đi qua `/knowledge/file`:
  - Tài liệu dạng file: chuyển tiếp upload trực tiếp.
  - Tài liệu dạng văn bản: chuyển thành luồng byte tạm dạng UTF-8 `.md` rồi mới upload.
- Việc cập nhật tài liệu áp dụng chiến lược "tạo mới rồi xóa cũ", tránh phụ thuộc vào request body cập nhật không ổn định.
- Sau khi xóa tài liệu, kiểm tra knowledge base ở phía remote có rỗng không, nếu thỏa điều kiện thì xóa knowledge base ở phía remote.

2. `manager/backend/controllers/knowledge.go`

- Việc kiểm tra provider trong `CreateKnowledgeBaseDocumentByUpload` thêm `weknora`.
- Kiểm thử recall `TestKnowledgeBaseSearch` thêm nhánh `queryKnowledgeTestByWeknora`.
- Xử lý ngưỡng (threshold) vẫn theo quy tắc hiện tại: ngưỡng của request > ngưỡng của knowledge base > ngưỡng toàn cục.
- Nếu interface tìm kiếm của WeKnora không có tham số ngưỡng gốc, sẽ lọc lần hai theo `score` ở phía cục bộ.

3. `manager/backend/controllers/admin.go`

- Cấu trúc tổng hợp `knowledge_search` hiện có đã hỗ trợ nhiều provider, không cần sửa schema.
- Giữ nguyên cấu trúc output `knowledge.default_provider + knowledge.providers`.

## 6. Phương án cải tạo chương trình chính (internal/domain/rag)

1. Thêm mới `internal/domain/rag/weknora_searcher.go`

- Triển khai interface `Searcher`.
- Gọi `POST /api/v1/knowledge-search`, truy hồi chính xác theo `knowledge_base_ids`.
- Tái sử dụng cơ chế concurrency, timeout, dung sai lỗi (fault-tolerance) và tổng hợp hiện có.
- Ánh xạ kết quả trúng vào `KnowledgeSearchHit`:
  - `Content <- content`
  - `Title <- knowledge_title` (khi rỗng thì lùi về (fallback) dùng tên knowledge base cục bộ)
  - `Score <- score`

2. Sửa `internal/domain/rag/manager.go`

- `getSearcher()` thêm nhánh `weknora`.
- Logic đọc cấu hình provider giữ nguyên (đọc từ `knowledge.providers.weknora`).

## 7. Cải tạo frontend trang quản trị (manager/frontend)

1. `manager/frontend/src/views/admin/KnowledgeSearchConfig.vue`

- Dropdown provider thêm `weknora`.
- Thêm mục cấu hình mới (đề xuất):
  - `base_url` (mặc định `http://127.0.0.1:8080`)
  - `api_key`
  - `score_threshold` (mặc định `0.2`)
  - `chunk_size` (mặc định `1000`)
  - `chunk_overlap` (mặc định `200`)
  - `separators` (mặc định `["\\n\\n","\\n","。","！","？",";","；"]`)
  - `enable_multimodal` (mặc định `true`)
  - `embedding_model_id` (đề xuất bắt buộc)
  - `summary_model_id` (tùy chọn)
  - `rerank_model_id` (tùy chọn)
  - `vlm_model_id` (tùy chọn)
  - `parse_poll_interval_ms`, `parse_timeout_ms` (tùy chọn)

2. `manager/frontend/src/views/user/KnowledgeBases.vue`

- Phần hiển thị provider không cần thêm cột mới (đã có trường provider sẵn).
- Thuộc tính `accept` khi upload file thêm nhánh `weknora`.
- Phần văn bản mô tả thêm giải thích về đường dẫn upload của WeKnora và cơ chế phân tích bất đồng bộ.

## 8. Các quyết định triển khai then chốt (đề xuất xác nhận)

1. Tài liệu dạng văn bản có bắt buộc đi qua interface thủ công hay không

- Đề xuất phiên bản đầu tiên thống nhất đi qua `/knowledge/file` (đóng gói văn bản thành `.md`), giảm sự khác biệt giữa các interface và rủi ro tương thích.

2. Nguồn gốc model Embedding

- Đề xuất `embedding_model_id` ban đầu là trường bắt buộc do quản trị viên nhập.
- Có thể nâng cấp tùy chọn: nếu để trống, khi khởi động sẽ gọi `/api/v1/models` để tự động chọn model `Embedding` mặc định.

3. Giới hạn định dạng file

- WeKnora không đưa ra danh sách trắng (whitelist) nghiêm ngặt cho tài liệu; đề xuất phiên bản đầu tiên áp dụng "giới hạn phía frontend tương đối thoáng + báo lỗi dự phòng ở phía backend/remote".
- Nếu bạn cần whitelist nghiêm ngặt, có thể thu hẹp lại ở giai đoạn 2 dựa trên các định dạng đã kiểm chứng thực tế là ổn định.

## 9. Danh sách nghiệm thu

1. Quản trị viên thêm mới cấu hình `weknora` và đặt làm mặc định.
2. Sau khi người dùng thường tạo knowledge base, tự động tạo knowledge-base ở phía remote, ghi ngược lại `external_kb_id`.
3. Sau khi thêm mới tài liệu văn bản/tài liệu upload file, trạng thái chuyển đổi theo `uploading -> uploaded -> parsing -> synced`.
4. Khi phân tích thất bại, `sync_status=parse_failed`, và ghi vào `sync_error`.
5. Sau khi xóa tài liệu, tài liệu ở phía remote bị xóa; khi knowledge base ở remote rỗng, sẽ tự động xóa knowledge base theo chiến lược.
6. Kiểm thử recall có thể trả về kết quả trúng từ WeKnora, ngưỡng có hiệu lực ở phía cục bộ.
7. Luồng chat của chương trình chính có thể kích hoạt truy hồi một cách không cảm nhận khi liên kết với knowledge base `weknora`.

## 10. Rủi ro và rollback

1. Rủi ro

- Sự khác biệt giữa các phiên bản WeKnora có thể dẫn đến thay đổi trường trong request body (đặc biệt là các trường cấu hình khi tạo knowledge base).
- Thời gian phân tích tài liệu kéo dài, cần có chiến lược polling và timeout phối hợp.
- Nếu interface tìm kiếm thiếu tham số ngưỡng gốc, cần lọc lần hai ở phía cục bộ.

2. Rollback

- Chỉ cần vô hiệu hóa/xóa cấu hình `weknora` là có thể ngừng sử dụng, không ảnh hưởng đến `dify/ragflow` hiện có.
- Ở tầng code, nhánh provider có thể rollback độc lập, không liên quan đến thay đổi cấu trúc database.
