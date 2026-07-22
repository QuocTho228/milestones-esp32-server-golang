package chat

import (
	"context"

	"milestones-esp32-server-golang/internal/domain/speaker"
)

// SpeakerManager Trình quản lý nhận diện vân giọng nói (voiceprint) (bọc SpeakerProvider)
type SpeakerManager struct {
	provider speaker.SpeakerProvider
}

type peekableSpeakerProvider interface {
	PeekAndIdentify(ctx context.Context, requestID string) (*speaker.IdentifyResult, bool, error)
}

// NewSpeakerManager Tạo trình quản lý vân giọng nói
func NewSpeakerManager(provider speaker.SpeakerProvider) *SpeakerManager {
	return &SpeakerManager{
		provider: provider,
	}
}

// StartStreaming Khởi động nhận diện dạng streaming
func (sm *SpeakerManager) StartStreaming(ctx context.Context, sampleRate int, agentId string) error {
	return sm.provider.StartStreaming(ctx, sampleRate, agentId)
}

// SendAudioChunk Gửi một khối (chunk) audio
func (sm *SpeakerManager) SendAudioChunk(ctx context.Context, pcmData []float32) error {
	return sm.provider.SendAudioChunk(ctx, pcmData)
}

// FinishAndIdentify Hoàn tất nhận diện và lấy kết quả
func (sm *SpeakerManager) FinishAndIdentify(ctx context.Context) (*speaker.IdentifyResult, error) {
	return sm.provider.FinishAndIdentify(ctx)
}

// Close Đóng trình quản lý vân giọng nói
func (sm *SpeakerManager) Close() error {
	return sm.provider.Close()
}

// IsActive Kiểm tra xem có đang ở trạng thái hoạt động không
func (sm *SpeakerManager) IsActive() bool {
	return sm.provider.IsActive()
}

// PeekAndIdentify Lấy kết quả nhận diện vân giọng nói tạm thời (không kết thúc lượt hiện tại)
// Trả về: kết quả nhận diện, có bị server chống dội (debounce) hay không, lỗi
func (sm *SpeakerManager) PeekAndIdentify(ctx context.Context, requestID string) (*speaker.IdentifyResult, bool, error) {
	if sm == nil || sm.provider == nil {
		return nil, false, nil
	}
	peekProvider, ok := sm.provider.(peekableSpeakerProvider)
	if !ok {
		return nil, false, nil
	}
	return peekProvider.PeekAndIdentify(ctx, requestID)
}