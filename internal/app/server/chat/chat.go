package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/viper"

	"milestones-esp32-server-golang/constants"
	"milestones-esp32-server-golang/internal/app/server/auth"
	"milestones-esp32-server-golang/internal/app/server/chat/plugins"
	types_conn "milestones-esp32-server-golang/internal/app/server/types"
	types_audio "milestones-esp32-server-golang/internal/data/audio"
	. "milestones-esp32-server-golang/internal/data/client"
	. "milestones-esp32-server-golang/internal/data/msg"
	chathooks "milestones-esp32-server-golang/internal/domain/chat/hooks"
	"milestones-esp32-server-golang/internal/domain/chat/streamtransform"
	userconfig "milestones-esp32-server-golang/internal/domain/config"
	"milestones-esp32-server-golang/internal/domain/mcp"
	"milestones-esp32-server-golang/internal/domain/openclaw"
	pkghooks "milestones-esp32-server-golang/internal/pkg/hooks"
	log "milestones-esp32-server-golang/logger"
)

type ChatManager struct {
	DeviceID  string
	transport types_conn.IConn

	clientState       *ClientState
	serverTransport   *ServerTransport
	mcpTransport      *McpTransport
	hookHub           *chathooks.Hub
	transformRegistry *streamtransform.Registry

	sessionMu sync.RWMutex
	session   *ChatSession

	startingSession     *ChatSession
	startingSessionDone chan struct{}

	ctx    context.Context
	cancel context.CancelFunc

	helloMu      sync.Mutex
	helloInited  bool
	mcpInitState chatMcpInitState

	speakRequestMu      sync.Mutex
	pendingSpeakRequest *pendingSpeakRequest
	lastSpeakPathWarmAt atomic.Int64
	speakReadyTimeout   time.Duration

	// Bảo vệ Close, tránh đóng nhiều lần
	closeOnce      sync.Once
	managerClosing atomic.Bool
	needFreshHello bool

	mqttRebootstrapPending atomic.Bool

	retainedSessionCleanupMu     sync.Mutex
	retainedSessionCleanupTimer  *time.Timer
	retainedSessionCleanupTarget *ChatSession
	retainedSessionIdleTimeout   time.Duration
}

type pendingSpeakRequest struct {
	sessionID string
	done      chan struct{}
	timer     *time.Timer

	once sync.Once
	mu   sync.Mutex
	err  error
}

func (p *pendingSpeakRequest) resolve(err error) {
	if p == nil {
		return
	}
	p.once.Do(func() {
		if p.timer != nil {
			p.timer.Stop()
		}
		p.mu.Lock()
		p.err = err
		p.mu.Unlock()
		close(p.done)
	})
}

func (p *pendingSpeakRequest) Err() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

type chatMcpInitState uint8

const (
	chatMcpInitStateIdle chatMcpInitState = iota
	chatMcpInitStateInFlight
	chatMcpInitStateReady
)

const (
	defaultSpeakRequestReuseWindow = 60 * time.Second
	defaultSpeakReadyTimeout       = 5 * time.Second
	defaultRetainedSessionIdleTTL  = 10 * time.Minute
)

type brokerOnlineAwareTransport interface {
	IsBrokerOnline() bool
}

type ChatManagerOption func(*ChatManager)

var (
	chatHookAsyncExecutorOnce sync.Once
	chatHookAsyncExecutor     *pkghooks.AsyncExecutor
)

func sharedChatHookAsyncExecutor() *pkghooks.AsyncExecutor {
	chatHookAsyncExecutorOnce.Do(func() {
		asyncCfg := pkghooks.AsyncConfig{
			QueueSize:    viper.GetInt("chat_hooks.async.queue_size"),
			WorkerCount:  viper.GetInt("chat_hooks.async.worker_count"),
			DropWhenFull: viper.GetBool("chat_hooks.async.drop_when_full"),
			Timeout:      time.Duration(viper.GetInt("chat_hooks.async.timeout_ms")) * time.Millisecond,
		}
		chatHookAsyncExecutor = pkghooks.NewAsyncExecutor(context.Background(), asyncCfg)
		log.Infof("Khởi tạo executor observer hook chat dùng chung toàn cục: queue_size=%d worker_count=%d drop_when_full=%v timeout=%s", asyncCfg.QueueSize, asyncCfg.WorkerCount, asyncCfg.DropWhenFull, asyncCfg.Timeout)
	})
	return chatHookAsyncExecutor
}

func newChatHookHub(parent context.Context) *chathooks.Hub {
	asyncCfg := pkghooks.AsyncConfig{
		QueueSize:    viper.GetInt("chat_hooks.async.queue_size"),
		WorkerCount:  viper.GetInt("chat_hooks.async.worker_count"),
		DropWhenFull: viper.GetBool("chat_hooks.async.drop_when_full"),
		Timeout:      time.Duration(viper.GetInt("chat_hooks.async.timeout_ms")) * time.Millisecond,
	}
	hub := chathooks.NewHub(parent, pkghooks.WithAsyncConfig(asyncCfg), pkghooks.WithAsyncExecutor(sharedChatHookAsyncExecutor()))
	stats := hub.Stats()
	log.Infof("Khởi tạo chat hook hub: queue_size=%d worker_count=%d drop_when_full=%v timeout=%s dropped_async=%d", asyncCfg.QueueSize, asyncCfg.WorkerCount, asyncCfg.DropWhenFull, asyncCfg.Timeout, stats.DroppedAsync)
	return hub
}

func chatHookBuiltinOverrides() map[string]chathooks.BuiltinPluginConfig {
	overrides := map[string]chathooks.BuiltinPluginConfig{}
	for _, reg := range chathooks.BuiltinRegistrations() {
		path := "chat_hooks.plugins." + reg.Meta.Name
		cfg := chathooks.BuiltinPluginConfig{}
		if viper.IsSet(path + ".enabled") {
			enabled := viper.GetBool(path + ".enabled")
			cfg.Enabled = &enabled
		}
		if viper.IsSet(path + ".priority") {
			cfg.Priority = viper.GetInt(path + ".priority")
		}
		overrides[reg.Meta.Name] = cfg
	}
	return overrides
}

func NewChatManager(deviceID string, transport types_conn.IConn, options ...ChatManagerOption) (*ChatManager, error) {
	cm := &ChatManager{
		DeviceID:          deviceID,
		transport:         transport,
		speakReadyTimeout: defaultSpeakReadyTimeout,
	}

	for _, option := range options {
		option(cm)
	}

	ctx := context.WithValue(context.Background(), "chat_session_operator", ChatSessionOperator(cm))
	ctx = withTTSTurnEndPolicyHandler(ctx, cm)
	cm.ctx, cm.cancel = context.WithCancel(ctx)

	clientState, err := GenClientState(cm.ctx, cm.DeviceID)
	if err != nil {
		log.Errorf("Khởi tạo trạng thái client thất bại: %v", err)
		_ = cm.transport.Close()
		return nil, err
	}
	cm.clientState = clientState

	cm.serverTransport = NewServerTransport(cm.transport, clientState)
	cm.mcpTransport = &McpTransport{
		Client:          clientState,
		ServerTransport: cm.serverTransport,
	}

	cm.transport.OnClose(cm.OnClose)

	cm.hookHub = newChatHookHub(cm.ctx)
	if !viper.IsSet("chat_hooks.enabled") || viper.GetBool("chat_hooks.enabled") {
		if err := chathooks.RegisterBuiltinPlugins(cm.hookHub, chatHookBuiltinOverrides()); err != nil {
			log.Errorf("Đăng ký chat hook builtin plugins thất bại: %v", err)
			_ = cm.transport.Close()
			return nil, err
		}
		log.Infof("Đã tải chat hook plugins: %+v", cm.hookHub.PluginMetas())
	}

	cm.transformRegistry = streamtransform.NewRegistry()
	plugins.Init(cm.transformRegistry)

	return cm, nil
}

