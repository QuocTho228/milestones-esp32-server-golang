package llm_memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	i_redis "milestones-esp32-server-golang/internal/db/redis"
	log "milestones-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"

	"github.com/redis/go-redis/v9"
)

var (
	memoryInstance *Memory
	once           sync.Once
	configOnce     sync.Once
)

// Memory Đại diện cho bộ nhớ hội thoại (conversation memory)
type Memory struct {
	redisClient *redis.Client
	keyPrefix   string
	sync.RWMutex
}

// Get Lấy instance của bộ nhớ (memory)
func Get() *Memory {
	if memoryInstance == nil {
		once.Do(func() {
			redisInstance := i_redis.GetClient()

			memoryInstance = &Memory{
				redisClient: redisInstance,
				keyPrefix:   viper.GetString("redis.key_prefix"),
			}
		})
	}
	return memoryInstance
}

// GetWithConfig Sử dụng cấu hình để lấy instance của bộ nhớ (theo mô hình singleton)
func GetWithConfig(config map[string]interface{}) (*Memory, error) {
	var initErr error
	configOnce.Do(func() {
		// Đọc cấu hình liên quan đến redis từ config
		redisConfig, ok := config["redis"]
		if !ok {
			initErr = fmt.Errorf("cấu hình redis không tồn tại")
			return
		}

		redisConfigMap, ok := redisConfig.(map[string]interface{})
		if !ok {
			initErr = fmt.Errorf("định dạng cấu hình redis sai")
			return
		}

		// Đọc cấu hình key_prefix
		var keyPrefix string
		if keyPrefixInterface, exists := redisConfigMap["key_prefix"]; exists {
			if kp, ok := keyPrefixInterface.(string); ok {
				keyPrefix = kp
			} else {
				initErr = fmt.Errorf("redis.key_prefix phải là chuỗi (string)")
				return
			}
		} else {
			keyPrefix = "milestones:" // Giá trị mặc định
		}

		// Lấy Redis client (ở đây vẫn dùng cách lấy Redis client hiện có)
		// Vì việc khởi tạo Redis client khá phức tạp, nên tạm thời giữ nguyên cách làm hiện tại
		redisClient := i_redis.GetClient()
		if redisClient == nil {
			initErr = fmt.Errorf("không thể lấy được Redis client")
			return
		}

		// Tạo instance bộ nhớ LLM
		memoryInstance = &Memory{
			redisClient: redisClient,
			keyPrefix:   keyPrefix,
		}

		log.Log().Infof("Khởi tạo bộ nhớ LLM thành công, key_prefix: %s", keyPrefix)
	})

	if initErr != nil {
		return nil, initErr
	}
	return memoryInstance, nil
}

// NewWithConfig Sử dụng cấu hình để tạo mới instance bộ nhớ LLM
func NewWithConfig(config map[string]interface{}) (*Memory, error) {
	// Đọc cấu hình liên quan đến redis từ config
	redisConfig, ok := config["redis"]
	if !ok {
		return nil, fmt.Errorf("cấu hình redis không tồn tại")
	}

	redisConfigMap, ok := redisConfig.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("định dạng cấu hình redis sai")
	}

	// Đọc cấu hình key_prefix
	var keyPrefix string
	if keyPrefixInterface, exists := redisConfigMap["key_prefix"]; exists {
		if kp, ok := keyPrefixInterface.(string); ok {
			keyPrefix = kp
		} else {
			return nil, fmt.Errorf("redis.key_prefix phải là chuỗi (string)")
		}
	} else {
		keyPrefix = "milestones:" // Giá trị mặc định
	}

	// Lấy Redis client (ở đây vẫn dùng cách lấy Redis client hiện có)
	// Vì việc khởi tạo Redis client khá phức tạp, nên tạm thời giữ nguyên cách làm hiện tại
	redisClient := i_redis.GetClient()
	if redisClient == nil {
		return nil, fmt.Errorf("không thể lấy được Redis client")
	}

	// Tạo instance bộ nhớ LLM
	llmMemory := &Memory{
		redisClient: redisClient,
		keyPrefix:   keyPrefix,
	}

	log.Log().Infof("Khởi tạo bộ nhớ LLM thành công, key_prefix: %s", keyPrefix)
	return llmMemory, nil
}

// NewMemory Tạo mới instance bộ nhớ (chỉ dùng cho việc kiểm thử)
func NewMemory(redisClient *redis.Client) *Memory {
	return &Memory{
		redisClient: redisClient,
	}
}

// getMemoryKey Sinh Redis key tương ứng với thiết bị
func (m *Memory) getMemoryKey(deviceID string) string {
	return fmt.Sprintf("%s:llm:%s", m.keyPrefix, deviceID)
}

// getSystemPromptKey Sinh Redis key tương ứng với system prompt của thiết bị
func (m *Memory) getSystemPromptKey(deviceID string) string {
	return fmt.Sprintf("%s:llm:system:%s", m.keyPrefix, deviceID)
}

// AddMessage Thêm một tin nhắn hội thoại mới vào bộ nhớ
func (m *Memory) AddMessage(ctx context.Context, deviceID string, agentID string, msg schema.Message) error {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return nil
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", err)
	}

	key := m.getMemoryKey(deviceID)
	// Sử dụng timestamp theo nano giây làm điểm số (score)
	// ZREVRANGE sẽ trả về kết quả có điểm số từ lớn đến nhỏ
	score := float64(time.Now().UnixNano())

	log.Debugf("Thêm tin nhắn vào bộ nhớ: %s, %s", key, string(msgBytes))

	return m.redisClient.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: string(msgBytes),
	}).Err()
}

