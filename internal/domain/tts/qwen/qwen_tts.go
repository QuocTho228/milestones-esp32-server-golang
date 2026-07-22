package qwen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
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

const (
	defaultAPIURLBeijing    = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	defaultAPIURLSingapore  = "https://dashscope-intl.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	defaultQwenModel        = "qwen3-tts-flash"
	defaultQwenVoice        = "Cherry"
	defaultQwenLanguageType = "Chinese"
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
			Timeout:   60 * time.Second,
		}
	})
	return httpClient
}

// QwenTTSProvider Provider TTS của Qwen (Alibaba Cloud)
type QwenTTSProvider struct {
	APIKey        string
	APIURL        string
	Model         string
	Voice         string
	LanguageType  string
	Stream        bool
	FrameDuration int
}

// qwenRequest cấu trúc request
type qwenRequest struct {
	Model string           `json:"model"`
	Input qwenRequestInput `json:"input"`
}

type qwenRequestInput struct {
	Text         string `json:"text"`
	Voice        string `json:"voice"`
	LanguageType string `json:"language_type,omitempty"`
}

// qwenResponse cấu trúc phản hồi dùng chung cho cả dạng thường và dạng stream
type qwenResponse struct {
	StatusCode int        `json:"status_code"`
	RequestID  string     `json:"request_id"`
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	Output     qwenOutput `json:"output"`
	Usage      qwenUsage  `json:"usage"`
}

type qwenOutput struct {
	Text         interface{}   `json:"text"`
	FinishReason string        `json:"finish_reason"`
	Choices      interface{}   `json:"choices"`
	Audio        qwenAudioInfo `json:"audio"`
}

type qwenAudioInfo struct {
	Data      string `json:"data"`       // Dữ liệu âm thanh dạng Base64 khi xuất ở chế độ stream (16bit PCM)
	URL       string `json:"url"`        // URL file WAV khi xuất ở chế độ không stream
	ID        string `json:"id"`         // ID âm thanh
	ExpiresAt int64  `json:"expires_at"` // Mốc thời gian hết hạn của URL
}

type qwenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	Characters   int `json:"characters"`
}

// NewQwenTTSProvider tạo mới một Provider TTS Qwen của Alibaba Cloud
func NewQwenTTSProvider(config map[string]interface{}) *QwenTTSProvider {
	apiKey, _ := config["api_key"].(string)
	apiURL, _ := config["api_url"].(string)
	model, _ := config["model"].(string)
	voice, _ := config["voice"].(string)
	languageType, _ := config["language_type"].(string)
	stream, _ := config["stream"].(bool)
	frameDuration, _ := config["frame_duration"].(float64)
	region, _ := config["region"].(string)

	// Xử lý API URL / khu vực (region)
	if apiURL == "" {
		if strings.EqualFold(region, "singapore") {
			apiURL = defaultAPIURLSingapore
		} else {
			apiURL = defaultAPIURLBeijing
		}
	}

	// Giá trị mặc định
	if model == "" {
		model = defaultQwenModel
	}
	if voice == "" {
		voice = defaultQwenVoice
	}
	if languageType == "" {
		languageType = defaultQwenLanguageType
	}
	if frameDuration == 0 {
		frameDuration = audio.FrameDuration
	}

	return &QwenTTSProvider{
		APIKey:        apiKey,
		APIURL:        apiURL,
		Model:         model,
		Voice:         voice,
		LanguageType:  languageType,
		Stream:        stream,
		FrameDuration: int(frameDuration),
	}
}