func GenClientState(pctx context.Context, deviceID string) (*ClientState, error) {
	configProvider, err := userconfig.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		log.Errorf("Lấy provider cấu hình người dùng thất bại: %+v", err)
		return nil, err
	}
	deviceConfig, err := configProvider.GetUserConfig(pctx, deviceID)
	if err != nil {
		log.Errorf("Lấy cấu hình thiết bị %s thất bại: %+v", deviceID, err)
		return nil, err
	}
	deviceConfig.MemoryMode = NormalizeMemoryMode(deviceConfig.MemoryMode)
	deviceConfig.SpeakerChatMode = NormalizeSpeakerChatMode(deviceConfig.SpeakerChatMode)

	ctx, cancel := context.WithCancel(pctx)

	maxSilenceDuration := viper.GetInt64("chat.chat_max_silence_duration")
	if !viper.IsSet("chat.chat_max_silence_duration") {
		maxSilenceDuration = 400
	}

	isDeviceActivated, err := configProvider.IsDeviceActivated(ctx, deviceID, "")
	if err != nil {
		log.Errorf("Kiểm tra trạng thái kích hoạt thiết bị thất bại: %v", err)
	}

	clientState := &ClientState{
		IsActivated:       isDeviceActivated,
		Dialogue:          &Dialogue{},
		Abort:             false,
		ListenMode:        "auto",
		ListenPhase:       ListenPhaseIdle,
		DeviceID:          deviceID,
		AgentID:           deviceConfig.AgentId,
		Ctx:               ctx,
		Cancel:            cancel,
		SystemPrompt:      deviceConfig.SystemPrompt,
		DeviceConfig:      deviceConfig,
		OutputAudioFormat: types_audio.AudioFormat{},
		OpusAudioBuffer:   make(chan []byte, 100),
		AsrAudioBuffer: &AsrAudioBuffer{
			PcmData:          make([]float32, 0),
			AudioBufferMutex: sync.RWMutex{},
		},
		VoiceStatus: VoiceStatus{
			HaveVoice:            false,
			HaveVoiceLastTime:    0,
			VoiceStop:            false,
			SilenceThresholdTime: maxSilenceDuration,
		},
		SessionCtx: Ctx{},
	}
	applyOutputAudioFormatForTTS(clientState)

	return clientState, nil
}

func applyOutputAudioFormatForTTS(clientState *ClientState) {
	clientState.OutputAudioFormat = types_audio.AudioFormat{
		SampleRate:    types_audio.SampleRate,
		Channels:      types_audio.Channels,
		FrameDuration: types_audio.FrameDuration,
		Format:        types_audio.Format,
	}
	ttsType := clientState.DeviceConfig.Tts.Provider
	if ttsType == constants.TtsTypeMilestones {
		clientState.OutputAudioFormat.SampleRate = 24000
		clientState.OutputAudioFormat.FrameDuration = 20
	}
}

func (c *ChatManager) ReloadDeviceConfig(ctx context.Context) error {
	configProvider, err := userconfig.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		return fmt.Errorf("lấy provider cấu hình thất bại: %w", err)
	}

	deviceConfig, err := configProvider.GetUserConfig(ctx, c.DeviceID)
	if err != nil {
		return fmt.Errorf("lấy cấu hình thiết bị thất bại: %w", err)
	}
	deviceConfig.MemoryMode = NormalizeMemoryMode(deviceConfig.MemoryMode)
	deviceConfig.SpeakerChatMode = NormalizeSpeakerChatMode(deviceConfig.SpeakerChatMode)

	oldAgentID := c.clientState.AgentID
	c.clientState.AgentID = deviceConfig.AgentId
	c.clientState.DeviceConfig = deviceConfig
	c.clientState.SystemPrompt = deviceConfig.SystemPrompt
	c.clientState.SpeakerTTSConfig = nil
	openclaw.GetManager().ExitMode(oldAgentID, c.DeviceID)
	openclaw.GetManager().ExitMode(c.clientState.AgentID, c.DeviceID)
	applyOutputAudioFormatForTTS(c.clientState)
	log.Infof("Cấu hình thiết bị %s đã được làm mới, agent hiện tại=%s", c.DeviceID, deviceConfig.AgentId)
	return nil
}

func (c *ChatManager) Start() error {
	go c.cmdMessageLoop(c.ctx)
	go c.audioMessageLoop(c.ctx)

	<-c.ctx.Done()
	return nil
}

func (c *ChatManager) handleLoopExit(loopName string, ctx context.Context) {
	if r := recover(); r != nil {
		log.Errorf("Thiết bị %s %s loop panic: %v\n%s", c.DeviceID, loopName, r, string(debug.Stack()))
	}
	if ctx == nil || ctx.Err() != nil {
		return
	}
	if c.serverTransport != nil && c.serverTransport.IsClosed() {
		return
	}
	log.Warnf("Thiết bị %s %s loop thoát bất thường, đóng ChatManager", c.DeviceID, loopName)
	if err := c.Close(); err != nil {
		log.Warnf("Đóng ChatManager sau khi loop %s của thiết bị %s thoát thất bại: %v", loopName, c.DeviceID, err)
	}
}

func (c *ChatManager) cmdMessageLoop(ctx context.Context) {
	defer c.handleLoopExit("cmd", ctx)

	recvFailCount := 0
	for {
		select {
		case <-ctx.Done():
			log.Infof("Thiết bị %s recvCmd context cancel", c.DeviceID)
			return
		default:
		}

		if recvFailCount > 3 {
			log.Errorf("Thiết bị %s recv cmd timeout vượt ngưỡng", c.DeviceID)
			return
		}

		message, err := c.serverTransport.RecvCmd(ctx, 120)
		if err != nil {
			log.Errorf("recv cmd error: %v", err)
			recvFailCount++
			continue
		}
		if message == nil {
			continue
		}

		recvFailCount = 0
		log.Infof("Nhận được tin nhắn văn bản: %s", string(message))
		if err := c.handleTextMessage(message); err != nil {
			log.Errorf("Xử lý tin nhắn văn bản thất bại: %v, nội dung tin nhắn: %s", err, string(message))
		}
	}
}

