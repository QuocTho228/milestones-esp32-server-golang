# Tài liệu thiết kế Chat Hook / Plugin V2

## 1. Mục tiêu tài liệu

Tài liệu này nhằm trả lời ba câu hỏi:

1. Ranh giới hợp lý của kiến trúc Hook trong chuỗi chat hiện tại là gì;
2. V2 nên ưu tiên bổ sung những năng lực nào;
3. Trên tiền đề càng ít xáo trộn luồng nghiệp vụ chính hiện có càng tốt, làm thế nào để tiến hóa theo từng giai đoạn.

Định vị của tài liệu này không phải là "xây dựng một chợ plugin (plugin marketplace) hoàn chỉnh", mà là để triển khai cho repo hiện tại một phương án Chat Hook / Plugin V2 **có thể quản trị (governable), có thể mở rộng, có thể quan sát (observable)**.

---

## 2. Tóm tắt (TL;DR)

### 2.1 Kết luận trong một câu

Việc triển khai hiện tại đã sở hữu một dạng phôi thai khả dụng của **Chat Hook Framework**, nhưng vẫn chưa phù hợp để định nghĩa trực tiếp là một "Plugin Platform hoàn chỉnh".

### 2.2 Chủ trương cốt lõi của V2

Trọng tâm của V2 không phải là "lật đổ và viết lại từ đầu", mà là:

- Giữ lại các điểm tích hợp (access point) nghiệp vụ hiện có;
- Tách rõ ràng **Interceptor (có thể ghi đè luồng chính)** và **Observer (chỉ làm quan sát)**;
- Bổ sung cho việc thực thi bất đồng bộ các yếu tố **hàng đợi có giới hạn (bounded queue), timeout, chiến lược loại bỏ (drop), chỉ số đo lường (metrics)**;
- Đưa vào **PluginMeta + Registry + Lifecycle**;
- Bổ sung hợp đồng (contract) cho các sự kiện ASR / LLM / TTS / Metric.

### 2.3 Đề xuất mức độ ưu tiên

| Ưu tiên | Đề xuất                                    | Mục tiêu                                     |
| ------- | ------------------------------------------ | -------------------------------------------- |
| P0      | Quản trị Async Runtime                     | Ngăn plugin chậm kéo sập cả hệ thống         |
| P0      | Tách ngữ nghĩa Interceptor / Observer      | Giảm việc dùng lẫn ngữ nghĩa                 |
| P0      | Tài liệu hợp đồng Payload                  | Giảm việc dùng sai plugin                    |
| P1      | PluginMeta / Registry                      | Giúp việc đăng ký plugin có thể quản lý được |
| P1      | Lifecycle / Config                         | Hỗ trợ plugin có trạng thái (stateful)       |
| P2      | Đa hàng đợi / đa worker / tích hợp tracing | Hỗ trợ mở rộng phức tạp hơn                  |

---

## 3. Đánh giá hiện trạng

## 3.1 Những phần thiết kế hiện tại đã làm đúng

Việc triển khai hiện tại đã có các ưu điểm sau:

1. **Điểm tích hợp được chọn đúng**
   - Hook đã được gắn vào output cuối cùng của ASR, input/output của LLM, điểm bắt đầu/kết thúc input/output của TTS, giai đoạn Metric;
   - Đều là những vị trí có giá trị nghiệp vụ cao nhất trong chuỗi chat chính.

2. **Phân tầng về cơ bản là đúng**
   - `internal/pkg/hooks` chịu trách nhiệm cho khung thực thi (execution framework) tổng quát;
   - `internal/domain/chat/hooks` chịu trách nhiệm cho ngữ cảnh domain chat, sự kiện và typed payload;
   - `internal/app/server/chat/*` chỉ chịu trách nhiệm emit tại các vị trí phù hợp.

