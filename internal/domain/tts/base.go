package tts

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"milestones-esp32-server-golang/constants"
	"milestones-esp32-server-golang/internal/domain/tts/cosyvoice"
	"milestones-esp32-server-golang/internal/domain/tts/doubao"
	"milestones-esp32-server-golang/internal/domain/tts/edge"
	"milestones-esp32-server-golang/internal/domain/tts/edge_offline"
	"milestones-esp32-server-golang/internal/domain/tts/minimax"
	"milestones-esp32-server-golang/internal/domain/tts/openai"
	"milestones-esp32-server-golang/internal/domain/tts/qwen"
	"milestones-esp32-server-golang/internal/domain/tts/streaming"
	"milestones-esp32-server-golang/internal/domain/tts/milestones"
	"milestones-esp32-server-golang/internal/domain/tts/xunfei"
	"milestones-esp32-server-golang/internal/domain/tts/xunfei_super_tts"
	"milestones-esp32-server-golang/internal/domain/tts/zhipu"
)

// BaseTTSProvider là interface TTS cơ bản (không bao gồm phương thức Context)
type BaseTTSProvider interface {
	TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error)
	TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error)
}

// DualStreamProvider là interface tùy chọn cho TTS mà cả đầu vào và đầu ra đều ở dạng streaming: vừa nhận văn bản vừa tổng hợp đầu ra. Provider nào hỗ trợ thì sẽ triển khai interface này.
type DualStreamProvider interface {
	StreamingSynthesize(ctx context.Context, textChan <-chan string, sampleRate int, channels int, frameDuration int) (outputChan chan streaming.SynthesisEvent, err error)
}

// TTSProvider là interface TTS đầy đủ (bao gồm phương thức Context)
type TTSProvider interface {
	BaseTTSProvider
	// SetVoice thiết lập tham số âm sắc (voice) một cách động
	// voiceConfig: map chứa cấu hình liên quan đến âm sắc, ví dụ {"voice": "xxx"} hoặc {"spk_id": "xxx"}
	SetVoice(voiceConfig map[string]interface{}) error
	// Close đóng tài nguyên, giải phóng kết nối, v.v.
	Close() error
	// IsValid kiểm tra tài nguyên có hợp lệ hay không (kết nối còn sống hay không, v.v.)
	IsValid() bool
}

// GetTTSProvider lấy về một nhà cung cấp TTS đầy đủ (hỗ trợ Context)
// providerName: có thể là config_id/provider hoặc key của resource pool (ví dụ "edge_tts:zh-CN-XiaoxiaoNeural")
// config: map cấu hình được phân tích (parse) từ trường json_data của bảng configs trong cơ sở dữ liệu
// Ưu tiên sử dụng trường provider trong config, nếu không thì phân tích từ providerName (lấy phần trước dấu ":")
func GetTTSProvider(providerName string, config map[string]interface{}) (TTSProvider, error) {
	effectiveName := providerName
	if configProvider, ok := config["provider"].(string); ok && configProvider != "" {
		effectiveName = configProvider
	}
	// key của resource pool có định dạng "provider:voiceID", lấy nửa đầu làm loại provider
	if idx := strings.Index(effectiveName, ":"); idx > 0 {
		effectiveName = effectiveName[:idx]
	}
	var baseProvider BaseTTSProvider

	switch effectiveName {
	case constants.TtsTypeDoubao:
		baseProvider = doubao.NewDoubaoTTSProvider(config)
	case constants.TtsTypeDoubaoWS:
		baseProvider = doubao.NewDoubaoWSProvider(config)
	case constants.TtsTypeCosyvoice:
		baseProvider = cosyvoice.NewCosyVoiceTTSProvider(config)
	case constants.TtsTypeEdge:
		baseProvider = edge.NewEdgeTTSProvider(config)
	case constants.TtsTypeEdgeOffline:
		baseProvider = edge_offline.NewEdgeOfflineTTSProvider(config)
	case constants.TtsTypeMilestones:
		baseProvider = milestones.NewMilestonesProvider(config)
	case constants.TtsTypeXunfei:
		baseProvider = xunfei.NewXunfeiTTSProvider(config)
	case constants.TtsTypeXunfeiSuper:
		baseProvider = xunfei_super_tts.NewXunfeiSuperTTSProvider(config)
	case constants.TtsTypeOpenAI:
		baseProvider = openai.NewOpenAITTSProvider(config)
	case constants.TtsTypeZhipu:
		baseProvider = zhipu.NewZhipuTTSProvider(config)
	case constants.TtsTypeMinimax:
		baseProvider = minimax.NewMinimaxTTSProvider(config)
	case constants.TtsTypeAliyunQwen:
		baseProvider = qwen.NewQwenTTSProvider(config)
	case constants.TtsTypeIndexTTSVLLM:
		baseProvider = openai.NewOpenAITTSProvider(buildIndexTTSOpenAIConfig(config))
	default:
		return nil, fmt.Errorf("Nhà cung cấp TTS không được hỗ trợ: %s", effectiveName)
	}

	if baseProvider == nil {
		return nil, fmt.Errorf("Không thể tạo nhà cung cấp TTS: %s", effectiveName)
	}

	// Dùng adapter để bọc provider cơ bản, chuyển đổi thành TTSProvider đầy đủ
	provider := &ContextTTSAdapter{baseProvider}

	return provider, nil
}

