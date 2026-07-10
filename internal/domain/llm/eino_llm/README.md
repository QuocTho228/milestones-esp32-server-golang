# Eino LLM Provider - Triển khai đa nhà cung cấp thống nhất

## Tổng quan

EinoLLMProvider là triển khai nhà cung cấp LLM thống nhất dựa trên framework CloudWeGo Eino, hỗ trợ nhiều nhà cung cấp mô hình ngôn ngữ lớn khác nhau, bao gồm OpenAI và Ollama. Triển khai này sử dụng hoàn toàn các kiểu dữ liệu và interface gốc của Eino, mang lại trải nghiệm API nhất quán.

## Đặc điểm cốt lõi

### ✅ Hỗ trợ đa nhà cung cấp

- **OpenAI**: Hỗ trợ các model GPT-3.5, GPT-4...
- **Ollama**: Hỗ trợ các model mã nguồn mở triển khai local
- **Interface thống nhất**: Tất cả các nhà cung cấp sử dụng cùng một API

### ✅ Triển khai gốc theo Eino

- Sử dụng trực tiếp các kiểu `*schema.Message` và `*schema.ToolInfo`
- Gọi các phương thức `chatModel.Generate()` và `chatModel.Stream()`
- Hỗ trợ `chatModel.BindTools()` để gắn kết công cụ (tool binding)

### ✅ Hỗ trợ chức năng đầy đủ

- Phản hồi dạng streaming và không streaming
- Gọi công cụ (tool call) và gắn kết hàm (function binding)
- Kiểm soát và hủy context
- Gọi cấu hình theo dạng chuỗi (chain)

### ✅ Tính tương thích cao

- Triển khai interface `LLMProvider` chuẩn
- Hỗ trợ di chuyển liền mạch cho code hiện có
- Cung cấp chuyển đổi kiểu dữ liệu tương thích ngược

## Thiết kế kiến trúc

```
┌─────────────────────────────────────────────────────────────┐
│                    LLMProvider Interface                    │
│  Response() / ResponseWithFunctions() / ResponseWithContext() │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   EinoLLMProvider                          │
│  • Quản lý cấu hình thống nhất                              │
│  • Hỗ trợ đa nhà cung cấp                                    │
│  • Gọi theo chuỗi (chain call)                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                 Eino ChatModel Interface                   │
│  Generate() / Stream() / BindTools()                       │
└─────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    ▼                   ▼
┌─────────────────────────┐  ┌─────────────────────────┐
│   OpenAI ChatModel      │  │   Ollama ChatModel      │
│   (eino-ext/openai)     │  │   (eino-ext/ollama)     │
└─────────────────────────┘  └─────────────────────────┘
```

## Bắt đầu nhanh

### 1. Cấu hình cơ bản

```go
// Cấu hình OpenAI
openaiConfig := map[string]interface{}{
    "type":       "openai",
    "model_name": "gpt-3.5-turbo",
    "api_key":    "your-openai-api-key",
    "base_url":   "https://api.openai.com/v1",
    "max_tokens": 500,
    "streamable": true,
}

// Cấu hình Ollama
ollamaConfig := map[string]interface{}{
    "type":       "ollama",
    "model_name": "llama2",
    "base_url":   "http://localhost:11434",
    "max_tokens": 500,
    "streamable": true,
}
```

### 2. Tạo Provider

```go
// Tạo provider OpenAI
openaiProvider, err := NewEinoLLMProvider(openaiConfig)
if err != nil {
    log.Fatalf("Tạo provider OpenAI thất bại: %v", err)
}

// Tạo provider Ollama
ollamaProvider, err := NewEinoLLMProvider(ollamaConfig)
if err != nil {
    log.Fatalf("Tạo provider Ollama thất bại: %v", err)
}
```

### 3. Sử dụng kiểu message gốc của Eino

```go
messages := []*schema.Message{
    {
        Role:    schema.System,
        Content: "Bạn là một trợ lý hữu ích",
    },
    {
        Role:    schema.User,
        Content: "Hãy giới thiệu về framework Eino",
    },
}
```

### 4. Đối thoại cơ bản

