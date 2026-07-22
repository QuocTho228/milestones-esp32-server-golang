package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/schema"

	"milestones-esp32-server-golang/internal/domain/llm"
	llm_common "milestones-esp32-server-golang/internal/domain/llm/common"
	"milestones-esp32-server-golang/internal/pool"
	log "milestones-esp32-server-golang/logger"
)

var openClawWarmupSchedule = []time.Duration{
	1 * time.Second,
	10 * time.Second,
	20 * time.Second,
	30 * time.Second,
	40 * time.Second,
	50 * time.Second,
	60 * time.Second,
	70 * time.Second,
	80 * time.Second,
	90 * time.Second,
	100 * time.Second,
}

const (
	openClawWarmupPlanTimeout = 8 * time.Second
	openClawWarmupPlanSize    = 11
	// openClawWarmupHintMaxRunes giới hạn độ dài (theo số ký tự) của cụm chủ đề
	// được trích ra từ câu nói của người dùng, dùng để đưa vào prompt hệ thống.
	openClawWarmupHintMaxRunes = 30
)

const openClawWarmupSystemPrompt = `Bạn là trợ lý "giữ nhịp" trong một cuộc hội thoại thoại thời gian thực, không phải người trả lời chính.

Nhiệm vụ của bạn là: trước khi câu trả lời chính được trả về, hãy tạo ra 11 câu tiếng Việt rất ngắn để tiếp lời, giúp cho quá trình chờ đợi nghe như lúc nào cũng có người đang phản hồi.

Yêu cầu bắt buộc:
1. Chỉ đảm nhiệm việc giữ nhịp, không được trả lời trực tiếp câu hỏi, không được đưa ra sự kiện, kết luận, gợi ý, các bước, phân tích, giải thích hay suy đoán.
2. Giọng điệu phải giống người thật đang tiếp lời nhẹ nhàng trong cuộc gọi: ngắn gọn, tự nhiên, đời thường, kiên nhẫn.
3. Không được giống nhân viên chăm sóc khách hàng, không giống thông báo hệ thống, không giống bản tin, không giống văn bản quảng cáo.
4. Cấm nhắc lại nguyên văn câu nói của người dùng, đặc biệt không được ghép nguyên các chỉ lệnh như "giúp tôi kiểm tra", "giúp tôi xem", "giúp tôi tra cứu", "nói cho tôi biết" vào câu trả lời.
5. Nếu cần nhắc đến chủ đề, chỉ được rút gọn thành cụm danh từ theo góc nhìn của trợ lý, ví dụ "thời tiết Hà Nội ngày mốt", "lịch trình này"; không dùng câu mệnh lệnh.
6. 1 đến 2 câu đầu nên nhẹ nhàng hơn, không nhất thiết phải nhắc đến từ khóa chủ đề, ví dụ "để tôi xem", "chờ tôi chút"; không mở đầu ngay bằng lời trấn an quá nặng nề.
7. Các câu sau đó dần dần thể hiện "tôi vẫn đang xem", "tôi vẫn đang xác nhận", nhưng phải tự nhiên, không lặp lại một cách máy móc.
8. Tránh dùng các cách nói cứng nhắc như "đang xử lý cho bạn", "vui lòng chờ", "đang tiếp tục theo dõi", "đang truy xuất dữ liệu", "đang kết nối dịch vụ".
9. Mỗi câu phải là một câu ngắn, phù hợp để phát bằng giọng nói, độ dài kiểm soát trong khoảng 6 đến 16 ký tự.
10. Bạn sẽ nhận được các mốc thời gian phát thực tế. 11 câu thoại phải được thiết kế nghiêm ngặt theo thứ tự các mốc thời gian này:
   - Giây thứ 1: như vừa mới nhận câu hỏi, tiếp lời nhẹ nhàng.
   - Giây thứ 10: bổ sung một câu tự nhiên, giọng điệu vẫn nhẹ.
   - Giây thứ 20, 30: bắt đầu thể hiện "tôi vẫn đang xem", nhưng đừng máy móc.
   - Giây thứ 40, 50, 60: tiếp tục trấn an, có thể nói rõ hơn "vẫn đang xác nhận".
   - Giây thứ 70, 80, 90, 100: thừa nhận là hơi lâu, nhưng vẫn tự nhiên, bình tĩnh, không than phiền.
11. Chỉ xuất ra đúng một mảng JSON nghiêm ngặt, độ dài phải là 11.
12. Mỗi phần tử JSON phải có định dạng: {"text":"câu giữ nhịp"}.
13. Cấm xuất ra số thứ tự, Markdown, giải thích, khối mã hoặc bất kỳ nội dung nào ngoài JSON.`

