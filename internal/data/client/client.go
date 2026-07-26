package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"sync"

	utypes "milestones-esp32-server-golang/internal/domain/config/types"
	"milestones-esp32-server-golang/internal/domain/llm"
	llm_common "milestones-esp32-server-golang/internal/domain/llm/common"
	"milestones-esp32-server-golang/internal/domain/memory"
	"milestones-esp32-server-golang/internal/domain/speaker"
	"milestones-esp32-server-golang/internal/domain/tts"

	. "milestones-esp32-server-golang/internal/data/audio"

	log "milestones-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

// Dialogue đại diện cho lịch sử hội thoại
type Dialogue struct {
	mu       sync.RWMutex // Khóa đọc/ghi bảo vệ Messages
	Messages []*schema.Message
}

const (
	ClientStatusInit       = "init"
	ClientStatusListening  = "listening"
	ClientStatusListenStop = "listenStop"
	ClientStatusLLMStart   = "llmStart"
	ClientStatusTTSStart   = "ttsStart"

	ListenPhaseIdle      = "idle"
	ListenPhaseStarting  = "starting"
	ListenPhaseListening = "listening"

	CommandTypeDetect      = "detect"
	CommandTypeListenStart = "listen_start"
	CommandTypeListenStop  = "listen_stop"

	MemoryModeNone  = "none"
	MemoryModeShort = "short"
	MemoryModeLong  = "long"

	SpeakerChatModeOff            = "off"
	SpeakerChatModeIdentifiedOnly = "identified_only"
)

func NormalizeMemoryMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case MemoryModeNone:
		return MemoryModeNone
	case MemoryModeLong:
		return MemoryModeLong
	default:
		return MemoryModeShort
	}
}

func NormalizeSpeakerChatMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case SpeakerChatModeIdentifiedOnly:
		return SpeakerChatModeIdentifiedOnly
	default:
		return SpeakerChatModeOff
	}
}

type SendAudioData func(audioData []byte) error

// ClientState đại diện cho trạng thái của client
type ClientState struct {
	cmdMu sync.Mutex

	IsActivated bool
	// Lịch sử hội thoại
	Dialogue *Dialogue
	// Trạng thái ngắt
	Abort bool
	// Chế độ thu âm
	ListenMode string
	// Trạng thái luồng listen start: idle / starting / listening
	ListenPhase string
	// ID thiết bị
	DeviceID string
	AgentID  string
	// ID phiên
	SessionID string

	//Cấu hình thiết bị
	DeviceConfig utypes.UConfig

	Vad
	Asr
	Llm

	// Nhà cung cấp TTS
	TTSProvider      tts.TTSProvider        // Nhà cung cấp TTS mặc định
	SpeakerTTSConfig map[string]interface{} // Cấu hình TTS nhận dạng giọng nói (config đầy đủ, ưu tiên sử dụng)
	// Nhà cung cấp bộ nhớ
	MemoryProvider memory.MemoryProvider
	MemoryContext  string //ngữ cảnh bộ nhớ

	// Kiểm soát ngữ cảnh
	Ctx    context.Context
	Cancel context.CancelFunc

	SessionCtx         Ctx //Ngữ cảnh của một cuộc hội thoại
	AfterAsrSessionCtx Ctx //Ngữ cảnh luồng sau asr

	//prompt, lời nhắc hệ thống
	SystemPrompt string

	InputAudioFormat  AudioFormat //Định dạng âm thanh đầu vào
	OutputAudioFormat AudioFormat //Định dạng âm thanh đầu ra

	// Bộ đệm dữ liệu âm thanh opus nhận được
	OpusAudioBuffer chan []byte

	// Bộ đệm dữ liệu âm thanh pcm nhận được
	AsrAudioBuffer *AsrAudioBuffer

	VoiceStatus
	AudioIdle AudioIdleClock

	UdpSendAudioData SendAudioData //Gửi dữ liệu âm thanh
	Statistic        Statistic     //Thống kê thời gian
	MqttLastActiveTs int64         //Thời gian hoạt động cuối cùng
	VadLastActiveTs  int64         //Thời gian hoạt động vad cuối cùng, quá 60s && không ở trạng thái tts thì ngắt kết nối

	Status string //Trạng thái listening, llmStart, ttsStart

	IsTtsStart        bool //Đã bắt đầu tts chưa
	IsWelcomeSpeaking bool //Đã phát lời chào chưa
	IsWelcomePlaying  bool //Đang phát lời chào

	LastCmdType string
	LastCmdAt   time.Time

	// Nhận dạng giọng nói liên quan
	SpeakerProvider speaker.SpeakerProvider // Nhà cung cấp nhận dạng giọng nói (khởi tạo trong session)

	// Hàm callback bất đồng bộ để lấy kết quả nhận dạng giọng nói (đặt trong session)
	OnVoiceSilenceSpeakerCallback func(ctx context.Context)

	// Hàm callback sự kiện im lặng giọng nói (đặt trong session)
	OnVoiceSilenceMetricCallback func(ctx context.Context, ts int64)

	// Hàm callback khi ASR trả về ký tự đầu tiên (đặt trong session)
	OnAsrFirstTextCallback func(text string, isFinal bool)
}

