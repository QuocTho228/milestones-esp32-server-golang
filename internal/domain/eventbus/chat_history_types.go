package eventbus

import (
	"context"
	"time"
)

// UserMessageEvent Sự kiện tin nhắn của người dùng
// Deprecated: Sử dụng AddMessageEvent thay thế, thống nhất dùng sự kiện TopicAddMessage
type UserMessageEvent struct {
	Ctx         context.Context
	SessionID   string
	DeviceID    string
	AgentID     string

	// Kết quả ASR
	Text      string
	AudioData []byte  // Dữ liệu âm thanh gốc (PCM float32 chuyển sang byte)
	AudioSize int     // Số mẫu âm thanh (sample count)

	// Thông tin định dạng âm thanh (dùng để chuyển đổi sang WAV)
	SampleRate int // Tần số lấy mẫu (sample rate)
	Channels   int // Số kênh (channel)

	// Metadata
	Timestamp time.Time
}

// AssistantMessageEvent Sự kiện phản hồi của trợ lý (bot)
// Deprecated: Sử dụng AddMessageEvent thay thế, thống nhất dùng sự kiện TopicAddMessage
type AssistantMessageEvent struct {
	Ctx         context.Context
	SessionID   string
	DeviceID    string
	AgentID     string

	// Kết quả LLM
	Text string

	// Kết quả TTS
	AudioData [][]byte // Dữ liệu âm thanh tổng hợp (định dạng Opus, mảng khung âm thanh)
	AudioSize int      // Kích thước âm thanh (byte)

	// Thông tin định dạng âm thanh (dùng để chuyển đổi sang WAV)
	SampleRate int // Tần số lấy mẫu (sample rate)
	Channels   int // Số kênh (channel)

	// Metadata
	TTSDuration int // Mili giây
	Timestamp   time.Time
}