func buildIndexTTSOpenAIConfig(config map[string]interface{}) map[string]interface{} {
	const (
		defaultIndexTTSURL   = "http://127.0.0.1:7860/audio/speech"
		defaultIndexTTSModel = "indextts-vllm"
	)

	normalized := make(map[string]interface{}, len(config)+4)
	for k, v := range config {
		normalized[k] = v
	}

	apiURL, _ := normalized["api_url"].(string)
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		apiURL = defaultIndexTTSURL
	} else {
		parsed, err := url.Parse(apiURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			trimmed := strings.TrimRight(apiURL, "/")
			if !strings.HasSuffix(strings.ToLower(trimmed), "/audio/speech") {
				trimmed += "/audio/speech"
			}
			apiURL = trimmed
		} else {
			if strings.TrimSpace(parsed.Path) == "" || parsed.Path == "/" {
				parsed.Path = "/audio/speech"
				parsed.RawPath = ""
				apiURL = parsed.String()
			}
		}
	}
	normalized["api_url"] = strings.TrimRight(apiURL, "/")

	if model, _ := normalized["model"].(string); strings.TrimSpace(model) == "" {
		normalized["model"] = defaultIndexTTSModel
	}
	if responseFormat, _ := normalized["response_format"].(string); strings.TrimSpace(responseFormat) == "" {
		normalized["response_format"] = "wav"
	}
	if _, exists := normalized["stream"]; !exists {
		normalized["stream"] = false
	}
	if _, exists := normalized["speed"]; !exists {
		normalized["speed"] = float64(1.0)
	}

	return normalized
}

// ContextTTSAdapter là một adapter, bổ sung hỗ trợ Context cho provider TTS cơ bản
type ContextTTSAdapter struct {
	Provider BaseTTSProvider
}

// StreamingSynthesize ủy quyền (proxy) tới interface tổng hợp song luồng của provider gốc
func (a *ContextTTSAdapter) StreamingSynthesize(ctx context.Context, textChan <-chan string, sampleRate int, channels int, frameDuration int) (outputChan chan streaming.SynthesisEvent, err error) {
	// Kiểm tra xem Provider gốc bên dưới có hỗ trợ song luồng hay không
	if dsProvider, ok := a.Provider.(DualStreamProvider); ok {
		return dsProvider.StreamingSynthesize(ctx, textChan, sampleRate, channels, frameDuration)
	}
	return nil, fmt.Errorf("Provider gốc bên dưới không hỗ trợ tổng hợp song luồng")
}

// TextToSpeech ủy quyền tới provider gốc
func (a *ContextTTSAdapter) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	return a.Provider.TextToSpeech(ctx, text, sampleRate, channels, frameDuration)
}

// TextToSpeechStream ủy quyền tới provider gốc
func (a *ContextTTSAdapter) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {
	return a.Provider.TextToSpeechStream(ctx, text, sampleRate, channels, frameDuration)
}

