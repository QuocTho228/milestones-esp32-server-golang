package chat

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	. "milestones-esp32-server-golang/internal/data/client"
	chathooks "milestones-esp32-server-golang/internal/domain/chat/hooks"
	"milestones-esp32-server-golang/internal/domain/chat/streamtransform"
	config_types "milestones-esp32-server-golang/internal/domain/config/types"
	"milestones-esp32-server-golang/internal/domain/eventbus"
	"milestones-esp32-server-golang/internal/domain/llm"
	llm_common "milestones-esp32-server-golang/internal/domain/llm/common"
	"milestones-esp32-server-golang/internal/domain/speaker"
	"milestones-esp32-server-golang/internal/pool"
	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

const (
	MaxMessageCount = 10

	McpReadResourcePageSize       = 100 * 1024
	McpReadResourceStreamDoneFlag = "[DONE]"
)

// Kiểu Context key dùng để tránh xung đột
type contextKey int

const (
	ttsPlaybackCompletionGrace time.Duration = 150 * time.Millisecond
	fullTextKey                contextKey    = iota
	toolRoundMessagesKey
	ttsTurnTrackerKey
	ttsPlaybackStartHookKey
	ttsTurnEndPolicyKey
	ttsTurnEndPolicyHandlerKey
	ttsTurnPlaybackSettledKey
)

const (
	interruptExtraKey      = "interrupt"
	interruptByExtraKey    = "interrupt_by"
	interruptStageExtraKey = "interrupt_stage"
	interruptContentSuffix = " [Người dùng ngắt lời]"
)

// GetLastMessageID lấy MessageID của tin nhắn được lưu gần nhất (dùng cho lưu trữ hai giai đoạn)
func (l *LLMManager) GetLastMessageID(role string) (string, bool) {
	l.lastMessageIDMu.RLock()
	defer l.lastMessageIDMu.RUnlock()
	id, ok := l.lastMessageID[role]
	return id, ok
}

type LLMResponseChannelItem struct {
	ctx          context.Context
	userMessage  *schema.Message
	responseChan chan llm_common.LLMResponseStruct
	onStartFunc  func(args ...any)
	onEndFunc    func(err error, args ...any)
}

type llmHandleResult struct {
	ok                      bool
	suppressProtocolTtsStop bool
}

func llmHandleResultFromArgs(args []any) llmHandleResult {
	if len(args) == 0 {
		return llmHandleResult{}
	}
	result, ok := args[0].(llmHandleResult)
	if !ok {
		return llmHandleResult{}
	}
	return result
}

func (l *LLMManager) finishTTSTurn(ctx context.Context, stopErr error, result llmHandleResult) {
	l.finishTTSTurnWithReason(ctx, stopErr, result, "LLMManager.finishTTSTurn")
}

func (l *LLMManager) finishTTSTurnWithReason(ctx context.Context, stopErr error, result llmHandleResult, reason string) {
	if l == nil || l.ttsManager == nil {
		return
	}

	if result.suppressProtocolTtsStop {
		// Công cụ media sẽ đợi phát xong rồi mới quay lại đây để hoàn tất, lúc này vẫn cần gửi bổ sung tts_stop ở cấp giao thức,
		// nếu không client sẽ bị kẹt ở trạng thái “đang nói”.
		log.Debugf("Đầu ra media đã hoàn tất, tiếp tục quy trình kết thúc TTS thông thường để gửi tts stop")
	}

	l.ttsManager.EnqueueTtsStopWithReason(ctx, reason)
	l.ttsManager.RequestTurnEnd(ctx, stopErr)
}

type llmResponseChannelOptions struct {
	disableTTSCommands bool
	onStartFunc        func(args ...any)
	onEndFunc          func(err error, args ...any)
	onTTSPlaybackStart func()
	ttsTurnEndPolicy   ttsTurnEndPolicy
}

type ttsPlaybackStartHook func()

type ttsTurnEndPolicy uint8

const (
	ttsTurnEndPolicyNone ttsTurnEndPolicy = iota
	ttsTurnEndPolicyGoodbyeAndIdle
)

type ttsTurnEndPolicyHandler interface {
	handleTTSTurnEndPolicy(ctx context.Context, policy ttsTurnEndPolicy, stopErr error)
}

func withTTSPlaybackStartHook(ctx context.Context, hook func()) context.Context {
	if ctx == nil || hook == nil {
		return ctx
	}

	var once sync.Once
	return context.WithValue(ctx, ttsPlaybackStartHookKey, ttsPlaybackStartHook(func() {
		once.Do(hook)
	}))
}

func ttsPlaybackStartHookFromContext(ctx context.Context) func() {
	if ctx == nil {
		return nil
	}
	hook, ok := ctx.Value(ttsPlaybackStartHookKey).(ttsPlaybackStartHook)
	if !ok || hook == nil {
		return nil
	}
	return func() {
		hook()
	}
}

func withTTSTurnEndPolicy(ctx context.Context, policy ttsTurnEndPolicy) context.Context {
	if ctx == nil || policy == ttsTurnEndPolicyNone {
		return ctx
	}
	return context.WithValue(ctx, ttsTurnEndPolicyKey, policy)
}

func ttsTurnEndPolicyFromContext(ctx context.Context) ttsTurnEndPolicy {
	if ctx == nil {
		return ttsTurnEndPolicyNone
	}
	policy, ok := ctx.Value(ttsTurnEndPolicyKey).(ttsTurnEndPolicy)
	if !ok {
		return ttsTurnEndPolicyNone
	}
	return policy
}

func withTTSTurnEndPolicyHandler(ctx context.Context, handler ttsTurnEndPolicyHandler) context.Context {
	if ctx == nil || handler == nil {
		return ctx
	}
	return context.WithValue(ctx, ttsTurnEndPolicyHandlerKey, handler)
}

func withTTSTurnPlaybackSettled(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ttsTurnPlaybackSettledKey, true)
}

func ttsTurnPlaybackSettledFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	settled, ok := ctx.Value(ttsTurnPlaybackSettledKey).(bool)
	return ok && settled
}

func ttsTurnEndPolicyHandlerFromContext(ctx context.Context) ttsTurnEndPolicyHandler {
	if ctx == nil {
		return nil
	}
	handler, ok := ctx.Value(ttsTurnEndPolicyHandlerKey).(ttsTurnEndPolicyHandler)
	if !ok {
		return nil
	}
	return handler
}

type ttsTurnTracker struct {
	mu      sync.Mutex
	pending int
	doneCh  chan struct{}
}

func newTTSTurnTracker() *ttsTurnTracker {
	doneCh := make(chan struct{})
	close(doneCh)
	return &ttsTurnTracker{doneCh: doneCh}
}

func (t *ttsTurnTracker) Add() func(error) {
	if t == nil {
		return func(error) {}
	}

	t.mu.Lock()
	if t.pending == 0 {
		t.doneCh = make(chan struct{})
	}
	t.pending++
	t.mu.Unlock()

	var once sync.Once
	return func(error) {
		once.Do(func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			if t.pending == 0 {
				return
			}
			t.pending--
			if t.pending == 0 {
				close(t.doneCh)
			}
		})
	}
}

