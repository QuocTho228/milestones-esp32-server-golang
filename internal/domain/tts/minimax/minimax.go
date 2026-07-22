package minimax

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"

	"github.com/gorilla/websocket"
)

// Định nghĩa hằng số
const (
	wsURL = "wss://api.minimaxi.com/ws/v1/t2a_v2"
)

// WebSocket Dialer toàn cục
var wsDialer = websocket.Dialer{
	ReadBufferSize:   16384, // Bộ đệm đọc 16KB
	WriteBufferSize:  16384, // Bộ đệm ghi 16KB
	HandshakeTimeout: 45 * time.Second,
}

// MinimaxTTSProvider Provider TTS của Minimax
type MinimaxTTSProvider struct {
	APIKey     string
	Model      string
	Voice      string
	Speed      float64
	Volume     float64
	Pitch      int
	SampleRate int
	Bitrate    int
	Format     string
	Channel    int

	// Quản lý kết nối
	conn      *websocket.Conn
	connMutex sync.RWMutex
	// Khóa gửi (send lock), đảm bảo tại một thời điểm chỉ có một yêu cầu đang sử dụng kết nối
	sendMutex sync.Mutex
}

// Cấu trúc thông điệp WebSocket
type minimaxMessage struct {
	Event           string        `json:"event,omitempty"`
	Model           string        `json:"model,omitempty"`
	VoiceSetting    *voiceSetting `json:"voice_setting,omitempty"`
	AudioSetting    *audioSetting `json:"audio_setting,omitempty"`
	ContinuousSound bool          `json:"continuous_sound,omitempty"`
	Text            string        `json:"text,omitempty"`
}

type minimaxResp struct {
	SessionId string            `json:"session_id,omitempty"`
	Event     string            `json:"event,omitempty"`
	TraceId   string            `json:"trace_id,omitempty"`
	Data      *minimaxData      `json:"data,omitempty"`
	IsFinal   bool              `json:"is_final,omitempty"`
	BaseResp  *minimaxBaseResp  `json:"base_resp,omitempty"`
	ExtraInfo *minimaxExtraInfo `json:"extra_info,omitempty"`
}

type minimaxExtraInfo struct {
	AudioLength     int    `json:"audio_length"`
	AudioSampleRate int    `json:"audio_sample_rate"`
	AudioDuration   int    `json:"audio_duration"`
	AudioSize       int    `json:"audio_size"`
	Bitrate         int    `json:"bitrate"`
	AudioFormat     string `json:"audio_format"`
	AudioChannel    int    `json:"audio_channel"`

	UsageCharacters int `json:"usage_characters"`
	WordCount       int `json:"word_count"`
}

type minimaxBaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type voiceSetting struct {
	VoiceID              string  `json:"voice_id"`
	Speed                float64 `json:"speed"`
	Vol                  float64 `json:"vol"`
	Pitch                int     `json:"pitch"`
	EnglishNormalization bool    `json:"english_normalization"`
}

type audioSetting struct {
	SampleRate int    `json:"sample_rate"`
	Bitrate    int    `json:"bitrate"`
	Format     string `json:"format"`
	Channel    int    `json:"channel"`
}

type minimaxData struct {
	Audio string `json:"audio"`
}

// NewMinimaxTTSProvider tạo mới một Provider TTS của Minimax
func NewMinimaxTTSProvider(config map[string]interface{}) *MinimaxTTSProvider {
	apiKey, _ := config["api_key"].(string)
	model, _ := config["model"].(string)
	voice, _ := config["voice"].(string)
	speed, _ := config["speed"].(float64)
	volume, _ := config["vol"].(float64)
	if volume == 0 {
		volume, _ = config["volume"].(float64)
	}
	pitch, _ := config["pitch"].(float64)
	sampleRate, _ := config["sample_rate"].(float64)
	bitrate, _ := config["bitrate"].(float64)
	format, _ := config["format"].(string)
	channel, _ := config["channel"].(float64)

	// Thiết lập giá trị mặc định
	if model == "" {
		model = "speech-2.8-hd"
	}
	if voice == "" {
		voice = "male-qn-qingse"
	}
	if speed == 0 {
		speed = 1.0
	}
	if volume == 0 {
		volume = 1.0
	}
	if sampleRate == 0 {
		sampleRate = 32000
	}
	if bitrate == 0 {
		bitrate = 128000
	}
	if format == "" {
		format = "mp3"
	}
	if channel == 0 {
		channel = 1
	}

	return &MinimaxTTSProvider{
		APIKey:     apiKey,
		Model:      model,
		Voice:      voice,
		Speed:      speed,
		Volume:     volume,
		Pitch:      int(pitch),
		SampleRate: int(sampleRate),
		Bitrate:    int(bitrate),
		Format:     format,
		Channel:    int(channel),
	}
}