type openClawWarmupTask struct {
	correlationID string
	sessionCtx    context.Context
	warmupCtx     context.Context
	cancelWarmup  context.CancelFunc

	linesMu sync.RWMutex
	lines   []string

	stateMu                  sync.Mutex
	speechStarted            bool
	speechEnded              bool
	nextWarmupSegmentIsStart bool
	planReadyAt              time.Time
	planReadySignaled        bool

	spokeAny    atomic.Bool
	planReadyCh chan struct{}
}

type openClawWarmupLine struct {
	Text string `json:"text"`
}

func (s *ChatSession) startOpenClawWarmup(correlationID string, userText string) {
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" || s == nil || s.clientState == nil {
		return
	}

	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	parentCtx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	warmupCtx, cancelWarmup := context.WithCancel(parentCtx)
	task := &openClawWarmupTask{
		correlationID:            correlationID,
		sessionCtx:               parentCtx,
		warmupCtx:                warmupCtx,
		cancelWarmup:             cancelWarmup,
		lines:                    make([]string, openClawWarmupPlanSize),
		nextWarmupSegmentIsStart: true,
		planReadyCh:              make(chan struct{}),
	}

	s.replaceOpenClawWarmup(task)
	log.Infof("OpenClaw warmup started: device=%s correlation_id=%s", s.clientState.DeviceID, correlationID)

	go s.runOpenClawWarmupTask(task, userText)
}

func (s *ChatSession) replaceOpenClawWarmup(task *openClawWarmupTask) {
	s.openClawWarmupMu.Lock()
	oldTask := s.openClawWarmup
	s.openClawWarmup = task
	s.openClawWarmupMu.Unlock()

	if oldTask != nil {
		oldTask.cancelWarmupOnly()
	}
}

func (task *openClawWarmupTask) cancelWarmupOnly() {
	if task == nil || task.cancelWarmup == nil {
		return
	}
	task.cancelWarmup()
}

func (task *openClawWarmupTask) markSpeechStarted() bool {
	if task == nil {
		return false
	}
	task.stateMu.Lock()
	defer task.stateMu.Unlock()
	if task.speechStarted || task.speechEnded {
		return false
	}
	task.speechStarted = true
	return true
}

func (task *openClawWarmupTask) markSpeechEnded() bool {
	if task == nil {
		return false
	}
	task.stateMu.Lock()
	defer task.stateMu.Unlock()
	if !task.speechStarted || task.speechEnded {
		return false
	}
	task.speechEnded = true
	return true
}

func (task *openClawWarmupTask) takeWarmupSegmentStartFlag() bool {
	if task == nil {
		return true
	}
	task.stateMu.Lock()
	defer task.stateMu.Unlock()
	isStart := task.nextWarmupSegmentIsStart
	task.nextWarmupSegmentIsStart = false
	return isStart
}

func (task *openClawWarmupTask) markPlanReady(readyAt time.Time) {
	if task == nil {
		return
	}
	task.stateMu.Lock()
	if task.planReadySignaled {
		task.stateMu.Unlock()
		return
	}
	task.planReadyAt = readyAt
	task.planReadySignaled = true
	close(task.planReadyCh)
	task.stateMu.Unlock()
}

func (task *openClawWarmupTask) waitPlanReady(ctx context.Context) (time.Time, bool) {
	if task == nil {
		return time.Time{}, false
	}

	select {
	case <-ctx.Done():
		return time.Time{}, false
	case <-task.planReadyCh:
	}

	task.stateMu.Lock()
	defer task.stateMu.Unlock()
	if task.planReadyAt.IsZero() {
		return time.Time{}, false
	}
	return task.planReadyAt, true
}

func (task *openClawWarmupTask) hasSpokenAny() bool {
	if task == nil {
		return false
	}
	return task.spokeAny.Load()
}

func (s *ChatSession) getOpenClawWarmupTask(correlationID string) *openClawWarmupTask {
	if s == nil {
		return nil
	}
	correlationID = strings.TrimSpace(correlationID)
	s.openClawWarmupMu.Lock()
	defer s.openClawWarmupMu.Unlock()
	task := s.openClawWarmup
	if task == nil {
		return nil
	}
	if correlationID != "" && task.correlationID != correlationID {
		return nil
	}
	return task
}