func (c *ChatManager) audioMessageLoop(ctx context.Context) {
	defer c.handleLoopExit("audio", ctx)

	for {
		select {
		case <-ctx.Done():
			log.Debugf("Thiết bị %s recvAudio context cancel", c.DeviceID)
			return
		default:
		}

		message, err := c.serverTransport.RecvAudio(ctx, 600)
		if err != nil {
			log.Errorf("recv audio error: %v", err)
			return
		}
		if message == nil {
			continue
		}

		session := c.GetSession()
		if session == nil {
			log.Debugf("Thiết bị %s hiện không có ChatSession nào đang hoạt động, bỏ dữ liệu âm thanh", c.DeviceID)
			continue
		}
		if c.hasPendingSpeakRequest() {
			log.Debugf("Thiết bị %s hiện có speak_request đang chờ hoàn tất, bỏ dữ liệu âm thanh", c.DeviceID)
			continue
		}

		log.Debugf("Nhận được dữ liệu âm thanh, kích thước: %d byte", len(message))
		isAuth := viper.GetBool("auth.enable")
		if isAuth && !c.clientState.IsActivated {
			log.Debugf("Thiết bị %s chưa được kích hoạt, bỏ qua dữ liệu âm thanh", c.clientState.DeviceID)
			continue
		}
		if c.clientState.GetClientVoiceStop() {
			log.Debug("Client đã dừng nói, bỏ qua dữ liệu âm thanh")
			continue
		}

		if ok := session.HandleAudioMessage(message); !ok {
			log.Warnf("Buffer âm thanh đã đầy, bỏ dữ liệu âm thanh")
		}
	}
}

func (c *ChatManager) handleTextMessage(message []byte) error {
	var clientMsg ClientMessage
	if err := json.Unmarshal(message, &clientMsg); err != nil {
		log.Errorf("Phân tích tin nhắn thất bại: %v", err)
		return fmt.Errorf("phân tích tin nhắn thất bại: %v", err)
	}

	if clientMsg.Type != MessageTypeGoodBye {
		c.cancelRetainedSessionCleanup("nhận được tin nhắn hoạt động từ thiết bị")
	}

	switch clientMsg.Type {
	case MessageTypeHello:
		return c.HandleHelloMessage(&clientMsg)
	case MessageTypeSpeakReady:
		return c.HandleSpeakReadyMessage(&clientMsg)
	case MessageTypeListen:
		return c.HandleListenMessage(&clientMsg)
	case MessageTypeAbort:
		return c.HandleAbortMessage(&clientMsg)
	case MessageTypeIot:
		return c.HandleIoTMessage(&clientMsg)
	case MessageTypeMcp:
		return c.HandleMcpMessage(&clientMsg)
	case MessageTypeGoodBye:
		return c.HandleGoodByeMessage(&clientMsg)
	default:
		return fmt.Errorf("loại tin nhắn không xác định: %s", clientMsg.Type)
	}
}

func (c *ChatManager) HandleHelloMessage(msg *ClientMessage) error {
	if msg.AudioParams == nil {
		return fmt.Errorf("tin nhắn hello thiếu audio_params")
	}
	transportType := strings.TrimSpace(msg.Transport)
	if transportType == "" && c.serverTransport != nil {
		transportType = c.serverTransport.GetTransportType()
	}
	switch transportType {
	case types_conn.TransportTypeWebsocket, types_conn.TransportTypeMqttUdp:
	default:
		return fmt.Errorf("loại transport không được hỗ trợ: %s", transportType)
	}

	c.helloMu.Lock()
	defer c.helloMu.Unlock()

	clientState := c.clientState
	clientState.InputAudioFormat = *msg.AudioParams
	isFirstHello := !c.helloInited
	requiresFreshHello := c.requiresFreshHello()
	isDuplicateMqttHello := !isFirstHello && !requiresFreshHello &&
		c.serverTransport != nil && c.serverTransport.GetTransportType() == types_conn.TransportTypeMqttUdp
	if c.helloInited {
		prevAgentID := clientState.AgentID
		if err := c.refreshDeviceConfigOnHello(); err != nil {
			log.Warnf("Thiết bị %s làm mới cấu hình khi duplicate hello thất bại, giảm cấp và tiếp tục: %v", clientState.DeviceID, err)
		}
		c.resetOpenClawModeOnHello(prevAgentID, clientState.AgentID)
	} else {
		c.resetOpenClawModeOnHello(clientState.AgentID)
	}

	if isFirstHello || requiresFreshHello {
		preferredSessionID := ""
		if isFirstHello {
			preferredSessionID = strings.TrimSpace(clientState.SessionID)
		}
		session, err := auth.A().EnsureSession(msg.DeviceID, preferredSessionID)
		if err != nil {
			return fmt.Errorf("tạo phiên thất bại: %v", err)
		}
		clientState.SessionID = session.ID
		c.helloInited = true
	}
	if isDuplicateMqttHello {
		log.Infof("Thiết bị %s nhận được duplicate_hello, thực hiện đàm phán lại hello", clientState.DeviceID)
		c.markMqttConversationStateStale("duplicate_hello")
	}

	chatSession, err := c.ensureSessionForHello()
	if err != nil {
		if isFirstHello || requiresFreshHello {
			c.setNeedFreshHello(true)
		}
		return err
	}
	if err := c.sendHelloResponse(msg); err != nil {
		if isFirstHello || requiresFreshHello {
			c.setNeedFreshHello(true)
			if chatSession != nil {
				chatSession.CloseWithReason(chatSessionCloseReasonFatalError)
			}
		}
		return err
	}
	c.refreshSpeakPathWarmFromTransport()
	c.scheduleMcpInitOnHelloLocked(msg)
	if !isFirstHello && !requiresFreshHello {
		log.Infof("Thiết bị %s xử lý duplicate_hello hoàn tất, đàm phán lại hello đã được làm mới", clientState.DeviceID)
	}
	return nil
}

func (c *ChatManager) scheduleMcpInitOnHelloLocked(msg *ClientMessage) {
	if !c.hasMcpFeature(msg) {
		return
	}
	c.scheduleMcpInitLocked()
}

func (c *ChatManager) scheduleMcpInitLocked() {
	if c.mcpTransport == nil {
		return
	}
	if c.mcpInitState == chatMcpInitStateInFlight {
		return
	}
	if !shouldScheduleDeviceMcpRuntimeInit(c.clientState.DeviceID, c.mcpTransport) {
		return
	}
	if c.mcpInitState == chatMcpInitStateReady {
		log.Warnf(
			"Thiết bị %s bị lệch trạng thái MCP: ChatManager đã ready, nhưng transport=%s cần khởi tạo lại",
			c.DeviceID,
			strings.TrimSpace(c.mcpTransport.GetMcpTransportType()),
		)
	}

	c.mcpInitState = chatMcpInitStateInFlight
	deviceID := c.clientState.DeviceID
	transportType := strings.TrimSpace(c.mcpTransport.GetMcpTransportType())
	go func() {
		err := initMcp(deviceID, c.mcpTransport)
		c.finishMcpInit(transportType, err)
	}()
}

func (c *ChatManager) finishMcpInit(transportType string, err error) {
	c.helloMu.Lock()
	defer c.helloMu.Unlock()

	if c.ctx.Err() != nil || c.managerClosing.Load() {
		return
	}
	if c.mcpTransport == nil {
		c.mcpInitState = chatMcpInitStateIdle
		return
	}
	currentTransportType := strings.TrimSpace(c.mcpTransport.GetMcpTransportType())
	if currentTransportType != strings.TrimSpace(transportType) {
		return
	}

	if err != nil {
		c.mcpInitState = chatMcpInitStateIdle
		log.Warnf("Thiết bị %s khởi tạo MCP thất bại, chờ retry ở hello tiếp theo: %v", c.DeviceID, err)
		return
	}

	c.mcpInitState = chatMcpInitStateReady
}