3. **Typed façade đã được thiết lập**
   - Phía nghiệp vụ không còn thao tác trực tiếp với `any`;
   - Đây là nền tảng tốt để V2 tiếp tục tăng cường hợp đồng và quản trị.

4. **Khả năng cắm-rút (pluggability) đã khả dụng**
   - Hiện tại đã có thể hỗ trợ các năng lực built-in như plugin thống kê, ghi đè văn bản, chặn luồng...

## 3.2 Những thiếu sót chính hiện tại

Điều cần giải quyết nhất hiện nay không phải là "trừu tượng hóa sai", mà là "năng lực quản trị chưa đủ".

### A. Dùng lẫn ngữ nghĩa

Hiện tại đang dùng chung một mô hình Hook duy nhất để tải hai loại nhu cầu hoàn toàn khác nhau:

- Interceptor có thể ghi đè luồng chính;
- Metric / audit / telemetry chỉ làm quan sát.

Điều này gây ra các vấn đề sau:

- Ngữ nghĩa `stop` không phù hợp với mọi sự kiện;
- Việc `ghi đè payload` chỉ phù hợp với một phần các giai đoạn;
- Người viết plugin khó hiểu được sự kiện nào cho phép làm gì.

### B. Quản trị thực thi bất đồng bộ chưa đủ

Các điểm đau (pain point) hiện tại của việc thực thi async:

- Hàng đợi không có giới hạn trên;
- Thiếu timeout;
- Thiếu chỉ số dropped;
- Tất cả async handler dùng chung một luồng đơn để tiêu thụ tuần tự;
- Thiếu cơ chế cô lập đối với plugin chậm.

### C. Cách đăng ký plugin thiên về hard-code

Hiện tại chủ yếu đăng ký thông qua `RegisterBuiltinPlugins` trong code. Cách này tuy đơn giản, nhưng bất lợi cho:

- Việc xem hiện đang tải những plugin nào;
- Việc bật/tắt;
- Việc cấu hình theo từng môi trường;
- Việc debug ở cấp độ plugin.

### D. Hợp đồng (contract) chưa rõ ràng

Hiện tại tuy đã có `ASROutputData` / `LLMInputData` / `LLMOutputData` / `TTSInputData` / `MetricData`, nhưng vẫn còn thiếu các ràng buộc sau:

- Những trường nào được phép sửa;
- Những trường nào không được phép để trống;
- Ngữ nghĩa nghiệp vụ của `stop` là gì;
- `Err` có được phép ghi đè hay không;
- Thời gian tối đa mà plugin được phép chạy là bao lâu.

---

## 4. Định vị và nguyên tắc thiết kế của V2

## 4.1 Định vị kiến trúc

V2 đề xuất đặt tên chính thức cho hệ thống là:

> **Chat Interceptor & Observer Framework**

thay vì gọi trực tiếp là:

> Plugin Platform

Cách đặt tên này bám sát hơn với năng lực thực tế của giai đoạn hiện tại, đồng thời cũng có lợi hơn cho việc kiểm soát kỳ vọng của team.

## 4.2 Nguyên tắc thiết kế

V2 nên tuân theo các nguyên tắc sau:

1. **Giữ ổn định cho luồng nghiệp vụ chính**
   - Không sửa đổi trên diện rộng logic hiện có của ASR / LLM / TTS;
   - Ưu tiên tăng cường tại tầng Hook Runtime và Domain Facade.

2. **Làm quản trị trước, làm nền tảng hóa sau**
   - Giải quyết trước các vấn đề ngữ nghĩa, ranh giới, giám sát, tính ổn định;
   - Sau đó mới xem xét đến hệ sinh thái plugin phức tạp hơn.

3. **Phân biệt Interceptor và Observer trước**
   - Mọi hành vi có thể ghi đè luồng chính đều phải được phân loại rõ ràng thành Interceptor;
   - Mọi hành vi quan sát chỉ-đọc đều phải được phân loại rõ ràng thành Observer.

