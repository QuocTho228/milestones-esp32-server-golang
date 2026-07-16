package aliyun_qwen3

import (
	"encoding/base64"
	"encoding/json"

	log "milestones-esp32-server-golang/logger"
)

// ClientEvent cấu trúc cơ bản của sự kiện gửi từ client
type ClientEvent struct {
	EventID string   `json:"event_id,omitempty"`
	Type    string   `json:"type"`
	Session *Session `json:"session,omitempty"`
	Audio   string   `json:"audio,omitempty"` // Dữ liệu âm thanh được mã hóa Base64
}

// Session cấu hình session trong sự kiện session.update
type Session struct {
	Modalities              []string                 `json:"modalities"`
	InputAudioFormat        string                   `json:"input_audio_format,omitempty"`
	SampleRate              int                      `json:"sample_rate,omitempty"`
	InputAudioTranscription *InputAudioTranscription `json:"input_audio_transcription,omitempty"`
	TurnDetection           *TurnDetection           `json:"turn_detection"`
}

// InputAudioTranscription cấu hình chuyển văn bản từ âm thanh (transcription)
type InputAudioTranscription struct {
	Language string `json:"language,omitempty"`
}

// TurnDetection cấu hình VAD (Voice Activity Detection - phát hiện hoạt động giọng nói)
type TurnDetection struct {
	Type              string  `json:"type,omitempty"`                // "server_vad" hoặc không thiết lập
	Threshold         float64 `json:"threshold,omitempty"`           // Ngưỡng VAD
	SilenceDurationMs int     `json:"silence_duration_ms,omitempty"` // Thời gian im lặng (đơn vị mili-giây)
}

// ServerEvent cấu trúc cơ bản của sự kiện phản hồi từ server
type ServerEvent struct {
	Type            string     `json:"type"`
	EventID         string     `json:"event_id,omitempty"`
	PreviousEventID string     `json:"previous_event_id,omitempty"`
	Session         *Session   `json:"session,omitempty"`
	Item            *Item      `json:"item,omitempty"`
	Text            string     `json:"text,omitempty"`
	Stash           string     `json:"stash,omitempty"`
	Transcript      string     `json:"transcript,omitempty"`
	Error           *ErrorInfo `json:"error,omitempty"`
}

// Item mục trong phiên (session item, ví dụ: kết quả chuyển văn bản từ âm thanh đầu vào)
type Item struct {
	ID            string         `json:"id,omitempty"`
	Type          string         `json:"type,omitempty"`
	Status        string         `json:"status,omitempty"`
	Transcription *Transcription `json:"transcription,omitempty"`
}

// Transcription kết quả chuyển văn bản từ âm thanh
type Transcription struct {
	Text     string `json:"text,omitempty"`
	Language string `json:"language,omitempty"`
}

// ErrorInfo thông tin lỗi
type ErrorInfo struct {
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

// NewSessionUpdateEvent tạo sự kiện session.update
func NewSessionUpdateEvent(config Config) *ClientEvent {
	session := &Session{
		Modalities:              []string{"text"},
		InputAudioFormat:        config.Format,
		SampleRate:              config.SampleRate,
		InputAudioTranscription: &InputAudioTranscription{Language: config.Language},
	}

	if config.AutoEnd {
		session.TurnDetection = &TurnDetection{
			Type:              "server_vad",
			Threshold:         config.VADThreshold,
			SilenceDurationMs: config.VADSilenceMs,
		}
	} else {
		session.TurnDetection = nil
	}

	event := &ClientEvent{
		EventID: "session_update",
		Type:    "session.update",
		Session: session,
	}

	// Debug: in ra sự kiện session.update
	if jsonBytes, err := json.Marshal(event); err == nil {
		log.Debugf("[aliyun_qwen3] session.update JSON: %s", string(jsonBytes))
	}

	return event
}

// NewAudioAppendEvent tạo sự kiện input_audio_buffer.append
func NewAudioAppendEvent(audioData []byte) *ClientEvent {
	encoded := base64.StdEncoding.EncodeToString(audioData)
	return &ClientEvent{
		Type:  "input_audio_buffer.append",
		Audio: encoded,
	}
}

// NewAudioCommitEvent tạo sự kiện input_audio_buffer.commit
func NewAudioCommitEvent() *ClientEvent {
	return &ClientEvent{
		EventID: "audio_commit",
		Type:    "input_audio_buffer.commit",
	}
}

// NewSessionFinishEvent tạo sự kiện session.finish
func NewSessionFinishEvent() *ClientEvent {
	return &ClientEvent{
		EventID: "session_finish",
		Type:    "session.finish",
	}
}

// IsTranscriptionEvent kiểm tra xem có phải là sự kiện chuyển văn bản từ âm thanh (transcription) hay không
func IsTranscriptionEvent(event *ServerEvent) bool {
	return event.Type == "conversation.item.input_audio_transcription.text" ||
		event.Type == "conversation.item.input_audio_transcription.completed"
}

// IsFinalTranscription kiểm tra xem có phải là kết quả chuyển văn bản cuối cùng hay không
func IsFinalTranscription(event *ServerEvent) bool {
	return event.Type == "conversation.item.input_audio_transcription.completed"
}

// GetTranscriptionText lấy văn bản đã được chuyển từ âm thanh
func GetTranscriptionText(event *ServerEvent) string {
	if event == nil {
		return ""
	}
	if event.Item != nil && event.Item.Transcription != nil && event.Item.Transcription.Text != "" {
		return event.Item.Transcription.Text
	}
	if event.Transcript != "" {
		return event.Transcript
	}
	if event.Text != "" {
		return event.Text
	}
	if event.Stash != "" {
		return event.Stash
	}
	return ""
}