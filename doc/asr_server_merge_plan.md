# Phương án hợp nhất asr_server (theo cùng hình thức với manager/backend)

## Mục tiêu

- **asr_server vẫn giữ hình thức là một repository độc lập**: có `go.mod`, `main.go` riêng, có thể clone, build, chạy độc lập.
- **Chương trình chính có thể khởi tạo (init) nó**: giống như `manager/backend`, tiến trình chính sẽ dùng `replace` để tham chiếu tới thư mục con này, và khi cần có thể khởi động dịch vụ HTTP của asr_server ngay bên trong tiến trình (ở một cổng riêng), không cần chạy thành một tiến trình tách biệt.

## Cách đưa vào: khuyến nghị dùng Git Submodule

Repo chính có thể lấy được thư mục `asr_server/` bằng hai cách:

| Cách thực hiện                  | Mô tả                                                                                                                                                                                                                  |
| ------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Git Submodule (khuyến nghị)** | asr_server vẫn là một Git repo độc lập; repo chính dùng `git submodule add` để tham chiếu, kết quả là một thư mục "trỏ tới một commit cụ thể của asr_server"; repo chính chỉ ghi lại đường dẫn submodule và số commit. |
| Copy/di chuyển mã nguồn         | Đưa trực tiếp mã nguồn của asr_server vào thư mục repo chính; asr_server và repo chính dùng chung một lịch sử Git (hoặc trở thành một phần của repo chính).                                                            |

Phần dưới đây trình bày các bước theo hướng **Submodule** là chính; logic `replace` và khởi động nhúng (embed) ở phía repo chính giống hệt như cách "copy mã nguồn".

## Tham khảo: hình thức hợp nhất của manager/backend

| Mục                          | Cách làm của manager/backend                                                                                 |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------ |
| Thư mục                      | `manager/backend/` nằm trong repo chính                                                                      |
| Tên module                   | `milestones/manager/backend` (khai báo trong go.mod của backend)                                             |
| Tham chiếu ở repo chính      | `replace milestones/manager/backend => ./manager/backend`                                                    |
| Chạy độc lập                 | `manager/backend/main.go`: LoadWithPath → database.Init → router.Setup → r.Run()                             |
| Nhúng vào chương trình chính | `cmd/server/manager_http.go`: dùng cùng bộ config/database/router, tự khởi tạo `http.Server` ở một cổng khác |

## Thiết kế hợp nhất asr_server (theo đúng hình thức trên)

### 1. Thư mục và module (theo cách Submodule)

- **asr_server cần có sẵn một Git repo độc lập trước** (nếu hiện đang nằm trong monorepo, có thể tách ra thành repo độc lập, hoặc dùng URL của repo asr_server hiện có).
- **Thêm submodule vào repo chính** (thực hiện tại thư mục gốc của repo chính, và thư mục `asr_server` chưa tồn tại):
  ```bash
  cd milestones-esp32-server-golang
  git submodule add <URL repo asr_server> asr_server
  ```
  Sau khi hoàn tất, repo chính sẽ có thêm:
  - Thư mục `asr_server/` (nội dung là bản checkout hiện tại của repo asr_server tại một commit)
  - File `.gitmodules`, và có thể xem bản ghi submodule bằng `git submodule status`
- **Đường dẫn thư mục**: trong repo chính là `milestones-esp32-server-golang/asr_server/`, nhất quán với "cách copy mã nguồn"; mã Go và `replace` trong go.mod của repo chính đều trỏ tới `./asr_server`.
- **Tên module**: giữ nguyên tên module hiện tại của asr_server là **`voice_server`** (để khi dùng như repo độc lập có thể `go build` trực tiếp, không cần sửa import).
- **go.mod của repo chính**: thêm một dòng:
  - `replace voice_server => ./asr_server`
- **go.mod của asr_server**: vẫn giữ `module voice_server`, không tham chiếu đến repo chính; khi là repo độc lập thì không có replace, sau khi hợp nhất vào repo chính chỉ cần phía repo chính có replace là đủ.

**Khi clone repo chính cần kéo theo submodule** (chọn một trong hai cách):