4. **Làm rõ hợp đồng trước, mở rộng số lượng plugin sau**
   - Khi chưa có hợp đồng, plugin càng nhiều thì chi phí bảo trì càng cao.

5. **Tiến hóa dần dần, không viết lại một lần**
   - Ưu tiên thiết kế tương thích với interface `emit` hiện có;
   - Việc di trú (migration) cần được thực hiện theo từng giai đoạn.

---

## 5. Kiến trúc tổng thể của V2

## 5.1 Cấu trúc ba tầng

V2 tiếp nối cấu trúc ba tầng hiện tại, nhưng tăng cường ranh giới trách nhiệm.

### Tầng 1: Tầng luồng nghiệp vụ chính

Trách nhiệm:

- Emit tại các nút quan trọng của ASR / LLM / TTS / Session Metric;
- Không trực tiếp nhận biết việc đăng ký plugin, chiến lược lập lịch, vòng đời.

Không chịu trách nhiệm:

- Đăng ký plugin;
- Quản trị thực thi plugin;
- Quản lý metadata plugin.

### Tầng 2: Tầng Chat Domain Hook

Trách nhiệm:

- Định nghĩa sự kiện thuộc domain chat, typed payload, domain context;
- Cung cấp cho code nghiệp vụ một điểm vào thống nhất và ổn định;
- Ràng buộc hợp đồng trường dữ liệu và ngữ nghĩa stop/error.

### Tầng 3: Tầng Generic Runtime

Trách nhiệm:

- Đăng ký plugin;
- Sắp xếp thứ tự và thực thi;
- Lập lịch async;
- timeout / drop / metrics;
- Quản lý vòng đời.

## 5.2 Vai trò của Hook trong đường đi của request

```text
Văn bản cuối cùng của ASR
  -> ASR Output Interceptors
  -> LLM Input Interceptors
  -> Thực thi LLM
  -> LLM Output Interceptors
  -> TTS Input Interceptors
  -> TTS Output Observers

Đồng thời:
  Metric Observers quan sát tại các giai đoạn turn_start / asr_first / asr_final / llm_start /
  llm_first / llm_end / tts_start / tts_first / tts_stop, v.v.
```

Cốt lõi của cách phân chia này là:

- Luồng nghiệp vụ chính chỉ quan tâm "emit khi nào";
- Interceptor chịu trách nhiệm ghi đè;
- Observer chịu trách nhiệm quan sát;
- Runtime chịu trách nhiệm quản trị.

---

## 6. Thiết kế mô hình sự kiện

## 6.1 Phân tầng sự kiện

### A. Nhóm sự kiện Interceptor

Dùng cho việc ghi đè đồng bộ và kiểm soát luồng.

Đề xuất giữ lại:

- `chat.asr.output`
- `chat.llm.input`
- `chat.llm.output`
- `chat.tts.input`

Nhóm sự kiện này cần có:

- Thực thi có thứ tự theo priority;
- Có thể sửa payload;
- Có thể `stop`;
- Có thể trả về error;
- Bắt buộc phải trả kết quả nhanh.

### B. Nhóm sự kiện Observer

Dùng cho quan sát, đo lường (metric), ghi log, trace, audit, v.v.

Đề xuất phân loại vào Observer:

- `chat.metric`
- `chat.tts.output.start`
- `chat.tts.output.stop`
- Các sự kiện audit / trace / debug mở rộng về sau

Nhóm sự kiện này cần có:

- Mặc định chỉ-đọc;
- Không được phép dừng (stop) luồng chính;
- Không tham gia vào việc thay đổi payload của luồng chính;
- Có thể thực thi bất đồng bộ;
- Lỗi chỉ ảnh hưởng đến chuỗi quan sát.

## 6.2 Đề xuất đặt tên sự kiện

