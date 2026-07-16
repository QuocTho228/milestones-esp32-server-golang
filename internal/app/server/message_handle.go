package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"hash/fnv"
	"runtime"
	"sync"
	"time"

	data_client "milestones-esp32-server-golang/internal/data/client"
	"milestones-esp32-server-golang/internal/data/history"
	"milestones-esp32-server-golang/internal/domain/eventbus"
	"milestones-esp32-server-golang/internal/domain/memory/llm_memory"
	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

var (
	// MessageWorkerNum số lượng worker xử lý tin nhắn (dựa trên số nhân CPU, cấu hình thống nhất, dùng cho xử lý Redis+History)
	// Phải là lũy thừa của 2 để thuận tiện cho việc phân phối theo hash
	MessageWorkerNum = getMessageWorkerNum()
)

// getMessageWorkerNum tính số lượng worker dựa trên số nhân CPU, làm tròn lên lũy thừa gần nhất của 2
// Giá trị nhỏ nhất là 4, giá trị lớn nhất là 64
func getMessageWorkerNum() int {
	cpuNum := runtime.NumCPU()

	// Giá trị nhỏ nhất là 4, giá trị lớn nhất là 64
	if cpuNum < 4 {
		return 4
	}
	if cpuNum > 64 {
		return 64
	}

	// Làm tròn lên lũy thừa gần nhất của 2
	power := 1
	for power < cpuNum {
		power <<= 1
	}
	return power
}

