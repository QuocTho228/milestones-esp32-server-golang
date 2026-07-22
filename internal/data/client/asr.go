package client

import (
	"bytes"
	"context"
	"strings"
	"sync"
	asr_types "milestones-esp32-server-golang/internal/domain/asr/types"
	log "milestones-esp32-server-golang/logger"
)

type Asr struct {
	lock sync.RWMutex
	// Context và các channel của ASR
	Ctx              context.Context
	Cancel           context.CancelFunc
	AsrEnd           chan bool
	AsrAudioChannel  chan []float32                 // channel đầu vào audio dạng streaming
	AsrResultChannel chan asr_types.StreamingResult // channel xuất ra các đoạn kết quả nhận dạng ASR theo dạng streaming
	AsrResult        bytes.Buffer                   // lưu văn bản cuối cùng nhận dạng được trong lần này
	Statue           int                            // 0: khởi tạo, 1: đang nhận dạng, 2: kết thúc nhận dạng
	AutoEnd          bool                           // auto_end nghĩa là dùng ASR tự động xác định điểm kết thúc, không dùng module VAD nữa

	// Loại và chế độ của ASR
	AsrType string // Loại ASR, ví dụ "funasr", "doubao"
	Mode    string // Chế độ ASR, ví dụ "online", "offline"

	// Tham chiếu đến ClientState, dùng để gọi callback thông báo
	ClientState *ClientState

	// Bộ đệm audio lịch sử chat: liên tục tích lũy dữ liệu audio đã gửi tới ASR
	HistoryAudioBuffer []float32

	// Cho biết vòng ASR hiện tại đã nhận được văn bản không rỗng đầu tiên hay chưa
	ReceivedTextInTurn bool
}

func (a *Asr) Reset() {
	a.AsrResult.Reset()
}

func (a *Asr) CancelWithReason(reason string) {
	a.lock.RLock()
	cancel := a.Cancel
	a.lock.RUnlock()

	if cancel != nil {
		log.Debugf("Asr.CancelWithReason: reason=%s", reason)
		cancel()
	}
}

func (a *Asr) RetireAsrResult(ctx context.Context) (asr_types.StreamingResult, bool, error) {
	defer func() {
		a.Reset()
	}()

	log.Log().Debugf("asr type: %s, mode: %s", a.AsrType, a.Mode)

	// Dùng biến cục bộ để theo dõi xem đã gửi sự kiện ký tự đầu tiên hay chưa
	firstTextSent := false
	var emptyResult asr_types.StreamingResult

	for {
		select {
		case <-ctx.Done():
			log.Debugf("RetireAsrResult: ctx done, exit")
			return emptyResult, false, nil
		default:
			// Tránh trường hợp khi ctx bị hủy vẫn có xác suất chọn trúng channel,
			// dẫn đến việc sử dụng kết quả của một context đã bị hủy
			select {
			case result, ok := <-a.AsrResultChannel:
				log.Debugf("asr result: %s, ok: %+v, isFinal: %+v, emptyReason: %s, error: %+v", result.Text, ok, result.IsFinal, result.EmptyReason, result.Error)
				if result.Error != nil {
					if result.RetryReason != "" {
						log.Warnf("ASR trả về lỗi có thể khôi phục (%s), giao cho tầng trên xử lý khôi phục: %v", result.RetryReason, result.Error)
						return result, true, nil
					}
					return emptyResult, false, result.Error
				}

				// Kiểm tra ký tự trả về lần đầu (văn bản không rỗng và chưa từng gửi trước đó)
				if result.Text != "" && !firstTextSent && a.ClientState != nil && a.ClientState.OnAsrFirstTextCallback != nil {
					firstTextSent = true
					// Gọi hàm callback để thông báo ký tự đầu tiên
					a.ClientState.OnAsrFirstTextCallback(result.Text, result.IsFinal)
				}

				if a.AsrType == "funasr" &&
					strings.EqualFold(a.Mode, "2pass") &&
					strings.EqualFold(result.Mode, "2pass-online") {
					if result.IsFinal {
						log.Debugf("funasr 2pass-online bị gắn nhầm cờ final, tiếp tục chờ kết quả cuối cùng từ 2pass-offline")
					}
					continue
				}

				if result.IsFinal {
					return result, true, nil
				}

				if !ok {
					log.Debugf("asr result channel closed")
					return emptyResult, true, nil
				}
			}
		}
	}
}

func (a *Asr) MarkTextReceived() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.ReceivedTextInTurn = true
}

func (a *Asr) HasReceivedText() bool {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.ReceivedTextInTurn
}

func (a *Asr) ResetReceivedText() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.ReceivedTextInTurn = false
}