func (t *ttsTurnTracker) Wait(ctx context.Context) error {
	if t == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	t.mu.Lock()
	pending := t.pending
	doneCh := t.doneCh
	t.mu.Unlock()

	if pending == 0 {
		return nil
	}

	select {
	case <-doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type LLMManager struct {
	clientState       *ClientState
	session           *ChatSession
	serverTransport   *ServerTransport
	ttsManager        *TTSManager
	transformRegistry *streamtransform.Registry

	einoTools []*schema.ToolInfo

	llmResponseQueue *util.Queue[LLMResponseChannelItem]

	// Lưu trữ MessageID của tin nhắn được lưu gần nhất (dùng cho lưu trữ hai giai đoạn)
	// key: role (user/assistant), value: MessageID
	lastMessageID   map[string]string
	lastMessageIDMu sync.RWMutex // Bảo vệ truy cập đồng thời vào lastMessageID
}

func NewLLMManager(clientState *ClientState, serverTransport *ServerTransport, ttsManager *TTSManager, session *ChatSession, transformRegistry *streamtransform.Registry) *LLMManager {
	return &LLMManager{
		clientState:       clientState,
		session:           session,
		serverTransport:   serverTransport,
		ttsManager:        ttsManager,
		transformRegistry: transformRegistry,
		llmResponseQueue:  util.NewQueue[LLMResponseChannelItem](10),
		lastMessageID:     make(map[string]string),
	}
}

func (l *LLMManager) openOutputPipeline(ctx context.Context) (*streamtransform.Pipeline, error) {
	if l == nil || l.transformRegistry == nil {
		return &streamtransform.Pipeline{}, nil
	}

	sessionID := ""
	deviceID := ""
	if l.clientState != nil {
		sessionID = l.clientState.SessionID
		deviceID = l.clientState.DeviceID
	}

	return l.transformRegistry.Open(streamtransform.Context{
		Ctx:       ctx,
		SessionID: sessionID,
		DeviceID:  deviceID,
		RequestID: fmt.Sprintf("%s-%d", sessionID, time.Now().UnixNano()),
	})
}

func (l *LLMManager) emitLLMOutputRaw(ctx context.Context, data chathooks.LLMOutputRawData) (chathooks.LLMOutputRawData, bool, error) {
	if l == nil || l.session == nil || l.session.hookHub == nil {
		return data, false, nil
	}
	return l.session.hookHub.EmitLLMOutputRaw(l.session.hookContext(ctx), data)
}

// handleLLMWithContextAndTools sử dụng context để xử lý phản hồi LLM (tương thích cả có công cụ và không có công cụ)
// Bên trong tự động quản lý việc lấy và giải phóng tài nguyên LLM
func (l *LLMManager) handleLLMWithContextAndTools(
	ctx context.Context,
	dialogue []*schema.Message,
	tools []*schema.ToolInfo,
) (chan llm_common.LLMResponseStruct, error) {
	// Lấy tài nguyên LLM
	llmWrapper, err := pool.Acquire[llm.LLMProvider](
		"llm",
		l.clientState.DeviceConfig.Llm.Provider,
		l.clientState.DeviceConfig.Llm.Config,
	)
	if err != nil {
		return nil, fmt.Errorf("Lấy tài nguyên LLM thất bại: %w", err)
	}

	// Lấy provider
	llmProvider := llmWrapper.GetProvider()

	// Gọi LLM provider
	msgChan := llmProvider.ResponseWithContext(ctx, l.clientState.SessionID, dialogue, tools)

	pipeline, err := l.openOutputPipeline(ctx)
	if err != nil {
		pool.Release(llmWrapper)
		return nil, fmt.Errorf("Tạo pipeline biến đổi luồng đầu ra LLM thất bại: %w", err)
	}

	// Tạo channel phản hồi
	responseChannel := make(chan llm_common.LLMResponseStruct, 2)
	startTs := time.Now().UnixMilli()
	var firstSegment bool
	var rawFullText strings.Builder

	// Khởi động goroutine để xử lý phản hồi
	go func() {
		defer func() {
			log.Debugf("full Response with %d tools, fullText: %s", len(tools), rawFullText.String())
			close(responseChannel)
			if closeErr := pipeline.Close(); closeErr != nil {
				log.Warnf("Đóng pipeline biến đổi luồng đầu ra LLM thất bại: %v", closeErr)
			}
			// Giải phóng tài nguyên
			pool.Release(llmWrapper)
			log.Debugf("Tài nguyên LLM đã được giải phóng")
		}()

		isFirstOutput := true
		llmFirstTokenMarked := false

		emitResponse := func(item streamtransform.Item) bool {
			response := llm_common.LLMResponseStruct{
				IsEnd: item.IsEnd,
			}

			switch item.Kind {
			case streamtransform.ItemKindToolCalls:
				response.ToolCalls = item.ToolCalls
				if len(item.ToolCalls) > 0 {
					response.IsStart = isFirstOutput
				}
			case streamtransform.ItemKindTextDelta, streamtransform.ItemKindTextSegment:
				response.Text = item.Text
				if item.Meta != nil {
					if emo, ok := item.Meta["emotion"].(string); ok && emo != "" {
						response.Emotion = emo
					}
				}
				if strings.TrimSpace(item.Text) != "" {
					response.IsStart = isFirstOutput
					if !firstSegment {
						firstSegment = true
						firstSentenceTs := time.Now().UnixMilli()
						if l.clientState.MarkLlmFirstSentenceAt(firstSentenceTs) && l.session != nil {
							l.session.TraceLlmFirstSentence(ctx, firstSentenceTs)
						}
						log.Infof("Thống kê thời gian: câu đầu tiên của llm: %d ms", firstSentenceTs-startTs)
					}
					if isFirstOutput {
						isFirstOutput = false
					}
				}
			default:
				return true
			}

			if strings.TrimSpace(response.Text) == "" && len(response.ToolCalls) == 0 && !response.IsEnd {
				return true
			}

			select {
			case <-ctx.Done():
				log.Infof("Context đã bị hủy, dừng xử lý phản hồi LLM: %v, context done, exit", ctx.Err())
				return false
			case responseChannel <- response:
				return true
			}
		}

		pushToPipeline := func(item streamtransform.Item) (bool, error) {
			items, stop, err := pipeline.Push(item)
			if err != nil {
				return false, err
			}
			for _, out := range items {
				if !emitResponse(out) {
					return true, nil
				}
			}
			return stop, nil
		}

		pushRawText := func(delta string, isEnd bool, errVal error) (bool, error) {
			payload, stop, hookErr := l.emitLLMOutputRaw(ctx, chathooks.LLMOutputRawData{
				Delta:    delta,
				FullText: rawFullText.String(),
				IsEnd:    isEnd,
				Err:      errVal,
			})
			if hookErr != nil {
				log.Warnf("Thực thi hook LLM_OUTPUT_RAW thất bại: %v", hookErr)
			}
			if stop {
				log.Infof("Hook LLM_OUTPUT_RAW yêu cầu dừng quy trình hiện tại")
				return true, nil
			}
			if payload.Delta != "" {
				rawFullText.WriteString(payload.Delta)
			}
			return pushToPipeline(streamtransform.Item{
				Kind:  streamtransform.ItemKindTextDelta,
				Text:  payload.Delta,
				IsEnd: payload.IsEnd,
			})
		}

		pushRawToolCalls := func(toolCalls []schema.ToolCall) (bool, error) {
			payload, stop, hookErr := l.emitLLMOutputRaw(ctx, chathooks.LLMOutputRawData{
				FullText:  rawFullText.String(),
				ToolCalls: toolCalls,
			})
			if hookErr != nil {
				log.Warnf("Thực thi hook LLM_OUTPUT_RAW thất bại: %v", hookErr)
			}
			if stop {
				log.Infof("Hook LLM_OUTPUT_RAW yêu cầu dừng quy trình hiện tại")
				return true, nil
			}
			if len(payload.ToolCalls) == 0 {
				return false, nil
			}
			return pushToPipeline(streamtransform.Item{
				Kind:      streamtransform.ItemKindToolCalls,
				ToolCalls: payload.ToolCalls,
			})
		}

		for {
			select {
			case <-ctx.Done():
				log.Infof("Context đã bị hủy, dừng xử lý phản hồi LLM: %v, context done, exit", ctx.Err())
				return
			case message, ok := <-msgChan:
				if !ok {
					stop, pushErr := pushRawText("", true, nil)
					if pushErr != nil {
						log.Errorf("Xử lý luồng kết thúc LLM thất bại: %v", pushErr)
					}
					if stop || pushErr != nil {
						return
					}
					return
				}
				if message == nil {
					continue
				}
				if llm.IsLLMErrorMessage(message) {
					errMsg := llm.LLMErrorMessage(message)
					log.Warnf("LLM trả về lỗi: %s", errMsg)

					// Không đọc nguyên văn lỗi kỹ thuật (429, rate limit, timeout...) cho người dùng nghe.
					// Thay bằng câu trả lời thân thiện, phù hợp persona trợ lý.
					friendlyMsg := friendlyErrorMessageFor(errMsg)

					stop, pushErr := pushRawText(friendlyMsg, true, nil)
					if pushErr != nil {
						log.Errorf("Xử lý đầu ra lỗi LLM thất bại: %v", pushErr)
					}
					if stop || pushErr != nil {
						return
					}
					return
				}
				if message.Content != "" {
					if !llmFirstTokenMarked {
						firstTokenTs := time.Now().UnixMilli()
						l.clientState.MarkLlmFirstToken()
						if l.session != nil {
							l.session.TraceLlmFirstToken(ctx, firstTokenTs)
						}
						llmFirstTokenMarked = true
					}
					stop, pushErr := pushRawText(message.Content, false, nil)
					if pushErr != nil {
						log.Errorf("Xử lý luồng văn bản LLM thất bại: %v", pushErr)
						return
					}
					if stop {
						return
					}
				}
				if len(message.ToolCalls) > 0 {
					log.Infof("Xử lý gọi công cụ: %+v", message.ToolCalls)
					stop, pushErr := pushRawToolCalls(message.ToolCalls)
					if pushErr != nil {
						log.Errorf("Xử lý luồng công cụ LLM thất bại: %v", pushErr)
						return
					}
					if stop {
						return
					}
				}
			}
		}
	}()

	return responseChannel, nil
}

func (l *LLMManager) Start(ctx context.Context) {
	l.processLLMResponseQueue(ctx)
}

func (l *LLMManager) processLLMResponseQueue(ctx context.Context) {
	for {
		item, err := l.llmResponseQueue.Pop(ctx, 0) // kiểu chặn (blocking)
		if err != nil {
			if err == util.ErrQueueCtxDone {
				return
			}
			// Lỗi khác
			continue
		}

		log.Debugf("processLLMResponseQueue item: %+v", item)
		if item.onStartFunc != nil {
			item.onStartFunc()
		}

		// Gọi handleLLMResponse, nó sẽ lấy fullText và toolCalls từ context và điền vào
		result, err := l.handleLLMResponse(item.ctx, item.userMessage, item.responseChan)
		if waitErr := waitForTTSTurnDrainIfRoot(item.ctx); err == nil && waitErr != nil {
			err = waitErr
		}

		if item.onEndFunc != nil {
			item.onEndFunc(err, result)
		}
	}
}

func (l *LLMManager) ClearLLMResponseQueue() {
	l.llmResponseQueue.Clear()
}

func (l *LLMManager) AddTextToTTSQueue(text string) error {
	return l.AddTextToTTSQueueWithOptions(text, llmResponseChannelOptions{})
}

func (l *LLMManager) AddTextToTTSQueueWithOptions(text string, options llmResponseChannelOptions) error {
	log.Debugf("AddTextToTTSQueue text: %s", text)
	msg := &schema.Message{
		Role:    schema.User,
		Content: text,
	}
	llmResponseChan := make(chan llm_common.LLMResponseStruct, 10)
	llmResponseChan <- llm_common.LLMResponseStruct{
		IsStart: true,
		IsEnd:   true,
		Text:    text,
	}
	close(llmResponseChan)

	sessionCtx := l.clientState.SessionCtx.Get(l.clientState.Ctx)
	ctx := l.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	ctx = withTTSPlaybackStartHook(ctx, options.onTTSPlaybackStart)
	ctx = withTTSTurnEndPolicy(ctx, options.ttsTurnEndPolicy)
	if err := l.HandleLLMResponseChannelAsyncWithOptions(ctx, msg, llmResponseChan, options); err != nil {
		log.Warnf("AddTextToTTSQueue enqueue failed: %v", err)
		return err
	}

	return nil
}

func chainLLMResponseStartHooks(hooks ...func(args ...any)) func(args ...any) {
	filtered := make([]func(args ...any), 0, len(hooks))
	for _, hook := range hooks {
		if hook != nil {
			filtered = append(filtered, hook)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return func(args ...any) {
		for _, hook := range filtered {
			hook(args...)
		}
	}
}

func chainLLMResponseEndHooks(hooks ...func(err error, args ...any)) func(err error, args ...any) {
	filtered := make([]func(err error, args ...any), 0, len(hooks))
	for _, hook := range hooks {
		if hook != nil {
			filtered = append(filtered, hook)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return func(err error, args ...any) {
		for _, hook := range filtered {
			hook(err, args...)
		}
	}
}

func (l *LLMManager) HandleLLMResponseChannelAsync(ctx context.Context, userMessage *schema.Message, responseChan chan llm_common.LLMResponseStruct) error {
	return l.handleLLMResponseChannelAsync(ctx, userMessage, responseChan, llmResponseChannelOptions{})
}

func (l *LLMManager) HandleLLMResponseChannelAsyncWithOptions(ctx context.Context, userMessage *schema.Message, responseChan chan llm_common.LLMResponseStruct, options llmResponseChannelOptions) error {
	return l.handleLLMResponseChannelAsync(ctx, userMessage, responseChan, options)
}

func (l *LLMManager) handleLLMResponseChannelAsync(ctx context.Context, userMessage *schema.Message, responseChan chan llm_common.LLMResponseStruct, options llmResponseChannelOptions) error {
	ctx = ensureTTSTurnTrackerInContext(ctx)
	ctx = withTTSPlaybackStartHook(ctx, options.onTTSPlaybackStart)
	ctx = withTTSTurnEndPolicy(ctx, options.ttsTurnEndPolicy)

	needSendTtsCmd := true
	val := ctx.Value("nest")
	nest := 0
	log.Debugf("AddLLMResponseChannel nest: %+v", val)
	if n, ok := val.(int); ok {
		nest = n
		if nest > 1 {
			needSendTtsCmd = false
		}
	}
	if options.disableTTSCommands {
		needSendTtsCmd = false
	}

	// Khởi tạo hoặc tái sử dụng fullText trong context (dùng cho lịch sử chat)
	// Nếu context đã có fullText (tiếp tục yêu cầu LLM sau khi gọi công cụ), thì tái sử dụng; nếu không thì tạo mới
	var fullText *strings.Builder
	if existingFullText, ok := ctx.Value(fullTextKey).(*strings.Builder); ok && existingFullText != nil {
		fullText = existingFullText
		log.Debugf("Tái sử dụng fullText hiện có, độ dài hiện tại: %d", fullText.Len())
	} else {
		fullText = &strings.Builder{}
		ctx = context.WithValue(ctx, fullTextKey, fullText)
		log.Debugf("Tạo fullText mới")
	}

	var onStartFunc func(...any)
	var onEndFunc func(err error, args ...any)

	if needSendTtsCmd {
		onStartFunc = func(...any) {
			// Kiểm tra xem có phải là lần gọi LLM đầu tiên không (dựa vào giá trị nest trong context), chỉ xóa bộ đệm âm thanh TTS khi là lần gọi đầu tiên
			val := ctx.Value("nest")
			if nest, ok := val.(int); !ok || nest <= 1 {
				// Lần gọi đầu tiên hoặc không có giá trị nest, xóa bộ đệm âm thanh TTS
				l.ttsManager.ClearAudioHistory()
				log.Debugf("onStartFunc lần gọi đầu tiên, đã xóa bộ đệm âm thanh TTS")
			}
			l.ttsManager.EnqueueTtsStartWithReason(ctx, "LLMManager.handleLLMResponseChannelAsync onStart")
		}
		onEndFunc = func(err error, args ...any) {
			handleResult := llmHandleResultFromArgs(args)
			l.clientState.MarkLlmEnd()
			if l.session != nil {
				l.session.TraceLlmEnd(ctx, time.Now().UnixMilli(), err)
			}
			strFullText := fullText.String()

			l.finishTTSTurnWithReason(ctx, err, handleResult, "LLMManager.handleLLMResponseChannelAsync onEnd")

			// Lấy fullText từ closure
			audioData := l.ttsManager.GetAndClearAudioHistory()

			// Tính tổng kích thước âm thanh (tổng số byte của tất cả các khung)
			audioSize := 0
			for _, frame := range audioData {
				audioSize += len(frame)
			}

			// Chỉ gửi sự kiện khi là lần gọi đầu tiên (nest<=1)
			if nest <= 1 {
				// Lấy MessageID từ LLMManager (vai trò Assistant)
				// Nếu không tìm thấy MessageID, nghĩa là giai đoạn lưu thứ nhất chưa hoàn tất, không thực hiện cập nhật giai đoạn hai
				messageID, ok := l.GetLastMessageID(string(schema.Assistant))
				if !ok {
					log.Warnf("Không tìm thấy MessageID khi TTS hoàn tất, bỏ qua cập nhật âm thanh giai đoạn hai")
					return
				}

				// Phát sự kiện: giai đoạn hai (cập nhật âm thanh)
				assistantMsg := schema.AssistantMessage(strFullText, nil)
				eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
					ClientState: l.clientState,
					Msg:         *assistantMsg,
					MessageID:   messageID,
					AudioData:   audioData, // Giai đoạn hai: có âm thanh
					AudioSize:   audioSize,
					SampleRate:  l.clientState.OutputAudioFormat.SampleRate,
					Channels:    l.clientState.OutputAudioFormat.Channels,
					Timestamp:   time.Now(),
					IsUpdate:    true, // cập nhật tin nhắn
				})
			}
		}
	}

	onStartFunc = chainLLMResponseStartHooks(onStartFunc, options.onStartFunc)
	onEndFunc = chainLLMResponseEndHooks(onEndFunc, options.onEndFunc)

	item := LLMResponseChannelItem{
		ctx:          ctx,
		userMessage:  userMessage,
		responseChan: responseChan,
		onStartFunc:  onStartFunc,
		onEndFunc:    onEndFunc,
	}

	err := l.llmResponseQueue.Push(item)
	if err != nil {
		log.Warnf("llmResponseQueue đã đầy hoặc đã đóng, bỏ tin nhắn")
		return fmt.Errorf("llmResponseQueue đã đầy hoặc đã đóng, bỏ tin nhắn")
	}
	return nil
}

func (l *LLMManager) HandleLLMResponseChannelSync(ctx context.Context, userMessage *schema.Message, llmResponseChannel chan llm_common.LLMResponseStruct, einoTools []*schema.ToolInfo) (bool, error) {
	ctx = ensureTTSTurnTrackerInContext(ctx)

	needSendTtsCmd := true
	val := ctx.Value("nest")
	nest := 0
	log.Debugf("AddLLMResponseChannel nest: %+v", val)
	if n, ok := val.(int); ok {
		nest = n
		if nest > 1 {
			needSendTtsCmd = false
		}
	}

	// Khởi tạo hoặc tái sử dụng fullText trong context (dùng cho lịch sử chat)
	// Nếu context đã có fullText (tiếp tục yêu cầu LLM sau khi gọi công cụ), thì tái sử dụng; nếu không thì tạo mới
	var fullText *strings.Builder
	if existingFullText, ok := ctx.Value(fullTextKey).(*strings.Builder); ok && existingFullText != nil {
		fullText = existingFullText
		log.Debugf("Tái sử dụng fullText hiện có, độ dài hiện tại: %d", fullText.Len())
	} else {
		fullText = &strings.Builder{}
		ctx = context.WithValue(ctx, fullTextKey, fullText)
		log.Debugf("Tạo fullText mới")
	}

	if needSendTtsCmd {
		// Kiểm tra xem có phải là lần gọi LLM đầu tiên không (dựa vào giá trị nest trong context), chỉ xóa bộ đệm âm thanh TTS khi là lần gọi đầu tiên
		if nest <= 1 {
			// Lần gọi đầu tiên hoặc không có giá trị nest, xóa bộ đệm âm thanh TTS
			l.ttsManager.ClearAudioHistory()
			log.Debugf("HandleLLMResponseChannelSync lần gọi đầu tiên, đã xóa bộ đệm âm thanh TTS")
		}
		l.ttsManager.EnqueueTtsStartWithReason(ctx, "LLMManager.HandleLLMResponseChannelSync start")
	}

	result, err := l.handleLLMResponse(ctx, userMessage, llmResponseChannel)
	if waitErr := waitForTTSTurnDrainIfRoot(ctx); err == nil && waitErr != nil {
		err = waitErr
	}
	l.clientState.MarkLlmEnd()
	if l.session != nil {
		l.session.TraceLlmEnd(ctx, time.Now().UnixMilli(), err)
	}
	strFullText := fullText.String()

	if needSendTtsCmd {
		l.finishTTSTurnWithReason(ctx, err, result, "LLMManager.HandleLLMResponseChannelSync end")

		// Thu thập âm thanh TTS và gửi sự kiện lịch sử chat
		// Lưu ý: phản hồi LLM sau khi gọi công cụ (nest > 1) cũng sẽ tích lũy âm thanh vào bộ đệm, nhưng không xóa
		// Chỉ xóa bộ đệm và gửi sự kiện khi là lần gọi đầu tiên (nest<=1)
		audioData := l.ttsManager.GetAndClearAudioHistory()

		// Tính tổng kích thước âm thanh (tổng số byte của tất cả các khung)
		audioSize := 0
		for _, frame := range audioData {
			audioSize += len(frame)
		}

		// Chỉ gửi sự kiện khi là lần gọi đầu tiên (nest<=1)
		if nest <= 1 {
			// Lấy MessageID từ LLMManager (vai trò Assistant)
			// Nếu không tìm thấy MessageID, nghĩa là giai đoạn lưu thứ nhất chưa hoàn tất, không thực hiện cập nhật giai đoạn hai
			messageID, ok := l.GetLastMessageID(string(schema.Assistant))
			if !ok {
				log.Warnf("Không tìm thấy MessageID khi TTS hoàn tất, bỏ qua cập nhật âm thanh giai đoạn hai")
				return result.ok, err
			}

			// Phát sự kiện: giai đoạn hai (cập nhật âm thanh)
			assistantMsg := schema.AssistantMessage(strFullText, nil)
			eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
				ClientState: l.clientState,
				Msg:         *assistantMsg,
				MessageID:   messageID,
				AudioData:   audioData, // Giai đoạn hai: có âm thanh
				AudioSize:   audioSize,
				SampleRate:  l.clientState.OutputAudioFormat.SampleRate,
				Channels:    l.clientState.OutputAudioFormat.Channels,
				Timestamp:   time.Now(),
			})
		}
	} else {
		// Trường hợp nest > 1: mặc dù không gửi lệnh TTS, nhưng dữ liệu âm thanh vẫn sẽ được tích lũy vào bộ đệm
		// Các âm thanh này sẽ được thu thập cùng nhau khi phản hồi lần đầu kết thúc (nest <= 1)
		log.Debugf("Phản hồi LLM sau khi gọi công cụ (nest=%d), dữ liệu âm thanh sẽ được tích lũy vào bộ đệm", nest)
	}

	return result.ok, err
}

// handleLLMResponse xử lý phản hồi LLM
func (l *LLMManager) handleLLMResponse(ctx context.Context, userMessage *schema.Message, llmResponseChannel chan llm_common.LLMResponseStruct) (llmHandleResult, error) {
	log.Debugf("handleLLMResponse start")
	defer log.Debugf("handleLLMResponse end")

	// Lấy fullText từ context (dùng cho lịch sử chat)
	fullText := ctx.Value(fullTextKey).(*strings.Builder)
	state := l.clientState
	// toolCalls sử dụng biến cục bộ (logic gọi công cụ nội bộ, không liên quan đến lịch sử chat)
	var toolCalls []schema.ToolCall
	toolExecCtx := context.WithValue(ctx, "nest", 2)
	toolExecCtx = context.WithValue(toolExecCtx, fullTextKey, fullText)
	if speechStartHook := ttsPlaybackStartHookFromContext(ctx); speechStartHook != nil {
		toolExecCtx = withTTSPlaybackStartHook(toolExecCtx, speechStartHook)
	}
	if l.clientState.GetMemoryMode() == MemoryModeNone && userMessage != nil {
		toolExecCtx = appendToolRoundMessagesToContext(toolExecCtx, []*schema.Message{userMessage})
	}
	ttsTracker := ttsTurnTrackerFromContext(ctx)
	var onTTSItemEnqueued func() func(error)
	onTTSPlaybackStart := ttsPlaybackStartHookFromContext(ctx)
	if ttsTracker != nil {
		onTTSItemEnqueued = ttsTracker.Add
	}
	toolExecutor := newToolCallExecutor(l, toolExecCtx)
	assistantSaved := false
	result := llmHandleResult{}

	saveInterruptedAssistant := func() {
		if assistantSaved {
			return
		}
		if ctx.Err() == nil {
			return
		}
		text := strings.TrimSpace(fullText.String())
		if text == "" {
			return
		}
		msg := schema.AssistantMessage(text, nil)
		msg.Extra = map[string]any{
			interruptExtraKey:      true,
			interruptByExtraKey:    "user",
			interruptStageExtraKey: "llm",
		}
		if err := l.AddLlmMessage(ctx, msg); err != nil {
			log.Errorf("Lưu tin nhắn trợ lý bị ngắt thất bại: %v", err)
			return
		}
		assistantSaved = true
	}

	select {
	case <-ctx.Done():
		saveInterruptedAssistant()
		log.Debugf("handleLLMResponse ctx done, return")
		return result, nil
	default:
	}

	for {
		select {
		case <-ctx.Done():
			// Context đã bị hủy, ưu tiên xử lý logic hủy
			saveInterruptedAssistant()
			log.Infof("%s Context đã bị hủy, dừng xử lý phản hồi LLM, context done, exit", state.DeviceID)
			return result, nil
		default:
			// Kiểm tra không chặn, nếu ctx chưa Done, tiếp tục xử lý phản hồi LLM
			select {
			case llmResponse, ok := <-llmResponseChannel:
				if !ok {
					// Kênh đã đóng, thoát goroutine
					log.Infof("Kênh phản hồi LLM đã đóng, thoát goroutine")
					result.ok = true
					return result, nil
				}
				if ctx.Err() != nil {
					saveInterruptedAssistant()
					log.Infof("%s Context đã bị hủy khi phân đoạn LLM đến, bỏ qua phản hồi đến muộn và thoát", state.DeviceID)
					return result, nil
				}

				log.Debugf("Phản hồi LLM: %+v", llmResponse)

				if len(llmResponse.ToolCalls) > 0 {
					log.Debugf("Đã nhận được công cụ: %+v", llmResponse.ToolCalls)
					toolCalls = append(toolCalls, llmResponse.ToolCalls...)
					toolExecutor.Submit(llmResponse.ToolCalls)
				}

				hasText := strings.TrimSpace(llmResponse.Text) != ""
				if hasText || llmResponse.IsStart || llmResponse.IsEnd {
					// Việc kết thúc song song (dual-stream) phụ thuộc vào tín hiệu IsEnd của văn bản rỗng, không thể chỉ chuyển cho TTS khi có văn bản.
					if err := l.ttsManager.handleTextResponseWithHooks(ctx, llmResponse, false, onTTSItemEnqueued, onTTSPlaybackStart); err != nil {
						result.ok = true
						return result, err
					}
				}
				if hasText {
					fullText.WriteString(llmResponse.Text)
				}

				if llmResponse.IsEnd {
					if len(toolCalls) == 0 {
						//Ghi vào redis
						if userMessage != nil {
							if userMessage.Role == schema.User {
								// Kiểm tra xem tin nhắn người dùng đã được lưu chưa (đã lưu khi xử lý ASR)
								// Xác định bằng cách kiểm tra tin nhắn cuối cùng có phải là tin nhắn người dùng và nội dung có khớp không
								/*messages := l.clientState.GetMessages(1)
								shouldSave := true
								if len(messages) > 0 {
									lastMsg := messages[len(messages)-1]
									if lastMsg.Role == schema.User && lastMsg.Content == userMessage.Content {
										// Tin nhắn người dùng đã được lưu trước đó (được lưu khi xử lý ASR), bỏ qua
										shouldSave = false
										log.Debugf("Tin nhắn người dùng đã được lưu khi xử lý ASR, bỏ qua việc lưu trùng lặp: %s", userMessage.Content)
									}
								}
								if shouldSave {
									if err := l.AddLlmMessage(ctx, userMessage); err != nil {
										log.Errorf("Lưu tin nhắn người dùng thất bại: %v", err)
									}
								}*/
							}
						}
						strFullText := fullText.String()
						if strings.TrimSpace(strFullText) != "" || len(toolCalls) > 0 {
							if err := l.AddLlmMessage(ctx, schema.AssistantMessage(strFullText, toolCalls)); err != nil {
								log.Errorf("Lưu tin nhắn trợ lý thất bại: %v", err)
							} else {
								assistantSaved = true
							}
						}
					}
					if len(toolCalls) > 0 {
						toolSummary, err := l.handleToolCallResponse(toolExecCtx, schema.AssistantMessage(fullText.String(), toolCalls), toolCalls, toolExecutor)
						if err != nil {
							log.Errorf("Xử lý phản hồi gọi công cụ thất bại: %v", err)
							result.ok = true
							return result, fmt.Errorf("Xử lý phản hồi gọi công cụ thất bại: %v", err)
						}
						result.suppressProtocolTtsStop = toolSummary.hasMediaOutput
						if !toolSummary.invokeToolSuccess && strings.TrimSpace(llmResponse.Text) != "" {
							if err := l.ttsManager.handleTextResponseWithHooks(ctx, llmResponse, false, nil, onTTSPlaybackStart); err != nil {
								result.ok = true
								return result, err
							}
							fullText.WriteString(llmResponse.Text)
						}
					}

					result.ok = true
					return result, nil
				}
			case <-ctx.Done():
				// Context đã bị hủy, thoát goroutine
				saveInterruptedAssistant()
				log.Infof("%s Context đã bị hủy, dừng xử lý phản hồi LLM, context done, exit", state.DeviceID)
				return result, nil
			}
		}
	}
}

func (l *LLMManager) DoLLmRequest(ctx context.Context, userMessage *schema.Message, einoTools []*schema.ToolInfo, isSync bool, speakerResult *speaker.IdentifyResult) error {
	log.Debugf("Gửi yêu cầu LLM có công cụ, seesionID: %s, requestEinoMessages: %+v", l.clientState.SessionID, userMessage)
	clientState := l.clientState

	l.einoTools = einoTools

	//Ghép lịch sử tin nhắn và tin nhắn hiện tại của người dùng
	requestMessages := l.GetMessages(ctx, userMessage, MaxMessageCount, speakerResult)

	if l.session != nil {
		payload, stop, hookErr := l.session.hookHub.EmitLLMInput(l.session.hookContext(ctx), chathooks.LLMInputData{
			UserMessage:     userMessage,
			RequestMessages: requestMessages,
			Tools:           einoTools,
		})
		if hookErr != nil {
			log.Warnf("Thực thi hook LLM_INPUT thất bại: %v", hookErr)
		}
		userMessage = payload.UserMessage
		requestMessages = payload.RequestMessages
		einoTools = payload.Tools
		if stop {
			log.Infof("Hook LLM_INPUT yêu cầu dừng quy trình hiện tại")
			return nil
		}
	}

	clientState.SetStartLlmTs()
	if l.session != nil {
		l.session.TraceLlmStart(ctx, time.Now().UnixMilli())
	}
	clientState.SetStatus(ClientStatusLLMStart)

	// Gọi phương thức nội bộ để xử lý phản hồi LLM, tài nguyên được quản lý bên trong phương thức
	responseSentences, err := l.handleLLMWithContextAndTools(
		ctx,
		requestMessages,
		einoTools,
	)
	if err != nil {
		log.Errorf("Gửi yêu cầu LLM có công cụ thất bại, seesionID: %s, error: %v", l.clientState.SessionID, err)
		return fmt.Errorf("Gửi yêu cầu LLM có công cụ thất bại: %v", err)
	}

	log.Debugf("DoLLmRequest goroutine bắt đầu - SessionID: %s, trạng thái context: %v", l.clientState.SessionID, ctx.Err())

	if isSync {
		// Xử lý đồng bộ: tài nguyên sẽ được tự động giải phóng trong defer của handleLLMWithContextAndTools
		_, err := l.HandleLLMResponseChannelSync(ctx, userMessage, responseSentences, einoTools)
		if err != nil {
			log.Errorf("Xử lý phản hồi LLM thất bại, seesionID: %s, error: %v", l.clientState.SessionID, err)
			return err
		}
	} else {
		// Xử lý bất đồng bộ: tài nguyên sẽ được tự động giải phóng trong defer của handleLLMWithContextAndTools
		err = l.HandleLLMResponseChannelAsync(ctx, userMessage, responseSentences)
		if err != nil {
			log.Errorf("Xử lý phản hồi LLM thất bại, seesionID: %s, error: %v", l.clientState.SessionID, err)
		}
	}

	log.Debugf("DoLLmRequest kết thúc - SessionID: %s", l.clientState.SessionID)

	return nil
}

// AddMessage thêm tin nhắn vào lịch sử chat (điểm vào thống nhất, áp dụng cho mọi loại tin nhắn)
func (l *LLMManager) AddMessage(ctx context.Context, msg *schema.Message) error {
	if msg == nil {
		log.Warnf("Cố gắng thêm tin nhắn nil vào lịch sử chat")
		return fmt.Errorf("Tin nhắn không được là nil")
	}

	// Tạo MessageID (dùng hàm băm MD5 để rút ngắn độ dài, tránh vượt quá giới hạn varchar(64) của database)
	// Định dạng gốc: {SessionID}-{Role}-{Timestamp}
	rawMessageID := fmt.Sprintf("%s-%s-%d",
		l.clientState.SessionID,
		msg.Role,
		time.Now().UnixMilli())
	// Dùng hàm băm MD5 để tạo chuỗi hex cố định 32 ký tự
	hash := md5.Sum([]byte(rawMessageID))
	messageID := hex.EncodeToString(hash[:])

	// Thêm đồng bộ vào bộ nhớ
	l.clientState.AddMessage(msg)

	// Tin nhắn vai trò Tool: lưu trực tiếp, không liên quan đến lưu hai giai đoạn (không có âm thanh)
	if msg.Role == schema.Tool {
		eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
			ClientState: l.clientState,
			Msg:         *msg,
			MessageID:   messageID,
			AudioData:   nil, // Vai trò Tool không có âm thanh
			AudioSize:   0,
			SampleRate:  0,
			Channels:    0,
			Timestamp:   time.Now(),
			IsUpdate:    false, // Lưu một lần
		})
		return nil
	}

	// Vai trò User/Assistant: lưu hai giai đoạn
	// Lưu trữ MessageID vào LLMManager để dùng cho việc cập nhật âm thanh sau này
	if msg.Role == schema.User || msg.Role == schema.Assistant {
		l.lastMessageIDMu.Lock()
		l.lastMessageID[string(msg.Role)] = messageID
		l.lastMessageIDMu.Unlock()
	}

	// Phát sự kiện: giai đoạn một (chỉ văn bản, không có âm thanh)
	eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
		ClientState: l.clientState,
		Msg:         *msg,
		MessageID:   messageID,
		AudioData:   nil, // Giai đoạn một: không có âm thanh
		AudioSize:   0,
		SampleRate:  0,
		Channels:    0,
		Timestamp:   time.Now(),
		IsUpdate:    false, // Tin nhắn mới
	})

	return nil
}

