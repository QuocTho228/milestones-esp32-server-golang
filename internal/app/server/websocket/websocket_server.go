package websocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"milestones-esp32-server-golang/internal/app/server/auth"
	"milestones-esp32-server-golang/internal/app/server/types"
	"milestones-esp32-server-golang/internal/domain/mcp"
	"milestones-esp32-server-golang/internal/domain/openclaw"
	log "milestones-esp32-server-golang/logger"
)

// WebSocketServer 表示 WebSocket 服务器
type WebSocketServer struct {
	// 配置升级器
	upgrader websocket.Upgrader
	// 客户端状态，使用 sync.Map 实现并发安全
	clientStates sync.Map
	// 认证管理器
	authManager *auth.AuthManager
	// 端口
	port int
	// MCP管理器
	globalMCPManager *mcp.GlobalMCPManager

	onNewConnection    types.OnNewConnection
	onOpenClawResponse func(event openclaw.ResponseDelivery) bool
	onInjectMessage    func(deviceID, message string, skipLlm bool, autoListen bool) error
}

// Option 类型定义
// WebSocketServerOption 用于配置 WebSocketServer 的可选参数
type WebSocketServerOption func(*WebSocketServer)

// WithAuthManager 设置认证管理器
func WithAuthManager(authManager *auth.AuthManager) WebSocketServerOption {
	return func(s *WebSocketServer) {
		s.authManager = authManager
	}
}

// WithMCPManager 设置 MCP 管理器
func WithMCPManager(mcpManager *mcp.GlobalMCPManager) WebSocketServerOption {
	return func(s *WebSocketServer) {
		s.globalMCPManager = mcpManager
	}
}

func WithOnNewConnection(onNewConnection types.OnNewConnection) WebSocketServerOption {
	return func(s *WebSocketServer) {
		s.onNewConnection = onNewConnection
	}
}

func WithOnOpenClawResponse(handler func(event openclaw.ResponseDelivery) bool) WebSocketServerOption {
	return func(s *WebSocketServer) {
		s.onOpenClawResponse = handler
	}
}

func WithOnInjectMessage(handler func(deviceID, message string, skipLlm bool, autoListen bool) error) WebSocketServerOption {
	return func(s *WebSocketServer) {
		s.onInjectMessage = handler
	}
}

// NewWebSocketServer 创建新的 WebSocket 服务器（WithOption 方式）
func NewWebSocketServer(port int, opts ...WebSocketServerOption) *WebSocketServer {
	s := &WebSocketServer{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源的连接
			},
		},
		// 默认值
		authManager:      auth.A(),
		port:             port,
		globalMCPManager: mcp.GetGlobalMCPManager(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start 启动 WebSocket 服务器
func (s *WebSocketServer) Start() error {
	// 启动所有MCP管理器（通过统一管理器）
	if err := mcp.StartMCPManagers(); err != nil {
		log.Errorf("Cụm MCP Manager không khởi động được: %v", err)
		return err
	}

	// 启动会话清理
	go s.cleanupSessions()

	// 注册路由处理器
	http.HandleFunc("/milestones/mqtt_udp/v1/", s.handleMqttUdpChat)
	http.HandleFunc("/milestones/v1/", s.handleChat)
	http.HandleFunc("/milestones/ota/", s.handleOta)
	http.HandleFunc("/milestones/ota/activate", s.handleOtaActivate)
	http.HandleFunc("/mcp", s.handleMCPWebSocket)
	http.HandleFunc("/ws/openclaw", s.handleOpenClawWebSocket)
	http.HandleFunc("/milestones/api/mcp/tools/", s.handleMCPAPI)
	http.HandleFunc("/milestones/api/vision", s.handleVisionAPI) //图片识别API

	http.HandleFunc("/admin/inject_msg", s.handleInjectMsg)

	listenAddr := fmt.Sprintf("0.0.0.0:%d", s.port)
	log.Infof("Máy chủ WebSocket khởi động trong ws://%s/milestones/v1/", listenAddr)
	log.Infof("Điểm cuối WebSocket MCP: ws://%s/mcp?token=xxx", listenAddr)
	log.Infof("Điểm cuối WebSocket OpenClaw: ws://%s/ws/openclaw?token=xxx", listenAddr)
	log.Infof("Điểm cuối MCP API: http://%s/milestones/api/mcp/tools/{deviceId}", listenAddr)

	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Log().Fatalf("Máy chủ WebSocket không khởi động được.: %v", err)
		return err
	}
	return nil
}

// handleGetDeviceTools 获取设备的工具列表
func (s *WebSocketServer) handleGetDeviceTools(w http.ResponseWriter, r *http.Request, deviceID string) {

}

