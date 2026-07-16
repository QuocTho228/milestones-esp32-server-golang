package server

import (
	"context"
	"hash/fnv"
	"sync"
	. "milestones-esp32-server-golang/internal/data/client"
	"milestones-esp32-server-golang/internal/domain/eventbus"
	log "milestones-esp32-server-golang/logger"
)

// EventWrapper bộ bọc (wrapper) sự kiện, dùng để xử lý thống nhất các loại sự kiện khác nhau
type EventWrapper struct {
	Topic string      // tên topic
	Data  interface{} // dữ liệu sự kiện
}

// TopicHandler interface xử lý topic dùng chung
type TopicHandler interface {
	// Process xử lý sự kiện
	Process(ctx context.Context, data interface{}) error
	// GetRoutingKey lấy key dùng để định tuyến (routing) theo hash (thường là DeviceID hoặc SessionID)
	GetRoutingKey(data interface{}) string
}

// UnifiedWorkerPool worker pool thống nhất, có thể xử lý nhiều topic
type UnifiedWorkerPool struct {
	workers   []chan *EventWrapper
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	handlers  map[string]TopicHandler // ánh xạ topic -> handler
	workerNum int
	mu        sync.RWMutex // bảo vệ map handlers
}

// NewUnifiedWorkerPool tạo worker pool thống nhất
func NewUnifiedWorkerPool(workerNum int) *UnifiedWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &UnifiedWorkerPool{
		workers:   make([]chan *EventWrapper, workerNum),
		ctx:       ctx,
		cancel:    cancel,
		handlers:  make(map[string]TopicHandler),
		workerNum: workerNum,
	}

	// Khởi tạo channel cho từng worker và khởi động goroutine
	for i := 0; i < workerNum; i++ {
		pool.workers[i] = make(chan *EventWrapper, 100) // đệm (buffer) 100 tin nhắn
		pool.wg.Add(1)
		go pool.workerLoop(i)
	}

	log.Infof("Quá trình khởi tạo UnifiedWorkerPool đã hoàn tất, đang khởi chạy %d goroutine worker (có khả năng xử lý nhiều topic).", workerNum)
	return pool
}

// RegisterHandler đăng ký trình xử lý cho topic
func (p *UnifiedWorkerPool) RegisterHandler(topic string, handler TopicHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[topic] = handler
	log.Infof("UnifiedWorkerPool: đăng ký bộ xử lý topic [%s]", topic)
}

