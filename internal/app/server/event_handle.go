package server

import (
	"context"
	"hash/fnv"
	"sync"
	. "milestones-esp32-server-golang/internal/data/client"
	"milestones-esp32-server-golang/internal/domain/eventbus"
	log "milestones-esp32-server-golang/logger"
)

// EventWrapper 事件包装器，用于统一处理不同类型的事件
type EventWrapper struct {
	Topic string      // topic名称
	Data  interface{} // 事件数据
}

// TopicHandler 通用topic处理器接口
type TopicHandler interface {
	// Process 处理事件
	Process(ctx context.Context, data interface{}) error
	// GetRoutingKey 获取用于hash路由的key（通常是DeviceID或SessionID）
	GetRoutingKey(data interface{}) string
}

// UnifiedWorkerPool 统一的worker池，可以处理多个topic
type UnifiedWorkerPool struct {
	workers   []chan *EventWrapper
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	handlers  map[string]TopicHandler // topic -> handler 映射
	workerNum int
	mu        sync.RWMutex // 保护 handlers map
}

// NewUnifiedWorkerPool 创建统一的worker池
func NewUnifiedWorkerPool(workerNum int) *UnifiedWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &UnifiedWorkerPool{
		workers:   make([]chan *EventWrapper, workerNum),
		ctx:       ctx,
		cancel:    cancel,
		handlers:  make(map[string]TopicHandler),
		workerNum: workerNum,
	}

	// 初始化每个worker的channel并启动goroutine
	for i := 0; i < workerNum; i++ {
		pool.workers[i] = make(chan *EventWrapper, 100) // 缓冲100个消息
		pool.wg.Add(1)
		go pool.workerLoop(i)
	}

	log.Infof("Quá trình khởi tạo UnifiedWorkerPool đã hoàn tất, đang khởi chạy %d goroutine worker (có khả năng xử lý nhiều topic).", workerNum)
	return pool
}

// RegisterHandler 注册topic处理器
func (p *UnifiedWorkerPool) RegisterHandler(topic string, handler TopicHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[topic] = handler
	log.Infof("UnifiedWorkerPool: đăng ký bộ xử lý topic [%s]", topic)
}

// workerLoop 每个worker的处理循环（保证顺序处理）
func (p *UnifiedWorkerPool) workerLoop(index int) {
	defer p.wg.Done()
	defer log.Infof("UnifiedWorkerPool worker %d từ bỏ", index)

	ch := p.workers[index]
	for {
		select {
		case <-p.ctx.Done():
			// 清理channel中的剩余消息
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
				// channel已关闭
				return
			}
			if event != nil {
				p.processEvent(event)
			}
		}
	}
}

// processEvent 处理事件（根据topic分发到对应的handler）
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

// Route 路由事件到对应的worker（使用hash分布）
func (p *UnifiedWorkerPool) Route(topic string, data interface{}) bool {
	p.mu.RLock()
	handler, exists := p.handlers[topic]
	p.mu.RUnlock()

	if !exists {
		log.Warnf("UnifiedWorkerPool: topic [%s] Không có trình xử lý nào được đăng ký, không thể định tuyến", topic)
		return false
	}

	// 获取路由key
	key := handler.GetRoutingKey(data)
	if key == "" {
		log.Warnf("UnifiedWorkerPool: topic [%s] lộ trình key trống, không thể định tuyến tin nhắn", topic)
		return false
	}

	// 计算hash值，路由到对应的worker
	workerIndex := p.hashKey(key)

	// 创建事件包装器
	event := &EventWrapper{
		Topic: topic,
		Data:  data,
	}

	// 非阻塞发送到对应的worker channel
	select {
	case p.workers[workerIndex] <- event:
		return true
	default:
		log.Warnf("UnifiedWorkerPool: topic [%s] worker %d của channel đầy, thông báo bị loại bỏ, key: %s",
			topic, workerIndex, key)
		return false
	}
}

// hashKey 计算key的hash值，返回worker索引
func (p *UnifiedWorkerPool) hashKey(key string) int {
	if key == "" {
		return 0
	}
	h := fnv.New32a()
	h.Write([]byte(key))
	hash := h.Sum32()
	return int(hash) % p.workerNum
}

// Close 关闭worker池
func (p *UnifiedWorkerPool) Close() {
	p.cancel()
	p.wg.Wait()

	// 关闭所有worker channels
	for i := 0; i < p.workerNum; i++ {
		close(p.workers[i])
	}

	log.Info("Đã đóng UnifiedWorkerPool")
}

type EventHandle struct {
	// 统一的worker池，可以处理多个topic
	workerPool *UnifiedWorkerPool
	// App 引用，用于获取 ChatManager
	app *App
}

// SessionEndHandler SessionEnd事件处理器
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

	// 将消息加到长期记忆体中
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

// ExitChatHandler ExitChat事件处理器
type ExitChatHandler struct {
	eventHandle *EventHandle // 持有 EventHandle 引用，用于访问 App
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

	// 根据 deviceId 获取 ChatManager
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
	// 创建统一的worker池
	workerPool := NewUnifiedWorkerPool(MessageWorkerNum)

	// 注册SessionEnd处理器
	sessionEndHandler := &SessionEndHandler{}
	workerPool.RegisterHandler(eventbus.TopicSessionEnd, sessionEndHandler)

	handle := &EventHandle{
		workerPool: workerPool,
		app:        app,
	}

	// 注册ExitChat处理器
	exitChatHandler := &ExitChatHandler{
		eventHandle: handle,
	}
	workerPool.RegisterHandler(eventbus.TopicExitChat, exitChatHandler)

	log.Infof("Quá trình khởi tạo EventHandle đã hoàn tất (nhiều chủ đề được xử lý bằng một nhóm worker thống nhất; quá trình xử lý Redis đã được chuyển sang MessageWorker).")
	return handle, nil
}

func (s *EventHandle) Start() error {
	// 订阅SessionEnd事件
	go s.HandleSessionEnd()

	// 订阅ExitChat事件
	go s.HandleExitChat()

	// 在这里可以添加其他topic的订阅
	// go s.HandleDeviceOnline()

	return nil
}

// HandleSessionEnd 订阅并处理SessionEnd事件
func (s *EventHandle) HandleSessionEnd() error {
	eventbus.Get().Subscribe(eventbus.TopicSessionEnd, func(clientState *ClientState) {
		if clientState == nil {
			log.Warnf("HandleSessionEnd: clientState is nil, skipping")
			return
		}

		// 路由到统一的worker池
		s.workerPool.Route(eventbus.TopicSessionEnd, clientState)
	})
	return nil
}

// HandleExitChat 订阅并处理ExitChat事件
func (s *EventHandle) HandleExitChat() error {
	eventbus.Get().Subscribe(eventbus.TopicExitChat, func(event *eventbus.ExitChatEvent) {
		if event == nil {
			log.Warnf("HandleExitChat: event is nil, skipping")
			return
		}

		// 路由到统一的worker池
		s.workerPool.Route(eventbus.TopicExitChat, event)
	})
	return nil
}

// RegisterTopic 注册新topic的处理器（便捷方法）
func (s *EventHandle) RegisterTopic(topic string, handler TopicHandler) {
	s.workerPool.RegisterHandler(topic, handler)
}

// Close 关闭EventHandle，优雅关闭worker池
func (s *EventHandle) Close() {
	if s.workerPool != nil {
		s.workerPool.Close()
	}
	log.Info("Đã đóng EventHandle")
}