// AddLlmMessage giữ khả năng tương thích ngược, ủy quyền cho AddMessage
func (l *LLMManager) AddLlmMessage(ctx context.Context, msg *schema.Message) error {
	return l.AddMessage(ctx, msg)
}

func (l *LLMManager) GetMessages(ctx context.Context, userMessage *schema.Message, count int, speakerResult *speaker.IdentifyResult) []*schema.Message {
	memoryMode := l.clientState.GetMemoryMode()
	includeHistory := memoryMode != MemoryModeNone

	// Lấy ngữ cảnh từ dialogue; ở chế độ none chỉ cho phép mang theo tin nhắn tạm thời của chuỗi gọi công cụ hiện tại
	messageList := make([]*schema.Message, 0)
	if includeHistory {
		messageList = l.clientState.GetMessages(count)
		if userMessage != nil {
			messageList = trimTrailingUserMessages(messageList)
		}
	} else if toolRoundMessages := toolRoundMessagesFromContext(ctx); len(toolRoundMessages) > 0 {
		messageList = toolRoundMessages
	}

	// Xây dựng system prompt
	systemPrompt := l.clientState.SystemPrompt
	globalSystemPrompt := strings.TrimSpace(viper.GetString("chat.global_system_prompt"))
	if globalSystemPrompt != "" {
		if systemPrompt != "" {
			systemPrompt = globalSystemPrompt + "\n\n" + systemPrompt
		} else {
			systemPrompt = globalSystemPrompt
		}
	}

	// Thêm thông tin thời gian và ngày hiện tại
	now := time.Now()
	systemPrompt += fmt.Sprintf("\nThời gian và ngày hiện tại: %s %s", now.Format("02/01/2006 15:04:05"), now.Format("Monday"))

	if memoryMode == MemoryModeLong && l.clientState.MemoryContext != "" {
		systemPrompt += fmt.Sprintf("\nThông tin cá nhân hóa người dùng: \n%s", l.clientState.MemoryContext)
	}

	log.Debugf("speakerResult: %+v, voiceIdentify: %+v", speakerResult, l.clientState.DeviceConfig.VoiceIdentify)

	// Tích hợp kết quả nhận dạng người nói vào systemPrompt
	if speakerResult != nil && speakerResult.Identified {
		// Khớp thông tin speakerGroup trong userConfig dựa trên speakerResult
		if l.clientState.DeviceConfig.VoiceIdentify != nil {
			// Ưu tiên khớp bằng SpeakerName (key của VoiceIdentify là speakerGroup.Name)
			if speakerGroupInfo, found := l.clientState.DeviceConfig.VoiceIdentify[speakerResult.SpeakerName]; found {
				// Nếu tìm thấy speakerGroup khớp, tích hợp mô tả vào systemPrompt
				if speakerGroupInfo.Prompt != "" {
					systemPrompt += fmt.Sprintf("\nThông tin người đối thoại nhận dạng được dựa trên vân giọng nói: \n%s", speakerGroupInfo.Prompt)
				}
			}
		}
	}

	//search memory
	if memoryMode == MemoryModeLong && l.clientState.MemoryProvider != nil && userMessage != nil {
		memoryContext, err := l.clientState.MemoryProvider.Search(ctx, l.clientState.GetDeviceIDOrAgentID(), userMessage.Content, 10, 180)
		if err != nil {
			log.Errorf("Tìm kiếm bộ nhớ thất bại: %v", err)
		}
		log.Debugf("Tìm kiếm bộ nhớ thành công, nội dung đầu vào: %s, nội dung bộ nhớ: %s", userMessage.Content, memoryContext)
		if memoryContext != "" {
			systemPrompt += fmt.Sprintf("\nThông tin liên quan từ lịch sử: \n%s", memoryContext)
		}
	}

	systemPrompt += buildKnowledgeSearchRoutingPolicy(l.clientState.DeviceConfig.KnowledgeBases)

	retMessage := make([]*schema.Message, 0)
	retMessage = append(retMessage, &schema.Message{
		Role:    schema.System,
		Content: systemPrompt,
	})
	// Lọc bỏ các tin nhắn assistant rỗng, tránh gây lỗi 400 khi gửi đến LLM API
	// Tin nhắn assistant rỗng (Content rỗng và ToolCalls rỗng) sẽ gây ra lỗi API
	for _, msg := range messageList {
		if msg != nil && msg.Role == schema.Assistant && msg.Content == "" && len(msg.ToolCalls) == 0 {
			log.Debugf("Lọc bỏ tin nhắn assistant rỗng, tránh gửi đến LLM API")
			continue
		}
		msgCopy := cloneMessageForRequest(msg)
		if isInterruptedMessage(msgCopy) {
			msgCopy.Content = decorateInterruptedContent(msgCopy.Content)
		}
		retMessage = append(retMessage, msgCopy)
	}
	if userMessage != nil {
		// Kiểm tra xem tin nhắn cuối cùng trong retMessage đã là tin nhắn người dùng giống hệt chưa, tránh thêm trùng lặp
		shouldAdd := true
		if len(retMessage) > 0 {
			lastMsg := retMessage[len(retMessage)-1]
			if lastMsg.Role == schema.User && lastMsg.Content == userMessage.Content {
				// Tin nhắn cuối cùng đã là tin nhắn người dùng giống hệt, bỏ qua việc thêm
				shouldAdd = false
				//log.Debugf("Tin nhắn cuối cùng đã là tin nhắn người dùng giống hệt, bỏ qua việc thêm trùng lặp: %s", userMessage.Content)
			}
		}
		if shouldAdd {
			retMessage = append(retMessage, userMessage)
		}
	}
	return retMessage
}