Tiếp tục sử dụng cách đặt tên phân tầng hiện có, không đề xuất thay đổi lớn ngay hệ thống đặt tên. Khuyến nghị giữ nguyên:

- `chat.asr.output`
- `chat.llm.input`
- `chat.llm.output`
- `chat.tts.input`
- `chat.tts.output.start`
- `chat.tts.output.stop`
- `chat.metric`

Lý do:

- Cách đặt tên hiện tại đã đủ trực quan;
- Tương thích với triển khai hiện có;
- Chi phí di trú (migration) thấp nhất.

---

## 7. Thiết kế Runtime

## 7.1 PluginMeta

V2 đưa vào metadata thống nhất, thuận tiện cho việc hiển thị, bật/tắt, chẩn đoán và sắp xếp thứ tự.

```go
package hooks

type PluginKind string

const (
    PluginKindInterceptor PluginKind = "interceptor"
    PluginKindObserver    PluginKind = "observer"
)

type PluginMeta struct {
    Name        string
    Version     string
    Description string
    Priority    int
    Enabled     bool
    Kind        PluginKind
    Stage       string
}
```

### Giải thích thiết kế

- `Name`: định danh duy nhất toàn cục;
- `Version`: thuận tiện cho việc tương thích và triển khai theo giai đoạn (gray release) về sau;
- `Priority`: căn cứ để sắp xếp thứ tự;
- `Enabled`: công tắc bật/tắt khi đang chạy;
- `Kind`: phân biệt interceptor / observer;
- `Stage`: khai báo giai đoạn mà plugin được gắn vào.

## 7.2 Registry

V2 đề xuất đưa vào một trung tâm đăng ký (registry) tường minh, thay vì để Runtime tự quyết định "có những plugin nào".

```go
package hooks

type Registration struct {
    Meta     PluginMeta
    Register func(*Hub)
}

type Registry interface {
    Add(reg Registration)
    List() []Registration
}
```

### Registry chịu trách nhiệm gì

- Lưu trữ định nghĩa plugin;
- Cung cấp danh sách đăng ký có thể liệt kê (enumerable);
- Hỗ trợ lọc theo cấu hình xem có bật hay không;
- Cung cấp dữ liệu nền tảng cho việc debug và quan sát.

### Registry không chịu trách nhiệm gì

- Không trực tiếp thực thi plugin;
- Không trực tiếp mang trạng thái nghiệp vụ;
- Không thay thế logic thực thi của Runtime.

## 7.3 Lifecycle

Cung cấp vòng đời tối thiểu cho các plugin có trạng thái.

```go
package hooks

type Lifecycle interface {
    Init(context.Context) error
    Close() error
}
```

Đề xuất:

- Plugin không trạng thái (stateless) có thể không cần triển khai;
- Plugin có cache, background task, connection pool nên triển khai interface này;
- Runtime quản lý thống nhất thời điểm gọi.

## 7.4 Interface Interceptor

```go
package hooks

type Interceptor[T any] interface {
    Meta() PluginMeta
    Handle(Context, T) (T, bool, error)
}
```

Ý đồ thiết kế:

- Giữ lại ba năng lực "ghi đè + stop + error";
- Tận dụng generic để tăng cường ràng buộc tại thời điểm biên dịch;
- Giảm rủi ro dùng sai do `any` gây ra.

## 7.5 Interface Observer

```go
package hooks

type Observer[T any] interface {
    Meta() PluginMeta
    Handle(Context, T)
}
```

Ý đồ thiết kế:

- Làm rõ "chỉ quan sát, không ghi đè";
- Loại bỏ về mặt ngữ nghĩa việc dùng sai `stop`;
- Thuận tiện cho việc sau này áp dụng chiến lược lập lịch độc lập cho observer.

## 7.6 Async Runtime

### Vấn đề hiện tại

Vấn đề lớn nhất của mô hình thực thi async hiện tại không phải là "không hoạt động được", mà là "thiếu ranh giới (boundary)".