// SetVoice ủy quyền tới phương thức SetVoice của Provider gốc bên dưới
func (a *ContextTTSAdapter) SetVoice(voiceConfig map[string]interface{}) error {
	// Nếu Provider gốc bên dưới đã triển khai phương thức SetVoice thì gọi trực tiếp
	if setter, ok := a.Provider.(interface {
		SetVoice(map[string]interface{}) error
	}); ok {
		return setter.SetVoice(voiceConfig)
	}
	// Nếu không thì trả về lỗi không được hỗ trợ
	return fmt.Errorf("Provider gốc bên dưới không hỗ trợ phương thức SetVoice")
}

// TextToSpeechWithContext là phiên bản chuyển văn bản thành giọng nói có hỗ trợ Context
func (a *ContextTTSAdapter) TextToSpeechWithContext(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	// Kiểm tra xem provider có hỗ trợ trực tiếp phiên bản Context hay không
	if provider, ok := a.Provider.(interface {
		TextToSpeechWithContext(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error)
	}); ok {
		// Provider hỗ trợ trực tiếp phiên bản Context
		return provider.TextToSpeechWithContext(ctx, text, sampleRate, channels, frameDuration)
	}

	// Nếu không thì dùng phiên bản chuẩn, và triển khai kiểm soát context thông qua goroutine và channel
	resultChan := make(chan struct {
		frames [][]byte
		err    error
	})

	go func() {
		frames, err := a.Provider.TextToSpeech(ctx, text, sampleRate, channels, frameDuration)
		select {
		case <-ctx.Done():
			// Context đã bị hủy, không gửi kết quả
			return
		case resultChan <- struct {
			frames [][]byte
			err    error
		}{frames, err}:
			// Kết quả đã được gửi
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultChan:
		return result.frames, result.err
	}
}

// TextToSpeechStreamWithContext là phiên bản streaming của chuyển văn bản thành giọng nói có hỗ trợ Context
func (a *ContextTTSAdapter) TextToSpeechStreamWithContext(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, cancelFunc func(), err error) {
	// Kiểm tra xem provider có hỗ trợ trực tiếp phiên bản Context hay không
	if provider, ok := a.Provider.(interface {
		TextToSpeechStreamWithContext(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (chan []byte, func(), error)
	}); ok {
		// Provider hỗ trợ trực tiếp phiên bản Context
		return provider.TextToSpeechStreamWithContext(ctx, text, sampleRate, channels, frameDuration)
	}

	// Nếu không thì dùng phiên bản chuẩn, nhưng tạo một wrapper để xử lý việc hủy context
	streamCtx, cancel := context.WithCancel(ctx)
	streamChan, err := a.Provider.TextToSpeechStream(streamCtx, text, sampleRate, channels, frameDuration)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	cancelFunc = cancel

	// Tạo một channel đầu ra mới, dùng để chuyển tiếp dữ liệu và xử lý việc hủy
	outputChan = make(chan []byte, 10)

	// Tạo một goroutine để chuyển tiếp dữ liệu và theo dõi việc hủy context
	go func() {
		defer close(outputChan)

		for {
			select {
			case <-streamCtx.Done():
				// Context đã bị hủy, gọi hàm hủy gốc rồi thoát
				cancelFunc()
				return
			case frame, ok := <-streamChan:
				if !ok {
					// Channel gốc đã đóng
					return
				}
				// Chuyển tiếp dữ liệu
				select {
				case <-streamCtx.Done():
					// Context đã bị hủy
					cancelFunc()
					return
				case outputChan <- frame:
					// Chuyển tiếp dữ liệu thành công
				}
			}
		}
	}()

	return outputChan, cancelFunc, nil
}

// Close đóng tài nguyên
func (a *ContextTTSAdapter) Close() error {
	// Nếu Provider gốc bên dưới đã triển khai phương thức Close thì gọi trực tiếp
	if closer, ok := a.Provider.(interface {
		Close() error
	}); ok {
		return closer.Close()
	}
	return nil
}

// IsValid kiểm tra tài nguyên có hợp lệ hay không
func (a *ContextTTSAdapter) IsValid() bool {
	// Nếu Provider gốc bên dưới đã triển khai phương thức IsValid thì gọi trực tiếp
	if validator, ok := a.Provider.(interface {
		IsValid() bool
	}); ok {
		return validator.IsValid()
	}
	// Nếu không thì kiểm tra xem Provider có phải nil hay không
	return a.Provider != nil
}