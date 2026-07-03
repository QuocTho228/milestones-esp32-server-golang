package memory

import (
	"context"
	"fmt"
	"sync"

	"milestones-esp32-server-golang/internal/domain/config/types"
	log "milestones-esp32-server-golang/logger"
)

// MemoryUserConfigProvider - Trình cung cấp cấu hình người dùng trong bộ nhớ
// Triển khai giao diện UserConfigProvider, lưu trữ cấu hình trong bộ nhớ
// Lưu ý: dữ liệu sẽ mất sau khi khởi động lại, phù hợp cho kiểm thử hoặc lưu trữ tạm thời
type MemoryUserConfigProvider struct {
	mu         sync.RWMutex
	configs    map[string]types.UConfig
	maxEntries int
}

// MemoryConfig - Cấu trúc cấu hình bộ nhớ
type MemoryConfig struct {
	MaxEntries int `json:"max_entries"` // Số lượng mục lưu trữ tối đa
}

// NewMemoryUserConfigProvider - Tạo trình cung cấp cấu hình người dùng trong bộ nhớ
// config: map tham số cấu hình, bao gồm max_entries, v.v.
func NewMemoryUserConfigProvider(config map[string]interface{}) (*MemoryUserConfigProvider, error) {
	// Phân tích tham số cấu hình
	memoryConfig := &MemoryConfig{
		MaxEntries: 1000, // Mặc định tối đa 1000 cấu hình
	}

	if maxEntries, ok := config["max_entries"].(int); ok && maxEntries > 0 {
		memoryConfig.MaxEntries = maxEntries
	} else if maxEntriesFloat, ok := config["max_entries"].(float64); ok && maxEntriesFloat > 0 {
		memoryConfig.MaxEntries = int(maxEntriesFloat)
	}

	provider := &MemoryUserConfigProvider{
		configs:    make(map[string]types.UConfig),
		maxEntries: memoryConfig.MaxEntries,
	}

	log.Log().Infof("Khởi tạo trình cung cấp cấu hình người dùng trong bộ nhớ thành công, số mục tối đa: %d", memoryConfig.MaxEntries)
	return provider, nil
}

// GetUserConfig - Lấy cấu hình người dùng
func (m *MemoryUserConfigProvider) GetUserConfig(ctx context.Context, userID string) (types.UConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.configs[userID]
	if !exists {
		log.Log().Debugf("Cấu hình của người dùng %s không tồn tại, trả về cấu hình rỗng", userID)
		return types.UConfig{}, nil
	}

	return config, nil
}

// SetUserConfig - Thiết lập cấu hình người dùng
func (m *MemoryUserConfigProvider) SetUserConfig(ctx context.Context, userID string, config types.UConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Kiểm tra xem đã vượt quá số mục lưu trữ tối đa chưa
	if len(m.configs) >= m.maxEntries && !m.configExists(userID) {
		return fmt.Errorf("Đã đạt số mục lưu trữ tối đa %d, không thể thêm cấu hình mới", m.maxEntries)
	}

	m.configs[userID] = config
	log.Log().Infof("Thiết lập cấu hình cho người dùng %s thành công (lưu trữ trong bộ nhớ)", userID)
	return nil
}

// DeleteUserConfig - Xóa cấu hình người dùng
func (m *MemoryUserConfigProvider) DeleteUserConfig(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.configs[userID]; !exists {
		log.Log().Warnf("Cấu hình của người dùng %s không tồn tại, không cần xóa", userID)
		return nil
	}

	delete(m.configs, userID)
	log.Log().Infof("Xóa cấu hình của người dùng %s thành công (lưu trữ trong bộ nhớ)", userID)
	return nil
}

// Close - Đóng trình cung cấp (trình cung cấp bộ nhớ không cần dọn dẹp đặc biệt)
func (m *MemoryUserConfigProvider) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Xóa toàn bộ cấu hình
	m.configs = make(map[string]types.UConfig)
	log.Log().Info("Trình cung cấp cấu hình người dùng trong bộ nhớ đã đóng, toàn bộ cấu hình đã được xóa")
	return nil
}

// configExists - Kiểm tra cấu hình có tồn tại hay không (phương thức nội bộ, cần giữ khóa khi gọi)
func (m *MemoryUserConfigProvider) configExists(userID string) bool {
	_, exists := m.configs[userID]
	return exists
}

// GetStats - Lấy thông tin thống kê lưu trữ (phương thức tiện ích bổ sung)
func (m *MemoryUserConfigProvider) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_configs": len(m.configs),
		"max_entries":   m.maxEntries,
		"usage_percent": float64(len(m.configs)) / float64(m.maxEntries) * 100,
	}
}

// ListUserIDs - Liệt kê tất cả user ID (phương thức tiện ích bổ sung)
func (m *MemoryUserConfigProvider) ListUserIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userIDs := make([]string, 0, len(m.configs))
	for userID := range m.configs {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

// GetSystemConfig - Lấy cấu hình hệ thống
func (m *MemoryUserConfigProvider) GetSystemConfig(ctx context.Context) (string, error) {
	// Trình cung cấp cấu hình bộ nhớ không cung cấp cấu hình hệ thống
	return "", nil
}

// Init - Khởi tạo trình cung cấp cấu hình Memory
func Init(ctx context.Context) error {
	log.Log().Info("Memory config provider initialized successfully")
	return nil
}

// Close - Đóng trình cung cấp cấu hình Memory, dọn dẹp tài nguyên
func Close() error {
	log.Log().Info("Memory config provider closed")
	return nil
}

// IsConnected - Kiểm tra trình cung cấp cấu hình Memory đã kết nối hay chưa
func IsConnected() bool {
	// Trình cung cấp cấu hình bộ nhớ luôn ở trạng thái "đã kết nối"
	return true
}