package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

const validateTimeout = 25 * time.Second

// ValidateMCPConfigMap Thực hiện kiểm tra sơ bộ ở cấp độ kết nối (initialize + tools/list) cho cấu hình mcp truyền vào.
func ValidateMCPConfigMap(mcpConfig map[string]interface{}) error {
	if mcpConfig == nil {
		return fmt.Errorf("cấu hình mcp trống")
	}

	global := asAnyMap(mcpConfig["global"])
	if global == nil {
		return fmt.Errorf("thiếu cấu hình mcp.global")
	}

	enabled := toBool(global["enabled"])
	if !enabled {
		return nil
	}

	servers, err := decodeServerConfigs(global["servers"])
	if err != nil {
		return fmt.Errorf("phân tích mcp.global.servers thất bại: %w", err)
	}
	if len(servers) == 0 {
		return fmt.Errorf("mcp.global.enabled=true nhưng servers trống")
	}

	return ValidateServerConfigs(servers)
}

// ValidateServerConfigs Kiểm tra tính khả dụng của cấu hình server.
func ValidateServerConfigs(serverConfigs []MCPServerConfig) error {
	if len(serverConfigs) == 0 {
		return fmt.Errorf("chưa cung cấp bất kỳ cấu hình server MCP nào")
	}

	errs := make([]string, 0)
	enabledCount := 0
	for _, cfg := range serverConfigs {
		if !cfg.Enabled {
			continue
		}
		enabledCount++
		if err := validateSingleServer(cfg); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", cfg.Name, err))
		}
	}

	if enabledCount == 0 {
		return fmt.Errorf("không có server MCP nào được bật")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func validateSingleServer(config MCPServerConfig) error {
	transportInstance, endpoint, err := buildMCPTransport(config)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), validateTimeout)
	defer cancel()

	mcpClient := client.NewClient(transportInstance)
	defer mcpClient.Close()

	if err := mcpClient.Start(ctx); err != nil {
		return fmt.Errorf("khởi động thất bại(%s): %w", endpoint, err)
	}

	initReq := mcpgo.InitializeRequest{
		Params: mcpgo.InitializeParams{
			ProtocolVersion: mcpgo.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcpgo.Implementation{
				Name:    "milestones-mcp-validator",
				Version: "1.0.0",
			},
			Capabilities: mcpgo.ClientCapabilities{
				Experimental: make(map[string]any),
			},
		},
	}

	if _, err := mcpClient.Initialize(ctx, initReq); err != nil {
		return fmt.Errorf("initialize thất bại: %w", err)
	}

	if _, err := mcpClient.ListTools(ctx, mcpgo.ListToolsRequest{}); err != nil {
		return fmt.Errorf("tools/list thất bại: %w", err)
	}

	return nil
}

func decodeServerConfigs(v interface{}) ([]MCPServerConfig, error) {
	if v == nil {
		return []MCPServerConfig{}, nil
	}
	body, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var ret []MCPServerConfig
	if err := json.Unmarshal(body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func asAnyMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	ret, _ := v.(map[string]interface{})
	return ret
}

func toBool(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	case float64:
		return t != 0
	case int:
		return t != 0
	default:
		return false
	}
}