func (s *ChatSession) takeOpenClawWarmupTask(correlationID string) *openClawWarmupTask {
	if s == nil {
		return nil
	}
	correlationID = strings.TrimSpace(correlationID)
	s.openClawWarmupMu.Lock()
	defer s.openClawWarmupMu.Unlock()
	task := s.openClawWarmup
	if task == nil {
		return nil
	}
	if correlationID != "" && task.correlationID != correlationID {
		return nil
	}
	s.openClawWarmup = nil
	return task
}

func (s *ChatSession) cancelOpenClawWarmup(correlationID string, interrupt bool) bool {
	if s == nil {
		return false
	}

	task := s.getOpenClawWarmupTask(correlationID)
	if task == nil {
		return false
	}
	if task.warmupCtx.Err() != nil {
		return false
	}

	task.cancelWarmupOnly()
	if interrupt && task.hasSpokenAny() {
		s.InterruptAndClearTTSQueueWithReason(fmt.Sprintf("OpenClaw warmup canceled correlation_id=%s", correlationID))
	}

	log.Infof(
		"OpenClaw warmup canceled: device=%s correlation_id=%s interrupt=%v spoke_any=%v",
		s.clientState.DeviceID,
		task.correlationID,
		interrupt,
		task.hasSpokenAny(),
	)
	return true
}

func (s *ChatSession) finishOpenClawWarmup(correlationID string, interrupt bool) bool {
	task := s.takeOpenClawWarmupTask(correlationID)
	if task == nil {
		return false
	}

	task.cancelWarmupOnly()
	if interrupt {
		s.InterruptAndClearTTSQueueWithReason(fmt.Sprintf("OpenClaw warmup finished correlation_id=%s interrupt", correlationID))
	}
	s.endOpenClawSpeech(task)

	log.Infof(
		"OpenClaw warmup finished: device=%s correlation_id=%s interrupt=%v spoke_any=%v",
		s.clientState.DeviceID,
		task.correlationID,
		interrupt,
		task.hasSpokenAny(),
	)
	return true
}

func (s *ChatSession) beginOpenClawSpeech(task *openClawWarmupTask) {
	if task == nil {
		return
	}
	if !task.markSpeechStarted() {
		return
	}
	s.ttsManager.ClearAudioHistory()
	s.ttsManager.EnqueueTtsStartWithReason(task.sessionCtx, fmt.Sprintf("OpenClaw warmup start correlation_id=%s", task.correlationID))
}

func (s *ChatSession) endOpenClawSpeech(task *openClawWarmupTask) {
	if task == nil {
		return
	}
	if !task.markSpeechEnded() {
		return
	}
	s.ttsManager.GetAndClearAudioHistory()
}

func (s *ChatSession) runOpenClawWarmupTask(task *openClawWarmupTask, userText string) {
	planCtx, cancel := context.WithTimeout(task.warmupCtx, openClawWarmupPlanTimeout)
	defer cancel()
	defer log.Infof(
		"OpenClaw warmup task stopped: device=%s correlation_id=%s warmup_err=%v session_err=%v spoke_any=%v",
		s.clientState.DeviceID,
		task.correlationID,
		task.warmupCtx.Err(),
		task.sessionCtx.Err(),
		task.hasSpokenAny(),
	)

	go func() {
		lines, err := s.generateOpenClawWarmupPlan(planCtx, task.correlationID, userText)
		if err != nil {
			if planCtx.Err() == nil {
				log.Warnf("OpenClaw warmup plan generation failed: device=%s correlation_id=%s err=%v", s.clientState.DeviceID, task.correlationID, err)
			}
			task.markPlanReady(time.Time{})
			return
		}
		task.setLines(lines)
		task.markPlanReady(time.Now())
		log.Infof("OpenClaw warmup plan ready: device=%s correlation_id=%s line_count=%d", s.clientState.DeviceID, task.correlationID, len(lines))
	}()

	baseAt, ok := task.waitPlanReady(task.warmupCtx)
	if !ok {
		return
	}

	for idx, delay := range openClawWarmupSchedule {
		if !waitOpenClawWarmupUntil(task.warmupCtx, baseAt.Add(delay)) {
			return
		}
		if task.warmupCtx.Err() != nil {
			return
		}

		text := task.lineAt(idx)
		if text == "" {
			continue
		}

		log.Infof(
			"OpenClaw warmup speaking: device=%s correlation_id=%s slot=%d text=%q",
			s.clientState.DeviceID,
			task.correlationID,
			idx,
			text,
		)
		if err := s.speakOpenClawWarmupLine(task, text); err != nil && task.sessionCtx.Err() == nil {
			log.Warnf("OpenClaw warmup speak failed: device=%s correlation_id=%s slot=%d err=%v", s.clientState.DeviceID, task.correlationID, idx, err)
			return
		}
		task.spokeAny.Store(true)
	}

	// Không dọn dẹp active task ở đây: câu âm thanh giữ nhịp cuối cùng có thể vẫn đang
	// được gửi/phát, cần tiếp tục cho phép thực hiện ngắt ưu tiên khi câu đầu tiên của
	// OpenClaw đến.
}

