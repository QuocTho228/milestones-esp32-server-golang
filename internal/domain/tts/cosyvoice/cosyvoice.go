package cosyvoice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"milestones-esp32-server-golang/internal/data/audio"
	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"
)

// HTTP client toàn cục, dùng để triển khai connection pool
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
			Timeout:   30 * time.Second,
		}
	})
	return httpClient
}

// CosyVoiceTTSProvider nhà cung cấp dịch vụ TTS CosyVoice
type CosyVoiceTTSProvider struct {
	APIURL        string
	SpeakerID     string
	FrameDuration int
	TargetSR      int
	AudioFormat   string
	InstructText  string
}

// Cấu trúc response
type cosyVoiceResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    []byte `json:"data"`
}

// NewCosyVoiceTTSProvider tạo mới nhà cung cấp dịch vụ TTS CosyVoice
func NewCosyVoiceTTSProvider(config map[string]interface{}) *CosyVoiceTTSProvider {
	apiURL, _ := config["api_url"].(string)
	speakerID, _ := config["spk_id"].(string)
	frameDuration, _ := config["frame_duration"].(float64)
	targetSR, _ := config["target_sr"].(float64)
	audioFormat, _ := config["audio_format"].(string)
	instructText, _ := config["instruct_text"].(string)

	// Thiết lập giá trị mặc định
	if apiURL == "" {
		apiURL = "https://tts.linkerai.cn/tts"
	}
	if speakerID == "" {
		speakerID = "OUeAo1mhq6IBExi"
	}
	if frameDuration == 0 {
		frameDuration = audio.FrameDuration
	}
	if targetSR == 0 {
		targetSR = audio.SampleRate
	}
	if audioFormat == "" {
		audioFormat = "mp3"
	}

	return &CosyVoiceTTSProvider{
		APIURL:        apiURL,
		SpeakerID:     speakerID,
		FrameDuration: int(frameDuration),
		TargetSR:      int(targetSR),
		AudioFormat:   audioFormat,
		InstructText:  instructText,
	}
}

// TextToSpeech chuyển văn bản thành giọng nói, trả về dữ liệu các khung âm thanh (audio frame) và lỗi (nếu có)
func (p *CosyVoiceTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	// Xây dựng tham số truy vấn (query)
	params := url.Values{}
	params.Add("tts_text", text)
	params.Add("spk_id", p.SpeakerID)
	params.Add("frame_durition", fmt.Sprintf("%d", p.FrameDuration))
	params.Add("stream", "true") // yêu cầu dạng streaming
	params.Add("target_sr", fmt.Sprintf("%d", p.TargetSR))
	params.Add("audio_format", p.AudioFormat)

	startTs := time.Now().UnixMilli()

	// Xây dựng URL đầy đủ
	requestURL := fmt.Sprintf("%s?%s", p.APIURL, params.Encode())

	// Tạo HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Tạo request thất bại: %v", err)
	}

	req.Header.Set("Accept", "application/json")

	// Sử dụng connection pool để gửi request
	client := getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gửi request thất bại: %v", err)
	}
	defer resp.Body.Close()

	// Đọc response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Đọc response thất bại: %v", err)
	}

	// Kiểm tra mã trạng thái (status code) của response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request thất bại, status code: %d, response: %s", resp.StatusCode, string(body))
	}

	// Kiểm tra loại nội dung (content type) và độ dài nội dung (content length) của response
	// contentType := resp.Header.Get("Content-Type")
	contentLength := resp.ContentLength

	// Ghi độ dài response vào log
	log.Debugf("Nhận được response TTS, Content-Length: %d", contentLength)

	// Kiểm tra Content-Length có hợp lý hay không
	if contentLength == 0 {
		log.Errorf("API trả về response rỗng, Content-Length = 0")
		return nil, fmt.Errorf("API trả về response rỗng, Content-Length = 0")
	}

	// Header file MP3 cần tối thiểu 100 byte mới có thể parse bình thường
	// -1 nghĩa là độ dài không xác định (ví dụ truyền theo dạng chunk)
	if contentLength > 0 && contentLength < 100 {
		log.Errorf("Response trả về từ API quá nhỏ, không thể parse thành MP3: %d byte", contentLength)
		return nil, fmt.Errorf("Response trả về từ API quá nhỏ, không thể parse thành MP3: %d byte", contentLength)
	}

	// Chuyển đổi sang khung Opus (Opus frame)
	if p.AudioFormat == "mp3" {
		// Tạo một pipe
		doneChan := make(chan struct{})
		outputChan := make(chan []byte, 1000)

		// Tạo bộ giải mã (decoder) MP3
		mp3Decoder, err := util.CreateAudioDecoder(ctx, io.NopCloser(bytes.NewReader(body)), outputChan, frameDuration, p.AudioFormat)
		if err != nil {
			close(doneChan)
			return nil, fmt.Errorf("Tạo bộ giải mã MP3 thất bại: %v", err)
		}
		// Khởi động quá trình giải mã
		go func() {
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("Giải mã MP3 thất bại: %v", err)
			}
		}()

		// Thu thập tất cả các khung Opus
		var opusFrames [][]byte
		for frame := range outputChan {
			opusFrames = append(opusFrames, frame)
		}

		return opusFrames, nil
	}

	return nil, fmt.Errorf("Định dạng âm thanh không được hỗ trợ: %s", p.AudioFormat)
}

