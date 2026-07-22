package chat

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	mcp_go "github.com/mark3labs/mcp-go/mcp"
)

// MCPResponseType định nghĩa loại phản hồi (response) của MCP
type MCPResponseType string

const (
	// Loại hành động (action): cần thực hiện một hành động cụ thể, thường sẽ dừng các bước xử lý tiếp theo
	MCPResponseTypeAction MCPResponseType = "action"
	// Loại tài nguyên âm thanh (audio): cần thực hiện một hành động cụ thể, thường dừng xử lý tiếp theo, nhưng không cần trả về stop
	MCPResponseTypeAudio MCPResponseType = "audio"

	// Loại nội dung (content): trả về thông tin nội dung, cho phép tiếp tục xử lý sau đó
	MCPResponseTypeContent MCPResponseType = "content"
	// Loại lỗi (error): xử lý các trường hợp lỗi
	MCPResponseTypeError MCPResponseType = "error"
)

// MCPResponseBase là cấu trúc cơ bản dùng chung cho mọi phản hồi MCP
type MCPResponseBase struct {
	Type      MCPResponseType `json:"type"`
	Success   bool            `json:"success"`
	Timestamp int64           `json:"timestamp"`
	ToolName  string          `json:"tool_name"`
}

// MCPActionResponse phản hồi loại hành động - dùng cho các tình huống cần thực hiện hành động như phát nhạc, thoát cuộc trò chuyện, v.v.
type MCPActionResponse struct {
	MCPResponseBase
	Action   string            `json:"action"`
	Message  string            `json:"message"`
	Status   string            `json:"status"`
	Metadata map[string]string `json:"metadata,omitempty"`
	// Các cờ điều khiển
	FinalAction       bool   `json:"final_action"`
	NoFurtherResponse bool   `json:"no_further_response"`
	SilenceLLM        bool   `json:"silence_llm"`
	UserState         string `json:"user_state"`
	Instruction       string `json:"instruction,omitempty"`
}