// workerLoop vòng lặp xử lý của từng worker (đảm bảo xử lý tuần tự)
func (p *UnifiedWorkerPool) workerLoop(index int) {
	defer p.wg.Done()
	defer log.Infof("UnifiedWorkerPool worker %d từ bỏ", index)

	ch := p.workers[index]
	for {
		select {
		case <-p.ctx.Done():
			// Dọn dẹp các tin nhắn còn lại trong channel
			for {
				select {
				case event := <-ch:
					if event != nil {
						p.processEvent(event)
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
				p.processEvent(event)
			}
		}
	}
}

// processEvent xử lý sự kiện (phân phối đến handler tương ứng dựa theo topic)
func (p *UnifiedWorkerPool) processEvent(event *EventWrapper) {
	p.mu.RLock()
	handler, exists := p.handlers[event.Topic]
	p.mu.RUnlock()

	if !exists {
		log.Warnf("UnifiedWorkerPool: topic [%s] Không có người xử lý nào được đăng ký, bỏ qua", event.Topic)
		return
	}

	if err := handler.Process(context.Background(), event.Data); err != nil {
		log.Errorf("UnifiedWorkerPool: topic [%s] Xử lý không thành công: %v", event.Topic, err)
	}
}

// Route định tuyến sự kiện đến worker tương ứng (sử dụng phân phối theo hash)
func (p *UnifiedWorkerPool) Route(topic string, data interface{}) bool {
	p.mu.RLock()
	handler, exists := p.handlers[topic]
	p.mu.RUnlock()

	if !exists {
		log.Warnf("UnifiedWorkerPool: topic [%s] Không có trình xử lý nào được đăng ký, không thể định tuyến", topic)
		return false
	}

	// Lấy key định tuyến
	key := handler.GetRoutingKey(data)
	if key == "" {
		log.Warnf("UnifiedWorkerPool: topic [%s] lộ trình key trống, không thể định tuyến tin nhắn", topic)
		return false
	}

	// Tính giá trị hash, định tuyến đến worker tương ứng
	workerIndex := p.hashKey(key)

	// Tạo bộ bọc (wrapper) sự kiện
	event := &EventWrapper{
		Topic: topic,
		Data:  data,
	}

	// Gửi không chặn (non-blocking) đến channel của worker tương ứng
	select {
	case p.workers[workerIndex] <- event:
		return true
	default:
		log.Warnf("UnifiedWorkerPool: topic [%s] worker %d của channel đầy, thông báo bị loại bỏ, key: %s",
			topic, workerIndex, key)
		return false
	}
}

// hashKey tính giá trị hash của key, trả về chỉ số (index) của worker
func (p *UnifiedWorkerPool) hashKey(key string) int {
	if key == "" {
		return 0
	}
	h := fnv.New32a()
	h.Write([]byte(key))
	hash := h.Sum32()
	return int(hash) % p.workerNum
}

// Close đóng worker pool
func (p *UnifiedWorkerPool) Close() {
	p.cancel()
	p.wg.Wait()

	// Đóng tất cả channel của các worker
	for i := 0; i < p.workerNum; i++ {
		close(p.workers[i])
	}

	log.Info("Đã đóng UnifiedWorkerPool")
}

type EventHandle struct {
	// worker pool thống nhất, có thể xử lý nhiều topic
	workerPool *UnifiedWorkerPool
	// Tham chiếu đến App, dùng để lấy ChatManager
	app *App
}

// SessionEndHandler trình xử lý sự kiện SessionEnd
type SessionEndHandler struct{}

func (h *SessionEndHandler) Process(ctx context.Context, data interface{}) error {
	clientState, ok := data.(*ClientState)
	if !ok || clientState == nil {
		return nil
	}

	if clientState.MemoryProvider == nil {
		return nil
	}
	if clientState.GetMemoryMode() != MemoryModeLong {
		return nil
	}

	log.Debugf("HandleSessionEnd: deviceId: %s", clientState.DeviceID)

	// Thêm tin nhắn vào bộ nhớ dài hạn (long-term memory)
	err := clientState.MemoryProvider.Flush(
		clientState.Ctx,
		clientState.GetDeviceIDOrAgentID())
	if err != nil {
		log.Errorf("flush message to memory provider failed: %v", err)
		return err
	}
	return nil
}

func (h *SessionEndHandler) GetRoutingKey(data interface{}) string {
	clientState, ok := data.(*ClientState)
	if !ok || clientState == nil {
		return ""
	}
	return clientState.DeviceID
}

// ExitChatHandler trình xử lý sự kiện ExitChat
type ExitChatHandler struct {
	eventHandle *EventHandle // Giữ tham chiếu đến EventHandle, dùng để truy cập App
}

func (h *ExitChatHandler) Process(ctx context.Context, data interface{}) error {
	event, ok := data.(*eventbus.ExitChatEvent)
	if !ok || event == nil {
		return nil
	}

	clientState := event.ClientState
	if clientState == nil {
		return nil
	}

	log.Debugf("Xử lý sự kiện thoát khỏi cuộc trò chuyện: device_id: %s, reason: %s, trigger: %s, user_text: %s",
		clientState.DeviceID, event.Reason, event.TriggerType, event.UserText)

	// Lấy ChatManager theo deviceId
	if h.eventHandle == nil || h.eventHandle.app == nil {
		log.Warnf("Đối tượng EventHandle hoặc App chưa được khởi tạo, do đó không thể lấy được ChatManager.")
		return nil
	}

	chatManager, exists := h.eventHandle.app.GetChatManager(clientState.DeviceID)
	if !exists {
		log.Warnf("Không tìm thấy ChatManager cho thiết bị %s, có thể đã bị đóng.", clientState.DeviceID)
		return nil
	}

	return chatManager.ExitChat()
}

func (h *ExitChatHandler) GetRoutingKey(data interface{}) string {
	event, ok := data.(*eventbus.ExitChatEvent)
	if !ok || event == nil || event.ClientState == nil {
		return ""
	}
	return event.ClientState.DeviceID
}

func NewEventHandle(app *App) (*EventHandle, error) {
	// Tạo worker pool thống nhất
	workerPool := NewUnifiedWorkerPool(MessageWorkerNum)

	// Đăng ký trình xử lý SessionEnd
	sessionEndHandler := &SessionEndHandler{}
	workerPool.RegisterHandler(eventbus.TopicSessionEnd, sessionEndHandler)

	handle := &EventHandle{
		workerPool: workerPool,
		app:        app,
	}

	// Đăng ký trình xử lý ExitChat
	exitChatHandler := &ExitChatHandler{
		eventHandle: handle,
	}
	workerPool.RegisterHandler(eventbus.TopicExitChat, exitChatHandler)

	log.Infof("Quá trình khởi tạo EventHandle đã hoàn tất (nhiều chủ đề được xử lý bằng một nhóm worker thống nhất; quá trình xử lý Redis đã được chuyển sang MessageWorker).")
	return handle, nil
}

func (s *EventHandle) Start() error {
	// Đăng ký (subscribe) sự kiện SessionEnd
	go s.HandleSessionEnd()

	// Đăng ký sự kiện ExitChat
	go s.HandleExitChat()

	// Có thể thêm đăng ký cho các topic khác tại đây
	// go s.HandleDeviceOnline()

	return nil
}

// HandleSessionEnd đăng ký và xử lý sự kiện SessionEnd
func (s *EventHandle) HandleSessionEnd() error {
	eventbus.Get().Subscribe(eventbus.TopicSessionEnd, func(clientState *ClientState) {
		if clientState == nil {
			log.Warnf("HandleSessionEnd: clientState is nil, skipping")
			return
		}

		// Định tuyến đến worker pool thống nhất
		s.workerPool.Route(eventbus.TopicSessionEnd, clientState)
	})
	return nil
}

// HandleExitChat đăng ký và xử lý sự kiện ExitChat
func (s *EventHandle) HandleExitChat() error {
	eventbus.Get().Subscribe(eventbus.TopicExitChat, func(event *eventbus.ExitChatEvent) {
		if event == nil {
			log.Warnf("HandleExitChat: event is nil, skipping")
			return
		}

		// Định tuyến đến worker pool thống nhất
		s.workerPool.Route(eventbus.TopicExitChat, event)
	})
	return nil
}

// RegisterTopic đăng ký trình xử lý cho topic mới (phương thức tiện lợi)
func (s *EventHandle) RegisterTopic(topic string, handler TopicHandler) {
	s.workerPool.RegisterHandler(topic, handler)
}

// Close đóng EventHandle, tắt worker pool một cách an toàn (graceful shutdown)
func (s *EventHandle) Close() {
	if s.workerPool != nil {
		s.workerPool.Close()
	}
	log.Info("Đã đóng EventHandle")
}