func (c *ChatManager) hasMcpFeature(msg *ClientMessage) bool {
	if msg == nil || msg.Features == nil {
		return false
	}
	isMcp, ok := msg.Features["mcp"]
	return ok && isMcp
}

func (c *ChatManager) sendHelloResponse(msg *ClientMessage) error {
	transportType := strings.TrimSpace(msg.Transport)
	if transportType == "" {
		transportType = c.serverTransport.GetTransportType()
	}

	switch transportType {
	case types_conn.TransportTypeWebsocket:
		return c.serverTransport.SendHello(types_conn.TransportTypeWebsocket, &c.clientState.OutputAudioFormat, nil)
	case types_conn.TransportTypeMqttUdp:
		udpConfig, err := c.buildMqttHelloUdpConfig()
		if err != nil {
			return err
		}
		return c.serverTransport.SendHello(types_conn.TransportTypeMqttUdp, &c.clientState.OutputAudioFormat, udpConfig)
	default:
		return fmt.Errorf("loại transport không được hỗ trợ: %s", transportType)
	}
}

func (c *ChatManager) buildMqttHelloUdpConfig() (*UdpConfig, error) {
	udpExternalHost := viper.GetString("udp.external_host")
	udpExternalPort := viper.GetInt("udp.external_port")

	aesKey, err := c.serverTransport.GetData("aes_key")
	if err != nil {
		return nil, fmt.Errorf("lấy aes_key thất bại: %v", err)
	}
	fullNonce, err := c.serverTransport.GetData("full_nonce")
	if err != nil {
		return nil, fmt.Errorf("lấy full_nonce thất bại: %v", err)
	}

	strAesKey, ok := aesKey.(string)
	if !ok {
		return nil, fmt.Errorf("aes_key không phải là chuỗi")
	}
	strFullNonce, ok := fullNonce.(string)
	if !ok {
		return nil, fmt.Errorf("full_nonce không phải là chuỗi")
	}

	return &UdpConfig{
		Server: udpExternalHost,
		Port:   udpExternalPort,
		Key:    strAesKey,
		Nonce:  strFullNonce,
	}, nil
}

func (c *ChatManager) HandleListenMessage(msg *ClientMessage) error {
	if c.requiresHelloBootstrapForSession() {
		log.Infof(
			"Thiết bị %s phiên hiện tại đã đóng hoặc chưa hoàn tất hello, bỏ qua listen %s, chờ hello mới",
			c.DeviceID,
			strings.TrimSpace(msg.State),
		)
		return nil
	}
	session, err := c.ensureSession()
	if err != nil {
		return err
	}
	c.clearMqttRebootstrapPending("listen_message")
	return session.HandleListenMessage(msg)
}

func (c *ChatManager) HandleAbortMessage(msg *ClientMessage) error {
	session := c.GetSession()
	if session == nil {
		log.Debugf("Thiết bị %s hiện không có ChatSession nào đang hoạt động, bỏ qua abort", c.DeviceID)
		return nil
	}
	return session.HandleAbortMessage(msg)
}

func (c *ChatManager) HandleIoTMessage(msg *ClientMessage) error {
	if err := c.serverTransport.SendIot(msg); err != nil {
		return fmt.Errorf("gửi phản hồi thất bại: %v", err)
	}
	log.Infof("Thiết bị %s lệnh IoT: %s", msg.DeviceID, msg.Text)
	return nil
}

func (c *ChatManager) HandleMcpMessage(msg *ClientMessage) error {
	return mcp.HandleDeviceIotMcpMessage(c.clientState.DeviceID, c.mcpTransport.GetMcpTransportType(), msg.PayLoad)
}

func (c *ChatManager) HandleGoodByeMessage(msg *ClientMessage) error {
	session := c.GetSession()
	if session != nil {
		log.Infof("Thiết bị %s nhận được goodbye từ phía thiết bị, giữ lại ChatSession và reset về trạng thái im lặng", c.DeviceID)
		session.ResetToSilentState()
		c.scheduleRetainedSessionCleanup(session, "peer_goodbye")
	} else {
		log.Infof("Thiết bị %s nhận được goodbye từ phía thiết bị, nhưng hiện không có ChatSession nào đang hoạt động, chỉ dọn dẹp đường truyền âm thanh", c.DeviceID)
	}

	c.resetSpeakPathAfterGoodbye()
	return c.transport.CloseAudioChannel()
}

func (c *ChatManager) HandleSpeakReadyMessage(msg *ClientMessage) error {
	if msg == nil {
		return nil
	}
	if c.serverTransport == nil || c.serverTransport.GetTransportType() != types_conn.TransportTypeMqttUdp {
		return nil
	}
	if msg.State != "" && msg.State != MessageStateReady {
		log.Debugf("Thiết bị %s trạng thái speak_ready không phải ready, bỏ qua: %+v", c.DeviceID, msg)
		return nil
	}
	if msg.SpeakUDPConfig != nil && !msg.SpeakUDPConfig.Ready {
		log.Warnf("Thiết bị %s speak_ready udp_config.ready=false, bỏ qua", c.DeviceID)
		return nil
	}

	c.speakRequestMu.Lock()
	pending := c.pendingSpeakRequest
	c.speakRequestMu.Unlock()
	if pending == nil {
		log.Debugf("Thiết bị %s nhận được speak_ready không có request đang chờ xử lý, bỏ qua", c.DeviceID)
		return nil
	}
	if pending.sessionID != "" && strings.TrimSpace(msg.SessionID) != pending.sessionID {
		log.Warnf("Thiết bị %s session_id của speak_ready không khớp: got=%s want=%s", c.DeviceID, msg.SessionID, pending.sessionID)
		return nil
	}

	c.markSpeakPathWarm(time.Now())
	c.clearMqttRebootstrapPending("speak_ready")
	c.finishPendingSpeakRequest(pending, nil)

	reuseExisting := false
	if msg.SpeakUDPConfig != nil {
		reuseExisting = msg.SpeakUDPConfig.ReuseExisting
	}
	log.Infof("Thiết bị %s speak_ready đã sẵn sàng, reuse_existing=%v", c.DeviceID, reuseExisting)
	return nil
}

func (c *ChatManager) ensureSession() (*ChatSession, error) {
	return c.ensureSessionInternal(false)
}

func (c *ChatManager) ensureSessionForHello() (*ChatSession, error) {
	return c.ensureSessionInternal(true)
}

func (c *ChatManager) ensureSessionInternal(allowFreshHello bool) (*ChatSession, error) {
	for {
		c.sessionMu.Lock()
		if c.session != nil {
			session := c.session
			c.sessionMu.Unlock()
			if session.IsClosing() {
				return nil, fmt.Errorf("ChatSession đang đóng, vui lòng thử lại sau")
			}
			return session, nil
		}
		if c.startingSession != nil {
			waitCh := c.startingSessionDone
			c.sessionMu.Unlock()
			if waitCh == nil {
				return nil, fmt.Errorf("ChatSession đang khởi động, vui lòng thử lại sau")
			}
			<-waitCh
			continue
		}
		if !c.helloInited {
			c.sessionMu.Unlock()
			return nil, fmt.Errorf("hello chưa được khởi tạo, không thể tạo ChatSession")
		}
		if c.needFreshHello && !allowFreshHello {
			c.sessionMu.Unlock()
			return nil, fmt.Errorf("ChatSession đã thoát, vui lòng gửi lại hello")
		}

		session := NewChatSession(
			c.clientState,
			c.serverTransport,
			c.hookHub,
			c.transformRegistry,
			WithChatSessionCloseHandler(c.handleSessionClosed),
		)
		c.startingSession = session
		c.startingSessionDone = make(chan struct{})
		c.sessionMu.Unlock()

		err := session.Start(c.ctx)
		if err != nil {
			session.CloseWithReason(chatSessionCloseReasonFatalError)
		}
		c.finishSessionStart(session, allowFreshHello, err)
		if err != nil {
			return nil, err
		}
		if session.IsClosing() {
			return nil, fmt.Errorf("ChatSession đang đóng, vui lòng thử lại sau")
		}
		return session, nil
	}
}

