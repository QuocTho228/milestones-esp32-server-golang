package eventbus

const (
	TopicAddMessage = "add_message"
	TopicSessionEnd = "session_end"
	TopicExitChat   = "exit_chat" // Sự kiện thoát chat

	// Các sự kiện liên quan đến lịch sử chat (đã lỗi thời, thống nhất dùng TopicAddMessage)
	// Deprecated: Sử dụng TopicAddMessage thay thế
	TopicChatHistoryUserMessage      = "chat_history_user_message"      // Tin nhắn người dùng (sau ASR) - Đã lỗi thời
	TopicChatHistoryAssistantMessage = "chat_history_assistant_message" // Phản hồi của bot (sau LLM+TTS) - Đã lỗi thời
)