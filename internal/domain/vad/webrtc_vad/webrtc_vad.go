package webrtc_vad

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"milestones-esp32-server-golang/internal/domain/vad/inter"

	"github.com/hackers365/go-webrtcvad"
)

const (
	// DefaultSampleRate tốc độ lấy mẫu mà WebRTC VAD hỗ trợ (8000, 16000, 32000, 48000)
	DefaultSampleRate = 16000
	// DefaultMode mức độ nhạy của VAD (0: kém nhạy nhất, 3: nhạy nhất)
	DefaultMode = 2
	// FrameDuration thời lượng mỗi khung (ms), WebRTC VAD hỗ trợ 10ms, 20ms, 30ms
	FrameDuration = 20
)

// WebRTCVAD hiện thực WebRTC VAD, hiện đã hiện thực interface Resource
type WebRTCVAD struct {
	webrtcVad      *webrtcvad.VAD
	sampleRate     int          // Tốc độ lấy mẫu
	mode           int          // Chế độ VAD
	frameSize      int          // Số mẫu mỗi khung
	frameSizeBytes int          // Số byte mỗi khung
	initialized    bool         // Đã khởi tạo hay chưa
	lastUsed       time.Time    // Thời điểm sử dụng gần nhất
	mu             sync.RWMutex // Khóa đọc/ghi
}

// AcquireVAD tạo và trả về thực thể WebRTC VAD (được quản lý bởi resource pool toàn cục)
func AcquireVAD(config map[string]interface{}) (inter.VAD, error) {
	vadConfig := getVadConfigFromMap(config)

	vad := &WebRTCVAD{
		sampleRate: vadConfig.SampleRate,
		mode:       vadConfig.Mode,
		lastUsed:   time.Now(),
	}

	// Khởi tạo thực thể
	if err := vad.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize WebRTC VAD: %w", err)
	}

	return vad, nil
}

// ReleaseVAD giải phóng thực thể VAD
func ReleaseVAD(vad inter.VAD) error {
	if vad != nil {
		return vad.Close()
	}
	return nil
}

// NewWebRTCVAD tạo mới một thực thể WebRTC VAD
func NewWebRTCVAD() inter.VAD {
	return &WebRTCVAD{
		sampleRate: DefaultSampleRate,
		mode:       DefaultMode,
		lastUsed:   time.Now(),
	}
}

// NewWebRTCVADWithConfig tạo thực thể WebRTC VAD với cấu hình chỉ định
func NewWebRTCVADWithConfig(sampleRate, mode int) (inter.VAD, error) {
	if !isValidSampleRate(sampleRate) {
		return nil, fmt.Errorf("unsupported sample rate: %d, supported rates: 8000, 16000, 32000, 48000", sampleRate)
	}
	if mode < 0 || mode > 3 {
		return nil, fmt.Errorf("invalid VAD mode: %d, must be 0-3", mode)
	}

	vad := &WebRTCVAD{
		sampleRate: sampleRate,
		mode:       mode,
		lastUsed:   time.Now(),
	}

	err := vad.init()
	if err != nil {
		return nil, err
	}

	return vad, nil
}

// init khởi tạo WebRTC VAD
func (w *WebRTCVAD) init() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.initialized {
		return nil
	}

	// Tính kích thước khung
	w.frameSize = w.sampleRate / 1000 * FrameDuration
	w.frameSizeBytes = w.frameSize * 2 // PCM 16-bit

	// Tạo thực thể VAD
	var err error
	w.webrtcVad, err = webrtcvad.New()
	if w.webrtcVad == nil {
		return fmt.Errorf("failed to create WebRTC VAD instance")
	}

	err = w.webrtcVad.SetMode(w.mode)
	if err != nil {
		webrtcvad.Free(w.webrtcVad)
		return fmt.Errorf("failed to set WebRTC VAD mode: %+v", err)
	}

	w.initialized = true
	w.lastUsed = time.Now()
	return nil
}

func (w *WebRTCVAD) IsVAD(pcmData []float32) (bool, error) {
	return w.isVad(pcmData, w.sampleRate, w.frameSize)
}