func (c *ChatManager) requiresFreshHello() bool {
	c.sessionMu.RLock()
	defer c.sessionMu.RUnlock()
	return c.needFreshHello
}

func (c *ChatManager) setNeedFreshHello(required bool) {
	c.sessionMu.Lock()
	c.needFreshHello = required
	c.sessionMu.Unlock()
}

func (c *ChatManager) needsMqttRebootstrap() bool {
	if c == nil {
		return false
	}
	return c.mqttRebootstrapPending.Load()
}

func (c *ChatManager) clearMqttRebootstrapPending(reason string) {
	if c == nil {
		return
	}
	if c.mqttRebootstrapPending.Swap(false) {
		log.Debugf("Thiết bị %s xóa cờ đánh dấu tái tạo phiên MQTT: reason=%s", c.DeviceID, reason)
	}
}

func (c *ChatManager) hasPendingSpeakRequest() bool {
	if c == nil {
		return false
	}
	c.speakRequestMu.Lock()
	defer c.speakRequestMu.Unlock()
	return c.pendingSpeakRequest != nil
}

func (c *ChatManager) resetSpeakPathAfterMqttRebootstrap(reason string) {
	if c == nil {
		return
	}
	c.lastSpeakPathWarmAt.Store(0)

	c.speakRequestMu.Lock()
	pending := c.pendingSpeakRequest
	c.speakRequestMu.Unlock()
	if pending != nil {
		c.finishPendingSpeakRequest(pending, fmt.Errorf("việc tái tạo đường truyền MQTT khiến đường phát chủ động đã bị reset: %s", reason))
	}
}

func (c *ChatManager) markMqttConversationStateStale(reason string) {
	if c == nil || c.serverTransport == nil || c.serverTransport.GetTransportType() != types_conn.TransportTypeMqttUdp {
		return
	}
	if c.managerClosing.Load() {
		return
	}
	if reason == "duplicate_hello" && c.hasPendingSpeakRequest() {
		log.Infof("Thiết bị %s nhận được duplicate_hello, phát hiện speak_request đang chờ hoàn tất, xử lý theo hướng đàm phán lại bắt tay phát chủ động, bỏ qua việc chuẩn hóa lại tái tạo phiên", c.DeviceID)
		return
	}

	alreadyPending := c.mqttRebootstrapPending.Swap(true)
	if alreadyPending {
		log.Debugf("Thiết bị %s cờ tái tạo phiên MQTT đã tồn tại, bỏ qua việc chuẩn hóa lại trùng lặp: reason=%s", c.DeviceID, reason)
		return
	}

	session := c.GetSession()
	if session != nil && !session.IsClosing() {
		log.Infof("Thiết bị %s đường truyền MQTT tái tạo, reset trạng thái phiên hiện tại: reason=%s", c.DeviceID, reason)
		session.ResetToSilentState()
		c.scheduleRetainedSessionCleanup(session, "mqtt_transport_rebootstrap")
	} else if c.clientState != nil {
		log.Infof("Thiết bị %s đường truyền MQTT tái tạo, dọn dẹp trạng thái client còn sót lại: reason=%s", c.DeviceID, reason)
		c.clientState.Destroy()
		c.clientState.Abort = false
		c.clientState.IsWelcomeSpeaking = false
		c.clientState.IsWelcomePlaying = false
	}

	c.resetSpeakPathAfterMqttRebootstrap(reason)
}

func (c *ChatManager) resetMcpRuntimeOnMqttTransportReady() {
	if c == nil {
		return
	}

	c.helloMu.Lock()
	defer c.helloMu.Unlock()

	c.mcpInitState = chatMcpInitStateIdle
	if c.clientState == nil || c.mcpTransport == nil {
		return
	}

	log.Infof("Thiết bị %s nhận được broadcast online của MQTT, hủy runtime IoT MCP hiện có, chuẩn bị khởi tạo lại", c.DeviceID)
	closeDeviceMcpRuntime(c.clientState.DeviceID, c.mcpTransport)
}

func (c *ChatManager) HandleMqttTransportReady() {
	c.markMqttConversationStateStale("transport_ready")
	c.resetMcpRuntimeOnMqttTransportReady()
}

func (c *ChatManager) resetSpeakPathAfterGoodbye() {
	c.resetSpeakPathAfterSessionReset(fmt.Errorf("goodbye từ phía thiết bị khiến đường phát chủ động đã bị reset"))
}

func (c *ChatManager) resetSpeakPathAfterServerSessionClose(reason string) {
	c.resetSpeakPathAfterSessionReset(fmt.Errorf("việc server đóng phiên khiến đường phát chủ động đã bị reset: %s", reason))
}

func (c *ChatManager) resetSpeakPathAfterSessionReset(err error) {
	if c == nil {
		return
	}
	c.lastSpeakPathWarmAt.Store(0)

	c.speakRequestMu.Lock()
	pending := c.pendingSpeakRequest
	c.speakRequestMu.Unlock()
	if pending != nil {
		c.finishPendingSpeakRequest(pending, err)
	}
}

func (c *ChatManager) getRetainedSessionIdleTimeout() time.Duration {
	if c != nil && c.retainedSessionIdleTimeout > 0 {
		return c.retainedSessionIdleTimeout
	}
	if !viper.IsSet("chat.retained_session_idle_timeout_ms") {
		return defaultRetainedSessionIdleTTL
	}
	ms := viper.GetInt64("chat.retained_session_idle_timeout_ms")
	if ms <= 0 {
		return defaultRetainedSessionIdleTTL
	}
	return time.Duration(ms) * time.Millisecond
}

func (c *ChatManager) cancelRetainedSessionCleanup(reason string) {
	if c == nil {
		return
	}
	c.retainedSessionCleanupMu.Lock()
	timer := c.retainedSessionCleanupTimer
	c.retainedSessionCleanupTimer = nil
	c.retainedSessionCleanupTarget = nil
	c.retainedSessionCleanupMu.Unlock()

	if timer != nil {
		timer.Stop()
		log.Debugf("Thiết bị %s hủy dọn dẹp thời kỳ giữ lại của ChatSession: reason=%s", c.DeviceID, reason)
	}
}

