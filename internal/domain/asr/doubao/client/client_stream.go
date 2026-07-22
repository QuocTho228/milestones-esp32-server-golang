package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"milestones-esp32-server-golang/internal/domain/asr/doubao/request"
	"milestones-esp32-server-golang/internal/domain/asr/doubao/response"
	"milestones-esp32-server-golang/internal/util"

	log "milestones-esp32-server-golang/logger"
)

type AsrWsClient struct {
	seq            int
	url            string
	connect        *websocket.Conn
	appId          string
	accessKey      string
	resourceID     string
	connectID      string
	debugID        string
	requestOptions request.FullClientRequestOptions
	mu             sync.RWMutex // Protects connect from concurrent access

	// Các trường liên quan đến việc thiết lập kết nối trễ (lazy connect)
	connectOnce  sync.Once     // Đảm bảo kết nối chỉ được thiết lập một lần
	connectReady chan struct{} // Thông báo cho goroutine nhận biết kết nối đã được thiết lập
	connectErr   error         // Lỗi xảy ra khi thiết lập kết nối
	connectErrMu sync.Mutex    // Bảo vệ connectErr
}

func NewAsrWsClient(url string, appKey, accessKey, resourceID, connectID, debugID string, requestOptions request.FullClientRequestOptions) *AsrWsClient {
	return &AsrWsClient{
		seq:            1,
		url:            url,
		appId:          appKey,
		accessKey:      accessKey,
		resourceID:     resourceID,
		connectID:      connectID,
		debugID:        debugID,
		requestOptions: requestOptions,
		connectReady:   make(chan struct{}),
	}
}

func (c *AsrWsClient) logPrefix() string {
	if c.debugID == "" {
		return "[doubao-asr:unknown]"
	}
	return fmt.Sprintf("[doubao-asr:%s]", c.debugID)
}

func previewText(text string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 32
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

func firstNonEmptyUtteranceText(payload *response.AsrResponsePayload) string {
	if payload == nil {
		return ""
	}
	for _, utterance := range payload.Result.Utterances {
		if utterance.Text != "" {
			return utterance.Text
		}
	}
	return ""
}

func (c *AsrWsClient) CreateConnection(ctx context.Context) error {
	header := request.NewAuthHeader(c.appId, c.accessKey, c.resourceID, c.connectID)
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, c.url, header)
	if err != nil {
		if resp != nil {
			var body string
			if resp.Body != nil {
				bodyBytes, readErr := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if readErr == nil {
					body = string(bodyBytes)
				}
			}
			return fmt.Errorf("dial websocket err: %w, status=%d, body=%s", err, resp.StatusCode, body)
		}
		return fmt.Errorf("dial websocket err: %w", err)
	}
	logID := ""
	if resp != nil {
		logID = resp.Header.Get("X-Tt-Logid")
		if logID == "" {
			logID = resp.Header.Get("x-tt-logid")
		}
	}
	log.Debugf("%s websocket đã thiết lập kết nối thành công: connect_id=%s, logid=%s", c.logPrefix(), c.connectID, logID)
	c.mu.Lock()
	c.connect = conn
	c.mu.Unlock()
	return nil
}

func (c *AsrWsClient) SendFullClientRequest() error {
	c.mu.RLock()
	conn := c.connect
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("websocket connection is nil")
	}

	fullClientRequest := request.NewFullClientRequest(c.requestOptions)
	c.seq++
	err := conn.WriteMessage(websocket.BinaryMessage, fullClientRequest)
	if err != nil {
		return fmt.Errorf("full client message write websocket err: %w", err)
	}
	_, resp, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("full client message read err: %w", err)
	}
	_ = resp
	//respStruct := response.ParseResponse(resp)
	//log.Println(respStruct)
	return nil
}