// IsSpeakerEnabled kiểm tra xem nhận dạng giọng nói có được bật không (đọc từ cấu hình toàn cục)
func (c *ClientState) IsSpeakerEnabled() bool {
	// Lấy trường enable từ cấu hình toàn cục (viper)
	enabled := viper.GetBool("voice_identify.enable")
	return enabled
}

// HasSpeakerGroups kiểm tra xem cấu hình thiết bị có nhóm giọng nói không
func (c *ClientState) HasSpeakerGroups() bool {
	// Kiểm tra xem cấu hình thiết bị có cấu hình nhóm giọng nói không
	return len(c.DeviceConfig.VoiceIdentify) > 0
}

func (c *ClientState) IsRealTime() bool {
	return c.ListenMode == "realtime"
}

func (c *ClientState) GetMemoryMode() string {
	return NormalizeMemoryMode(c.DeviceConfig.MemoryMode)
}

func (c *ClientState) GetSpeakerChatMode() string {
	return NormalizeSpeakerChatMode(c.DeviceConfig.SpeakerChatMode)
}

func (c *ClientState) RequireMatchedSpeakerForChat() bool {
	return c.HasSpeakerGroups() && c.GetSpeakerChatMode() == SpeakerChatModeIdentifiedOnly
}

func (c *ClientState) HasMatchedConfiguredSpeaker(result *speaker.IdentifyResult) bool {
	if result == nil || !result.Identified {
		return false
	}
	_, ok := c.DeviceConfig.VoiceIdentify[result.SpeakerName]
	return ok
}

func (c *ClientState) GetDeviceIDOrAgentID() string {
	if c.AgentID != "" {
		return c.AgentID
	}
	return c.DeviceID
}

