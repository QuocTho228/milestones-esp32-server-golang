package zhipu

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"milestones-esp32-server-golang/internal/data/audio"
	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"

	"github.com/gopxl/beep"
	sse "github.com/tmaxmax/go-sse"
)

// HTTP client toàn cục, triển khai connection pool (bể kết nối)
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

const (
	zhipuDefaultSampleRate = 24000
	zhipuLeadingFadeInMs   = 5
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
			Timeout:   60 * time.Second,
		}
	})
	return httpClient
}

// ZhipuTTSProvider là nhà cung cấp TTS của Zhipu
type ZhipuTTSProvider struct {
	APIKey         string
	APIURL         string
	Model          string
	Voice          string
	ResponseFormat string
	Speed          float64
	Volume         float64
	Stream         bool
	EncodeFormat   string // Chỉ dùng khi ở chế độ streaming: base64 hoặc hex
	FrameDuration  int
}

// Cấu trúc yêu cầu (request) (theo tài liệu API của Zhipu)
type zhipuRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
	Volume         float64 `json:"volume,omitempty"`
	Stream         bool    `json:"stream,omitempty"`
	EncodeFormat   string  `json:"encode_format,omitempty"` // Chỉ dùng khi ở chế độ streaming: base64 hoặc hex
}

// Cấu trúc phản hồi Event Stream (tương tự định dạng của OpenAI)
type zhipuEventStreamResponse struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason,omitempty"`
		Delta        struct {
			Role             string `json:"role,omitempty"`
			Content          string `json:"content,omitempty"` // Dữ liệu âm thanh được mã hóa base64
			ReturnSampleRate int    `json:"return_sample_rate,omitempty"`
			ReturnFormat     string `json:"return_format,omitempty"`
		} `json:"delta"`
	} `json:"choices"`
}

// NewZhipuTTSProvider tạo một nhà cung cấp TTS Zhipu mới
func NewZhipuTTSProvider(config map[string]interface{}) *ZhipuTTSProvider {
	apiKey, _ := config["api_key"].(string)
	apiURL, _ := config["api_url"].(string)
	model, _ := config["model"].(string)
	voice, _ := config["voice"].(string)
	responseFormat, _ := config["response_format"].(string)
	speed, _ := config["speed"].(float64)
	volume, _ := config["volume"].(float64)
	stream, _ := config["stream"].(bool)
	encodeFormat, _ := config["encode_format"].(string)
	frameDuration, _ := config["frame_duration"].(float64)

	// Thiết lập giá trị mặc định
	if apiURL == "" {
		apiURL = "https://open.bigmodel.cn/api/paas/v4/audio/speech"
	}
	if model == "" {
		model = "glm-tts"
	}
	if voice == "" {
		voice = "tongtong" // Âm sắc (voice) mặc định
	}
	if responseFormat == "" {
		responseFormat = "pcm" // Zhipu mặc định dùng pcm, cũng hỗ trợ wav
	}
	if speed == 0 {
		speed = 1.0 // Từ 0.5 đến 2.0
	}
	if volume == 0 {
		volume = 1.0 // Từ 0 đến 10
	}
	if encodeFormat == "" {
		encodeFormat = "base64" // Mặc định base64, cũng hỗ trợ hex
	}
	if frameDuration == 0 {
		frameDuration = audio.FrameDuration
	}

	return &ZhipuTTSProvider{
		APIKey:         apiKey,
		APIURL:         apiURL,
		Model:          model,
		Voice:          voice,
		ResponseFormat: responseFormat,
		Stream:         stream,
		Speed:          speed,
		Volume:         volume,
		EncodeFormat:   encodeFormat,
		FrameDuration:  int(frameDuration),
	}
}

