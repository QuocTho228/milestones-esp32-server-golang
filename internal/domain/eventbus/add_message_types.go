package eventbus

import (
	"time"
	. "milestones-esp32-server-golang/internal/data/client"

	"github.com/cloudwego/eino/schema"
)

// AddMessageEvent Sự kiện thêm tin nhắn thống nhất
type AddMessageEvent struct {
	// Trạng thái client
	ClientState *ClientState

	// Nội dung tin nhắn (thống nhất sử dụng schema.Message)
	// schema.Message là định dạng tin nhắn LLM chuẩn, bao gồm:
	// - Role: Vai trò của tin nhắn (User/Assistant/System/Tool)
	// - Content: Nội dung văn bản của tin nhắn
	// - ToolCalls: Danh sách lời gọi công cụ (tool call) (tùy chọn)
	// - ToolCallID: ID của lời gọi công cụ (dùng cho vai trò Tool)
	Msg schema.Message

	// ID tin nhắn (dùng để liên kết việc lưu theo 2 giai đoạn)
	MessageID string

	// Dữ liệu âm thanh (tùy chọn, không thuộc định dạng chuẩn của schema.Message)
	// Giai đoạn 1: AudioData = nil (chỉ lưu văn bản)
	// Giai đoạn 2: AudioData != nil (cập nhật âm thanh)
	AudioData [][]byte // Mảng khung âm thanh TTS/ASR (định dạng Opus hoặc PCM)
	AudioSize int      // Kích thước âm thanh (byte)

	// Thông tin định dạng âm thanh (không thuộc định dạng chuẩn của schema.Message)
	SampleRate int // Tần số lấy mẫu (sample rate)
	Channels   int // Số kênh (channel)

	// Metadata (không thuộc định dạng chuẩn của schema.Message)
	Timestamp   time.Time
	TTSDuration int // Thời gian xử lý TTS (mili giây)

	// Đánh dấu giai đoạn
	IsUpdate bool // true = cập nhật âm thanh, false = thêm tin nhắn mới
}