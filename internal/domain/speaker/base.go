package speaker

import (
	"context"
)

// SpeakerProvider giao diện (interface) cho nhà cung cấp nhận diện vân giọng nói
type SpeakerProvider interface {
	// StartStreaming khởi động luồng nhận diện streaming
	StartStreaming(ctx context.Context, sampleRate int, agentId string) error

	// SendAudioChunk gửi khối dữ liệu âm thanh
	SendAudioChunk(ctx context.Context, audioData []float32) error

	// FinishAndIdentify hoàn tất đầu vào và lấy kết quả nhận diện
	FinishAndIdentify(ctx context.Context) (*IdentifyResult, error)

	// IsActive kiểm tra xem có đang ở trạng thái kích hoạt hay không
	IsActive() bool

	// Close đóng kết nối
	Close() error
}

// GetSpeakerProvider lấy nhà cung cấp nhận diện vân giọng nói
func GetSpeakerProvider(config map[string]interface{}) (SpeakerProvider, error) {
	return NewAsrServerProvider(config)
}