// MCPAudioResponse phản hồi loại hành động - dùng cho các tình huống cần thực hiện hành động như phát nhạc, thoát cuộc trò chuyện, v.v.
type MCPAudioResponse struct {
	MCPResponseBase
	Data      []byte            `json:"data"`
	MusicName string            `json:"music_name"`
	Action    string            `json:"action"`
	Status    string            `json:"status"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	// Các cờ điều khiển
	FinalAction bool `json:"final_action"`
}

// MCPContentResponse phản hồi loại nội dung - dùng cho các tình huống lấy thời gian, tra cứu thông tin, trả về dữ liệu, v.v.
type MCPContentResponse struct {
	MCPResponseBase
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// MCPErrorResponse phản hồi loại lỗi - xử lý lỗi một cách thống nhất
type MCPErrorResponse struct {
	MCPResponseBase
	Error      string `json:"error"`
	ErrorCode  string `json:"error_code,omitempty"`
	Details    string `json:"details,omitempty"`
	Suggestion string `json:"suggestion,omitempty"` // Gợi ý dành cho người dùng
}

// MCPResponse là interface phản hồi MCP thống nhất
type MCPResponse interface {
	GetType() MCPResponseType
	GetSuccess() bool
	IsTerminal() bool // Có phải là thao tác kết thúc (terminal) hay không
	ToJSON() (string, error)
	GetContent() []mcp_go.Content
	GetAction() string // Lấy loại hành động
}

// Triển khai interface MCPResponse
func (r *MCPActionResponse) GetType() MCPResponseType { return MCPResponseTypeAction }
func (r *MCPActionResponse) GetSuccess() bool         { return r.Success }
func (r *MCPActionResponse) IsTerminal() bool         { return r.FinalAction || r.NoFurtherResponse }
func (r *MCPActionResponse) GetAction() string        { return r.Action }
func (r *MCPActionResponse) GetContent() []mcp_go.Content {
	return []mcp_go.Content{
		mcp_go.TextContent{
			Type: "text",
			Text: r.Message,
		},
	}
}

// Bổ sung triển khai các phương thức interface cho MCPAudioResponse
func (r *MCPAudioResponse) GetType() MCPResponseType { return MCPResponseTypeAudio }
func (r *MCPAudioResponse) GetSuccess() bool         { return r.Success }
func (r *MCPAudioResponse) IsTerminal() bool         { return r.FinalAction }
func (r *MCPAudioResponse) GetAction() string        { return r.Action }
func (r *MCPAudioResponse) GetContent() []mcp_go.Content {
	return []mcp_go.Content{
		mcp_go.TextContent{
			Type: "text",
			Text: r.MusicName,
		},
		mcp_go.AudioContent{
			Type:     "audio",
			Data:     base64.StdEncoding.EncodeToString(r.Data),
			MIMEType: "audio/mpeg",
		},
	}
}

func (r *MCPContentResponse) GetType() MCPResponseType { return MCPResponseTypeContent }
func (r *MCPContentResponse) GetSuccess() bool         { return r.Success }
func (r *MCPContentResponse) IsTerminal() bool         { return false } // Loại nội dung thường không kết thúc luồng xử lý
func (r *MCPContentResponse) GetAction() string        { return "" }    // Loại nội dung không có hành động
func (r *MCPContentResponse) GetContent() []mcp_go.Content {
	return []mcp_go.Content{
		mcp_go.TextContent{
			Type: "text",
			Text: r.Message,
		},
	}
}

func (r *MCPErrorResponse) GetType() MCPResponseType { return MCPResponseTypeError }
func (r *MCPErrorResponse) GetSuccess() bool         { return r.Success }
func (r *MCPErrorResponse) IsTerminal() bool         { return false } // Loại lỗi cho phép tiếp tục xử lý sau đó
func (r *MCPErrorResponse) GetAction() string        { return "" }    // Loại lỗi không có hành động
func (r *MCPErrorResponse) GetContent() []mcp_go.Content {
	return []mcp_go.Content{
		mcp_go.TextContent{
			Type: "text",
			Text: r.Error,
		},
	}
}

// Triển khai phương thức ToJSON
func (r *MCPActionResponse) ToJSON() (string, error) {
	data, err := json.Marshal(r)
	return string(data), err
}

// Bổ sung phương thức ToJSON cho MCPAudioResponse
func (r *MCPAudioResponse) ToJSON() (string, error) {
	data, err := json.Marshal(r)
	return string(data), err
}

func (r *MCPContentResponse) ToJSON() (string, error) {
	data, err := json.Marshal(r)
	return string(data), err
}

func (r *MCPErrorResponse) ToJSON() (string, error) {
	data, err := json.Marshal(r)
	return string(data), err
}

// Các hàm khởi tạo tiện lợi (constructor)

// NewActionResponse tạo phản hồi loại hành động
func NewActionResponse(toolName, action, message, status string, terminal bool) *MCPActionResponse {
	return &MCPActionResponse{
		MCPResponseBase: MCPResponseBase{
			Type:      MCPResponseTypeAction,
			Success:   true,
			Timestamp: time.Now().Unix(),
			ToolName:  toolName,
		},
		Action:            action,
		Message:           message,
		Status:            status,
		FinalAction:       terminal,
		NoFurtherResponse: terminal,
		SilenceLLM:        terminal,
	}
}

// NewAudioResponse tạo phản hồi loại âm thanh - đã sửa lại kiểu trả về
func NewAudioResponse(toolName, action, status string, terminal bool, data []byte) *MCPAudioResponse {
	return &MCPAudioResponse{
		MCPResponseBase: MCPResponseBase{
			Type:      MCPResponseTypeAudio,
			Success:   true,
			Timestamp: time.Now().Unix(),
			ToolName:  toolName,
		},
		Data:        data,
		Action:      action,
		Status:      status,
		FinalAction: terminal,
	}
}

// NewContentResponse tạo phản hồi loại nội dung
func NewContentResponse(toolName string, data interface{}, message string) *MCPContentResponse {
	return &MCPContentResponse{
		MCPResponseBase: MCPResponseBase{
			Type:      MCPResponseTypeContent,
			Success:   true,
			Timestamp: time.Now().Unix(),
			ToolName:  toolName,
		},
		Data:    data,
		Message: message,
	}
}

// NewErrorResponse tạo phản hồi loại lỗi
func NewErrorResponse(toolName, error, errorCode, suggestion string) *MCPErrorResponse {
	return &MCPErrorResponse{
		MCPResponseBase: MCPResponseBase{
			Type:      MCPResponseTypeError,
			Success:   false,
			Timestamp: time.Now().Unix(),
			ToolName:  toolName,
		},
		Error:      error,
		ErrorCode:  errorCode,
		Suggestion: suggestion,
	}
}

// ParseMCPResponse phân tích (parse) phản hồi MCP từ chuỗi JSON
func ParseMCPResponse(jsonStr string) (MCPResponse, error) {
	var base MCPResponseBase
	if err := json.Unmarshal([]byte(jsonStr), &base); err != nil {
		return nil, err
	}

	switch base.Type {
	case MCPResponseTypeAction:
		var response MCPActionResponse
		if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
			return nil, err
		}
		return &response, nil
	case MCPResponseTypeAudio:
		var response MCPAudioResponse
		if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
			return nil, err
		}
		return &response, nil
	case MCPResponseTypeContent:
		var response MCPContentResponse
		if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
			return nil, err
		}
		return &response, nil
	case MCPResponseTypeError:
		var response MCPErrorResponse
		if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
			return nil, err
		}
		return &response, nil
	default:
		return NewErrorResponse("unknown", "Loại phản hồi không xác định", "INVALID_TYPE", "Vui lòng kiểm tra lại cách triển khai công cụ"), fmt.Errorf("loại phản hồi không xác định: %s", base.Type)
	}
}