```go
// Phản hồi dạng streaming
responseChan := provider.Response("session_id", messages)
for content := range responseChan {
    fmt.Print(content)
}
```

### 5. Gọi công cụ (Tool call)

```go
tools := []*schema.ToolInfo{
    {
        Name: "get_weather",
        ParamsOneOf: &schema.ParamsOneOf{
            // Định nghĩa tham số công cụ
        },
    },
}

toolResponseChan := provider.ResponseWithFunctions("session_id", messages, tools)
for response := range toolResponseChan {
    switch resp := response.(type) {
    case map[string]string:
        if resp["type"] == "content" {
            fmt.Print(resp["content"])
        }
    case map[string]interface{}:
        if resp["type"] == "tool_calls" {
            fmt.Printf("Gọi công cụ: %+v\n", resp["tool_calls"])
        }
    }
}
```

### 6. Gọi theo chuỗi (Chain call)

```go
enhancedProvider := provider.
    WithMaxTokens(1000).
    WithStreamable(false)

fmt.Printf("Loại provider: %s\n", enhancedProvider.GetProviderType())
fmt.Printf("Thông tin model: %+v\n", enhancedProvider.GetModelInfo())
```

## Tài liệu API

### Interface cốt lõi

#### `NewEinoLLMProvider(config map[string]interface{}) (*EinoLLMProvider, error)`

Tạo một instance provider Eino LLM mới.

**Tham số:**

- `config`: Bản đồ cấu hình, bắt buộc phải có trường `type`

**Trả về:**

- `*EinoLLMProvider`: Instance provider
- `error`: Thông tin lỗi

#### `Response(sessionID string, dialogue []*schema.Message) chan string`

Tạo phản hồi văn bản cơ bản.

#### `ResponseWithFunctions(sessionID string, dialogue []*schema.Message, functions []*schema.ToolInfo) chan interface{}`

Tạo phản hồi kèm gọi công cụ.

#### `ResponseWithContext(ctx context.Context, sessionID string, dialogue []*schema.Message) chan string`

Tạo phản hồi có kiểm soát context.

### Các tùy chọn cấu hình

| Trường       | Kiểu   | Bắt buộc | Mô tả                                          |
| ------------ | ------ | -------- | ---------------------------------------------- |
| `type`       | string | ✅       | Loại provider: "openai", "ollama"              |
| `model_name` | string | ✅       | Tên model                                      |
| `api_key`    | string | ⚠️       | API key (bắt buộc với OpenAI)                  |
| `base_url`   | string | ❌       | baseURL                                        |
| `max_tokens` | int    | ❌       | Số token tối đa (mặc định: 500)                |
| `streamable` | bool   | ❌       | Có hỗ trợ streaming hay không (mặc định: true) |

### Các phương thức chuỗi (Chain methods)

#### `WithMaxTokens(maxTokens int) *EinoLLMProvider`

Thiết lập số token tối đa, trả về instance provider mới.

#### `WithStreamable(streamable bool) *EinoLLMProvider`

Thiết lập hỗ trợ streaming, trả về instance provider mới.

#### `GetChatModel() model.ChatModel`

Lấy instance ChatModel gốc của Eino bên dưới.

#### `GetProviderType() string`

Lấy loại provider.

#### `GetModelInfo() map[string]interface{}`

Lấy thông tin và metadata của model.

## Cách sử dụng nâng cao

### Sử dụng trực tiếp Eino ChatModel

```go
chatModel := provider.GetChatModel()

// Gọi trực tiếp Generate
result, err := chatModel.Generate(ctx, messages)
if err != nil {
    log.Printf("Tạo phản hồi thất bại: %v", err)
    return
}
fmt.Printf("Kết quả: %s\n", result.Content)

// Gọi trực tiếp Stream
streamReader, err := chatModel.Stream(ctx, messages)
if err != nil {
    log.Printf("Gọi stream thất bại: %v", err)
    return
}
defer streamReader.Close()

for {
    message, err := streamReader.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Printf("Nhận dữ liệu thất bại: %v", err)
        break
    }
    fmt.Print(message.Content)
}
```

### Quản lý đa Provider