// isVad phát hiện hoạt động giọng nói trong dữ liệu audio
func (w *WebRTCVAD) isVad(pcmData []float32, sampleRate int, frameSize int) (bool, error) {
	if len(pcmData) == 0 {
		return false, nil
	}

	//log.Debugf("isVad, pcmData len: %d, frameSize: %d", len(pcmData), frameSize)

	// Cập nhật thời điểm sử dụng gần nhất
	w.lastUsed = time.Now()

	//pcmBytes := pcmData
	// Chuyển dữ liệu float32 sang dữ liệu PCM int16
	pcmBytes := w.float32ToPCMBytes(pcmData)

	// Nếu độ dài dữ liệu không đủ một khung, trả về false
	if len(pcmBytes) < frameSize {
		return false, nil
	}

	// Xử lý dữ liệu nhiều khung, lấy kết quả của khung cuối cùng
	var isActive bool
	var err error

	activityCount := 0
	for i := 0; i+frameSize <= len(pcmBytes); i += frameSize {
		frameData := pcmBytes[i : i+frameSize]

		isActive, err = w.webrtcVad.Process(sampleRate, frameData)
		if err != nil {
			return false, fmt.Errorf("WebRTC VAD process error: %w", err)
		}
		if isActive {
			activityCount++
		}
	}

	frameCount := len(pcmBytes) / frameSize
	isActive = activityCount >= frameCount/2

	//log.Debugf("isVad, isActive: %v, activityCount: %d", isActive, activityCount)
	return isActive, nil
}

func (w *WebRTCVAD) IsVADExt(pcmData []float32, sampleRate int, frameSize int) (bool, error) {
	return w.isVad(pcmData, sampleRate, frameSize)
}

// Reset đặt lại trạng thái bộ phát hiện
func (w *WebRTCVAD) Reset() error {
	return nil
}

// Close đóng và giải phóng resource (hiện thực interface Resource)
func (w *WebRTCVAD) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.initialized && w.webrtcVad != nil {
		webrtcvad.Free(w.webrtcVad)
		w.initialized = false
	}
	return nil
}

// IsValid kiểm tra resource có hợp lệ hay không (hiện thực interface Resource)
func (w *WebRTCVAD) IsValid() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.initialized && w.webrtcVad != nil
}

// float32ToPCMBytes chuyển mảng float32 sang mảng byte PCM 16-bit
func (w *WebRTCVAD) float32ToPCMBytes(samples []float32) []byte {
	pcmBytes := make([]byte, len(samples)*2)

	for i, sample := range samples {
		// Chuyển float32 (-1.0 đến 1.0) sang int16 (-32768 đến 32767)
		var intSample int16
		if sample > 1.0 {
			intSample = 32767
		} else if sample < -1.0 {
			intSample = -32768
		} else {
			intSample = int16(sample * 32767)
		}

		// Ghi vào mảng byte theo thứ tự little-endian
		binary.LittleEndian.PutUint16(pcmBytes[i*2:], uint16(intSample))
	}

	return pcmBytes
}

// isValidSampleRate kiểm tra tốc độ lấy mẫu có được WebRTC VAD hỗ trợ hay không
func isValidSampleRate(sampleRate int) bool {
	validRates := []int{8000, 16000, 32000, 48000}
	for _, rate := range validRates {
		if rate == sampleRate {
			return true
		}
	}
	return false
}

// SetMode thiết lập chế độ nhạy của VAD
func (w *WebRTCVAD) SetMode(mode int) error {
	if mode < 0 || mode > 3 {
		return fmt.Errorf("invalid VAD mode: %d, must be 0-3", mode)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.mode = mode

	if w.initialized {
		return w.webrtcVad.SetMode(mode)
	}

	return nil
}

// SetSampleRate thiết lập tốc độ lấy mẫu
func (w *WebRTCVAD) SetSampleRate(sampleRate int) error {
	if !isValidSampleRate(sampleRate) {
		return fmt.Errorf("unsupported sample rate: %d, supported rates: 8000, 16000, 32000, 48000", sampleRate)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Nếu đã khởi tạo thì cần khởi tạo lại
	if w.initialized {
		w.Close()
	}

	w.sampleRate = sampleRate
	return nil
}

// GetSampleRate lấy tốc độ lấy mẫu hiện tại
func (w *WebRTCVAD) GetSampleRate() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.sampleRate
}

// GetMode lấy chế độ VAD hiện tại
func (w *WebRTCVAD) GetMode() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.mode
}

// GetLastUsed lấy thời điểm sử dụng gần nhất
func (w *WebRTCVAD) GetLastUsed() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastUsed
}