// GetMessages Lấy toàn bộ bộ nhớ hội thoại của thiết bị
func (m *Memory) GetMessages(ctx context.Context, deviceID string, agentID string, count int) ([]*schema.Message, error) {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return []*schema.Message{}, nil
	}

	key := m.getMemoryKey(deviceID)

	if count == 0 {
		count = 10
	}

	// Sử dụng ZREVRANGE để lấy N tin nhắn mới nhất
	// Điểm số (timestamp) lớn nằm trước, nên cần đảo ngược thứ tự để đảm bảo tin nhắn cũ nằm trước
	startIndex := int64(-(count))
	results, err := m.redisClient.ZRange(ctx, key, startIndex, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("get messages failed: %w", err)
	}

	// Cấp phát trước slice
	messages := make([]*schema.Message, 0)

	for i := 0; i < len(results); i++ {
		msg := schema.Message{}
		if err := json.Unmarshal([]byte(results[i]), &msg); err != nil {
			return nil, fmt.Errorf("unmarshal message failed: %w", err)
		}

		messages = append(messages, &msg)
	}

	return messages, nil
}

// GetMessagesForLLM Lấy định dạng tin nhắn phù hợp để dùng cho LLM
func (m *Memory) GetMessagesForLLM(ctx context.Context, deviceID string, count int) ([]*schema.Message, error) {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return []*schema.Message{}, nil
	}

	// Lấy tin nhắn lịch sử (đã theo thứ tự thời gian: cũ -> mới)
	memoryMessages, err := m.GetMessages(ctx, deviceID, "", count)
	if err != nil {
		return nil, err
	}

	return memoryMessages, nil
}

// SetSystemPrompt Thiết lập hoặc cập nhật system prompt cho thiết bị
func (m *Memory) SetSystemPrompt(ctx context.Context, deviceID string, prompt string) error {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return nil
	}

	key := m.getSystemPromptKey(deviceID)
	return m.redisClient.Set(ctx, key, prompt, 0).Err()
}

// GetSystemPrompt Lấy system prompt của thiết bị
func (m *Memory) GetSystemPrompt(ctx context.Context, deviceID string) (schema.Message, error) {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return schema.Message{Role: schema.System, Content: viper.GetString("system_prompt")}, nil
	}

	key := m.getSystemPromptKey(deviceID)

	result, err := m.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return schema.Message{}, nil // Trả về cấu trúc tin nhắn rỗng
	}
	if err != nil {
		return schema.Message{}, fmt.Errorf("get system prompt failed: %w", err)
	}

	return schema.Message{
		Role:    schema.System,
		Content: result,
	}, nil
}

// ResetMemory Đặt lại (reset) bộ nhớ hội thoại của thiết bị (bao gồm cả system prompt)
func (m *Memory) ResetMemory(ctx context.Context, deviceID string) error {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return nil
	}

	// Xóa lịch sử hội thoại
	historyKey := m.getMemoryKey(deviceID)
	if err := m.redisClient.Del(ctx, historyKey).Err(); err != nil {
		return fmt.Errorf("delete history failed: %w", err)
	}

	return nil
}

// GetLastNMessages Lấy N tin nhắn gần nhất
func (m *Memory) GetLastNMessages(ctx context.Context, deviceID string, n int64) ([]schema.Message, error) {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return []schema.Message{}, nil
	}

	key := m.getMemoryKey(deviceID)

	// Lấy N tin nhắn cuối cùng
	results, err := m.redisClient.ZRevRange(ctx, key, 0, n-1).Result()
	if err != nil {
		return nil, fmt.Errorf("get last messages failed: %w", err)
	}

	messages := make([]schema.Message, 0, len(results))
	for i := len(results) - 1; i >= 0; i-- { // Đảo ngược thứ tự để giữ đúng thứ tự thời gian
		var msg schema.Message
		if err := json.Unmarshal([]byte(results[i]), &msg); err != nil {
			return nil, fmt.Errorf("unmarshal message failed: %w", err)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// RemoveOldMessages Xóa các tin nhắn trước một thời điểm chỉ định
func (m *Memory) RemoveOldMessages(ctx context.Context, deviceID string, before time.Time) error {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return nil
	}

	key := m.getMemoryKey(deviceID)
	score := float64(before.UnixNano())

	return m.redisClient.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%f", score)).Err()
}

// Summary Lấy bản tóm tắt (summary) của hội thoại
func (m *Memory) GetSummary(ctx context.Context, deviceID string) (string, error) {
	return "", nil
}

// SetSummary Thiết lập bản tóm tắt (summary) của hội thoại
func (m *Memory) SetSummary(ctx context.Context, deviceID string, summary string) error {
	return nil
}

// Thực hiện tóm tắt (summarization)
func (m *Memory) Summary(ctx context.Context, deviceID string, msgList []schema.Message) (string, error) {
	return "", nil
}

func (m *Memory) GetContext(ctx context.Context, deviceID string, agentID string, maxToken int) (string, error) {
	return "", nil
}

func (m *Memory) Search(ctx context.Context, deviceID string, query string, topK int, timeRangeDays int64) (string, error) {
	return "", nil
}