// Các phương thức liên quan đến lịch sử tin nhắn bắt đầu
func (c *ClientState) AddMessage(msg *schema.Message) {
	if msg == nil {
		log.Warnf("Cố gắng thêm tin nhắn nil vào lịch sử hội thoại")
		return
	}
	c.Dialogue.mu.Lock()
	defer c.Dialogue.mu.Unlock()

	// Lớp bảo vệ chống lưu trùng: nếu message ngay trước đó (cuối Dialogue.Messages) giống hệt
	// message sắp thêm (cùng Role, cùng Content, cùng tập tool_call_id), bỏ qua không thêm.
	//
	// Lý do: đã quan sát thấy race condition (chưa xác định chính xác điểm gây ra) khiến cùng
	// 1 assistant message mang tool_calls bị lưu 2 lần liên tiếp không có tool-result message
	// chen giữa. Hậu quả: API LLM trả lỗi 400 "insufficient tool messages following tool_calls
	// message", và vì lỗi này được lưu luôn vào lịch sử, MỌI lần gọi lại sau đó trong cùng phiên
	// đều kế thừa lịch sử đã hỏng và lặp lại lỗi y hệt. Việc chặn trùng lặp tại đây là tuyến
	// phòng thủ cuối cùng, an toàn tuyệt đối (message hợp lệ không bao giờ bị chặn nhầm vì so
	// sánh nội dung + role + tool_call_id chính xác), không phụ thuộc vào việc tìm ra nguyên
	// nhân gốc ở tầng gọi LLM/thực thi tool.
	if n := len(c.Dialogue.Messages); n > 0 {
		if isDuplicateMessage(c.Dialogue.Messages[n-1], msg) {
			log.Warnf("Phát hiện tin nhắn trùng lặp liên tiếp (Role=%s), bỏ qua để tránh hỏng lịch sử hội thoại", msg.Role)
			return
		}
	}

	c.Dialogue.Messages = append(c.Dialogue.Messages, msg)
}

// isDuplicateMessage so sánh 2 message có "giống hệt" nhau không, dùng để chặn lưu trùng lặp
// liên tiếp. So sánh Role, Content, và tập hợp tool_call_id (không quan tâm thứ tự) của ToolCalls
// (đối với assistant message) hoặc ToolCallID (đối với tool-result message).
func isDuplicateMessage(prev, next *schema.Message) bool {
	if prev == nil || next == nil {
		return false
	}
	if prev.Role != next.Role || prev.Content != next.Content {
		return false
	}
	if prev.Role == schema.Tool {
		return prev.ToolCallID == next.ToolCallID
	}
	if len(prev.ToolCalls) != len(next.ToolCalls) {
		return false
	}
	if len(prev.ToolCalls) == 0 {
		// Cả 2 đều không có ToolCalls, Role+Content đã khớp -> coi là trùng
		// (áp dụng cho user/assistant message thuần văn bản bị gửi lặp do retry).
		return true
	}
	prevIDs := make(map[string]bool, len(prev.ToolCalls))
	for _, tc := range prev.ToolCalls {
		prevIDs[tc.ID] = true
	}
	for _, tc := range next.ToolCalls {
		if !prevIDs[tc.ID] {
			return false
		}
	}
	return true
}

func (c *ClientState) GetMessages(count int) []*schema.Message {
	c.Dialogue.mu.RLock()
	defer c.Dialogue.mu.RUnlock()

	// Thêm kiểm tra biên, ngăn mảng vượt chỉ mục
	if len(c.Dialogue.Messages) == 0 {
		return []*schema.Message{}
	}

	// Tính toán chỉ mục bắt đầu, đảm bảo không vượt chỉ mục
	startIndex := len(c.Dialogue.Messages) - count
	if startIndex < 0 {
		startIndex = 0
	}

	return AlignToolMessages(c.Dialogue.Messages[startIndex:])
}