func waitOpenClawWarmupUntil(ctx context.Context, deadline time.Time) bool {
	wait := time.Until(deadline)
	if wait <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (task *openClawWarmupTask) setLines(lines []string) {
	if task == nil || len(lines) == 0 {
		return
	}

	task.linesMu.Lock()
	defer task.linesMu.Unlock()

	if task.lines == nil {
		task.lines = make([]string, openClawWarmupPlanSize)
	}
	for idx := 0; idx < openClawWarmupPlanSize && idx < len(lines); idx++ {
		if text := sanitizeOpenClawWarmupText(lines[idx]); text != "" {
			task.lines[idx] = text
		}
	}
}

func (task *openClawWarmupTask) lineAt(index int) string {
	if task == nil || index < 0 {
		return ""
	}

	task.linesMu.RLock()
	defer task.linesMu.RUnlock()

	if index >= len(task.lines) {
		return ""
	}
	return strings.TrimSpace(task.lines[index])
}

func (s *ChatSession) speakOpenClawWarmupLine(task *openClawWarmupTask, text string) error {
	text = sanitizeOpenClawWarmupText(text)
	if text == "" {
		return nil
	}
	if task == nil {
		return nil
	}
	if task.sessionCtx.Err() != nil {
		return task.sessionCtx.Err()
	}

	s.beginOpenClawSpeech(task)
	if task.sessionCtx.Err() != nil {
		return task.sessionCtx.Err()
	}

	resp := llm_common.LLMResponseStruct{
		Text:    text,
		IsStart: task.takeWarmupSegmentStartFlag(),
		IsEnd:   true,
	}
	// Câu giữ nhịp cần đảm bảo đã đi vào luồng gửi, tránh bị phản hồi chính thức
	// sau đó "trông như không có tác dụng".
	return s.ttsManager.handleTextResponse(task.sessionCtx, resp, true)
}

func (s *ChatSession) generateOpenClawWarmupPlan(ctx context.Context, correlationID string, userText string) ([]string, error) {
	llmWrapper, err := pool.Acquire[llm.LLMProvider](
		"llm",
		s.clientState.DeviceConfig.Llm.Provider,
		s.clientState.DeviceConfig.Llm.Config,
	)
	if err != nil {
		return nil, fmt.Errorf("acquire llm provider: %w", err)
	}
	defer pool.Release(llmWrapper)

	dialogue := []*schema.Message{
		schema.SystemMessage(openClawWarmupSystemPrompt),
		schema.UserMessage(buildOpenClawWarmupUserPrompt(userText)),
	}

	msgChan := llmWrapper.GetProvider().ResponseWithContext(
		ctx,
		buildOpenClawWarmupSessionID(s.clientState.SessionID, correlationID),
		dialogue,
		nil,
	)

	raw, err := collectOpenClawWarmupResponse(ctx, msgChan)
	if err != nil {
		return nil, err
	}
	lines := parseOpenClawWarmupPlan(raw)
	if countOpenClawWarmupLines(lines) == 0 {
		return nil, fmt.Errorf("empty warmup plan")
	}
	return lines, nil
}

func buildOpenClawWarmupUserPrompt(userText string) string {
	trimmed := strings.TrimSpace(userText)
	topic := formatOpenClawWarmupTopic(buildOpenClawWarmupHint(userText))
	topicLine := "Không nhắc lại các chỉ lệnh của người dùng kiểu \"giúp tôi kiểm tra\"."
	if topic != "" {
		topicLine = fmt.Sprintf("Nếu cần nhắc đến chủ đề, chỉ được rút gọn thành cụm danh từ \"%s\", không nhắc lại các chỉ lệnh của người dùng kiểu \"giúp tôi kiểm tra\".", topic)
	}
	return fmt.Sprintf(
		"Nhiệm vụ của người dùng trong lượt này:\n%s\n\n%s\n\nCác mốc thời gian phát thực tế lần lượt là: giây thứ 1, giây thứ 10, giây thứ 20, giây thứ 30, giây thứ 40, giây thứ 50, giây thứ 60, giây thứ 70, giây thứ 80, giây thứ 90, giây thứ 100.\nVui lòng xuất ra 11 câu giữ nhịp, tương ứng lần lượt với 11 mốc thời gian nêu trên.",
		trimmed,
		topicLine,
	)
}

func buildOpenClawWarmupSessionID(sessionID string, correlationID string) string {
	base := strings.TrimSpace(sessionID)
	if base == "" {
		base = "openclaw"
	}
	correlationID = strings.TrimSpace(correlationID)
	if len(correlationID) > 12 {
		correlationID = correlationID[:12]
	}
	if correlationID == "" {
		return base + ":warmup"
	}
	return base + ":warmup:" + correlationID
}

func collectOpenClawWarmupResponse(ctx context.Context, msgChan chan *schema.Message) (string, error) {
	var builder strings.Builder

	for {
		select {
		case <-ctx.Done():
			return builder.String(), ctx.Err()
		case msg, ok := <-msgChan:
			if !ok {
				return builder.String(), nil
			}
			if msg == nil {
				continue
			}
			if llm.IsLLMErrorMessage(msg) {
				errMsg := strings.TrimSpace(llm.LLMErrorMessage(msg))
				if errMsg == "" {
					errMsg = "unknown llm error"
				}
				return builder.String(), fmt.Errorf("llm returned error: %s", errMsg)
			}
			if msg.Content != "" {
				builder.WriteString(msg.Content)
			}
		}
	}
}

func parseOpenClawWarmupPlan(raw string) []string {
	lines := make([]string, openClawWarmupPlanSize)

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return lines
	}

	candidate := raw
	start := strings.Index(candidate, "[")
	end := strings.LastIndex(candidate, "]")
	if start >= 0 && end > start {
		candidate = candidate[start : end+1]
	}

	var objectItems []openClawWarmupLine
	if err := json.Unmarshal([]byte(candidate), &objectItems); err == nil {
		return buildOpenClawWarmupPlanLines(objectItemsToStrings(objectItems))
	}

	var stringItems []string
	if err := json.Unmarshal([]byte(candidate), &stringItems); err == nil {
		return buildOpenClawWarmupPlanLines(stringItems)
	}

	log.Warnf("OpenClaw warmup plan parse failed, ignored: raw=%q", raw)
	return lines
}

