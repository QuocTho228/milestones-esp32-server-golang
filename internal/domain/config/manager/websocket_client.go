package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map/v2"

	"milestones-esp32-server-golang/internal/domain/config/types"
	"milestones-esp32-server-golang/internal/domain/mcp"
	"milestones-esp32-server-golang/internal/domain/openclaw"
	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"
)

type MessageHandleFunc func(*WebSocketRequest) (string, error)

type WebSocketClient struct {
	conn           *websocket.Conn
	baseURL        string
	requestTimeout time.Duration
	responseChans  map[string]chan *WebSocketResponse
	callbacks      map[string]func(*WebSocketResponse)
	requestHandler func(*WebSocketRequest) // Xử lý các yêu cầu đến
	mu             sync.RWMutex
	writeMu        sync.Mutex // Bảo vệ các hoạt động ghi WebSocket khỏi các hoạt động ghi đồng thời
	isConnected    bool
	connectMu      sync.Mutex
	messageQueue   chan *WebSocketRequest
	workers        sync.WaitGroup

	messageHandle cmap.ConcurrentMap[string, MessageHandleFunc]
	uuid          string

	// Kết nối lại các trường liên quan
	retryStopChan  chan struct{}  // Kết nối lại tín hiệu dừng coroutine
	retryWg        sync.WaitGroup // Kết nối lại nhóm chờ coroutine
	retryMu        sync.Mutex     // Bảo vệ các hoạt động liên quan đến kết nối lại
	isRetrying     bool           // Đang kết nối lại
	isShuttingDown bool           // Đang đóng kết nối (chủ động ngắt kết nối, không kết nối lại)
}

type WebSocketRequest struct {
	ID      string                 `json:"id"`
	Method  string                 `json:"method"`
	Path    string                 `json:"path"`
	Headers map[string]string      `json:"headers,omitempty"`
	Body    map[string]interface{} `json:"body,omitempty"`
}

