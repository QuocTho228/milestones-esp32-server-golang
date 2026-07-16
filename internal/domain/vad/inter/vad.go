package inter

// Giao diện phát hiện hoạt động giọng nói VAD
type VAD interface {
	// IsVAD phát hiện hoạt động lời nói trong dữ liệu âm thanh.
	IsVAD(pcmData []float32) (bool, error)

	IsVADExt(pcmData []float32, sampleRate int, frameSize int) (bool, error)
	// Thao tác Reset sẽ khôi phục trạng thái của đầu dò.
	Reset() error
	// Đóng và giải phóng tài nguyên.
	Close() error
	// IsValid kiểm tra xem một tài nguyên có hợp lệ hay không.
	IsValid() bool
}
