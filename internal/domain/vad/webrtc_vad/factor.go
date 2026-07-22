package webrtc_vad

import (
	"fmt"
	"time"
	"milestones-esp32-server-golang/internal/util"
)

// WebRTCVADConfig cấu hình WebRTC VAD
type WebRTCVADConfig struct {
	SampleRate int
	Mode       int
}

// WebRTCVADFactory factory (nhà máy tạo instance) cho WebRTC VAD, hiện thực interface ResourceFactory
type WebRTCVADFactory struct {
	config WebRTCVADConfig
}

// NewWebRTCVADFactory tạo factory WebRTC VAD
func NewWebRTCVADFactory(config WebRTCVADConfig) *WebRTCVADFactory {
	if config.SampleRate == 0 {
		config.SampleRate = DefaultSampleRate
	}
	if config.Mode < 0 || config.Mode > 3 {
		config.Mode = DefaultMode
	}

	return &WebRTCVADFactory{
		config: config,
	}
}

// Create tạo mới một thực thể resource WebRTC VAD
func (f *WebRTCVADFactory) Create() (util.Resource, error) {
	vad := &WebRTCVAD{
		sampleRate: f.config.SampleRate,
		mode:       f.config.Mode,
		lastUsed:   time.Now(),
	}

	// Khởi tạo thực thể
	if err := vad.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize WebRTC VAD: %w", err)
	}

	return vad, nil
}

// Validate kiểm tra resource có hợp lệ hay không
func (f *WebRTCVADFactory) Validate(resource util.Resource) bool {
	vad, ok := resource.(*WebRTCVAD)
	if !ok {
		return false
	}
	return vad.IsValid()
}

// Reset đặt lại trạng thái resource
func (f *WebRTCVADFactory) Reset(resource util.Resource) error {
	vad, ok := resource.(*WebRTCVAD)
	if !ok {
		return fmt.Errorf("invalid resource type")
	}
	return vad.Reset()
}