### Mục tiêu thiết kế của V2

Bổ sung cho async observer các năng lực sau:

- bounded queue (hàng đợi có giới hạn);
- timeout;
- thống kê dropped;
- chỉ số thực thi theo từng plugin (per-plugin);
- có thể mở rộng thêm đa hàng đợi / đa worker về sau.

### Cấu hình đề xuất

```go
package hooks

type AsyncConfig struct {
    QueueSize    int
    WorkerCount  int
    DropWhenFull bool
    Timeout      time.Duration
}
```

### Giá trị mặc định đề xuất

- `QueueSize = 1024`
- `WorkerCount = 1`
- `DropWhenFull = true`
- `Timeout = 200ms`

### Chiến lược đề xuất

1. Mặc định giữ single worker trước, đảm bảo ngữ nghĩa tuần tự;
2. Khi hàng đợi đầy, ưu tiên bỏ (drop) sự kiện observer, thay vì làm chậm luồng chính;
3. Ghi lại số lượng dropped và số lượng timeout;
4. Nếu về sau xuất hiện observer tải cao, mới xem xét tách hàng đợi theo sự kiện hoặc theo plugin.

---

## 8. Hợp đồng domain (Domain Contract)

V2 bắt buộc phải viết rõ ràng tường minh hợp đồng payload.

## 8.1 ASROutputData

Công dụng: ghi đè văn bản cuối cùng của ASR và kết quả nhận diện người nói (speaker).

| Trường          | Có được sửa không | Ghi chú                                        |
| --------------- | ----------------- | ---------------------------------------------- |
| `Text`          | Có                | Có thể làm sạch, chuẩn hóa, lọc                |
| `SpeakerResult` | Có                | Có thể sửa hoặc tăng cường thông tin người nói |

Ràng buộc:

- Plugin không được phép chặn (block) trong thời gian dài;
- `stop=true` nghĩa là văn bản của lượt này không tiếp tục đi vào LLM;
- Nếu trả về văn bản rỗng, plugin phải tự chịu trách nhiệm rõ ràng cho hậu quả.

## 8.2 LLMInputData

Công dụng: ghi đè tin nhắn và tool trước khi phát khởi request LLM.

| Trường            | Có được sửa không | Ghi chú                                                  |
| ----------------- | ----------------- | -------------------------------------------------------- |
| `UserMessage`     | Có                | Không được phép để trống                                 |
| `RequestMessages` | Có                | Có thể cắt bớt, sắp xếp lại, tiêm (inject) system prompt |
| `Tools`           | Có                | Có thể lọc hoặc thêm vào                                 |

Ràng buộc:

- `UserMessage` không được phép là `nil`;
- Plugin phải đảm bảo output vẫn thỏa mãn yêu cầu đầu vào tối thiểu của LLM Provider tuyến dưới;
- `stop=true` nghĩa là request LLM lần này bị chặn và kết thúc.

## 8.3 LLMOutputData

Công dụng: ghi đè văn bản hiển thị hoặc bổ sung ngữ nghĩa lỗi sau khi LLM output hoàn tất.

| Trường     | Có được sửa không | Ghi chú                                                                   |
| ---------- | ----------------- | ------------------------------------------------------------------------- |
| `FullText` | Có                | Có thể làm ghi đè an toàn, chỉnh format, làm cho thân thiện với giọng nói |
| `Err`      | Cẩn trọng         | Đề xuất bổ sung ngữ cảnh, không đề xuất nuốt trực tiếp lỗi gốc phía dưới  |

Ràng buộc:

- Không đề xuất plugin ghi đè lỗi thật gốc mà không để lại dấu vết;
- `stop=true` nghĩa là phía sau sẽ không tiếp tục đi vào TTS hoặc cập nhật tin nhắn;
- Về sau có thể tách `Err` thành `OriginErr` / `DisplayErr`.

## 8.4 TTSInputData