// TextToSpeechStream triển khai tổng hợp giọng nói dạng streaming
func (p *CosyVoiceTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {
	// Xây dựng tham số truy vấn (query)
	params := url.Values{}
	params.Add("tts_text", text)
	params.Add("spk_id", p.SpeakerID)
	params.Add("frame_durition", fmt.Sprintf("%d", frameDuration))
	params.Add("stream", "true") // yêu cầu dạng streaming
	params.Add("target_sr", fmt.Sprintf("%d", sampleRate))
	params.Add("audio_format", p.AudioFormat)

	startTs := time.Now().UnixMilli()

	// Xây dựng URL đầy đủ
	requestURL := fmt.Sprintf("%s?%s", p.APIURL, params.Encode())

	// Tạo HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Tạo request thất bại: %v", err)
	}

	req.Header.Set("Accept", "application/json")

	// Sử dụng connection pool để tạo client
	client := getHTTPClient()

	// Tạo channel đầu ra
	outputChan = make(chan []byte, 100)
	// Khởi động goroutine để xử lý response dạng streaming
	go func() {
		decoderStarted := false
		defer func() {
			if !decoderStarted {
				close(outputChan)
			}
		}()

		// Gửi request
		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("Gửi request thất bại: %v", err)
			return
		}
		defer func() {
			resp.Body.Close()
		}()

		// Kiểm tra mã trạng thái (status code) của response
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("API request thất bại, status code: %d, response: %s", resp.StatusCode, string(body))
			return
		}

		// Kiểm tra loại nội dung (content type) và độ dài nội dung (content length) của response
		// contentType := resp.Header.Get("Content-Type")
		contentLength := resp.ContentLength

		// Ghi độ dài response vào log
		log.Debugf("Nhận được response TTS, Content-Length: %d", contentLength)

		// Kiểm tra Content-Length có hợp lý hay không
		if contentLength == 0 {
			log.Errorf("API trả về response rỗng, Content-Length = 0")
			return
		}

		// Header file MP3 cần tối thiểu 100 byte mới có thể parse bình thường
		// -1 nghĩa là độ dài không xác định (ví dụ truyền theo dạng chunk)
		if contentLength > 0 && contentLength < 100 {
			log.Errorf("Response trả về từ API quá nhỏ, không thể parse thành MP3: %d byte", contentLength)
			return
		}

		// Xử lý response dạng streaming theo định dạng âm thanh
		if p.AudioFormat == "mp3" {
			// Tạo bộ giải mã MP3, truyền context thay vì dùng channel done
			mp3Decoder, err := util.CreateAudioDecoder(ctx, resp.Body, outputChan, frameDuration, p.AudioFormat)
			if err != nil {
				log.Errorf("Tạo bộ giải mã MP3 thất bại: %v", err)
				return
			}

			// Khởi động quá trình giải mã
			decoderStarted = true
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("Giải mã MP3 thất bại: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("Tổng hợp TTS dạng streaming đã bị hủy, văn bản: %s", text)
				return
			default:
				log.Infof("Thời gian xử lý tts: từ lúc nhận đầu vào đến khi lấy xong dữ liệu MP3 mất: %d ms", time.Now().UnixMilli()-startTs)

			}
		} else {
			log.Errorf("Hiện tại chỉ hỗ trợ tổng hợp streaming ở định dạng MP3")
		}
	}()

	return outputChan, nil
}

// SetVoice thiết lập tham số âm sắc (giọng nói)
func (p *CosyVoiceTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	if spkID, ok := voiceConfig["spk_id"].(string); ok && spkID != "" {
		p.SpeakerID = spkID
		return nil
	}
	return fmt.Errorf("Cấu hình âm sắc không hợp lệ: thiếu spk_id")
}

// Close đóng tài nguyên (Provider không có trạng thái nội tại, không cần đóng)
func (p *CosyVoiceTTSProvider) Close() error {
	return nil
}

// IsValid kiểm tra tài nguyên có hợp lệ hay không
func (p *CosyVoiceTTSProvider) IsValid() bool {
	return p != nil
}