func objectItemsToStrings(items []openClawWarmupLine) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, item.Text)
	}
	return lines
}

func buildOpenClawWarmupPlanLines(items []string) []string {
	lines := make([]string, openClawWarmupPlanSize)
	for idx := 0; idx < openClawWarmupPlanSize && idx < len(items); idx++ {
		if text := sanitizeOpenClawWarmupText(items[idx]); text != "" {
			lines[idx] = text
		}
	}
	return lines
}

func countOpenClawWarmupLines(lines []string) int {
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func sanitizeOpenClawWarmupText(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "\"'`[]{}")
	text = strings.TrimLeft(text, "0123456789.,- ")
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return ""
	}
	if isInvalidOpenClawWarmupText(text) {
		return ""
	}

	runes := []rune(text)
	if len(runes) > 16 {
		return ""
	}
	return text
}

// isInvalidOpenClawWarmupText phát hiện các câu "giữ nhịp" bị mô hình lặp lại
// nguyên văn chỉ lệnh của người dùng (ví dụ "giúp tôi kiểm tra..."), là dấu hiệu
// vi phạm yêu cầu số 4 trong system prompt.
func isInvalidOpenClawWarmupText(text string) bool {
	for _, bad := range []string{
		"giúp tôi",
		"giúp mình",
		"cho tôi",
		"nói cho tôi biết",
		"làm ơn giúp",
		"phiền bạn giúp",
		"phiền giúp",
		"có thể giúp tôi",
		"có thể giúp",
		"bạn giúp tôi",
	} {
		if strings.Contains(text, bad) {
			return true
		}
	}
	return false
}

