package edge_offline

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"

	"github.com/gopxl/beep"
	"github.com/gorilla/websocket"
)

// EdgeOfflineTTSProvider là nhà cung cấp TTS qua WebSocket
type EdgeOfflineTTSProvider struct {
	ServerURL        string
	Timeout          time.Duration
	HandshakeTimeout time.Duration

	// Quản lý kết nối
	conn      *websocket.Conn
	connMutex sync.RWMutex
	// Khóa gửi (sendMutex), đảm bảo tại một thời điểm chỉ có một yêu cầu sử dụng kết nối
	sendMutex sync.Mutex
}

// NewEdgeOfflineTTSProvider tạo mới một Edge Offline TTS Provider
func NewEdgeOfflineTTSProvider(config map[string]interface{}) *EdgeOfflineTTSProvider {
	serverURL, _ := config["server_url"].(string)
	timeout, _ := config["timeout"].(float64)
	handshakeTimeout, _ := config["handshake_timeout"].(float64)

	// Thiết lập giá trị mặc định
	if serverURL == "" {
		serverURL = "ws://localhost:8080/tts"
	}
	if timeout == 0 {
		timeout = 30 // Timeout mặc định 30 giây
	}
	if handshakeTimeout == 0 {
		handshakeTimeout = 10 // Timeout bắt tay (handshake) mặc định 10 giây
	}

	return &EdgeOfflineTTSProvider{
		ServerURL:        serverURL,
		Timeout:          time.Duration(timeout) * time.Second,
		HandshakeTimeout: time.Duration(handshakeTimeout) * time.Second,
	}
}

// getConnection lấy kết nối, nếu chưa có thì tạo mới
func (p *EdgeOfflineTTSProvider) getConnection(ctx context.Context) (*websocket.Conn, error) {
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

	// Kiểm tra lại lần nữa (double-check), có thể goroutine khác đã tạo kết nối rồi
	if p.conn != nil {
		return p.conn, nil
	}

	// Tạo kết nối mới
	dialer := &websocket.Dialer{
		HandshakeTimeout: p.HandshakeTimeout,
	}
	conn, _, err := dialer.DialContext(ctx, p.ServerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Kết nối WebSocket thất bại: %v", err)
	}

	p.conn = conn
	log.Infof("Kết nối WebSocket đã được thiết lập")
	return conn, nil
}

// clearConnection xóa kết nối (dùng để kết nối lại khi mất kết nối)
func (p *EdgeOfflineTTSProvider) clearConnection() {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
		log.Infof("Kết nối WebSocket đã được xóa, chờ lần kết nối lại tiếp theo")
	}
}

// writeMessage ghi message vào kết nối WebSocket một cách an toàn
func (p *EdgeOfflineTTSProvider) writeMessage(conn *websocket.Conn, messageType int, data []byte) error {
	// Dùng khóa đọc (RLock) để bảo vệ thao tác ghi kết nối, tránh việc ghi đồng thời gây lộn xộn dữ liệu
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	// Kiểm tra kết nối có hợp lệ không
	if conn == nil {
		return fmt.Errorf("Kết nối đã đóng")
	}

	return conn.WriteMessage(messageType, data)
}

// TextToSpeech chuyển văn bản thành giọng nói, trả về dữ liệu khung âm thanh
func (p *EdgeOfflineTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	var frames [][]byte

	// Dùng khóa gửi để bảo vệ, đảm bảo tại một thời điểm chỉ có một yêu cầu sử dụng kết nối
	p.sendMutex.Lock()
	// Lưu ý: không giải phóng khóa khi hàm return, mà giải phóng khi goroutine hoàn thành

	// Lấy kết nối (tái sử dụng hoặc tạo mới)
	conn, err := p.getConnection(ctx)
	if err != nil {
		p.sendMutex.Unlock() // Giải phóng khóa ngay khi lấy kết nối thất bại
		return nil, err
	}

	// Gửi văn bản (dùng phương thức ghi được bảo vệ)
	err = p.writeMessage(conn, websocket.TextMessage, []byte(text))
	if err != nil {
		// Gửi thất bại, xóa kết nối, lần sau dùng sẽ tự động kết nối lại
		log.Errorf("Gửi văn bản thất bại: %v, xóa kết nối", err)
		p.clearConnection()
		p.sendMutex.Unlock() // Giải phóng khóa ngay khi gửi thất bại
		return nil, fmt.Errorf("Gửi văn bản thất bại: %v", err)
	}

	// Tạo pipe để truyền dữ liệu âm thanh
	pipeReader, pipeWriter := io.Pipe()
	outputChan := make(chan []byte, 1000)
	startTs := time.Now().UnixMilli()

	// Tạo bộ giải mã âm thanh
	audioDecoder, err := util.CreateAudioDecoder(ctx, pipeReader, outputChan, frameDuration, "mp3")
	if err != nil {
		pipeReader.Close()
		p.sendMutex.Unlock() // Giải phóng khóa ngay khi tạo bộ giải mã thất bại
		return nil, fmt.Errorf("Tạo bộ giải mã âm thanh thất bại: %v", err)
	}

	decoderDone := make(chan struct{})
	go func() {
		defer close(decoderDone)
		if err := audioDecoder.Run(startTs); err != nil {
			log.Errorf("Giải mã âm thanh thất bại: %v", err)
		}
	}()

	// Dùng WaitGroup để chờ goroutine đọc hoàn thành
	var wg sync.WaitGroup
	wg.Add(1)

	// Nhận dữ liệu WebSocket và ghi vào pipe; khóa được giải phóng thống nhất qua defer trong goroutine này, đảm bảo dù kết thúc bình thường, lỗi hay panic đều được giải phóng
	done := make(chan struct{})
	go func() {
		defer wg.Done()
		defer p.sendMutex.Unlock()
		defer close(done)
		defer pipeWriter.Close()

		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return
				}
				log.Errorf("Đọc message WebSocket thất bại: %v, xóa kết nối", err)
				// Kết nối bị ngắt, xóa kết nối, lần sau dùng sẽ tự động kết nối lại
				p.clearConnection()
				return
			}

			if messageType == websocket.BinaryMessage {
				if _, err := pipeWriter.Write(data); err != nil {
					log.Errorf("Ghi dữ liệu âm thanh thất bại: %v", err)
					return
				}
			}
		}
	}()

	// Thu thập tất cả các khung Opus
	collectorDone := make(chan struct{})
	go func() {
		for frame := range outputChan {
			frames = append(frames, frame)
		}
		close(collectorDone)
	}()

	// Chờ hoàn thành hoặc timeout
	select {
	case <-ctx.Done():
		_ = pipeWriter.CloseWithError(ctx.Err())
		p.clearConnection()
		<-decoderDone
		<-collectorDone
		return nil, fmt.Errorf("Tổng hợp TTS timeout hoặc bị hủy")
	case <-done:
		<-decoderDone
		<-collectorDone
		return frames, nil
	}
}