// ensureConnection đảm bảo kết nối đã được thiết lập (kết nối trễ - lazy connect, có cơ chế thử lại)
func (c *AsrWsClient) ensureConnection(ctx context.Context) error {
	var err error
	c.connectOnce.Do(func() {
		log.Debugf("%s thiết lập kết nối trễ: đã nhận gói audio đầu tiên, bắt đầu thiết lập kết nối", c.logPrefix())

		// Cấu hình thử lại (retry)
		const (
			maxRetries = 3                      // Số lần thử lại tối đa (tổng cộng thử 4 lần: 1 lần ban đầu + 3 lần thử lại)
			retryDelay = 500 * time.Millisecond // Độ trễ giữa các lần thử lại
		)

		for attempt := 1; attempt <= maxRetries+1; attempt++ {
			// Thử thiết lập kết nối
			err = c.CreateConnection(ctx)
			if err != nil {
				if attempt <= maxRetries {
					log.Warnf("%s thiết lập kết nối trễ thất bại (lần %d): %v, thử lại sau %v", c.logPrefix(), attempt, err, retryDelay)
					select {
					case <-ctx.Done():
						err = fmt.Errorf("việc thiết lập kết nối đã bị hủy: %w", ctx.Err())
						c.connectErrMu.Lock()
						c.connectErr = err
						c.connectErrMu.Unlock()
						return
					case <-time.After(retryDelay):
						// Chờ đủ độ trễ cố định rồi thử lại
					}
					continue
				} else {
					// Lần thử lại cuối cùng thất bại
					log.Errorf("%s thiết lập kết nối trễ thất bại (lần %d, đã đạt số lần thử lại tối đa): %v", c.logPrefix(), attempt, err)
					c.connectErrMu.Lock()
					c.connectErr = err
					c.connectErrMu.Unlock()
					return
				}
			}

			// Kết nối thiết lập thành công, gửi request khởi tạo
			err = c.SendFullClientRequest()
			if err != nil {
				// Gửi request khởi tạo thất bại, đóng kết nối và thử lại
				log.Warnf("%s gửi request khởi tạo thất bại (lần %d): %v", c.logPrefix(), attempt, err)
				c.Close()

				if attempt <= maxRetries {
					log.Warnf("%s thử lại thiết lập kết nối sau %v", c.logPrefix(), retryDelay)
					select {
					case <-ctx.Done():
						err = fmt.Errorf("việc thiết lập kết nối đã bị hủy: %w", ctx.Err())
						c.connectErrMu.Lock()
						c.connectErr = err
						c.connectErrMu.Unlock()
						return
					case <-time.After(retryDelay):
						// Chờ đủ độ trễ cố định rồi thử lại
					}
					continue
				} else {
					// Lần thử lại cuối cùng thất bại
					log.Errorf("%s gửi request khởi tạo thất bại (lần %d, đã đạt số lần thử lại tối đa): %v", c.logPrefix(), attempt, err)
					c.connectErrMu.Lock()
					c.connectErr = err
					c.connectErrMu.Unlock()
					return
				}
			}

			// Cả kết nối lẫn khởi tạo đều thành công
			if attempt > 1 {
				log.Infof("%s thiết lập kết nối trễ thành công (lần thử thứ %d)", c.logPrefix(), attempt)
			} else {
				log.Debugf("%s thiết lập kết nối trễ thành công", c.logPrefix())
			}
			// Thông báo cho goroutine nhận biết kết nối đã được thiết lập
			close(c.connectReady)
			return
		}
	})
	return err
}