/*
func AlignMessage(messages []*schema.Message) []*schema.Message {
	findMsgTypeUser := false
	// Để đảm bảo tính toàn vẹn của tin nhắn, duyệt tìm tin nhắn sau User đầu tiên
	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		if msg == nil {
			continue
		}
		if !findMsgTypeUser {
			if msg.Role == schema.User {
				return messages[i:]
			}
			continue
		}
	}
	return messages
}
*/
// AlignToolMessages đảm bảo tool_call_id trong tin nhắn role:tool khớp với id trong tool_calls của tin nhắn role:assistant
// Nếu không khớp thì xóa tin nhắn tool tương ứng, đồng thời xử lý trường hợp ngược lại không khớp
func AlignToolMessages(messages []*schema.Message) []*schema.Message {
	if len(messages) == 0 {
		return messages
	}

	// Thu thập tất cả id tool_calls từ tin nhắn assistant
	validToolCallIDs := make(map[string]bool)
	// Thu thập tất cả tool_call_id từ tin nhắn tool
	usedToolCallIDs := make(map[string]bool)

	// Duyệt lần đầu: thu thập id tool_calls từ tin nhắn assistant và tool_call_id từ tin nhắn tool
	for _, msg := range messages {
		if msg == nil {
			continue
		}

		if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
			for _, toolCall := range msg.ToolCalls {
				if toolCall.ID != "" {
					validToolCallIDs[toolCall.ID] = true
				}
			}
		}

		if msg.Role == schema.Tool && msg.ToolCallID != "" {
			usedToolCallIDs[msg.ToolCallID] = true
		}
	}

	// Lọc tin nhắn, xử lý trường hợp không khớp hai chiều
	var alignedMessages []*schema.Message
	for _, msg := range messages {
		if msg == nil {
			continue
		}

		// Nếu là tin nhắn tool, kiểm tra tool_call_id có hợp lệ không
		if msg.Role == schema.Tool {
			if msg.ToolCallID != "" && validToolCallIDs[msg.ToolCallID] {
				alignedMessages = append(alignedMessages, msg)
			}
		} else if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
			// Chỉ giữ lại các tool_calls ĐÃ có tin nhắn tool-result tương ứng.
			// Nếu giữ nguyên msg.ToolCalls trong khi thiếu tool-result cho 1 vài phần tử,
			// API sẽ báo lỗi "insufficient tool messages following tool_calls message".
			keptToolCalls := make([]schema.ToolCall, 0, len(msg.ToolCalls))
			for _, toolCall := range msg.ToolCalls {
				if toolCall.ID != "" && usedToolCallIDs[toolCall.ID] {
					keptToolCalls = append(keptToolCalls, toolCall)
				}
			}

			if len(keptToolCalls) == 0 {
				// Không còn tool_call nào hợp lệ.
				// Nếu message cũng không có nội dung text, bỏ hẳn (assistant message rỗng gây lỗi API).
				if strings.TrimSpace(msg.Content) == "" {
					continue
				}
				// Còn nội dung text: giữ lại message nhưng loại bỏ ToolCalls (tạo bản sao, không sửa msg gốc).
				msgCopy := *msg
				msgCopy.ToolCalls = nil
				alignedMessages = append(alignedMessages, &msgCopy)
				continue
			}

			if len(keptToolCalls) != len(msg.ToolCalls) {
				// Một phần tool_calls không có tool-result khớp: tạo bản sao chỉ giữ phần hợp lệ,
				// tránh sửa trực tiếp msg.ToolCalls (con trỏ này có thể đang được dùng chung ở nơi khác).
				msgCopy := *msg
				msgCopy.ToolCalls = keptToolCalls
				alignedMessages = append(alignedMessages, &msgCopy)
			} else {
				// Tất cả tool_calls đều hợp lệ: giữ nguyên message, append đúng 1 lần.
				alignedMessages = append(alignedMessages, msg)
			}
		} else {
			// Các loại tin nhắn khác giữ nguyên
			alignedMessages = append(alignedMessages, msg)
		}
	}

	return alignedMessages
}

func (c *ClientState) InitMessages(messages []*schema.Message) error {
	c.Dialogue.mu.Lock()
	defer c.Dialogue.mu.Unlock()
	c.Dialogue.Messages = AlignToolMessages(messages)
	return nil
}

// Các phương thức liên quan đến lịch sử tin nhắn kết thúc

func (c *ClientState) SetTtsStart(isStart bool) {
	c.IsTtsStart = isStart
}

func (c *ClientState) GetTtsStart() bool {
	return c.IsTtsStart
}

func (c *ClientState) GetMaxIdleDuration() int64 {
	if !viper.IsSet("chat.max_idle_duration") {
		return 30000
	}

	maxIdleDuration := viper.GetInt64("chat.max_idle_duration")
	if maxIdleDuration <= 0 {
		return math.MaxInt64
	}
	return maxIdleDuration
}