Công dụng: xử lý làm cho văn bản có thể đọc được (readable) trước khi đưa vào TTS.

| Trường    | Có được sửa không | Ghi chú                                           |
| --------- | ----------------- | ------------------------------------------------- |
| `Text`    | Có                | Có thể làm cho số, dấu câu, emoji có thể đọc được |
| `IsStart` | Mặc định không    | Coi là trường ranh giới giao thức                 |
| `IsEnd`   | Mặc định không    | Coi là trường ranh giới giao thức                 |

Ràng buộc:

- Plugin thông thường chỉ nên sửa `Text`;
- `IsStart` / `IsEnd` nên dành riêng cho plugin có quyền cao hơn hoặc plugin chuyên dụng;
- `stop=true` nghĩa là đoạn hiện tại không được đưa vào TTS.

## 8.5 MetricData

Công dụng: quan sát chuỗi xử lý.

| Trường  | Có được sửa không | Ghi chú                       |
| ------- | ----------------- | ----------------------------- |
| `Stage` | Không             | Chỉ-đọc                       |
| `Ts`    | Không             | Chỉ-đọc                       |
| `Err`   | Không             | Chỉ-đọc, chỉ dùng để quan sát |

Ràng buộc:

- Không được phép dừng (stop) luồng chính;
- Không được phép ghi đè rồi phản hồi ngược lại luồng chính;
- Chỉ dùng cho log, metric, trace, debug.

---

## 9. Mô hình cấu hình

Đề xuất bổ sung mô hình cấu hình tối thiểu cho hệ thống Hook:

```yaml
chat_hooks:
  enabled: true
  async:
    queue_size: 1024
    worker_count: 1
    drop_when_full: true
    timeout_ms: 200
  plugins:
    statistic_plugin:
      enabled: true
      priority: 100
```

Mục tiêu thiết kế cấu hình:

- Hỗ trợ bật/tắt plugin;
- Hỗ trợ ghi đè priority;
- Hỗ trợ kiểm soát tham số vận hành của async;
- Dành sẵn không gian cho schema cấu hình cấp-plugin trong tương lai.

---

## 10. Yêu cầu về khả năng quan sát (Observability)

Runtime tối thiểu cần thu thập các chỉ số sau:

- số lần gọi plugin;
- thời gian xử lý (duration) của plugin;
- số lần error;
- số lần stop (interceptor);
- số lần dropped (observer async);
- số lần timeout;
- độ dài async queue hiện tại.

Đề xuất log/metric nên bao gồm:

- `plugin_name`
- `plugin_kind`
- `stage`
- `priority`
- `duration_ms`
- `result`

Nếu sau này tích hợp thêm tracing, có thể ghi lại thêm:

- session_id
- device_id
- turn_id
- correlation_id

---

## 11. Kế hoạch di trú (Migration Plan)

## 11.1 Giai đoạn 1: Tăng cường Runtime (P0)

Mục tiêu: nâng cao độ ổn định trên tiền đề không thay đổi các điểm tích hợp nghiệp vụ.

Hạng mục công việc:

- Bổ sung bounded queue cho async observer;
- Bổ sung thống kê timeout và dropped;
- Bổ sung các chỉ số nền tảng cho Runtime;
- Giữ tương thích với các interface hiện có `Emit` / `RegisterSync` / `RegisterAsync`.

Kết quả đầu ra:

- Nâng cao độ ổn định;
- Cung cấp ranh giới cho việc mở rộng observer về sau.

## 11.2 Giai đoạn 2: Phân tầng ngữ nghĩa (P0)

Mục tiêu: phân biệt tường minh Interceptor và Observer.

Hạng mục công việc:

- Thêm façade rõ ràng mới trong tầng Domain Hook;
- Chuyển sự kiện `Metric` thành ngữ nghĩa observer chuẩn;
- Làm rõ tập hợp các sự kiện không được phép stop.