```bash
# Clone và kéo submodule luôn trong một lần
git clone --recurse-submodules <URL repo chính>

# Hoặc clone trước rồi khởi tạo submodule sau
git clone <URL repo chính>
cd milestones-esp32-server-golang
git submodule update --init --recursive
```

**CI / build tự động**: nếu repo chính cần build mã có phụ thuộc vào asr_server, cần chạy `git submodule update --init --recursive` trước khi build (hoặc dùng `--recurse-submodules` khi clone).

### 2. Chạy độc lập (asr_server vẫn là "repo độc lập")

- Khi clone/mở riêng thư mục `asr_server`:
  - `go build -o asr_server .`
  - `./asr_server` sử dụng `config.json` (hoặc chỉ định đường dẫn bằng `-config`), hành vi giống như hiện tại.
- Không phụ thuộc vào repo chính; `replace` của repo chính chỉ ảnh hưởng tới việc build của repo chính.

### 3. Khởi tạo ở chương trình chính (nhúng asr_server)

- **Điểm vào**: thêm file `cmd/server/asr_server_http.go` trong repo chính (cùng cấp với `manager_http.go`).
- **Logic** (giống hệt manager_http):
  1. Tiến trình chính khi khởi động sẽ dựa vào cấu hình để quyết định có gọi hay không (ví dụ tham số `-asr-enable` + `-asr-config`).
  2. Sử dụng các package của asr_server:
     - `voice_server/config`: gọi `InitConfig(configPath)`, sau đó `GetConfig()` để lấy `*Config`.
     - `voice_server/internal/bootstrap`: gọi `InitApp(cfg)` để lấy `*AppDependencies`.
     - `voice_server/internal/router`: gọi `NewRouter(deps)` để lấy `*gin.Engine`.
  3. Dùng `deps.RateLimiter.Middleware(r)` làm Handler, khởi tạo `http.Server` ở **một cổng riêng** (ví dụ 8080), chạy `ListenAndServe` trong một goroutine.
  4. Khi thoát, cung cấp hàm `StopAsrServerHTTP()` để gọi `Shutdown` cho `http.Server`, đồng thời giải phóng các tài nguyên cần thiết (ví dụ các thành phần trong bootstrap cần được Close).
- **Cấu hình**: asr_server vẫn dùng file `config.json` của riêng nó; khi nhúng, đường dẫn file cấu hình do tham số của tiến trình chính hoặc cấu hình của repo chính chỉ định (ví dụ `asr_server/config.json` hoặc `config/asr_server.json`).

### 4. Danh sách thay đổi ở repo chính (theo cách Submodule)

| Vị trí                                                                  | Thay đổi                                                                                                                                                                                                               |
| ----------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Gốc repo chính                                                          | Chạy `git submodule add <URL repo asr_server> asr_server`, để có được thư mục `asr_server/` và file `.gitmodules` (asr_server cần có sẵn repo Git độc lập trước)                                                       |
| `milestones-esp32-server-golang/go.mod`                                 | Thêm `replace voice_server => ./asr_server`; nếu mã của repo chính cần import voice_server, thêm `voice_server` vào `require` (hoặc để `go mod tidy` tự bổ sung)                                                       |
| `milestones-esp32-server-golang/cmd/server/main.go`                     | Parse tham số `-asr-enable`, `-asr-config`; nếu enable, gọi `StartAsrServerHTTP(configPath)` trước khi `Run()`; sau `<-quit` thì gọi `StopAsrServerHTTP()`                                                             |
| Thêm mới `milestones-esp32-server-golang/cmd/server/asr_server_http.go` | Triển khai `StartAsrServerHTTP(configPath string)`, `StopAsrServerHTTP()`, bên trong dùng `voice_server/config`, `voice_server/internal/bootstrap`, `voice_server/internal/router`, theo đúng mô hình của manager_http |

### 5. Những phần cần asr_server hỗ trợ expose ra ngoài

