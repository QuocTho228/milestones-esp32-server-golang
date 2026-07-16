package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"milestones-esp32-server-golang/internal/app/mqtt_server"
	"milestones-esp32-server-golang/internal/app/server/chat"
	"milestones-esp32-server-golang/internal/app/server/mqtt_udp"
	"milestones-esp32-server-golang/internal/app/server/types"
	"milestones-esp32-server-golang/internal/app/server/websocket"
	"milestones-esp32-server-golang/internal/data/history"
	user_config "milestones-esp32-server-golang/internal/domain/config"
	config_types "milestones-esp32-server-golang/internal/domain/config/types"
	"milestones-esp32-server-golang/internal/domain/mcp"
	"milestones-esp32-server-golang/internal/domain/openclaw"
	"milestones-esp32-server-golang/internal/pool"
	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/spf13/viper"
)

// App quản lý thống nhất tất cả các dịch vụ giao thức và ChatManager

type App struct {
	wsServer       *websocket.WebSocketServer
	mqttUdpAdapter *mqtt_udp.MqttUdpAdapter
	mqttUdpMu      sync.RWMutex

	// Quản lý ChatManager - sử dụng concurrent map
	chatManagers cmap.ConcurrentMap[string, *chat.ChatManager]
}

func NewApp() *App {
	var err error
	app := &App{
		chatManagers: cmap.New[*chat.ChatManager](),
	}
	mcp.RegisterCurrentDeviceTransportResolver(func(deviceID string) string {
		chatManager, exists := app.GetChatManager(deviceID)
		if !exists || chatManager == nil {
			return ""
		}
		return chatManager.GetTransportType()
	})
	app.wsServer = app.newWebSocketServer()
	app.mqttUdpAdapter, err = app.newMqttUdpAdapter()
	if err != nil {
		log.Errorf("newMqttUdpAdapter err: %+v", err)
		return nil
	}
	return app
}

func (a *App) Run() {
	go a.wsServer.Start()
	log.Infof("enter Run, mqtt_server.enable: %v", viper.GetBool("mqtt_server.enable"))
	if viper.GetBool("mqtt_server.enable") {
		go func() {
			err := a.startMqttServer()
			if err != nil {
				log.Errorf("startMqttServer err: %+v", err)
			}
		}()
	}
	a.mqttUdpMu.RLock()
	adapter := a.mqttUdpAdapter
	a.mqttUdpMu.RUnlock()
	if adapter != nil {
		go adapter.Start() // Không chặn luồng chính (non-blocking), việc kết nối và thử lại được thực hiện ở background bên trong adapter
	}

	// Đăng ký các công cụ MCP cục bộ liên quan đến chat
	a.registerChatMCPTools()

	a.registerHandler()

	a.initEventHandle()

	// Khởi động giám sát thống kê resource pool (xuất log mỗi 5 phút một lần)
	ctx := context.Background()
	pool.StartStatsMonitor(ctx, 5*time.Minute)

	// Khởi động báo cáo thống kê resource pool (báo cáo lên manager backend mỗi 5 giây một lần)
	pool.StartStatsReporter(ctx)

	select {} // Chặn (block) luồng chính
}

func (app *App) initEventHandle() {
	eventHandle, err := NewEventHandle(app)
	if err != nil {
		log.Errorf("Khởi tạo EventHandle thất bại: %v", err)
		return
	}
	if err := eventHandle.Start(); err != nil {
		log.Errorf("Không khởi động được EventHandle: %v", err)
		return
	}

	// Khởi tạo trình xử lý tin nhắn (luôn được bật, xử lý thống nhất Redis+MemoryProvider+History)
	historyCfg := history.HistoryClientConfig{
		BaseURL:   util.GetBackendURL(),
		AuthToken: util.GetManagerAuthToken(),
		Timeout:   viper.GetDuration("manager.history_timeout"),
		Enabled:   true, // Luôn được bật
	}
	NewMessageWorker(historyCfg)
	log.Info("Đã khởi tạo trình xử lý tin nhắn")
}

func (app *App) currentMqttConfig() *mqtt_udp.MqttConfig {
	if !viper.GetBool("mqtt.enable") {
		return nil
	}
	return &mqtt_udp.MqttConfig{
		Broker:   viper.GetString("mqtt.broker"),
		Type:     viper.GetString("mqtt.type"),
		Port:     viper.GetInt("mqtt.port"),
		ClientID: viper.GetString("mqtt.client_id"),
		Username: viper.GetString("mqtt.username"),
		Password: viper.GetString("mqtt.password"),
	}
}