// TextToSpeech chuyển văn bản thành giọng nói, trả về dữ liệu khung âm thanh và lỗi (nếu có)
func (p *ZhipuTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	startTs := time.Now().UnixMilli()

	// Giới hạn độ dài văn bản (API Zhipu tối đa 1024 ký tự)
	if len(text) > 1024 {
		text = text[:1024]
		log.Warnf("Độ dài văn bản vượt quá 1024 ký tự, đã bị cắt bớt")
	}

	// Tạo phần thân yêu cầu (request body)
	reqBody := zhipuRequest{
		Model:          p.Model,
		Input:          text,
		Voice:          p.Voice,
		ResponseFormat: p.ResponseFormat,
		Speed:          p.Speed,
		Volume:         p.Volume,
		Stream:         false, // Không phải chế độ streaming
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("Tuần tự hóa (serialize) yêu cầu thất bại: %v", err)
	}

	// Tạo yêu cầu HTTP
	req, err := http.NewRequestWithContext(ctx, "POST", p.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("Tạo yêu cầu thất bại: %v", err)
	}

	// Thiết lập header cho yêu cầu
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	// Gửi yêu cầu bằng connection pool
	client := getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gửi yêu cầu thất bại: %v", err)
	}
	defer resp.Body.Close()

	// Kiểm tra mã trạng thái phản hồi
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Yêu cầu API thất bại, mã trạng thái: %d, phản hồi: %s", resp.StatusCode, string(body))
	}

	// Kiểm tra độ dài nội dung phản hồi
	contentLength := resp.ContentLength
	log.Debugf("Nhận được phản hồi TTS của Zhipu, Content-Length: %d", contentLength)

	// Kiểm tra xem Content-Length có hợp lý không
	if contentLength == 0 {
		log.Errorf("API trả về phản hồi rỗng, Content-Length bằng 0")
		return nil, fmt.Errorf("API trả về phản hồi rỗng, Content-Length bằng 0")
	}

	// Xử lý phản hồi theo định dạng âm thanh (Zhipu chỉ hỗ trợ wav và pcm)
	if p.ResponseFormat == "wav" || p.ResponseFormat == "pcm" {
		audioReader := io.ReadCloser(resp.Body)
		if strings.EqualFold(p.ResponseFormat, "pcm") {
			pcmData, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("Đọc dữ liệu PCM của Zhipu thất bại: %v", err)
			}
			audioReader = io.NopCloser(bytes.NewReader(
				applyPCM16MonoLeadingFadeIn(pcmData, leadingFadeInSampleCount(zhipuDefaultSampleRate, zhipuLeadingFadeInMs)),
			))
		}

		// Tạo một channel để thu thập các khung âm thanh
		outputChan := make(chan []byte, 1000)

		// Tạo bộ giải mã âm thanh
		decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, audioReader, outputChan, frameDuration, p.ResponseFormat, sampleRate)
		if err != nil {
			return nil, fmt.Errorf("Tạo bộ giải mã âm thanh thất bại: %v", err)
		}
		if strings.EqualFold(p.ResponseFormat, "pcm") {
			decoder.WithFormat(beep.Format{
				SampleRate:  beep.SampleRate(zhipuDefaultSampleRate),
				NumChannels: 1,
			})
		}

		// Bắt đầu quá trình giải mã
		go func() {
			if err := decoder.Run(startTs); err != nil {
				log.Errorf("Giải mã âm thanh thất bại: %v", err)
			}
		}()

		// Thu thập tất cả các khung âm thanh
		var audioFrames [][]byte
		for frame := range outputChan {
			audioFrames = append(audioFrames, frame)
		}

		log.Debugf("Zhipu TTS hoàn tất, thời gian từ lúc nhập đến khi nhận xong dữ liệu âm thanh: %d ms", time.Now().UnixMilli()-startTs)
		return audioFrames, nil
	}

	return nil, fmt.Errorf("Định dạng âm thanh không được hỗ trợ: %s, Zhipu chỉ hỗ trợ wav và pcm", p.ResponseFormat)
}