type WebSocketResponse struct {
	ID      string                 `json:"id"`
	Status  int                    `json:"status"`
	Headers map[string]string      `json:"headers,omitempty"`
	Body    map[string]interface{} `json:"body,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

type managerWSClientClaims struct {
	Purpose string `json:"purpose"`
	UUID    string `json:"uuid"`
	jwt.RegisteredClaims
}

var (
	defaultClient           *WebSocketClient
	clientOnce              sync.Once
	systemConfigPushHandler func(map[string]interface{})
)

// SetSystemConfigPushHandler Thiết lập callback khi nhận được thông báo đẩy system_config (chương trình chính dùng để hợp nhất vào viper, v.v.), được user_config inject lúc Init
func SetSystemConfigPushHandler(fn func(map[string]interface{})) {
	systemConfigPushHandler = fn
}

func GetDefaultClient() *WebSocketClient {
	clientOnce.Do(func() {
		defaultClient = NewWebSocketClient()
	})
	return defaultClient
}

func NewWebSocketClient() *WebSocketClient {
	// Ưu tiên lấy từ biến môi trường, nếu biến môi trường không tồn tại thì lấy từ cấu hình
	baseURL := util.GetBackendURL()
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return &WebSocketClient{
		baseURL:        baseURL,
		requestTimeout: 30 * time.Second,
		responseChans:  make(map[string]chan *WebSocketResponse),
		callbacks:      make(map[string]func(*WebSocketResponse)),
		messageQueue:   make(chan *WebSocketRequest, 100),
		messageHandle:  cmap.New[MessageHandleFunc](),
		uuid:           uuid.New().String(),
		retryStopChan:  make(chan struct{}),
		isRetrying:     false,
	}
}

func NewWebSocketClientWithHandler(requestHandler func(*WebSocketRequest)) *WebSocketClient {
	client := NewWebSocketClient()
	client.requestHandler = requestHandler
	return client
}

func (c *WebSocketClient) Connect(ctx context.Context) error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if c.isConnected {
		return nil
	}

	// Chuyển đổi HTTP URL thành WebSocket URL
	wsURL := "ws://" + c.baseURL[7:] + "/ws" // bỏ "http://" và thêm "/ws"
	wsToken, err := c.generateWSToken()
	if err != nil {
		return fmt.Errorf("Không tạo được mã xác thực WebSocket: %v", err)
	}

	// Thiết lập kết nối WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Origin": []string{c.baseURL},
		"UUID":   []string{c.uuid},
		"Authorization": []string{
			"Bearer " + wsToken,
		},
	})
	if err != nil {
		return fmt.Errorf("Kết nối WebSocket không thành công: %v", err)
	}

	c.conn = conn
	c.isConnected = true

	// Thiết lập trình xử lý ping
	conn.SetPongHandler(func(appData string) error {
		log.Debugf("Máy khách WebSocket nhận được thông tin pong")
		return nil
	})

	// Khởi động vòng lặp xử lý tin nhắn
	go c.handleMessages()

	// Khởi động luồng xử lý gửi tin nhắn
	c.startWorkers()

	// Khởi động kiểm tra nhịp tim (heartbeat)
	go c.startHeartbeat()

	log.Debugf("Máy khách WebSocket được kết nối với: %s", wsURL)
	return nil
}

func (c *WebSocketClient) generateWSToken() (string, error) {
	claims := managerWSClientClaims{
		Purpose: "manager-ws-client",
		UUID:    c.uuid,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret := []byte(util.GetManagerEndpointAuthToken())
	return token.SignedString(secret)
}

func (c *WebSocketClient) Disconnect() error {
	return c.disconnect(false)
}

// disconnect - phương thức ngắt kết nối nội bộ
// manualDisconnect: true nghĩa là chủ động ngắt kết nối (không kích hoạt kết nối lại), false nghĩa là ngắt kết nối do lỗi (kích hoạt kết nối lại)
func (c *WebSocketClient) disconnect(manualDisconnect bool) error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if !c.isConnected {
		return nil
	}

	if manualDisconnect {
		c.isShuttingDown = true
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Debugf("Lỗi đóng kết nối WebSocket: %v", err)
		}
		c.conn = nil
	}

	c.isConnected = false
	c.mu.Lock()
	// Đóng tất cả các kênh phản hồi
	for _, ch := range c.responseChans {
		close(ch)
	}
	c.responseChans = make(map[string]chan *WebSocketResponse)
	c.callbacks = make(map[string]func(*WebSocketResponse))
	c.mu.Unlock()

	// Dừng các luồng xử lý
	close(c.messageQueue)
	c.workers.Wait()
	// Tạo lại hàng đợi tin nhắn
	c.messageQueue = make(chan *WebSocketRequest, 100)

	log.Debugf("Kết nối WebSocket bị ngắt kết nối")
	return nil
}

func (c *WebSocketClient) IsConnected() bool {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()
	return c.isConnected
}

func (c *WebSocketClient) SendRequest(ctx context.Context, method, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return nil, fmt.Errorf("Kết nối không thành công: %v", err)
		}
	}

	// Tạo UUID làm ID yêu cầu
	requestID := uuid.New().String()

	request := WebSocketRequest{
		ID:     requestID,
		Method: method,
		Path:   path,
		Body:   body,
	}

	// Tạo kênh phản hồi
	responseChan := make(chan *WebSocketResponse, 1)
	c.mu.Lock()
	c.responseChans[requestID] = responseChan
	c.mu.Unlock()

	// Dọn dẹp kênh phản hồi
	defer func() {
		c.mu.Lock()
		delete(c.responseChans, requestID)
		c.mu.Unlock()
		close(responseChan)
	}()

	// Gửi yêu cầu (được bảo vệ bằng khóa ghi)
	c.writeMu.Lock()
	err := c.conn.WriteJSON(request)
	c.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("Gửi yêu cầu không thành công: %v", err)
	}

	// Chờ phản hồi
	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(c.requestTimeout):
		return nil, fmt.Errorf("Yêu cầu hết thời gian chờ")
	case <-ctx.Done():
		return nil, fmt.Errorf("Hủy bỏ ngữ cảnh")
	}
}

// Phương thức tiện lợi - sử dụng ping gốc của WebSocket
func (c *WebSocketClient) Ping() error {
	if !c.IsConnected() {
		return fmt.Errorf("Kết nối WebSocket chưa được thiết lập")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
}

func (c *WebSocketClient) GetStatus(ctx context.Context) (*WebSocketResponse, error) {
	return c.SendRequest(ctx, "GET", "/api/ws/status", nil)
}

func (c *WebSocketClient) Echo(ctx context.Context, message string) (*WebSocketResponse, error) {
	return c.SendRequest(ctx, "POST", "/api/ws/echo", map[string]interface{}{
		"message": message,
	})
}

// Phương thức tiện lợi toàn cục
func ConnectManagerWebSocket(ctx context.Context) error {
	return GetDefaultClient().Connect(ctx)
}

func DisconnectManagerWebSocket() error {
	client := GetDefaultClient()
	client.StopReconnect()
	return client.disconnect(true) // chủ động ngắt kết nối, không kích hoạt kết nối lại
}

func SendManagerRequest(ctx context.Context, method, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	return GetDefaultClient().SendRequest(ctx, method, path, body)
}

func ManagerWebSocketPing(ctx context.Context) error {
	return GetDefaultClient().Ping()
}

func ManagerWebSocketStatus(ctx context.Context) (*WebSocketResponse, error) {
	return GetDefaultClient().GetStatus(ctx)
}

func ManagerWebSocketEcho(ctx context.Context, message string) (*WebSocketResponse, error) {
	return GetDefaultClient().Echo(ctx, message)
}

func IsManagerWebSocketConnected() bool {
	return GetDefaultClient().IsConnected()
}

func SendDeviceRequest(ctx context.Context, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	return GetDefaultClient().SendRequest(ctx, "POST", path, body)
}

// startWorkers - khởi động luồng xử lý gửi tin nhắn
func (c *WebSocketClient) startWorkers() {
	workerCount := 3 // khởi động 3 luồng xử lý

	for i := 0; i < workerCount; i++ {
		c.workers.Add(1)
		go func(workerID int) {
			defer c.workers.Done()

			log.Debugf("Luồng xử lý WebSocket của trình quản lý %d đã được khởi tạo.", workerID)

			for request := range c.messageQueue {
				if !c.IsConnected() {
					log.Debugf("Luồng xử lý %d: Không kết nối được WebSocket, yêu cầu bị hủy bỏ.", workerID)
					continue
				}

				// Gửi yêu cầu (được bảo vệ bằng khóa ghi)
				c.writeMu.Lock()
				err := c.conn.WriteJSON(request)
				c.writeMu.Unlock()
				if err != nil {
					log.Debugf("Luồng xử lý %d: Gửi yêu cầu thất bại: %v", workerID, err)
					// kết nối có thể đã bị ngắt, kích hoạt kết nối lại
					c.handleConnectionError()
					continue
				}

				log.Debugf("Luồng xử lý %d: Yêu cầu đã được gửi %s", workerID, request.ID)
			}

			log.Debugf("Luồng xử lý WebSocket của trình quản lý %d đã dừng hoạt động.", workerID)
		}(i)
	}
}

// handleConnectionError - xử lý lỗi kết nối
func (c *WebSocketClient) handleConnectionError() {
	if c.IsConnected() {
		log.Warn("Đã phát hiện lỗi kết nối WebSocket. Kết nối đang được đóng lại...")
		c.disconnect(false) // ngắt kết nối do lỗi, sẽ kích hoạt kết nối lại
		// Kích hoạt kết nối lại
		c.triggerReconnect()
	}
}

// startHeartbeat Khởi động kiểm tra nhịp tim (heartbeat)
func (c *WebSocketClient) startHeartbeat() {
	ticker := time.NewTicker(30 * time.Second) // gửi ping mỗi 30 giây
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !c.IsConnected() {
				return
			}

			// Gửi tin nhắn ping
			c.writeMu.Lock()
			err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
			c.writeMu.Unlock()

			if err != nil {
				log.Warnf("Gửi ping thất bại, kết nối có thể đã ngắt: %v", err)
				c.disconnect(false) // ngắt kết nối do lỗi, sẽ kích hoạt kết nối lại
				// Kích hoạt kết nối lại
				c.triggerReconnect()
				return
			}
			log.Debugf("Đã gửi tin nhắn ping thành công")

		case <-c.retryStopChan:
			return
		}
	}
}

// triggerReconnect - kích hoạt kết nối lại (không chặn)
func (c *WebSocketClient) triggerReconnect() {
	c.retryMu.Lock()
	defer c.retryMu.Unlock()

	// Nếu đang tắt, không kích hoạt kết nối lại
	if c.isShuttingDown {
		log.Debug("Đang tắt, không có kết nối lại nào được kích hoạt.")
		return
	}

	// Nếu đã đang kết nối lại, không kích hoạt lặp lại
	if c.isRetrying {
		return
	}

	c.isRetrying = true
	// Khởi động goroutine kết nối lại
	c.retryWg.Add(1)
	go c.startReconnectLoop()
}

// startReconnectLoop - khởi động vòng lặp kết nối lại (sử dụng thuật toán exponential backoff)
func (c *WebSocketClient) startReconnectLoop() {
	defer func() {
		c.retryMu.Lock()
		c.isRetrying = false
		c.retryMu.Unlock()
		c.retryWg.Done()
	}()

	// Tham số thuật toán backoff được hard-code
	initialDelay := 3 * time.Second // độ trễ ban đầu 3 giây
	maxDelay := 1 * time.Minute     // độ trễ tối đa 1 phút
	backoffMultiplier := 2.0        // hệ số backoff

	delay := initialDelay
	retryCount := 0

	log.Infof("Quy trình thử lại kết nối WebSocket của trình quản lý đã được khởi động.")

	for {
		// Kiểm tra xem có nên dừng kết nối lại không
		select {
		case <-c.retryStopChan:
			log.Info("Đã nhận được tín hiệu dừng, hãy dừng kết nối lại.")
			return
		default:
		}

		// Nếu đang tắt, dừng kết nối lại
		c.retryMu.Lock()
		shuttingDown := c.isShuttingDown
		c.retryMu.Unlock()
		if shuttingDown {
			log.Info("Đang đóng, vui lòng ngừng kết nối.")
			return
		}

		// Nếu đã kết nối, dừng kết nối lại
		if c.IsConnected() {
			log.Info("Kết nối WebSocket của trình quản lý đã được khôi phục, hãy ngừng kết nối lại.")
			return
		}

		retryCount++
		log.Warnf("Kết nối WebSocket của trình quản lý thất bại (%d lần), đang chờ %v lần trước khi thử lại...", retryCount, delay)

		// Chờ thời gian trễ
		select {
		case <-time.After(delay):
			// Tiếp tục kết nối lại
		case <-c.retryStopChan:
			log.Info("Đã nhận được tín hiệu dừng, hãy dừng kết nối lại.")
			return
		}

		// Thử kết nối
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := c.Connect(ctx)
		cancel()

		if err != nil {
			log.Warnf("Kết nối WebSocket của trình quản lý thất bại (số %d): %v", retryCount, err)
			// Tính thời gian trễ lần tiếp theo (exponential backoff)
			delay = time.Duration(float64(delay) * backoffMultiplier)
			if delay > maxDelay {
				delay = maxDelay
			}
			continue
		}

		// Kết nối thành công
		log.Info("Kết nối WebSocket của trình quản lý thành công")
		return
	}
}

// StopReconnect - dừng goroutine kết nối lại
func (c *WebSocketClient) StopReconnect() {
	c.retryMu.Lock()
	c.isShuttingDown = true
	shouldClose := c.retryStopChan != nil
	c.retryMu.Unlock()

	if shouldClose {
		// Sử dụng select để tránh đóng kênh nhiều lần
		select {
		case <-c.retryStopChan:
			// Kênh đã được đóng
		default:
			close(c.retryStopChan)
		}
		c.retryWg.Wait()
		log.Info("Quy trình kết nối lại WebSocket của trình quản lý đã được đóng lại một cách an toàn.")
	}
}

// SendRequestWithCallback - gửi yêu cầu và xử lý phản hồi bằng callback
func (c *WebSocketClient) SendRequestWithCallback(ctx context.Context, method, path string, body map[string]interface{}, callback func(*WebSocketResponse)) error {
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return fmt.Errorf("Kết nối thất bại: %v", err)
		}
	}

	// Tạo UUID làm ID yêu cầu
	requestID := uuid.New().String()

	request := WebSocketRequest{
		ID:     requestID,
		Method: method,
		Path:   path,
		Body:   body,
	}

	// Đăng ký callback
	c.mu.Lock()
	c.callbacks[requestID] = callback
	c.mu.Unlock()

	// Dọn dẹp callback
	defer func() {
		c.mu.Lock()
		delete(c.callbacks, requestID)
		c.mu.Unlock()
	}()

	// Đưa yêu cầu vào hàng đợi
	select {
	case c.messageQueue <- &request:
		log.Debugf("Yêu cầu %s đã được thêm vào hàng đợi.", requestID)
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("Hàng đợi tin nhắn đã đầy, yêu cầu đã hết thời gian")
	case <-ctx.Done():
		return fmt.Errorf("Hủy bỏ ngữ cảnh")
	}
}

// SendRequestAsync - gửi yêu cầu bất đồng bộ
func (c *WebSocketClient) SendRequestAsync(ctx context.Context, method, path string, body map[string]interface{}) (string, error) {
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return "", fmt.Errorf("Kết nối thất bại: %v", err)
		}
	}

	// Tạo UUID làm ID yêu cầu
	requestID := uuid.New().String()

	request := WebSocketRequest{
		ID:     requestID,
		Method: method,
		Path:   path,
		Body:   body,
	}

	// Đưa yêu cầu vào hàng đợi
	select {
	case c.messageQueue <- &request:
		log.Debugf("Yêu cầu bất đồng bộ %s đã được thêm vào hàng đợi.", requestID)
		return requestID, nil
	case <-time.After(5 * time.Second):
		return "", fmt.Errorf("Hàng đợi tin nhắn đã đầy và yêu cầu đã hết thời gian chờ.")
	case <-ctx.Done():
		return "", fmt.Errorf("Hủy bỏ ngữ cảnh")
	}
}

// GetResponse - lấy phản hồi cho ID yêu cầu chỉ định (dùng cho yêu cầu bất đồng bộ)
func (c *WebSocketClient) GetResponse(requestID string, timeout time.Duration) (*WebSocketResponse, error) {
	responseChan := make(chan *WebSocketResponse, 1)

	// Đăng ký callback tạm thời
	c.mu.Lock()
	c.callbacks[requestID] = func(response *WebSocketResponse) {
		responseChan <- response
	}
	c.mu.Unlock()

	// Dọn dẹp callback
	defer func() {
		c.mu.Lock()
		delete(c.callbacks, requestID)
		c.mu.Unlock()
		close(responseChan)
	}()

	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("Hết thời gian chờ phản hồi")
	}
}

// handleSystemConfigPush - xử lý thay đổi cấu hình hệ thống được server đẩy xuống, gọi bất đồng bộ callback đã đăng ký
func (c *WebSocketClient) handleSystemConfigPush(data map[string]interface{}) {
	if systemConfigPushHandler == nil {
		log.Debugf("Đã nhận được thông báo system_config, nhưng chưa có hàm gọi lại nào được đăng ký để xử lý nó.")
		return
	}
	go systemConfigPushHandler(data)
}

// handleMessages - xử lý tin nhắn WebSocket nhận được
func (c *WebSocketClient) handleMessages() {
	for {
		if !c.isConnected {
			return
		}

		// Đọc loại tin nhắn
		messageType, reader, err := c.conn.NextReader()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Debugf("Lỗi đọc WebSocket: %v", err)
			}
			c.disconnect(false) // ngắt kết nối do lỗi, sẽ kích hoạt kết nối lại
			// Kích hoạt kết nối lại
			c.triggerReconnect()
			return
		}

		// Xử lý các loại tin nhắn khác nhau
		switch messageType {
		case websocket.TextMessage:
			// Xử lý tin nhắn JSON
			var rawMessage map[string]interface{}
			if err := json.NewDecoder(reader).Decode(&rawMessage); err != nil {
				log.Errorf("Không thể phân tích cú pháp tin nhắn JSON: %v", err)
				continue
			}

			// Xác định theo loại tin nhắn: đẩy từ server (system_config), yêu cầu, phản hồi
			if msgType, _ := rawMessage["type"].(string); msgType == "system_config" {
				if data, ok := rawMessage["data"].(map[string]interface{}); ok {
					c.handleSystemConfigPush(data)
				} else {
					log.Warnf("Đã nhận được bản cập nhật system_config nhưng định dạng dữ liệu không hợp lệ.")
				}
			} else if method, exists := rawMessage["method"]; exists && method != nil {
				// Đây là yêu cầu nhận được
				c.handleIncomingRequest(rawMessage)
			} else if status, exists := rawMessage["status"]; exists && status != nil {
				// Đây là phản hồi nhận được
				c.handleIncomingResponse(rawMessage)
			} else {
				log.Warnf("Đã nhận được một tin nhắn WebSocket không được nhận dạng: %+v", rawMessage)
			}

		case websocket.PingMessage:
			// Xử lý tin nhắn ping, tự động trả lời pong (được bảo vệ bằng khóa ghi)
			log.Debugf("Khi nhận được tin nhắn ping, hãy tự động trả lời bằng tin nhắn pong.")
			c.writeMu.Lock()
			err := c.conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(10*time.Second))
			c.writeMu.Unlock()
			if err != nil {
				log.Errorf("Gửi pong thất bại: %v", err)
			}

		case websocket.PongMessage:
			// Xử lý tin nhắn pong
			log.Debugf("Tôi nhận được tin nhắn từ Pong.")

		case websocket.CloseMessage:
			// Xử lý tin nhắn đóng kết nối
			log.Debugf("Đã nhận được thông báo tắt máy.")
			c.disconnect(false) // ngắt kết nối do lỗi, sẽ kích hoạt kết nối lại
			// Kích hoạt kết nối lại
			c.triggerReconnect()
			return

		default:
			log.Warnf("Đã nhận được tin nhắn WebSocket không được nhận dạng: %d", messageType)
		}
	}
}

// handleIncomingRequest - xử lý yêu cầu nhận được
func (c *WebSocketClient) handleIncomingRequest(rawMessage map[string]interface{}) {
	var request WebSocketRequest
	if err := mapToStruct(rawMessage, &request); err != nil {
		log.Errorf("Lỗi phân tích yêu cầu WebSocket: %v", err)
		return
	}

	log.Debugf("Đã nhận được yêu cầu: ID=%s, Method=%s, Path=%s", request.ID, request.Method, request.Path)

	// Nếu có trình xử lý yêu cầu đã đăng ký, gọi nó
	if c.requestHandler != nil {
		go c.requestHandler(&request)
	} else {
		// Nếu chưa đăng ký trình xử lý, dùng trình xử lý mặc định để xử lý các path đã biết
		c.handleDefaultRequest(&request)
	}
}

func (c *WebSocketClient) RegisterMessageHandler(ctx context.Context, path string, handler types.EventHandler) {
	f := func(request *WebSocketRequest) (string, error) {
		return handler(ctx, request.Path, request.Body)
	}
	c.messageHandle.Set(path, f)
}

// handleDefaultRequest - trình xử lý yêu cầu mặc định
func (c *WebSocketClient) handleDefaultRequest(request *WebSocketRequest) {
	switch request.Path {
	case "/api/config/test":
		// Kiểm tra cấu hình có thể tốn nhiều thời gian (VAD/ASR/LLM/TTS chạy tuần tự), đưa vào goroutine riêng để tránh chặn vòng lặp đọc, hỗ trợ nhiều yêu cầu đồng thời
		go c.handleConfigTestRequest(request)

	case "/api/mcp/tools":
		// Xử lý yêu cầu danh sách công cụ MCP
		c.handleMcpToolListRequest(request)

	case "/api/mcp/status":
		c.handleMcpStatusRequest(request)

	case "/api/mcp/call":
		// Xử lý yêu cầu gọi công cụ MCP
		c.handleMcpToolCallRequest(request)

	case "/api/openclaw/status":
		c.handleOpenClawStatusRequest(request)

	case "/api/openclaw/chat":
		c.handleOpenClawChatRequest(request)

	case "/api/server/info":
		// Trả về thông tin server
		response := map[string]interface{}{
			"server_name": "milestones-server",
			"version":     "1.0.0",
			"uptime":      time.Now().Format(time.RFC3339),
			"request_id":  request.ID,
		}

		if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
			log.Errorf("Không thể gửi phản hồi tin nhắn đến máy chủ: %v", err)
		}

	case "/api/server/ping":
		// Phản hồi ping đơn giản
		response := map[string]interface{}{
			"message": "pong from server",
			"time":    time.Now().Format(time.RFC3339),
		}

		if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
			log.Errorf("Phản hồi ping không được gửi đi: %v", err)
		}
	default:
		handler, exists := c.messageHandle.Get(request.Path)
		if exists {
			// Gọi trình xử lý và xử lý giá trị trả về
			result, err := handler(request)
			if err != nil {
				log.Errorf("Yêu cầu %s không được xử lý thành công: %v", request.Path, err)
				// Gửi phản hồi lỗi
				if err := c.SendResponse(request.ID, 500, nil, err.Error()); err != nil {
					log.Errorf("Không thể gửi phản hồi lỗi: %v", err)
				}
			} else {
				// Gửi phản hồi thành công
				response := map[string]interface{}{
					"result": result,
				}
				if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
					log.Errorf("Không thể gửi phản hồi thành công: %v", err)
				}
			}
		} else {
			log.Warnf("Đã nhận được yêu cầu WebSocket không xác định: %s, ID: %s", request.Path, request.ID)

			// Gửi phản hồi 404
			if err := c.SendResponse(request.ID, 404, nil, "Unknown endpoint"); err != nil {
				log.Errorf("Không thể gửi phản hồi lỗi: %v", err)
			}
		}
	}
}

// configTestTotalTimeout - tổng thời gian chờ cho kiểm tra cấu hình (tổng của VAD+ASR+LLM+TTS)
const configTestTotalTimeout = 90 * time.Second

// handleConfigTestRequest - xử lý yêu cầu kiểm tra cấu hình: VAD/ASR/LLM/TTS sử dụng cấu hình được gửi xuống cùng WAV/văn bản cố định để thực hiện kiểm tra nhẹ
func (c *WebSocketClient) handleConfigTestRequest(request *WebSocketRequest) {
	data, _ := request.Body["data"].(map[string]interface{})
	if data == nil {
		log.Debugf("[config_test] yêu cầu ID=%s thiếu trường data", request.ID)
		_ = c.SendResponse(request.ID, 400, nil, "Trường data bị thiếu")
		return
	}
	testText, _ := request.Body["test_text"].(string)
	// debug: số lượng config theo từng loại trong yêu cầu (không tính provider)
	log.Debugf("[config_test] yêu cầu ID=%s test_text=%q data có số lượng mục: vad=%d asr=%d llm=%d tts=%d",
		request.ID, testText,
		countConfigKeys(data["vad"]), countConfigKeys(data["asr"]),
		countConfigKeys(data["llm"]), countConfigKeys(data["tts"]))

	type configTestResult struct {
		vad, asr, llm, tts map[string]interface{}
	}
	done := make(chan configTestResult, 1)
	go func() {
		vadR, asrR, llmR, ttsR := RunConfigTest(data, testText)
		done <- configTestResult{vadR, asrR, llmR, ttsR}
	}()

	var vadR, asrR, llmR, ttsR map[string]interface{}
	select {
	case res := <-done:
		vadR, asrR, llmR, ttsR = res.vad, res.asr, res.llm, res.tts
	case <-time.After(configTestTotalTimeout):
		log.Warnf("[config_test] yêu cầu ID=%s tổng thời gian chờ %v", request.ID, configTestTotalTimeout)
		body := map[string]interface{}{
			"vad": map[string]interface{}{"_error": map[string]interface{}{"ok": false, "message": "Tổng thời gian chờ cho kiểm tra cấu hình"}},
			"asr": map[string]interface{}{"_error": map[string]interface{}{"ok": false, "message": "Tổng thời gian chờ cho kiểm tra cấu hình"}},
			"llm": map[string]interface{}{"_error": map[string]interface{}{"ok": false, "message": "Tổng thời gian chờ cho kiểm tra cấu hình"}},
			"tts": map[string]interface{}{"_error": map[string]interface{}{"ok": false, "message": "Tổng thời gian chờ cho kiểm tra cấu hình"}},
		}
		_ = c.SendResponse(request.ID, 200, body, "")
		return
	}

	// Khi yêu cầu có loại đó nhưng không có config nào để kiểm tra, trả về _none để frontend hiển thị lý do
	fillEmptyConfigTestResult(data, "vad", vadR)
	fillEmptyConfigTestResult(data, "asr", asrR)
	fillEmptyConfigTestResult(data, "llm", llmR)
	fillEmptyConfigTestResult(data, "tts", ttsR)
	body := map[string]interface{}{
		"vad": vadR,
		"asr": asrR,
		"llm": llmR,
		"tts": ttsR,
	}
	log.Debugf("[config_test] phản hồi ID=%s số lượng kết quả của mỗi loại: vad=%d asr=%d llm=%d tts=%d",
		request.ID, len(vadR), len(asrR), len(llmR), len(ttsR))
	_ = c.SendResponse(request.ID, 200, body, "")
}

// fillEmptyConfigTestResult - khi yêu cầu chứa loại đó nhưng kết quả kiểm tra rỗng, ghi mục _none
func fillEmptyConfigTestResult(data map[string]interface{}, typ string, result map[string]interface{}) {
	if _, has := data[typ]; !has || len(result) > 0 {
		return
	}
	msg := "Chưa được cấu hình hoặc chưa được kích hoạt" + strings.ToUpper(typ)
	result["_none"] = map[string]interface{}{"ok": false, "message": msg}
	log.Debugf("[config_test] thuộc kiểu %s không có kết quả, ghi vào _none: %s", typ, msg)
}

// countConfigKeys - đếm số mục config trong data ngoại trừ provider, dùng cho debug
func countConfigKeys(v interface{}) int {
	m, ok := v.(map[string]interface{})
	if !ok {
		return 0
	}
	n := 0
	for k := range m {
		if k != "provider" {
			n++
		}
	}
	return n
}

// handleIncomingResponse - xử lý phản hồi nhận được
func (c *WebSocketClient) handleIncomingResponse(rawMessage map[string]interface{}) {
	var response WebSocketResponse
	if err := mapToStruct(rawMessage, &response); err != nil {
		log.Errorf("Không thể phân tích phản hồi WebSocket: %v", err)
		return
	}

	log.Debugf("Đã nhận được phản hồi: ID=%s, Status=%d", response.ID, response.Status)

	// Tìm kênh phản hồi và callback tương ứng
	c.mu.RLock()
	responseChan, exists := c.responseChans[response.ID]
	callback, callbackExists := c.callbacks[response.ID]
	c.mu.RUnlock()

	if exists {
		select {
		case responseChan <- &response:
		default:
			log.Debugf("Kênh phản hồi đã đầy, phản hồi sẽ bị loại bỏ: %s", response.ID)
		}
	}

	if callbackExists {
		go callback(&response)
	}

	if !exists && !callbackExists {
		log.Debugf("Đã nhận được ID phản hồi không xác định: %s", response.ID)
	}
}

// SendResponse - gửi phản hồi cho yêu cầu đã nhận
func (c *WebSocketClient) SendResponse(requestID string, status int, body map[string]interface{}, errorMsg string) error {
	if !c.IsConnected() {
		return fmt.Errorf("Kết nối WebSocket chưa được thiết lập")
	}

	response := WebSocketResponse{
		ID:     requestID,
		Status: status,
		Body:   body,
		Error:  errorMsg,
	}

	// Được bảo vệ bằng khóa ghi
	c.writeMu.Lock()
	err := c.conn.WriteJSON(response)
	c.writeMu.Unlock()
	if err != nil {
		return fmt.Errorf("Không thể gửi phản hồi: %v", err)
	}

	log.Debugf("Đã gửi phản hồi: ID=%s, Status=%d", requestID, status)
	return nil
}

// SetRequestHandler - thiết lập trình xử lý yêu cầu
func (c *WebSocketClient) SetRequestHandler(handler func(*WebSocketRequest)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requestHandler = handler
}

// mapToStruct - hàm hỗ trợ: chuyển đổi map thành struct
func mapToStruct(data map[string]interface{}, target interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}

func toolInfoToSchemaMap(paramsOneOf interface{}) map[string]interface{} {
	if paramsOneOf == nil {
		return nil
	}

	// Trường nội bộ của ParamsOneOf không được export, json.Marshal trực tiếp có thể trả về {}.
	// Ưu tiên dùng ToOpenAPIV3() chính thức để đảm bảo lấy được schema tham số thực.
	if p, ok := paramsOneOf.(*einoschema.ParamsOneOf); ok && p != nil {
		if openAPISchema, err := p.ToOpenAPIV3(); err == nil && openAPISchema != nil {
			raw, err := json.Marshal(openAPISchema)
			if err == nil {
				decoded := map[string]interface{}{}
				if err = json.Unmarshal(raw, &decoded); err == nil {
					if len(decoded) > 0 {
						return decoded
					}
				}
			}
		}
	}

	raw, err := json.Marshal(paramsOneOf)
	if err != nil {
		return nil
	}

	decoded := map[string]interface{}{}
	if err = json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}

	if openAPIV3, ok := decoded["openAPIV3"].(map[string]interface{}); ok {
		return openAPIV3
	}
	if openAPIV3, ok := decoded["open_api_v3"].(map[string]interface{}); ok {
		return openAPIV3
	}
	if len(decoded) == 0 {
		return nil
	}
	return decoded
}

func convertReportedToolsToToolList(reportedTools map[string]tool.InvokableTool) ([]map[string]interface{}, error) {
	toolList := make([]map[string]interface{}, 0)

	names := make([]string, 0, len(reportedTools))
	for name := range reportedTools {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		invokable := reportedTools[name]
		toolInfo := map[string]interface{}{
			"name":        name,
			"description": fmt.Sprintf("Công cụ MCP: %s", name),
			"schema":      true,
		}

		if info, err := invokable.Info(context.Background()); err == nil && info != nil {
			if info.Desc != "" {
				toolInfo["description"] = info.Desc
			}
			inputSchema := toolInfoToSchemaMap(info.ParamsOneOf)
			if inputSchema != nil {
				toolInfo["input_schema"] = inputSchema
			}
		}

		toolList = append(toolList, toolInfo)
	}

	return toolList, nil
}

func getDeviceMcpTools(deviceID string) ([]map[string]interface{}, error) {
	reportedTools, err := mcp.RefreshReportedToolsByDeviceID(deviceID)
	if err != nil {
		log.Errorf("Không thể làm mới danh sách các thiết bị báo cáo công cụ MCP: %v", err)
		return nil, err
	}

	return convertReportedToolsToToolList(reportedTools)
}

func getAgentMcpTools(agentID string) ([]map[string]interface{}, error) {
	reportedTools, err := mcp.RefreshReportedToolsByAgentID(agentID)
	if err != nil {
		log.Errorf("Không thể làm mới danh sách các công cụ báo cáo MCP dành cho các tác nhân thông minh: %v", err)
		return nil, err
	}

	return convertReportedToolsToToolList(reportedTools)
}

// handleMcpToolListRequest - xử lý yêu cầu danh sách công cụ MCP
func (c *WebSocketClient) handleMcpToolListRequest(request *WebSocketRequest) {
	// Lấy agent_id/device_id từ request body
	agentID := ""
	deviceID := ""
	if request.Body != nil {
		if id, ok := request.Body["agent_id"].(string); ok {
			agentID = id
		}
		if id, ok := request.Body["device_id"].(string); ok {
			deviceID = id
		}
	}

	if agentID == "" && deviceID == "" {
		log.Warnf("Đã nhận được yêu cầu cung cấp danh sách các công cụ MCP, nhưng thông tin agent_id/device_id bị thiếu.")
		if err := c.SendResponse(request.ID, 400, nil, "Thiếu tham số agent_id hoặc device_id"); err != nil {
			log.Errorf("Gửi phản hồi lỗi thất bại: %v", err)
		}
		return
	}

	log.Infof("Xử lý yêu cầu danh sách công cụ MCP, agent_id: %s, device_id: %s", agentID, deviceID)

	if agentID != "" && deviceID != "" {
		if err := c.SendResponse(request.ID, 400, nil, "Không thể truyền đồng thời agent_id và device_id."); err != nil {
			log.Errorf("Gửi phản hồi lỗi thất bại: %v", err)
		}
		return
	}

	var (
		toolList []map[string]interface{}
		err      error
	)
	if deviceID != "" {
		toolList, err = getDeviceMcpTools(deviceID)
	} else {
		toolList, err = getAgentMcpTools(agentID)
	}
	if err != nil {
		log.Errorf("Gửi yêu cầu danh sách công cụ MCP thất bại: %v", err)
		if err := c.SendResponse(request.ID, 500, nil, fmt.Sprintf("Gửi yêu cầu danh sách công cụ MCP thất bại: %v", err)); err != nil {
			log.Errorf("Gửi phản hồi lỗi thất bại: %v", err)
		}
		return
	}

	// Xây dựng phản hồi
	response := map[string]interface{}{
		"agent_id":  agentID,
		"device_id": deviceID,
		"tools":     toolList,
		"count":     len(toolList),
	}

	// Gửi phản hồi
	if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
		log.Errorf("Việc gửi phản hồi danh sách công cụ MCP đã thất bại: %v", err)
	}
}

func (c *WebSocketClient) handleMcpStatusRequest(request *WebSocketRequest) {
	agentID := ""
	if request.Body != nil {
		if id, ok := request.Body["agent_id"].(string); ok {
			agentID = strings.TrimSpace(id)
		}
	}

	if agentID == "" {
		_ = c.SendResponse(request.ID, 400, nil, "Thiếu tham số agent_id")
		return
	}

	connected, clientCount := mcp.GetWsEndpointConnectionStatus(agentID)
	status := "offline"
	if connected {
		status = "online"
	}

	response := map[string]interface{}{
		"agent_id":     agentID,
		"connected":    connected,
		"status":       status,
		"client_count": clientCount,
	}
	_ = c.SendResponse(request.ID, 200, response, "")
}

// Phương thức tiện lợi toàn cục (phiên bản bất đồng bộ)
func SendManagerRequestAsync(ctx context.Context, method, path string, body map[string]interface{}) (string, error) {
	return GetDefaultClient().SendRequestAsync(ctx, method, path, body)
}

func SendManagerRequestWithCallback(ctx context.Context, method, path string, body map[string]interface{}, callback func(*WebSocketResponse)) error {
	return GetDefaultClient().SendRequestWithCallback(ctx, method, path, body, callback)
}

func GetManagerResponse(requestID string, timeout time.Duration) (*WebSocketResponse, error) {
	return GetDefaultClient().GetResponse(requestID, timeout)
}

// Phương thức hỗ trợ giao tiếp hai chiều
func SetManagerRequestHandler(handler func(*WebSocketRequest)) {
	GetDefaultClient().SetRequestHandler(handler)
}

func SendManagerResponse(requestID string, status int, body map[string]interface{}, errorMsg string) error {
	return GetDefaultClient().SendResponse(requestID, status, body, errorMsg)
}

// Tạo client có trình xử lý yêu cầu
func NewManagerClientWithHandler(handler func(*WebSocketRequest)) *WebSocketClient {
	return NewWebSocketClientWithHandler(handler)
}

// SendMcpToolListRequest - gửi yêu cầu danh sách công cụ MCP
func SendMcpToolListRequest(ctx context.Context, agentID string) (*WebSocketResponse, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
	}
	return SendManagerRequest(ctx, "GET", "/api/mcp/tools", body)
}

// SendMcpToolListRequestAsync - gửi bất đồng bộ yêu cầu danh sách công cụ MCP
func SendMcpToolListRequestAsync(ctx context.Context, agentID string) (string, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
	}
	return SendManagerRequestAsync(ctx, "GET", "/api/mcp/tools", body)
}

// SendMcpToolListRequestWithCallback - gửi yêu cầu danh sách công cụ MCP sử dụng callback
func SendMcpToolListRequestWithCallback(ctx context.Context, agentID string, callback func(*WebSocketResponse)) error {
	body := map[string]interface{}{
		"agent_id": agentID,
	}
	return SendManagerRequestWithCallback(ctx, "GET", "/api/mcp/tools", body, callback)
}

// Init - khởi tạo trình cung cấp cấu hình Manager
// Bao gồm khởi tạo kết nối WebSocket và cơ chế kết nối lại
func Init(ctx context.Context) error {
	log.Infof("Initializing Manager config provider with WebSocket client")

	// Tạo WebSocket client
	client := GetDefaultClient()

	// Thử kết nối đến server WebSocket
	if err := client.Connect(ctx); err != nil {
		log.Warnf("Kết nối ban đầu đến WebSocket của trình quản lý đã thất bại: %v, cơ chế kết nối lại sẽ được khởi động.", err)
		// Ngay cả khi kết nối ban đầu thất bại, vẫn khởi động cơ chế kết nối lại
		client.triggerReconnect()
	} else {
		log.Infof("Manager config provider initialized successfully")
	}

	return nil
}

// Close - đóng trình cung cấp cấu hình Manager, dọn dẹp tài nguyên
func Close() error {
	log.Infof("Closing Manager config provider")

	// Dừng goroutine kết nối lại
	client := GetDefaultClient()
	client.StopReconnect()

	// Chủ động ngắt kết nối (không kích hoạt kết nối lại)
	client.disconnect(true)

	return nil
}

// IsConnected - kiểm tra trình cung cấp cấu hình Manager đã kết nối hay chưa
func IsConnected() bool {
	return IsManagerWebSocketConnected()
}

// handleMcpToolCallRequest - xử lý yêu cầu gọi công cụ MCP
func (c *WebSocketClient) handleMcpToolCallRequest(request *WebSocketRequest) {
	agentID := ""
	deviceID := ""
	toolName := ""
	arguments := map[string]interface{}{}
	if request.Body != nil {
		if id, ok := request.Body["agent_id"].(string); ok {
			agentID = id
		}
		if id, ok := request.Body["device_id"].(string); ok {
			deviceID = id
		}
		if t, ok := request.Body["tool_name"].(string); ok {
			toolName = t
		}
		if args, ok := request.Body["arguments"].(map[string]interface{}); ok {
			arguments = args
		}
	}

	if toolName == "" || (agentID == "" && deviceID == "") {
		_ = c.SendResponse(request.ID, 400, nil, "Thiếu tham số tool_name hoặc agent_id/device_id")
		return
	}

	if agentID != "" && deviceID != "" {
		_ = c.SendResponse(request.ID, 400, nil, "Không thể truyền đồng thời agent_id và device_id")
		return
	}

	var (
		invokable tool.InvokableTool
		ok        bool
	)
	if deviceID != "" {
		invokable, ok = mcp.GetReportedToolByDeviceIDAndName(deviceID, toolName)
	} else {
		invokable, ok = mcp.GetReportedToolByAgentIDAndName(agentID, toolName)
	}
	if !ok {
		var (
			result    string
			rawCalled bool
			err       error
		)
		if deviceID != "" {
			result, rawCalled, err = mcp.RawCallReportedToolByDeviceID(deviceID, toolName, arguments)
		} else {
			result, rawCalled, err = mcp.RawCallReportedToolByAgentID(agentID, toolName, arguments)
		}
		if rawCalled {
			if err != nil {
				_ = c.SendResponse(request.ID, 500, nil, fmt.Sprintf("Lỗi khi gọi công cụ (raw call): %v", err))
				return
			}
			log.Warnf("Công cụ %s không xuất hiện trong danh sách công cụ và đã được ghi đè bởi một lệnh raw call: device=%s agent=%s", toolName, deviceID, agentID)
			_ = c.SendResponse(request.ID, 200, map[string]interface{}{
				"agent_id":  agentID,
				"device_id": deviceID,
				"tool_name": toolName,
				"result":    result,
			}, "")
			return
		}
		_ = c.SendResponse(request.ID, 404, nil, fmt.Sprintf("Công cụ đó không tồn tại: %s", toolName))
		return
	}

	argBytes, _ := json.Marshal(arguments)
	result, err := invokable.InvokableRun(context.Background(), string(argBytes))
	if err != nil {
		_ = c.SendResponse(request.ID, 500, nil, fmt.Sprintf("Lỗi khi gọi công cụ: %v", err))
		return
	}

	_ = c.SendResponse(request.ID, 200, map[string]interface{}{
		"agent_id":  agentID,
		"device_id": deviceID,
		"tool_name": toolName,
		"result":    result,
	}, "")
}

func (c *WebSocketClient) handleOpenClawStatusRequest(request *WebSocketRequest) {
	agentID := ""
	if request.Body != nil {
		if id, ok := request.Body["agent_id"].(string); ok {
			agentID = strings.TrimSpace(id)
		}
	}
	if agentID == "" {
		_ = c.SendResponse(request.ID, 400, nil, "missing agent_id")
		return
	}

	manager := openclaw.GetManager()
	connected := manager.GetAgentSession(agentID) != nil
	status := "offline"
	if connected {
		status = "online"
	}

	_ = c.SendResponse(request.ID, 200, map[string]interface{}{
		"agent_id":  agentID,
		"connected": connected,
		"status":    status,
	}, "")
}

const (
	defaultOpenClawChatTimeoutMs = 10 * 60 * 1000
	minOpenClawChatTimeoutMs     = 1000
	maxOpenClawChatTimeoutMs     = 10 * 60 * 1000
	openClawChatTestSessionID    = "openclaw-chat-test-global"
)

func buildOpenClawTestDeviceID(agentID string) string {
	trimmed := strings.TrimSpace(agentID)
	if trimmed == "" {
		trimmed = "unknown"
	}
	return "__openclaw_test__:" + trimmed
}

func buildOpenClawTestSessionID() string {
	return openClawChatTestSessionID
}

func parseOpenClawTimeoutMs(v interface{}) int {
	timeout := defaultOpenClawChatTimeoutMs
	switch x := v.(type) {
	case int:
		timeout = x
	case int32:
		timeout = int(x)
	case int64:
		timeout = int(x)
	case float64:
		timeout = int(x)
	case float32:
		timeout = int(x)
	}
	if timeout < minOpenClawChatTimeoutMs {
		timeout = minOpenClawChatTimeoutMs
	}
	if timeout > maxOpenClawChatTimeoutMs {
		timeout = maxOpenClawChatTimeoutMs
	}
	return timeout
}

func parseOpenClawStreamEvents(v interface{}) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "1", "true", "yes", "on":
			return true
		}
	case int:
		return x != 0
	case int32:
		return x != 0
	case int64:
		return x != 0
	case float32:
		return x != 0
	case float64:
		return x != 0
	}
	return false
}

func openClawStreamSnippet(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}

func (c *WebSocketClient) handleOpenClawChatRequest(request *WebSocketRequest) {
	agentID := ""
	message := ""
	sessionID := ""
	timeoutMs := defaultOpenClawChatTimeoutMs
	streamEvents := false

	if request.Body != nil {
		if id, ok := request.Body["agent_id"].(string); ok {
			agentID = strings.TrimSpace(id)
		}
		if msg, ok := request.Body["message"].(string); ok {
			message = strings.TrimSpace(msg)
		}
		if rawSessionID, ok := request.Body["session_id"].(string); ok && strings.TrimSpace(rawSessionID) != "" {
			sessionID = strings.TrimSpace(rawSessionID)
		}
		timeoutMs = parseOpenClawTimeoutMs(request.Body["timeout_ms"])
		streamEvents = parseOpenClawStreamEvents(request.Body["stream_events"])
	}

	if agentID == "" {
		_ = c.SendResponse(request.ID, 400, nil, "missing agent_id")
		return
	}
	if message == "" {
		_ = c.SendResponse(request.ID, 400, nil, "missing message")
		return
	}
	if sessionID == "" {
		sessionID = buildOpenClawTestSessionID()
	}

	manager := openclaw.GetManager()
	if manager.GetAgentSession(agentID) == nil {
		_ = c.SendResponse(request.ID, 409, nil, fmt.Sprintf("openclaw session not connected for agent %s", agentID))
		return
	}

	testDeviceID := buildOpenClawTestDeviceID(agentID)
	// Dọn dẹp bộ nhớ đệm lịch sử của thiết bị kiểm tra, tránh lẫn với kết quả kiểm tra lần trước.
	manager.ReplayOfflineMessages(testDeviceID, func(msg openclaw.OfflineMessage) error {
		return nil
	})

	start := time.Now()
	messageID, err := manager.SendMessage(agentID, testDeviceID, message, sessionID)
	if err != nil {
		errMsg := strings.ToLower(strings.TrimSpace(err.Error()))
		if strings.Contains(errMsg, "session not found") {
			_ = c.SendResponse(request.ID, 409, nil, fmt.Sprintf("openclaw session not connected for agent %s", agentID))
			return
		}
		_ = c.SendResponse(request.ID, 500, nil, fmt.Sprintf("openclaw send failed: %v", err))
		return
	}
	if streamEvents {
		log.Infof(
			"openclaw chat stream started: request_id=%s agent=%s message_id=%s session=%s timeout_ms=%d",
			request.ID,
			agentID,
			messageID,
			sessionID,
			timeoutMs,
		)
	}

	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	var replyBuilder strings.Builder
	chunks := make([]string, 0, 8)
	done := false
	firstChunkLatencyMs := -1
	for time.Now().Before(deadline) {
		manager.ReplayOfflineMessages(testDeviceID, func(msg openclaw.OfflineMessage) error {
			correlationID := strings.TrimSpace(msg.CorrelationID)
			if correlationID != "" && correlationID != messageID {
				return nil
			}
			chunk := strings.TrimSpace(msg.Text)
			if chunk != "" {
				replyBuilder.WriteString(chunk)
				chunks = append(chunks, chunk)
				if firstChunkLatencyMs < 0 {
					firstChunkLatencyMs = int(time.Since(start).Milliseconds())
				}
				if streamEvents {
					log.Infof(
						"openclaw chat stream chunk received: request_id=%s agent=%s message_id=%s chunk_index=%d chunk_len=%d chunk_snippet=%q",
						request.ID,
						agentID,
						messageID,
						len(chunks),
						len(chunk),
						openClawStreamSnippet(chunk, 64),
					)
				}
				if streamEvents {
					partialBody := map[string]interface{}{
						"agent_id":    agentID,
						"message_id":  messageID,
						"chunk":       chunk,
						"chunk_index": len(chunks),
						"reply":       strings.TrimSpace(replyBuilder.String()),
						"latency_ms":  int(time.Since(start).Milliseconds()),
						"done":        false,
					}
					if firstChunkLatencyMs >= 0 {
						partialBody["first_chunk_latency_ms"] = firstChunkLatencyMs
					}
					if err := c.SendResponse(request.ID, http.StatusPartialContent, partialBody, ""); err != nil {
						log.Warnf("openclaw chat stream partial response send failed: request_id=%s, err=%v", request.ID, err)
					}
				}
			}
			if msg.IsEnd {
				if streamEvents {
					log.Infof(
						"openclaw chat stream end marker received: request_id=%s agent=%s message_id=%s chunk_count=%d partial_reply_len=%d elapsed_ms=%d",
						request.ID,
						agentID,
						messageID,
						len(chunks),
						len(strings.TrimSpace(replyBuilder.String())),
						int(time.Since(start).Milliseconds()),
					)
				}
				done = true
			}
			return nil
		})
		if done {
			break
		}
		time.Sleep(120 * time.Millisecond)
	}
	reply := strings.TrimSpace(replyBuilder.String())

	if !done {
		// Dọn dẹp bộ nhớ đệm offline của thiết bị kiểm tra, tránh tích lũy.
		manager.ReplayOfflineMessages(testDeviceID, func(msg openclaw.OfflineMessage) error {
			return nil
		})
		if reply == "" {
			if streamEvents {
				log.Warnf(
					"openclaw chat stream timeout without reply: request_id=%s agent=%s message_id=%s timeout_ms=%d",
					request.ID,
					agentID,
					messageID,
					timeoutMs,
				)
			}
			_ = c.SendResponse(request.ID, 504, nil, "openclaw response timeout")
			return
		}
		if streamEvents {
			log.Warnf(
				"openclaw chat stream timeout with partial reply: request_id=%s agent=%s message_id=%s chunk_count=%d reply_len=%d elapsed_ms=%d",
				request.ID,
				agentID,
				messageID,
				len(chunks),
				len(reply),
				int(time.Since(start).Milliseconds()),
			)
		}
		_ = c.SendResponse(request.ID, 504, map[string]interface{}{
			"agent_id":               agentID,
			"message_id":             messageID,
			"reply":                  reply,
			"chunks":                 chunks,
			"chunk_count":            len(chunks),
			"latency_ms":             int(time.Since(start).Milliseconds()),
			"first_chunk_latency_ms": firstChunkLatencyMs,
			"timeout_ms":             timeoutMs,
			"finished":               false,
		}, "openclaw response timeout (partial reply received)")
		return
	}

	latencyMs := int(time.Since(start).Milliseconds())
	if streamEvents {
		log.Infof(
			"openclaw chat stream completed: request_id=%s agent=%s message_id=%s chunk_count=%d reply_len=%d latency_ms=%d",
			request.ID,
			agentID,
			messageID,
			len(chunks),
			len(reply),
			latencyMs,
		)
	}
	var firstChunkLatency interface{}
	if firstChunkLatencyMs >= 0 {
		firstChunkLatency = firstChunkLatencyMs
	}
	_ = c.SendResponse(request.ID, 200, map[string]interface{}{
		"agent_id":               agentID,
		"message_id":             messageID,
		"reply":                  reply,
		"chunks":                 chunks,
		"chunk_count":            len(chunks),
		"latency_ms":             latencyMs,
		"first_chunk_latency_ms": firstChunkLatency,
		"timeout_ms":             timeoutMs,
		"finished":               true,
	}, "")
}