// TextToSpeech chuyển văn bản thành giọng nói không dạng stream: gọi HTTP API, tải file WAV rồi giải mã thành các khung
func (p *QwenTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	startTs := time.Now().UnixMilli()

	// Xây dựng request body
	reqBody := qwenRequest{
		Model: p.Model,
		Input: qwenRequestInput{
			Text:         text,
			Voice:        p.Voice,
			LanguageType: p.LanguageType,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi tuần tự hóa (serialize) yêu cầu: %v", err)
	}

	// Tạo HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("lỗi khi tạo yêu cầu: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	client := getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gửi yêu cầu thất bại: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("yêu cầu API thất bại, mã trạng thái: %d, phản hồi: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("đọc phản hồi thất bại: %v", err)
	}

	var ttsResp qwenResponse
	if err := json.Unmarshal(body, &ttsResp); err != nil {
		return nil, fmt.Errorf("phân tích phản hồi thất bại: %v, nội dung phản hồi: %s", err, string(body))
	}

	if ttsResp.StatusCode != 200 {
		return nil, fmt.Errorf("lỗi API Qwen TTS [%s]: %s", ttsResp.Code, ttsResp.Message)
	}

	if ttsResp.Output.Audio.URL == "" {
		return nil, fmt.Errorf("phản hồi không chứa URL âm thanh")
	}

	log.Debugf("Qwen TTS không dạng stream, đang tải URL âm thanh: %s", ttsResp.Output.Audio.URL)

	// Tải file WAV, sau đó dùng bộ giải mã chung để chuyển thành các khung
	wavReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ttsResp.Output.Audio.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("tạo yêu cầu tải âm thanh thất bại: %v", err)
	}

	wavResp, err := client.Do(wavReq)
	if err != nil {
		return nil, fmt.Errorf("tải âm thanh thất bại: %v", err)
	}
	defer wavResp.Body.Close()

	if wavResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(wavResp.Body)
		return nil, fmt.Errorf("tải âm thanh thất bại, mã trạng thái: %d, phản hồi: %s", wavResp.StatusCode, string(body))
	}

	outputChan := make(chan []byte, 1000)

	decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, wavResp.Body, outputChan, frameDuration, "wav", sampleRate)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi tạo bộ giải mã âm thanh Qwen: %v", err)
	}

	// Khởi chạy giải mã
	go func() {
		if err := decoder.Run(startTs); err != nil {
			log.Errorf("Giải mã âm thanh Qwen TTS không dạng stream thất bại: %v", err)
		}
	}()

	var frames [][]byte
	for frame := range outputChan {
		frames = append(frames, frame)
	}

	log.Debugf("Qwen TTS không dạng stream hoàn tất, từ lúc nhập đến khi nhận xong dữ liệu âm thanh mất: %d ms", time.Now().UnixMilli()-startTs)
	return frames, nil
}