func (app *App) newMqttUdpAdapter() (*mqtt_udp.MqttUdpAdapter, error) {
	mqttConfig := app.currentMqttConfig()
	if mqttConfig == nil {
		return nil, nil
	}

	udpServer, err := app.newUdpServer()
	if err != nil {
		return nil, err
	}

	return mqtt_udp.NewMqttUdpAdapter(
		mqttConfig,
		mqtt_udp.WithUdpServer(udpServer),
		mqtt_udp.WithOnNewConnection(app.OnNewConnection),
		mqtt_udp.WithOnDeviceOnline(app.DeviceOnline),
		mqtt_udp.WithOnDeviceOffline(app.DeviceOffline),
		mqtt_udp.WithOnTransportReady(app.onMqttTransportReady),
		mqtt_udp.WithOfflineGracePeriod(app.mqttOfflineGracePeriod()),
	), nil
}

func (app *App) mqttOfflineGracePeriod() time.Duration {
	if duration := viper.GetDuration("mqtt.transport_offline_grace_period"); duration > 0 {
		return duration
	}
	if seconds := viper.GetInt("mqtt.transport_offline_grace_period_seconds"); seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return 2 * time.Minute
}

func (app *App) newUdpServer() (*mqtt_udp.UdpServer, error) {
	udpPort := viper.GetInt("udp.listen_port")
	externalHost := viper.GetString("udp.external_host")
	externalPort := viper.GetInt("udp.external_port")

	udpServer := mqtt_udp.NewUDPServer(udpPort, externalHost, externalPort)
	err := udpServer.Start()
	if err != nil {
		log.Fatalf("udpServer.Start err: %+v", err)
		return nil, err
	}
	return udpServer, nil
}

func (app *App) newWebSocketServer() *websocket.WebSocketServer {
	port := viper.GetInt("websocket.port")
	return websocket.NewWebSocketServer(
		port,
		websocket.WithOnNewConnection(app.OnNewConnection),
		websocket.WithOnOpenClawResponse(app.OnOpenClawResponse),
		websocket.WithOnInjectMessage(func(deviceID, message string, skipLlm bool, autoListen bool) error {
			chatManager, exists := app.GetChatManager(deviceID)
			if !exists || chatManager == nil {
				return fmt.Errorf("device %s not found or offline", deviceID)
			}
			return chatManager.InjectMessage(message, skipLlm, autoListen)
		}),
	)
}

func (app *App) startMqttServer() error {
	return mqtt_server.StartMqttServer()
}

// ReloadMqttServer Cập nhật nóng (hot reload) MQTT Server: dừng trước, sau đó dựa vào mqtt_server.enable để quyết định có khởi động lại hay không (nếu chưa bật thì chỉ dừng mà không khởi động)
func (app *App) ReloadMqttServer() {
	_ = mqtt_server.StopMqttServer()
	if !viper.GetBool("mqtt_server.enable") {
		return
	}
	if err := app.startMqttServer(); err != nil {
		log.Errorf("ReloadMqttServer start: %v", err)
	}
}

// ReloadMqttUdp Cập nhật nóng MQTT+UDP: dừng adapter cũ trước, sau đó dựa vào mqtt.enable để quyết định có tạo mới và khởi động hay không (nếu chưa bật thì chỉ dừng mà không khởi động)
func (app *App) ReloadMqttUdp() {
	app.mqttUdpMu.Lock()
	old := app.mqttUdpAdapter
	app.mqttUdpAdapter = nil
	app.mqttUdpMu.Unlock()
	if old != nil {
		old.Stop()
	}
	if !viper.GetBool("mqtt.enable") {
		return
	}
	adapter, err := app.newMqttUdpAdapter()
	if err != nil {
		log.Errorf("ReloadMqttUdp newMqttUdpAdapter: %v", err)
		return
	}
	app.mqttUdpMu.Lock()
	app.mqttUdpAdapter = adapter
	app.mqttUdpMu.Unlock()
	time.Sleep(500 * time.Millisecond)
	go adapter.Start()
}

