package nomemo

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// NoMemoProvider Triển khai nhà cung cấp bộ nhớ (memory provider) rỗng
// Dùng khi người dùng không cần chức năng bộ nhớ, tất cả các phương thức đều là triển khai rỗng
type NoMemoProvider struct{}

// Get Lấy instance của NoMemoProvider
func Get() *NoMemoProvider {
	return &NoMemoProvider{}
}

// AddMessage Thêm một tin nhắn vào bộ nhớ (triển khai rỗng)
func (n *NoMemoProvider) AddMessage(ctx context.Context, agentID string, msg schema.Message) error {
	// Triển khai rỗng, không thực hiện thao tác nào
	return nil
}

// GetMessages Lấy tin nhắn lịch sử của người dùng (triển khai rỗng)
func (n *NoMemoProvider) GetMessages(ctx context.Context, agentId string, count int) ([]*schema.Message, error) {
	// Trả về danh sách tin nhắn rỗng
	return []*schema.Message{}, nil
}

// GetContext Lấy thông tin context của người dùng (triển khai rỗng)
func (n *NoMemoProvider) GetContext(ctx context.Context, agentId string, maxToken int) (string, error) {
	// Trả về chuỗi rỗng
	return "", nil
}

// Search Tìm kiếm bộ nhớ của người dùng (triển khai rỗng)
func (n *NoMemoProvider) Search(ctx context.Context, agentId string, query string, topK int, timeRangeDays int64) (string, error) {
	// Trả về chuỗi rỗng
	return "", nil
}

// Flush Làm mới (flush) bộ nhớ của người dùng (triển khai rỗng)
func (n *NoMemoProvider) Flush(ctx context.Context, agentId string) error {
	// Triển khai rỗng, không thực hiện thao tác nào
	return nil
}

// ResetMemory Đặt lại (reset) bộ nhớ của người dùng (triển khai rỗng)
func (n *NoMemoProvider) ResetMemory(ctx context.Context, agentId string) error {
	// Triển khai rỗng, không thực hiện thao tác nào
	return nil
}