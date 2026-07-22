package vad

import (
	"errors"
	"milestones-esp32-server-golang/constants"
	"milestones-esp32-server-golang/internal/domain/vad/inter"
	"milestones-esp32-server-golang/internal/domain/vad/silero_vad"
	"milestones-esp32-server-golang/internal/domain/vad/ten_vad"
	// "milestones-esp32-server-golang/internal/domain/vad/webrtc_vad"
)

func AcquireVAD(provider string, config map[string]interface{}) (inter.VAD, error) {
	// Ưu tiên sử dụng provider trong config, nếu không thì dùng provider trong tham số
	if configProvider, ok := config["provider"].(string); ok && configProvider != "" {
		provider = configProvider
	}

	// Nếu provider trống, trả về thông báo lỗi rõ ràng
	if provider == "" {
		return nil, errors.New("vad provider is empty, please set provider in config (supported: silero_vad, ten_vad)")
	}

	switch provider {
	case constants.VadTypeSileroVad:
		return silero_vad.AcquireVAD(config)
	// case constants.VadTypeWebRTCVad:
	// 	return webrtc_vad.AcquireVAD(config)
	case constants.VadTypeTenVad:
		return ten_vad.AcquireVAD(config)
	default:
		return nil, errors.New("invalid vad provider: " + provider + " (supported: silero_vad, ten_vad)")
	}
}

func ReleaseVAD(vad inter.VAD) error {
	// Dựa theo loại vad, gọi phương thức ReleaseVAD tương ứng
	switch vad.(type) {
	// case *webrtc_vad.WebRTCVAD:
	// 	return webrtc_vad.ReleaseVAD(vad)
	case *silero_vad.SileroVAD:
		return silero_vad.ReleaseVAD(vad)
	case *ten_vad.TenVAD:
		return ten_vad.ReleaseVAD(vad)
	default:
		return errors.New("invalid vad type")
	}
}