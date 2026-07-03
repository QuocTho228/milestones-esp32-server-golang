package chat

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"

	. "milestones-esp32-server-golang/internal/data/client"
	"milestones-esp32-server-golang/internal/data/history"
	. "milestones-esp32-server-golang/internal/data/msg"
	chathooks "milestones-esp32-server-golang/internal/domain/chat/hooks"
	"milestones-esp32-server-golang/internal/domain/chat/streamtransform"
	user_config "milestones-esp32-server-golang/internal/domain/config"
	"milestones-esp32-server-golang/internal/domain/config/types"
	"milestones-esp32-server-golang/internal/domain/eventbus"
	"milestones-esp32-server-golang/internal/domain/llm"
	llm_common "milestones-esp32-server-golang/internal/domain/llm/common"
	"milestones-esp32-server-golang/internal/domain/mcp"
	"milestones-esp32-server-golang/internal/domain/memory"
	"milestones-esp32-server-golang/internal/domain/memory/llm_memory"
	"milestones-esp32-server-golang/internal/domain/openclaw"
	"milestones-esp32-server-golang/internal/domain/speaker"
	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"
)

type AsrResponseChannelItem struct {
	ctx           context.Context
	text          string
	speakerResult *speaker.IdentifyResult
}

const detectLLMDebounceDuration = 300 * time.Millisecond

type detectAction string

const (
	detectActionSilent  detectAction = "silent"
	detectActionWelcome detectAction = "welcome"
	detectActionLLM     detectAction = "llm"
)

type welcomePlaybackResult struct {
	natural bool
}

const (
	chatSessionCloseReasonManagerShutdown     = "manager_shutdown"
	chatSessionCloseReasonExplicitExit        = "explicit_exit"
	chatSessionCloseReasonFatalError          = "fatal_error"
	chatSessionCloseReasonAudioIdleTimeout    = "audio_idle_timeout"
	chatSessionCloseReasonRetainedIdleTimeout = "retained_idle_timeout"
)

type ChatSession struct {
	clientState     *ClientState
	asrManager      *ASRManager
	ttsManager      *TTSManager
	llmManager      *LLMManager
	speakerManager  *SpeakerManager
	mediaPlayer     *SessionMediaPlayer
	serverTransport *ServerTransport

	ctx    context.Context
	cancel context.CancelFunc

	chatTextQueue *util.Queue[AsrResponseChannelItem]

	// Lưu tạm kết quả nhận diện vân giọng nói (được bảo vệ bằng lock)
	speakerResultMu        sync.RWMutex
	pendingSpeakerResult   *speaker.IdentifyResult
	speakerResultReady     chan struct{} // Chỉ dùng để thông báo sẵn sàng, không truyền dữ liệu
	turnSpeakerInterrupted atomic.Bool

	vadLoopStarted              bool
	listenStartSeq              atomic.Uint64
	realtimeListenSessionActive atomic.Bool

	// Khi thiết bị chưa kích hoạt bị trigger với tần suất cao, tái sử dụng kết quả phán định "chưa kích hoạt" gần nhất trong thời gian ngắn, tránh gọi API quá thường xuyên.
	activationCheckMu     sync.Mutex
	lastActivationFalseAt time.Time

	// Bảo vệ Close, tránh đóng nhiều lần
	closeOnce sync.Once
	closing   atomic.Bool

	// Bảo vệ stopSpeaking, tránh xung đột đồng thời với AddAsrResultToQueue/HandleWelcome
	stopSpeakingMu sync.Mutex

	welcomePlaybackMu     sync.Mutex
	welcomePlaybackDoneCh chan welcomePlaybackResult

	detectLLMDebounceMu    sync.Mutex
	detectLLMDebounceTimer *time.Timer

	openClawStreamMu sync.Mutex
	openClawStreams  map[string]chan llm_common.LLMResponseStruct

	openClawWarmupMu sync.Mutex
	openClawWarmup   *openClawWarmupTask

	hookHub      *chathooks.Hub
	closeHandler func(session *ChatSession, reason string)
}

type ChatSessionOption func(*ChatSession)

func WithChatSessionCloseHandler(handler func(session *ChatSession, reason string)) ChatSessionOption {
	return func(s *ChatSession) {
		s.closeHandler = handler
	}
}

func NewChatSession(clientState *ClientState, serverTransport *ServerTransport, hookHub *chathooks.Hub, transformRegistry *streamtransform.Registry, opts ...ChatSessionOption) *ChatSession {
	s := &ChatSession{
		clientState:        clientState,
		serverTransport:    serverTransport,
		chatTextQueue:      util.NewQueue[AsrResponseChannelItem](10),
		speakerResultReady: make(chan struct{}, 1), // Buffer là 1, tránh bị block
		openClawStreams:    make(map[string]chan llm_common.LLMResponseStruct),
		hookHub:            hookHub,
	}
	for _, opt := range opts {
		opt(s)
	}

	s.asrManager = NewASRManager(clientState, serverTransport)
	s.asrManager.session = s // Thiết lập tham chiếu session
	s.ttsManager = NewTTSManager(clientState, serverTransport, s)
	s.mediaPlayer = NewSessionMediaPlayer(s)
	s.llmManager = NewLLMManager(clientState, serverTransport, s.ttsManager, s, transformRegistry)

	clientState.OnVoiceSilenceMetricCallback = func(ctx context.Context, ts int64) {
		s.TraceVoiceSilence(ctx, ts)
	}

	// Nếu bật nhận diện vân giọng nói, tạo speaker manager
	if clientState.IsSpeakerEnabled() {
		// Lấy địa chỉ dịch vụ nhận diện vân giọng nói từ cấu hình hệ thống (viper)
		baseURL := viper.GetString("voice_identify.base_url")
		if baseURL != "" {
			// Thiết lập địa chỉ dịch vụ và ngưỡng vào cấu hình
			speakerConfig := map[string]interface{}{
				"base_url": baseURL,
			}
			// Đọc cấu hình ngưỡng, nếu chưa cấu hình thì dùng giá trị mặc định 0.6
			if viper.IsSet("voice_identify.threshold") {
				threshold := viper.GetFloat64("voice_identify.threshold")
				speakerConfig["threshold"] = threshold
			}

			provider, err := speaker.GetSpeakerProvider(speakerConfig)
			if err != nil {
				log.Warnf("Tạo speaker provider thất bại: %v", err)
			} else {
				clientState.SpeakerProvider = provider
				s.speakerManager = NewSpeakerManager(provider)
				log.Debugf("Thiết bị %s bật nhận diện vân giọng nói", clientState.DeviceID)

				// Thiết lập callback lấy kết quả vân giọng nói bất đồng bộ
				clientState.OnVoiceSilenceSpeakerCallback = func(ctx context.Context) {
					log.Debugf("[Nhận diện vân giọng nói] OnVoiceSilenceSpeakerCallback được gọi, deviceID: %s", clientState.DeviceID)

					// Lấy kết quả vân giọng nói bất đồng bộ
					go func() {
						log.Debugf("[Nhận diện vân giọng nói] Bắt đầu lấy kết quả nhận diện bất đồng bộ, deviceID: %s", clientState.DeviceID)

						// Kiểm tra speakerManager có đang active hay không
						if !s.speakerManager.IsActive() {
							//log.Warnf("[Nhận diện vân giọng nói] speakerManager chưa active, không thể lấy kết quả nhận diện")
							return
						}
						// Xóa kết quả trước đó
						s.speakerResultMu.Lock()
						oldResult := s.pendingSpeakerResult
						s.pendingSpeakerResult = nil
						s.speakerResultMu.Unlock()
						if oldResult != nil {
							log.Debugf("[Nhận diện vân giọng nói] Đã xóa kết quả nhận diện trước đó: identified=%v, speaker_id=%s", oldResult.Identified, oldResult.SpeakerID)
						}

						// Xóa thông báo sẵn sàng (không chặn)
						select {
						case <-s.speakerResultReady:
							log.Debugf("[Nhận diện vân giọng nói] Đã xóa channel thông báo sẵn sàng")
						default:
							log.Debugf("[Nhận diện vân giọng nói] Channel thông báo sẵn sàng đã rỗng")
						}

						result, err := s.speakerManager.FinishAndIdentify(ctx)
						if err != nil {
							log.Warnf("[Nhận diện vân giọng nói] Lấy kết quả nhận diện thất bại: %v, deviceID: %s", err, clientState.DeviceID)
							// Nhận diện vân giọng nói thất bại không ảnh hưởng đến luồng chính, lưu kết quả nil
							s.speakerResultMu.Lock()
							s.pendingSpeakerResult = nil
							s.speakerResultMu.Unlock()
							log.Debugf("[Nhận diện vân giọng nói] Đã lưu kết quả nil (nhận diện thất bại)")
						} else if result != nil && result.Identified {
							log.Infof("[Nhận diện vân giọng nói] Đã nhận diện được người nói: %s (độ tin cậy: %.4f, ngưỡng: %.4f), deviceID: %s",
								result.SpeakerName, result.Confidence, result.Threshold, clientState.DeviceID)
							log.Debugf("[Nhận diện vân giọng nói] Chi tiết kết quả nhận diện: speaker_id=%s, speaker_name=%s, confidence=%.4f, threshold=%.4f",
								result.SpeakerID, result.SpeakerName, result.Confidence, result.Threshold)
							s.speakerResultMu.Lock()
							s.pendingSpeakerResult = result
							s.speakerResultMu.Unlock()
							log.Debugf("[Nhận diện vân giọng nói] Đã lưu kết quả nhận diện (đã nhận diện được)")
						} else {
							// Không nhận diện được người nói, cũng lưu kết quả
							if result != nil {
								log.Debugf("[Nhận diện vân giọng nói] Không nhận diện được người nói: identified=%v, confidence=%.4f, threshold=%.4f, deviceID: %s",
									result.Identified, result.Confidence, result.Threshold, clientState.DeviceID)
							} else {
								log.Debugf("[Nhận diện vân giọng nói] Kết quả nhận diện là nil, deviceID: %s", clientState.DeviceID)
							}
							s.speakerResultMu.Lock()
							s.pendingSpeakerResult = result
							s.speakerResultMu.Unlock()
							log.Debugf("[Nhận diện vân giọng nói] Đã lưu kết quả nhận diện (không nhận diện được)")
						}

						// Thông báo kết quả đã sẵn sàng
						select {
						case s.speakerResultReady <- struct{}{}:
							log.Debugf("[Nhận diện vân giọng nói] Đã gửi thông báo kết quả sẵn sàng, deviceID: %s", clientState.DeviceID)
						default:
							log.Warnf("[Nhận diện vân giọng nói] Channel thông báo kết quả sẵn sàng đã đầy, không thể gửi thông báo, deviceID: %s", clientState.DeviceID)
						}
					}()
				}
			}
		}
	}

	// Thiết lập callback cho lần đầu ASR trả về ký tự
	clientState.OnAsrFirstTextCallback = func(text string, isFinal bool) {
		clientState.Asr.MarkTextReceived()
		clientState.ClearAudioIdleTimeoutPending()
		clientState.PauseAudioIdleWindow(time.Now())
		log.Debugf("ASR trả về ký tự lần đầu: device=%s, text=%s, isFinal=%v", clientState.DeviceID, text, isFinal)
		clientState.MarkAsrFirstText()
		s.TraceAsrFirstText(clientState.Ctx, time.Now().UnixMilli())
		if clientState.IsRealTime() && viper.GetInt("chat.realtime_mode") == 4 {
			if s.isRealtimeMcpAudioGateActive() {
				log.Debugf("Thiết bị %s realtime media playback gating đang active, bỏ qua việc ngắt bởi ký tự ASR đầu tiên: text=%s", clientState.DeviceID, text)
				return
			}
			s.StopAssistantOutputAfterAsrWithReason(true, "ChatSession.OnAsrFirstTextCallback realtime_mode=4")
		}
	}

	return s
}