// cleanupSessions 定期清理过期会话
func (s *WebSocketServer) cleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.authManager.CleanupSessions(30 * time.Minute)
	}
}

// handleWebSocket 处理 WebSocket 连接
func (s *WebSocketServer) handleChat(w http.ResponseWriter, r *http.Request) {
	s.internalHandleChat(w, r, false)
}

// handleWebSocket 处理 WebSocket 连接
func (s *WebSocketServer) handleMqttUdpChat(w http.ResponseWriter, r *http.Request) {
	s.internalHandleChat(w, r, true)
}

// handleWebSocket 处理 WebSocket 连接
func (s *WebSocketServer) internalHandleChat(w http.ResponseWriter, r *http.Request, isMqttUdp bool) {
	deviceID, clientID := extractDeviceAndClientID(r)
	if deviceID == "" {
		log.Warn("Thiếu device-id, vui lòng chuyển nó từ tham số Header hoặc URL")
		http.Error(w, "Thiếu device-id (hỗ trợ tham số Header hoặc URL)", http.StatusBadRequest)
		return
	}
	if clientID == "" {
		log.Debugf("Kết nối không cung cấp client-id, device_id: %s", deviceID)
	}

	/*isAuth := viper.GetBool("auth.enable")
	if isAuth {
		token := r.Header.Get("Authorization")
		if token == "" {
			log.Warn("缺少 Authorization 请求头")
			http.Error(w, "缺少 Authorization 请求头", http.StatusUnauthorized)
			return
		}

		// 验证令牌
		if !s.authManager.ValidateToken(token) {
			log.Warnf("无效的令牌: %s", token)
			http.Error(w, "无效的令牌", http.StatusUnauthorized)
			return
		}
	}*/

	// 升级 HTTP 连接为 WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("Nâng cấp WebSocket không thành công: %v", err)
		return
	}

	// 适配为 IConn 接口
	wsConn := NewWebSocketConn(conn, deviceID, isMqttUdp)
	if s.onNewConnection != nil {
		s.onNewConnection(wsConn)
	}

}

func extractDeviceAndClientID(r *http.Request) (string, string) {
	deviceKeys := []string{"Device-Id", "device-id", "DEVICE-ID", "device_id", "Device_Id", "deviceId"}
	clientKeys := []string{"Client-Id", "client-id", "CLIENT-ID", "client_id", "Client_Id", "clientId"}

	headerDeviceID, headerDeviceKey := findHeaderValue(r.Header, deviceKeys)
	queryDeviceID, queryDeviceKey := findQueryValue(r.URL.Query(), deviceKeys)
	headerClientID, headerClientKey := findHeaderValue(r.Header, clientKeys)
	queryClientID, queryClientKey := findQueryValue(r.URL.Query(), clientKeys)

	deviceID := headerDeviceID
	if deviceID == "" {
		deviceID = queryDeviceID
	} else if queryDeviceID != "" && queryDeviceID != headerDeviceID {
		log.Warnf("Nếu device-id khác nhau giữa Header (%s) và tham số URL (%s), thì giá trị trong Header sẽ được ưu tiên.", headerDeviceKey, queryDeviceKey)
	}

	clientID := headerClientID
	if clientID == "" {
		clientID = queryClientID
	} else if queryClientID != "" && queryClientID != headerClientID {
		log.Warnf("Nếu client-id khác nhau giữa Header (%s) và tham số URL (%s), thì giá trị trong Header sẽ được ưu tiên.", headerClientKey, queryClientKey)
	}

	return deviceID, clientID
}

func findHeaderValue(header http.Header, keys []string) (string, string) {
	for _, key := range keys {
		if value := header.Get(key); value != "" {
			return value, key
		}
	}
	return "", ""
}

func findQueryValue(values url.Values, keys []string) (string, string) {
	for _, key := range keys {
		if value := values.Get(key); value != "" {
			return value, key
		}
	}
	return "", ""
}

func (s *WebSocketServer) handleInjectMsg(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.onInjectMessage == nil {
		http.Error(w, "inject message handler unavailable", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		DeviceID   string `json:"device_id"`
		Message    string `json:"message"`
		SkipLlm    bool   `json:"skip_llm"`
		AutoListen *bool  `json:"auto_listen"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" {
		http.Error(w, "device_id is required", http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}
	autoListen := true
	if req.AutoListen != nil {
		autoListen = *req.AutoListen
	}
	if err := s.onInjectMessage(req.DeviceID, req.Message, req.SkipLlm, autoListen); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"device_id":   req.DeviceID,
		"message":     req.Message,
		"skip_llm":    req.SkipLlm,
		"auto_listen": autoListen,
	})
}
