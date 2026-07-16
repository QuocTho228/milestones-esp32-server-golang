package domain

// Hằng số kiểu thông báo
const (
	MessageTypeHello  = "hello"  // Thông báo handshake
	MessageTypeAbort  = "abort"  // Thông báo tạm ngừng hoạt động
	MessageTypeListen = "listen" // Thông báo lắng nghe
	MessageTypeIot    = "iot"    // Thông báo IoT
)

// Hằng số kiểu thông báo máy chủ
const (
	ServerMessageTypeHello = "hello" // Thông báo handshake
	ServerMessageTypeStt   = "stt"   // Thông báo chuyển đổi giọng nói thành văn bản
	ServerMessageTypeTts   = "tts"   // Thông báo chuyển đổi văn bản thành giọng nói
	ServerMessageTypeIot   = "iot"   // Thông báo IoT
	ServerMessageTypeLlm   = "llm"   // Thông báo mô hình ngôn ngữ lớn
	ServerMessageTypeText  = "text"  // Thông báo văn bản
)

// Hằng số trạng thái tin nhắn
const (
	MessageStateStart   = "start"   // Trạng thái bắt đầu
	MessageStateStop    = "stop"    // Trạng thái dừng
	MessageStateDetect  = "detect"  // Trạng thái phát hiện
	MessageStateAbort   = "abort"   // Trạng thái tạm ngừng
	MessageStateSuccess = "success" // Trạng thái thành công
)