func (s *ChatSession) Start(pctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(pctx)

	if s.clientState.InputAudioFormat.SampleRate <= 0 || s.clientState.InputAudioFormat.Channels <= 0 {
		return fmt.Errorf("định dạng audio đầu vào chưa được khởi tạo, vui lòng hoàn tất bắt tay hello trước")
	}

	err := s.InitAsrLlmTts()
	if err != nil {
		log.Errorf("Khởi tạo ASR/LLM/TTS thất bại: %v", err)
		return err
	}

	// Tải tin nhắn lịch sử bất đồng bộ, không chặn việc khởi động session
	go func() {
		err := s.initHistoryMessages()
		if err != nil {
			log.Errorf("Khởi tạo lịch sử hội thoại thất bại: %v", err)
		}
	}()

	if !s.vadLoopStarted {
		// Session-level idle watchdog cần tồn tại độc lập với vòng đời của một lần ASR loop,
		// nhờ đó ở chế độ auto sau khi một vòng kết thúc thành công vẫn có thể tiếp tục đếm thời gian idle của kết nối.
		go s.asrManager.runAudioIdleTimeoutWatchdog(s.ctx)
		s.asrManager.ProcessVadAudio(s.ctx)
		s.vadLoopStarted = true
	}

	go s.processChatText(s.ctx)  //Xử lý tin nhắn hội thoại sau ASR
	go s.llmManager.Start(s.ctx) //Xử lý chuỗi tin nhắn trả về sau LLM
	go s.ttsManager.Start(s.ctx) //Xử lý hàng đợi tin nhắn của TTS
	if s.mediaPlayer != nil {
		s.mediaPlayer.AttachSession()
	}

	return nil
}

// Khởi tạo lịch sử hội thoại vào bộ nhớ
func (s *ChatSession) initHistoryMessages() error {
	var historyMessages []*schema.Message
	var err error

	if s.clientState.GetMemoryMode() == MemoryModeNone {
		log.Debugf("Thiết bị %s chế độ ghi nhớ=none, bỏ qua việc tải tin nhắn lịch sử", s.clientState.DeviceID)
		return nil
	}

	// Chọn nguồn dữ liệu theo cấu hình (không có quan hệ ưu tiên, chọn trực tiếp)
	useRedis := s.shouldUseRedis()
	useManager := s.shouldUseManager()

	// Xác thực trường bắt buộc: DeviceID không được để trống
	if s.clientState.DeviceID == "" {
		log.Debugf("DeviceID rỗng, bỏ qua việc tải tin nhắn lịch sử (có thể được gọi trước tin nhắn hello)")
		return nil
	}

	// Chọn nguồn dữ liệu theo cấu hình (không có quan hệ ưu tiên, chọn trực tiếp)
	if useRedis {
		// Tải từ Redis
		historyMessages, err = llm_memory.Get().GetMessages(
			s.ctx,
			s.clientState.DeviceID,
			s.clientState.AgentID,
			20)
		if err != nil {
			log.Warnf("Tải tin nhắn lịch sử từ Redis thất bại: %v", err)
			return err
		}
		log.Infof("Đã tải %d tin nhắn lịch sử từ Redis", len(historyMessages))
	} else if useManager {
		// Tải từ Manager
		historyMessages, err = s.loadFromManager()
		if err != nil {
			log.Warnf("Tải tin nhắn lịch sử từ Manager thất bại: %v", err)
			return err
		}
		log.Infof("Đã tải %d tin nhắn lịch sử từ Manager", len(historyMessages))
	} else {
		// Cả hai nguồn dữ liệu đều chưa được cấu hình, không tải tin nhắn lịch sử
		log.Debugf("Cả Redis và Manager đều chưa được cấu hình, bỏ qua việc tải tin nhắn lịch sử")
		return nil
	}

	if len(historyMessages) > 0 {
		s.clientState.InitMessages(historyMessages)
		log.Infof("Tải thành công %d tin nhắn lịch sử", len(historyMessages))
	} else {
		log.Debugf("Không tải được tin nhắn lịch sử (có thể không có lịch sử)")
	}

	return nil
}

// shouldUseRedis Xác định có sử dụng Redis làm nguồn dữ liệu hay không
func (s *ChatSession) shouldUseRedis() bool {
	// Xác định dựa trên config_provider.type
	providerType := viper.GetString("config_provider.type")
	return providerType == "redis"
}

// shouldUseManager Xác định có sử dụng Manager làm nguồn dữ liệu hay không
func (s *ChatSession) shouldUseManager() bool {
	// Xác định dựa trên config_provider.type
	providerType := viper.GetString("config_provider.type")
	return providerType == "manager"
}

// loadFromManager Tải tin nhắn lịch sử từ database Manager
func (s *ChatSession) loadFromManager() ([]*schema.Message, error) {
	// Tạo HistoryClient
	historyCfg := history.HistoryClientConfig{
		BaseURL:   util.GetBackendURL(),
		AuthToken: util.GetManagerAuthToken(),
		Timeout:   viper.GetDuration("manager.history_timeout"),
		Enabled:   true,
	}
	client := history.NewHistoryClient(historyCfg)

	if s.clientState.DeviceID == "" || s.clientState.AgentID == "" {
		return []*schema.Message{}, nil
	}

	req := &history.GetMessagesRequest{
		DeviceID:  s.clientState.DeviceID,
		AgentID:   s.clientState.AgentID,
		SessionID: s.clientState.SessionID,
		Limit:     20,
	}

	resp, err := client.GetMessages(s.ctx, req)
	if err != nil {
		return nil, err
	}

	// Chuyển đổi sang định dạng schema.Message
	messages := make([]*schema.Message, 0, len(resp.Messages))
	for _, item := range resp.Messages {
		var msg *schema.Message
		switch item.Role {
		case "user":
			msg = schema.UserMessage(item.Content)
		case "assistant":
			msg = schema.AssistantMessage(item.Content, item.ToolCalls)
		case "tool":
			msg = schema.ToolMessage(item.Content, item.ToolCallID)
		case "system":
			msg = schema.SystemMessage(item.Content)
		default:
			log.Warnf("Vai trò tin nhắn không xác định: %s", item.Role)
			continue
		}

		messages = append(messages, msg)
	}

	for _, msg := range messages {
		log.Debugf("Tin nhắn lịch sử: %+v", msg)
	}

	return messages, nil
}

// Thực hiện sau khi mqtt nhận được type: listen, state: start
func (c *ChatSession) InitAsrLlmTts() error {
	//Khởi tạo cấu trúc asr
	c.clientState.InitAsr()

	// Khởi tạo memory (memory không nằm trong resource pool)
	memoryMode := c.clientState.GetMemoryMode()
	memoryConfig := c.clientState.DeviceConfig.Memory
	memoryType := memory.MemoryType(memoryConfig.Provider)
	if memoryMode != MemoryModeLong {
		memoryType = memory.MemoryTypeNone
	}

	memoryProvider, err := memory.GetProvider(memoryType, memoryConfig.Config)
	if err != nil {
		return fmt.Errorf("tạo Memory provider thất bại: %v", err)
	}
	c.clientState.MemoryProvider = memoryProvider

	if memoryMode == MemoryModeLong {
		// Khởi tạo memory context (chỉ ở chế độ ghi nhớ dài hạn)
		context, err := memoryProvider.GetContext(c.ctx, c.clientState.GetDeviceIDOrAgentID(), 500)
		if err != nil {
			log.Warnf("Khởi tạo memory context thất bại: %v", err)
		}
		c.clientState.MemoryContext = context
	} else {
		c.clientState.MemoryContext = ""
	}

	return nil
}