func (c *ChatManager) scheduleRetainedSessionCleanup(session *ChatSession, reason string) {
	if c == nil || session == nil || session.IsClosing() {
		return
	}

	timeout := c.getRetainedSessionIdleTimeout()
	c.cancelRetainedSessionCleanup("reschedule")

	c.retainedSessionCleanupMu.Lock()
	c.retainedSessionCleanupTarget = session
	c.retainedSessionCleanupTimer = time.AfterFunc(timeout, func() {
		c.runRetainedSessionCleanup(session, timeout)
	})
	c.retainedSessionCleanupMu.Unlock()

	log.Infof(
		"Thiết bị %s ChatSession đã vào trạng thái giữ lại: timeout=%s reason=%s",
		c.DeviceID,
		timeout,
		reason,
	)
}

func (c *ChatManager) runRetainedSessionCleanup(session *ChatSession, timeout time.Duration) {
	if c == nil || session == nil {
		return
	}

	c.retainedSessionCleanupMu.Lock()
	if c.retainedSessionCleanupTarget != session {
		c.retainedSessionCleanupMu.Unlock()
		return
	}
	c.retainedSessionCleanupTimer = nil
	c.retainedSessionCleanupTarget = nil
	c.retainedSessionCleanupMu.Unlock()

	c.sessionMu.RLock()
	currentSession := c.session
	c.sessionMu.RUnlock()
	if currentSession != session || session.IsClosing() {
		return
	}

	log.Infof("Thiết bị %s ChatSession nhàn rỗi vượt quá %s, thực hiện dọn dẹp triệt để", c.DeviceID, timeout)
	session.CloseWithReason(chatSessionCloseReasonRetainedIdleTimeout)
}

func (c *ChatManager) finishSessionStart(session *ChatSession, allowFreshHello bool, startErr error) {
	var waitCh chan struct{}

	c.sessionMu.Lock()
	if c.startingSession == session {
		waitCh = c.startingSessionDone
		c.startingSession = nil
		c.startingSessionDone = nil
		if startErr == nil && !session.IsClosing() {
			c.session = session
			if allowFreshHello {
				c.needFreshHello = false
			}
		}
	}
	c.sessionMu.Unlock()

	if waitCh != nil {
		close(waitCh)
	}
}

func (c *ChatManager) handleSessionClosed(session *ChatSession, reason string) {
	var waitCh chan struct{}

	c.cancelRetainedSessionCleanup("session_closed")

	c.sessionMu.Lock()
	switch {
	case c.session == session:
		c.session = nil
	case c.startingSession == session:
		waitCh = c.startingSessionDone
		c.startingSession = nil
		c.startingSessionDone = nil
	default:
		c.sessionMu.Unlock()
		log.Debugf("Thiết bị %s nhận được callback close ChatSession đã hết hạn, bỏ qua việc dọn dẹp tiếp theo", c.DeviceID)
		return
	}
	c.sessionMu.Unlock()

	if waitCh != nil {
		close(waitCh)
		log.Debugf("Thiết bị %s ChatSession đã đóng trong giai đoạn khởi động, đã dọn dẹp trạng thái khởi động", c.DeviceID)
		return
	}

	if reason == chatSessionCloseReasonManagerShutdown {
		// manager_shutdown có thể đến từ việc server chủ động shutdown(true), cũng có thể đến từ việc
		// đường truyền bên dưới đã ngắt kết nối rồi dọn dẹp tài nguyên shutdown(false). Trường hợp
		// shutdown(true) chủ động vẫn do ServerTransport.Close() gửi goodbye, tránh gửi nhầm ở luồng ngắt kết nối bị động.
		return
	}

	if c.serverTransport == nil {
		return
	}

	switch c.serverTransport.GetTransportType() {
	case types_conn.TransportTypeWebsocket:
		if err := c.shutdown(true); err != nil {
			log.Warnf("Đóng websocket transport thất bại: %v", err)
		}
	case types_conn.TransportTypeMqttUdp:
		if reason == chatSessionCloseReasonRetainedIdleTimeout {
			c.setNeedFreshHello(true)
			c.resetSpeakPathAfterServerSessionClose(reason)
			return
		}
		if !shouldSendMqttGoodbyeOnSessionClose(reason) {
			return
		}
		c.setNeedFreshHello(true)
		c.resetSpeakPathAfterServerSessionClose(reason)
		if err := c.serverTransport.SendMqttGoodbye(); err != nil {
			log.Warnf("Gửi mqtt goodbye thất bại: %v", err)
		}
	}
}

func shouldSendMqttGoodbyeOnSessionClose(reason string) bool {
	switch reason {
	case chatSessionCloseReasonExplicitExit,
		chatSessionCloseReasonFatalError,
		chatSessionCloseReasonAudioIdleTimeout:
		return true
	default:
		return false
	}
}

func (c *ChatManager) shutdown(closeTransport bool) error {
	var shutdownErr error

	c.closeOnce.Do(func() {
		c.cancelRetainedSessionCleanup("manager_shutdown")

		if c.clientState != nil {
			log.Infof("Đóng ChatManager, thiết bị %s", c.clientState.DeviceID)
		}

		c.sessionMu.RLock()
		session := c.session
		startingSession := c.startingSession
		c.sessionMu.RUnlock()

		if session != nil {
			session.CloseWithReason(chatSessionCloseReasonManagerShutdown)
		}
		if startingSession != nil && startingSession != session {
			startingSession.CloseWithReason(chatSessionCloseReasonManagerShutdown)
		}

		if c.clientState != nil && c.mcpTransport != nil {
			mcp.CloseDeviceIotOverMcp(c.clientState.DeviceID, c.mcpTransport)
		}

		if c.hookHub != nil {
			c.hookHub.Close()
		}

		if closeTransport {
			c.managerClosing.Store(true)
			defer c.managerClosing.Store(false)

			if c.serverTransport != nil {
				shutdownErr = c.serverTransport.Close()
			} else if c.transport != nil {
				shutdownErr = c.transport.Close()
			}
		} else if c.serverTransport != nil {
			if err := c.serverTransport.CloseWithoutTransport(); err != nil {
				log.Warnf("Đóng lớp bọc server transport thất bại: %v", err)
			}
		}

		if c.cancel != nil {
			c.cancel()
		}
	})

	return shutdownErr
}

func (c *ChatManager) Close() error {
	return c.shutdown(true)
}

func (c *ChatManager) OnClose(deviceId string) {
	log.Infof("Thiết bị %s đã ngắt kết nối", deviceId)
	if c.managerClosing.Load() {
		return
	}
	if err := c.shutdown(false); err != nil {
		log.Warnf("Dọn dẹp tài nguyên sau khi đóng kết nối thất bại: %v", err)
	}
}

func (c *ChatManager) GetClientState() *ClientState {
	return c.clientState
}

func (c *ChatManager) GetDeviceId() string {
	return c.clientState.DeviceID
}

func (c *ChatManager) GetTransportType() string {
	if c == nil || c.serverTransport == nil {
		return ""
	}
	if c.ctx != nil && c.ctx.Err() != nil {
		return ""
	}
	if c.serverTransport.IsClosed() {
		return ""
	}
	if awareTransport, ok := c.transport.(brokerOnlineAwareTransport); ok && !awareTransport.IsBrokerOnline() {
		return ""
	}
	return c.serverTransport.GetTransportType()
}

func (c *ChatManager) WarmupMcp() {
	c.helloMu.Lock()
	defer c.helloMu.Unlock()
	c.scheduleMcpInitLocked()
}

func (c *ChatManager) GetSession() *ChatSession {
	c.sessionMu.RLock()
	defer c.sessionMu.RUnlock()
	return c.session
}