func buildKnowledgeSearchRoutingPolicy(knowledgeBases []config_types.KnowledgeBaseRef) string {
	if len(knowledgeBases) == 0 {
		return ""
	}

	availableKBs := make([]string, 0, len(knowledgeBases))
	for _, kb := range knowledgeBases {
		if strings.EqualFold(strings.TrimSpace(kb.Status), "inactive") {
			continue
		}
		if strings.TrimSpace(kb.ExternalKBID) == "" {
			continue
		}
		name := strings.TrimSpace(kb.Name)
		if name == "" {
			name = strings.TrimSpace(kb.ExternalKBID)
		}
		if name == "" {
			continue
		}
		if kb.ID == 0 {
			continue
		}
		desc := strings.TrimSpace(kb.Description)
		if desc == "" {
			desc = "Không có mô tả"
		}
		availableKBs = append(availableKBs, fmt.Sprintf("%d: Tên=%s; Mô tả=%s", kb.ID, name, desc))
		if len(availableKBs) >= 8 {
			break
		}
	}
	if len(availableKBs) == 0 {
		return ""
	}

	return fmt.Sprintf(
		"\nQuy tắc tra cứu cơ sở tri thức (công cụ: search_knowledge):\nCác cơ sở tri thức khả dụng (id:tên+mô tả): %s\n"+
			"1. Điều kiện kích hoạt: người dùng hỏi về sự kiện, quy trình, tham số, quy tắc, định nghĩa, điều khoản, so sánh v.v. cần căn cứ tài liệu, hoặc người dùng yêu cầu rõ ràng “trả lời theo cơ sở tri thức/tài liệu”.\n"+
			"2. Điều kiện không kích hoạt: trò chuyện phiếm, chào hỏi, đồng hành cảm xúc, sáng tác thuần túy, gợi ý mang tính chủ quan thuần túy.\n"+
			"3. Cách gọi: mỗi lượt gọi tối đa 1 lần, query trích xuất từ khóa cốt lõi của câu hỏi người dùng, top_k mặc định là 5; nếu có thể xác định cơ sở tri thức cụ thể, hãy truyền knowledge_base_ids (có thể nhiều).\n"+
			"4. Quy tắc lựa chọn: chỉ truyền ID cơ sở tri thức có ngữ nghĩa liên quan nhất đến câu hỏi hiện tại; nếu không thể xác định thì có thể không truyền knowledge_base_ids.\n"+
			"5. Xử lý khi thông tin không đủ: nếu bằng chứng không đủ, không được bịa đặt, hãy yêu cầu người dùng bổ sung từ khóa cụ thể hơn.\n"+
			"6. Yêu cầu đầu ra: khi trả lời, cấm đề cập đến “cơ sở tri thức”, “tra cứu”, “MCP”, “gọi công cụ”, “kết quả trúng” hoặc các thông tin nguồn/quy trình khác.",
		strings.Join(availableKBs, ", "),
	)
}