// HandleAudioMessage Xử lý tin nhắn audio
func (c *ChatSession) HandleAudioMessage(data []byte) bool {
	select {
	case c.clientState.OpusAudioBuffer <- data:
		return true
	default:
		log.Warnf("Bộ đệm audio đã đầy, bỏ dữ liệu audio")
	}
	return false
}

// handleListenMessage Xử lý tin nhắn listen
func (s *ChatSession) HandleListenMessage(msg *ClientMessage) error {
	// Xử lý theo trạng thái
	switch msg.State {
	case MessageStateStart:
		s.HandleListenStart(msg)
	case MessageStateStop:
		s.HandleListenStop()
	case MessageStateDetect:
		s.HandleListenDetect(msg)
	}

	// Ghi log
	log.Infof("Thiết bị %s cập nhật trạng thái theo dõi audio: %s", msg.DeviceID, msg.State)
	return nil
}

func (s *ChatSession) beginListenStart() uint64 {
	startSeq := s.listenStartSeq.Add(1)
	if s.clientState.IsRealTime() {
		s.realtimeListenSessionActive.Store(true)
	}
	s.clientState.SetListenPhase(ListenPhaseStarting)
	return startSeq
}

func (s *ChatSession) invalidateListenStart() {
	s.listenStartSeq.Add(1)
	s.realtimeListenSessionActive.Store(false)
	s.clientState.SetListenPhase(ListenPhaseIdle)
}

func (s *ChatSession) isCurrentListenStart(startSeq uint64) bool {
	return startSeq == s.listenStartSeq.Load()
}

func (s *ChatSession) isRealtimeListenSessionActive() bool {
	return s.realtimeListenSessionActive.Load()
}

func (s *ChatSession) shouldIgnoreListenStartError(startSeq uint64, ctx context.Context, err error) bool {
	if !s.isCurrentListenStart(startSeq) {
		return true
	}
	if ctx != nil && ctx.Err() != nil {
		return true
	}
	if s.clientState.Ctx.Err() != nil {
		return true
	}
	return errors.Is(err, context.Canceled)
}

func (s *ChatSession) shouldIgnoreAsrLoopError(startSeq uint64, ctx context.Context, err error) bool {
	if !s.isCurrentListenStart(startSeq) {
		return true
	}
	if ctx != nil && ctx.Err() != nil {
		return true
	}
	if s.clientState.Ctx.Err() != nil {
		return true
	}
	return errors.Is(err, context.Canceled)
}

func isAutoListenActive(state *ClientState) bool {
	if state == nil || state.ListenMode != "auto" {
		return false
	}
	phase := state.GetListenPhase()
	return phase == ListenPhaseStarting || phase == ListenPhaseListening
}

func shouldIgnoreListenStartDuringWelcome(mode string, welcomePlaying bool) bool {
	return mode != "realtime" && welcomePlaying
}

func shouldWaitRealtimeListenStartDuringWelcome(mode string, welcomePlaying bool) bool {
	return false
}

func shouldInterruptOutputOnListenStart(mode string, welcomePlaying bool) bool {
	if mode == "realtime" && welcomePlaying {
		return false
	}
	return true
}

func completeWelcomePlaybackWaitCh(ch chan welcomePlaybackResult, natural bool) {
	if ch == nil {
		return
	}
	select {
	case ch <- welcomePlaybackResult{natural: natural}:
	default:
	}
	close(ch)
}

func (s *ChatSession) beginWelcomePlaybackWait() {
	if s == nil {
		return
	}

	s.welcomePlaybackMu.Lock()
	staleCh := s.welcomePlaybackDoneCh
	s.welcomePlaybackDoneCh = make(chan welcomePlaybackResult, 1)
	s.welcomePlaybackMu.Unlock()

	if staleCh != nil {
		completeWelcomePlaybackWaitCh(staleCh, false)
	}
}

func (s *ChatSession) completeWelcomePlaybackWait(natural bool) {
	if s == nil {
		return
	}

	s.welcomePlaybackMu.Lock()
	ch := s.welcomePlaybackDoneCh
	s.welcomePlaybackDoneCh = nil
	s.welcomePlaybackMu.Unlock()

	completeWelcomePlaybackWaitCh(ch, natural)
}

func (s *ChatSession) currentWelcomePlaybackWaitCh() <-chan welcomePlaybackResult {
	if s == nil {
		return nil
	}

	s.welcomePlaybackMu.Lock()
	ch := s.welcomePlaybackDoneCh
	s.welcomePlaybackMu.Unlock()
	return ch
}

func (s *ChatSession) waitForWelcomePlaybackCompletion() bool {
	if s == nil {
		return true
	}

	doneCh := s.currentWelcomePlaybackWaitCh()
	if doneCh == nil {
		return true
	}

	var sessionDone <-chan struct{}
	if s.ctx != nil {
		sessionDone = s.ctx.Done()
	}

	log.Infof("Thiết bị %s realtime listen start đang chờ TTS lời chào kết thúc", s.clientState.DeviceID)

	select {
	case result, ok := <-doneCh:
		if !ok {
			log.Infof("Thiết bị %s channel chờ lời chào đã đóng, hủy realtime listen start", s.clientState.DeviceID)
			return false
		}
		if !result.natural {
			log.Infof("Thiết bị %s lời chào bị ngắt, hủy lần realtime listen start này", s.clientState.DeviceID)
			return false
		}
		log.Infof("Thiết bị %s phát lời chào hoàn tất, tiếp tục realtime listen start", s.clientState.DeviceID)
		return true
	case <-s.clientState.Ctx.Done():
		log.Debugf("Thiết bị %s client ctx đã bị hủy, dừng việc chờ realtime listen start", s.clientState.DeviceID)
		return false
	case <-sessionDone:
		log.Debugf("Thiết bị %s session ctx đã bị hủy, dừng việc chờ realtime listen start", s.clientState.DeviceID)
		return false
	}
}

func resolveDetectAction(text string, enableGreeting bool, welcomeAlreadySpoken bool, autoListenActive bool) detectAction {
	if text == "" {
		return detectActionSilent
	}
	if enableGreeting && isWakeupWord(text) {
		if !welcomeAlreadySpoken {
			return detectActionWelcome
		}
		if autoListenActive {
			return detectActionSilent
		}
		return detectActionLLM
	}
	return detectActionLLM
}

func (s *ChatSession) cancelPendingDetectLLM() {
	if s == nil {
		return
	}

	s.detectLLMDebounceMu.Lock()
	timer := s.detectLLMDebounceTimer
	s.detectLLMDebounceTimer = nil
	s.detectLLMDebounceMu.Unlock()

	if timer != nil {
		timer.Stop()
	}
}

