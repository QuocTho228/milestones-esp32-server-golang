package speaker

import (
	"context"
	"fmt"
	"sync"

	log "milestones-esp32-server-golang/logger"
)

// AsrServerProvider nhà cung cấp nhận diện vân giọng nói (voiceprint) qua asr_server
type AsrServerProvider struct {
	streamingClient *StreamingClient
	threshold       float32 // ngưỡng nhận diện vân giọng nói
	isActive        bool
	mutex           sync.Mutex
}

// NewAsrServerProvider tạo mới nhà cung cấp nhận diện vân giọng nói asr_server
func NewAsrServerProvider(config map[string]interface{}) (*AsrServerProvider, error) {
	baseURL, ok := config["base_url"].(string)
	if !ok || baseURL == "" {
		return nil, fmt.Errorf("thiếu trường service.base_url trong cấu hình")
	}

	// Đọc cấu hình ngưỡng (threshold), giá trị mặc định là 0.4
	threshold := float32(0.4)
	if thresholdVal, ok := config["threshold"]; ok {
		switch v := thresholdVal.(type) {
		case float64:
			threshold = float32(v)
		case float32:
			threshold = v
		case int:
			threshold = float32(v)
		case int64:
			threshold = float32(v)
		}
		// Kiểm tra phạm vi hợp lệ của ngưỡng
		if threshold < 0 || threshold > 1 {
			log.Warnf("Ngưỡng %.4f vượt quá phạm vi hợp lệ [0.0, 1.0], sử dụng giá trị mặc định 0.4", threshold)
			threshold = 0.4
		}
	}

	streamingClient := NewStreamingClient(baseURL)
	return &AsrServerProvider{
		streamingClient: streamingClient,
		threshold:       threshold,
		isActive:        false,
	}, nil
}

// StartStreaming khởi động luồng nhận diện streaming
func (p *AsrServerProvider) StartStreaming(ctx context.Context, sampleRate int, agentId string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.isActive {
		return nil // đã được kích hoạt, trả về ngay
	}

	err := p.streamingClient.Connect(sampleRate, agentId, p.threshold)
	if err != nil {
		log.Warnf("Khởi động luồng nhận diện vân giọng nói thất bại: %v", err)
		return err
	}

	p.isActive = true
	log.Debugf("Luồng nhận diện vân giọng nói đã được khởi động, tần số lấy mẫu: %d Hz, agent_id: %s, ngưỡng: %.4f", sampleRate, agentId, p.threshold)
	return nil
}

// SendAudioChunk gửi khối dữ liệu âm thanh
func (p *AsrServerProvider) SendAudioChunk(ctx context.Context, pcmData []float32) error {
	p.mutex.Lock()
	isActive := p.isActive
	streamingClient := p.streamingClient
	p.mutex.Unlock()

	if !isActive {
		return nil // chưa kích hoạt, bỏ qua âm thầm
	}

	err := streamingClient.SendAudioChunk(pcmData)
	if err != nil {
		log.Warnf("Gửi khối âm thanh tới dịch vụ nhận diện vân giọng nói thất bại: %v", err)
		// Khi gửi thất bại, đánh dấu trạng thái không kích hoạt
		p.mutex.Lock()
		p.isActive = false
		p.mutex.Unlock()
		return err
	}

	return nil
}

// FinishAndIdentify hoàn tất nhận diện và lấy kết quả
func (p *AsrServerProvider) FinishAndIdentify(ctx context.Context) (*IdentifyResult, error) {
	p.mutex.Lock()
	if !p.isActive {
		p.mutex.Unlock()
		return nil, nil // chưa kích hoạt, trả về nil
	}
	p.isActive = false
	streamingClient := p.streamingClient
	p.mutex.Unlock()

	result, err := streamingClient.FinishAndIdentify(ctx)

	if err != nil {
		log.Warnf("Lấy kết quả nhận diện vân giọng nói thất bại: %v", err)
		return nil, err
	}

	return result, nil
}

// PeekAndIdentify lấy kết quả nhận diện tạm thời (không kết thúc lượt hiện tại)
// Trả về: kết quả nhận diện, có bị giới hạn tần suất (debounce) từ phía server hay không, lỗi (nếu có)
func (p *AsrServerProvider) PeekAndIdentify(ctx context.Context, requestID string) (*IdentifyResult, bool, error) {
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	default:
	}

	p.mutex.Lock()
	isActive := p.isActive
	streamingClient := p.streamingClient
	p.mutex.Unlock()

	if !isActive {
		return nil, false, nil
	}

	result, throttled, err := streamingClient.PeekAndIdentify(ctx, requestID)
	if err != nil {
		if !streamingClient.IsConnected() {
			p.mutex.Lock()
			p.isActive = false
			p.mutex.Unlock()
		}
		log.Warnf("Lấy kết quả nhận diện tạm thời vân giọng nói thất bại: %v", err)
		return nil, throttled, err
	}

	return result, throttled, nil
}

// Close đóng nhà cung cấp nhận diện vân giọng nói
func (p *AsrServerProvider) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.isActive = false
	if p.streamingClient != nil {
		return p.streamingClient.Close()
	}
	return nil
}

// IsActive kiểm tra xem có đang ở trạng thái kích hoạt hay không
func (p *AsrServerProvider) IsActive() bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.isActive
}