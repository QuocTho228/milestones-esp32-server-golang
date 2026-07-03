package llm

import (
	"context"

	log "milestones-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
)

// ConvertMCPToolsToEinoTools chuyển đổi các công cụ MCP sang định dạng ToolInfo của Eino.
func ConvertMCPToolsToEinoTools(ctx context.Context, mcpTools map[string]interface{}) ([]*schema.ToolInfo, error) {
	var einoTools []*schema.ToolInfo

	for toolName, mcpTool := range mcpTools {
		// Cố gắng thu thập thông tin về công cụ.
		if invokableTool, ok := mcpTool.(interface {
			Info(context.Context) (*schema.ToolInfo, error)
		}); ok {
			toolInfo, err := invokableTool.Info(ctx)
			if err != nil {
				log.Errorf("Không thể truy xuất thông tin công cụ %s: %v", toolName, err)
				continue
			}
			einoTools = append(einoTools, toolInfo)
		} else {
			log.Warnf("Công cụ %s không hỗ trợ giao diện Info, bỏ qua chuyển đổi", toolName)
		}
	}

	log.Infof("Đã chuyển đổi thành công %d công cụ MCP sang định dạng Eino", len(einoTools))
	return einoTools, nil
}