// TextToSpeech tổng hợp một lần (hiện chưa hỗ trợ, sử dụng phương án triển khai dạng stream)
func (p *MinimaxTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	// Minimax chủ yếu hỗ trợ dạng stream, ở đây thu thập dữ liệu stream rồi trả về
	outputChan, err := p.TextToSpeechStream(ctx, text, sampleRate, channels, frameDuration)
	if err != nil {
		return nil, err
	}

	var frames [][]byte
	for frame := range outputChan {
		frames = append(frames, frame)
	}

	return frames, nil
}

// TextToSpeechStream triển khai tổng hợp giọng nói dạng stream (luồng)
func (p *MinimaxTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {
	startTs := time.Now().UnixMilli()

	// Sử dụng send lock để bảo vệ, đảm bảo tại một thời điểm chỉ có một yêu cầu đang sử dụng kết nối
	p.sendMutex.Lock()
	// Lưu ý: không giải phóng khóa khi hàm return, mà giải phóng khi goroutine hoàn tất

	// Lấy kết nối (tái sử dụng hoặc tạo mới)
	conn, err := p.getConnection(ctx)
	if err != nil {
		p.sendMutex.Unlock()
		return nil, fmt.Errorf("lấy kết nối WebSocket thất bại: %v", err)
	}

	// Tạo channel đầu ra
	outputChan = make(chan []byte, 100)

	// Tạo pipe dùng cho việc giải mã âm thanh
	pipeReader, pipeWriter := io.Pipe()

	// Khởi chạy goroutine cho bộ giải mã âm thanh
	go func() {
		decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, p.Format, sampleRate)
		if err != nil {
			log.Errorf("Lỗi khi tạo bộ giải mã âm thanh: %v", err)
			pipeReader.Close()
			close(outputChan)
			return
		}

		if err := decoder.Run(startTs); err != nil {
			log.Errorf("Giải mã âm thanh thất bại: %v", err)
		}
	}()

	// Sử dụng WaitGroup để chờ goroutine đọc hoàn tất
	var wg sync.WaitGroup
	wg.Add(1)

	// Khởi chạy goroutine đọc và xử lý; khóa sẽ được giải phóng thống nhất bằng defer trong goroutine này,
	// đảm bảo dù kết thúc bình thường, gặp lỗi, hay panic thì khóa vẫn được giải phóng
	go func() {
		defer wg.Done()
		defer p.sendMutex.Unlock()
		defer func() {
			pipeWriter.Close()
			pipeReader.Close()
		}()

		p.processStreamTTS(ctx, conn, text, pipeWriter)
	}()

	// Chạy nền chờ goroutine hoàn tất rồi giải phóng khóa
	go func() {
		wg.Wait()
		log.Debugf("Tổng hợp giọng nói dạng stream của Minimax TTS hoàn tất, thời gian xử lý: %d ms", time.Now().UnixMilli()-startTs)
	}()

	return outputChan, nil
}

