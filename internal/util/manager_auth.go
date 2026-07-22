package util

import (
	"strings"

	"github.com/spf13/viper"
)

const DefaultManagerAuthToken = "milestones_admin_secret_key"
const DefaultManagerEndpointAuthToken = "milestones_mcp_openclaw_secret_key"

// GetManagerAuthToken lấy Token xác thực nội bộ dùng chung giữa chương trình chính và console.
// Thứ tự ưu tiên:
// 1. manager.auth_token
// 2. Giá trị mặc định (hai bên phải nhất quán)
func GetManagerAuthToken() string {
	if token := strings.TrimSpace(viper.GetString("manager.auth_token")); token != "" {
		return token
	}
	return DefaultManagerAuthToken
}

// GetManagerEndpointAuthToken lấy Token ký/xác thực JWT cho endpoint MCP/OpenClaw.
// Thứ tự ưu tiên:
// 1. manager.endpoint_auth_token
// 2. Giá trị mặc định (cần nhất quán với console)
func GetManagerEndpointAuthToken() string {
	if token := strings.TrimSpace(viper.GetString("manager.endpoint_auth_token")); token != "" {
		return token
	}
	return DefaultManagerEndpointAuthToken
}