func (s *ChatSession) scheduleDetectLLM(text string) {
	if s == nil {
		return
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	s.cancelPendingDetectLLM()

	var timer *time.Timer
	timer = time.AfterFunc(detectLLMDebounceDuration, func() {
		s.detectLLMDebounceMu.Lock()
		if s.detectLLMDebounceTimer != timer {
			s.detectLLMDebounceMu.Unlock()
			return
		}
		s.detectLLMDebounceTimer = nil
		s.detectLLMDebounceMu.Unlock()

		if s.IsClosing() || s.clientState == nil {
			return
		}
		if s.clientState.Ctx != nil && s.clientState.Ctx.Err() != nil {
			return
		}

		if phase := s.clientState.GetListenPhase(); phase != ListenPhaseIdle {
			log.Debugf("Detect LLM debounce skipped because listen phase=%s", phase)
			return
		}

		if err := s.AddAsrResultToQueue(text, nil); err != nil {
			log.Errorf("Detect LLM debounce enqueue failed: %v", err)
		}
	})

	s.detectLLMDebounceMu.Lock()
	s.detectLLMDebounceTimer = timer
	s.detectLLMDebounceMu.Unlock()
}

func (s *ChatSession) HandleListenDetect(msg *ClientMessage) error {
	// Khi có detect mới đến, trước tiên hủy debounce detect->LLM của lần trước chưa được trigger,
	// tránh việc văn bản dẫn nhập cũ bị gửi lặp lại vào hàng đợi LLM sau đó.
	s.cancelPendingDetectLLM()

	// Kiểm tra trạng thái kích hoạt thiết bị
	isActivated, err := s.CheckDeviceActivated()
	if err != nil {
		log.Errorf("Kiểm tra trạng thái kích hoạt thiết bị thất bại: %v", err)
		return err
	}
	if !isActivated {
		return nil
	}

	// Lấy snapshot lịch sử lệnh "trước khi detect này đến" trước, sau đó mới ghi lại detect hiện tại,
	// để log tiếp theo hiển thị history là lệnh trước đó, chứ không phải chính detect hiện tại.
	now := time.Now()
	prevHistory := s.clientState.GetCommandHistorySnapshot()
	s.clientState.RecordCommandArrival(CommandTypeDetect, now)

	// listen detect đại diện cho việc thiết bị phát hiện một đoạn văn bản dẫn nhập có thể sử dụng được,
	// ở đây không đi thẳng vào việc theo dõi chính thức, mà trước tiên phán định xem nó nên trigger lời chào, bỏ qua âm thầm, hay trì hoãn để vào LLM.
	if msg.Text != "" {
		text := removePunctuation(msg.Text)
		enableGreeting := viper.GetBool("enable_greeting")
		autoListenActive := isAutoListenActive(s.clientState)
		// Việc xử lý wake word chia thành ba loại:
		// 1. Lần đầu wake up và cho phép lời chào: phát lời chào;
		// 2. Lời chào đã phát và hiện đang ở auto listen: bỏ qua wake up lặp lại;
		// 3. Trường hợp khác: đi theo đường buffer detect -> LLM như văn bản thông thường.
		action := resolveDetectAction(text, enableGreeting, s.clientState.IsWelcomeSpeaking, autoListenActive)

		log.Debugf(
			"Detect recv: device=%s text=%q action=%s autoListenActive=%v history={%s} welcomeSpeaking=%v welcomePlaying=%v",
			msg.DeviceID,
			text,
			action,
			autoListenActive,
			prevHistory.DebugString(now),
			s.clientState.IsWelcomeSpeaking,
			s.clientState.IsWelcomePlaying,
		)

		if action == detectActionSilent {
			return nil
		}

		// Khi detect quyết định phát lời chào hoặc tiếp quản hội thoại, cần dừng output còn sót lại hiện tại trước,
		// tránh việc TTS/LLM vòng cũ giao thoa với hành động detect vòng mới.
		s.StopSpeakingWithReason(true, fmt.Sprintf("HandleListenDetect action=%s text=%q", action, text))

		if action == detectActionWelcome {
			s.HandleWelcome()
			return nil
		}

		if action == detectActionLLM {
			// Văn bản ở giai đoạn detect trước tiên thực hiện một debounce rất ngắn;
			// nếu ngay sau đó nhận được listen start, việc theo dõi chính thức sẽ tiếp quản.
			s.scheduleDetectLLM(text)
		}
	}
	return nil
}

func (s *ChatSession) HandleNotActivated() {
	configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		log.Errorf("Lấy config provider thất bại: %v", err)
		return
	}

	code, challenge, message, timeoutMs := configProvider.GetActivationInfo(s.clientState.Ctx, s.clientState.DeviceID, "client_id")
	if code == "" {
		log.Errorf("Lấy thông tin kích hoạt thất bại: %v", err)
		return
	}

	log.Infof("Mã kích hoạt: %s, mã challenge: %s, thông điệp: %s, thời gian timeout: %d", code, challenge, message, timeoutMs)

	s.ttsManager.EnqueueTtsStartWithReason(s.clientState.Ctx, "HandleNotActivated")
	defer s.ttsManager.EnqueueTtsStopWithReason(s.clientState.Ctx, "HandleNotActivated")

	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	err = s.ttsManager.handleTextResponse(ctx, llm_common.LLMResponseStruct{
		Text: fmt.Sprintf("Vui lòng thêm thiết bị trong trang quản trị, mã kích hoạt: %s", code),
	}, false)
	s.ttsManager.RequestTurnEnd(ctx, err)

}

func (s *ChatSession) HandleWelcome() {
	greetingText := s.GetRandomGreeting()

	s.stopSpeakingMu.Lock()
	defer s.stopSpeakingMu.Unlock()

	if s.clientState.Ctx.Err() != nil {
		log.Debugf("HandleWelcome client ctx đã bị hủy, bỏ qua lời chào")
		return
	}

	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	if ctx.Err() != nil {
		log.Debugf("HandleWelcome afterAsr ctx đã bị hủy, bỏ qua lời chào")
		return
	}

	s.clientState.IsWelcomeSpeaking = true
	s.clientState.IsWelcomePlaying = true
	s.beginWelcomePlaybackWait()

	go func(ctx context.Context, greetingText string) {
		if ctx.Err() != nil || s.clientState.Ctx.Err() != nil {
			s.completeWelcomePlaybackWait(false)
			return
		}

		s.ttsManager.EnqueueTtsStartWithReason(s.clientState.Ctx, "HandleWelcome")
		err := s.ttsManager.handleTextResponse(ctx, llm_common.LLMResponseStruct{Text: greetingText}, true)
		s.ttsManager.EnqueueTtsStopWithReason(s.clientState.Ctx, "HandleWelcome natural end")
		s.ttsManager.RequestTurnEnd(ctx, err)
	}(ctx, greetingText)
}