// processStreamTTS xử lý quy trình tổng hợp TTS dạng stream
func (p *MinimaxTTSProvider) processStreamTTS(ctx context.Context, conn *websocket.Conn, text string, pipeWriter *io.PipeWriter) {
	// Gửi thông điệp bắt đầu tác vụ
	startMsg := minimaxMessage{
		Event: "task_start",
		Model: p.Model,
		VoiceSetting: &voiceSetting{
			VoiceID:              p.Voice,
			Speed:                p.Speed,
			Vol:                  p.Volume,
			Pitch:                p.Pitch,
			EnglishNormalization: false,
		},
		AudioSetting: &audioSetting{
			SampleRate: p.SampleRate,
			Bitrate:    p.Bitrate,
			Format:     p.Format,
			Channel:    p.Channel,
		},
		ContinuousSound: false,
	}

	log.Debugf("minimax gửi thông điệp bắt đầu tác vụ: model=%s, voice=%s, format=%s", p.Model, p.Voice, p.Format)
	if err := p.sendMessage(conn, startMsg); err != nil {
		log.Errorf("Gửi thông điệp bắt đầu tác vụ thất bại: %v", err)
		p.clearConnection()
		return
	}

	// Chờ xác nhận bắt đầu tác vụ
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msg, err := p.readMessage(conn)
	if err != nil {
		// Kiểm tra xem có phải lỗi timeout hay không
		if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
			log.Errorf("Hết thời gian chờ xác nhận bắt đầu tác vụ (không nhận được phản hồi trong 10 giây)")
		} else {
			log.Errorf("Đọc xác nhận bắt đầu tác vụ thất bại: %v", err)
		}
		p.clearConnection()
		return
	}

	log.Debugf("Nhận được thông điệp xác nhận bắt đầu tác vụ: %+v", msg)

	if msg.Event != "task_started" {
		log.Errorf("Bắt đầu tác vụ thất bại, kỳ vọng 'task_started', nhận được: event=%s, thông điệp đầy đủ=%+v", msg.Event, msg)
		if msg.BaseResp != nil && msg.BaseResp.StatusCode != 0 {
			log.Errorf("Chi tiết lỗi: status_code=%d, status_msg=%s", msg.BaseResp.StatusCode, msg.BaseResp.StatusMsg)
		}
		p.clearConnection()
		return
	}
	// Đặt lại thời gian chờ đọc
	conn.SetReadDeadline(time.Time{})

	log.Debugf("Xác nhận bắt đầu tác vụ thành công")

	// Gửi thông điệp văn bản
	continueMsg := minimaxMessage{
		Event: "task_continue",
		Text:  text,
	}

	if err := p.sendMessage(conn, continueMsg); err != nil {
		log.Errorf("Gửi thông điệp văn bản thất bại: %v", err)
		p.clearConnection()
		return
	}

	// Đọc dữ liệu âm thanh
	chunkCount := 0
	for {
		select {
		case <-ctx.Done():
			log.Debugf("Tổng hợp giọng nói dạng stream của Minimax TTS đã bị hủy, văn bản: %s", text)
			// Gửi thông điệp kết thúc tác vụ
			finishMsg := minimaxMessage{Event: "task_finish"}
			p.sendMessage(conn, finishMsg)

			// Theo tài liệu, sau khi server nhận được task_finish sẽ đóng kết nối WebSocket
			// Thử đọc phản hồi task_finished (nếu server có gửi)
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			if finishResp, err := p.readMessage(conn); err == nil {
				log.Debugf("Nhận được xác nhận kết thúc tác vụ: event=%s, thông điệp đầy đủ=%+v", finishResp.Event, finishResp)
			} else {
				// Kết nối có thể đã bị đóng, đây là hành vi bình thường
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Debugf("Server đã đóng kết nối (hành vi bình thường)")
					if closeErr, ok := err.(*websocket.CloseError); ok {
						log.Debugf("Chi tiết close frame: code=%d, text=%s", closeErr.Code, closeErr.Text)
					}
				} else {
					log.Debugf("Đọc xác nhận kết thúc tác vụ thất bại: %v", err)
					if closeErr, ok := err.(*websocket.CloseError); ok {
						log.Debugf("Chi tiết close frame: code=%d, text=%s", closeErr.Code, closeErr.Text)
					}
				}
			}

			// Xóa trạng thái kết nối vì server đã đóng kết nối
			p.clearConnection()
			return
		default:
		}

		// Thiết lập thời gian chờ đọc
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		msg, err := p.readMessage(conn)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Errorf("Đọc thông điệp WebSocket thất bại: %v", err)
				// Thử lấy thông tin close frame
				if closeErr, ok := err.(*websocket.CloseError); ok {
					log.Errorf("Chi tiết close frame của WebSocket: code=%d, text=%s", closeErr.Code, closeErr.Text)
				}
				p.clearConnection()
				return
			}
			// Đóng bình thường hoặc lỗi khi đọc
			log.Debugf("Kết nối WebSocket đã đóng hoặc lỗi khi đọc: %v", err)
			if closeErr, ok := err.(*websocket.CloseError); ok {
				log.Debugf("Chi tiết close frame của WebSocket: code=%d, text=%s", closeErr.Code, closeErr.Text)
			}
			return
		}

		if msg.BaseResp != nil && msg.BaseResp.StatusCode != 0 {
			log.Errorf("BaseResp: status_code=%d, status_msg=%s", msg.BaseResp.StatusCode, msg.BaseResp.StatusMsg)
		}

		// Kiểm tra xem có thông điệp lỗi hay không
		if msg.Event == "error" || msg.Event == "task_error" {
			log.Errorf("Nhận được thông điệp lỗi: %+v", msg)
			if msg.BaseResp != nil && msg.BaseResp.StatusCode != 0 {
				log.Errorf("Chi tiết lỗi: status_code=%d, status_msg=%s", msg.BaseResp.StatusCode, msg.BaseResp.StatusMsg)
			}
			p.clearConnection()
			return
		}

		// Xử lý dữ liệu âm thanh
		if msg.Data != nil && msg.Data.Audio != "" {
			chunkCount++

			// Chuyển dữ liệu âm thanh mã hóa hex sang dạng nhị phân
			audioBytes, err := hex.DecodeString(msg.Data.Audio)
			if err != nil {
				log.Errorf("Giải mã dữ liệu âm thanh thất bại: %v", err)
				continue
			}

			// Ghi vào pipe để bộ giải mã xử lý
			if _, err := pipeWriter.Write(audioBytes); err != nil {
				log.Errorf("Ghi dữ liệu âm thanh vào pipe thất bại: %v", err)
				p.clearConnection()
				return
			}
		}

		// Kiểm tra xem đã hoàn tất chưa
		if msg.IsFinal {
			log.Debugf("Nhận được đoạn âm thanh cuối cùng, tổng cộng %d đoạn", chunkCount)
			// Gửi thông điệp kết thúc tác vụ
			finishMsg := minimaxMessage{Event: "task_finish"}
			p.sendMessage(conn, finishMsg)

			// Xóa trạng thái kết nối vì server đã đóng kết nối
			// Lần sử dụng tiếp theo cần tạo kết nối mới
			p.clearConnection()
			return
		}
	}
}