- **config**: đã có sẵn `InitConfig(path)`, `GetConfig()`, tiến trình chính có thể dùng trực tiếp.
- **bootstrap**: đã có sẵn `InitApp(cfg *config.Config)`, trả về `*AppDependencies`, tiến trình chính có thể dùng trực tiếp.
- **router**: đã có sẵn `NewRouter(deps) *gin.Engine`; tiến trình chính dùng `deps.RateLimiter.Middleware(r)` làm Handler là đủ.
- **Thoát an toàn (graceful shutdown)**: nếu bên trong bootstrap có tài nguyên cần `Close()` (ví dụ pool VAD, recognizer toàn cục, v.v.), cần cung cấp một hàm thống nhất kiểu `Shutdown(deps *AppDependencies)` trong asr_server để `StopAsrServerHTTP()` gọi tới; nếu hiện tại chưa có, có thể tạm thời chỉ làm `Server.Shutdown` trước, bổ sung sau.

### 6. Phụ thuộc và build

- Các phụ thuộc của asr_server (sherpa-onnx, qdrant, ten-vad, v.v.) vẫn giữ trong **asr_server/go.mod**; repo chính **không** đưa các phụ thuộc của asr_server lên `require` của go.mod chính, chỉ tham chiếu submodule qua `require voice_server` (hoặc tương đương), để `go mod tidy` tự đồng bộ phụ thuộc trong repo chính.
- Nếu khi build repo chính bị thiếu phụ thuộc, có thể thêm trực tiếp các phụ thuộc trực tiếp mà asr_server dùng vào `require` của go.mod repo chính.
- CGO, các thư viện local (như file so/dll của ten_vad, sherpa-onnx) vẫn đặt theo cách hiện tại của asr_server trong thư mục asr_server, hoặc gom về thư mục `lib/` chung của repo chính, chỉ cần ghi rõ trong script build/tài liệu.

### 7. Điểm khác biệt so với manager/backend

- Tên module của manager/backend là `milestones/manager/backend`, còn asr_server giữ nguyên `voice_server`, nhờ vậy khi asr_server đóng vai trò repo độc lập thì không cần sửa import.
- Repo chính chỉ cần `replace voice_server => ./asr_server` là đủ, không cần sửa đường dẫn package nội bộ của asr_server.
- Cách "khởi tạo" ở chương trình chính là nhất quán: không gọi hàm `main()` của asr_server, chỉ tái sử dụng config + bootstrap + router, khởi tạo một dịch vụ HTTP với cổng riêng ngay bên trong tiến trình chính.

### 8. Tóm tắt (theo cách Submodule)

- **Repo độc lập**: asr_server là một Git repo độc lập, có `go.mod` riêng (`module voice_server`) và `main.go`, có thể clone, build, chạy độc lập.
- **Hợp nhất vào repo chính**: repo chính dùng **Git submodule** để tham chiếu asr_server, tạo ra thư mục `asr_server/`; repo chính có `replace voice_server => ./asr_server`; sau khi clone repo chính cần chạy `git submodule update --init` (hoặc `git clone --recurse-submodules`).
- **Khởi tạo ở chương trình chính**: repo chính thêm mới `asr_server_http.go`, dựa theo cấu hình để khởi động dịch vụ HTTP của asr_server ngay trong tiến trình (ở cổng riêng), logic nhất quán với `manager_http.go`.

**Ghi chú về build**: asr_server phụ thuộc vào sherpa-onnx (CGO), repo chính dùng **build tag** để biến việc nhúng thành tùy chọn (optional):

- **Build mặc định** (không bật nhúng asr_server): `go build -o milestones_server ./cmd/server`, lúc này nếu dùng `-asr-enable` sẽ in ra thông báo "chưa được biên dịch vào binary này".
- **Bật nhúng asr_server**: `go build -tags asr_server -o milestones_server ./cmd/server`, yêu cầu máy build phải có sẵn môi trường CGO và các thư viện cần thiết cho sherpa-onnx.

Nếu xác nhận triển khai theo phương án này, có thể tiếp tục làm rõ thêm: danh sách trách nhiệm của `Shutdown(deps)` bên trong asr_server, cổng mặc định và đường dẫn cấu hình mặc định, cũng như cách đặt tên và giá trị mặc định cho các tham số trong `main.go` của repo chính.
