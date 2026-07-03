package ten_vad

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	log "milestones-esp32-server-golang/logger"

	. "milestones-esp32-server-golang/internal/domain/vad/inter"
)

// Cấu hình VAD mặc định
var defaultVADConfig = map[string]interface{}{
	"hop_size":  512,
	"threshold": 0.3,
}

// TenVAD Triển khai mô hình TEN-VAD
type TenVAD struct {
	handle    unsafe.Pointer
	hopSize   int
	threshold float32
	mu        sync.Mutex
}

// NewTenVAD Tạo instance TenVAD
func NewTenVAD(config map[string]interface{}) (*TenVAD, error) {
	hopSize, ok := config["hop_size"].(int)
	if !ok {
		// Thử chuyển đổi từ float64
		if hopSizeFloat, ok := config["hop_size"].(float64); ok {
			hopSize = int(hopSizeFloat)
		} else {
			hopSize = 512 // Giá trị mặc định
		}
	}

	threshold, ok := config["threshold"].(float64)
	if !ok {
		// Thử chuyển đổi từ float32
		if thresholdFloat32, ok := config["threshold"].(float32); ok {
			threshold = float64(thresholdFloat32)
		} else {
			threshold = 0.3 // Giá trị mặc định
		}
	}

	// Tạo instance TEN-VAD
	tenVAD := GetInstance()
	handle, err := tenVAD.CreateInstance(hopSize, float32(threshold))
	if err != nil {
		return nil, fmt.Errorf("Tạo instance TEN-VAD thất bại: %v", err)
	}

	log.Debugf("Tạo instance TEN-VAD thành công, hopSize: %d, threshold: %f", hopSize, threshold)

	return &TenVAD{
		handle:    handle,
		hopSize:   hopSize,
		threshold: float32(threshold),
	}, nil
}

// IsVAD Triển khai phương thức IsVAD của interface VAD
func (t *TenVAD) IsVAD(pcmData []float32) (bool, error) {
	return t.IsVADExt(pcmData, 16000, t.hopSize)
}

// IsVADExt Triển khai phương thức IsVADExt của interface VAD
func (t *TenVAD) IsVADExt(pcmData []float32, sampleRate int, frameSize int) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.handle == nil {
		return false, errors.New("Instance TEN-VAD chưa được khởi tạo")
	}

	if len(pcmData) == 0 {
		return false, nil
	}

	// Chuyển đổi float32 sang int16
	// Phạm vi float32: -1.0 đến 1.0
	// Phạm vi int16: -32768 đến 32767
	int16Data := make([]int16, len(pcmData))
	for i, f := range pcmData {
		// Giới hạn phạm vi và chuyển đổi
		if f > 1.0 {
			f = 1.0
		} else if f < -1.0 {
			f = -1.0
		}
		int16Data[i] = int16(f * 32768.0)
	}

	// Xử lý theo khung hopSize
	tenVAD := GetInstance()
	hasVoice := false
	voiceFrameCount := 0

	for i := 0; i < len(int16Data); i += t.hopSize {
		end := i + t.hopSize
		if end > len(int16Data) {
			end = len(int16Data)
		}

		frame := int16Data[i:end]
		// Nếu độ dài khung nhỏ hơn hopSize, cần đệm hoặc bỏ qua
		if len(frame) < t.hopSize {
			// Đối với khung cuối cùng, nếu độ dài không đủ, có thể bỏ qua hoặc đệm
			// Ở đây chọn bỏ qua khung không đủ
			continue
		}

		_, flag, err := tenVAD.ProcessAudio(t.handle, frame)
		if err != nil {
			log.Errorf("TEN-VAD xử lý khung âm thanh thất bại: %v", err)
			continue
		}

		// flag == 1 nghĩa là phát hiện giọng nói
		if flag == 1 {
			hasVoice = true
			voiceFrameCount++
		}
	}

	// Nếu có ít nhất một khung phát hiện giọng nói, coi là có hoạt động giọng nói
	return hasVoice, nil
}

// Reset Đặt lại trạng thái bộ phát hiện VAD
func (t *TenVAD) Reset() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// TEN-VAD không cần reset, mỗi lần xử lý đều độc lập
	// Nhưng có thể tạo lại instance để reset trạng thái
	// Không làm gì ở đây vì TEN-VAD không có trạng thái
	return nil
}

// Close Đóng và giải phóng tài nguyên
func (t *TenVAD) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.handle != nil {
		tenVAD := GetInstance()
		err := tenVAD.DestroyInstance(t.handle)
		if err != nil {
			return fmt.Errorf("Hủy instance TEN-VAD thất bại: %v", err)
		}
		t.handle = nil
	}
	return nil
}

// IsValid Kiểm tra tài nguyên có hợp lệ không
func (t *TenVAD) IsValid() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.handle != nil
}

// AcquireVAD Tạo và trả về instance TEN-VAD (được quản lý bởi pool tài nguyên toàn cục)
func AcquireVAD(config map[string]interface{}) (VAD, error) {
	return NewTenVAD(config)
}

// ReleaseVAD Giải phóng instance VAD
func ReleaseVAD(vad VAD) error {
	if vad != nil {
		return vad.Close()
	}
	return nil
}