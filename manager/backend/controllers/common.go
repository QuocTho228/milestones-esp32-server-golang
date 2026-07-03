package controllers

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// WebSocketControllerInterface định nghĩa interface của controller WebSocket
type WebSocketControllerInterface interface {
	RequestMcpToolDetailsFromClient(ctx context.Context, agentID string) ([]MCPTool, error)
}

// GetAgentMcpToolsCommon hàm dùng chung để lấy danh sách công cụ MCP của agent
// Hàm này có thể được dùng chung bởi controller quản trị viên và controller người dùng
func GetAgentMcpToolsCommon(
	c *gin.Context,
	agentID string,
	webSocketController WebSocketControllerInterface,
	agentValidator func(agentID string) error, // hàm xác thực quyền truy cập agent
) {
	log.Printf("GetAgentMcpToolsCommon bắt đầu thực thi, agentID: %s", agentID)

	if agentID == "" {
		log.Printf("Lỗi: tham số agent_id trống")
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}

	// Xác thực quyền truy cập agent (logic xác thực do bên gọi cung cấp)
	if err := agentValidator(agentID); err != nil {
		log.Printf("Xác thực agent thất bại: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Xác thực agent thành công, bắt đầu kiểm tra controller WebSocket")

	// Kiểm tra controller WebSocket có tồn tại không
	if webSocketController == nil {
		// Khi controller WebSocket không tồn tại, trả về danh sách rỗng thay vì báo lỗi
		log.Printf("Controller WebSocket chưa được khởi tạo, trả về danh sách công cụ rỗng")
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": []interface{}{}}})
		return
	}

	log.Printf("Controller WebSocket đã tồn tại, bắt đầu yêu cầu danh sách công cụ MCP")

	// Tạo context
	ctx := context.Background()

	// Lấy chi tiết công cụ (bao gồm schema và ví dụ mẫu)
	tools, err := webSocketController.RequestMcpToolDetailsFromClient(ctx, agentID)
	if err != nil {
		log.Printf("Lấy danh sách công cụ MCP thất bại: %v", err)
		// Nếu lấy thất bại, trả về danh sách rỗng thay vì báo lỗi
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": []interface{}{}}})
		return
	}

	log.Printf("Lấy danh sách công cụ MCP thành công: count=%d", len(tools))
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": tools}})
}

func newMcpEndpointData(endpoint string) gin.H {
	return gin.H{
		"endpoint":     endpoint,
		"status":       "unknown",
		"connected":    false,
		"tools_count":  0,
		"client_count": 0,
	}
}

func applyMcpEndpointStatus(data gin.H, statusResult map[string]interface{}) {
	if data == nil || statusResult == nil {
		return
	}

	connected, _ := statusResult["connected"].(bool)
	status, _ := statusResult["status"].(string)
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		if connected {
			status = "online"
		} else {
			status = "offline"
		}
	}

	data["connected"] = connected
	data["status"] = status
	if clientCount, ok := statusResult["client_count"]; ok {
		data["client_count"] = clientCount
	}
	if toolsCount, ok := statusResult["tools_count"]; ok {
		data["tools_count"] = toolsCount
	}
	if statusMessage, ok := statusResult["status_message"].(string); ok && strings.TrimSpace(statusMessage) != "" {
		data["status_message"] = statusMessage
	}
}