```go
providers := make(map[string]*EinoLLMProvider)

configs := map[string]map[string]interface{}{
    "openai": {
        "type":       "openai",
        "model_name": "gpt-3.5-turbo",
        "api_key":    "your-openai-key",
    },
    "ollama": {
        "type":       "ollama",
        "model_name": "llama2",
        "base_url":   "http://localhost:11434",
    },
}

for name, config := range configs {
    provider, err := NewEinoLLMProvider(config)
    if err != nil {
        log.Printf("Tạo provider %s thất bại: %v", name, err)
        continue
    }
    providers[name] = provider
}

// Sử dụng các provider khác nhau để xử lý cùng một yêu cầu
for name, provider := range providers {
    fmt.Printf("=== Phản hồi từ provider %s ===\n", name)
    responseChan := provider.Response("session", messages)
    for content := range responseChan {
        fmt.Print(content)
    }
    fmt.Println()
}
```

## Kiểm thử (Testing)

Chạy toàn bộ bộ test:

```bash
go test ./internal/domain/llm/eino_llm/... -v
```

### Phạm vi test bao phủ

- ✅ Tạo và cấu hình provider
- ✅ Hỗ trợ nhiều loại provider
- ✅ Chức năng đối thoại cơ bản
- ✅ Chức năng gọi công cụ
- ✅ Gọi theo chuỗi (chain call)
- ✅ Xử lý lỗi
- ✅ Kiểm thử hiệu năng (benchmark)

## Dependency

- `github.com/cloudwego/eino` v0.3.40+
- `github.com/cloudwego/eino-ext` v0.0.1-alpha+

## Thực hành tốt nhất

### 1. Xử lý lỗi

```go
provider, err := NewEinoLLMProvider(config)
if err != nil {
    log.Errorf("Tạo provider thất bại: %v", err)
    return
}
```

### 2. Kiểm soát context

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

responseChan := provider.ResponseWithContext(ctx, sessionID, messages)
```

### 3. Quản lý tài nguyên

```go
// Đối với phản hồi dạng streaming, đảm bảo tiêu thụ hết toàn bộ dữ liệu
for content := range responseChan {
    // Xử lý nội dung
}
```

### 4. Quản lý cấu hình

```go
// Sử dụng biến môi trường để quản lý thông tin nhạy cảm
config := map[string]interface{}{
    "type":       "openai",
    "model_name": "gpt-3.5-turbo",
    "api_key":    os.Getenv("OPENAI_API_KEY"),
}
```

## Hướng dẫn mở rộng

### Thêm Provider mới

Để thêm hỗ trợ cho một provider mới, cần:

1. Thêm triển khai mới trong hàm `createXXXChatModel`
2. Thêm case mới trong câu lệnh switch của `NewEinoLLMProvider`
3. Đảm bảo provider mới triển khai interface `model.ChatModel`

### Cấu hình tùy chỉnh

Có thể mở rộng bản đồ cấu hình để hỗ trợ các tùy chọn riêng của từng provider:

```go
config := map[string]interface{}{
    "type":        "openai",
    "model_name":  "gpt-4",
    "api_key":     "your-key",
    "temperature": 0.7,  // Tham số tùy chỉnh
    "top_p":       0.9,  // Tham số tùy chỉnh
}
```

## Lịch sử phiên bản

### v3.0.0 (Phiên bản hiện tại)

- ✅ Viết lại hoàn toàn dựa trên framework Eino
- ✅ Hỗ trợ đa provider (OpenAI, Ollama)
- ✅ Sử dụng kiểu dữ liệu gốc của Eino
- ✅ Gọi trực tiếp các phương thức Eino ChatModel
- ✅ Loại bỏ tầng adapter, nâng cao hiệu năng
- ✅ Bao phủ test đầy đủ

### v2.x.x (Đã lỗi thời)

- Triển khai hỗn hợp, sử dụng mẫu adapter (adapter pattern)
- Tích hợp Eino một phần

### v1.x.x (Đã lỗi thời)

- Dựa trên triển khai OpenAI truyền thống
- Không tích hợp Eino

## Đóng góp

Hoan nghênh gửi Issue và Pull Request để cải thiện triển khai này.

## Giấy phép (License)

Dự án này tuân theo giấy phép ở thư mục gốc của dự án.
