package eventbus

import (
	"time"

	. "milestones-esp32-server-golang/internal/data/client"
)

// ExitChatEvent Sự kiện thoát chat
type ExitChatEvent struct {
	// Trạng thái client
	ClientState *ClientState

	// Lý do thoát
	Reason string // "Người dùng chủ động thoát", "Thoát do gọi công cụ (tool call)", "Thoát do quá thời gian chờ (timeout)", v.v.

	// Cách thức kích hoạt việc thoát
	TriggerType string // "exit_words" (phát hiện từ khóa thoát), "tool_call" (gọi công cụ), "timeout" (quá thời gian chờ), v.v.

	// Văn bản gốc do người dùng nhập (nếu có)
	UserText string

	// Dấu thời gian (timestamp)
	Timestamp time.Time
}