func trimTrailingUserMessages(messages []*schema.Message) []*schema.Message {
	end := len(messages)
	for end > 0 {
		msg := messages[end-1]
		if msg == nil || msg.Role != schema.User {
			break
		}
		end--
	}
	return messages[:end]
}

func isInterruptedMessage(msg *schema.Message) bool {
	if msg == nil || msg.Extra == nil {
		return false
	}
	v, ok := msg.Extra[interruptExtraKey]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	default:
		return false
	}
}

func decorateInterruptedContent(content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}
	if strings.HasSuffix(content, interruptContentSuffix) {
		return content
	}
	return content + interruptContentSuffix
}

func cloneMessagesForRequest(messages []*schema.Message) []*schema.Message {
	if len(messages) == 0 {
		return nil
	}

	cloned := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		cloned = append(cloned, cloneMessageForRequest(msg))
	}

	return cloned
}

func toolRoundMessagesFromContext(ctx context.Context) []*schema.Message {
	if ctx == nil {
		return nil
	}

	messages, ok := ctx.Value(toolRoundMessagesKey).([]*schema.Message)
	if !ok || len(messages) == 0 {
		return nil
	}

	return cloneMessagesForRequest(messages)
}

func ttsTurnTrackerFromContext(ctx context.Context) *ttsTurnTracker {
	if ctx == nil {
		return nil
	}

	tracker, ok := ctx.Value(ttsTurnTrackerKey).(*ttsTurnTracker)
	if !ok {
		return nil
	}

	return tracker
}

