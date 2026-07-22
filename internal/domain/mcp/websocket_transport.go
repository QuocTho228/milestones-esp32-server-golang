package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	log "milestones-esp32-server-golang/logger"
)

const (
	// DefaultRequestTimeout Thời gian chờ (timeout) request mặc định
	DefaultRequestTimeout = 30 * time.Second
	// DefaultCloseTimeout Thời gian chờ (timeout) đóng kết nối mặc định
	DefaultCloseTimeout = 5 * time.Second
)

type pendingResponseResult struct {
	response *transport.JSONRPCResponse
	err      error
}

type pendingResponse struct {
	resultCh chan pendingResponseResult
	once     sync.Once
}

func newPendingResponse() *pendingResponse {
	return &pendingResponse{
		resultCh: make(chan pendingResponseResult, 1),
	}
}

func (p *pendingResponse) resolve(response *transport.JSONRPCResponse, err error) {
	if p == nil {
		return
	}
	p.once.Do(func() {
		p.resultCh <- pendingResponseResult{
			response: response,
			err:      err,
		}
	})
}

type jsonRPCMessageEnvelope struct {
	Method string           `json:"method"`
	ID     *json.RawMessage `json:"id"`
}

func classifyJSONRPCMessage(message []byte) (method string, hasID bool, err error) {
	var envelope jsonRPCMessageEnvelope
	if err := json.Unmarshal(message, &envelope); err != nil {
		return "", false, err
	}
	return envelope.Method, envelope.ID != nil, nil
}

func requestIDKey(id mcp.RequestId) string {
	raw, err := id.MarshalJSON()
	if err == nil {
		return string(raw)
	}
	return id.String()
}

func isTransportTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	lowerErr := strings.ToLower(err.Error())
	return strings.Contains(lowerErr, "timeout") || strings.Contains(lowerErr, "hết thời gian")
}

/**
// Interface for the transport layer.
type Interface interface {
	// Start the connection. Start should only be called once.
	Start(ctx context.Context) error

	// SendRequest sends a json RPC request and returns the response synchronously.
	SendRequest(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error)

	// SendNotification sends a json RPC Notification to the server.
	SendNotification(ctx context.Context, notification mcp.JSONRPCNotification) error

	// SetNotificationHandler sets the handler for notifications.
	// Any notification before the handler is set will be discarded.
	SetNotificationHandler(handler func(notification mcp.JSONRPCNotification))

	// Close the connection.
	Close() error
}
*/

type WebsocketTransport struct {
	url  string
	conn *websocket.Conn

	notifyHandler func(notification mcp.JSONRPCNotification)
	// Callback khi đóng kết nối
	onCloseHandler func(reason string)

	// Quản lý các channel phản hồi
	respChans    map[string]*pendingResponse
	respChansMux sync.RWMutex

	// Kiểm soát việc lắng nghe tin nhắn
	readDone chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc

	// Trạng thái kết nối
	closed    bool
	closedMux sync.RWMutex

	// Lock ghi (write) của WebSocket, tránh ghi đồng thời
	writeMux sync.Mutex

	// Cấu hình timeout
	requestTimeout time.Duration
	closeTimeout   time.Duration
}

func (t *WebsocketTransport) Send(ctx context.Context, msg []byte) error {
	// Kiểm tra trạng thái kết nối
	t.closedMux.RLock()
	if t.closed {
		t.closedMux.RUnlock()
		return fmt.Errorf("connection is closed")
	}
	t.closedMux.RUnlock()

	// Gửi tin nhắn (dùng mutex để bảo vệ thao tác ghi)
	t.writeMux.Lock()
	err := t.conn.WriteMessage(websocket.TextMessage, msg)
	t.writeMux.Unlock()
	return err
}

func NewWebsocketTransport(conn *websocket.Conn) (*WebsocketTransport, error) {
	ctx, cancel := context.WithCancel(context.Background())

	wst := &WebsocketTransport{
		conn:           conn,
		respChans:      make(map[string]*pendingResponse),
		readDone:       make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
		requestTimeout: DefaultRequestTimeout,
		closeTimeout:   DefaultCloseTimeout,
	}
	// Khởi động goroutine lắng nghe tin nhắn
	go wst.readMessages()

	return wst, nil
}