func (c *AsrWsClient) SendMessages(ctx context.Context, audioStream <-chan []float32, stopChan <-chan struct{}) error {
	messageChan := make(chan []byte)
	packetCount := 0
	totalSamples := 0
	exitReason := "unknown"
	defer func() {
		log.Debugf(
			"%s SendMessages exit: reason=%s, packets=%d, total_samples=%d, next_seq=%d",
			c.logPrefix(),
			exitReason,
			packetCount,
			totalSamples,
			c.seq,
		)
	}()
	go func() {
		for message := range messageChan {
			c.mu.RLock()
			conn := c.connect
			c.mu.RUnlock()

			if conn == nil {
				log.Debugf("%s websocket connection is nil, stopping message writer", c.logPrefix())
				return
			}

			err := conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Debugf("%s write message err: %s", c.logPrefix(), err)
				return
			}
		}
	}()

	defer close(messageChan)
	firstPacket := true
	for {
		select {
		case <-ctx.Done():
			exitReason = "context_done"
			return fmt.Errorf("send messages context done")
		case <-stopChan:
			exitReason = "stop_chan"
			return fmt.Errorf("send messages stop chan")
		case audioData, ok := <-audioStream:
			if !ok {
				exitReason = "audio_stream_closed"
				log.Debugf("%s sendMessages audioStream closed", c.logPrefix())
				// Nếu kết nối chưa được thiết lập (trường hợp im lặng), trả về ngay
				c.mu.RLock()
				conn := c.connect
				c.mu.RUnlock()
				if conn == nil {
					log.Debugf("%s audioStream đã đóng và kết nối chưa được thiết lập, trả về ngay (trường hợp im lặng)", c.logPrefix())
					return nil
				}
				// Kết nối đã được thiết lập, gửi message kết thúc
				endMessage := request.NewAudioOnlyRequest(-c.seq, []byte{})
				messageChan <- endMessage
				log.Debugf("%s gửi gói audio kết thúc: seq=%d", c.logPrefix(), -c.seq)
				return nil
			}

			// Khi nhận được gói audio đầu tiên, thiết lập kết nối
			if firstPacket {
				firstPacket = false
				err := c.ensureConnection(ctx)
				if err != nil {
					exitReason = "ensure_connection_failed"
					log.Errorf("%s thiết lập kết nối thất bại: %v", c.logPrefix(), err)
					return fmt.Errorf("ensure connection err: %w", err)
				}
			}

			packetCount++
			totalSamples += len(audioData)
			if packetCount <= 3 || packetCount%25 == 0 {
				log.Debugf(
					"%s gửi gói audio: idx=%d, seq=%d, samples=%d, total_samples=%d",
					c.logPrefix(),
					packetCount,
					c.seq,
					len(audioData),
					totalSamples,
				)
			}

			byteData := make([]byte, len(audioData)*2)
			util.Float32ToPCMBytes(audioData, byteData)
			message := request.NewAudioOnlyRequest(c.seq, byteData)
			messageChan <- message
			c.seq++
		}
	}
}

func (c *AsrWsClient) recvMessages(ctx context.Context, resChan chan<- *response.AsrResponse, stopChan chan<- struct{}) {
	recvCount := 0
	for {
		c.mu.RLock()
		conn := c.connect
		c.mu.RUnlock()

		if conn == nil {
			log.Debugf("%s websocket connection is nil, stopping message receiver", c.logPrefix())
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Warnf("%s đọc phản hồi từ Doubao thất bại: recv_count=%d, err=%v", c.logPrefix(), recvCount, err)
			return
		}
		resp := response.ParseResponse(message)
		recvCount++

		textLen := 0
		textSnippet := ""
		utteranceCount := 0
		firstUtterance := ""
		audioDuration := 0
		if resp.PayloadMsg != nil {
			textLen = len([]rune(resp.PayloadMsg.Result.Text))
			textSnippet = previewText(resp.PayloadMsg.Result.Text, 24)
			utteranceCount = len(resp.PayloadMsg.Result.Utterances)
			firstUtterance = previewText(firstNonEmptyUtteranceText(resp.PayloadMsg), 24)
			audioDuration = resp.PayloadMsg.AudioInfo.Duration
		}
		log.Debugf(
			"%s nhận được gói phản hồi: idx=%d, payload_seq=%d, event=%d, last=%v, code=%d, text_len=%d, text=%q, utterances=%d, first_utterance=%q, audio_duration=%d",
			c.logPrefix(),
			recvCount,
			resp.PayloadSequence,
			resp.Event,
			resp.IsLastPackage,
			resp.Code,
			textLen,
			textSnippet,
			utteranceCount,
			firstUtterance,
			audioDuration,
		)
		select {
		case <-ctx.Done():
			return
		case resChan <- resp:
		}
		if resp.IsLastPackage {
			log.Debugf("%s đã nhận gói phản hồi cuối cùng, dừng nhận: recv_count=%d", c.logPrefix(), recvCount)
			return
		}
		if resp.Code != 0 {
			log.Warnf("%s gói phản hồi trả về mã lỗi, thông báo cho goroutine gửi dừng lại: recv_count=%d, code=%d", c.logPrefix(), recvCount, resp.Code)
			close(stopChan)
			return
		}
	}
}

