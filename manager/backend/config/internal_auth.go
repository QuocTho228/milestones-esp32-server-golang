package config

import "strings"

const DefaultInternalAuthToken = "milestones_admin_secret_key"
const DefaultEndpointAuthToken = "milestones_mcp_openclaw_secret_key"

// ResolveInternalAuthToken giải quyết Token dùng chung cho các dịch vụ nội bộ của console.
// Thứ tự ưu tiên:
// 1. internal_auth_token trong file cấu hình
// 2. Giá trị mặc định (phải nhất quán với chương trình chính)
func ResolveInternalAuthToken(cfg *Config) string {
	if cfg != nil {
		if token := strings.TrimSpace(cfg.InternalAuthToken); token != "" {
			return token
		}
	}
	return DefaultInternalAuthToken
}

// ResolveEndpointAuthToken giải quyết Token ký cho JWT của endpoint MCP/OpenClaw.
// Thứ tự ưu tiên:
// 1. endpoint_auth_token trong file cấu hình
// 2. Giá trị mặc định (phải nhất quán với chương trình chính)
func ResolveEndpointAuthToken(cfg *Config) string {
	if cfg != nil {
		if token := strings.TrimSpace(cfg.EndpointAuthToken); token != "" {
			return token
		}
	}
	return DefaultEndpointAuthToken
}