func (a *Asr) StopWithReason(reason string) {
	a.lock.Lock()
	defer a.lock.Unlock()

	if a.AsrAudioChannel != nil {
		log.Debugf("Asr.StopWithReason: reason=%s", reason)
		close(a.AsrAudioChannel) // đóng channel audio đầu vào của ASR để báo ASR dừng lại và trả về kết quả
		a.AsrAudioChannel = nil  // vì đã đóng nên cần gán về nil
	}
}

func (a *Asr) Stop() {
	a.StopWithReason("Asr.Stop")
}

func (a *Asr) HasOpenAudioInput() bool {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return a.AsrAudioChannel != nil
}

func (a *Asr) AddAudioData(pcmFrameData []float32) error {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.AsrAudioChannel != nil {
		// Dùng select để gửi không chặn (non-blocking), tránh deadlock khi channel đầy
		select {
		case a.AsrAudioChannel <- pcmFrameData:
			// Gửi thành công, đồng thời lưu lại dữ liệu audio vào bộ đệm để dùng cho lịch sử chat
			a.HistoryAudioBuffer = append(a.HistoryAudioBuffer, pcmFrameData...)
		default:
			// channel đã đầy, bỏ qua dữ liệu audio lần này để tránh bị chặn dẫn đến deadlock
			log.Warnf("AsrAudioChannel đã đầy, bỏ qua dữ liệu audio lần này")
		}
	}
	return nil
}

// GetHistoryAudio lấy bộ đệm audio lịch sử (trả về bản sao, không xóa dữ liệu gốc)
func (a *Asr) GetHistoryAudio() []float32 {
	a.lock.Lock()
	defer a.lock.Unlock()
	if len(a.HistoryAudioBuffer) == 0 {
		return nil
	}
	// Trả về bản sao để tránh việc chỉnh sửa bên ngoài ảnh hưởng đến dữ liệu gốc
	result := make([]float32, len(a.HistoryAudioBuffer))
	copy(result, a.HistoryAudioBuffer)
	return result
}

// GetHistoryAudioLen lấy độ dài bộ đệm audio lịch sử (số điểm lấy mẫu - sample)
func (a *Asr) GetHistoryAudioLen() int {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return len(a.HistoryAudioBuffer)
}

// ClearHistoryAudio xóa bộ đệm audio lịch sử
func (a *Asr) ClearHistoryAudio() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.HistoryAudioBuffer = nil
}

type AsrAudioBuffer struct {
	PcmData          []float32
	AudioBufferMutex sync.RWMutex
}

func (a *AsrAudioBuffer) AddAsrAudioData(pcmFrameData []float32) error {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	a.PcmData = append(a.PcmData, pcmFrameData...)
	return nil
}

func (a *AsrAudioBuffer) GetAsrDataSize() int {
	a.AudioBufferMutex.RLock()
	defer a.AudioBufferMutex.RUnlock()
	return len(a.PcmData)
}

// GetFrameCount lấy số lượng frame (cần truyền vào kích thước frame để tính toán)
func (a *AsrAudioBuffer) GetFrameCount(frameSize int) int {
	a.AudioBufferMutex.RLock()
	defer a.AudioBufferMutex.RUnlock()
	if frameSize == 0 {
		return 0
	}
	return len(a.PcmData) / frameSize
}

func (a *AsrAudioBuffer) GetAndClearAllData() []float32 {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	pcmData := make([]float32, len(a.PcmData))
	copy(pcmData, a.PcmData)
	a.PcmData = []float32{}
	return pcmData
}

// GetAsrData lấy dữ liệu theo kiểu cửa sổ trượt - sliding window (cần truyền vào kích thước frame để tính toán)
func (a *AsrAudioBuffer) GetAsrData(frameCount int, frameSize int) []float32 {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	pcmDataLen := len(a.PcmData)
	retSize := frameCount * frameSize
	if pcmDataLen < retSize {
		retSize = pcmDataLen
	}
	pcmData := make([]float32, retSize)
	copy(pcmData, a.PcmData[pcmDataLen-retSize:])
	return pcmData
}

// RemoveAsrAudioData xóa bỏ dữ liệu audio theo số lượng frame chỉ định (cần truyền vào kích thước frame để tính toán)
func (a *AsrAudioBuffer) RemoveAsrAudioData(frameCount int, frameSize int) {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	removeSize := frameCount * frameSize
	if removeSize > len(a.PcmData) {
		removeSize = len(a.PcmData)
	}
	a.PcmData = a.PcmData[removeSize:]
}

func (a *AsrAudioBuffer) ClearAsrAudioData() {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	a.PcmData = nil
}