package eino_llm

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"

	log "milestones-esp32-server-golang/logger"
)

// ExampleConfig Cấu hình mẫu (ví dụ)
var ExampleConfig = map[string]interface{}{
	"type":       "eino_llm",
	"model_name": "gpt-3.5-turbo",
	"api_key":    "your-api-key-here",
	"base_url":   "https://api.openai.com/v1",
	"max_tokens": 500,
	"streamable": true,
}

// ExampleUsage Minh họa cách sử dụng EinoLLMProvider
func ExampleUsage() {
	// 1. Ví dụ cấu hình OpenAI
	openaiConfig := map[string]interface{}{
		"type":       "openai",
		"model_name": "gpt-3.5-turbo",
		"api_key":    "your-openai-api-key",
		"base_url":   "https://api.openai.com/v1",
		"max_tokens": 500,
		"streamable": true,
	}

	// 2. Ví dụ cấu hình Ollama
	ollamaConfig := map[string]interface{}{
		"type":       "ollama",
		"model_name": "llama2",
		"base_url":   "http://localhost:11434",
		"max_tokens": 500,
		"streamable": true,
	}

	// 3. Tạo provider
	openaiProvider, err := NewEinoLLMProvider(openaiConfig)
	if err != nil {
		log.Errorf("Tạo provider OpenAI thất bại: %v", err)
		return
	}

	ollamaProvider, err := NewEinoLLMProvider(ollamaConfig)
	if err != nil {
		log.Errorf("Tạo provider Ollama thất bại: %v", err)
		return
	}

	// 4. Sử dụng kiểu tin nhắn gốc của Eino
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "Bạn là một trợ lý hữu ích",
		},
		{
			Role:    schema.User,
			Content: "Vui lòng giới thiệu về framework Eino",
		},
	}

	// 5. Hội thoại cơ bản
	fmt.Println("=== Hội thoại cơ bản với OpenAI ===")
	responseChan := openaiProvider.ResponseWithContext(context.Background(), "example_session", messages, nil)
	for resp := range responseChan {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("Gọi công cụ (tool call): %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()

	fmt.Println("=== Hội thoại cơ bản với Ollama ===")
	responseChan = ollamaProvider.ResponseWithContext(context.Background(), "example_session", messages, nil)
	for resp := range responseChan {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("Gọi công cụ (tool call): %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()

	// 6. Ví dụ gọi công cụ (tool call)
	tools := []*schema.ToolInfo{
		{
			Name:        "get_weather",
			ParamsOneOf: &schema.ParamsOneOf{
				// Định nghĩa tham số công cụ
			},
		},
	}

	fmt.Println("=== Hội thoại kèm gọi công cụ ===")
	toolResponseChan := openaiProvider.ResponseWithContext(context.Background(), "example_session", messages, tools)
	for resp := range toolResponseChan {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("Gọi công cụ (tool call): %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()

	// 7. Ví dụ gọi theo chuỗi (chain call)
	fmt.Println("=== Ví dụ gọi theo chuỗi (chain call) ===")
	enhancedProvider := openaiProvider.
		WithMaxTokens(1000).
		WithStreamable(false)

	fmt.Printf("Loại provider: %s\n", enhancedProvider.GetProviderType())
	fmt.Printf("Thông tin model: %+v\n", enhancedProvider.GetModelInfo())
}

// ExampleAdvancedUsage Ví dụ sử dụng nâng cao
func ExampleAdvancedUsage() {
	config := map[string]interface{}{
		"type":       "openai",
		"model_name": "gpt-4",
		"api_key":    "your-api-key",
		"max_tokens": 1000,
		"streamable": true,
	}

	provider, err := NewEinoLLMProvider(config)
	if err != nil {
		log.Errorf("Tạo provider thất bại: %v", err)
		return
	}

	// Sử dụng context để kiểm soát
	ctx := context.Background()
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Vui lòng viết một bài viết dài về AI",
		},
	}

	fmt.Println("=== Hội thoại có kiểm soát bằng context ===")
	responseChan := provider.ResponseWithContext(ctx, "advanced_session", messages, nil)
	for resp := range responseChan {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("Gọi công cụ (tool call): %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()

	// Sử dụng trực tiếp Eino ChatModel
	chatModel := provider.GetChatModel()
	result, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Errorf("Gọi trực tiếp ChatModel thất bại: %v", err)
		return
	}

	fmt.Printf("Kết quả gọi trực tiếp: %s\n", result.Content)
}

// ExampleMultiProvider Ví dụ nhiều provider
func ExampleMultiProvider() {
	providers := make(map[string]*EinoLLMProvider)

	// Tạo nhiều provider
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
			log.Errorf("Tạo provider %s thất bại: %v", name, err)
			continue
		}
		providers[name] = provider
	}

	// Sử dụng các provider khác nhau để xử lý cùng một request
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Xin chào, hãy tự giới thiệu về bạn",
		},
	}

	for name, provider := range providers {
		fmt.Printf("=== Phản hồi từ provider %s ===\n", name)
		responseChan := provider.ResponseWithContext(context.Background(), "multi_session", messages, nil)
		for resp := range responseChan {
			if resp.Content != "" {
				fmt.Print(resp.Content)
			}
			if len(resp.ToolCalls) > 0 {
				fmt.Printf("Gọi công cụ (tool call): %+v\n", resp.ToolCalls)
			}
		}
		fmt.Println()
	}
}