func (a *ChatSession) checkExitWords(text string) bool {
	exitWords := []string{"tạm biệt", "thôi nhé", "thoát", "thoát hội thoại"}
	for _, word := range exitWords {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func normalizeOpenClawKeywordText(text string) string {
	return removePunctuation(strings.ToLower(strings.TrimSpace(text)))
}

func containsOpenClawKeyword(text string, keywords []string) bool {
	normalizedText := normalizeOpenClawKeywordText(text)
	if normalizedText == "" {
		return false
	}
	for _, keyword := range keywords {
		normalizedKeyword := normalizeOpenClawKeywordText(keyword)
		if normalizedKeyword == "" {
			continue
		}
		if strings.Contains(normalizedText, normalizedKeyword) {
			return true
		}
	}
	return false
}

func (s *ChatSession) isOpenClawEnterKeyword(text string) bool {
	return containsOpenClawKeyword(text, s.clientState.DeviceConfig.OpenClaw.EnterKeywords)
}

func (s *ChatSession) isOpenClawExitKeyword(text string) bool {
	return containsOpenClawKeyword(text, s.clientState.DeviceConfig.OpenClaw.ExitKeywords)
}

func openClawLogSnippet(text string, maxRunes int) string {
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

func (s *ChatSession) GetRandomGreeting() string {
	greetingList := viper.GetStringSlice("greeting_list")
	if len(greetingList) == 0 {
		return "Xin chào, có gì hay ho không."
	}
	rand.Seed(time.Now().UnixNano())
	return greetingList[rand.Intn(len(greetingList))]
}

func (s *ChatSession) AddTextToTTSQueue(text string) error {
	return s.llmManager.AddTextToTTSQueue(text)
}

func (s *ChatSession) AddTextToTTSQueueWithOptions(text string, options llmResponseChannelOptions) error {
	return s.llmManager.AddTextToTTSQueueWithOptions(text, options)
}

func (s *ChatSession) IsTTSActive() bool {
	if s == nil || s.ttsManager == nil {
		return false
	}
	return s.ttsManager.ttsActive.Load()
}

func (s *ChatSession) getOrCreateOpenClawStream(correlationID string) (chan llm_common.LLMResponseStruct, bool, error) {
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" {
		return nil, false, fmt.Errorf("missing correlation_id")
	}

	s.openClawStreamMu.Lock()
	if existing, ok := s.openClawStreams[correlationID]; ok {
		s.openClawStreamMu.Unlock()
		return existing, false, nil
	}
	streamChan := make(chan llm_common.LLMResponseStruct, 16)
	s.openClawStreams[correlationID] = streamChan
	s.openClawStreamMu.Unlock()

	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	options := llmResponseChannelOptions{}
	hasWarmup := s.getOpenClawWarmupTask(correlationID) != nil
	if hasWarmup {
		options.disableTTSCommands = true
		options.onEndFunc = func(err error, args ...any) {
			// Warm-up đã tiếp quản start, khi phản hồi OpenClaw chính thức kết thúc cần bổ sung stop tại đây;
			// không thể gửi ở điểm chuyển đổi warm-up, nếu không sẽ cắt ngang phản hồi chính giữa chừng.
			if !s.clientState.IsRealTime() {
				s.ttsManager.EnqueueTtsStopWithReason(ctx, fmt.Sprintf("OpenClaw stream end correlation_id=%s", correlationID))
			}
			s.ttsManager.RequestTurnEnd(ctx, err)
			s.finishOpenClawWarmup(correlationID, false)
		}
	}
	log.Infof("OpenClaw stream created: device=%s correlation_id=%s warmup_attached=%v", s.clientState.DeviceID, correlationID, hasWarmup)
	if err := s.llmManager.HandleLLMResponseChannelAsyncWithOptions(ctx, nil, streamChan, options); err != nil {
		if hasWarmup && !s.clientState.IsRealTime() {
			s.ttsManager.EnqueueTtsStopWithReason(ctx, fmt.Sprintf("OpenClaw stream setup failed correlation_id=%s", correlationID))
		}
		if hasWarmup {
			s.ttsManager.RequestTurnEnd(ctx, err)
		}
		s.openClawStreamMu.Lock()
		delete(s.openClawStreams, correlationID)
		s.openClawStreamMu.Unlock()
		close(streamChan)
		return nil, false, err
	}

	return streamChan, true, nil
}

func (s *ChatSession) closeOpenClawStream(correlationID string) {
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" {
		return
	}
	s.openClawStreamMu.Lock()
	delete(s.openClawStreams, correlationID)
	s.openClawStreamMu.Unlock()
}

func (s *ChatSession) clearOpenClawStreams() {
	s.openClawStreamMu.Lock()
	s.openClawStreams = make(map[string]chan llm_common.LLMResponseStruct)
	s.openClawStreamMu.Unlock()
}

func (s *ChatSession) clearPendingSpeakerResult() {
	if s == nil {
		return
	}

	s.speakerResultMu.Lock()
	s.pendingSpeakerResult = nil
	s.speakerResultMu.Unlock()

	for {
		select {
		case <-s.speakerResultReady:
		default:
			return
		}
	}
}

func (s *ChatSession) InjectOpenClawResponse(event openclaw.ResponseDelivery) error {
	correlationID := strings.TrimSpace(event.CorrelationID)
	text := strings.TrimSpace(event.Text)

	// Fallback non-streaming: khi không có correlation_id thì tiêm trực tiếp theo từng câu đơn.
	if correlationID == "" {
		if text == "" {
			return nil
		}
		return s.AddTextToTTSQueue(text)
	}

	// Đoạn rỗng ở giữa không có ý nghĩa, bỏ qua trực tiếp; đoạn rỗng kết thúc được giữ lại để hoàn tất.
	if text == "" && !event.IsEnd {
		return nil
	}

	streamChan, created, err := s.getOrCreateOpenClawStream(correlationID)
	if err != nil {
		return err
	}

	isStart := event.IsStart
	if created && !isStart {
		// Nếu đoạn đến đầu tiên không được đánh dấu start, thì fallback khởi tạo đoạn đầu tiên.
		isStart = true
	}
	if isStart {
		if task := s.getOpenClawWarmupTask(correlationID); task != nil {
			if text != "" {
				// Chỉ dừng warm-up khi đoạn nội dung thực sự có thể phát đầu tiên đến, tránh bị đoạn dẫn nhập quá ngắn chiếm quyền quá sớm.
				// Đánh dấu đoạn đầu của warm-up chỉ dùng cho TTS warm-up, không được nuốt mất IsStart của đoạn đầu phản hồi chính thức,
				// nếu không phản hồi chính thức sẽ bị hạ cấp thành TTS đơn câu, snapshot tiếp theo cũng sẽ bị coi là câu thứ hai và phát lại.
				s.cancelOpenClawWarmup(correlationID, false)
				s.beginOpenClawSpeech(task)
			} else {
				isStart = false
			}
		}
	} else if event.IsEnd {
		s.cancelOpenClawWarmup(correlationID, false)
	}

	resp := llm_common.LLMResponseStruct{
		Text:    text,
		IsStart: isStart,
		IsEnd:   event.IsEnd,
	}

	select {
	case <-s.ctx.Done():
		return fmt.Errorf("chat session closed")
	case streamChan <- resp:
	}

	if event.IsEnd {
		s.closeOpenClawStream(correlationID)
	}

	return nil
}

// InterruptAndClearTTSQueue Kích hoạt ngắt TTS và xóa hàng đợi gửi (dùng cho các trường hợp như VAD ngắt ở chế độ realtime)
func (s *ChatSession) InterruptAndClearTTSQueue() {
	s.InterruptAndClearTTSQueueWithReason("ChatSession.InterruptAndClearTTSQueue")
}

func (s *ChatSession) InterruptAndClearTTSQueueWithReason(reason string) {
	log.Infof("interrupt and clear tts queue requested: device=%s reason=%s state={%s}", s.clientState.DeviceID, normalizeTTSReason(reason), s.ttsManager.debugState())
	if s.mediaPlayer != nil {
		if err := s.mediaPlayer.Suspend(); err != nil && !errors.Is(err, context.Canceled) {
			log.Warnf("Tạm dừng phát media thất bại: %v", err)
		}
	}
	s.ttsManager.ClearTTSQueue()
	s.ttsManager.InterruptAndStopWithReason(s.clientState.Ctx, true, context.Canceled, reason)
}

// handleAbortMessage Xử lý tin nhắn abort
func (s *ChatSession) HandleAbortMessage(msg *ClientMessage) error {
	s.cancelPendingDetectLLM()

	// Thiết lập trạng thái ngắt
	s.clientState.Abort = true

	if s.clientState.IsRealTime() {
		s.StopAssistantOutputAfterAsrWithReason(true, "HandleAbortMessage realtime")
	} else {
		s.StopSpeakingWithReason(true, "HandleAbortMessage auto")
	}

	// Ghi log
	log.Infof("Thiết bị %s abort session", msg.DeviceID)
	return nil
}

func (s *ChatSession) CheckDeviceActivated() (bool, error) {
	if viper.GetBool("auth.enable") {
		if !s.clientState.IsActivated {
			const falseCheckThrottle = time.Second
			s.activationCheckMu.Lock()
			lastFalseAt := s.lastActivationFalseAt
			s.activationCheckMu.Unlock()
			if !lastFalseAt.IsZero() && time.Since(lastFalseAt) < falseCheckThrottle {
				log.Debugf("Thiết bị %s trạng thái kích hoạt vẫn là chưa kích hoạt, bỏ qua việc xác thực realtime lặp lại", s.clientState.DeviceID)
				return false, nil
			}

			configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
			if err != nil {
				log.Errorf("Lấy config provider thất bại: %v", err)
				return false, err
			}
			//Gọi API để xác nhận lại trạng thái kích hoạt
			isActivated, err := configProvider.IsDeviceActivated(s.clientState.Ctx, s.clientState.DeviceID, "client_id")
			if err != nil {
				log.Errorf("Lấy trạng thái kích hoạt thất bại: %v", err)
				return false, err
			}
			if isActivated {
				s.clientState.IsActivated = true
				s.activationCheckMu.Lock()
				s.lastActivationFalseAt = time.Time{}
				s.activationCheckMu.Unlock()
			} else {
				s.activationCheckMu.Lock()
				s.lastActivationFalseAt = time.Now()
				s.activationCheckMu.Unlock()
				s.HandleNotActivated()
				return false, nil
			}
		}
	}
	return true, nil
}

func (s *ChatSession) HandleListenStart(msg *ClientMessage) error {
	s.cancelPendingDetectLLM()

	// Kiểm tra trạng thái kích hoạt trước
	isActivated, err := s.CheckDeviceActivated()
	if err != nil {
		log.Errorf("Kiểm tra trạng thái kích hoạt thiết bị thất bại: %v", err)
		return err
	}
	if !isActivated {
		return nil
	}

	now := time.Now()
	prevHistory := s.clientState.GetCommandHistorySnapshot()

	// Ở chế độ auto/manual, trong lúc phát lời chào thiết bị có thể tự động gửi lại listen start;
	// loại gói tin này không nên chiếm quyền lời chào, do đó khi lời chào vẫn đang phát thì bỏ qua trực tiếp.
	if shouldIgnoreListenStartDuringWelcome(msg.Mode, s.clientState.IsWelcomePlaying) {
		log.Infof("Thiết bị %s đang phát lời chào, bỏ qua listen start: history={%s}", msg.DeviceID, prevHistory.DebugString(now))
		return nil
	}

	log.Debugf(
		"ListenStart recv: device=%s mode=%s history={%s} welcomeSpeaking=%v welcomePlaying=%v phase=%s",
		msg.DeviceID,
		msg.Mode,
		prevHistory.DebugString(now),
		s.clientState.IsWelcomeSpeaking,
		s.clientState.IsWelcomePlaying,
		s.clientState.GetListenPhase(),
	)

	// Mục tiêu xử lý của realtime và auto/manual khác nhau:
	// realtime giống như "phiên theo dõi thường trực", cố gắng không ngắt luồng hiện tại;
	// auto/manual giống như "mở một vòng thu âm chính thức mới", sẽ reset output hiện tại và khởi động lại ASR.
	if msg.Mode == "realtime" {
		// Khi phiên realtime listen hiện tại chưa đi đến listen stop / session cancel / close,
		// các gói listen start lặp lại đều được bỏ qua âm thầm, tránh ngắt luồng hiện tại.
		if s.clientState.IsRealTime() && s.isRealtimeListenSessionActive() {
			return nil
		}

		// Khi realtime lần đầu vào, nếu lời chào vẫn đang phát thì chờ nó kết thúc tự nhiên;
		// chỉ khi lời chào phát xong hoàn toàn mới tiếp tục vào realtime listen.
		if shouldWaitRealtimeListenStartDuringWelcome(msg.Mode, s.clientState.IsWelcomePlaying) {
			if !s.waitForWelcomePlaybackCompletion() {
				return nil
			}
		}

		s.clientState.RecordCommandArrival(CommandTypeListenStart, now)
		if shouldInterruptOutputOnListenStart(msg.Mode, s.clientState.IsWelcomePlaying) {
			// Trong trường hợp không bảo vệ lời chào, listen start đại diện cho việc tiếp quản theo dõi vòng mới,
			// cần chủ động dừng TTS/LLM hiện tại, tránh việc nói và nghe diễn ra đồng thời.
			s.StopSpeakingWithReason(true, fmt.Sprintf("HandleListenStart mode=%s", msg.Mode))
		}

		s.clientState.ListenMode = msg.Mode
		log.Infof("Thiết bị %s chế độ thu âm: %s", msg.DeviceID, msg.Mode)

		shouldStartAudioIdleWindow := s.clientState.GetListenPhase() != ListenPhaseListening
		startSeq := s.beginListenStart()
		go func() {
			if err := s.OnListenStart(startSeq, shouldStartAudioIdleWindow); err != nil {
				log.Errorf("Thiết bị %s khởi động listen start thất bại: %v", msg.DeviceID, err)
			}
		}()
		return nil
	}

	if s.clientState.GetListenPhase() == ListenPhaseStarting {
		log.Infof("Thiết bị %s listen start đang khởi động, bỏ qua listen start lặp lại", msg.DeviceID)
		return nil
	}

	s.clientState.RecordCommandArrival(CommandTypeListenStart, now)

	// Khi chế độ auto/manual vào đây, được coi là mở tường minh một vòng thu âm mới:
	// cập nhật mode, dừng output cũ, sau đó khởi động bất đồng bộ OnListenStart để khởi tạo ASR.
	s.clientState.ListenMode = msg.Mode
	log.Infof("Thiết bị %s chế độ thu âm: %s", msg.DeviceID, msg.Mode)
	s.StopSpeakingWithReason(true, fmt.Sprintf("HandleListenStart mode=%s", msg.Mode))

	startSeq := s.beginListenStart()
	go func() {
		if err := s.OnListenStart(startSeq, true); err != nil {
			log.Errorf("Thiết bị %s khởi động listen start thất bại: %v", msg.DeviceID, err)
		}
	}()

	return nil
}

func (s *ChatSession) HandleListenStop() error {
	s.cancelPendingDetectLLM()
	s.clientState.RecordCommandArrival(CommandTypeListenStop, time.Now())
	/*if s.clientState.ListenMode == "auto" {
		s.clientState.CancelSessionCtx()
	}*/

	//Gọi
	if s.clientState.IsRealTime() {
		s.invalidateListenStart()
	}
	s.clientState.OnManualStop()

	return nil
}

func (s *ChatSession) OnListenStart(startSeq uint64, shouldStartAudioIdleWindow bool) error {
	log.Debugf("OnListenStart start")
	defer log.Debugf("OnListenStart end")

	if !s.isCurrentListenStart(startSeq) {
		log.Debugf("OnListenStart stale before init, skip")
		return nil
	}

	select {
	case <-s.clientState.Ctx.Done():
		log.Debugf("OnListenStart Ctx done, return")
		if s.isCurrentListenStart(startSeq) {
			s.clientState.SetListenPhase(ListenPhaseIdle)
		}
		return nil
	default:
	}

	// Chế độ realtime: bỏ qua Destroy, giữ ASR chạy liên tục, nhưng xóa AudioBuffer
	var ctx context.Context
	if s.clientState.IsRealTime() {
		s.clientState.AsrAudioBuffer.ClearAsrAudioData()
	} else {
		s.stopSpeakingMu.Lock()
		if !s.isCurrentListenStart(startSeq) {
			s.stopSpeakingMu.Unlock()
			log.Debugf("OnListenStart stale before destroy, skip")
			return nil
		}
		s.clientState.Destroy()
		if !s.isCurrentListenStart(startSeq) {
			s.stopSpeakingMu.Unlock()
			log.Debugf("OnListenStart stale after destroy, skip")
			return nil
		}

		s.clientState.SetListenPhase(ListenPhaseStarting)
		s.clientState.SetStatus(ClientStatusListening)
		ctx = s.clientState.SessionCtx.Get(s.clientState.Ctx)

		// Việc khởi tạo trạng thái liên quan đến ASR cần nhất quán với việc tái tạo context của session.
		if s.clientState.ListenMode == "manual" {
			s.clientState.VoiceStatus.SetClientHaveVoice(true)
		}
		s.stopSpeakingMu.Unlock()
	}

	if s.clientState.IsRealTime() {
		s.clientState.SetListenPhase(ListenPhaseStarting)
		s.clientState.SetStatus(ClientStatusListening)
		ctx = s.clientState.SessionCtx.Get(s.clientState.Ctx)

		//Khởi tạo các thành phần liên quan đến asr
		if s.clientState.ListenMode == "manual" {
			s.clientState.VoiceStatus.SetClientHaveVoice(true)
		}
	}

	// Khởi động nhận diện streaming asr, tái sử dụng hàm restartAsrRecognition
	if !s.isCurrentListenStart(startSeq) {
		log.Debugf("OnListenStart stale before ASR restart, skip")
		return nil
	}
	err := s.asrManager.RestartAsrRecognition(ctx)
	if err != nil {
		if s.shouldIgnoreListenStartError(startSeq, ctx, err) {
			log.Infof("OnListenStart interrupted during ASR restart, ignore err: %v", err)
			if s.isCurrentListenStart(startSeq) {
				s.clientState.SetListenPhase(ListenPhaseIdle)
			}
			return nil
		}

		log.Errorf("Nhận diện streaming asr thất bại: %v", err)
		if s.isCurrentListenStart(startSeq) {
			s.clientState.SetListenPhase(ListenPhaseIdle)
		}
		s.CloseWithReason(chatSessionCloseReasonFatalError)
		return err
	}

	if !s.isCurrentListenStart(startSeq) {
		log.Debugf("OnListenStart stale after ASR restart, cancel current start")
		s.clientState.Asr.CancelWithReason("ChatSession.OnListenStart: stale listen start after ASR restart")
		return nil
	}

	s.clientState.SetListenPhase(ListenPhaseListening)
	if shouldStartAudioIdleWindow {
		s.clientState.StartAudioIdleWindow(time.Now())
	}

	// Định nghĩa callback lưu tin nhắn
	onMessageSave := func(userMsg *schema.Message, messageID string, audioData []float32) {
		// Lấy văn bản ASR và audio đồng thời, lưu một lần (không cần hai giai đoạn)
		eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
			ClientState: s.clientState,
			Msg:         *userMsg,
			MessageID:   messageID,
			AudioData:   [][]byte{util.Float32SliceToBytes(audioData)}, // Chuyển đổi thành mảng byte
			AudioSize:   len(audioData) * 4,                            // float32 = 4 bytes
			SampleRate:  s.clientState.InputAudioFormat.SampleRate,
			Channels:    s.clientState.InputAudioFormat.Channels,
			IsUpdate:    false, // Lưu một lần (văn bản + audio)
			Timestamp:   time.Now(),
		})
	}

	// Định nghĩa callback xử lý lỗi
	onError := func(err error) {
		if s.shouldIgnoreAsrLoopError(startSeq, ctx, err) {
			log.Infof("Vòng lặp nhận diện ASR kết thúc trong lúc reset/thoát, bỏ qua err: %v", err)
			return
		}
		log.Errorf("Lỗi vòng lặp nhận diện ASR: %v", err)
		s.CloseWithReason(chatSessionCloseReasonFatalError)
	}

	// Khởi động vòng lặp xử lý kết quả nhận diện ASR (quản lý tài nguyên nằm bên trong ASRManager)
	s.asrManager.StartAsrRecognitionLoop(ctx, onMessageSave, onError)

	return nil
}

