package common

import (
	"github.com/cloudwego/eino/schema"
)

// Các cấu trúc yêu cầu và phản hồi.
// Message: Đại diện cho một tin nhắn trong cuộc hội thoại.

// Các hằng số loại phản hồi.
const (
	ResponseTypeContent   = "content"
	ResponseTypeToolCalls = "tool_calls"
)

type LLMResponseStruct struct {
	Text      string            `json:"text,omitempty"`
	IsStart   bool              `json:"is_start"`
	IsEnd     bool              `json:"is_end"`
	ToolCalls []schema.ToolCall `json:"tool_calls,omitempty"`
}
