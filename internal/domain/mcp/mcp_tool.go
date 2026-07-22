package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	log "milestones-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

var callRemoteMCPTool = func(ctx context.Context, cli *client.Client, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return cli.CallTool(ctx, request)
}

var reconnectGlobalMCPServer = func(serverName string) (*client.Client, error) {
	return GetGlobalMCPManager().reconnectServer(serverName)
}

// LocalToolHandler Kiểu hàm xử lý công cụ cục bộ (local tool)
type LocalToolHandler func(ctx context.Context, argumentsInJSON string) (string, error)

// mcpTool Triển khai công cụ MCP, hỗ trợ cả công cụ từ xa (remote) và cục bộ (local)
type McpTool struct {
	info       *schema.ToolInfo
	originName string
	serverName string
	client     *client.Client

	// Hỗ trợ công cụ cục bộ
	isLocal      bool
	localHandler LocalToolHandler
}

// Info Lấy thông tin công cụ, triển khai interface BaseTool
func (t *McpTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *McpTool) callName() string {
	if t.originName != "" {
		return t.originName
	}
	if t.info != nil {
		return t.info.Name
	}
	return ""
}

func mcpToolMatchesName(invokable tool.InvokableTool, name string) bool {
	mcpTool, ok := invokable.(*McpTool)
	if !ok || mcpTool == nil {
		return false
	}
	if mcpTool.info != nil && mcpTool.info.Name == name {
		return true
	}
	return mcpTool.originName != "" && mcpTool.originName == name
}

func findInvokableToolByName(tools map[string]tool.InvokableTool, name string) (tool.InvokableTool, bool) {
	if invokable, ok := tools[name]; ok {
		return invokable, true
	}
	for _, invokable := range tools {
		if mcpToolMatchesName(invokable, name) {
			return invokable, true
		}
	}
	return nil, false
}

func remoteCallNameForTool(invokable tool.InvokableTool, fallback string) string {
	if mcpTool, ok := invokable.(*McpTool); ok && mcpTool != nil {
		if name := mcpTool.callName(); name != "" {
			return name
		}
	}
	return fallback
}

func (t *McpTool) InvokeableLocalRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	toolInfo := t.info
	if t.localHandler == nil {
		return "", fmt.Errorf("hàm xử lý của công cụ cục bộ %s chưa được định nghĩa", toolInfo.Name)
	}

	log.Infof("Thực thi công cụ cục bộ: %s, tham số: %s", toolInfo.Name, argumentsInJSON)

	resultStr, err := t.localHandler(ctx, argumentsInJSON)
	if err != nil {
		log.Errorf("Thực thi công cụ cục bộ %s thất bại: %v", toolInfo.Name, err)
		return "", fmt.Errorf("thực thi công cụ cục bộ thất bại: %v", err)
	}
	if len(resultStr) > 2048 {
		log.Infof("Thực thi công cụ cục bộ %s thành công, độ dài kết quả: %d", toolInfo.Name, len(resultStr))
	} else {
		log.Infof("Thực thi công cụ cục bộ %s thành công, kết quả: %+s", toolInfo.Name, resultStr)
	}

	return resultStr, nil
}

// InvokableRun Gọi công cụ, triển khai interface InvokableTool
func (t *McpTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// Nếu là công cụ cục bộ, gọi trực tiếp hàm xử lý cục bộ
	if t.isLocal {
		return t.InvokeableLocalRun(ctx, argumentsInJSON, opts...)
	}

	retContent := ""

	// Logic gọi công cụ MCP từ xa (remote)
	// Kiểm tra client có sẵn sàng hay không
	if t.client == nil {
		return retContent, fmt.Errorf("gọi công cụ MCP thất bại: MCP client chưa được khởi tạo")
	}

	// Phân tích tham số
	var arguments map[string]interface{}
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &arguments); err != nil {
			return retContent, fmt.Errorf("phân tích tham số công cụ thất bại: %v", err)
		}
	}

	// Chuẩn bị request gọi
	toolName := t.callName()
	callRequest := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: arguments,
		},
	}

	result, err := callRemoteMCPTool(ctx, t.client, callRequest)
	if err != nil {
		if !isRetryableRemoteCallError(err) {
			return retContent, fmt.Errorf("gọi công cụ thất bại: %v", err)
		}

		log.Warnf("Công cụ %s gọi thất bại, chuẩn bị kết nối lại server %s rồi thử lại: %v", t.info.Name, t.serverName, err)

		newClient, reconnectErr := reconnectGlobalMCPServer(t.serverName)
		if reconnectErr != nil {
			return retContent, fmt.Errorf("gọi công cụ thất bại: %v, và kết nối lại server cũng thất bại: %v", err, reconnectErr)
		}

		t.client = newClient
		result, err = callRemoteMCPTool(ctx, t.client, callRequest)
		if err != nil {
			return retContent, fmt.Errorf("gọi lại sau khi kết nối lại vẫn thất bại: %v", err)
		}
	}

	if err != nil {
		return retContent, fmt.Errorf("gọi công cụ thất bại: %v", err)
	}

	resultStr, err := result.MarshalJSON()
	if err != nil {
		return retContent, fmt.Errorf("chuyển đổi nội dung trả về từ lời gọi công cụ thất bại: %v", err)
	}

	return string(resultStr), nil
}

func (t *McpTool) GetClient() *client.Client {
	return t.client
}

func (t *McpTool) GetServerName() string {
	return t.serverName
}