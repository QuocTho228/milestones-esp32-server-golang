# Tài liệu tích hợp API cho MemOS Provider độc lập (dựa trên tài liệu chính thức, có thể ghi đè endpoint theo triển khai thực tế)

> Tài liệu chính thức: `https://memos-docs.openmem.net/cn/api_docs/start/overview`
>
> Ví dụ base_url: `https://memos.memtensor.cn/api/openmem/v1`
>
> Mục tiêu: Tích hợp MemOS như một **provider độc lập**, không còn dùng chung với `mem0`.

---

## 1. Nguyên tắc

- Không còn đoán mò đường dẫn API.
- Không còn định tuyến `memos` sang client `mem0`.
- Chỉ thực hiện ánh xạ trường dữ liệu và endpoint dựa theo tài liệu chính thức bạn cung cấp.

---

## 2. Ràng buộc cải tạo trong repository hiện tại

Interface hệ thống `MemoryProvider` yêu cầu phải triển khai:

- `AddMessage`
- `GetMessages`
- `GetContext`
- `Search`
- `Flush`
- `ResetMemory`

Do đó khi tích hợp MemOS, bắt buộc phải tìm ra API tương ứng (hoặc tổ hợp API) trong tài liệu chính thức cho từng mục và hoàn thành việc ánh xạ.

---

## 3. Các điểm chính cần tích hợp (theo tài liệu chính thức)

Các trường dưới đây được tích hợp theo đường dẫn cố định chính thức, không hiển thị cấu hình endpoint trên console:

1. Phương thức xác thực
   - Tên Header:
   - Tiền tố Token (ví dụ `Bearer `):

2. API ghi nhớ (Write memory)
   - Method + Path:
   - Ví dụ request body:
   - Các trường quan trọng trong response body:

3. API truy vấn ký ức (Query memory)
   - Method + Path:
   - Tham số lọc (agent_id / user_id / session_id...):
   - Ví dụ response body:

4. API tìm kiếm/gợi nhớ (Search/Recall)
   - Method + Path:
   - Tham số (query/top_k/threshold/time_range):
   - Các trường response (văn bản, điểm số, timestamp):

5. API xóa/reset
   - Method + Path:
   - Phạm vi xóa (cấp user/cấp session/cấp agent):

6. Có tồn tại API flush/index refresh hay không
   - Nếu không có, `Flush` sẽ được giáng cấp về mặt ngữ nghĩa như thế nào:

---

## 4. Kế hoạch triển khai code (thực hiện sau khi xác nhận)

```text
internal/domain/memory/memos/
  memos_client.go        # Lệnh gọi HTTP thực tế
  types.go               # DTO cho request/response
  mapper.go              # API -> schema.Message
  memos_test.go          # httptest mock
```

Và cần sửa đổi:

- `internal/domain/memory/base.go`
  - `MemoryTypeMemOS -> memos.GetWithConfig(config)`
- Phía cấu hình quản trị giữ nguyên `memos` (đã hỗ trợ)
- Cấu hình mẫu giữ nguyên `memory.memos` (đã hỗ trợ)

---

## 5. Giải thích môi trường

Môi trường thực thi hiện tại khi gửi request tới trang tài liệu chính thức trả về lỗi 403, không thể tự động lấy nội dung tài liệu ở local.

Hiện tại đã triển khai theo đường dẫn cố định, console không cung cấp chức năng chỉnh sửa đường dẫn endpoint.

## 6. Giải thích triển khai hiện tại

- URL request thực tế = `base_url + endpoint_path` (ví dụ `http://host/api/v1` + `/add/message`).
- Đã triển khai `memos_client.go`, mặc định sử dụng các endpoint sau:
  - `/add/message`
  - `/get/messages`
  - `/search/memory`
  - `/flush`
  - `/reset/memory`
- Các đường dẫn sử dụng ngữ nghĩa chính thức cố định: `/add/message`, `/get/messages`, `/search/memory`, `/flush`, `/reset/memory`.

## 7. Ràng buộc trường dữ liệu của Add Message (đã điều chỉnh theo tài liệu)

- `user_id` / `conversation_id` là bắt buộc.
- `agent_id` là tùy chọn, chỉ truyền khi có giá trị.
- Trong triển khai hiện tại, `agentID` được ánh xạ đồng thời sang cả `user_id` và `conversation_id`; khi `agentID` rỗng sẽ báo lỗi trực tiếp, không còn sử dụng giá trị placeholder mặc định.

## 8. Ánh xạ trường dữ liệu của Search Memory (đã điều chỉnh theo tài liệu)

- Path: `/search/memory`
- `user_id`: sử dụng ánh xạ từ `agentID`
- `conversation_id`: sử dụng ánh xạ từ `agentID`
- `query`: truyền thẳng (pass-through) từ input của người dùng
- `memory_limit_number`: được ánh xạ từ `topK`
- `relativity`: được ánh xạ từ `search_threshold`