func ensureTTSTurnTrackerInContext(ctx context.Context) context.Context {
	if ttsTurnTrackerFromContext(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, ttsTurnTrackerKey, newTTSTurnTracker())
}

func waitForTTSTurnDrainIfRoot(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if nest, ok := ctx.Value("nest").(int); ok && nest > 1 {
		return nil
	}

	tracker := ttsTurnTrackerFromContext(ctx)
	if tracker == nil {
		return nil
	}

	return tracker.Wait(ctx)
}

func appendToolRoundMessagesToContext(ctx context.Context, messages []*schema.Message) context.Context {
	if len(messages) == 0 {
		return ctx
	}

	combined := toolRoundMessagesFromContext(ctx)
	combined = append(combined, cloneMessagesForRequest(messages)...)
	if len(combined) == 0 {
		return ctx
	}

	return context.WithValue(ctx, toolRoundMessagesKey, combined)
}

func cloneMessageForRequest(msg *schema.Message) *schema.Message {
	if msg == nil {
		return nil
	}
	msgCopy := *msg

	if msg.ToolCalls != nil {
		msgCopy.ToolCalls = append([]schema.ToolCall(nil), msg.ToolCalls...)
	}
	if msg.MultiContent != nil {
		msgCopy.MultiContent = append([]schema.ChatMessagePart(nil), msg.MultiContent...)
	}
	if msg.Extra != nil {
		msgCopy.Extra = make(map[string]any, len(msg.Extra))
		for k, v := range msg.Extra {
			msgCopy.Extra[k] = v
		}
	}
	if msg.ResponseMeta != nil {
		respMetaCopy := *msg.ResponseMeta
		msgCopy.ResponseMeta = &respMetaCopy
	}

	return &msgCopy
}
// friendlyErrorMessageFor chuyển lỗi kỹ thuật từ LLM provider (rate limit, timeout,
// lỗi xác thực...) thành câu trả lời tự nhiên, phù hợp persona trợ lý, thay vì đọc
// nguyên văn mã lỗi/tên provider cho người dùng nghe qua TTS.
func friendlyErrorMessageFor(errMsg string) string {
	lower := strings.ToLower(errMsg)
	switch {
	case strings.Contains(lower, "429") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests"):
		return "Xin lỗi bạn, mình đang hơi quá tải một chút, đợi vài giây rồi hỏi lại giúp mình nhé."
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded"):
		return "Mình xử lý hơi lâu quá, bạn thử hỏi lại câu đó lần nữa nhé."
	case strings.Contains(lower, "401") || strings.Contains(lower, "403") || strings.Contains(lower, "unauthorized"):
		return "Mình đang gặp chút trục trặc kỹ thuật, bạn báo với thầy cô hoặc quản trị viên giúp mình nhé."
	default:
		return "Mình đang gặp chút trục trặc, bạn thử hỏi lại giúp mình nhé."
	}
}