// ReloadMqttUdpWithFlags Dựa vào các cờ thay đổi để quyết định có cập nhật nóng MQTT+UDP hay không
func (app *App) ReloadMqttUdpWithFlags(doMqttReload, doUdpReload bool) {
	if !doMqttReload && !doUdpReload {
		return
	}
	if !viper.GetBool("mqtt.enable") {
		log.Infof("ReloadMqttUdpWithFlags: mqtt disabled, stopping mqtt+udp")
		app.ReloadMqttUdp()
		return
	}

	app.mqttUdpMu.RLock()
	adapter := app.mqttUdpAdapter
	app.mqttUdpMu.RUnlock()

	if adapter == nil {
		log.Infof("ReloadMqttUdpWithFlags: mqtt enabled but adapter is nil, starting mqtt+udp")
		newAdapter, err := app.newMqttUdpAdapter()
		if err != nil {
			log.Errorf("ReloadMqttUdpWithFlags newMqttUdpAdapter: %v", err)
			return
		}
		if newAdapter == nil {
			return
		}
		app.mqttUdpMu.Lock()
		app.mqttUdpAdapter = newAdapter
		app.mqttUdpMu.Unlock()
		time.Sleep(500 * time.Millisecond)
		go newAdapter.Start()
		return
	}

	if doMqttReload && doUdpReload {
		log.Infof("ReloadMqttUdpWithFlags: mqtt & udp config changed, reloading mqtt+udp")
		app.ReloadMqttUdp()
		return
	}
	if doMqttReload {
		log.Infof("ReloadMqttUdpWithFlags: mqtt config changed, reloading mqtt only")
		mqttConfig := app.currentMqttConfig()
		if mqttConfig == nil {
			app.ReloadMqttUdp()
			return
		}
		adapter.ReloadMqttClient(mqttConfig)
		return
	}
	if doUdpReload {
		log.Infof("ReloadMqttUdpWithFlags: udp listen changed, reloading udp only")
		udpServer, err := app.newUdpServer()
		if err != nil {
			log.Errorf("ReloadMqttUdpWithFlags newUdpServer: %v", err)
			return
		}
		adapter.ReloadUdpServer(udpServer)
	}
}

// ReloadMCP Cập nhật nóng MCP: nếu bị vô hiệu hóa thì chỉ dừng MCP toàn cục; nếu bật và đã khởi động rồi thì khởi động lại MCP toàn cục, nếu chưa khởi động thì khởi động cụm MCP
func (app *App) ReloadMCP() error {
	if !viper.GetBool("mcp.global.enabled") {
		// Vô hiệu hóa: chỉ dừng chứ không khởi động, để tránh phụ thuộc vào logic bên trong Start() hoặc vấn đề thứ tự khi hợp nhất
		if err := mcp.GetGlobalMCPManager().Stop(); err != nil {
			return err
		}
		return nil
	}
	mgr := mcp.GetMCPManager()
	if mgr.IsStarted() {
		if err := mgr.RestartManager("global"); err != nil {
			return err
		}
		return nil
	}
	if err := mcp.StartMCPManagers(); err != nil {
		return err
	}
	return nil
}

// Tất cả các kết nối mới của mọi giao thức đều đi qua đây
func (a *App) OnNewConnection(transport types.IConn) {
	deviceID := transport.GetDeviceID()
	transportType := transport.GetTransportType()
	notifyLifecycleOnManager := transportType != types.TransportTypeMqttUdp

	// Kiểm tra xem ChatManager của thiết bị này đã tồn tại hay chưa
	if existingManager, exists := a.chatManagers.Get(deviceID); exists {
		log.Infof("Thiết bị %s đã tồn tại trong chatManager, hãy đóng kết nối cũ trước.", deviceID)
		// Đóng ChatManager cũ
		existingManager.Close()
		a.chatManagers.Remove(deviceID)
	}

	// Tạo ChatManager mới
	chatManager, err := chat.NewChatManager(deviceID, transport)
	if err != nil {
		log.Errorf("Tạo chatManager thất bại: %v", err)
		return
	}

	// Lưu trữ ChatManager
	a.chatManagers.Set(deviceID, chatManager)

	if notifyLifecycleOnManager {
		a.DeviceOnline(deviceID)
	}

	log.Infof("Ứng dụng chatManager cho thiết bị %s đã được tạo và lưu trữ.", deviceID)

	// Gửi lại tin nhắn ngoại tuyến OpenClaw (thử lại có độ trễ, để tránh trường hợp phiên chưa được khởi tạo ngay khi kết nối vừa thiết lập)
	go a.replayOpenClawOfflineMessages(deviceID)

	// Khởi động ChatManager
	go func() {
		defer func() {
			// Khi ChatManager kết thúc, gỡ khỏi bảng ánh xạ
			if storedManager, exists := a.chatManagers.Get(deviceID); exists && storedManager == chatManager {
				a.chatManagers.Remove(deviceID)
				log.Infof("chatManager dành cho thiết bị %s đã bị xóa khỏi danh sách ánh xạ.", deviceID)
				if notifyLifecycleOnManager {
					a.DeviceOffline(deviceID)
				}
			}
		}()

		if err := chatManager.Start(); err != nil {
			log.Errorf("chatManager không khởi động được: %v", err)
		}
	}()
}

