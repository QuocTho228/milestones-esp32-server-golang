package memory

import (
	"context"
	"fmt"

	"milestones-esp32-server-golang/internal/domain/memory/mem0"
	"milestones-esp32-server-golang/internal/domain/memory/memobase"
	"milestones-esp32-server-golang/internal/domain/memory/memos"
	"milestones-esp32-server-golang/internal/domain/memory/nomemo"

	"github.com/cloudwego/eino/schema"
)

// MemoryProvider Interface của nhà cung cấp bộ nhớ (memory provider)
// Định nghĩa các phương thức cốt lõi mà mọi memory provider đều phải triển khai
type MemoryProvider interface {
	// AddMessage Thêm một tin nhắn vào bộ nhớ
	AddMessage(ctx context.Context, agentID string, msg schema.Message) error

	// GetMessages Lấy tin nhắn lịch sử của người dùng
	GetMessages(ctx context.Context, agentId string, count int) ([]*schema.Message, error)

	// GetContext Lấy thông tin context của người dùng, dùng để tăng cường (enhance) LLM prompt
	GetContext(ctx context.Context, agentId string, maxToken int) (string, error)

	// Search Tìm kiếm bộ nhớ của người dùng
	Search(ctx context.Context, agentId string, query string, topK int, timeRangeDays int64) (string, error)

	// Flush Làm mới (flush) bộ nhớ của người dùng
	Flush(ctx context.Context, agentId string) error

	// ResetMemory Đặt lại (reset) bộ nhớ của người dùng
	ResetMemory(ctx context.Context, agentId string) error
}

// MemoryType Loại bộ nhớ (memory)
type MemoryType string

const (
	MemoryTypeNone     MemoryType = "nomemo"
	MemoryTypeMemobase MemoryType = "memobase" // Bộ nhớ dài hạn (long-term memory) Memobase
	MemoryTypeMem0     MemoryType = "mem0"     // Dịch vụ bộ nhớ Mem0
	MemoryTypeMemOS    MemoryType = "memos"    // MemOS (tương thích API của Mem0)
)

// GetProvider Lấy nhà cung cấp bộ nhớ theo loại được chỉ định
func GetProvider(memoryType MemoryType, config map[string]interface{}) (MemoryProvider, error) {
	return GetProviderByType(memoryType, config)
}

// GetProviderByType Lấy nhà cung cấp bộ nhớ dựa theo loại
func GetProviderByType(memoryType MemoryType, config map[string]interface{}) (MemoryProvider, error) {
	if memoryType == "" {
		memoryType = MemoryTypeNone
	}
	switch memoryType {
	case MemoryTypeNone:
		return nomemo.Get(), nil
	case MemoryTypeMemobase:
		return memobase.GetWithConfig(config)
	case MemoryTypeMem0:
		return mem0.GetMem0ClientWithConfig(config)
	case MemoryTypeMemOS:
		return memos.GetWithConfig(config)
	default:
		return nil, fmt.Errorf("unsupported memory type: %v", memoryType)
	}
}