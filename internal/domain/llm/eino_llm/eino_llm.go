package eino_llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	log "milestones-esp32-server-golang/logger"
)

// EinoLLMProvider Nhà cung cấp LLM dựa trên framework Eino
// Sử dụng trực tiếp interface và các kiểu ChatModel của Eino, hỗ trợ openai và ollama
type EinoLLMProvider struct {
	chatModel        model.ToolCallingChatModel
	modelName        string
	maxTokens        int
	streamable       bool
	config           map[string]interface{}
	providerType     string // "openai" hoặc "ollama"
	reasoningTracker *reasoningContentTracker
}

// EinoConfig Cấu hình Eino LLM
type EinoConfig struct {
	Type       string                 `json:"type"` // "openai" hoặc "ollama"
	ModelName  string                 `json:"model_name"`
	APIKey     string                 `json:"api_key"`
	BaseURL    string                 `json:"base_url"`
	MaxTokens  int                    `json:"max_tokens"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Streamable bool                   `json:"streamable,omitempty"`
}

// Cấu hình connection pool
const (
	maxIdleConns          = 200
	maxIdleConnsPerHost   = 50
	idleConnTimeout       = 90 * time.Second
	dialTimeout           = 30 * time.Second
	keepAliveTimeout      = 30 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	responseHeaderTimeout = 60 * time.Second
)

// HTTP client toàn cục, dùng cho tất cả các request OpenAI
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// getHTTPClient Trả về HTTP client đã được cấu hình connection pool
func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: keepAliveTimeout,
			}).DialContext,
			MaxIdleConns:          maxIdleConns,
			MaxIdleConnsPerHost:   maxIdleConnsPerHost,
			IdleConnTimeout:       idleConnTimeout,
			TLSHandshakeTimeout:   tlsHandshakeTimeout,
			ResponseHeaderTimeout: responseHeaderTimeout,
			ExpectContinueTimeout: 1 * time.Second,
			DisableKeepAlives:     false,
		}

		httpClient = &http.Client{
			Transport: transport,
			// Trong trường hợp streaming output, không dùng http.Client.Timeout để cắt ngang toàn bộ kết nối,
			// mà chuyển sang dùng ctx để kiểm soát vòng đời của request.
			Timeout: 0,
		}
	})

	return httpClient
}

// NewEinoLLMProvider Tạo mới một Eino LLM provider, hỗ trợ openai và ollama tùy theo type
func NewEinoLLMProvider(config map[string]interface{}) (*EinoLLMProvider, error) {
	//log.Debugf("NewEinoLLMProvider config: %+v", config)
	var tracker *reasoningContentTracker
	if enabled, _ := config[reasoningDetectConfigKey].(bool); enabled {
		tracker = &reasoningContentTracker{}
		config[reasoningTrackerConfigKey] = tracker
	}
	parsedConfig, err := decodeOpenAICompatibleConfig(config)
	if err != nil {
		return nil, fmt.Errorf("phân tích cấu hình LLM thất bại: %v", err)
	}

	providerType := parsedConfig.Type
	if providerType == "" {
		return nil, fmt.Errorf("type không được để trống, phải là 'openai' hoặc 'ollama'")
	}

	modelName := parsedConfig.ModelName
	if modelName == "" {
		return nil, fmt.Errorf("model_name không được để trống")
	}

	maxTokens := 500
	if parsedConfig.MaxTokens != nil {
		maxTokens = *parsedConfig.MaxTokens
	}

	streamable := true
	if parsedConfig.Streamable != nil {
		streamable = *parsedConfig.Streamable
	}

	var chatModel model.ToolCallingChatModel

	// Tạo các implementation ChatModel khác nhau tùy theo type
	switch providerType {
	case "openai":
		chatModel, err = createOpenAIChatModel(config)
		if err != nil {
			return nil, fmt.Errorf("tạo OpenAI ChatModel thất bại: %v", err)
		}
	case "ollama":
		chatModel, err = createOllamaChatModel(config)
		if err != nil {
			return nil, fmt.Errorf("tạo Ollama ChatModel thất bại: %v", err)
		}
	default:
		return nil, fmt.Errorf("không hỗ trợ loại model: %s", providerType)
	}

	provider := &EinoLLMProvider{
		chatModel:        chatModel,
		modelName:        modelName,
		maxTokens:        maxTokens,
		streamable:       streamable,
		config:           config,
		providerType:     providerType,
		reasoningTracker: tracker,
	}

	return provider, nil
}

func (p *EinoLLMProvider) HasReasoningContent() bool {
	return p != nil && p.reasoningTracker != nil && p.reasoningTracker.HasReturned()
}

// createOpenAIChatModel Tạo implementation ChatModel cho OpenAI
func createOpenAIChatModel(config map[string]interface{}) (model.ToolCallingChatModel, error) {
	ctx := context.Background()

	parsedConfig, err := decodeOpenAICompatibleConfig(config)
	if err != nil {
		return nil, fmt.Errorf("phân tích cấu hình tương thích OpenAI thất bại: %v", err)
	}

	modelName := parsedConfig.ModelName
	if modelName == "" {
		modelName = "gpt-3.5-turbo"
	}

	apiKey := parsedConfig.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	httpClient := buildThinkingHTTPClient(config, getHTTPClient())
	useMaxCompletionTokens := shouldUseMaxCompletionTokens(parsedConfig.Provider, modelName)

	// Tạo cấu hình OpenAI ChatModel
	openaiConfig := &openai.ChatModelConfig{
		Model:      modelName,
		APIKey:     apiKey,
		HTTPClient: httpClient,
	}

	if parsedConfig.BaseURL != "" {
		openaiConfig.BaseURL = parsedConfig.BaseURL
	}
	if parsedConfig.APIVersion != "" {
		openaiConfig.APIVersion = parsedConfig.APIVersion
	}
	if !useMaxCompletionTokens && parsedConfig.MaxTokens != nil && *parsedConfig.MaxTokens > 0 {
		openaiConfig.MaxTokens = parsedConfig.MaxTokens
	}
	if parsedConfig.Temperature != nil {
		openaiConfig.Temperature = parsedConfig.Temperature
	}
	if parsedConfig.TopP != nil {
		openaiConfig.TopP = parsedConfig.TopP
	}

	log.Debugf("openaiConfig: %+v", openaiConfig)

	// Sử dụng implementation OpenAI chính thức của eino-ext
	chatModel, err := openai.NewChatModel(ctx, openaiConfig)
	if err != nil {
		return nil, fmt.Errorf("tạo OpenAI ChatModel thất bại: %v", err)
	}

	log.Infof("Tạo OpenAI ChatModel thành công, model: %s", modelName)
	return chatModel, nil
}

// createOllamaChatModel Tạo implementation ChatModel cho Ollama
func createOllamaChatModel(config map[string]interface{}) (model.ToolCallingChatModel, error) {
	ctx := context.Background()

	modelName, _ := config["model_name"].(string)
	baseURL, _ := config["base_url"].(string)

	if modelName == "" || baseURL == "" {
		log.Warnf("model_name và base_url không được để trống, sử dụng model mặc định: %s", modelName)
		return nil, fmt.Errorf("model_name và base_url không được để trống")
	}

	// Tạo cấu hình Ollama ChatModel
	ollamaConfig := &ollama.ChatModelConfig{
		BaseURL: baseURL,
		Model:   modelName,
	}

	// Sử dụng implementation Ollama chính thức của eino-ext
	chatModel, err := ollama.NewChatModel(ctx, ollamaConfig)
	if err != nil {
		return nil, fmt.Errorf("tạo Ollama ChatModel thất bại: %v", err)
	}

	log.Infof("Tạo Ollama ChatModel thành công, model: %s", modelName)
	return chatModel, nil
}

// GetModelInfo Lấy thông tin model
func (p *EinoLLMProvider) GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"model_name":      p.modelName,
		"max_tokens":      p.maxTokens,
		"streamable":      p.streamable,
		"type":            "eino",
		"provider_type":   p.providerType,
		"framework":       "eino",
		"adapter_version": "3.0.0",
		"base_url":        p.config["base_url"],
	}
}

// ResponseWithFunctions Phản hồi kèm function calling, sử dụng kiểu tool gốc của Eino, gọi trực tiếp EinoResponseWithTools
func (p *EinoLLMProvider) ResponseWithContext(ctx context.Context, sessionID string, dialogue []*schema.Message, functions []*schema.ToolInfo) chan *schema.Message {

	log.Infof("[Eino-LLM] Bắt đầu xử lý request kèm tool - SessionID: %s, Type: %s", sessionID, p.providerType)

	logMessages(dialogue)
	// Gọi trực tiếp EinoResponseWithTools để lấy phản hồi gốc của Eino
	einoResponseChan := p.EinoResponseWithTools(ctx, sessionID, dialogue, functions)

	log.Infof("[Eino-LLM] Xử lý request gọi tool hoàn tất - SessionID: %s", sessionID)

	return einoResponseChan
}

func logMessages(messages []*schema.Message) {
	for _, msg := range messages {
		if msg == nil {
			log.Debugf("history llm msg: <nil>")
			continue
		}
		log.Debugf("history llm msg: %s\n", msg.String())
	}
}

// llmExtraErrorKey Giữ nhất quán với domain/llm.LLMExtraErrorKey, dùng để truyền lỗi khi thất bại (tránh circular dependency)
const llmExtraErrorKey = "error"

// sendLLMError Gửi message lỗi kèm Extra.error vào channel
func sendLLMError(ch chan *schema.Message, err error) {
	ch <- &schema.Message{
		Role:  schema.System,
		Extra: map[string]any{llmExtraErrorKey: err.Error()},
	}
}

// isRateLimitError kiểm tra lỗi có phải do rate limit (HTTP 429) không
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests")
}

// retryAfterRe bắt cụm "try again in 12.12s" (định dạng lỗi rate limit của Groq)
var retryAfterRe = regexp.MustCompile(`try again in ([\d.]+)s`)

// parseRetryAfter cố gắng đọc thời gian chờ được provider gợi ý từ nội dung lỗi.
// Nếu không tìm thấy hoặc parse thất bại, trả về giá trị fallback.
func parseRetryAfter(err error, fallback time.Duration) time.Duration {
	if err == nil {
		return fallback
	}
	matches := retryAfterRe.FindStringSubmatch(err.Error())
	if len(matches) < 2 {
		return fallback
	}
	seconds, parseErr := strconv.ParseFloat(matches[1], 64)
	if parseErr != nil || seconds <= 0 {
		return fallback
	}
	// Thêm chút buffer để chắc chắn rate limit đã reset phía provider
	return time.Duration(seconds*1000)*time.Millisecond + 300*time.Millisecond
}

// EinoResponseWithTools Sử dụng trực tiếp kiểu Eino để phản hồi kèm tool
func (p *EinoLLMProvider) EinoResponseWithTools(ctx context.Context, sessionID string, messages []*schema.Message, tools []*schema.ToolInfo) chan *schema.Message {
	responseChan := make(chan *schema.Message, 200)

	var err error
	go func() {
		defer close(responseChan)
		if p.reasoningTracker != nil {
			p.reasoningTracker.Reset()
		}

		log.Infof("[Eino-LLM] Bắt đầu xử lý request tool của Eino - SessionID: %s, tools: %+v", sessionID, tools)

		// Nếu có tool, cần bind tool vào ChatModel
		if len(tools) > 0 {
			p.chatModel, err = p.chatModel.WithTools(tools)
			if err != nil {
				log.Errorf("Bind tool thất bại: %v", err)
				sendLLMError(responseChan, err)
				return
			}
		}

		if p.streamable {
			log.Debugf("EinoLLMProvider.EinoResponseWithTools() streamable: %t", p.streamable)
			// Sử dụng trực tiếp phương thức Stream của Eino
			// Nếu gặp lỗi rate limit (429), thử lại tối đa maxRetries lần trước khi fallback sang Generate,
			// dựa vào thời gian chờ mà provider gợi ý trong nội dung lỗi (nếu có).
			const maxRetries = 2
			var streamReader *schema.StreamReader[*schema.Message]
			var err error
			for attempt := 0; attempt <= maxRetries; attempt++ {
				streamReader, err = p.chatModel.Stream(ctx, messages, p.buildModelCallOptions()...)
				if err == nil {
					break
				}
				if !isRateLimitError(err) || attempt == maxRetries {
					log.Errorf("Gọi stream tool của Eino thất bại: %v", err)
					break
				}
				waitDuration := parseRetryAfter(err, 5*time.Second)
				log.Warnf("Bị giới hạn tốc độ (429) khi gọi Stream, thử lại sau %v (lần %d/%d)", waitDuration, attempt+1, maxRetries)
				select {
				case <-time.After(waitDuration):
				case <-ctx.Done():
					sendLLMError(responseChan, ctx.Err())
					return
				}
			}
			if err != nil {
				// Đối với implementation mock, nếu Stream thất bại thì fallback sang Generate
				message, genErr := p.chatModel.Generate(ctx, messages, p.buildModelCallOptions()...)
				if genErr != nil {
					log.Errorf("Sinh phản hồi tool của Eino thất bại: %v", genErr)
					sendLLMError(responseChan, genErr)
					return
				}
				if message != nil {
					responseChan <- message
				}
				return
			}

			if streamReader != nil {
				defer streamReader.Close()

				var currentToolCall *schema.ToolCall
				var toolCallBuffer string
				var isToolCallComplete bool
				var streamChunkCount int

				// Xử lý phản hồi dạng stream
				for {
					message, err := streamReader.Recv()
					//log.Debugf("streamReader.Recv() message: %+v", message)
					if err == io.EOF {
						if streamChunkCount == 0 {
							sendLLMError(responseChan, errors.New("phản hồi stream rỗng"))
							break
						}
						// Nếu còn tool call chưa hoàn thành, gửi lần cuối
						if currentToolCall != nil {
							completeMessage := &schema.Message{
								Role:      schema.Assistant,
								ToolCalls: []schema.ToolCall{*currentToolCall},
							}
							responseChan <- completeMessage
						}
						break
					}
					if err != nil {
						if ctxErr := ctx.Err(); ctxErr != nil {
							if errors.Is(ctxErr, context.Canceled) {
								log.Debugf("Phản hồi stream đã bị hủy: %v", ctxErr)
							} else {
								log.Warnf("Phản hồi stream đã kết thúc: %v", ctxErr)
							}
							break
						}
						log.Errorf("Nhận phản hồi stream thất bại: %v", err)
						sendLLMError(responseChan, err)
						break
					}

					if message != nil {
						streamChunkCount++
						// Kiểm tra xem có phải là bắt đầu của một tool call không
						if len(message.ToolCalls) > 0 {
							toolCall := message.ToolCalls[0]

							if toolCall.Function.Name != "" {
								// Bắt đầu tool call mới
								currentToolCall = &toolCall
								toolCallBuffer = toolCall.Function.Arguments
								isToolCallComplete = false
							} else if currentToolCall != nil {
								// Tích lũy tham số của tool call
								toolCallBuffer += toolCall.Function.Arguments
								currentToolCall.Function.Arguments = toolCallBuffer

								// Kiểm tra xem tham số đã là JSON hoàn chỉnh chưa
								if isValidJSON(toolCallBuffer) {
									isToolCallComplete = true
								}
							}

							// Nếu tool call đã hoàn chỉnh, gửi message
							if isToolCallComplete {
								completeMessage := &schema.Message{
									Role:      schema.Assistant,
									ToolCalls: []schema.ToolCall{*currentToolCall},
								}
								responseChan <- completeMessage

								// Reset trạng thái
								currentToolCall = nil
								toolCallBuffer = ""
								isToolCallComplete = false
							}
						} else if message.Content != "" {
							// Gửi message thông thường không phải tool call
							message.ToolCalls = nil
							responseChan <- message
						}
					}
				}
			} else {
				sendLLMError(responseChan, errors.New("phản hồi stream rỗng"))
			}
		} else {
			// Sử dụng trực tiếp phương thức Generate của Eino
			message, err := p.chatModel.Generate(ctx, messages, p.buildModelCallOptions()...)
			if err != nil {
				log.Errorf("Sinh phản hồi tool của Eino thất bại: %v", err)
				sendLLMError(responseChan, err)
				return
			}

			if message != nil {
				responseChan <- message
			}
		}

		log.Infof("[Eino-LLM] Xử lý request tool của Eino hoàn tất - SessionID: %s", sessionID)
	}()

	return responseChan
}

func (p *EinoLLMProvider) buildModelCallOptions() []model.Option {
	if p == nil || p.maxTokens <= 0 {
		return nil
	}

	provider := ""
	if p.config != nil {
		if rawProvider, ok := p.config["provider"].(string); ok {
			provider = rawProvider
		}
	}

	if shouldUseMaxCompletionTokens(provider, p.modelName) {
		return nil
	}

	return []model.Option{model.WithMaxTokens(p.maxTokens)}
}

// isValidJSON Kiểm tra xem chuỗi có phải là JSON hợp lệ hay không
func isValidJSON(str string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(str), &js) == nil
}

// GetChatModel Lấy ChatModel gốc bên dưới của Eino
func (p *EinoLLMProvider) GetChatModel() model.ToolCallingChatModel {
	return p.chatModel
}

// GetProviderType Lấy loại provider
func (p *EinoLLMProvider) GetProviderType() string {
	return p.providerType
}

// WithMaxTokens Thiết lập số token tối đa
func (p *EinoLLMProvider) WithMaxTokens(maxTokens int) *EinoLLMProvider {
	newProvider := *p
	newProvider.maxTokens = maxTokens
	return &newProvider
}

// WithStreamable Thiết lập có hỗ trợ streaming hay không
func (p *EinoLLMProvider) WithStreamable(streamable bool) *EinoLLMProvider {
	newProvider := *p
	newProvider.streamable = streamable
	return &newProvider
}

// Close Đóng tài nguyên (Provider không trạng thái, không cần đóng)
func (p *EinoLLMProvider) Close() error {
	return nil
}

// IsValid Kiểm tra tài nguyên có hợp lệ hay không
func (p *EinoLLMProvider) IsValid() bool {
	return p != nil && p.chatModel != nil
}