// TextToSpeechStream triển khai tổng hợp giọng nói dạng streaming
func (p *ZhipuTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {
	startTs := time.Now().UnixMilli()

	// Giới hạn độ dài văn bản (API Zhipu tối đa 1024 ký tự)
	if len(text) > 1024 {
		text = text[:1024]
		log.Warnf("Độ dài văn bản vượt quá 1024 ký tự, đã bị cắt bớt")
	}

	// Ở chế độ streaming chỉ hỗ trợ định dạng pcm và wav
	responseFormat := p.ResponseFormat

	// Tạo phần thân yêu cầu (request body)
	reqBody := zhipuRequest{
		Model:          p.Model,
		Input:          text,
		Voice:          p.Voice,
		ResponseFormat: responseFormat,
		Speed:          p.Speed,
		Volume:         p.Volume,
		Stream:         true,           // Streaming
		EncodeFormat:   p.EncodeFormat, // Sử dụng định dạng mã hóa đã cấu hình
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("Tuần tự hóa (serialize) yêu cầu thất bại: %v", err)
	}

	// Tạo yêu cầu HTTP
	req, err := http.NewRequestWithContext(ctx, "POST", p.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("Tạo yêu cầu thất bại: %v", err)
	}

	// Thiết lập header cho yêu cầu
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	// Tạo client bằng connection pool
	client := getHTTPClient()

	// Tạo channel đầu ra
	outputChan = make(chan []byte, 100)

	// Khởi chạy goroutine để xử lý phản hồi dạng streaming
	go func() {
		// Gửi yêu cầu
		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("Gửi yêu cầu tới Zhipu thất bại: %v", err)
			close(outputChan)
			return
		}
		defer resp.Body.Close()

		// Kiểm tra mã trạng thái phản hồi
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("Yêu cầu API Zhipu thất bại, mã trạng thái: %d, phản hồi: %s", resp.StatusCode, string(body))
			close(outputChan)
			return
		}

		// Kiểm tra xem Content-Type có phải là Event Stream không
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/event-stream") {
			log.Warnf("Content-Type mà API Zhipu trả về không phải text/event-stream: %s", contentType)
		}

		// Ở chế độ streaming chỉ hỗ trợ định dạng pcm và wav
		//log.Debugf("responseFormat (yêu cầu) dạng streaming của Zhipu TTS: %s", responseFormat)
		if responseFormat == "pcm" || responseFormat == "wav" {
			// Tạo pipe để truyền dữ liệu nhị phân đã giải mã tới bộ giải mã âm thanh
			pipeReader, pipeWriter := io.Pipe()

			// Khởi chạy goroutine để phân tích Event Stream và giải mã
			go func() {
				defer func() {
					if err := pipeWriter.Close(); err != nil {
						log.Debugf("Đóng đầu ghi của pipe thất bại: %v", err)
					}
				}()

				// Gọi phương thức phân tích độc lập
				if err := p.parseEventStream(ctx, resp.Body, pipeWriter, text); err != nil {
					log.Errorf("Phân tích Event Stream thất bại: %v", err)
				}
			}()

			// Tạo bộ giải mã âm thanh, đọc dữ liệu nhị phân đã giải mã từ pipe
			decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, responseFormat, sampleRate)
			if err != nil {
				log.Errorf("Tạo bộ giải mã âm thanh Zhipu thất bại: %v", err)
				pipeReader.Close()
				close(outputChan)
				return
			}
			if strings.EqualFold(responseFormat, "pcm") {
				decoder.WithFormat(beep.Format{
					SampleRate:  beep.SampleRate(zhipuDefaultSampleRate),
					NumChannels: 1,
				})
			}

			// Bắt đầu quá trình giải mã
			if err := decoder.Run(startTs); err != nil {
				log.Errorf("Giải mã âm thanh Zhipu thất bại: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("Tổng hợp giọng nói streaming của Zhipu TTS đã bị hủy, văn bản: %s", text)
				return
			default:
				log.Debugf("Thời gian xử lý Zhipu TTS: từ lúc nhập đến khi nhận xong dữ liệu âm thanh mất %d ms", time.Now().UnixMilli()-startTs)
			}
		} else {
			log.Errorf("Đầu ra streaming của Zhipu chỉ hỗ trợ định dạng pcm")
			close(outputChan)
		}
	}()

	return outputChan, nil
}

// parseEventStream sử dụng go-sse để phân tích phản hồi Event Stream của Zhipu, giải mã dữ liệu và ghi vào pipe
// ctx: context, dùng để hủy thao tác
// reader: bộ đọc phần thân phản hồi
// writer: đầu ghi của pipe, dùng để xuất dữ liệu nhị phân đã giải mã
// text: văn bản gốc, dùng để ghi log
func (p *ZhipuTTSProvider) parseEventStream(ctx context.Context, reader io.Reader, writer *io.PipeWriter, text string) error {
	// Cấu hình ReadConfig của go-sse, đặt MaxEventSize lớn hơn để xử lý token dài
	// Dữ liệu âm thanh mã hóa base64 mà Zhipu TTS trả về có thể vượt quá giới hạn mặc định 64KB
	readConfig := &sse.ReadConfig{
		MaxEventSize: 4 * 1024 * 1024, // 4MB, đủ để xử lý dữ liệu âm thanh mã hóa base64 có kích thước lớn
	}
	fadeTotalSamples := 0
	fadeSamplesRemaining := -1

	for ev, evErr := range sse.Read(reader, readConfig) {
		if evErr != nil {
			return fmt.Errorf("Đọc sự kiện SSE của Zhipu thất bại: %w", evErr)
		}

		select {
		case <-ctx.Done():
			log.Debugf("Tổng hợp giọng nói streaming của Zhipu TTS đã bị hủy, văn bản: %s", text)
			return ctx.Err()
		default:
		}

		// Định dạng Event Stream:
		// data: {"id":"...","choices":[{"delta":{"content":"base64_data"}}]}
		// data: {"choices":[{"finish_reason":"stop"}]}

		dataValue := strings.TrimSpace(ev.Data)
		if dataValue == "" {
			continue
		}

		// Phân tích JSON
		var eventResp zhipuEventStreamResponse
		if err := json.Unmarshal([]byte(dataValue), &eventResp); err != nil {
			log.Warnf("Phân tích JSON Event Stream của Zhipu thất bại: %v, dữ liệu: %s", err, previewString(dataValue, 200))
			continue
		}

		// Kiểm tra xem có finish_reason không, biểu thị luồng đã kết thúc
		for _, choice := range eventResp.Choices {
			if choice.FinishReason == "stop" {
				log.Debugf("Nhận được finish_reason: stop, Event Stream kết thúc")
				return nil
			}
		}

		// Trích xuất trường content của từng choice và xử lý độc lập
		for _, choice := range eventResp.Choices {
			if choice.Delta.Content != "" {
				decodedData, err := p.decodeAudioContent(choice.Delta.Content)
				if err != nil {
					return fmt.Errorf("Xử lý content thất bại: %v", err)
				}

				returnFormat := strings.TrimSpace(choice.Delta.ReturnFormat)
				if returnFormat == "" {
					returnFormat = p.ResponseFormat
				}
				if strings.EqualFold(returnFormat, "pcm") {
					if fadeSamplesRemaining < 0 {
						sampleRate := choice.Delta.ReturnSampleRate
						if sampleRate < 1 {
							sampleRate = zhipuDefaultSampleRate
						}
						fadeTotalSamples = leadingFadeInSampleCount(sampleRate, zhipuLeadingFadeInMs)
						fadeSamplesRemaining = fadeTotalSamples
					}
					applyPCM16MonoLeadingFadeInInPlace(decodedData, fadeTotalSamples, &fadeSamplesRemaining)
				}

				if len(decodedData) > 0 {
					if _, err := writer.Write(decodedData); err != nil {
						return fmt.Errorf("Ghi vào pipe thất bại: %v", err)
					}
				}
			}
		}
	}

	return nil
}