func (c *ChatManager) InjectMessage(message string, skipLlm bool, autoListen bool) error {
	c.cancelRetainedSessionCleanup("inject_message")
	if err := c.prepareSpeakPathForInjectedSpeech(message, autoListen); err != nil {
		return err
	}
	session, err := c.ensureSession()
	if err != nil {
		return err
	}
	options := llmResponseChannelOptions{
		onTTSPlaybackStart: c.newInjectedSpeechStartHook(),
		ttsTurnEndPolicy:   injectedSpeechTTSTurnEndPolicy(autoListen),
	}
	if skipLlm {
		return session.AddTextToTTSQueueWithOptions(message, options)
	}
	return session.AddAsrResultToQueueWithOptions(message, nil, options)
}

func (c *ChatManager) prepareSpeakPathForInjectedSpeech(previewText string, autoListen bool) error {
	if c == nil || c.serverTransport == nil {
		return nil
	}
	if c.serverTransport.GetTransportType() != types_conn.TransportTypeMqttUdp {
		log.Debugf("Thiết bị %s tin nhắn injected bỏ qua speak_request: transport=%s", c.DeviceID, c.serverTransport.GetTransportType())
		return nil
	}
	if !c.shouldSendSpeakRequest(time.Now()) {
		log.Debugf("Thiết bị %s tin nhắn injected tái sử dụng đường phát hiện có, bỏ qua speak_request", c.DeviceID)
		return nil
	}
	if _, err := c.ensureClientSessionID(); err != nil {
		return err
	}

	needSessionBootstrap := c.requiresHelloBootstrapForSession()
	pending, created := c.getOrCreatePendingSpeakRequest()
	if created {
		if err := c.serverTransport.SendSpeakRequest(previewText, autoListen); err != nil {
			c.finishPendingSpeakRequest(pending, err)
			return err
		}
		log.Infof("Thiết bị %s đã gửi speak_request, session_id=%s", c.DeviceID, pending.sessionID)
	}

	waitCtx := c.ctx
	if waitCtx == nil {
		waitCtx = context.Background()
	}
	if err := c.waitPendingSpeakRequest(waitCtx, pending); err != nil {
		return err
	}
	if needSessionBootstrap {
		if err := c.waitForInjectedSpeechSession(waitCtx); err != nil {
			return err
		}
	}
	return nil
}

func (c *ChatManager) shouldSendSpeakRequest(now time.Time) bool {
	if c == nil || c.serverTransport == nil {
		return false
	}
	if c.serverTransport.GetTransportType() != types_conn.TransportTypeMqttUdp {
		log.Debugf("Thiết bị %s xác định speak_request: transport=%s, không cần gửi", c.DeviceID, c.serverTransport.GetTransportType())
		return false
	}
	if c.requiresHelloBootstrapForSession() {
		log.Debugf("Thiết bị %s xác định speak_request: ChatSession phụ thuộc vào hello mới để thiết lập, cần gửi", c.DeviceID)
		return true
	}
	if c.needsMqttRebootstrap() {
		log.Debugf("Thiết bị %s xác định speak_request: đường truyền MQTT đang chờ tái tạo, cần gửi", c.DeviceID)
		return true
	}
	if c.isConversationActive() {
		log.Debugf("Thiết bị %s xác định speak_request: hiện đang trong phiên hội thoại, bỏ qua gửi", c.DeviceID)
		return false
	}

	warmAt := c.currentSpeakPathWarmAt()
	if warmAt <= 0 {
		log.Debugf("Thiết bị %s xác định speak_request: không có đường ấm nào để tái sử dụng, cần gửi", c.DeviceID)
		return true
	}
	reuseWindow := speakRequestReuseWindow()
	idleFor := now.Sub(time.UnixMilli(warmAt))
	if idleFor <= reuseWindow {
		log.Debugf("Thiết bị %s xác định speak_request: đường ấm vẫn còn hiệu lực idle_for=%s reuse_window=%s, bỏ qua gửi", c.DeviceID, idleFor, reuseWindow)
		return false
	}
	log.Debugf("Thiết bị %s xác định speak_request: đường ấm đã hết hạn idle_for=%s reuse_window=%s, cần gửi", c.DeviceID, idleFor, reuseWindow)
	return true
}

func (c *ChatManager) requiresHelloBootstrapForSession() bool {
	if c == nil {
		return false
	}
	if c.GetSession() != nil {
		return false
	}
	if !c.helloInited {
		return true
	}
	return c.requiresFreshHello()
}

func (c *ChatManager) ensureClientSessionID() (string, error) {
	if c == nil || c.clientState == nil {
		return "", fmt.Errorf("clientState chưa được khởi tạo")
	}
	sessionID := strings.TrimSpace(c.clientState.SessionID)
	if sessionID != "" {
		return sessionID, nil
	}
	session, err := auth.A().CreateSession(c.DeviceID)
	if err != nil {
		return "", fmt.Errorf("tạo phiên thất bại: %v", err)
	}
	c.clientState.SessionID = session.ID
	return session.ID, nil
}

func (c *ChatManager) waitForInjectedSpeechSession(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if _, err := c.ensureSession(); err == nil {
		return nil
	} else if !shouldRetryInjectedSpeechSessionWait(err) {
		return err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	timeout := c.speakReadyTimeout
	if timeout <= 0 {
		timeout = defaultSpeakReadyTimeout
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		if _, err := c.ensureSession(); err == nil {
			return nil
		} else {
			lastErr = err
			if !shouldRetryInjectedSpeechSessionWait(err) {
				return err
			}
		}

		select {
		case <-waitCtx.Done():
			if waitCtx.Err() == context.DeadlineExceeded {
				if lastErr != nil {
					return fmt.Errorf("chờ thiết lập ChatSession timeout: %w", lastErr)
				}
				return fmt.Errorf("chờ thiết lập ChatSession timeout")
			}
			return waitCtx.Err()
		case <-ticker.C:
		}
	}
}

func shouldRetryInjectedSpeechSessionWait(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "hello chưa được khởi tạo") ||
		strings.Contains(msg, "gửi lại hello") ||
		strings.Contains(msg, "đang khởi động") ||
		strings.Contains(msg, "đang đóng")
}

func (c *ChatManager) isConversationActive() bool {
	if c == nil || c.clientState == nil {
		return false
	}
	if phase := c.clientState.GetListenPhase(); phase != "" && phase != ListenPhaseIdle {
		return true
	}
	switch c.clientState.GetStatus() {
	case ClientStatusListening, ClientStatusLLMStart, ClientStatusTTSStart:
		return true
	}
	session := c.GetSession()
	return session != nil && session.IsTTSActive()
}

func (c *ChatManager) getOrCreatePendingSpeakRequest() (*pendingSpeakRequest, bool) {
	c.speakRequestMu.Lock()
	defer c.speakRequestMu.Unlock()

	if c.pendingSpeakRequest != nil {
		return c.pendingSpeakRequest, false
	}

	sessionID := ""
	if c.clientState != nil {
		sessionID = strings.TrimSpace(c.clientState.SessionID)
	}
	pending := &pendingSpeakRequest{
		sessionID: sessionID,
		done:      make(chan struct{}),
	}
	timeout := c.speakReadyTimeout
	if timeout <= 0 {
		timeout = defaultSpeakReadyTimeout
	}
	pending.timer = time.AfterFunc(timeout, func() {
		c.finishPendingSpeakRequest(pending, fmt.Errorf("chờ speak_ready timeout"))
	})
	c.pendingSpeakRequest = pending
	return pending, true
}