func (a *App) onMqttTransportReady(deviceID string) {
	chatManager, exists := a.GetChatManager(deviceID)
	if !exists || chatManager == nil {
		return
	}
	chatManager.HandleMqttTransportReady()
	chatManager.WarmupMcp()
}

// OnOpenClawResponse Callback gửi phản hồi thời gian thực (real-time) của OpenClaw
func (a *App) OnOpenClawResponse(event openclaw.ResponseDelivery) bool {
	deviceID := strings.TrimSpace(event.DeviceID)
	if deviceID == "" {
		return false
	}
	chatManager, exists := a.GetChatManager(deviceID)
	if !exists || chatManager == nil {
		return false
	}
	if err := chatManager.InjectOpenClawResponse(event); err != nil {
		log.Warnf(
			"Chèn tin nhắn thời gian thực vào OpenClaw đã thất bại, device=%s correlation_id=%s start=%v end=%v err=%v",
			deviceID,
			strings.TrimSpace(event.CorrelationID),
			event.IsStart,
			event.IsEnd,
			err,
		)
		return false
	}
	return true
}

func (a *App) replayOpenClawOfflineMessages(deviceID string) {
	manager := openclaw.GetManager()
	const maxRetry = 10
	for i := 0; i < maxRetry; i++ {
		time.Sleep(1 * time.Second)
		delivered, remaining := manager.ReplayOfflineMessages(deviceID, func(msg openclaw.OfflineMessage) error {
			chatManager, exists := a.GetChatManager(deviceID)
			if !exists || chatManager == nil {
				return fmt.Errorf("chat manager not ready")
			}
			if strings.TrimSpace(msg.Text) == "" {
				return nil
			}
			return chatManager.InjectMessage(msg.Text, true, false)
		})
		if delivered > 0 {
			log.Infof("Đã gửi lại tin nhắn ngoại tuyến OpenClaw thành công, device=%s delivered=%d remaining=%d", deviceID, delivered, remaining)
		}
		if remaining == 0 {
			return
		}
	}
}

// GetChatManager lấy ChatManager của thiết bị được chỉ định
func (a *App) GetChatManager(deviceID string) (*chat.ChatManager, bool) {
	return a.chatManagers.Get(deviceID)
}

// CloseChatManager đóng ChatManager của thiết bị được chỉ định
func (a *App) CloseChatManager(deviceID string) bool {
	if manager, exists := a.chatManagers.Get(deviceID); exists {
		manager.Close()
		a.chatManagers.Remove(deviceID)
		log.Infof("Ứng dụng chatManager trên thiết bị %s đã bị tắt và gỡ bỏ.", deviceID)
		return true
	}
	return false
}

// GetAllChatManagers lấy bản sao của tất cả ChatManager
func (a *App) GetAllChatManagers() map[string]*chat.ChatManager {
	// Trả về bản sao để tránh vấn đề truy cập đồng thời (concurrency)
	managers := make(map[string]*chat.ChatManager)
	for tuple := range a.chatManagers.IterBuffered() {
		managers[tuple.Key] = tuple.Val
	}
	return managers
}

// GetChatManagerCount lấy số lượng ChatManager đang hoạt động hiện tại
func (a *App) GetChatManagerCount() int {
	return a.chatManagers.Count()
}