// Triển khai interface Interface
func (t *WebsocketTransport) Start(ctx context.Context) error {
	return nil
}

func (t *WebsocketTransport) popPending(id string) *pendingResponse {
	t.respChansMux.Lock()
	defer t.respChansMux.Unlock()

	pending := t.respChans[id]
	if pending != nil {
		delete(t.respChans, id)
	}
	return pending
}

func (t *WebsocketTransport) failAllPending(err error) {
	t.respChansMux.Lock()
	pending := make([]*pendingResponse, 0, len(t.respChans))
	for id, pendingResp := range t.respChans {
		pending = append(pending, pendingResp)
		delete(t.respChans, id)
	}
	t.respChansMux.Unlock()

	for _, pendingResp := range pending {
		pendingResp.resolve(nil, err)
	}
}

// readMessages Liên tục lắng nghe tin nhắn WebSocket
func (t *WebsocketTransport) readMessages() {
	defer close(t.readDone)

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// Sử dụng cơ chế kiểm soát timeout ở tầng ngôn ngữ Go
			_, message, err := t.conn.ReadMessage()
			if err != nil {
				t.closedMux.Lock()
				t.closed = true
				t.closedMux.Unlock()
				t.failAllPending(fmt.Errorf("connection is closed"))

				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Errorf("WebSocket read error: %v", err)
				}

				// Thông báo cho tầng client khi kết nối bị đóng
				if t.onCloseHandler != nil {
					reason := "connection_closed"
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						reason = "normal_closure"
					} else if websocket.IsUnexpectedCloseError(err) {
						reason = "unexpected_closure"
					}
					t.onCloseHandler(reason)
				}

				return
			}

			// Xử lý tin nhắn nhận được
			t.handleMessage(message)
		}
	}
}

// handleMessage Xử lý tin nhắn nhận được
func (t *WebsocketTransport) handleMessage(message []byte) {
	method, hasID, err := classifyJSONRPCMessage(message)
	if err != nil {
		log.Warnf("Received unrecognized message: %s", string(message))
		return
	}

	if method != "" {
		if hasID {
			log.Warnf("Received unsupported JSON-RPC request: %s", method)
			return
		}

		var notification mcp.JSONRPCNotification
		if err := json.Unmarshal(message, &notification); err != nil {
			log.Warnf("Received malformed JSON-RPC notification: %s", string(message))
			return
		}
		t.handleNotification(&notification)
		return
	}

	if hasID {
		var response transport.JSONRPCResponse
		if err := json.Unmarshal(message, &response); err != nil {
			log.Warnf("Received malformed JSON-RPC response: %s", string(message))
			return
		}
		t.handleResponse(&response)
		return
	}

	// Định dạng tin nhắn không thể nhận dạng được
	log.Warnf("Received unrecognized message: %s", string(message))
}

// handleResponse Xử lý phản hồi JSON-RPC
func (t *WebsocketTransport) handleResponse(response *transport.JSONRPCResponse) {
	respByte, _ := json.Marshal(response)
	// Chuyển ID thành chuỗi để dùng làm khóa (key)
	idStr := requestIDKey(response.ID)

	pending := t.popPending(idStr)
	if pending == nil {
		log.Warnf("No response channel found for ID: %s, response: %+v", idStr, string(respByte))
		return
	}
	pending.resolve(response, nil)
}

// handleNotification Xử lý thông báo (notification) JSON-RPC
func (t *WebsocketTransport) handleNotification(notification *mcp.JSONRPCNotification) {
	if t.notifyHandler != nil {
		t.notifyHandler(*notification)
	}
}