// buildOpenClawWarmupHint trích ra một cụm danh từ chủ đề ngắn gọn từ câu nói
// của người dùng (ví dụ "thời tiết Hà Nội ngày mai"), để đưa vào prompt hệ thống
// làm gợi ý chủ đề cho các câu giữ nhịp.
func buildOpenClawWarmupHint(userText string) string {
	trimmed := strings.TrimSpace(userText)
	if trimmed == "" {
		return ""
	}

	normalized := removePunctuation(trimmed)
	if normalized == "" {
		return ""
	}
	normalized = trimOpenClawWarmupCommandPrefix(normalized)
	normalized = trimOpenClawWarmupQuestionSuffix(normalized)
	if normalized == "" {
		return ""
	}

	for _, keyword := range []string{"thời tiết", "nhiệt độ", "độ ẩm", "dự báo"} {
		if idx := strings.Index(normalized, keyword); idx >= 0 {
			// Trong tiếng Việt, từ khoá chủ đề thường đứng trước các thành phần bổ
			// nghĩa (địa điểm, thời gian), khác với tiếng Trung (từ khoá thường
			// đứng sau). Vì vậy ta giữ lại phần từ vị trí từ khoá đến hết chuỗi,
			// thay vì cắt bỏ phần sau từ khoá như bản gốc tiếng Trung.
			normalized = normalized[idx:]
			break
		}
	}

	runes := []rune(normalized)
	if len(runes) > openClawWarmupHintMaxRunes {
		runes = runes[:openClawWarmupHintMaxRunes]
	}
	normalized = strings.TrimSpace(string(runes))
	normalized = trimOpenClawWarmupQuestionSuffix(normalized)
	return normalized
}

func trimOpenClawWarmupCommandPrefix(text string) string {
	trimmed := strings.TrimSpace(text)
	for {
		changed := false
		for _, prefix := range []string{
			"phiền bạn giúp tôi tra cứu một chút",
			"phiền bạn giúp tôi kiểm tra một chút",
			"phiền bạn giúp tôi xem một chút",
			"làm ơn giúp tôi tra cứu một chút",
			"làm ơn giúp tôi kiểm tra một chút",
			"làm ơn giúp tôi xem một chút",
			"giúp tôi tra cứu một chút",
			"giúp tôi kiểm tra một chút",
			"giúp tôi xem một chút",
			"giúp tôi hỏi một chút",
			"cho tôi tra cứu một chút",
			"cho tôi kiểm tra một chút",
			"cho tôi xem một chút",
			"có thể giúp tôi kiểm tra một chút",
			"có thể giúp tôi xem một chút",
			"bạn giúp tôi kiểm tra được không",
			"bạn giúp tôi xem được không",
			"tôi muốn biết",
			"tôi muốn hỏi một chút",
			"tôi muốn hỏi",
			"làm ơn cho hỏi một chút",
			"làm ơn cho hỏi",
			"tra cứu một chút",
			"kiểm tra một chút",
			"xem một chút",
			"hỏi một chút",
			"giúp tôi tra cứu",
			"giúp tôi kiểm tra",
			"giúp tôi xem",
			"giúp tôi hỏi",
			"cho tôi tra cứu",
			"cho tôi kiểm tra",
			"cho tôi xem",
			"tra cứu",
			"kiểm tra",
			"xem",
			"hỏi",
		} {
			if strings.HasPrefix(trimmed, prefix) {
				trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
				changed = true
				break
			}
		}
		if !changed {
			break
		}
	}
	return trimmed
}

func trimOpenClawWarmupQuestionSuffix(text string) string {
	trimmed := strings.TrimSpace(text)
	for _, suffix := range []string{
		"thế nào",
		"như thế nào",
		"bao nhiêu",
		"là gì",
		"là cái gì",
		"à",
		"nhỉ",
		"vậy",
		"nhé",
	} {
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, suffix))
	}
	return trimmed
}

// formatOpenClawWarmupTopic chuẩn hoá khoảng trắng của cụm chủ đề trước khi
// đưa vào prompt. Bản gốc tiếng Trung còn chèn thêm trợ từ sở hữu "的" giữa
// phần bổ nghĩa và từ khoá (vì từ khoá nằm ở cuối cụm); tiếng Việt không cần
// trợ từ nối vì buildOpenClawWarmupHint đã đưa từ khoá lên đầu cụm, nên hàm
// này chỉ còn nhiệm vụ dọn khoảng trắng thừa.
func formatOpenClawWarmupTopic(hint string) string {
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return ""
	}
	return strings.Join(strings.Fields(hint), " ")
}