package history

import (
	"context"
	"fmt"
	"time"

	"milestones-esp32-server-golang/internal/components/http"

	"github.com/cloudwego/eino/schema"
)

// MessageType loại tin nhắn
type MessageType string

const (
	MessageTypeUser      MessageType = "user"
	MessageTypeAssistant MessageType = "assistant"
	MessageTypeTool      MessageType = "tool"   // kết quả gọi tool (công cụ)
	MessageTypeSystem    MessageType = "system" // tin nhắn hệ thống (nếu có sử dụng)
)

// HistoryClientConfig cấu hình client
type HistoryClientConfig struct {
	BaseURL   string        // địa chỉ backend Manager
	AuthToken string        // Token xác thực
	Timeout   time.Duration // thời gian timeout của request
	Enabled   bool          // có bật (enable) hay không
}

// HistoryClient client HTTP cho lịch sử chat
type HistoryClient struct {
	client  *http.ManagerClient
	enabled bool
}

// NewHistoryClient tạo client lịch sử chat
func NewHistoryClient(cfg HistoryClientConfig) *HistoryClient {
	managerClient := http.NewManagerClient(http.ManagerClientConfig{
		BaseURL:    cfg.BaseURL,
		AuthToken:  cfg.AuthToken,
		Timeout:    cfg.Timeout,
		MaxRetries: 3, // mặc định thử lại 3 lần
	})

	return &HistoryClient{
		client:  managerClient,
		enabled: cfg.Enabled,
	}
}

// SaveMessageRequest request lưu tin nhắn
type SaveMessageRequest struct {
	MessageID     string                 `json:"message_id"`
	DeviceID      string                 `json:"device_id"`
	AgentID       string                 `json:"agent_id"`
	SessionID     string                 `json:"session_id,omitempty"`
	Role          MessageType            `json:"role"`
	Content       string                 `json:"content"`
	ToolCallID    string                 `json:"tool_call_id,omitempty"`    // ID gọi tool (dùng cho vai trò Tool)
	ToolCallsJSON *string                `json:"tool_calls_json,omitempty"` // danh sách lệnh gọi tool dạng JSON (dùng cho vai trò Assistant), nil nghĩa là NULL
	AudioData     string                 `json:"audio_data,omitempty"`      // mã hóa base64
	AudioFormat   string                 `json:"audio_format,omitempty"`
	AudioDuration int                    `json:"audio_duration,omitempty"`
	AudioSize     int                    `json:"audio_size,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// SaveMessage lưu tin nhắn
func (c *HistoryClient) SaveMessage(ctx context.Context, req *SaveMessageRequest) error {
	if !c.enabled {
		return nil
	}
	return c.client.DoRequest(ctx, http.RequestOptions{
		Method: "POST",
		Path:   "/api/internal/history/messages",
		Body:   req,
	})
}

// UpdateMessageAudioRequest request cập nhật audio của tin nhắn
type UpdateMessageAudioRequest struct {
	MessageID   string                 `json:"message_id"`
	AudioData   string                 `json:"audio_data"` // mã hóa base64
	AudioFormat string                 `json:"audio_format"`
	AudioSize   int                    `json:"audio_size"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateMessageAudio cập nhật audio của tin nhắn
func (c *HistoryClient) UpdateMessageAudio(ctx context.Context, req *UpdateMessageAudioRequest) error {
	if !c.enabled {
		return nil
	}
	return c.client.DoRequest(ctx, http.RequestOptions{
		Method: "PUT",
		Path:   "/api/internal/history/messages/" + req.MessageID + "/audio",
		Body:   req,
	})
}

// GetMessagesRequest request lấy danh sách tin nhắn
type GetMessagesRequest struct {
	DeviceID  string `json:"device_id"`
	AgentID   string `json:"agent_id"`
	SessionID string `json:"session_id,omitempty"`
	Limit     int    `json:"limit"` // giới hạn số lượng
}

// GetMessagesResponse response trả về danh sách tin nhắn
type GetMessagesResponse struct {
	Messages []MessageItem `json:"messages"`
}

// MessageItem một mục tin nhắn (dùng cho việc tải dữ liệu khởi tạo, không bao gồm audio)
type MessageItem struct {
	MessageID  string            `json:"message_id"`
	Role       string            `json:"role"` // user/assistant/tool/system
	Content    string            `json:"content"`
	ToolCallID string            `json:"tool_call_id,omitempty"` // dùng cho vai trò Tool
	ToolCalls  []schema.ToolCall `json:"tool_calls,omitempty"`   // dùng cho vai trò Assistant
	CreatedAt  string            `json:"created_at"`
}

// GetMessages lấy tin nhắn từ cơ sở dữ liệu Manager (dùng cho việc tải dữ liệu khởi tạo)
func (c *HistoryClient) GetMessages(ctx context.Context, req *GetMessagesRequest) (*GetMessagesResponse, error) {
	if !c.enabled {
		return nil, fmt.Errorf("history client is disabled")
	}

	// xây dựng các tham số truy vấn (query params)
	queryParams := map[string]string{
		"device_id": req.DeviceID,
		"agent_id":  req.AgentID,
		"limit":     fmt.Sprintf("%d", req.Limit),
	}
	if req.SessionID != "" {
		queryParams["session_id"] = req.SessionID
	}

	var resp GetMessagesResponse
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method:      "GET",
		Path:        "/api/internal/history/messages",
		QueryParams: queryParams,
		Response:    &resp,
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}