// previewString trả về n ký tự đầu tiên của chuỗi để dùng cho việc ghi log
func previewString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// decodeAudioContent giải mã một trường content đơn lẻ
// content: chuỗi dữ liệu âm thanh được mã hóa base64 hoặc hex
func (p *ZhipuTTSProvider) decodeAudioContent(content string) ([]byte, error) {
	if content == "" {
		return nil, nil
	}

	// Giải mã theo encode_format
	var decodedData []byte
	var decodeErr error

	switch p.EncodeFormat {
	case "base64":
		decodedData, decodeErr = base64.StdEncoding.DecodeString(content)
	case "hex":
		decodedData, decodeErr = hex.DecodeString(content)
	default:
		log.Warnf("Định dạng mã hóa không xác định: %s, sử dụng base64", p.EncodeFormat)
		decodedData, decodeErr = base64.StdEncoding.DecodeString(content)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("Giải mã dữ liệu âm thanh thất bại: %v, độ dài dữ liệu: %d", decodeErr, len(content))
	}

	return decodedData, nil
}

func leadingFadeInSampleCount(sampleRate int, fadeMs int) int {
	if sampleRate < 1 {
		sampleRate = zhipuDefaultSampleRate
	}
	if fadeMs < 1 {
		return 0
	}
	samples := sampleRate * fadeMs / 1000
	if samples < 1 {
		return 1
	}
	return samples
}

func applyPCM16MonoLeadingFadeIn(data []byte, remainingSamples int) []byte {
	if len(data) == 0 || remainingSamples <= 0 {
		return data
	}
	cloned := make([]byte, len(data))
	copy(cloned, data)
	applyPCM16MonoLeadingFadeInInPlace(cloned, remainingSamples, &remainingSamples)
	return cloned
}

func applyPCM16MonoLeadingFadeInInPlace(data []byte, totalSamples int, remainingSamples *int) {
	if len(data) < 2 || totalSamples <= 0 || remainingSamples == nil || *remainingSamples <= 0 {
		return
	}

	samplePairs := len(data) / 2
	for i := 0; i < samplePairs && *remainingSamples > 0; i++ {
		offset := i * 2
		sample := int16(uint16(data[offset]) | uint16(data[offset+1])<<8)
		appliedIndex := totalSamples - *remainingSamples
		scaled := int32(sample) * int32(appliedIndex) / int32(totalSamples)
		binarySample := uint16(int16(scaled))
		data[offset] = byte(binarySample)
		data[offset+1] = byte(binarySample >> 8)
		*remainingSamples = *remainingSamples - 1
	}
}

// SetVoice thiết lập tham số âm sắc (voice)
func (p *ZhipuTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	if voice, ok := voiceConfig["voice"].(string); ok && voice != "" {
		p.Voice = voice
		return nil
	}
	return fmt.Errorf("Cấu hình âm sắc không hợp lệ: thiếu voice")
}

// Close đóng tài nguyên (Provider không trạng thái, không cần đóng)
func (p *ZhipuTTSProvider) Close() error {
	return nil
}

// IsValid kiểm tra tài nguyên có hợp lệ hay không
func (p *ZhipuTTSProvider) IsValid() bool {
	return p != nil
}