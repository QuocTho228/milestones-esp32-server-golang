package mcp

import (
	"fmt"
	"net/url"
	"strings"

	log "milestones-esp32-server-golang/logger"

	"github.com/spf13/viper"
)

// CheckMCPConfig kiểm tra cấu hình MCP và báo cáo các sự cố tiềm ẩn.
func CheckMCPConfig() {
	log.Info("=== Kiểm tra cấu hình MCP ===")

	// Kiểm tra trạng thái kích hoạt toàn cục
	globalEnabled := viper.GetBool("mcp.global.enabled")
	log.Infof("Trạng thái kích hoạt MCP toàn cục: %v", globalEnabled)

	if !globalEnabled {
		log.Info("MCP toàn cục bị tắt, kiểm tra cấu hình đã hoàn tất")
		return
	}

	// Kiểm tra cấu hình kết nối lại
	reconnectInterval := viper.GetInt("mcp.global.reconnect_interval")
	maxAttempts := viper.GetInt("mcp.global.max_reconnect_attempts")
	log.Infof("Cấu hình kết nối lại: Khoảng thời gian = %d giây, Số lần thử tối đa = %d", reconnectInterval, maxAttempts)

	// Kiểm tra cấu hình máy chủ
	var serverConfigs []MCPServerConfig
	if err := viper.UnmarshalKey("mcp.global.servers", &serverConfigs); err != nil {
		log.Errorf("❌ Cấu hình máy chủ MCP không thành công: %v", err)
		return
	}

	if len(serverConfigs) == 0 {
		log.Warn("⚠️  Không có máy chủ MCP nào được cấu hình.")
		return
	}

	log.Infof("Tổng cộng có %d máy chủ MCP đã được cấu hình:", len(serverConfigs))

	enabledCount := 0
	problemCount := 0

	for i, config := range serverConfigs {
		status := "✅"
		issues := []string{}

		// Kiểm tra tên
		if config.Name == "" {
			status = "❌"
			issues = append(issues, "Tên trống")
			problemCount++
		}

		transportType, endpoint, err := endpointForConfig(config)
		if err != nil {
			status = "❌"
			issues = append(issues, err.Error())
			problemCount++
		} else {
			if _, parseErr := url.ParseRequestURI(endpoint); parseErr != nil {
				status = "❌"
				issues = append(issues, "Định dạng URL không chính xác")
				problemCount++
			}
			if transportType == "sse" && !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
				status = "⚠️"
				issues = append(issues, "URL SSE có thể không đúng định dạng")
			}
		}

		// Kiểm tra trạng thái kích hoạt
		if config.Enabled {
			enabledCount++
		}

		// Kết quả kiểm tra đầu ra
		issueStr := ""
		if len(issues) > 0 {
			issueStr = fmt.Sprintf(" - Câu hỏi: %s", strings.Join(issues, ", "))
		}

		log.Infof("  [%d] %s %s (URL: %s, cho phép: %v)%s",
			i+1, status, config.Name, endpointForLog(config), config.Enabled, issueStr)
	}

	// Tóm tắt
	log.Infof("Kiểm tra cấu hình hoàn tất: %d máy chủ đã được kích hoạt, %d máy chủ gặp sự cố.", enabledCount, problemCount)

	if problemCount > 0 {
		log.Warn("⚠️  Nếu phát hiện sự cố cấu hình, vui lòng kiểm tra các lỗi trên và khắc phục chúng.")
	}

	log.Info("=== Kiểm tra cấu hình MCP đã hoàn tất ===")
}

func endpointForLog(config MCPServerConfig) string {
	_, endpoint, err := endpointForConfig(config)
	if err != nil {
		if strings.TrimSpace(config.Url) != "" {
			return config.Url
		}
		return config.SSEUrl
	}
	return endpoint
}
