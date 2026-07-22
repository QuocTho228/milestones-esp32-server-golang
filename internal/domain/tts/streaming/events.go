package streaming

// SentenceSignalType đại diện cho loại tín hiệu điều khiển ở cấp câu cần được gửi trước một đoạn âm thanh.
type SentenceSignalType string

const (
	SentenceSignalStart SentenceSignalType = "sentence_start"
	SentenceSignalEnd   SentenceSignalType = "sentence_end"
)

// SentenceSignal đại diện cho tín hiệu ranh giới câu có thứ tự, gắn liền với khối âm thanh hiện tại.
type SentenceSignal struct {
	Type SentenceSignalType
	Text string
}

// SynthesisEvent đại diện cho một đoạn đầu ra TTS song luồng (dual-stream).
// Audio là khối âm thanh hiện tại; SentenceSignals là các tín hiệu ranh giới câu cần được gửi trước khi gửi khối âm thanh này.
type SynthesisEvent struct {
	Audio           []byte
	SentenceSignals []SentenceSignal
}