// ExampleWithTools Ví dụ gọi công cụ (tool call)
func ExampleWithTools() {
	provider, err := NewEinoLLMProvider(ExampleConfig)
	if err != nil {
		log.Errorf("Tạo provider thất bại: %v", err)
		return
	}

	// Sử dụng kiểu tin nhắn gốc của Eino
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Hôm nay thời tiết ở Thượng Hải thế nào? Vui lòng giúp tôi tra cứu.",
		},
	}

	// Sử dụng kiểu công cụ (tool) gốc của Eino
	tools := []*schema.ToolInfo{
		{
			Name:        "get_weather",
			ParamsOneOf: &schema.ParamsOneOf{
				// Định nghĩa tham số công cụ đơn giản hóa
				// Trong sử dụng thực tế, cần định nghĩa đúng cấu trúc tham số ở đây
			},
		},
	}

	fmt.Println("=== Ví dụ gọi công cụ (tool call) ===")

	// Sử dụng interface gọi công cụ gốc của Eino
	fmt.Println("--- Gọi công cụ gốc của Eino ---")
	responseChan := provider.ResponseWithContext(context.Background(), "tool_session", messages, tools)
	for resp := range responseChan {
		fmt.Printf("Phản hồi: %+v\n", resp)
	}
}

// MultiProviderExample Ví dụ nhiều provider
func MultiProviderExample() {
	// Ví dụ provider OpenAI
	fmt.Println("=== Ví dụ provider OpenAI ===")
	openaiConfig := map[string]interface{}{
		"type":       "openai",
		"model_name": "gpt-3.5-turbo",
		"api_key":    "your-openai-api-key",
		"base_url":   "https://api.openai.com/v1",
		"max_tokens": 500,
	}

	openaiProvider, err := NewEinoLLMProvider(openaiConfig)
	if err != nil {
		log.Errorf("Tạo provider OpenAI thất bại: %v", err)
		return
	}

	fmt.Printf("Loại provider: %s\n", openaiProvider.GetProviderType())

	// Ví dụ provider Ollama
	fmt.Println("\n=== Ví dụ provider Ollama ===")
	ollamaConfig := map[string]interface{}{
		"type":       "ollama",
		"model_name": "llama2",
		"base_url":   "http://localhost:11434",
		"max_tokens": 500,
	}

	ollamaProvider, err := NewEinoLLMProvider(ollamaConfig)
	if err != nil {
		log.Errorf("Tạo provider Ollama thất bại: %v", err)
		return
	}

	fmt.Printf("Loại provider: %s\n", ollamaProvider.GetProviderType())

	// Sử dụng kiểu tin nhắn gốc của Eino
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Vui lòng tự giới thiệu về bạn.",
		},
	}

	// Kiểm thử lần lượt hai provider
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n--- Phản hồi từ OpenAI ---")
	openaiResponse := openaiProvider.ResponseWithContext(ctx, "openai_session", messages, nil)
	for resp := range openaiResponse {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("Gọi công cụ (tool call): %+v\n", resp.ToolCalls)
		}
	}

	fmt.Println("\n--- Phản hồi từ Ollama ---")
	ollamaResponse := ollamaProvider.ResponseWithContext(ctx, "ollama_session", messages, nil)
	for resp := range ollamaResponse {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("Gọi công cụ (tool call): %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()
}

// EinoFrameworkAdvantages Mô tả ưu điểm của framework Eino
func EinoFrameworkAdvantages() string {
	return `
Những ưu điểm chính của framework Eino:

1. **Thiết kế theo thành phần (component-based)**
   - Bộ trừu tượng thành phần phong phú (ChatModel, Tool, ChatTemplate, Retriever, v.v.)
   - Mỗi thành phần đều có interface đầu vào/đầu ra thống nhất
   - Hỗ trợ lồng ghép thành phần và đóng gói logic nghiệp vụ phức tạp

2. **Khả năng điều phối (orchestration) mạnh mẽ**
   - Điều phối luồng dữ liệu dựa trên đồ thị (graph)
   - Tự động xử lý kiểm tra kiểu dữ liệu, xử lý luồng (stream), quản lý đồng thời
   - Hỗ trợ thực thi rẽ nhánh, quản lý trạng thái, ánh xạ trường dữ liệu

3. **Xử lý luồng (streaming) hoàn chỉnh**
   - Tự động nối tiếp các khối dữ liệu dạng luồng
   - Tự động đóng gói dữ liệu không phải luồng thành luồng
   - Tự động hợp nhất nhiều luồng
   - Tự động sao chép luồng đến nhiều node phía sau (downstream)

4. **Khả năng mở rộng cao**
   - Hỗ trợ bộ xử lý callback tùy chỉnh
   - Hỗ trợ năm loại aspect (OnStart, OnEnd, OnError, v.v.)
   - Có thể chèn (inject) các mối quan tâm xuyên suốt (cross-cutting concerns) như log, tracing, giám sát

5. **Sẵn sàng cho production**
   - Cơ chế xử lý lỗi hoàn chỉnh
   - Hỗ trợ thao tác timeout và hủy (cancel)
   - Connection pool và tối ưu hiệu năng
   - Log và giám sát chi tiết

Đặc điểm của phiên bản triển khai này:

**Hỗ trợ đa provider**:
- Interface Eino thống nhất hỗ trợ cả OpenAI và Ollama
- Chuyển đổi provider linh hoạt thông qua cấu hình type
- Mỗi provider đều sử dụng cùng một interface Eino ChatModel

**Triển khai gốc theo Eino (Eino-native)**:
- Sử dụng trực tiếp kiểu *schema.Message để hội thoại
- Sử dụng trực tiếp kiểu *schema.ToolInfo để gọi công cụ
- Xây dựng hoàn toàn dựa trên framework Eino, không cần chuyển đổi kiểu dữ liệu

**Tính năng nâng cao**:
- Hỗ trợ gọi theo chuỗi (chain call) (WithMaxTokens, WithStreamable)
- Xử lý lỗi và ghi log thống nhất
- Hỗ trợ chế độ gọi cả streaming và non-streaming
- Tương thích hoàn toàn với interface LLMProvider ban đầu

**Thực hành tốt nhất**:
- Hỗ trợ hủy theo context và kiểm soát timeout
- Tích hợp log có cấu trúc và giám sát
- Quản lý cấu hình an toàn về kiểu dữ liệu (type-safe)
- Tự động quản lý và dọn dẹp tài nguyên

Cách triển khai này thực sự tận dụng được năng lực cốt lõi của framework Eino, đồng thời hỗ trợ nhiều nhà cung cấp LLM khác nhau.
`
}

// BasicUsageExample Ví dụ sử dụng cơ bản
func BasicUsageExample() {
	provider, err := NewEinoLLMProvider(ExampleConfig)
	if err != nil {
		log.Errorf("Tạo provider thất bại: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Minh họa cấu hình theo chuỗi (chain configuration)
	enhancedProvider := provider.
		WithMaxTokens(2000).
		WithStreamable(true)

	// Lấy ChatModel gốc của Eino ở tầng dưới
	chatModel := enhancedProvider.GetChatModel()
	fmt.Printf("ChatModel tầng dưới: %+v\n", chatModel)

	// Lấy loại provider
	providerType := enhancedProvider.GetProviderType()
	fmt.Printf("Loại provider: %s\n", providerType)

	// Lấy thông tin model đã được nâng cao
	modelInfo := enhancedProvider.GetModelInfo()
	fmt.Printf("Thông tin model đã nâng cao: %+v\n", modelInfo)

	// Ví dụ hội thoại phức tạp - sử dụng kiểu tin nhắn gốc của Eino
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "Bạn là một kiến trúc sư phần mềm chuyên nghiệp, thành thạo ngôn ngữ Go và phát triển ứng dụng AI.",
		},
		{
			Role:    schema.User,
			Content: "Vui lòng thiết kế kiến trúc hệ thống chatbot dựa trên framework Eino.",
		},
	}

	// Sử dụng cấu hình đã nâng cao để gọi
	responseChan := enhancedProvider.ResponseWithContext(ctx, "basic_example", messages, nil)
	fmt.Printf("Phản hồi thiết kế kiến trúc:\n")
	for resp := range responseChan {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("Gọi công cụ (tool call): %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()
}

// EinoNativeExample Ví dụ API gốc của Eino
func EinoNativeExample() {
	provider, err := NewEinoLLMProvider(ExampleConfig)
	if err != nil {
		log.Errorf("Tạo provider thất bại: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Sử dụng kiểu tin nhắn gốc của Eino
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "Bạn là một trợ lý AI hữu ích.",
		},
		{
			Role:    schema.User,
			Content: "Vui lòng giới thiệu ngắn gọn về framework Eino.",
		},
	}

	fmt.Println("=== Ví dụ API gốc của Eino ===")

	// 1. Sử dụng EinoResponse
	fmt.Println("--- EinoResponse ---")
	responseChan := provider.ResponseWithContext(ctx, "eino_session", messages, nil)
	for resp := range responseChan {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("Gọi công cụ (tool call): %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()

	// 2. Sử dụng EinoResponseWithTools
	fmt.Println("\n--- EinoResponseWithTools ---")
	tools := []*schema.ToolInfo{
		{
			Name:        "search_docs",
			ParamsOneOf: &schema.ParamsOneOf{
				// Định nghĩa tham số công cụ
			},
		},
	}

	toolResponseChan := provider.ResponseWithContext(ctx, "eino_tools_session", messages, tools)
	for resp := range toolResponseChan {
		if resp.Content != "" {
			fmt.Printf("Nội dung: %s\n", resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("Gọi công cụ (tool call): %+v\n", resp.ToolCalls)
		}
	}
}

func main() {
	BasicUsageExample()
}