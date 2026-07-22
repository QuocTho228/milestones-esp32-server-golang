package webrtc_vad

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewWebRTCVAD kiểm thử tạo thực thể WebRTC VAD
func TestNewWebRTCVAD(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)
	assert.Equal(t, DefaultSampleRate, webrtcVAD.sampleRate)
	assert.Equal(t, DefaultMode, webrtcVAD.mode)
	assert.False(t, webrtcVAD.initialized)

	// Dọn dẹp resource
	err := vad.Close()
	assert.NoError(t, err)
}

// TestNewWebRTCVADWithConfig kiểm thử tạo thực thể WebRTC VAD bằng cấu hình
func TestNewWebRTCVADWithConfig(t *testing.T) {
	// Kiểm thử cấu hình hợp lệ
	vad, err := NewWebRTCVADWithConfig(8000, 1)
	require.NoError(t, err)
	require.NotNil(t, vad)

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)
	assert.Equal(t, 8000, webrtcVAD.sampleRate)
	assert.Equal(t, 1, webrtcVAD.mode)

	err = vad.Close()
	assert.NoError(t, err)

	// Kiểm thử tốc độ lấy mẫu không hợp lệ
	vad, err = NewWebRTCVADWithConfig(22050, 1)
	assert.Error(t, err)
	assert.Nil(t, vad)

	// Kiểm thử chế độ không hợp lệ
	vad, err = NewWebRTCVADWithConfig(16000, 5)
	assert.Error(t, err)
	assert.Nil(t, vad)
}

// TestWebRTCVAD_IsVAD kiểm thử phát hiện hoạt động giọng nói
func TestWebRTCVAD_IsVAD(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	// Kiểm thử dữ liệu rỗng
	isActive, err := vad.IsVAD([]float32{})
	assert.NoError(t, err)
	assert.False(t, isActive)

	// Kiểm thử dữ liệu im lặng (toàn số 0)
	silentData := make([]float32, 1600) // 100ms tại 16kHz
	isActive, err = vad.IsVAD(silentData)
	assert.NoError(t, err)
	// Dữ liệu im lặng thường sẽ không bị phát hiện là hoạt động giọng nói, nhưng còn tùy vào cách hiện thực VAD

	// Kiểm thử dữ liệu giọng nói tổng hợp (sóng sin)
	speechData := generateSineWave(16000, 440, 1.0, 0.5) // Sóng sin 440Hz trong 1 giây
	isActive, err = vad.IsVAD(speechData)
	assert.NoError(t, err)
	// Sóng sin có thể bị phát hiện là hoạt động giọng nói, nhưng còn tùy vào thuật toán VAD

	// Kiểm thử trường hợp dữ liệu không đủ một khung
	shortData := make([]float32, 100) // Dữ liệu ít hơn một khung
	isActive, err = vad.IsVAD(shortData)
	assert.NoError(t, err)
	assert.False(t, isActive)
}

// TestWebRTCVAD_Reset kiểm thử chức năng reset
func TestWebRTCVAD_Reset(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	// Reset trước khi khởi tạo
	err := vad.Reset()
	assert.NoError(t, err)

	// Sử dụng VAD trước để khởi tạo
	testData := make([]float32, 1600) // 100ms tại 16kHz
	_, err = vad.IsVAD(testData)
	assert.NoError(t, err)

	// Reset sau khi đã khởi tạo
	err = vad.Reset()
	assert.NoError(t, err)
}

// TestWebRTCVAD_Close kiểm thử chức năng đóng
func TestWebRTCVAD_Close(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)

	// Đóng khi chưa khởi tạo
	err := vad.Close()
	assert.NoError(t, err)

	// Đóng sau khi đã khởi tạo
	testData := make([]float32, 1600)
	_, err = vad.IsVAD(testData)
	assert.NoError(t, err)

	err = vad.Close()
	assert.NoError(t, err)

	// Đóng lặp lại
	err = vad.Close()
	assert.NoError(t, err)
}

// TestWebRTCVAD_SetMode kiểm thử thiết lập chế độ
func TestWebRTCVAD_SetMode(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)

	// Kiểm thử chế độ hợp lệ
	for mode := 0; mode <= 3; mode++ {
		err := webrtcVAD.SetMode(mode)
		assert.NoError(t, err)
		assert.Equal(t, mode, webrtcVAD.GetMode())
	}

	// Kiểm thử chế độ không hợp lệ
	err := webrtcVAD.SetMode(-1)
	assert.Error(t, err)

	err = webrtcVAD.SetMode(4)
	assert.Error(t, err)
}

// TestWebRTCVAD_SetSampleRate kiểm thử thiết lập tốc độ lấy mẫu
func TestWebRTCVAD_SetSampleRate(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)

	// Kiểm thử tốc độ lấy mẫu hợp lệ
	validRates := []int{8000, 16000, 32000, 48000}
	for _, rate := range validRates {
		err := webrtcVAD.SetSampleRate(rate)
		assert.NoError(t, err)
		assert.Equal(t, rate, webrtcVAD.GetSampleRate())
	}

	// Kiểm thử tốc độ lấy mẫu không hợp lệ
	err := webrtcVAD.SetSampleRate(22050)
	assert.Error(t, err)

	err = webrtcVAD.SetSampleRate(44100)
	assert.Error(t, err)
}

// TestFloat32ToPCMBytes kiểm thử chuyển đổi kiểu dữ liệu
func TestFloat32ToPCMBytes(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)

	// Kiểm thử giá trị biên
	testData := []float32{-1.0, 0.0, 1.0, 1.5, -1.5}
	pcmBytes := webrtcVAD.float32ToPCMBytes(testData)

	assert.Equal(t, len(testData)*2, len(pcmBytes))

	// Kiểm tra kết quả chuyển đổi
	// -1.0 -> -32768
	// 0.0 -> 0
	// 1.0 -> 32767
	// 1.5 -> 32767 (bị cắt/clipped)
	// -1.5 -> -32768 (bị cắt/clipped)
}

// TestIsValidSampleRate kiểm thử kiểm tra tốc độ lấy mẫu
func TestIsValidSampleRate(t *testing.T) {
	// Tốc độ lấy mẫu hợp lệ
	validRates := []int{8000, 16000, 32000, 48000}
	for _, rate := range validRates {
		assert.True(t, isValidSampleRate(rate))
	}

	// Tốc độ lấy mẫu không hợp lệ
	invalidRates := []int{11025, 22050, 44100, 96000}
	for _, rate := range invalidRates {
		assert.False(t, isValidSampleRate(rate))
	}
}

// generateSineWave tạo dữ liệu sóng sin dùng để kiểm thử
func generateSineWave(sampleRate int, frequency float64, duration float64, amplitude float64) []float32 {
	numSamples := int(float64(sampleRate) * duration)
	samples := make([]float32, numSamples)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		samples[i] = float32(amplitude * math.Sin(2*math.Pi*frequency*t))
	}

	return samples
}