// startChat Bắt đầu hội thoại
func (s *ChatSession) AddAsrResultToQueue(text string, speakerResult *speaker.IdentifyResult) error {
	return s.AddAsrResultToQueueWithOptions(text, speakerResult, llmResponseChannelOptions{})
}

func (s *ChatSession) AddAsrResultToQueueWithOptions(text string, speakerResult *speaker.IdentifyResult, options llmResponseChannelOptions) error {
	log.Debugf("AddAsrResultToQueue text: %s", text)
	if speakerResult != nil && speakerResult.Identified {
		log.Debugf("AddAsrResultToQueue speaker: %s (confidence: %.2f)", speakerResult.SpeakerName, speakerResult.Confidence)
	}

	// Kiểm tra xem session đã bị dừng hay chưa (thông qua việc thử lấy lock để xác định)
	// Nếu StopSpeaking đang thực thi, ở đây sẽ chờ; nếu đã thực thi xong, tryLock sẽ trả về ngay lập tức
	if !s.stopSpeakingMu.TryLock() {
		log.Debugf("AddAsrResultToQueue đang thực thi StopSpeaking, bỏ tin nhắn")
		return nil
	}
	s.stopSpeakingMu.Unlock()

	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	// Kiểm tra xem sessionCtx đã bị hủy chưa
	if sessionCtx.Err() != nil {
		log.Debugf("AddAsrResultToQueue sessionCtx đã bị hủy, bỏ tin nhắn")
		return nil
	}
	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	ctx = withTTSPlaybackStartHook(ctx, options.onTTSPlaybackStart)
	ctx = withTTSTurnEndPolicy(ctx, options.ttsTurnEndPolicy)

	item := AsrResponseChannelItem{
		ctx:           ctx,
		text:          text,
		speakerResult: speakerResult,
	}
	err := s.chatTextQueue.Push(item)
	if err != nil {
		log.Warnf("chatTextQueue đã đầy hoặc đã đóng, bỏ tin nhắn")
	}
	return nil
}

func (s *ChatSession) processChatText(ctx context.Context) {
	log.Debugf("processChatText start")
	defer log.Debugf("processChatText end")

	for {
		item, err := s.chatTextQueue.Pop(ctx, 0)
		if err != nil {
			if err == util.ErrQueueCtxDone {
				return
			}
			continue
		}

		err = s.actionDoChat(item.ctx, item.text, item.speakerResult)
		if err != nil {
			log.Errorf("Xử lý hội thoại thất bại: %v", err)
			continue
		}
	}
}

func (s *ChatSession) ClearChatTextQueue() {
	s.chatTextQueue.Clear()
}

// DoExitChat Thực thi logic thoát hội thoại (gửi lời tạm biệt và đóng session)
func (s *ChatSession) DoExitChat() {
	// Lời tạm biệt thân thiện
	goodbyeText := "Vâng, tạm biệt! Hẹn gặp lại bạn lần sau nhé~"

	// Lưu một tin nhắn với vai trò assistant
	goodbyeMsg := schema.AssistantMessage(goodbyeText, nil)
	if err := s.llmManager.AddLlmMessage(s.clientState.Ctx, goodbyeMsg); err != nil {
		log.Errorf("Lưu tin nhắn tạm biệt thất bại: %v", err)
	}

	// Lấy context
	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)

	// Gửi TTS lời tạm biệt
	s.ttsManager.EnqueueTtsStartWithReason(ctx, "ChatSession.processGoodbye")

	err := s.ttsManager.handleTextResponse(ctx, llm_common.LLMResponseStruct{
		Text:    goodbyeText,
		IsStart: true,
		IsEnd:   true,
	}, true) // Xử lý đồng bộ, chờ TTS hoàn tất

	if err != nil {
		log.Errorf("Gửi lời tạm biệt thất bại: %v", err)
	}

	s.ttsManager.RequestTurnEnd(ctx, err)
	s.ttsManager.EnqueueTtsStopWithReason(ctx, "ChatSession.processGoodbye")
	// Đóng session
	s.CloseWithReason(chatSessionCloseReasonExplicitExit)
}