// TextToSpeechStream tổng hợp giọng nói dạng luồng (streaming)
func (p *EdgeOfflineTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (chan []byte, error) {
	outputChan := make(chan []byte, 100)

	go func() {
		// Dùng khóa gửi để bảo vệ, đảm bảo tại một thời điểm chỉ có một yêu cầu sử dụng kết nối
		p.sendMutex.Lock()

		// Lấy kết nối (tái sử dụng hoặc tạo mới)
		conn, err := p.getConnection(ctx)
		if err != nil {
			p.sendMutex.Unlock()
			close(outputChan)
			log.Errorf("Lấy kết nối WebSocket thất bại: %v", err)
			return
		}

		// Gửi văn bản (dùng phương thức ghi được bảo vệ)
		err = p.writeMessage(conn, websocket.TextMessage, []byte(text))
		if err != nil {
			p.sendMutex.Unlock()
			close(outputChan)
			log.Errorf("Gửi văn bản thất bại: %v, xóa kết nối", err)
			// Gửi thất bại, xóa kết nối, lần sau dùng sẽ tự động kết nối lại
			p.clearConnection()
			return
		}

		// Tạo pipe để truyền dữ liệu âm thanh
		pipeReader, pipeWriter := io.Pipe()
		startTs := time.Now().UnixMilli()
		audioDecoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, "pcm", sampleRate)
		if err != nil {
			p.sendMutex.Unlock()
			_ = pipeReader.Close()
			_ = pipeWriter.Close()
			close(outputChan)
			log.Errorf("Tạo bộ giải mã âm thanh thất bại: %v", err)
			return
		}
		audioDecoder.WithFormat(beep.Format{
			SampleRate:  beep.SampleRate(24000),
			NumChannels: channels,
			Precision:   2,
		})

		decoderDone := make(chan struct{})
		go func() {
			defer close(decoderDone)
			if err := audioDecoder.Run(startTs); err != nil {
				log.Errorf("Giải mã âm thanh thất bại: %v", err)
			}
		}()

		defer func() {
			_ = pipeWriter.Close()
			<-decoderDone
			// Giải phóng khóa sau khi đọc xong
			log.Debugf("TextToSpeechStream read completed, release sendMutex")
			p.sendMutex.Unlock()
		}()

		// Nhận dữ liệu WebSocket và ghi vào pipe (giữ khóa trong suốt quá trình đọc để đảm bảo tuần tự hóa)
		for {
			select {
			case <-ctx.Done():
				log.Debugf("TextToSpeechStream context done, exit")
				// Đóng pipeWriter, để bộ giải mã tự kết thúc và đóng channel
				return
			default:
				messageType, data, err := conn.ReadMessage()
				if err != nil {
					// Đóng pipeWriter, để bộ giải mã tự kết thúc và đóng channel
					pipeWriter.Close()
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						return
					}
					log.Errorf("Đọc message WebSocket thất bại: %v, xóa kết nối", err)
					// Kết nối bị ngắt, xóa kết nối, lần sau dùng sẽ tự động kết nối lại
					p.clearConnection()
					return
				}

				if messageType == websocket.BinaryMessage {
					if _, err := pipeWriter.Write(data); err != nil {
						log.Errorf("Ghi dữ liệu âm thanh thất bại: %v", err)
						return
					}
					return
				}
			}
		}
	}()

	return outputChan, nil
}

// SetVoice thiết lập tham số âm sắc (EdgeOffline không hỗ trợ thiết lập âm sắc động, nhưng không báo lỗi)
func (p *EdgeOfflineTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	// EdgeOffline kết nối qua WebSocket, âm sắc do server điều khiển, không hỗ trợ client thiết lập động
	// Trả về nil nghĩa là thao tác thành công (dù thực tế không thực hiện gì cả)
	return nil
}

// Close đóng tài nguyên, giải phóng kết nối
func (p *EdgeOfflineTTSProvider) Close() error {
	p.clearConnection()
	return nil
}

// IsValid kiểm tra tài nguyên có hợp lệ hay không
func (p *EdgeOfflineTTSProvider) IsValid() bool {
	p.connMutex.RLock()
	conn := p.conn
	p.connMutex.RUnlock()

	// Kiểm tra kết nối có tồn tại không
	return conn != nil
}