package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map/v2"
	"gorm.io/gorm"

	"milestones/manager/backend/models"
)

type WebSocketController struct {
	DB                *gorm.DB
	endpointAuthToken string
	upgrader          websocket.Upgrader
	clientsMap        cmap.ConcurrentMap[string, *WebSocketClient]
}

type WSClientClaims struct {
	Purpose string `json:"purpose"`
	UUID    string `json:"uuid"`
	jwt.RegisteredClaims
}

// WebSocketClient client kết nối tới Manager Backend
type WebSocketClient struct {
	ID           string
	conn         *websocket.Conn
	controller   *WebSocketController
	requestChans map[string]chan *WebSocketResponse
	callbacks    map[string]func(*WebSocketResponse)
	mu           sync.RWMutex
	isConnected  bool
	stopChan     chan struct{} // channel tín hiệu dừng
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

type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Schema      bool                   `json:"schema"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

const (
	defaultBroadcastRequestTimeout = 30 * time.Second
	defaultMcpStatusRequestTimeout = 3 * time.Second
	openClawChatDefaultTimeoutMs   = 10 * 60 * 1000
	openClawChatMinTimeoutMs       = 1000
	openClawChatMaxTimeoutMs       = 10 * 60 * 1000
)

// NewWebSocketController tạo mới WebSocket controller
func NewWebSocketController(db *gorm.DB, endpointAuthToken string) *WebSocketController {
	return &WebSocketController{
		DB:                db,
		endpointAuthToken: strings.TrimSpace(endpointAuthToken),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // cho phép tất cả nguồn gốc (origin), môi trường production nên giới hạn lại
			},
		},
		clientsMap: cmap.New[*WebSocketClient](),
	}
}

// HandleWebSocket xử lý nâng cấp (upgrade) kết nối WebSocket
func (ctrl *WebSocketController) HandleWebSocket(c *gin.Context) {
	tokenString := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(tokenString), "bearer ") {
		tokenString = strings.TrimSpace(tokenString[7:])
	}
	if tokenString == "" {
		tokenString = strings.TrimSpace(c.Query("token"))
	}
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu token xác thực WebSocket"})
		return
	}

	claims, err := ctrl.parseWSClientToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token WebSocket không hợp lệ"})
		return
	}
	if claims.Purpose != "manager-ws-client" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sử dụng token WebSocket không hợp lệ"})
		return
	}

	// Lấy UUID header
	clientUUID := c.GetHeader("UUID")
	if clientUUID == "" {
		log.Printf("WebSocket thiếu kết nối UUID header")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Thiếu UUID header"})
		return
	}
	if strings.TrimSpace(claims.UUID) != "" && strings.TrimSpace(claims.UUID) != strings.TrimSpace(clientUUID) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UUID không khớp với token"})
		return
	}

	// Nâng cấp (upgrade) kết nối HTTP thành kết nối WebSocket
	conn, err := ctrl.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Quá trình nâng cấp WebSocket đã thất bại: %v", err)
		return
	}

	// Kiểm tra xem đã tồn tại kết nối cùng UUID hay chưa
	if existingClient, exists := ctrl.clientsMap.Get(clientUUID); exists {
		log.Printf("Ngắt kết nối hiện có: %s", clientUUID)
		existingClient.conn.Close()
		existingClient.isConnected = false
	}

	// Tạo client mới
	client := &WebSocketClient{
		ID:           clientUUID,
		conn:         conn,
		controller:   ctrl,
		requestChans: make(map[string]chan *WebSocketResponse),
		callbacks:    make(map[string]func(*WebSocketResponse)),
		isConnected:  true,
		stopChan:     make(chan struct{}),
	}

	// Lưu vào clientsMap
	ctrl.clientsMap.Set(clientUUID, client)

	log.Printf("Đã kết nối thành công với một máy khách WebSocket mới: %s", clientUUID)

	// Khởi động xử lý message của client
	go client.handleMessages()

	// Khởi động kiểm tra heartbeat (nhịp tim)
	go client.heartbeat()
}

func (ctrl *WebSocketController) parseWSClientToken(tokenString string) (*WSClientClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &WSClientClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(ctrl.endpointAuthToken), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*WSClientClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrInvalidKey
	}
	return claims, nil
}

// Xóa bỏ client
func (ctrl *WebSocketController) removeClient(clientID string) {
	if client, exists := ctrl.clientsMap.Get(clientID); exists {
		// Gửi tín hiệu dừng cho heartbeat
		select {
		case client.stopChan <- struct{}{}:
			log.Printf("Tín hiệu dừng đã được gửi đến máy khách: %s", clientID)
		default:
			// Channel có thể đã đầy hoặc đã đóng, bỏ qua
		}

		// Đảm bảo trạng thái client được thiết lập đúng
		client.isConnected = false
		// Xóa khỏi map
		ctrl.clientsMap.Remove(clientID)
		log.Printf("Kết nối WebSocket đã bị ngắt: %s", clientID)
	}
}

// Lấy client theo UUID
func (ctrl *WebSocketController) GetClient(uuid string) *WebSocketClient {
	if client, exists := ctrl.clientsMap.Get(uuid); exists {
		return client
	}
	return nil
}

// Kiểm tra client với UUID chỉ định đã kết nối hay chưa
func (ctrl *WebSocketController) IsClientConnected(uuid string) bool {
	if client, exists := ctrl.clientsMap.Get(uuid); exists {
		return client.isConnected
	}
	return false
}

// GetFirstConnectedClientUUID trả về UUID của client đầu tiên đang kết nối, dùng cho các trường hợp như kiểm tra cấu hình
func (ctrl *WebSocketController) GetFirstConnectedClientUUID() string {
	for item := range ctrl.clientsMap.IterBuffered() {
		if client := item.Val; client.isConnected {
			return client.ID
		}
	}
	return ""
}

// Gửi message tới client với UUID chỉ định
func (ctrl *WebSocketController) SendToClient(uuid string, message interface{}) error {
	if client, exists := ctrl.clientsMap.Get(uuid); exists && client.isConnected {
		return client.conn.WriteJSON(message)
	}
	return fmt.Errorf("Máy khách %s chưa kết nối", uuid)
}

// Broadcast (phát) message tới tất cả client đang kết nối
func (ctrl *WebSocketController) Broadcast(message interface{}) {
	for item := range ctrl.clientsMap.IterBuffered() {
		if client := item.Val; client.isConnected {
			if err := client.conn.WriteJSON(message); err != nil {
				log.Printf("Không thể gửi tin nhắn đến máy khách %s: %v", client.ID, err)
			}
		}
	}
}

// BroadcastSystemConfig đẩy thay đổi cấu hình hệ thống tới tất cả client đang kết nối, định dạng giống với GET /api/system/configs: {"type":"system_config","data":{...}}
func (ctrl *WebSocketController) BroadcastSystemConfig(data gin.H) {
	ctrl.Broadcast(gin.H{"type": "system_config", "data": data})
}

// Xử lý message của client
func (client *WebSocketClient) handleMessages() {
	defer func() {
		client.conn.Close()
		client.isConnected = false
		client.controller.removeClient(client.ID)
	}()

	for {
		if !client.isConnected {
			return
		}

		// Đọc loại message
		messageType, reader, err := client.conn.NextReader()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Lỗi đọc WebSocket: %v", err)
			}
			return
		}

		// Xử lý các loại message khác nhau
		switch messageType {
		case websocket.TextMessage:
			// Xử lý message JSON
			var rawMessage map[string]interface{}
			if err := json.NewDecoder(reader).Decode(&rawMessage); err != nil {
				log.Printf("Không thể phân tích cú pháp tin nhắn JSON: %v", err)
				continue
			}
			// Xử lý message
			client.handleMessage(rawMessage)

		case websocket.PingMessage:
			// Xử lý message ping, tự động phản hồi pong
			log.Printf("Khi nhận được tin nhắn ping, hãy tự động trả lời bằng tin nhắn pong.")
			if err := client.conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
				log.Printf("Gửi pong thất bại: %v", err)
			}

		case websocket.PongMessage:
			// Xử lý message pong
			log.Printf("Tôi nhận được tin nhắn từ pong.")

		case websocket.CloseMessage:
			// Xử lý message đóng kết nối
			log.Printf("Đã nhận được thông báo tắt máy.")
			return

		default:
			log.Printf("Đã nhận được một tin nhắn WebSocket không xác định loại: %d", messageType)
		}
	}
}

// Xử lý message nhận được
func (client *WebSocketClient) handleMessage(rawMessage map[string]interface{}) {
	// Kiểm tra xem có phải message dạng request hay không
	if method, exists := rawMessage["method"]; exists && method != nil {
		client.handleRequest(rawMessage)
		return
	}

	// Kiểm tra xem có phải message dạng response hay không
	if status, exists := rawMessage["status"]; exists && status != nil {
		client.handleResponse(rawMessage)
		return
	}

	log.Printf("Đã nhận được một tin nhắn không thể nhận dạng: %+v", rawMessage)
}

// Xử lý message request
func (client *WebSocketClient) handleRequest(rawMessage map[string]interface{}) {
	var request WebSocketRequest
	if err := mapToStruct(rawMessage, &request); err != nil {
		log.Printf("Yêu cầu phân tích cú pháp thất bại: %v", err)
		return
	}

	log.Printf("Yêu cầu đã được nhận: ID=%s, Method=%s, Path=%s", request.ID, request.Method, request.Path)

	// Xử lý request và gửi response
	client.processRequest(&request)
}

// Xử lý message response
func (client *WebSocketClient) handleResponse(rawMessage map[string]interface{}) {
	var response WebSocketResponse
	if err := mapToStruct(rawMessage, &response); err != nil {
		log.Printf("Phân tích phản hồi thất bại: %v", err)
		return
	}

	log.Printf("Đã nhận được phản hồi: ID=%s, Status=%d", response.ID, response.Status)

	// Tìm channel response tương ứng
	client.mu.RLock()
	responseChan, exists := client.requestChans[response.ID]
	callback, callbackExists := client.callbacks[response.ID]
	client.mu.RUnlock()

	if exists {
		select {
		case responseChan <- &response:
		default:
			log.Printf("Kênh phản hồi đã đầy, phản hồi sẽ bị loại bỏ: %s", response.ID)
		}
	}

	if callbackExists {
		go callback(&response)
	}

	if !exists && !callbackExists {
		log.Printf("Đã nhận được ID phản hồi không xác định: %s", response.ID)
	}
}

// Xử lý request
func (client *WebSocketClient) processRequest(request *WebSocketRequest) {
	switch request.Path {
	case "/api/server/info":
		client.handleServerInfoRequest(request)

	case "/api/server/ping":
		client.handlePingRequest(request)

	case "/api/device/active":
		client.handleDeviceActiveRequest(request)

	case "/api/device/inactive":
		client.handleDeviceInactiveRequest(request)

	default:
		log.Printf("Đường dẫn yêu cầu không xác định: %s", request.Path)
		client.sendResponse(request.ID, 404, nil, "Unknown endpoint")
	}
}

// Xử lý request thông tin server
func (client *WebSocketClient) handleServerInfoRequest(request *WebSocketRequest) {
	response := map[string]interface{}{
		"server_name": "milestones-manager-backend",
		"version":     "1.0.0",
		"uptime":      time.Now().Format(time.RFC3339),
		"request_id":  request.ID,
		"client_id":   client.ID,
	}

	client.sendResponse(request.ID, 200, response, "")
}

// Xử lý request ping
func (client *WebSocketClient) handlePingRequest(request *WebSocketRequest) {
	response := map[string]interface{}{
		"message":   "pong from manager backend",
		"time":      time.Now().Format(time.RFC3339),
		"client_id": client.ID,
	}

	client.sendResponse(request.ID, 200, response, "")
}

// Xử lý request cập nhật thời gian hoạt động của thiết bị
func (client *WebSocketClient) handleDeviceActiveRequest(request *WebSocketRequest) {
	// Lấy device_id từ request body
	deviceID := ""
	if request.Body != nil {
		if id, ok := request.Body["device_id"].(string); ok {
			deviceID = id
		}
	}

	if deviceID == "" {
		log.Printf("Yêu cầu kích hoạt thiết bị đã được nhận, nhưng thông tin về device_id bị thiếu.")
		client.sendResponse(request.ID, 400, nil, "Thiếu thông số device_id")
		return
	}

	log.Printf("Xử lý yêu cầu cập nhật thời gian hoạt động của thiết bị, device_id: %s", deviceID)

	// Cập nhật thời gian hoạt động cuối cùng của thiết bị
	now := time.Now()
	result := client.controller.DB.Model(&models.Device{}).
		Where("device_name = ?", deviceID).
		Update("last_active_at", now)

	if result.Error != nil {
		log.Printf("Không thể cập nhật thời gian hoạt động của thiết bị: %v", result.Error)
		client.sendResponse(request.ID, 500, nil, fmt.Sprintf("Không thể cập nhật thời gian hoạt động của thiết bị: %v", result.Error))
		return
	}

	if result.RowsAffected == 0 {
		log.Printf("Thiết bị không tồn tại: %s", deviceID)
		client.sendResponse(request.ID, 404, nil, "Thiết bị không tồn tại")
		return
	}

	// Xây dựng response thành công
	response := map[string]interface{}{
		"device_id":      deviceID,
		"last_active_at": now.Format(time.RFC3339),
		"message":        "Thời gian hoạt động của thiết bị đã được cập nhật thành công.",
	}

	client.sendResponse(request.ID, 200, response, "")
	log.Printf("Thiết bị %s thời gian hoạt động đã được cập nhật thành: %s", deviceID, now.Format(time.RFC3339))
}

// Xử lý request thiết bị offline
func (client *WebSocketClient) handleDeviceInactiveRequest(request *WebSocketRequest) {
	// Lấy device_id từ request body
	deviceID := ""
	if request.Body != nil {
		if id, ok := request.Body["device_id"].(string); ok {
			deviceID = id
		}
	}

	if deviceID == "" {
		log.Printf("Đã nhận được yêu cầu thiết bị ngoại tuyến, nhưng thiếu device_id.")
		client.sendResponse(request.ID, 400, nil, "Thiếu thông số device_id")
		return
	}

	log.Printf("Xử lý các yêu cầu thiết bị ngoại tuyến, device_id: %s", deviceID)

	// Đặt thời gian hoạt động cuối cùng của thiết bị về 0 (trạng thái offline)
	result := client.controller.DB.Model(&models.Device{}).
		Where("device_name = ?", deviceID).
		Update("last_active_at", nil) // Đặt NULL để biểu thị trạng thái offline

	if result.Error != nil {
		log.Printf("Không thể cập nhật trạng thái ngoại tuyến của thiết bị: %v", result.Error)
		client.sendResponse(request.ID, 500, nil, fmt.Sprintf("Không thể cập nhật trạng thái ngoại tuyến của thiết bị: %v", result.Error))
		return
	}

	if result.RowsAffected == 0 {
		log.Printf("Thiết bị không tồn tại: %s", deviceID)
		client.sendResponse(request.ID, 404, nil, "Thiết bị đó không tồn tại.")
		return
	}

	// Xây dựng response thành công
	response := map[string]interface{}{
		"device_id":      deviceID,
		"last_active_at": nil, // trạng thái offline
		"message":        "Cập nhật trạng thái ngoại tuyến của thiết bị thành công.",
	}

	client.sendResponse(request.ID, 200, response, "")
	log.Printf("Thiết bị %s đang ở trạng thái ngoại tuyến.", deviceID)
}

// Gửi response
func (client *WebSocketClient) sendResponse(requestID string, status int, body map[string]interface{}, errorMsg string) {
	response := WebSocketResponse{
		ID:     requestID,
		Status: status,
		Body:   body,
		Error:  errorMsg,
	}

	if err := client.conn.WriteJSON(response); err != nil {
		log.Printf("Không thể gửi phản hồi: %v", err)
	} else {
		log.Printf("Phản hồi đã được gửi: ID=%s, Status=%d", requestID, status)
	}
}

// Kiểm tra heartbeat - sử dụng ping/pong nguyên bản (native) của WebSocket
func (client *WebSocketClient) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Đếm số lần ping thất bại liên tiếp
	pingFailCount := 0
	maxPingFailCount := 3 // cho phép thất bại liên tiếp tối đa 3 lần

	for {
		select {
		case <-client.stopChan:
			log.Printf("Khi nhận được tín hiệu dừng, quá trình theo dõi nhịp được dừng lại.")
			return
		case <-ticker.C:
			if !client.isConnected {
				return
			}

			// Kiểm tra xem kết nối còn hiệu lực hay không
			if client.conn == nil {
				log.Printf("Kết nối WebSocket trống, quá trình điếm nhịp đã dừng lại.")
				return
			}

			// Gửi ping nguyên bản (native) của WebSocket
			if err := client.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
				pingFailCount++
				log.Printf("Gửi lệnh ping thất bại (số %d): %v", pingFailCount, err)

				// Chỉ ngắt kết nối khi số lần thất bại liên tiếp vượt ngưỡng
				if pingFailCount >= maxPingFailCount {
					log.Printf("Sau %d lần ping thất bại liên tiếp, hãy ngắt kết nối WebSocket.", maxPingFailCount)
					client.conn.Close()
					return
				}
			} else {
				// ping thành công, reset lại số lần đếm thất bại
				if pingFailCount > 0 {
					log.Printf("Khôi phục ping thành công, số lần lỗi được đặt lại.")
					pingFailCount = 0
				}
			}
		}
	}
}

// Gửi request tới client (dùng để chủ động đẩy dữ liệu)
func (client *WebSocketClient) SendRequest(method, path string, body map[string]interface{}) error {
	request := WebSocketRequest{
		ID:     uuid.New().String(),
		Method: method,
		Path:   path,
		Body:   body,
	}

	return client.conn.WriteJSON(request)
}

// Gửi request và chờ response
func (client *WebSocketClient) SendRequestWithResponse(ctx context.Context, method, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	requestID := uuid.New().String()

	request := WebSocketRequest{
		ID:     requestID,
		Method: method,
		Path:   path,
		Body:   body,
	}

	// Tạo channel response
	responseChan := make(chan *WebSocketResponse, 1)
	client.mu.Lock()
	client.requestChans[requestID] = responseChan
	client.mu.Unlock()

	// Dọn dẹp channel response
	defer func() {
		client.mu.Lock()
		delete(client.requestChans, requestID)
		client.mu.Unlock()
		close(responseChan)
	}()

	// Gửi request
	if err := client.conn.WriteJSON(request); err != nil {
		return nil, fmt.Errorf("Yêu cầu gửi không thành công: %v", err)
	}

	// Chờ response
	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("Yêu cầu đã hết thời gian chờ")
	case <-ctx.Done():
		return nil, fmt.Errorf("Hủy bỏ ngữ cảnh")
	}
}

// mapToStruct hàm hỗ trợ: chuyển đổi map thành struct
func mapToStruct(data map[string]interface{}, target interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}

// Gửi request tới client với UUID chỉ định và chờ response
func (ctrl *WebSocketController) SendRequestToClient(ctx context.Context, uuid string, method, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	if client, exists := ctrl.clientsMap.Get(uuid); exists && client.isConnected {
		return client.SendRequestWithResponse(ctx, method, path, body)
	}
	return nil, fmt.Errorf("Máy khách %s chưa được kết nối", uuid)
}

// Yêu cầu danh sách công cụ MCP từ client (theo hình thức broadcast, chờ response danh sách không rỗng đầu tiên)
func (ctrl *WebSocketController) RequestMcpToolsFromClient(ctx context.Context, agentID string) ([]string, error) {
	toolDetails, err := ctrl.RequestMcpToolDetailsFromClient(ctx, agentID)
	if err != nil {
		return nil, err
	}

	toolNames := make([]string, 0, len(toolDetails))
	for _, detail := range toolDetails {
		toolNames = append(toolNames, detail.Name)
	}

	return toolNames, nil
}

func (ctrl *WebSocketController) RequestMcpToolDetailsFromClient(ctx context.Context, agentID string) ([]MCPTool, error) {
	log.Printf("Bắt đầu yêu cầu danh sách các công cụ MCP phía máy khách, agentID: %s", agentID)
	return ctrl.requestMcpToolsByBody(ctx, map[string]interface{}{"agent_id": agentID})
}

func (ctrl *WebSocketController) RequestMcpEndpointStatusFromClient(ctx context.Context, agentID string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
	}

	return ctrl.broadcastMcpStatusRequest(ctx, body)
}

// RequestDeviceMcpToolsFromClient yêu cầu danh sách công cụ MCP theo chiều thiết bị (theo hình thức broadcast, chờ response danh sách không rỗng đầu tiên)
func (ctrl *WebSocketController) RequestDeviceMcpToolsFromClient(ctx context.Context, deviceID string) ([]string, error) {
	toolDetails, err := ctrl.RequestDeviceMcpToolDetailsFromClient(ctx, deviceID)
	if err != nil {
		return nil, err
	}

	toolNames := make([]string, 0, len(toolDetails))
	for _, detail := range toolDetails {
		toolNames = append(toolNames, detail.Name)
	}

	return toolNames, nil
}

func (ctrl *WebSocketController) RequestDeviceMcpToolDetailsFromClient(ctx context.Context, deviceID string) ([]MCPTool, error) {
	log.Printf("Bắt đầu yêu cầu danh sách các công cụ MCP của thiết bị, ID thiết bị: %s", deviceID)
	return ctrl.requestMcpToolsByBody(ctx, map[string]interface{}{"device_id": deviceID})
}

func (ctrl *WebSocketController) requestMcpToolsByBody(ctx context.Context, body map[string]interface{}) ([]MCPTool, error) {
	response, err := ctrl.broadcastRequestAndWaitFirstSuccess(ctx, "GET", "/api/mcp/tools", body)
	if err != nil {
		return nil, err
	}

	toolsData, ok := response.Body["tools"]
	if !ok {
		return []MCPTool{}, nil
	}

	tools := make([]MCPTool, 0)
	switch v := toolsData.(type) {
	case []interface{}:
		for _, item := range v {
			if toolStr, ok := item.(string); ok {
				tools = append(tools, MCPTool{Name: toolStr, Description: fmt.Sprintf("Công cụ MCP: %s", toolStr), Schema: true})
				continue
			}

			toolMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := toolMap["name"].(string)
			if name == "" {
				continue
			}

			description, _ := toolMap["description"].(string)
			if description == "" {
				description = fmt.Sprintf("Công cụ MCP: %s", name)
			}

			parsed := MCPTool{Name: name, Description: description, Schema: true}
			if inputSchema, ok := toolMap["input_schema"].(map[string]interface{}); ok {
				parsed.InputSchema = inputSchema
			} else if inputSchema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
				// Tương thích với một số client trả về field dạng camelCase
				parsed.InputSchema = inputSchema
			}
			tools = append(tools, parsed)
		}
	case []string:
		for _, name := range v {
			tools = append(tools, MCPTool{Name: name, Description: fmt.Sprintf("Công cụ MCP: %s", name), Schema: true})
		}
	}

	return tools, nil
}

// CallMcpToolFromClient yêu cầu client thực thi lệnh gọi công cụ MCP
func (ctrl *WebSocketController) CallMcpToolFromClient(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	response, err := ctrl.broadcastRequestAndWaitFirstSuccess(ctx, "POST", "/api/mcp/call", body)
	if err != nil {
		return nil, err
	}

	if response.Body == nil {
		return map[string]interface{}{}, nil
	}

	return response.Body, nil
}

// RequestOpenClawStatusFromClient yêu cầu client trả về trạng thái kết nối OpenClaw
func (ctrl *WebSocketController) RequestOpenClawStatusFromClient(ctx context.Context, agentID string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
	}

	response, err := ctrl.broadcastRequestAndWaitFirstSuccess(ctx, "GET", "/api/openclaw/status", body)
	if err != nil {
		return nil, err
	}
	if response.Body == nil {
		return map[string]interface{}{}, nil
	}

	return response.Body, nil
}

// CallOpenClawChatFromClient yêu cầu client thực thi kiểm tra hội thoại (chat) OpenClaw
func (ctrl *WebSocketController) CallOpenClawChatFromClient(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	if body == nil {
		body = map[string]interface{}{}
	}
	timeoutMs := normalizeOpenClawChatTimeoutMs(body["timeout_ms"])
	body["timeout_ms"] = timeoutMs
	waitTimeout := time.Duration(timeoutMs)*time.Millisecond + 5*time.Second

	response, err := ctrl.broadcastRequestAndWaitFirstSuccessWithTimeout(ctx, "POST", "/api/openclaw/chat", body, waitTimeout)
	if err != nil {
		return nil, err
	}
	if response.Body == nil {
		return map[string]interface{}{}, nil
	}

	return response.Body, nil
}

type wsClientResponse struct {
	clientID string
	response *WebSocketResponse
}

// CallOpenClawChatStreamFromClient yêu cầu client thực thi kiểm tra hội thoại (chat) OpenClaw (callback dạng streaming)
func (ctrl *WebSocketController) CallOpenClawChatStreamFromClient(
	ctx context.Context,
	body map[string]interface{},
	onResponse func(*WebSocketResponse) error,
) (map[string]interface{}, error) {
	if body == nil {
		body = map[string]interface{}{}
	}
	timeoutMs := normalizeOpenClawChatTimeoutMs(body["timeout_ms"])
	body["timeout_ms"] = timeoutMs
	body["stream_events"] = true
	waitTimeout := time.Duration(timeoutMs)*time.Millisecond + 5*time.Second

	responseChan := make(chan wsClientResponse, 64)
	requestID := uuid.New().String()
	callbacksRegistered := 0

	for item := range ctrl.clientsMap.IterBuffered() {
		client := item.Val
		if !client.isConnected {
			continue
		}

		clientID := client.ID
		responseHandler := func(response *WebSocketResponse) {
			select {
			case responseChan <- wsClientResponse{clientID: clientID, response: response}:
			default:
				log.Printf("Kênh phản hồi trực tuyến OpenClaw đã đầy, các phản hồi đang bị loại bỏ: %s", requestID)
			}
		}

		client.mu.Lock()
		client.callbacks[requestID] = responseHandler
		client.mu.Unlock()
		callbacksRegistered++

		request := WebSocketRequest{
			ID:     requestID,
			Method: "POST",
			Path:   "/api/openclaw/chat",
			Body:   body,
		}
		if err := client.conn.WriteJSON(request); err != nil {
			log.Printf("Không thể gửi yêu cầu truyền phát OpenClaw đến máy khách %s: %v", client.ID, err)
		}
	}

	if callbacksRegistered == 0 {
		return nil, fmt.Errorf("Không có máy khách nào được kết nối")
	}

	defer func() {
		for item := range ctrl.clientsMap.IterBuffered() {
			client := item.Val
			client.mu.Lock()
			delete(client.callbacks, requestID)
			client.mu.Unlock()
		}
	}()

	selectedClientID := ""
	failedClients := map[string]bool{}
	firstError := ""
	timeout := time.After(waitTimeout)

	for {
		select {
		case event := <-responseChan:
			resp := event.response
			if resp == nil {
				continue
			}

			if selectedClientID == "" {
				if resp.Status >= http.StatusBadRequest {
					failedClients[event.clientID] = true
					if firstError == "" {
						msg := strings.TrimSpace(resp.Error)
						if msg != "" {
							firstError = msg
						}
					}
					if len(failedClients) >= callbacksRegistered {
						if firstError != "" {
							return nil, fmt.Errorf("%s", firstError)
						}
						return nil, fmt.Errorf("Tất cả khách hàng đều báo lỗi.")
					}
					continue
				}
				selectedClientID = event.clientID
			}

			if event.clientID != selectedClientID {
				continue
			}

			if onResponse != nil {
				if err := onResponse(resp); err != nil {
					return nil, err
				}
			}

			if resp.Status == http.StatusOK {
				if resp.Body == nil {
					return map[string]interface{}{}, nil
				}
				return resp.Body, nil
			}

			if resp.Status >= http.StatusBadRequest {
				msg := strings.TrimSpace(resp.Error)
				if msg == "" {
					msg = fmt.Sprintf("Yêu cầu truyền phát OpenClaw thất bại: status=%d", resp.Status)
				}
				return nil, fmt.Errorf("%s", msg)
			}
		case <-timeout:
			return nil, fmt.Errorf("Yêu cầu đã hết thời gian chờ")
		case <-ctx.Done():
			return nil, fmt.Errorf("Hủy bỏ ngữ cảnh")
		}
	}
}

func (ctrl *WebSocketController) broadcastRequestAndWaitFirstSuccess(ctx context.Context, method, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	return ctrl.broadcastRequestAndWaitFirstSuccessWithTimeout(ctx, method, path, body, defaultBroadcastRequestTimeout)
}

func isMcpStatusOnline(body map[string]interface{}) bool {
	if body == nil {
		return false
	}
	if connected, ok := body["connected"].(bool); ok && connected {
		return true
	}
	status, _ := body["status"].(string)
	return strings.EqualFold(strings.TrimSpace(status), "online")
}

func mcpStatusClientCount(body map[string]interface{}) int {
	if body == nil {
		return 0
	}
	switch v := body["client_count"].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func (ctrl *WebSocketController) broadcastMcpStatusRequest(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	requestID := uuid.New().String()

	clients := make([]*WebSocketClient, 0)
	for item := range ctrl.clientsMap.IterBuffered() {
		client := item.Val
		if client != nil && client.isConnected {
			clients = append(clients, client)
		}
	}
	if len(clients) == 0 {
		return nil, fmt.Errorf("Không có máy khách nào được kết nối")
	}

	responseChan := make(chan *WebSocketResponse, len(clients))
	responseHandler := func(response *WebSocketResponse) {
		select {
		case responseChan <- response:
		default:
			log.Printf("Kênh phản hồi trạng thái MCP đã đầy, các phản hồi đang bị loại bỏ: %s", response.ID)
		}
	}

	for _, client := range clients {
		client.mu.Lock()
		client.callbacks[requestID] = responseHandler
		client.mu.Unlock()
	}
	defer func() {
		for _, client := range clients {
			client.mu.Lock()
			delete(client.callbacks, requestID)
			client.mu.Unlock()
		}
	}()

	sentCount := 0
	for _, client := range clients {
		request := WebSocketRequest{ID: requestID, Method: "GET", Path: "/api/mcp/status", Body: body}
		if err := client.conn.WriteJSON(request); err != nil {
			log.Printf("Không thể gửi yêu cầu trạng thái MCP đến máy khách %s: %v", client.ID, err)
			continue
		}
		sentCount++
	}
	if sentCount == 0 {
		return nil, fmt.Errorf("Hiện không có khách hàng nào.")
	}

	offline := map[string]interface{}{
		"connected":    false,
		"status":       "offline",
		"client_count": 0,
	}
	responsesReceived := 0
	successResponses := 0
	firstError := ""
	timeout := time.After(defaultMcpStatusRequestTimeout)
	for {
		select {
		case response := <-responseChan:
			responsesReceived++
			if response != nil && response.Status == http.StatusOK {
				successResponses++
				if isMcpStatusOnline(response.Body) {
					return response.Body, nil
				}
				offline["client_count"] = mcpStatusClientCount(offline) + mcpStatusClientCount(response.Body)
			} else if response != nil && firstError == "" {
				firstError = strings.TrimSpace(response.Error)
			}

			if responsesReceived >= sentCount {
				if successResponses > 0 {
					return offline, nil
				}
				if firstError != "" {
					return nil, fmt.Errorf("%s", firstError)
				}
				return nil, fmt.Errorf("Tất cả khách hàng đều báo lỗi.")
			}
		case <-timeout:
			if successResponses > 0 {
				return offline, nil
			}
			if firstError != "" {
				return nil, fmt.Errorf("%s", firstError)
			}
			return nil, fmt.Errorf("Yêu cầu đã hết thời gian chờ")
		case <-ctx.Done():
			return nil, fmt.Errorf("Hủy bỏ ngữ cảnh")
		}
	}
}

func normalizeOpenClawChatTimeoutMs(v interface{}) int {
	timeout := openClawChatDefaultTimeoutMs
	switch x := v.(type) {
	case int:
		timeout = x
	case int32:
		timeout = int(x)
	case int64:
		timeout = int(x)
	case float32:
		timeout = int(x)
	case float64:
		timeout = int(x)
	}

	if timeout < openClawChatMinTimeoutMs {
		timeout = openClawChatMinTimeoutMs
	}
	if timeout > openClawChatMaxTimeoutMs {
		timeout = openClawChatMaxTimeoutMs
	}
	return timeout
}

func (ctrl *WebSocketController) broadcastRequestAndWaitFirstSuccessWithTimeout(
	ctx context.Context,
	method, path string,
	body map[string]interface{},
	waitTimeout time.Duration,
) (*WebSocketResponse, error) {
	if waitTimeout <= 0 {
		waitTimeout = defaultBroadcastRequestTimeout
	}

	responseChan := make(chan *WebSocketResponse, 10)
	requestID := uuid.New().String()

	responseHandler := func(response *WebSocketResponse) {
		select {
		case responseChan <- response:
		default:
			log.Printf("Kênh phản hồi đã đầy. Các phản hồi sẽ bị loại bỏ: %s", response.ID)
		}
	}

	callbacksRegistered := 0
	for item := range ctrl.clientsMap.IterBuffered() {
		client := item.Val
		if !client.isConnected {
			continue
		}

		client.mu.Lock()
		client.callbacks[requestID] = responseHandler
		client.mu.Unlock()
		callbacksRegistered++

		request := WebSocketRequest{ID: requestID, Method: method, Path: path, Body: body}
		if err := client.conn.WriteJSON(request); err != nil {
			log.Printf("Không thể gửi yêu cầu đến máy khách %s: %v", client.ID, err)
		}
	}

	if callbacksRegistered == 0 {
		return nil, fmt.Errorf("Không có máy khách nào được kết nối")
	}

	defer func() {
		for item := range ctrl.clientsMap.IterBuffered() {
			client := item.Val
			client.mu.Lock()
			delete(client.callbacks, requestID)
			client.mu.Unlock()
		}
	}()

	responsesReceived := 0
	firstError := ""
	timeout := time.After(waitTimeout)
	for {
		select {
		case response := <-responseChan:
			responsesReceived++
			if response != nil && response.Status == http.StatusOK {
				return response, nil
			}
			if response != nil && firstError == "" {
				msg := strings.TrimSpace(response.Error)
				if msg != "" {
					firstError = msg
				}
			}
			if responsesReceived >= callbacksRegistered {
				if firstError != "" {
					return nil, fmt.Errorf("%s", firstError)
				}
				return nil, fmt.Errorf("Tất cả khách hàng đều báo lỗi.")
			}
		case <-timeout:
			return nil, fmt.Errorf("Yêu cầu đã hết thời gian chờ")
		case <-ctx.Done():
			return nil, fmt.Errorf("Hủy bỏ ngữ cảnh")
		}
	}
}

// Yêu cầu thông tin server từ client
func (ctrl *WebSocketController) RequestServerInfoFromClient(ctx context.Context, uuid string) (*WebSocketResponse, error) {
	return ctrl.SendRequestToClient(ctx, uuid, "GET", "/api/server/info", nil)
}

func (ctrl *WebSocketController) RequestDeviceActivation(ctx context.Context, uuid, deviceID string) (*WebSocketResponse, error) {
	return ctrl.SendRequestToClient(ctx, uuid, "GET", "/api/device/activation", map[string]interface{}{
		"device_id": deviceID,
	})
}

// Yêu cầu ping từ client
func (ctrl *WebSocketController) RequestPingFromClient(ctx context.Context, uuid string) (*WebSocketResponse, error) {
	return ctrl.SendRequestToClient(ctx, uuid, "GET", "/api/server/ping", nil)
}

// InjectMessageToDevice chèn (inject) message vào thiết bị (theo hình thức broadcast)
func (ctrl *WebSocketController) InjectMessageToDevice(ctx context.Context, deviceID, message string, skipLlm bool, autoListen bool) error {
	body := map[string]interface{}{
		"device_id":   deviceID,
		"message":     message,
		"skip_llm":    skipLlm,
		"auto_listen": autoListen,
	}

	// Tạo request
	request := WebSocketRequest{
		ID:     uuid.New().String(),
		Method: "POST",
		Path:   "/api/device/inject_msg",
		Body:   body,
	}

	// Broadcast tới tất cả client đang kết nối
	var lastError error
	clientCount := 0

	for item := range ctrl.clientsMap.IterBuffered() {
		client := item.Val
		if client.isConnected {
			clientCount++
			if err := client.conn.WriteJSON(request); err != nil {
				log.Printf("Không thể phát sóng và chèn tin nhắn đến máy khách %s: %v", client.ID, err)
				lastError = err
			} else {
				log.Printf("Đã gửi thành công thông điệp tấn công đến máy khách %s", client.ID)
			}
		}
	}

	if clientCount == 0 {
		return fmt.Errorf("Không có máy khách nào được kết nối")
	}

	return lastError
}

// Gửi request bất đồng bộ (async) tới client (không chờ response)
func (ctrl *WebSocketController) SendRequestToClientAsync(uuid string, method, path string, body map[string]interface{}) error {
	if client, exists := ctrl.clientsMap.Get(uuid); exists && client.isConnected {
		return client.SendRequest(method, path, body)
	}
	return fmt.Errorf("Máy khách %s chưa kết nối", uuid)
}

// Lấy trạng thái kết nối của tất cả client
func (ctrl *WebSocketController) GetClientConnectionStatus() map[string]interface{} {
	clients := make([]map[string]interface{}, 0)
	for item := range ctrl.clientsMap.IterBuffered() {
		client := item.Val
		clients = append(clients, map[string]interface{}{
			"uuid":      client.ID,
			"connected": client.isConnected,
		})
	}

	return map[string]interface{}{
		"clients": clients,
		"count":   len(clients),
	}
}

// Lấy trạng thái kết nối của client chỉ định
func (ctrl *WebSocketController) GetClientStatus(uuid string) map[string]interface{} {
	if client, exists := ctrl.clientsMap.Get(uuid); exists {
		return map[string]interface{}{
			"uuid":      client.ID,
			"connected": client.isConnected,
			"message":   "Máy khách đã được kết nối",
		}
	}

	return map[string]interface{}{
		"uuid":      uuid,
		"connected": false,
		"message":   "Máy khách chưa được kết nối",
	}
}