func (s *ChatSession) Close() {
	s.CloseWithReason(chatSessionCloseReasonManagerShutdown)
}

func (s *ChatSession) IsClosing() bool {
	if s == nil {
		return true
	}
	return s.closing.Load()
}

func (s *ChatSession) CloseWithReason(reason string) {
	s.closing.Store(true)
	s.closeOnce.Do(func() {
		// Dọn dẹp tài nguyên ASR (quản lý tài nguyên nằm bên trong ASRManager)
		if s.asrManager != nil {
			s.asrManager.Cleanup()
		}
		deviceID := ""
		if s.clientState != nil {
			deviceID = s.clientState.DeviceID
		}
		log.Debugf("ChatSession.Close() bắt đầu dọn dẹp tài nguyên session, thiết bị %s", deviceID)

		if s.mediaPlayer != nil {
			s.mediaPlayer.DetachSession(true)
		}

		s.cancelPendingDetectLLM()

		// Hủy context ở cấp session
		if s.cancel != nil {
			s.cancel()
		}
		s.finishOpenClawWarmup("", false)

		// Dọn dẹp hàng đợi văn bản chat
		s.ClearChatTextQueue()
		s.clearOpenClawStreams()

		// Dừng nói và dọn dẹp tài nguyên liên quan đến audio. Trước đó trong luồng Close đã DetachSession(true),
		// ở đây không nên Suspend media lần nữa, nếu không sẽ xóa mất resumeOnAttach.
		s.stopSpeakingWithLock(true, true, false, "ChatSession.Close")

		if s.speakerManager != nil {
			s.speakerManager.Close()
		}

		if s.clientState != nil {
			eventbus.Get().Publish(eventbus.TopicSessionEnd, s.clientState)
		}

		log.Debugf("ChatSession.Close() dọn dẹp tài nguyên session hoàn tất, thiết bị %s", deviceID)

		if s.closeHandler != nil {
			s.closeHandler(s, reason)
		}
	})
}

func (s *ChatSession) actionDoChat(ctx context.Context, text string, speakerResult *speaker.IdentifyResult) error {
	select {
	case <-ctx.Done():
		log.Debugf("actionDoChat ctx done, return")
		return nil
	default:
	}

	agentID := strings.TrimSpace(s.clientState.AgentID)
	deviceID := strings.TrimSpace(s.clientState.DeviceID)
	openclawSessionID := strings.TrimSpace(s.clientState.SessionID)
	trimmedText := strings.TrimSpace(text)

	handledByRealtimeGate, gateErr := s.tryHandleRealtimeMcpAudioASR(ctx, trimmedText)
	if handledByRealtimeGate {
		return gateErr
	}

	openclawManager := openclaw.GetManager()
	if s.clientState.DeviceConfig.OpenClaw.Allowed {
		isOpenClawMode := openclawManager.IsModeEnabled(agentID, deviceID)
		isEnterKeyword := s.isOpenClawEnterKeyword(text)
		isExitKeyword := false
		if isOpenClawMode {
			isExitKeyword = s.isOpenClawExitKeyword(text)
		}
		log.Debugf(
			"Phán định route OpenClaw: agent=%s device=%s session=%s allowed=%v mode=%v enter_keyword=%v exit_keyword=%v text_len=%d text_trim_len=%d text_snippet=%q",
			agentID,
			deviceID,
			openclawSessionID,
			s.clientState.DeviceConfig.OpenClaw.Allowed,
			isOpenClawMode,
			isEnterKeyword,
			isExitKeyword,
			len(text),
			len(trimmedText),
			openClawLogSnippet(trimmedText, 64),
		)
		if isOpenClawMode {
			if isExitKeyword {
				s.finishOpenClawWarmup("", true)
				exited := openclawManager.ExitMode(agentID, deviceID)
				_ = s.AddTextToTTSQueue("Đã thoát chế độ OpenClaw")
				log.Infof("Thiết bị %s thoát chế độ OpenClaw: agent=%s exited=%v", deviceID, agentID, exited)
				return nil
			}

			log.Infof(
				"OpenClaw gửi STT: agent=%s device=%s session=%s text_len=%d text_snippet=%q",
				agentID,
				deviceID,
				openclawSessionID,
				len(trimmedText),
				openClawLogSnippet(trimmedText, 64),
			)
			s.finishOpenClawWarmup("", true)
			messageID, err := openclawManager.SendMessage(
				agentID,
				deviceID,
				text,
				openclawSessionID,
			)
			if err != nil {
				log.Warnf(
					"Thiết bị %s gửi tin nhắn OpenClaw thất bại, đã quay lại chế độ thông thường: agent=%s session=%s text_snippet=%q err=%v",
					deviceID,
					agentID,
					openclawSessionID,
					openClawLogSnippet(trimmedText, 64),
					err,
				)
				openclawManager.ExitMode(agentID, deviceID)
				_ = s.AddTextToTTSQueue("OpenClaw hiện không khả dụng, đã thoát chế độ OpenClaw")
			} else {
				s.startOpenClawWarmup(messageID, text)
				log.Infof("OpenClaw gửi STT thành công: agent=%s device=%s session=%s message_id=%s", agentID, deviceID, openclawSessionID, messageID)
			}
			return nil
		}

		if isEnterKeyword {
			if !openclawManager.EnterMode(agentID, deviceID) {
				_ = s.AddTextToTTSQueue("OpenClaw hiện không khả dụng, vui lòng thử lại sau")
				log.Warnf("Thiết bị %s vào chế độ OpenClaw thất bại: agent=%s agent session not ready", deviceID, agentID)
				return nil
			}
			_ = s.AddTextToTTSQueue("Đã vào chế độ OpenClaw, vui lòng tiếp tục nói")
			log.Infof("Thiết bị %s vào chế độ OpenClaw: agent=%s trigger=%q", deviceID, agentID, openClawLogSnippet(trimmedText, 32))
			return nil
		}
		log.Debugf(
			"OpenClaw không tiếp quản STT hiện tại: agent=%s device=%s mode=%v enter_keyword=%v text_snippet=%q",
			agentID,
			deviceID,
			isOpenClawMode,
			isEnterKeyword,
			openClawLogSnippet(trimmedText, 64),
		)
	} else {
		s.finishOpenClawWarmup("", false)
		if openclawManager.ExitMode(agentID, deviceID) {
			log.Debugf("Cấu hình OpenClaw chưa được bật, đã buộc thoát chế độ: agent=%s device=%s", agentID, deviceID)
		}
	}

	if s.checkExitWords(text) {
		// Phát sự kiện thoát chat
		eventbus.Get().Publish(eventbus.TopicExitChat, &eventbus.ExitChatEvent{
			ClientState: s.clientState,
			Reason:      "Người dùng chủ động thoát",
			TriggerType: "exit_words",
			UserText:    text,
			Timestamp:   time.Now(),
		})
		return nil
	}

	clientState := s.clientState

	sessionID := clientState.SessionID

	// Chuyển đổi TTS động sau khi nhận diện vân giọng nói (khôi phục TTS mặc định khi không nhận diện được)
	if err := s.switchTTSForSpeaker(speakerResult); err != nil {
		log.Warnf("Chuyển đổi TTS thất bại: %v", err)
		// Không ngắt luồng, tiếp tục sử dụng TTS hiện tại
	}

	// Tạo trực tiếp tin nhắn gốc của Eino
	userMessage := &schema.Message{
		Role:    schema.User,
		Content: text,
	}

	// Lấy danh sách tool MCP toàn cục
	mcpTools, err := mcp.GetToolsByDeviceIdWithTransport(
		clientState.DeviceID,
		clientState.AgentID,
		s.serverTransport.GetTransportType(),
		clientState.DeviceConfig.MCPServiceNames,
	)
	if err != nil {
		log.Errorf("Lấy tool của thiết bị %s thất bại: %v", clientState.DeviceID, err)
		mcpTools = make(map[string]tool.InvokableTool)
	}
	if !hasAvailableKnowledgeBase(clientState.DeviceConfig.KnowledgeBases) {
		if _, ok := mcpTools["search_knowledge"]; ok {
			delete(mcpTools, "search_knowledge")
			log.Infof("Thiết bị %s không liên kết với knowledge base khả dụng, đã loại bỏ tool search_knowledge", clientState.DeviceID)
		}
	}

	// Chuyển đổi tool MCP sang định dạng interface để truyền cho hàm chuyển đổi
	mcpToolsInterface := make(map[string]interface{})
	for name, tool := range mcpTools {
		mcpToolsInterface[name] = tool
	}

	// Chuyển đổi tool MCP sang định dạng Eino ToolInfo
	einoTools, err := llm.ConvertMCPToolsToEinoTools(ctx, mcpToolsInterface)
	if err != nil {
		log.Errorf("Chuyển đổi tool MCP thất bại: %v", err)
		einoTools = nil
	}

	toolNameList := make([]string, 0)
	for _, tool := range einoTools {
		toolNameList = append(toolNameList, tool.Name)
	}

	// Gửi request LLM kèm tool
	log.Infof("Sử dụng %d tool MCP để gửi request LLM, tools: %+v", len(einoTools), toolNameList)

	err = s.llmManager.DoLLmRequest(ctx, userMessage, einoTools, true, speakerResult)
	if err != nil {
		log.Errorf("Gửi request LLM kèm tool thất bại, seesionID: %s, error: %v", sessionID, err)
		return fmt.Errorf("gửi request LLM kèm tool thất bại: %v", err)
	}
	return nil
}