// TextToSpeechStream triển khai chuyển văn bản thành giọng nói dạng stream
func (p *QwenTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {

	startTs := time.Now().UnixMilli()

	// Xây dựng request body
	reqBody := qwenRequest{
		Model: p.Model,
		Input: qwenRequestInput{
			Text:         text,
			Voice:        p.Voice,
			LanguageType: p.LanguageType,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi tuần tự hóa (serialize) yêu cầu: %v", err)
	}

	// Tạo HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("lỗi khi tạo yêu cầu: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))
	req.Header.Set("X-DashScope-SSE", "enable") // Bật xuất dữ liệu dạng stream

	client := getHTTPClient()

	outputChan = make(chan []byte, 100)

	go func() {

		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("Gửi yêu cầu stream Qwen thất bại: %v", err)
			close(outputChan)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("Yêu cầu API stream Qwen thất bại, mã trạng thái: %d, phản hồi: %s", resp.StatusCode, string(body))
			close(outputChan)
			return
		}

		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/event-stream") {
			log.Warnf("Content-Type trả về từ API stream Qwen không phải text/event-stream: %s", contentType)
			close(outputChan)
			return
		}

		// Pipe: phân tích SSE -> PCM -> giải mã thành các khung
		pipeReader, pipeWriter := io.Pipe()

		// Phân tích SSE, ghi dữ liệu PCM gốc.
		// Trường audio.data mà Qwen trả về dạng stream trong thực tế có thể mang theo một header WAV,
		// cần loại bỏ trước khi xử lý như PCM.
		go func() {
			defer func() {
				if err := pipeWriter.Close(); err != nil {
					log.Debugf("Đóng đầu ghi của pipe Qwen thất bại: %v", err)
				}
			}()

			if err := p.parseEventStream(ctx, resp.Body, pipeWriter, text); err != nil {
				log.Errorf("Phân tích Event Stream của Qwen thất bại: %v", err)
			}
		}()

		// Tạo bộ giải mã âm thanh, đọc PCM từ pipe, xuất ra các khung opus
		decoder, err := util.CreateAudioDecoderWithSampleRate(
			ctx,
			pipeReader,
			outputChan,
			frameDuration,
			"pcm", // parseEventStream sẽ loại bỏ header WAV khi cần, xuất ra PCM 16bit thuần
			sampleRate,
		)
		if err != nil {
			log.Errorf("Lỗi khi tạo bộ giải mã âm thanh dạng stream của Qwen: %v", err)
			close(outputChan)
			pipeReader.Close()
			return
		}

		// Báo cho bộ giải mã biết thông tin sample rate/số kênh của PCM
		decoder.WithFormat(beep.Format{
			SampleRate:  beep.SampleRate(24000),
			NumChannels: 1,
		})

		// decoder.Run() sẽ tự đóng outputChan bên trong
		// Sử dụng sync.Once để đảm bảo dù decoder.Run() đã đóng channel, defer cũng không đóng lại lần nữa
		if err := decoder.Run(startTs); err != nil {
			log.Errorf("Giải mã âm thanh dạng stream của Qwen thất bại: %v", err)
			return
		}

		// Nếu decoder.Run() hoàn tất thành công, nó sẽ đóng channel
		// Do đó ở đây cần hủy thao tác đóng trong defer (đã được xử lý thông qua sync.Once)

		select {
		case <-ctx.Done():
			log.Debugf("Tổng hợp giọng nói dạng stream của Qwen TTS đã bị hủy, văn bản: %s", text)
			return
		default:
			log.Debugf("Thời gian xử lý dạng stream của Qwen TTS: từ lúc nhập đến khi nhận xong dữ liệu âm thanh mất: %d ms", time.Now().UnixMilli()-startTs)
		}
	}()

	return outputChan, nil
}

// parseEventStream sử dụng go-sse để phân tích SSE của Qwen (Alibaba Cloud), giải mã Base64 PCM và ghi vào pipe
func (p *QwenTTSProvider) parseEventStream(ctx context.Context, reader io.Reader, writer *io.PipeWriter, text string) error {
	var leadingAudio bytes.Buffer
	wroteLeadingAudio := false

	for ev, evErr := range sse.Read(reader, nil) {
		if evErr != nil {
			return fmt.Errorf("đọc sự kiện SSE của Qwen thất bại: %w", evErr)
		}

		select {
		case <-ctx.Done():
			log.Debugf("Tổng hợp giọng nói dạng stream của Qwen TTS đã bị hủy, văn bản: %s", text)
			return ctx.Err()
		default:
		}

		dataValue := strings.TrimSpace(ev.Data)
		if dataValue == "" {
			continue
		}

		var eventResp qwenResponse
		if err := json.Unmarshal([]byte(dataValue), &eventResp); err != nil {
			log.Warnf("Phân tích JSON của Event Stream Qwen thất bại: %v, dữ liệu: %s", err, previewString(dataValue, 200))
			continue
		}

		// Kiểm tra mã trạng thái nghiệp vụ (dữ liệu data dạng stream có thể không chứa status_code,
		// nếu không có thì mặc định là 0, coi như thành công)
		if eventResp.StatusCode != 0 && eventResp.StatusCode != 200 {
			return fmt.Errorf("lỗi API stream Qwen [%s]: %s", eventResp.Code, eventResp.Message)
		}

		// Giải mã dữ liệu PCM dạng Base64
		if eventResp.Output.Audio.Data != "" {
			encoded := cleanBase64(eventResp.Output.Audio.Data)
			audioBytes, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				log.Errorf("Giải mã Base64 PCM của Qwen thất bại: %v", err)
				continue
			}

			if len(audioBytes) > 0 {
				if !wroteLeadingAudio {
					leadingAudio.Write(audioBytes)
					normalized, needMore, detectedWAV, err := normalizeLeadingQwenAudio(leadingAudio.Bytes())
					if err != nil {
						return fmt.Errorf("phân tích header âm thanh dạng stream của Qwen thất bại: %w", err)
					}
					if needMore {
						continue
					}
					wroteLeadingAudio = true
					if detectedWAV {
						log.Infof("Âm thanh dạng stream của Qwen phát hiện có header WAV, đã loại bỏ và xử lý như PCM")
					}
					if len(normalized) == 0 {
						continue
					}
					if _, err := writer.Write(normalized); err != nil {
						return fmt.Errorf("ghi PCM vào pipe thất bại: %v", err)
					}
					continue
				}

				if _, err := writer.Write(audioBytes); err != nil {
					return fmt.Errorf("ghi PCM vào pipe thất bại: %v", err)
				}
			}
		}

		// Kiểm tra xem đã hoàn tất chưa
		if eventResp.Output.FinishReason == "stop" {
			log.Debugf("Stream của Qwen nhận được finish_reason=stop, request ID: %s", eventResp.RequestID)
			return nil
		}
	}

	return nil
}