func (c *AsrWsClient) StartAudioStream(ctx context.Context, audioStream <-chan []float32, resChan chan<- *response.AsrResponse) error {
	stopChan := make(chan struct{})
	sendDoneChan := make(chan error, 1) // Thông báo gửi đã hoàn thành (nil nghĩa là hoàn thành bình thường, error nghĩa là có lỗi)
	log.Debugf("%s StartAudioStream begin", c.logPrefix())

	// Khởi chạy goroutine gửi
	go func() {
		err := c.SendMessages(ctx, audioStream, stopChan)
		// Dù thành công hay thất bại, đều gửi thông báo
		sendDoneChan <- err
	}()

	// Chờ kết nối được thiết lập hoặc việc gửi hoàn thành
	select {
	case <-ctx.Done():
		log.Debugf("%s StartAudioStream context done before connect", c.logPrefix())
		return fmt.Errorf("start audio stream context done")
	case <-c.connectReady:
		// Kết nối đã được thiết lập, khởi chạy goroutine nhận
		log.Debugf("%s kết nối đã được thiết lập, khởi chạy goroutine nhận", c.logPrefix())
		c.recvMessages(ctx, resChan, stopChan)
		return nil
	case err := <-sendDoneChan:
		// Việc gửi đã hoàn thành (có thể hoàn thành bình thường hoặc có lỗi)
		if err != nil {
			// Xảy ra lỗi trong quá trình gửi
			log.Errorf("%s gửi luồng audio thất bại: %v", c.logPrefix(), err)
			return err
		}
		// Kiểm tra xem có phải trường hợp im lặng hay không (kết nối chưa được thiết lập)
		c.mu.RLock()
		conn := c.connect
		c.mu.RUnlock()
		if conn == nil {
			// Trường hợp im lặng: audioStream đã đóng nhưng kết nối chưa được thiết lập
			log.Debugf("%s trường hợp im lặng: kết nối chưa được thiết lập, gửi kết quả rỗng", c.logPrefix())
			payload := &response.AsrResponsePayload{}
			payload.Result.Text = ""
			resChan <- &response.AsrResponse{
				Code:          0,
				IsLastPackage: true,
				PayloadMsg:    payload,
			}
			return nil
		}
		// Kết nối đã được thiết lập, khởi chạy goroutine nhận (xử lý các phản hồi còn lại)
		log.Debugf("%s SendMessages đã kết thúc, bắt đầu nhận các phản hồi còn lại", c.logPrefix())
		c.recvMessages(ctx, resChan, stopChan)
		return nil
	}
}

func (c *AsrWsClient) Excute(ctx context.Context, audioStream chan []float32, resChan chan<- *response.AsrResponse) error {
	c.seq = 1
	if c.url == "" {
		return errors.New("url is empty")
	}
	err := c.CreateConnection(ctx)
	if err != nil {
		return fmt.Errorf("create connection err: %w", err)
	}
	err = c.SendFullClientRequest()
	if err != nil {
		return fmt.Errorf("send full request err: %w", err)
	}

	err = c.StartAudioStream(ctx, audioStream, resChan)
	if err != nil {
		return fmt.Errorf("start audio stream err: %w", err)
	}
	return nil
}

func (c *AsrWsClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connect != nil {
		err := c.connect.Close()
		c.connect = nil
		return err
	}
	return nil
}