func (c *ClientState) UsesAudioIdleClock() bool {
	if c == nil {
		return false
	}
	return c.ListenMode == "auto" || c.IsRealTime()
}

func (c *ClientState) ShouldCountAudioIdleTimeout() bool {
	if c == nil || !c.IsRealTime() {
		return true
	}
	if c.GetTtsStart() {
		return false
	}
	switch c.GetStatus() {
	case ClientStatusLLMStart, ClientStatusTTSStart:
		return false
	default:
		return true
	}
}

func (c *ClientState) StartAudioIdleWindow(now time.Time) {
	if c == nil || !c.UsesAudioIdleClock() {
		return
	}
	c.AudioIdle.Start(now)
	c.SetClientVoiceStop(false)
}

func (c *ClientState) PauseAudioIdleWindow(now time.Time) {
	if c == nil || !c.UsesAudioIdleClock() {
		return
	}
	c.AudioIdle.Pause(now)
}

func (c *ClientState) ResumeAudioIdleWindow(now time.Time) {
	if c == nil || !c.UsesAudioIdleClock() {
		return
	}
	c.AudioIdle.Resume(now)
	c.SetClientVoiceStop(false)
}

func (c *ClientState) ResetAudioIdleWindow() {
	if c == nil {
		return
	}
	c.AudioIdle.Reset()
}

func (c *ClientState) GetAudioIdleElapsed(now time.Time) time.Duration {
	if c == nil {
		return 0
	}
	return c.AudioIdle.Elapsed(now)
}

func (c *ClientState) AudioIdleStarted() bool {
	if c == nil {
		return false
	}
	return c.AudioIdle.Started()
}

func (c *ClientState) AudioIdlePaused() bool {
	if c == nil {
		return false
	}
	return c.AudioIdle.Paused()
}

func (c *ClientState) MarkAudioIdleTimeoutPending() bool {
	if c == nil {
		return false
	}
	return c.AudioIdle.MarkTimeoutPending()
}

func (c *ClientState) ClearAudioIdleTimeoutPending() {
	if c == nil {
		return
	}
	c.AudioIdle.ClearTimeoutPending()
}

func (c *ClientState) AudioIdleTimeoutPending() bool {
	if c == nil {
		return false
	}
	return c.AudioIdle.TimeoutPending()
}

func (c *ClientState) GetPreAsrTextSilenceDuration() int64 {
	if viper.IsSet("chat.pre_asr_text_silence_duration") {
		preTextSilenceDuration := viper.GetInt64("chat.pre_asr_text_silence_duration")
		if preTextSilenceDuration <= 0 {
			return math.MaxInt64
		}
		return preTextSilenceDuration
	}

	base := c.VoiceStatus.SilenceThresholdTime
	if base <= 0 {
		base = 400
	}
	preTextSilenceDuration := base * 4
	if preTextSilenceDuration < 1000 {
		preTextSilenceDuration = 1000
	}
	return preTextSilenceDuration
}

func (c *ClientState) UpdateLastActiveTs() {
	c.MqttLastActiveTs = time.Now().Unix()
}

func (c *ClientState) IsActive() bool {
	diff := time.Now().Unix() - c.MqttLastActiveTs
	return c.MqttLastActiveTs > 0 && diff <= ClientActiveTs
}

func (c *ClientState) SetStatus(status string) {
	c.Status = status
}

func (c *ClientState) GetStatus() string {
	return c.Status
}

func (c *ClientState) SetListenPhase(phase string) {
	c.ListenPhase = phase
}

func (c *ClientState) GetListenPhase() string {
	return c.ListenPhase
}