// getConnection lấy kết nối, nếu chưa có thì tạo mới
func (p *MinimaxTTSProvider) getConnection(ctx context.Context) (*websocket.Conn, error) {
	// Thử đọc kết nối hiện có trước
	p.connMutex.RLock()
	conn := p.conn
	p.connMutex.RUnlock()

	if conn != nil {
		return conn, nil
	}

	// Cần tạo kết nối mới
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	// Kiểm tra lại lần nữa (double-check), có thể goroutine khác đã tạo kết nối
	if p.conn != nil {
		return p.conn, nil
	}

	// Tạo HTTP header
	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	// Tạo kết nối mới
	conn, resp, err := wsDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		if resp != nil {
			log.Errorf("Kết nối WebSocket thất bại, mã trạng thái: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("kết nối WebSocket thất bại: %v", err)
	}

	// Thiết lập giới hạn đọc thông điệp
	conn.SetReadLimit(1024 * 1024) // Kích thước thông điệp tối đa 1MB

	// Thiết lập giữ kết nối (keep-alive)
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(1*time.Second))
	})

	// Chờ thông điệp xác nhận kết nối thành công
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("đọc thông điệp xác nhận kết nối thất bại: %v", err)
	}

	log.Debugf("Nhận được thông điệp xác nhận kết nối (dữ liệu gốc): %s", string(message))

	var connectMsg minimaxResp
	if err := json.Unmarshal(message, &connectMsg); err != nil {
		conn.Close()
		log.Errorf("Phân tích thông điệp xác nhận kết nối thất bại, thông điệp gốc: %s, lỗi: %v", string(message), err)
		return nil, fmt.Errorf("phân tích thông điệp xác nhận kết nối thất bại: %v", err)
	}

	log.Debugf("Nhận được thông điệp xác nhận kết nối (đã phân tích): %+v", connectMsg)

	if connectMsg.Event != "connected_success" {
		conn.Close()
		log.Errorf("Kết nối thất bại, kỳ vọng 'connected_success', nhận được: %+v", connectMsg)
		return nil, fmt.Errorf("kết nối thất bại, nhận được: %+v", connectMsg)
	}

	p.conn = conn
	log.Infof("Kết nối WebSocket của Minimax đã được thiết lập")
	return conn, nil
}

// clearConnection xóa kết nối (dùng khi mất kết nối cần kết nối lại)
func (p *MinimaxTTSProvider) clearConnection() {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
		log.Infof("Kết nối WebSocket của Minimax đã được xóa, chờ kết nối lại lần sau")
	}
}

// sendMessage gửi thông điệp JSON
func (p *MinimaxTTSProvider) sendMessage(conn *websocket.Conn, msg minimaxMessage) error {
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("kết nối đã đóng")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("lỗi khi tuần tự hóa (serialize) thông điệp: %v", err)
	}

	log.Debugf("minimax gửi thông điệp: %s", string(data))

	return conn.WriteMessage(websocket.TextMessage, data)
}

// readMessage đọc thông điệp JSON
func (p *MinimaxTTSProvider) readMessage(conn *websocket.Conn) (*minimaxResp, error) {
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	_ = messageType
	//log.Debugf("minimax đã nhận được tin nhắn WebSocket: type=%d, độ dài nội dung gốc=%d, nội dung=%s", messageType, len(message), string(message))

	var msg minimaxResp
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Errorf("Phân tích thông điệp thất bại, thông điệp gốc: %s, lỗi: %v", string(message), err)
		return nil, fmt.Errorf("phân tích thông điệp thất bại: %v", err)
	}

	return &msg, nil
}

// SetVoice thiết lập tham số giọng nói
func (p *MinimaxTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	return nil
}

// Close đóng tài nguyên, giải phóng kết nối
func (p *MinimaxTTSProvider) Close() error {
	p.clearConnection()
	return nil
}

// IsValid kiểm tra tài nguyên có hợp lệ hay không
func (p *MinimaxTTSProvider) IsValid() bool {
	p.connMutex.RLock()
	conn := p.conn
	p.connMutex.RUnlock()

	return conn != nil
}