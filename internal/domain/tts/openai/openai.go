package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep"

	"milestones-esp32-server-golang/internal/data/audio"
	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"
)

// HTTP client toàn cục, dùng để triển khai connection pool (bể kết nối)
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// Lấy HTTP client đã được cấu hình connection pool
func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second, // OpenAI TTS có thể cần nhiều thời gian hơn
		}
	})
	return httpClient
}

// OpenAITTSProvider Provider TTS của OpenAI
type OpenAITTSProvider struct {
	APIKey         string
	APIURL         string
	Model          string
	Voice          string
	ResponseFormat string
	Speed          float64
	Stream         bool
	FrameDuration  int
}

// Cấu trúc request
type openAIRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
	Stream         bool    `json:"stream,omitempty"`
}

// NewOpenAITTSProvider tạo mới một Provider TTS của OpenAI
func NewOpenAITTSProvider(config map[string]interface{}) *OpenAITTSProvider {
	apiKey, _ := config["api_key"].(string)
	apiURL, _ := config["api_url"].(string)
	model, _ := config["model"].(string)
	voice, _ := config["voice"].(string)
	responseFormat, _ := config["response_format"].(string)
	speed, _ := config["speed"].(float64)
	stream, _ := config["stream"].(bool)
	frameDuration, _ := config["frame_duration"].(float64)

	// Thiết lập giá trị mặc định
	if apiURL == "" {
		apiURL = "https://api.openai.com/v1/audio/speech"
	}
	if model == "" {
		model = "tts-1" // tts-1 hoặc tts-1-hd
	}
	if voice == "" {
		voice = "alloy" // alloy, echo, fable, onyx, nova, shimmer
	}
	if responseFormat == "" {
		responseFormat = "mp3" // mp3, opus, aac, flac, wav, pcm
	}
	if speed == 0 {
		speed = 1.0 // từ 0.25 đến 4.0
	}
	if frameDuration == 0 {
		frameDuration = audio.FrameDuration
	}

	return &OpenAITTSProvider{
		APIKey:         apiKey,
		APIURL:         apiURL,
		Model:          model,
		Voice:          voice,
		ResponseFormat: responseFormat,
		Stream:         stream,
		Speed:          speed,
		FrameDuration:  int(frameDuration),
	}
}

// TextToSpeech chuyển văn bản thành giọng nói, trả về dữ liệu các khung âm thanh và lỗi (nếu có)
func (p *OpenAITTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	streamChan, err := p.TextToSpeechStream(ctx, text, sampleRate, channels, frameDuration)
	if err != nil {
		return nil, err
	}

	audioFrames := make([][]byte, 0, 32)
	for frame := range streamChan {
		audioFrames = append(audioFrames, frame)
	}
	if len(audioFrames) == 0 {
		return nil, fmt.Errorf("OpenAI TTS trả về âm thanh rỗng")
	}
	return audioFrames, nil
}

// TextToSpeechStream triển khai tổng hợp giọng nói dạng stream (luồng)
func (p *OpenAITTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {
	startTs := time.Now().UnixMilli()

	// Tạo request body
	reqBody := openAIRequest{
		Model:          p.Model,
		Input:          text,
		Voice:          p.Voice,
		ResponseFormat: p.ResponseFormat,
		Speed:          p.Speed,
		Stream:         p.Stream,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi tuần tự hóa (serialize) yêu cầu: %v", err)
	}

	//log.Debugf("OpenAI TTS请求: %s", string(jsonData))

	// Tạo HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("lỗi khi tạo yêu cầu: %v", err)
	}

	// Thiết lập header cho request
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))
	}

	// Sử dụng client đã có connection pool
	client := getHTTPClient()

	// Tạo channel đầu ra
	outputChan = make(chan []byte, 100)

	// Khởi chạy goroutine để xử lý phản hồi dạng stream
	go func() {
		// Gửi yêu cầu
		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("Gửi yêu cầu OpenAI thất bại: %v", err)
			close(outputChan)
			return
		}
		defer resp.Body.Close()

		// Kiểm tra mã trạng thái phản hồi
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("Yêu cầu OpenAI API thất bại, mã trạng thái: %d, phản hồi: %s", resp.StatusCode, string(body))
			close(outputChan)
			return
		}

		// Kiểm tra độ dài nội dung phản hồi
		contentLength := resp.ContentLength
		log.Debugf("Nhận được phản hồi OpenAI TTS, Content-Length: %d", contentLength)

		// Kiểm tra xem Content-Length có hợp lý không
		if contentLength == 0 {
			log.Errorf("OpenAI API trả về phản hồi rỗng, Content-Length bằng 0")
			close(outputChan)
			return
		}

		responseFormat := strings.ToLower(strings.TrimSpace(p.ResponseFormat))
		decoderFormat := responseFormat
		if responseFormat == "opus" {
			decoderFormat = "ogg_opus"
			contentTypeFormat := util.GetAudioFormatByMimeType(strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type"))))
			if contentTypeFormat == "ogg_opus" || contentTypeFormat == "opus" {
				decoderFormat = contentTypeFormat
			}
		}

		if decoderFormat != "mp3" && decoderFormat != "wav" && decoderFormat != "pcm" && decoderFormat != "opus" && decoderFormat != "ogg_opus" {
			log.Errorf("Hiện chỉ hỗ trợ tổng hợp dạng stream cho các định dạng mp3/wav/pcm/opus/ogg_opus")
			close(outputChan)
			return
		}

		decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, resp.Body, outputChan, frameDuration, decoderFormat, sampleRate)
		if err != nil {
			log.Errorf("Lỗi khi tạo bộ giải mã âm thanh OpenAI: %v", err)
			close(outputChan)
			return
		}
		if decoderFormat == "opus" {
			sourceChannels := channels
			if sourceChannels < 1 {
				sourceChannels = 1
			}
			decoder.WithFormat(beep.Format{
				SampleRate:  beep.SampleRate(util.NormalizeOpusSampleRate(sampleRate)),
				NumChannels: sourceChannels,
			})
		}

		if err := decoder.Run(startTs); err != nil {
			log.Errorf("Giải mã âm thanh OpenAI thất bại: %v", err)
			return
		}

		select {
		case <-ctx.Done():
			log.Debugf("Tổng hợp giọng nói dạng stream của OpenAI TTS đã bị hủy, văn bản: %s", text)
			return
		default:
			log.Infof("Thời gian xử lý OpenAI TTS: từ lúc nhập đến khi nhận xong dữ liệu âm thanh mất: %d ms", time.Now().UnixMilli()-startTs)
		}
	}()

	return outputChan, nil
}

// SetVoice thiết lập tham số giọng nói
func (p *OpenAITTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	if voice, ok := voiceConfig["voice"].(string); ok && voice != "" {
		p.Voice = voice
		return nil
	}
	return fmt.Errorf("cấu hình giọng nói không hợp lệ: thiếu voice")
}

// Close đóng tài nguyên (Provider không trạng thái, không cần đóng)
func (p *OpenAITTSProvider) Close() error {
	return nil
}

// IsValid kiểm tra tài nguyên có hợp lệ hay không
func (p *OpenAITTSProvider) IsValid() bool {
	return p != nil
}