func hasAvailableKnowledgeBase(knowledgeBases []types.KnowledgeBaseRef) bool {
	for _, kb := range knowledgeBases {
		if strings.EqualFold(strings.TrimSpace(kb.Status), "inactive") {
			continue
		}
		if strings.TrimSpace(kb.ExternalKBID) == "" {
			continue
		}
		return true
	}
	return false
}

func (s *ChatSession) MarkTurnSpeakerInterrupted() {
	if s == nil {
		return
	}
	s.turnSpeakerInterrupted.Store(true)
}

func (s *ChatSession) ConsumeTurnSpeakerInterrupted() bool {
	if s == nil {
		return false
	}
	return s.turnSpeakerInterrupted.Swap(false)
}

func (s *ChatSession) ResetTurnSpeakerInterrupted() {
	if s == nil {
		return
	}
	s.turnSpeakerInterrupted.Store(false)
}

func (s *ChatSession) ShouldAllowSpeakerChat(speakerResult *speaker.IdentifyResult, speakerInterrupted bool) (bool, string) {
	if s == nil || s.clientState == nil {
		return true, ""
	}

	matchedConfiguredSpeaker := s.clientState.HasMatchedConfiguredSpeaker(speakerResult)
	if speakerInterrupted && !matchedConfiguredSpeaker {
		return false, "speaker_interrupt_without_identify"
	}

	if s.clientState.RequireMatchedSpeakerForChat() && !matchedConfiguredSpeaker {
		return false, "speaker_chat_mode_identified_only_not_matched"
	}

	return true, ""
}

// switchTTSForSpeaker Chuyển đổi TTS cho người nói đã được nhận diện
func (s *ChatSession) switchTTSForSpeaker(speakerResult *speaker.IdentifyResult) error {
	s.clientState.SpeakerTTSConfig = nil

	// 1. Kiểm tra speakerResult có phải là nil hay không
	if speakerResult == nil {
		log.Debug("speakerResult là nil, xóa cấu hình TTS theo vân giọng nói")
		return nil
	}

	// 2. Tìm cấu hình nhóm vân giọng nói
	speakerGroupInfo, found := s.clientState.DeviceConfig.VoiceIdentify[speakerResult.SpeakerName]
	if !found {
		// Không tìm thấy cấu hình, xóa cấu hình TTS theo vân giọng nói
		log.Debugf("Không tìm thấy cấu hình của nhóm vân giọng nói %s, xóa cấu hình TTS theo vân giọng nói", speakerResult.SpeakerName)
		return nil
	}

	// 3. Kiểm tra xem đã cấu hình giọng đọc tùy chỉnh chưa
	if speakerGroupInfo.TTSConfigID == nil || *speakerGroupInfo.TTSConfigID == "" {
		// Chưa cấu hình giọng đọc tùy chỉnh, xóa cấu hình TTS theo vân giọng nói
		log.Debugf("Nhóm vân giọng nói %s chưa cấu hình TTS tùy chỉnh, xóa cấu hình TTS theo vân giọng nói", speakerResult.SpeakerName)
		return nil
	}

	// 4. Tìm cấu hình TTS tương ứng từ cấu hình hệ thống (viper)
	var targetTTSConfig *types.TtsConfigItem
	ttsConfigsRaw := viper.Get("tts")
	if ttsConfigsRaw == nil {
		return fmt.Errorf("không tìm thấy tts trong cấu hình hệ thống")
	}

	// Phân tích cấu hình tts (hiện tại là một map, key là config_id)
	if ttsConfigsMap, ok := ttsConfigsRaw.(map[string]interface{}); ok {
		// Tìm config_id khớp
		if configItem, exists := ttsConfigsMap[*speakerGroupInfo.TTSConfigID]; exists {
			if configMap, ok := configItem.(map[string]interface{}); ok {
				// Phân tích mục cấu hình
				ttsItem := &types.TtsConfigItem{
					ConfigID: *speakerGroupInfo.TTSConfigID,
				}
				if name, ok := configMap["name"].(string); ok {
					ttsItem.Name = name
				}
				if provider, ok := configMap["provider"].(string); ok {
					ttsItem.Provider = provider
				}
				if isDefault, ok := configMap["is_default"].(bool); ok {
					ttsItem.IsDefault = isDefault
				}
				// Các trường khác của mục cấu hình được dùng trực tiếp làm config
				ttsItem.Config = make(map[string]interface{})
				for k, v := range configMap {
					if k != "name" && k != "provider" && k != "is_default" && k != "config_id" {
						ttsItem.Config[k] = v
					}
				}
				targetTTSConfig = ttsItem
			}
		}
	}

	if targetTTSConfig == nil {
		return fmt.Errorf("không tìm thấy cấu hình TTS %s", *speakerGroupInfo.TTSConfigID)
	}

	// 5. Sao chép cấu hình TTS để tránh chỉnh sửa cấu hình gốc
	ttsConfig := make(map[string]interface{})
	for k, v := range targetTTSConfig.Config {
		ttsConfig[k] = v
	}

	// 6. Nếu đã cấu hình giá trị giọng đọc, ghi đè vào cấu hình TTS
	if speakerGroupInfo.Voice != nil && *speakerGroupInfo.Voice != "" {
		// Thiết lập trường giọng đọc tương ứng theo provider
		if targetTTSConfig.Provider == "cosyvoice" {
			ttsConfig["spk_id"] = *speakerGroupInfo.Voice
		} else {
			ttsConfig["voice"] = *speakerGroupInfo.Voice
		}
		log.Debugf("Thiết lập giọng đọc cho người nói %s: %s", speakerResult.SpeakerName, *speakerGroupInfo.Voice)
	}
	if targetTTSConfig.Provider == "aliyun_qwen" &&
		speakerGroupInfo.VoiceModelOverride != nil &&
		strings.TrimSpace(*speakerGroupInfo.VoiceModelOverride) != "" {
		overrideModel := strings.TrimSpace(*speakerGroupInfo.VoiceModelOverride)
		ttsConfig["model"] = overrideModel
		log.Debugf("Ghi đè model Qwen cho người nói %s: %s", speakerResult.SpeakerName, overrideModel)
	}

	// 7. Lưu cấu hình TTS đầy đủ (deep copy)
	s.clientState.SpeakerTTSConfig = make(map[string]interface{})
	for k, v := range ttsConfig {
		s.clientState.SpeakerTTSConfig[k] = v
	}
	// Đảm bảo provider có trong config
	s.clientState.SpeakerTTSConfig["provider"] = targetTTSConfig.Provider

	log.Infof("✅ Chuyển đổi cấu hình TTS cho người nói %s thành công - Provider: %s, ConfigID: %s, Voice: %v",
		speakerResult.SpeakerName,
		targetTTSConfig.Provider,
		targetTTSConfig.ConfigID,
		speakerGroupInfo.Voice)

	return nil
}

func (s *ChatSession) hookContext(ctx context.Context) chathooks.Context {
	sessionID := ""
	deviceID := ""
	if s != nil && s.clientState != nil {
		sessionID = s.clientState.SessionID
		deviceID = s.clientState.DeviceID
	}

	return chathooks.Context{
		Ctx:       ctx,
		SessionID: sessionID,
		DeviceID:  deviceID,
	}
}

func (s *ChatSession) emitMetricStage(ctx context.Context, stage chathooks.MetricStage, ts int64, err error) {
	if s == nil {
		return
	}

	hookErr := s.hookHub.EmitMetric(s.hookContext(ctx), chathooks.MetricData{Stage: stage, Ts: ts, Err: err})
	if hookErr != nil {
		log.Warnf("METRIC hook thực thi thất bại: stage=%s err=%v", stage, hookErr)
	}
}

func (s *ChatSession) TraceTurnStart(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricTurnStart, ts, nil)
}

func (s *ChatSession) TraceTurnEnd(ctx context.Context, ts int64, err error) {
	s.emitMetricStage(ctx, chathooks.MetricTurnEnd, ts, err)
}

func (s *ChatSession) TraceVoiceSilence(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricVoiceSilence, ts, nil)
}

func (s *ChatSession) TraceAsrFirstText(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricAsrFirstText, ts, nil)
}

func (s *ChatSession) TraceAsrFinalText(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricAsrFinalText, ts, nil)
}

func (s *ChatSession) TraceLlmStart(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricLlmStart, ts, nil)
}

func (s *ChatSession) TraceLlmFirstToken(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricLlmFirstToken, ts, nil)
}

func (s *ChatSession) TraceLlmFirstSentence(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricLlmFirstSentence, ts, nil)
}

func (s *ChatSession) TraceLlmEnd(ctx context.Context, ts int64, err error) {
	s.emitMetricStage(ctx, chathooks.MetricLlmEnd, ts, err)
}

func (s *ChatSession) TraceTtsStart(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricTtsStart, ts, nil)
}

func (s *ChatSession) TraceTtsFirstFrame(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricTtsFirstFrame, ts, nil)
}

func (s *ChatSession) TraceTtsStop(ctx context.Context, ts int64, err error) {
	s.emitMetricStage(ctx, chathooks.MetricTtsStop, ts, err)
}