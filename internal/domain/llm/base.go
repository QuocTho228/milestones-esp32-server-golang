package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"milestones-esp32-server-golang/constants"
	"milestones-esp32-server-golang/internal/domain/llm/coze_llm"
	"milestones-esp32-server-golang/internal/domain/llm/dify_llm"
	"milestones-esp32-server-golang/internal/domain/llm/eino_llm"
)

// LLMExtraErrorKey Quy ước truyền lỗi xuyên suốt: key được dùng trong Message.Extra khi ResponseWithContext thất bại
const LLMExtraErrorKey = "error"

// IsLLMErrorMessage Kiểm tra xem có phải là tin nhắn lỗi được LLM truyền ra hay không (Extra có chứa error)
func IsLLMErrorMessage(msg *schema.Message) bool {
	if msg == nil || msg.Extra == nil {
		return false
	}
	v, ok := msg.Extra[LLMExtraErrorKey]
	if !ok || v == nil {
		return false
	}
	_, ok = v.(string)
	return ok
}

// LLMErrorMessage Phân tích nội dung lỗi từ Message.Extra (nếu là tin nhắn lỗi)
func LLMErrorMessage(msg *schema.Message) string {
	if msg == nil || msg.Extra == nil {
		return ""
	}
	v, ok := msg.Extra[LLMExtraErrorKey].(string)
	if !ok {
		return ""
	}
	return v
}

// LLMProvider Interface của nhà cung cấp mô hình ngôn ngữ lớn
// Tất cả các triển khai LLM phải tuân theo interface này, sử dụng kiểu dữ liệu gốc của Eino
type LLMProvider interface {
	// ResponseWithContext Phản hồi có kiểm soát bằng context, hỗ trợ thao tác hủy
	// ctx: context, có thể dùng để hủy các request chạy trong thời gian dài
	// sessionID: định danh phiên (session)
	// dialogue: lịch sử hội thoại, sử dụng kiểu tin nhắn gốc của Eino
	ResponseWithContext(ctx context.Context, sessionID string, dialogue []*schema.Message, functions []*schema.ToolInfo) chan *schema.Message

	ResponseWithVllm(ctx context.Context, file []byte, text string, mimeType string) (string, error)

	// GetModelInfo Lấy thông tin model
	// Trả về tên model và các metadata khác
	GetModelInfo() map[string]interface{}
	// Close Đóng tài nguyên, giải phóng kết nối, v.v.
	Close() error
	// IsValid Kiểm tra tài nguyên có hợp lệ hay không
	IsValid() bool
}

// LLMFactory Interface factory của mô hình ngôn ngữ lớn
// Dùng để tạo các nhà cung cấp LLM thuộc nhiều loại khác nhau
type LLMFactory interface {
	// CreateProvider Tạo nhà cung cấp LLM dựa theo cấu hình
	CreateProvider(config map[string]interface{}) (LLMProvider, error)
}

// GetLLMProvider Tạo nhà cung cấp LLM
// Thống nhất sử dụng EinoLLMProvider để xử lý tất cả các loại
func GetLLMProvider(providerName string, config map[string]interface{}) (LLMProvider, error) {
	cfg := cloneConfigMap(config)
	if providerName != "" {
		if _, ok := cfg["provider"]; !ok {
			cfg["provider"] = providerName
		}
	}

	llmType := resolveLLMType(providerName, cfg)
	cfg["type"] = llmType
	providerKey := resolveLLMProviderName(providerName, cfg, llmType)
	if defaultBaseURL := resolveDefaultBaseURL(providerKey); defaultBaseURL != "" {
		cfg["base_url"] = defaultBaseURL
	} else if baseURL, _ := cfg["base_url"].(string); strings.TrimSpace(baseURL) == "" {
		delete(cfg, "base_url")
	}

	switch llmType {
	case constants.LlmTypeOpenai, constants.LlmTypeOllama, constants.LlmTypeEinoLLM, constants.LlmTypeEino:
		// Thống nhất sử dụng EinoLLMProvider để xử lý tất cả các loại
		provider, err := eino_llm.NewEinoLLMProvider(cfg)
		if err != nil {
			return nil, fmt.Errorf("tạo Eino LLM provider thất bại: %v", err)
		}
		return provider, nil
	case constants.LlmTypeDify:
		provider, err := dify_llm.NewDifyLLMProvider(cfg)
		if err != nil {
			return nil, fmt.Errorf("tạo Dify LLM provider thất bại: %v", err)
		}
		return provider, nil
	case constants.LlmTypeCoze:
		provider, err := coze_llm.NewCozeLLMProvider(cfg)
		if err != nil {
			return nil, fmt.Errorf("tạo Coze LLM provider thất bại: %v", err)
		}
		return provider, nil
	}
	return nil, fmt.Errorf("LLM provider không được hỗ trợ: %s", llmType)
}

func resolveLLMProviderName(providerName string, config map[string]interface{}, llmType string) string {
	provider := strings.ToLower(strings.TrimSpace(providerName))
	if provider == "" {
		if rawProvider, ok := config["provider"].(string); ok {
			provider = strings.ToLower(strings.TrimSpace(rawProvider))
		}
	}
	if provider == "openai" {
		switch llmType {
		case constants.LlmTypeOllama:
			return "ollama"
		case constants.LlmTypeDify:
			return "dify"
		case constants.LlmTypeCoze:
			return "coze"
		}
	}
	return provider
}

func resolveDefaultBaseURL(provider string) string {
	switch provider {
	case "anthropic":
		return "https://api.anthropic.com/v1/"
	case "zhipu":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "aliyun":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case "doubao":
		return "https://ark.cn-beijing.volces.com/api/v3"
	case "siliconflow":
		return "https://api.siliconflow.cn/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	default:
		return ""
	}
}

func resolveLLMType(providerName string, config map[string]interface{}) string {
	provider := strings.ToLower(strings.TrimSpace(providerName))
	if provider == "" {
		if rawProvider, ok := config["provider"].(string); ok {
			provider = strings.ToLower(strings.TrimSpace(rawProvider))
		}
	}

	llmType, _ := config["type"].(string)
	llmType = strings.ToLower(strings.TrimSpace(llmType))

	if provider == "openai" {
		switch llmType {
		case constants.LlmTypeOllama:
			return constants.LlmTypeOllama
		case constants.LlmTypeDify:
			return constants.LlmTypeDify
		case constants.LlmTypeCoze:
			return constants.LlmTypeCoze
		}
	}

	switch provider {
	case "ollama":
		return constants.LlmTypeOllama
	case "dify":
		return constants.LlmTypeDify
	case "coze":
		return constants.LlmTypeCoze
	case "openai", "azure", "anthropic", "zhipu", "aliyun", "doubao", "siliconflow", "deepseek":
		return constants.LlmTypeOpenai
	}

	switch llmType {
	case constants.LlmTypeOllama:
		return constants.LlmTypeOllama
	case constants.LlmTypeDify:
		return constants.LlmTypeDify
	case constants.LlmTypeCoze:
		return constants.LlmTypeCoze
	case constants.LlmTypeOpenai, constants.LlmTypeEinoLLM, constants.LlmTypeEino:
		return constants.LlmTypeOpenai
	default:
		return constants.LlmTypeOpenai
	}
}

// Config Cấu trúc cấu hình LLM
type Config struct {
	ModelName  string                 `json:"model_name"`
	APIKey     string                 `json:"api_key"`
	BaseURL    string                 `json:"base_url"`
	MaxTokens  int                    `json:"max_tokens"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

func cloneConfigMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return make(map[string]interface{})
	}

	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}