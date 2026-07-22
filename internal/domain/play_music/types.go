package play_music

import (
	"context"
)

// MusicPlayerInterface Giao diện trình phát nhạc
type MusicPlayerInterface interface {
	// PlayMusicStream Phát nhạc từ URL, trả về kênh (channel) luồng âm thanh
	PlayMusicStream(ctx context.Context, url string) (chan []byte, error)

	// GetPlayerInfo Lấy thông tin trình phát
	GetPlayerInfo() map[string]interface{}

	// Stop Dừng trình phát
	Stop() error
}

// MusicPlayerConfig Cấu hình trình phát nhạc
type MusicPlayerConfig struct {
	FrameDuration int    `json:"frame_duration"` // Thời lượng mỗi khung (frame) (ms), mặc định 20ms
	AudioFormat   string `json:"audio_format"`   // Định dạng âm thanh, mặc định "mp3"
}

// DefaultMusicPlayerConfig Cấu hình mặc định của trình phát nhạc
func DefaultMusicPlayerConfig() *MusicPlayerConfig {
	return &MusicPlayerConfig{
		FrameDuration: 20,    // 20ms
		AudioFormat:   "mp3", // Định dạng MP3
	}
}

// ToMap Chuyển đổi cấu hình thành map
func (c *MusicPlayerConfig) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"frame_duration": c.FrameDuration,
		"audio_format":   c.AudioFormat,
	}
}

// AudioStreamInfo Thông tin luồng âm thanh
type AudioStreamInfo struct {
	URL           string `json:"url"`
	Format        string `json:"format"`         // Định dạng âm thanh, ví dụ "mp3", "wav"
	SampleRate    int    `json:"sample_rate"`    // Tần số lấy mẫu (sample rate)
	Channels      int    `json:"channels"`       // Số kênh âm thanh (mono/stereo)
	Duration      int64  `json:"duration"`       // Thời lượng (mili giây)
	ContentLength int64  `json:"content_length"` // Độ dài nội dung (byte)
}

// PlaybackStatus Trạng thái phát
type PlaybackStatus int

const (
	StatusIdle PlaybackStatus = iota
	StatusPlaying
	StatusPaused
	StatusStopped
	StatusError
)

// String Trả về biểu diễn dạng chuỗi của trạng thái
func (s PlaybackStatus) String() string {
	switch s {
	case StatusIdle:
		return "idle"
	case StatusPlaying:
		return "playing"
	case StatusPaused:
		return "paused"
	case StatusStopped:
		return "stopped"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// PlaybackEvent Sự kiện phát
type PlaybackEvent struct {
	Type      string      `json:"type"`      // Loại sự kiện: "started", "progress", "finished", "error"
	Timestamp int64       `json:"timestamp"` // Dấu thời gian (timestamp)
	Message   string      `json:"message"`   // Thông điệp sự kiện
	Data      interface{} `json:"data"`      // Dữ liệu bổ sung
}

// StreamingStats Thông tin thống kê phát trực tuyến (streaming)
type StreamingStats struct {
	BytesDownloaded int64          `json:"bytes_downloaded"` // Số byte đã tải xuống
	BytesDecoded    int64          `json:"bytes_decoded"`    // Số byte đã giải mã
	FramesGenerated int64          `json:"frames_generated"` // Số khung (frame) đã tạo
	StartTime       int64          `json:"start_time"`       // Thời điểm bắt đầu
	FirstFrameTime  int64          `json:"first_frame_time"` // Thời điểm khung đầu tiên
	Status          PlaybackStatus `json:"status"`           // Trạng thái hiện tại
	ErrorCount      int            `json:"error_count"`      // Số lần lỗi
}