Kết quả đầu ra:

- Ngữ nghĩa rõ ràng hơn;
- Người viết plugin ít có khả năng dùng sai hơn.

## 11.3 Giai đoạn 3: Registry + Meta (P1)

Mục tiêu: tách "có những plugin nào" ra khỏi logic hard-code.

Hạng mục công việc:

- Đưa vào `PluginMeta`;
- Đưa vào `Registration` / `Registry`;
- Hỗ trợ bật/tắt plugin theo cấu hình;
- Hỗ trợ liệt kê các plugin hiện đang được tải.

Kết quả đầu ra:

- Việc đăng ký trở nên minh bạch;
- Thuận tiện cho debug, quan sát và quản trị cấu hình.

## 11.4 Giai đoạn 4: Hợp đồng và vòng đời (P1)

Mục tiêu: chính thức hóa ranh giới của plugin.

Hạng mục công việc:

- Chốt cứng (cố định) hợp đồng payload;
- Đưa vào `Lifecycle`;
- Bổ sung quy trình khởi tạo và đóng cho các plugin có trạng thái.

Kết quả đầu ra:

- Phù hợp hơn để mang các plugin built-in phức tạp;
- Thuận tiện cho việc tiến hóa lên hệ thống plugin hoàn chỉnh hơn.

## 11.5 Giai đoạn 5: Năng lực nâng cao (P2)

Mục tiêu: hỗ trợ hệ sinh thái plugin phức tạp hơn.

Hạng mục công việc:

- Tách hàng đợi theo sự kiện;
- Runtime observer đa worker;
- Tích hợp sâu tracing / metrics;
- Chiến lược thực thi cô lập dành cho các plugin nặng.

---

## 12. Những gì không phải mục tiêu

Hiện tại V2 **không theo đuổi**:

- Sandbox cho plugin bên thứ ba không đáng tin cậy;
- Hệ thống plugin RPC ngoài tiến trình (out-of-process);
- Hệ sinh thái plugin phức tạp có thể hot-load;
- Chợ plugin (plugin marketplace) hoàn chỉnh.

Các năng lực này nên được đánh giá ở các phiên bản cao hơn trong tương lai, không nên đưa vào sớm gây thêm độ phức tạp không cần thiết.

---

## 13. Đề xuất triển khai thực tế

Nếu trong đợt phát triển (iteration) hiện tại chỉ được phép làm 3 việc, thứ tự đề xuất như sau:

1. **Làm trước việc quản trị Async Runtime**
   - Đây là bước mang lại lợi ích ổn định lớn nhất.

2. **Sau đó làm việc tách ngữ nghĩa Interceptor / Observer**
   - Đây là bước hiệu quả nhất để giảm rủi ro dùng sai.

3. **Bổ sung hợp đồng và Meta/Registry**
   - Đây là bước then chốt để đưa hệ thống từ "dùng được" tiến lên "có thể quản lý được".

---

## 14. Tổng kết

Mục tiêu của V2 không phải là gói lại hệ thống Hook hiện tại thành một "Plugin Platform" nghe có vẻ hoành tráng hơn, mà là tiến hóa nó thành một khung mở rộng thực sự:

- Ngữ nghĩa rõ ràng;
- Thực thi có thể kiểm soát được;
- Có thể quan sát được;
- Có thể mở rộng dần dần;
- Tương thích với luồng chat chính hiện tại.

Do đó, điều quan trọng nhất của V2 không phải là "thêm nhiều plugin hơn", mà là hoàn thành trước ba việc sau:

- Phân định rõ **Interceptor** và **Observer**;
- Bổ sung đầy đủ **ranh giới của async runtime**;
- Xây dựng **hợp đồng payload và metadata plugin**.

Sau khi hoàn thành ba bước này, hệ thống Hook của repo hiện tại mới thực sự có được nền tảng để tiến hóa lâu dài.