type Ctx struct {
	sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

func (c *Ctx) Reset() {
	c.ResetWithReason("Ctx.Reset")
}

func (c *Ctx) ResetWithReason(reason string) {
	c.Lock()
	defer c.Unlock()
	if c.ctx != nil {
		log.Debugf("Ctx.ResetWithReason: lý do=%s", reason)
		c.cancel()
		c.ctx = nil
		c.cancel = nil
	}
}

func (c *Ctx) Get(parentCtx context.Context) context.Context {
	c.Lock()
	defer c.Unlock()
	if c.ctx == nil || c.ctx.Err() != nil {
		if c.ctx != nil {
			c.cancel()
		}
		c.ctx, c.cancel = context.WithCancel(parentCtx)
	}
	return c.ctx
}

func (c *Ctx) Cancel() {
	c.CancelWithReason("Ctx.Cancel")
}

func (c *Ctx) CancelWithReason(reason string) {
	c.Lock()
	defer c.Unlock()
	if c.ctx != nil {
		log.Debugf("Ctx.CancelWithReason: lý do=%s", reason)
		c.cancel()
		c.ctx = nil
		c.cancel = nil
	}
}

func (s *ClientState) getLLMProvider() (llm.LLMProvider, error) {
	llmConfig := s.DeviceConfig.Llm
	providerName := llmConfig.Provider
	if providerName == "" {
		providerName = "openai"
	}
	llmProvider, err := llm.GetLLMProvider(providerName, llmConfig.Config)
	if err != nil {
		return nil, fmt.Errorf("Tạo nhà cung cấp LLM thất bại: %v", err)
	}
	return llmProvider, nil
}

func (s *ClientState) InitLlm() error {
	ctx, cancel := context.WithCancel(s.Ctx)

	llmProvider, err := s.getLLMProvider()
	if err != nil {
		log.Errorf("Tạo nhà cung cấp LLM thất bại: %v", err)
		return err
	}

	s.Llm = Llm{
		Ctx:         ctx,
		Cancel:      cancel,
		LLMProvider: llmProvider,
	}
	return nil
}

func (s *ClientState) InitAsr() error {
	asrConfig := s.DeviceConfig.Asr

	log.Infof("Khởi tạo asr, asrConfig: %+v", asrConfig)

	//Khởi tạo asr (không trực tiếp tạo AsrProvider, sử dụng pool tài nguyên)
	ctx, cancel := context.WithCancel(s.Ctx)
	s.Asr = Asr{
		Ctx:             ctx,
		Cancel:          cancel,
		AsrAudioChannel: make(chan []float32, 100),
		AsrEnd:          make(chan bool, 1),
		AsrResult:       bytes.Buffer{},
		AsrType:         asrConfig.Provider,
		ClientState:     s, // Thiết lập tham chiếu ClientState
	}

	// Thiết lập chế độ ASR
	if mode, ok := asrConfig.Config["mode"].(string); ok {
		s.Asr.Mode = mode
	}

	if rawAutoEnd, ok := asrConfig.Config["auto_end"]; ok {
		if autoEnd, ok := rawAutoEnd.(bool); ok {
			s.Asr.AutoEnd = autoEnd
		}
	}
	return nil
}

func (c *ClientState) Destroy() {
	c.Asr.StopWithReason("ClientState.Destroy")
	c.Vad.Reset()
	c.ResetAudioIdleWindow()
	c.ClearAudioIdleTimeoutPending()

	// Trả lại tài nguyên ASR (nếu có)
	// Lưu ý: cần import package pool ở đây, nhưng để tránh phụ thuộc vòng, xử lý ở nơi gọi
	// Hoặc sử dụng type assertion ở đây, nhưng cần import package pool
	// Tạm thời xử lý ở nơi gọi (ChatSession.Close)

	c.VoiceStatus.Reset()
	c.AsrAudioBuffer.ClearAsrAudioData()

	c.SessionCtx.ResetWithReason("ClientState.Destroy: session_ctx")
	c.AfterAsrSessionCtx.ResetWithReason("ClientState.Destroy: after_asr_ctx")

	c.Statistic.Reset()
	c.SetStatus(ClientStatusInit)
	c.SetListenPhase(ListenPhaseIdle)
	c.SetTtsStart(false)
}

type CommandHistorySnapshot struct {
	LastCmdType string
	LastCmdAt   time.Time
}

func (s CommandHistorySnapshot) DebugString(now time.Time) string {
	formatAt := func(at time.Time) string {
		if at.IsZero() {
			return "zero"
		}
		return at.Format(time.RFC3339Nano)
	}
	formatAge := func(at time.Time) string {
		if at.IsZero() {
			return "n/a"
		}
		return now.Sub(at).Truncate(time.Millisecond).String()
	}

	return fmt.Sprintf(
		"lệnh cuối=%q thời gian lệnh cuối=%s tuổi lệnh cuối=%s",
		s.LastCmdType,
		formatAt(s.LastCmdAt),
		formatAge(s.LastCmdAt),
	)
}

func (c *ClientState) RecordCommandArrival(cmdType string, at time.Time) {
	c.cmdMu.Lock()
	c.LastCmdType = cmdType
	c.LastCmdAt = at
	c.cmdMu.Unlock()
}

func (c *ClientState) GetCommandHistorySnapshot() CommandHistorySnapshot {
	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()
	return CommandHistorySnapshot{
		LastCmdType: c.LastCmdType,
		LastCmdAt:   c.LastCmdAt,
	}
}

func (state *ClientState) OnManualStop() {
	state.ClearAudioIdleTimeoutPending()
	state.OnVoiceSilence()
}

func (state *ClientState) OnVoiceSilence() {
	silenceTs := time.Now().UnixMilli()
	log.Debugf("OnVoiceSilence, thời lượng giọng nói: %d, thời lượng giọng nói trong phiên: %d", state.Vad.GetVoiceDuration(), state.Vad.GetVoiceDurationInSession())
	if state.MarkVoiceSilenceAt(silenceTs) && state.OnVoiceSilenceMetricCallback != nil {
		state.OnVoiceSilenceMetricCallback(state.Ctx, silenceTs)
	}
	state.Asr.ResetReceivedText()
	state.SetClientVoiceStop(true) //Đặt cờ dừng nói, dữ liệu âm thanh nhận được sẽ không vào vad
	//Client ngừng nói
	state.Asr.StopWithReason("ClientState.OnVoiceSilence") //Dừng asr và lấy kết quả, thực hiện llm
	//Giải phóng vad
	state.Vad.Reset() //Giải phóng instance vad

	state.SetStatus(ClientStatusListenStop)
	state.SetListenPhase(ListenPhaseIdle)

	// Nếu đã đặt callback bất đồng bộ lấy kết quả nhận dạng giọng nói, thì gọi
	if state.OnVoiceSilenceSpeakerCallback != nil {
		state.OnVoiceSilenceSpeakerCallback(state.Ctx)
	}
}

type Llm struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	// Nhà cung cấp LLM
	LLMProvider llm.LLMProvider
	// Kênh nhận asr to text
	LLmRecvChannel chan llm_common.LLMResponseStruct
}

type SpeakReadyUDPConfig struct {
	Ready         bool `json:"ready"`
	ReuseExisting bool `json:"reuse_existing,omitempty"`
}

// ClientMessage đại diện cho tin nhắn client
type ClientMessage struct {
	Type           string               `json:"type"`
	DeviceID       string               `json:"device_id,omitempty"`
	SessionID      string               `json:"session_id,omitempty"`
	Text           string               `json:"text,omitempty"`
	Mode           string               `json:"mode,omitempty"`
	State          string               `json:"state,omitempty"`
	Token          string               `json:"token,omitempty"`
	DeviceMac      string               `json:"device_mac,omitempty"`
	Version        int                  `json:"version,omitempty"`
	Transport      string               `json:"transport,omitempty"`
	Features       map[string]bool      `json:"features,omitempty"`
	AudioParams    *AudioFormat         `json:"audio_params,omitempty"`
	SpeakUDPConfig *SpeakReadyUDPConfig `json:"udp_config,omitempty"`
	PayLoad        json.RawMessage      `json:"payload,omitempty"`
}