func (c *ChatManager) waitPendingSpeakRequest(ctx context.Context, pending *pendingSpeakRequest) error {
	if pending == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-pending.done:
		return pending.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *ChatManager) finishPendingSpeakRequest(pending *pendingSpeakRequest, err error) {
	if pending == nil {
		return
	}
	c.speakRequestMu.Lock()
	if c.pendingSpeakRequest == pending {
		c.pendingSpeakRequest = nil
	}
	c.speakRequestMu.Unlock()
	pending.resolve(err)
}

func (c *ChatManager) refreshSpeakPathWarmFromTransport() {
	if c == nil || c.serverTransport == nil || !c.serverTransport.HasActiveUDPBinding() {
		return
	}
	if c.hasPendingSpeakRequest() {
		log.Debugf("Thiết bị %s hiện có speak_request đang chờ hoàn tất, bỏ qua việc làm mới đường ấm", c.DeviceID)
		return
	}
	if c.needsMqttRebootstrap() {
		log.Debugf("Thiết bị %s hiện có cờ tái tạo phiên MQTT, bỏ qua việc làm mới đường ấm", c.DeviceID)
		return
	}
	if ts := c.serverTransport.GetUDPLastActiveTs(); ts > 0 {
		c.updateSpeakPathWarmAt(ts)
		return
	}
	c.markSpeakPathWarm(time.Now())
}

func (c *ChatManager) currentSpeakPathWarmAt() int64 {
	if c == nil {
		return 0
	}
	latest := c.lastSpeakPathWarmAt.Load()
	if c.serverTransport != nil {
		if transportTs := c.serverTransport.GetUDPLastActiveTs(); transportTs > latest {
			latest = transportTs
		}
	}
	return latest
}

func (c *ChatManager) markSpeakPathWarm(ts time.Time) {
	if ts.IsZero() {
		ts = time.Now()
	}
	c.updateSpeakPathWarmAt(ts.UnixMilli())
}

func (c *ChatManager) updateSpeakPathWarmAt(ts int64) {
	if c == nil || ts <= 0 {
		return
	}
	for {
		current := c.lastSpeakPathWarmAt.Load()
		if current >= ts {
			return
		}
		if c.lastSpeakPathWarmAt.CompareAndSwap(current, ts) {
			return
		}
	}
}

func speakRequestReuseWindow() time.Duration {
	if !viper.IsSet("chat.speak_request_reuse_window_ms") {
		return defaultSpeakRequestReuseWindow
	}
	ms := viper.GetInt64("chat.speak_request_reuse_window_ms")
	if ms <= 0 {
		return defaultSpeakRequestReuseWindow
	}
	return time.Duration(ms) * time.Millisecond
}

func (c *ChatManager) newInjectedSpeechStartHook() func() {
	if c == nil {
		return nil
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			if c.serverTransport == nil || c.serverTransport.GetTransportType() != types_conn.TransportTypeMqttUdp {
				return
			}
			c.markSpeakPathWarm(time.Now())
		})
	}
}

func injectedSpeechTTSTurnEndPolicy(autoListen bool) ttsTurnEndPolicy {
	if autoListen {
		return ttsTurnEndPolicyNone
	}
	return ttsTurnEndPolicyGoodbyeAndIdle
}

func (c *ChatManager) handleTTSTurnEndPolicy(ctx context.Context, policy ttsTurnEndPolicy, stopErr error) {
	if c == nil || policy == ttsTurnEndPolicyNone {
		return
	}
	if stopErr != nil {
		log.Debugf("Thiết bị %s TTS turn end policy bị bỏ qua: policy=%d err=%v", c.DeviceID, policy, stopErr)
		return
	}

	switch policy {
	case ttsTurnEndPolicyGoodbyeAndIdle:
		if c.serverTransport == nil || c.serverTransport.GetTransportType() != types_conn.TransportTypeMqttUdp {
			return
		}
		if !ttsTurnPlaybackSettledFromContext(ctx) {
			timer := time.NewTimer(ttsPlaybackCompletionGrace)
			defer stopTimer(timer)

			select {
			case <-timer.C:
			case <-c.ctx.Done():
				log.Debugf("Thiết bị %s TTS turn end policy hủy goodbye bị trì hoãn: %v", c.DeviceID, c.ctx.Err())
				return
			}
		}
		if c.managerClosing.Load() || c.serverTransport == nil || c.serverTransport.GetTransportType() != types_conn.TransportTypeMqttUdp {
			return
		}

		session := c.GetSession()
		if session == nil || session.IsClosing() {
			log.Debugf("Thiết bị %s TTS turn end policy bị bỏ qua: phiên đã đóng", c.DeviceID)
			return
		}
		session.CloseWithReason(chatSessionCloseReasonExplicitExit)
	}
}

func (c *ChatManager) InjectOpenClawResponse(event openclaw.ResponseDelivery) error {
	c.cancelRetainedSessionCleanup("openclaw_response")
	session, err := c.ensureSession()
	if err != nil {
		return err
	}
	return session.InjectOpenClawResponse(event)
}

func (c *ChatManager) ExitChat() error {
	session := c.GetSession()
	if session == nil {
		return nil
	}
	session.DoExitChat()
	return nil
}

func (c *ChatManager) resetOpenClawModeOnHello(agentIDs ...string) {
	deviceID := strings.TrimSpace(c.clientState.DeviceID)
	if deviceID == "" {
		return
	}

	openclawManager := openclaw.GetManager()
	seen := make(map[string]struct{}, len(agentIDs))
	for _, agentID := range agentIDs {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			continue
		}
		if _, exists := seen[agentID]; exists {
			continue
		}
		seen[agentID] = struct{}{}
		if openclawManager.ExitMode(agentID, deviceID) {
			log.Infof("Thiết bị %s reset chế độ OpenClaw sau hello: agent=%s", deviceID, agentID)
		}
	}
}

func (c *ChatManager) refreshDeviceConfigOnHello() error {
	configProvider, err := userconfig.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		return fmt.Errorf("lấy provider cấu hình thất bại: %w", err)
	}

	deviceConfig, err := configProvider.GetUserConfig(c.clientState.Ctx, c.clientState.DeviceID)
	if err != nil {
		return fmt.Errorf("lấy cấu hình thiết bị thất bại: %w", err)
	}
	deviceConfig.MemoryMode = NormalizeMemoryMode(deviceConfig.MemoryMode)
	deviceConfig.SpeakerChatMode = NormalizeSpeakerChatMode(deviceConfig.SpeakerChatMode)

	prevAgentID := c.clientState.AgentID
	c.clientState.AgentID = deviceConfig.AgentId
	c.clientState.DeviceConfig = deviceConfig
	c.clientState.SystemPrompt = deviceConfig.SystemPrompt
	c.clientState.SpeakerTTSConfig = nil
	applyOutputAudioFormatForTTS(c.clientState)

	log.Infof("Thiết bị %s làm mới cấu hình hello thành công, agent: %s -> %s", c.clientState.DeviceID, prevAgentID, deviceConfig.AgentId)
	return nil
}