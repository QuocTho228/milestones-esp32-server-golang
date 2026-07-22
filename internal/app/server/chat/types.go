package chat

import (
	"context"

	config_types "milestones-esp32-server-golang/internal/domain/config/types"
)

// ChatSessionOperator định nghĩa interface thao tác ChatSession mà local mcp tool cần
// Interface này dùng để tách rời (decouple) LLMManager và ChatSession, tránh phụ thuộc vòng (circular dependency)
type ChatSessionOperator interface {
	// LocalMcpCloseChat Đóng phiên chat
	LocalMcpCloseChat() error

	// LocalMcpClearHistory Xóa lịch sử hội thoại
	LocalMcpClearHistory() error

	// LocalMcpPlayMusic Phát nhạc
	LocalMcpPlayMusic(ctx context.Context, params *PlayMusicParams) error

	// LocalMcpSwitchDeviceRole Chuyển đổi vai trò thiết bị theo tên vai trò (hỗ trợ khớp mờ)
	LocalMcpSwitchDeviceRole(ctx context.Context, roleName string) (string, error)

	// LocalMcpRestoreDeviceDefaultRole Khôi phục vai trò mặc định của thiết bị
	LocalMcpRestoreDeviceDefaultRole(ctx context.Context) error

	// LocalMcpSearchKnowledge Truy vấn cơ sở tri thức liên kết với agent hiện tại
	LocalMcpSearchKnowledge(ctx context.Context, query string, topK int, knowledgeBaseIDs []uint) ([]config_types.KnowledgeSearchHit, error)

	// LocalMcpControlMusicPlayback Điều khiển phát media ở cấp độ phiên (session) hiện tại
	LocalMcpControlMusicPlayback(ctx context.Context, params *MusicPlaybackControlParams) (*MusicPlaybackControlResult, error)

	// Trong tương lai có thể bổ sung thêm các thao tác khác
	// GetDeviceID() string
	// IsActive() bool
}