// CloseAllChatManagers đóng tất cả ChatManager
func (a *App) CloseAllChatManagers() {
	for tuple := range a.chatManagers.IterBuffered() {
		tuple.Val.Close()
		log.Infof("Ứng dụng chatManager trên thiết bị %s đã đóng.", tuple.Key)
	}

	// Xóa toàn bộ bảng ánh xạ
	a.chatManagers.Clear()
	log.Info("Tất cả các ChatManager đã đóng")
}

// registerChatMCPTools đăng ký các công cụ MCP cục bộ liên quan đến chat
func (s *App) registerChatMCPTools() {
	// Gọi hàm đăng ký của gói chat
	chat.RegisterChatMCPTools()

	log.Info("Các công cụ MCP cục bộ liên quan đến trò chuyện đã được đăng ký.")
}

func (s *App) DeviceOnline(deviceID string) {
	eventData := map[string]interface{}{
		"device_id": deviceID,
	}
	providerType := viper.GetString("config_provider.type")
	provider, err := user_config.GetProvider(providerType)
	if err != nil {
		log.Errorf("GetProvider err: %+v", err)
		return
	}
	provider.NotifyDeviceEvent(context.Background(), config_types.EventDeviceOnline, eventData)
}

func (s *App) DeviceOffline(deviceID string) {
	eventData := map[string]interface{}{
		"device_id": deviceID,
	}
	providerType := viper.GetString("config_provider.type")
	provider, err := user_config.GetProvider(providerType)
	if err != nil {
		log.Errorf("GetProvider err: %+v", err)
		return
	}
	provider.NotifyDeviceEvent(context.Background(), config_types.EventDeviceOffline, eventData)
}

func (a *App) registerHandler() {
	providerType := viper.GetString("config_provider.type")
	log.Infof("registerHandler: config_provider.type=%s", providerType)
	provider, err := user_config.GetProvider(providerType)
	if err != nil {
		log.Errorf("GetProvider err: %+v", err)
		return
	}
	provider.RegisterMessageEventHandler(context.Background(), config_types.EventHandleMessageInject, a.HandleInjectMsg)
	log.Infof("registerHandler: registered paths=[%s]", config_types.EventHandleMessageInject)
}

// HandleInjectMsg tiêm (inject) tin nhắn vào phía client
func (a *App) HandleInjectMsg(ctx context.Context, eventType string, eventData map[string]interface{}) (string, error) {
	type InjectMsg struct {
		SkipLlm    bool   `json:"skip_llm"`
		AutoListen *bool  `json:"auto_listen"`
		DeviceId   string `json:"device_id"`
		Message    string `json:"message"`
	}
	bodyBytes, _ := json.Marshal(eventData)
	var msg InjectMsg
	err := json.Unmarshal(bodyBytes, &msg)
	if err != nil {
		log.Errorf("HandleInjectMsg error: %+v", err)
		return "", fmt.Errorf("HandleInjectMsg error")
	}

	// Kiểm tra các tham số bắt buộc
	if msg.DeviceId == "" {
		log.Errorf("HandleInjectMsg: device_id is required")
		return "", fmt.Errorf("device_id is required")
	}
	if msg.Message == "" {
		log.Errorf("HandleInjectMsg: message is required")
		return "", fmt.Errorf("message is required")
	}

	// Lấy ChatManager của thiết bị được chỉ định
	chatManager, exists := a.GetChatManager(msg.DeviceId)
	if !exists {
		log.Errorf("HandleInjectMsg: device %s not found or offline", msg.DeviceId)
		return "", fmt.Errorf("device %s not found or offline", msg.DeviceId)
	}

	autoListen := true
	if msg.AutoListen != nil {
		autoListen = *msg.AutoListen
	}

	log.Debugf("HandleInjectMsg: injecting message to device %s, skip_llm: %v, auto_listen: %v, message: %s",
		msg.DeviceId, msg.SkipLlm, autoListen, msg.Message)

	// Sử dụng phương thức công khai (public method) của ChatManager để tiêm tin nhắn
	err = chatManager.InjectMessage(msg.Message, msg.SkipLlm, autoListen)
	if err != nil {
		log.Errorf("HandleInjectMsg: failed to inject message to device %s: %v", msg.DeviceId, err)
		return "", fmt.Errorf("failed to inject message: %v", err)
	}

	return "message injected successfully", nil
}