// MessageWorker trình xử lý tin nhắn
// Sử dụng một pool goroutine với số lượng cố định, định tuyến theo giá trị hash của SessionID, đảm bảo tin nhắn trong cùng một phiên được xử lý theo đúng thứ tự
// Xử lý thống nhất tin nhắn cho Redis, MemoryProvider và History
type MessageWorker struct {
	client  *history.HistoryClient
	workers []chan *eventbus.AddMessageEvent // channel của từng worker
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewMessageWorker tạo trình xử lý tin nhắn
func NewMessageWorker(cfg history.HistoryClientConfig) *MessageWorker {
	client := history.NewHistoryClient(cfg)
	ctx, cancel := context.WithCancel(context.Background())

	worker := &MessageWorker{
		client:  client,
		workers: make([]chan *eventbus.AddMessageEvent, MessageWorkerNum),
		ctx:     ctx,
		cancel:  cancel,
	}

	// Khởi tạo channel cho từng worker và khởi động goroutine
	for i := 0; i < MessageWorkerNum; i++ {
		worker.workers[i] = make(chan *eventbus.AddMessageEvent, 100) // đệm (buffer) 100 tin nhắn
		worker.wg.Add(1)
		go worker.workerLoop(i)
	}

	worker.subscribeEvents()
	log.Infof("Quá trình khởi tạo MessageWorker hoàn tất, bắt đầu %d goroutine worker (để xử lý Redis+MemoryProvider+History một cách đồng nhất).", MessageWorkerNum)
	return worker
}

// workerLoop vòng lặp xử lý của từng worker (đảm bảo xử lý tuần tự)
func (w *MessageWorker) workerLoop(index int) {
	defer w.wg.Done()
	defer log.Infof("MessageWorker worker %d kết thúc", index)

	ch := w.workers[index]
	for {
		select {
		case <-w.ctx.Done():
			// Dọn dẹp các tin nhắn còn lại trong channel
			for {
				select {
				case event := <-ch:
					if event != nil {
						w.processMessage(event)
					}
				default:
					return
				}
			}
		case event, ok := <-ch:
			if !ok {
				// channel đã đóng
				return
			}
			if event != nil {
				w.processMessage(event)
			}
		}
	}
}

// processMessage xử lý tin nhắn (thực thi tuần tự trong worker goroutine)
// Xử lý thống nhất Redis, MemoryProvider và History, đảm bảo tin nhắn của cùng một thiết bị/phiên được xử lý theo đúng thứ tự
func (w *MessageWorker) processMessage(event *eventbus.AddMessageEvent) {
	// 1. Xử lý History (tất cả tin nhắn)
	// Sử dụng context độc lập, không bị ảnh hưởng bởi event.ClientState.Ctx, đảm bảo việc lưu tin nhắn lịch sử không bị ảnh hưởng khi cuộc hội thoại bị hủy
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Xác định là thêm mới hay cập nhật
	if event.IsUpdate {
		// Giai đoạn 2: cập nhật âm thanh
		w.updateMessageAudio(ctx, event)
	} else {
		// Giai đoạn 1: lưu tin nhắn văn bản (bao gồm cả xử lý Redis)
		w.saveMessageText(ctx, event)
	}

	// 2. Xử lý MemoryProvider (chỉ khi !IsUpdate, độc lập với redis và manager)
	// Xử lý bộ nhớ dài hạn (memobase/mem0), cần thiết cho cả trường hợp redis lẫn manager
	if !event.IsUpdate {
		w.processMemoryProvider(event)
	}
}

// processMemoryProvider xử lý bộ nhớ dài hạn (memobase/mem0)
// Độc lập với redis và manager, cần thiết cho cả trường hợp redis lẫn manager
func (w *MessageWorker) processMemoryProvider(event *eventbus.AddMessageEvent) {
	clientState := event.ClientState
	if clientState.MemoryProvider == nil {
		return
	}
	if clientState.GetMemoryMode() != data_client.MemoryModeLong {
		return
	}

	err := clientState.MemoryProvider.AddMessage(
		clientState.Ctx,
		clientState.GetDeviceIDOrAgentID(),
		event.Msg)
	if err != nil {
		log.Errorf("add message to memory provider failed: %v", err)
	}
}

// hashSessionID tính giá trị hash của SessionID, trả về chỉ số (index) của worker
func (w *MessageWorker) hashSessionID(sessionID string) int {
	if sessionID == "" {
		return 0 // Nếu SessionID rỗng, sử dụng worker đầu tiên
	}

	// Sử dụng hàm hash FNV-1a
	h := fnv.New32a()
	h.Write([]byte(sessionID))
	hash := h.Sum32()
	return int(hash) % MessageWorkerNum
}

// subscribeEvents đăng ký (subscribe) sự kiện từ EventBus
func (w *MessageWorker) subscribeEvents() {
	bus := eventbus.Get()
	// Đăng ký sự kiện thêm tin nhắn thống nhất (lắng nghe cùng Topic với EventHandle)
	bus.Subscribe(eventbus.TopicAddMessage, w.handleAddMessage)
}

// handleAddMessage xử lý thống nhất sự kiện thêm tin nhắn (định tuyến đến worker tương ứng)
func (w *MessageWorker) handleAddMessage(event *eventbus.AddMessageEvent) {
	if event == nil || event.ClientState == nil {
		return
	}

	// Xác định key dùng để định tuyến: ưu tiên dùng SessionID, nếu rỗng thì dùng DeviceID
	key := event.ClientState.SessionID
	if key == "" {
		key = event.ClientState.DeviceID
	}
	if key == "" {
		log.Warnf("Nếu cả SessionID và DeviceID đều trống, tin nhắn sẽ không thể được định tuyến.")
		return
	}

	// Tính giá trị hash, định tuyến đến worker tương ứng
	workerIndex := w.hashSessionID(key)

	// Gửi không chặn (non-blocking) đến channel của worker tương ứng
	select {
	case w.workers[workerIndex] <- event:
		// Gửi thành công
	default:
		// channel đã đầy, ghi lại cảnh báo (thường không xảy ra vì channel có bộ đệm)
		log.Warnf("worker %d của channel đã đầy, tin nhắn bị loại bỏ, session_id: %s, device_id: %s",
			workerIndex, event.ClientState.SessionID, event.ClientState.DeviceID)
	}
}

// saveMessageText lưu tin nhắn văn bản (giai đoạn 1, hoặc lưu một lần cả văn bản+âm thanh)
// Bao gồm xử lý Redis (khi config_provider.type là redis)
func (w *MessageWorker) saveMessageText(ctx context.Context, event *eventbus.AddMessageEvent) {
	// Xử lý Redis (chỉ khi config_provider.type là redis)
	// Thêm vào danh sách tin nhắn Redis (dùng cho ngữ cảnh LLM)
	providerType := viper.GetString("config_provider.type")
	if providerType == "redis" {
		clientState := event.ClientState
		llm_memory.Get().AddMessage(
			clientState.Ctx,
			clientState.DeviceID,
			clientState.AgentID,
			event.Msg)
		return
	}

	// Xác định vai trò (role) của tin nhắn
	var role history.MessageType
	switch event.Msg.Role {
	case schema.User:
		role = history.MessageTypeUser
	case schema.Assistant:
		role = history.MessageTypeAssistant
	case schema.Tool:
		role = history.MessageTypeTool
	case schema.System:
		role = history.MessageTypeSystem
	default:
		log.Warnf("Vai trò tin nhắn không được hỗ trợ: %s", event.Msg.Role)
		return
	}

	// Chuyển đổi định dạng âm thanh (nếu có)
	var audioBase64 string
	var audioFormat string
	var audioSize int

	if len(event.AudioData) > 0 {
		// Tin nhắn ASR: văn bản và âm thanh được lấy đồng thời, lưu một lần
		var wavData []byte
		var err error

		// Chọn phương thức chuyển đổi âm thanh khác nhau tùy theo vai trò của tin nhắn
		if event.Msg.Role == schema.User {
			// Tin nhắn User (ASR): định dạng PCM float32
			if len(event.AudioData) > 0 {
				wavData, err = util.PCMFloat32BytesToWav(
					event.AudioData[0], // Tin nhắn User chỉ có một phần tử
					event.SampleRate,
					event.Channels)
			}
		} else {
			// Tin nhắn Assistant (TTS): định dạng Opus (về lý thuyết không nên xảy ra ở đây vì Assistant được lưu qua 2 giai đoạn)
			wavData, err = util.OpusFramesToWav(
				event.AudioData,
				event.SampleRate,
				event.Channels)
		}

		if err != nil {
			log.Errorf("Chuyển đổi âm thanh không thành công, device_id: %s, message_id: %s, role: %s, error: %v",
				event.ClientState.DeviceID, event.MessageID, event.Msg.Role, err)
			// Xử lý dự phòng (fallback): ghép trực tiếp tất cả các frame
			var fallbackData []byte
			for _, frame := range event.AudioData {
				fallbackData = append(fallbackData, frame...)
			}
			audioBase64 = base64.StdEncoding.EncodeToString(fallbackData)
			audioSize = event.AudioSize
			audioFormat = "raw" // Xử lý dự phòng dùng định dạng gốc (raw)
		} else {
			audioBase64 = base64.StdEncoding.EncodeToString(wavData)
			audioSize = len(wavData)
			audioFormat = "wav"
		}
	}

	// Xây dựng Metadata (chỉ lưu timestamp)
	metadata := map[string]interface{}{
		"timestamp": event.Timestamp.Format(time.RFC3339),
	}

	// Chuẩn bị các trường liên quan đến tool call
	var toolCallID string
	var toolCallsJSON *string

	// Vai trò Tool: lưu tool_call_id
	if event.Msg.Role == schema.Tool && event.Msg.ToolCallID != "" {
		toolCallID = event.Msg.ToolCallID
	}

	// Vai trò Assistant: lưu ToolCalls (nếu có)
	if event.Msg.Role == schema.Assistant && len(event.Msg.ToolCalls) > 0 {
		// Serialize ToolCalls thành chuỗi JSON
		toolCallsBytes, err := json.Marshal(event.Msg.ToolCalls)
		if err != nil {
			log.Warnf("Công cụ tuần tự ToolCalls thất bại, device_id: %s, message_id: %s, error: %v",
				event.ClientState.DeviceID, event.MessageID, err)
		} else {
			jsonStr := string(toolCallsBytes)
			toolCallsJSON = &jsonStr
		}
	}

	req := &history.SaveMessageRequest{
		MessageID:     event.MessageID,
		DeviceID:      event.ClientState.DeviceID,
		AgentID:       event.ClientState.AgentID,
		SessionID:     event.ClientState.SessionID,
		Role:          role,
		Content:       event.Msg.Content,
		ToolCallID:    toolCallID,
		ToolCallsJSON: toolCallsJSON,
		AudioData:     audioBase64,
		AudioFormat:   audioFormat,
		AudioSize:     audioSize,
		Metadata:      metadata,
	}

	if err := w.client.SaveMessage(ctx, req); err != nil {
		log.Errorf("Không lưu được tin nhắn, device_id: %s, message_id: %s, error: %v",
			event.ClientState.DeviceID, event.MessageID, err)
	}
}

// updateMessageAudio cập nhật âm thanh của tin nhắn (giai đoạn 2)
func (w *MessageWorker) updateMessageAudio(ctx context.Context, event *eventbus.AddMessageEvent) {
	// Chuyển đổi định dạng âm thanh
	var audioBase64 string
	var audioSize int

	if len(event.AudioData) > 0 {
		var wavData []byte
		var err error

		// Chọn phương thức chuyển đổi âm thanh khác nhau tùy theo vai trò của tin nhắn
		// Tin nhắn User (ASR): định dạng PCM float32, dùng PCMFloat32BytesToWav
		// Tin nhắn Assistant (TTS): định dạng Opus, dùng OpusFramesToWav
		if event.Msg.Role == schema.User {
			// Tin nhắn User: định dạng PCM float32
			// event.AudioData là [][]byte, nhưng tin nhắn User chỉ có một phần tử (mảng byte PCM float32 đầy đủ)
			if len(event.AudioData) > 0 {
				wavData, err = util.PCMFloat32BytesToWav(
					event.AudioData[0], // Tin nhắn User chỉ có một phần tử
					event.SampleRate,
					event.Channels)
			}
		} else {
			// Tin nhắn Assistant: định dạng Opus
			wavData, err = util.OpusFramesToWav(
				event.AudioData,
				event.SampleRate,
				event.Channels)
		}

		if err != nil {
			log.Errorf("Chuyển đổi âm thanh không thành công, device_id: %s, message_id: %s, role: %s, error: %v",
				event.ClientState.DeviceID, event.MessageID, event.Msg.Role, err)
			// Xử lý dự phòng (fallback): ghép trực tiếp tất cả các frame
			var fallbackData []byte
			for _, frame := range event.AudioData {
				fallbackData = append(fallbackData, frame...)
			}
			audioBase64 = base64.StdEncoding.EncodeToString(fallbackData)
			audioSize = event.AudioSize
		} else {
			audioBase64 = base64.StdEncoding.EncodeToString(wavData)
			audioSize = len(wavData)
		}
	}

	// Xây dựng yêu cầu cập nhật
	req := &history.UpdateMessageAudioRequest{
		MessageID:   event.MessageID,
		AudioData:   audioBase64,
		AudioFormat: "wav",
		AudioSize:   audioSize,
		Metadata: map[string]interface{}{
			"tts_duration": event.TTSDuration,
		},
	}

	// Gọi API cập nhật
	if err := w.client.UpdateMessageAudio(ctx, req); err != nil {
		log.Errorf("Cập nhật âm thanh tin nhắn không thành công, device_id: %s, message_id: %s, error: %v",
			event.ClientState.DeviceID, event.MessageID, err)
	}
}