func (t *WebsocketTransport) SendRequest(ctx context.Context, request transport.JSONRPCRequest) (*transport.JSONRPCResponse, error) {
	// Kiểm tra trạng thái kết nối
	t.closedMux.RLock()
	if t.closed {
		t.closedMux.RUnlock()
		return nil, fmt.Errorf("connection is closed")
	}
	t.closedMux.RUnlock()

	// Tạo channel phản hồi
	idStr := requestIDKey(request.ID)
	pending := newPendingResponse()

	// Đăng ký channel phản hồi
	t.respChansMux.Lock()
	t.respChans[idStr] = pending
	t.respChansMux.Unlock()

	// Gửi request (dùng mutex để bảo vệ thao tác ghi)
	t.writeMux.Lock()
	err := t.conn.WriteJSON(request)
	t.writeMux.Unlock()
	if err != nil {
		// Gửi thất bại, dọn dẹp channel
		t.popPending(idStr)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Sử dụng cơ chế kiểm soát timeout ở tầng ngôn ngữ Go để chờ phản hồi
	select {
	case result := <-pending.resultCh:
		if result.err != nil {
			return nil, result.err
		}
		return result.response, nil
	case <-ctx.Done():
		// Context bị hủy, dọn dẹp channel
		t.popPending(idStr)
		return nil, ctx.Err()
	case <-time.After(t.requestTimeout):
		// Kiểm soát timeout ở tầng ngôn ngữ Go
		t.popPending(idStr)
		return nil, fmt.Errorf("request timeout")
	}
}

func (t *WebsocketTransport) SendNotification(ctx context.Context, notification mcp.JSONRPCNotification) error {
	// Kiểm tra trạng thái kết nối
	t.closedMux.RLock()
	if t.closed {
		t.closedMux.RUnlock()
		return fmt.Errorf("connection is closed")
	}
	t.closedMux.RUnlock()

	// Gửi tin nhắn thông báo (dùng mutex để bảo vệ thao tác ghi)
	t.writeMux.Lock()
	err := t.conn.WriteJSON(notification)
	t.writeMux.Unlock()
	return err
}

func (t *WebsocketTransport) SetNotificationHandler(handler func(notification mcp.JSONRPCNotification)) {
	t.notifyHandler = handler
}

// SetOnCloseHandler Thiết lập callback khi đóng kết nối
func (t *WebsocketTransport) SetOnCloseHandler(handler func(reason string)) {
	t.onCloseHandler = handler
}

func (t *WebsocketTransport) Close() error {
	// Đánh dấu kết nối đã đóng
	t.closedMux.Lock()
	t.closed = true
	t.closedMux.Unlock()
	t.failAllPending(fmt.Errorf("connection is closed"))

	// Thông báo cho tầng client rằng kết nối sắp bị đóng
	if t.onCloseHandler != nil {
		t.onCloseHandler("manual_close")
	}

	// Hủy context
	t.cancel()

	// Chờ goroutine đọc kết thúc
	select {
	case <-t.readDone:
	case <-time.After(t.closeTimeout):
		log.Warnf("Timeout waiting for read goroutine to finish")
	}

	// Đóng kết nối WebSocket
	return t.conn.Close()
}

func (t *WebsocketTransport) GetSessionId() string {
	return t.conn.RemoteAddr().String()
}

// IsClosed Kiểm tra kết nối đã đóng hay chưa
func (t *WebsocketTransport) IsClosed() bool {
	t.closedMux.RLock()
	defer t.closedMux.RUnlock()
	return t.closed
}

// GetActiveRequests Lấy số lượng request đang hoạt động hiện tại
func (t *WebsocketTransport) GetActiveRequests() int {
	t.respChansMux.RLock()
	defer t.respChansMux.RUnlock()
	return len(t.respChans)
}

// SetRequestTimeout Thiết lập thời gian chờ (timeout) cho request
func (t *WebsocketTransport) SetRequestTimeout(timeout time.Duration) {
	t.requestTimeout = timeout
}

// SetCloseTimeout Thiết lập thời gian chờ (timeout) khi đóng kết nối
func (t *WebsocketTransport) SetCloseTimeout(timeout time.Duration) {
	t.closeTimeout = timeout
}

// GetRequestTimeout Lấy thời gian chờ (timeout) hiện tại của request
func (t *WebsocketTransport) GetRequestTimeout() time.Duration {
	return t.requestTimeout
}

// GetCloseTimeout Lấy thời gian chờ (timeout) hiện tại khi đóng kết nối
func (t *WebsocketTransport) GetCloseTimeout() time.Duration {
	return t.closeTimeout
}