func normalizeLeadingQwenAudio(data []byte) (normalized []byte, needMore bool, detectedWAV bool, err error) {
	if len(data) < 12 {
		return nil, true, false, nil
	}

	if !bytes.HasPrefix(data, []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WAVE")) {
		return data, false, false, nil
	}

	offset, needMore, err := qwenWAVDataOffset(data)
	if err != nil {
		return nil, false, true, err
	}
	if needMore {
		return nil, true, true, nil
	}
	if offset > len(data) {
		return nil, false, true, fmt.Errorf("offset dữ liệu WAV vượt giới hạn: %d > %d", offset, len(data))
	}
	return data[offset:], false, true, nil
}

func qwenWAVDataOffset(data []byte) (offset int, needMore bool, err error) {
	if len(data) < 12 {
		return 0, true, nil
	}
	if !bytes.HasPrefix(data, []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WAVE")) {
		return 0, false, fmt.Errorf("không phải header WAV hợp lệ")
	}

	offset = 12
	for {
		if len(data) < offset+8 {
			return 0, true, nil
		}

		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		if chunkSize < 0 {
			return 0, false, fmt.Errorf("kích thước chunk WAV không hợp lệ: %d", chunkSize)
		}
		offset += 8

		if chunkID == "data" {
			return offset, false, nil
		}

		nextOffset := offset + chunkSize
		if chunkSize%2 == 1 {
			nextOffset++
		}
		if len(data) < nextOffset {
			return 0, true, nil
		}
		offset = nextOffset
	}
}

// SetVoice thiết lập giọng nói
func (p *QwenTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	if voice, ok := voiceConfig["voice"].(string); ok && voice != "" {
		p.Voice = voice
		return nil
	}
	return fmt.Errorf("cấu hình giọng nói không hợp lệ: thiếu voice")
}

// Close đóng tài nguyên (Provider không trạng thái, không cần đóng)
func (p *QwenTTSProvider) Close() error {
	return nil
}

// IsValid kiểm tra tài nguyên có hợp lệ hay không
func (p *QwenTTSProvider) IsValid() bool {
	return p != nil
}

// cleanBase64 loại bỏ toàn bộ ký tự khoảng trắng trong chuỗi Base64
func cleanBase64(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == ' ' || ch == '\n' || ch == '\r' || ch == '\t' {
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

// previewString trả về n ký tự đầu tiên của chuỗi, dùng để ghi log
func previewString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}