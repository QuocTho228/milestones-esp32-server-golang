package mcp

import (
	"context"
	"fmt"
	"strings"

	log "milestones-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
	mcp_go "github.com/mark3labs/mcp-go/mcp"
)

func parseSelectedMCPServiceNames(raw string) map[string]struct{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	selected := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		selected[name] = struct{}{}
	}
	if len(selected) == 0 {
		return nil
	}
	return selected
}

func isGlobalToolAllowed(toolKey string, selected map[string]struct{}) bool {
	if len(selected) == 0 {
		return true
	}
	for serviceName := range selected {
		if strings.HasPrefix(toolKey, serviceName+"_") {
			return true
		}
	}
	return false
}

func filterGlobalToolsBySelectedServices(globalTools map[string]tool.InvokableTool, selectedNames string) map[string]tool.InvokableTool {
	selected := parseSelectedMCPServiceNames(selectedNames)
	if len(selected) == 0 {
		result := make(map[string]tool.InvokableTool, len(globalTools))
		for name, invokable := range globalTools {
			result[name] = invokable
		}
		return result
	}

	result := make(map[string]tool.InvokableTool)
	for toolKey, invokable := range globalTools {
		if isGlobalToolAllowed(toolKey, selected) {
			result[toolKey] = invokable
		}
	}
	return result
}

func GetToolByName(deviceId string, agentId string, toolName string, selectedMCPServiceNames string) (tool.InvokableTool, bool) {
	return GetToolByNameWithTransport(deviceId, agentId, "", toolName, selectedMCPServiceNames)
}

func GetToolByNameWithTransport(deviceId string, agentId string, transportType string, toolName string, selectedMCPServiceNames string) (tool.InvokableTool, bool) {
	// Ưu tiên lấy từ local manager
	localManager := GetLocalMCPManager()
	tool, ok := localManager.GetToolByName(toolName)
	if ok {
		return tool, ok
	}

	// Tiếp theo lấy từ global manager
	selected := parseSelectedMCPServiceNames(selectedMCPServiceNames)
	globalManager := GetGlobalMCPManager()
	if len(selected) == 0 {
		tool, ok = globalManager.GetToolByName(toolName)
		if ok {
			return tool, ok
		}
	} else {
		globalTools := globalManager.GetAllTools()

		// Tương thích với trường hợp truyền trực tiếp "server_tool"
		if invokable, exists := globalTools[toolName]; exists && isGlobalToolAllowed(toolName, selected) {
			return invokable, true
		}

		for serviceName := range selected {
			candidate := serviceName + "_" + toolName
			if invokable, exists := globalTools[candidate]; exists {
				return invokable, true
			}
		}
	}

	// Cuối cùng lấy từ device MCP client pool, ưu tiên tool được báo cáo bởi transport hiện tại
	if transportType = strings.TrimSpace(transportType); transportType != "" {
		deviceClient := mcpClientPool.GetMcpClient(deviceId)
		if deviceClient != nil {
			tool, ok = deviceClient.GetIotToolByTransportAndName(transportType, toolName)
			if ok {
				return tool, true
			}
		}
		if agentId != "" && agentId != deviceId {
			return mcpClientPool.GetToolByDeviceId(agentId, toolName)
		}
		return nil, false
	}

	tool, ok = mcpClientPool.GetToolByDeviceId(deviceId, toolName)
	if !ok && agentId != "" && agentId != deviceId {
		tool, ok = mcpClientPool.GetToolByDeviceId(agentId, toolName)
	}
	return tool, ok
}

func GetDeviceMcpClient(deviceId string) *DeviceMcpSession {
	return mcpClientPool.GetMcpClient(deviceId)
}

func GetOrCreateDeviceMcpClient(deviceId string) *DeviceMcpSession {
	return mcpClientPool.GetOrCreateMcpClient(deviceId)
}

func AddDeviceMcpClient(deviceId string, mcpClient *DeviceMcpSession) error {
	mcpClientPool.AddMcpClient(deviceId, mcpClient)
	return nil
}

func RemoveDeviceMcpClient(deviceId string) error {
	mcpClientPool.RemoveMcpClient(deviceId)
	return nil
}

func ShouldScheduleDeviceIotOverMcp(deviceId string, conn ConnInterface) bool {
	if deviceId = strings.TrimSpace(deviceId); deviceId == "" || conn == nil {
		return false
	}
	transportType := strings.TrimSpace(conn.GetMcpTransportType())
	if transportType == "" {
		return false
	}

	session := GetDeviceMcpClient(deviceId)
	if session == nil {
		return true
	}
	return session.ShouldScheduleIotInit(transportType, conn)
}

// EnsureDeviceIotOverMcp Đảm bảo runtime IotOverMcp phía thiết bị được gắn kết (bind) với transport.
// Tái sử dụng kết nối đã có; khi transport thay đổi thì thay thế kết nối cũ.
func EnsureDeviceIotOverMcp(deviceId string, conn ConnInterface) error {
	if deviceId == "" || conn == nil {
		return fmt.Errorf("deviceId hoặc conn rỗng")
	}
	transportType := strings.TrimSpace(conn.GetMcpTransportType())
	if transportType == "" {
		return fmt.Errorf("transportType rỗng")
	}

	mcpClientSession := GetOrCreateDeviceMcpClient(deviceId)
	if mcpClientSession == nil {
		return fmt.Errorf("lấy hoặc tạo phiên MCP của thiết bị thất bại")
	}

	transportType = normalizeDeviceTransportType(transportType)

	mcpClientSession.iotMux.Lock()
	existing := mcpClientSession.iotOverMcpByTransport[transportType]
	if existing != nil && existing.conn == conn {
		if existing.IsInitializing() || existing.IsReady() {
			mcpClientSession.iotMux.Unlock()
			return nil
		}
	}

	iotOverMcpClient := NewIotOverMcpClient(deviceId, transportType, conn)
	if iotOverMcpClient == nil {
		mcpClientSession.iotMux.Unlock()
		return fmt.Errorf("tạo client IotOverMcp thất bại")
	}
	var old *McpClientInstance
	if existing := mcpClientSession.iotOverMcpByTransport[transportType]; existing != nil && existing != iotOverMcpClient {
		old = existing
	}
	mcpClientSession.iotOverMcpByTransport[transportType] = iotOverMcpClient
	iotOverMcpClient.SetOnCloseHandler(mcpClientSession.handleMcpClientClose)
	mcpClientSession.iotMux.Unlock()
	if old != nil {
		old.closeWithReason("iot_transport_replaced")
	}

	if err := iotOverMcpClient.startIotOverMcp(); err != nil {
		iotOverMcpClient.setInitState(mcpClientInitStateIdle)
		CloseDeviceIotOverMcp(deviceId, conn)
		return fmt.Errorf("khởi tạo client IotOverMcp thất bại: %w", err)
	}
	iotOverMcpClient.setInitState(mcpClientInitStateReady)

	return nil
}

func HandleDeviceIotMcpMessage(deviceId string, transportType string, payload []byte) error {
	mcpClientSession := GetDeviceMcpClient(deviceId)
	if mcpClientSession == nil {
		return nil
	}
	transportType = strings.TrimSpace(transportType)
	if transportType == "" {
		return fmt.Errorf("transportType rỗng")
	}

	mcpClientSession.iotMux.RLock()
	iotClient := mcpClientSession.iotOverMcpByTransport[normalizeDeviceTransportType(transportType)]
	mcpClientSession.iotMux.RUnlock()
	if iotClient == nil {
		return nil
	}
	if iotClient.iotTransport != nil {
		// Message MCP inbound phía thiết bị đã được route theo device + transportType tới runtime hiện tại,
		// nên tiêm (inject) trực tiếp vào transport hiện tại, tránh việc cạnh tranh tiêu thụ với runtime cũ trên hàng đợi conn dùng chung.
		iotClient.iotTransport.handleMessage(payload)
		return nil
	}
	if iotClient.conn == nil {
		return nil
	}
	return iotClient.conn.HandleMcpMessage(payload)
}

func CloseDeviceIotOverMcp(deviceId string, conn ConnInterface) {
	mcpClientSession := GetDeviceMcpClient(deviceId)
	if mcpClientSession == nil {
		return
	}
	if conn == nil {
		return
	}

	mcpClientSession.iotMux.Lock()
	transportType := normalizeDeviceTransportType(conn.GetMcpTransportType())
	iotClient := mcpClientSession.iotOverMcpByTransport[transportType]
	if iotClient == nil {
		mcpClientSession.iotMux.Unlock()
		return
	}
	if conn != nil && iotClient.conn != conn {
		mcpClientSession.iotMux.Unlock()
		return
	}
	delete(mcpClientSession.iotOverMcpByTransport, transportType)
	mcpClientSession.iotMux.Unlock()

	iotClient.closeWithReason("device_iot_closed")
}

func GetToolsByDeviceId(deviceId string, agentId string, selectedMCPServiceNames string) (map[string]tool.InvokableTool, error) {
	return GetToolsByDeviceIdWithTransport(deviceId, agentId, "", selectedMCPServiceNames)
}

func GetToolsByDeviceIdWithTransport(deviceId string, agentId string, transportType string, selectedMCPServiceNames string) (map[string]tool.InvokableTool, error) {
	retTools := make(map[string]tool.InvokableTool)

	// Ưu tiên lấy từ local manager
	localManager := GetLocalMCPManager()
	localTools := localManager.GetAllTools()
	for toolName, tool := range localTools {
		retTools[toolName] = tool
	}
	log.Infof("Lấy được %d tool từ local manager", len(localTools))

	// Tiếp theo lấy từ global manager
	globalTools := GetGlobalMCPManager().GetAllTools()
	filteredGlobalTools := filterGlobalToolsBySelectedServices(globalTools, selectedMCPServiceNames)
	for toolName, tool := range filteredGlobalTools {
		// Ưu tiên tool local, nếu đã tồn tại tool cùng tên thì không ghi đè
		if _, exists := retTools[toolName]; !exists {
			retTools[toolName] = tool
		}
	}
	log.Infof("Lấy được %d tool từ global manager (sau khi lọc)", len(filteredGlobalTools))

	if transportType = strings.TrimSpace(transportType); transportType != "" && deviceId != "" {
		deviceClient := mcpClientPool.GetMcpClient(deviceId)
		if deviceClient != nil {
			for toolName, tool := range deviceClient.GetIotToolsByTransport(transportType) {
				if _, exists := retTools[toolName]; !exists {
					retTools[toolName] = tool
				}
			}
		}
	}

	if transportType == "" {
		deviceTools, err := mcpClientPool.GetAllToolsByDeviceIdAndAgentId(deviceId, agentId)
		if err != nil {
			log.Errorf("Lấy tool của thiết bị %s thất bại: %v", deviceId, err)
			return retTools, nil
		}
		for toolName, tool := range deviceTools {
			if _, exists := retTools[toolName]; !exists {
				retTools[toolName] = tool
			}
		}
		log.Infof("Lấy được %d tool từ thiết bị %s", deviceId, len(deviceTools))
	} else if agentId != "" && agentId != deviceId {
		log.Debugf("Bắt đầu lấy tool MCP ws endpoint từ agent %s, device=%s, transport=%s", agentId, deviceId, transportType)
		agentTools, err := mcpClientPool.GetWsEndpointMcpTools(agentId)
		if err != nil {
			log.Errorf("Lấy tool của agent %s thất bại: %v", agentId, err)
			return retTools, nil
		}
		log.Debugf("Lấy được %d tool MCP ws endpoint từ agent %s, device=%s", agentId, len(agentTools), deviceId)
		for toolName, tool := range agentTools {
			if _, exists := retTools[toolName]; !exists {
				retTools[toolName] = tool
			}
		}
	}
	log.Infof("Thiết bị %s tổng cộng lấy được %d tool", deviceId, len(retTools))

	return retTools, nil
}

func GetWsEndpointMcpTools(agentId string) (map[string]tool.InvokableTool, error) {
	return mcpClientPool.GetWsEndpointMcpTools(agentId)
}

func GetWsEndpointConnectionStatus(agentId string) (bool, int) {
	if strings.TrimSpace(agentId) == "" {
		return false, 0
	}
	client := mcpClientPool.GetMcpClient(agentId)
	if client == nil {
		return false, 0
	}
	return client.GetWsEndpointConnectionStatus()
}

// GetReportedToolsByDeviceID Lấy các tool được báo cáo bởi thiết bị thông qua Iot over MCP.
// Ở phạm vi thiết bị trên console chỉ trả về tool dưới transport websocket / mqtt_udp(udp), không trộn lẫn với các loại khác như ws endpoint.
func GetReportedToolsByDeviceID(deviceId string) (map[string]tool.InvokableTool, error) {
	retTools := make(map[string]tool.InvokableTool)
	if deviceId == "" {
		return retTools, nil
	}

	client := mcpClientPool.GetMcpClient(deviceId)
	if client == nil {
		return retTools, nil
	}

	transportType, resolved := ResolveCurrentDeviceTransport(deviceId)
	if !resolved || transportType == "" {
		return retTools, nil
	}

	for toolName, invokable := range client.GetIotToolsByTransport(transportType) {
		retTools[toolName] = invokable
	}

	return retTools, nil
}

// RefreshReportedToolsByDeviceID Buộc gửi một lần tools/list tới transport hiện đang online.
// Nếu refresh thất bại thì trả về danh sách rỗng, đồng thời xóa snapshot tool trong bộ nhớ của runtime tương ứng.
func RefreshReportedToolsByDeviceID(deviceId string) (map[string]tool.InvokableTool, error) {
	retTools := make(map[string]tool.InvokableTool)
	if deviceId == "" {
		return retTools, nil
	}

	client := mcpClientPool.GetMcpClient(deviceId)
	if client == nil {
		return retTools, nil
	}

	transportType, resolved := ResolveCurrentDeviceTransport(deviceId)
	if !resolved || transportType == "" {
		return retTools, nil
	}

	return client.RefreshIotToolsByTransport(transportType)
}

// GetReportedToolsByAgentID Chỉ lấy tool MCP được báo cáo bởi agent (WebSocket endpoint)
func GetReportedToolsByAgentID(agentId string) (map[string]tool.InvokableTool, error) {
	retTools := make(map[string]tool.InvokableTool)
	if agentId == "" {
		return retTools, nil
	}

	return mcpClientPool.GetWsEndpointMcpTools(agentId)
}

// RefreshReportedToolsByAgentID Buộc gửi một lần tools/list tới ws endpoint của agent.
// Nếu refresh thất bại thì trả về danh sách rỗng, đồng thời xóa snapshot tool trong bộ nhớ của runtime tương ứng.
func RefreshReportedToolsByAgentID(agentId string) (map[string]tool.InvokableTool, error) {
	retTools := make(map[string]tool.InvokableTool)
	if agentId == "" {
		return retTools, nil
	}

	client := mcpClientPool.GetMcpClient(agentId)
	if client == nil {
		return retTools, nil
	}

	return client.RefreshWsEndpointTools()
}

// GetReportedToolByDeviceIDAndName Chỉ tìm trong các tool được thiết bị báo cáo
func GetReportedToolByDeviceIDAndName(deviceId, toolName string) (tool.InvokableTool, bool) {
	if deviceId == "" {
		return nil, false
	}

	client := mcpClientPool.GetMcpClient(deviceId)
	if client == nil {
		return nil, false
	}

	transportType, resolved := ResolveCurrentDeviceTransport(deviceId)
	if !resolved || transportType == "" {
		return nil, false
	}

	invokable, ok := client.GetIotToolByTransportAndName(transportType, toolName)
	return invokable, ok
}

// GetReportedToolByAgentIDAndName Chỉ tìm trong các tool được agent báo cáo
func GetReportedToolByAgentIDAndName(agentId, toolName string) (tool.InvokableTool, bool) {
	reportedTools, err := GetReportedToolsByAgentID(agentId)
	if err != nil {
		log.Errorf("Lấy tool MCP do agent báo cáo thất bại: agent=%s err=%v", agentId, err)
		return nil, false
	}

	return findInvokableToolByName(reportedTools, toolName)
}

func RawCallReportedToolByDeviceID(deviceId, toolName string, arguments map[string]interface{}) (string, bool, error) {
	if deviceId == "" {
		return "", false, nil
	}

	client := mcpClientPool.GetMcpClient(deviceId)
	if client == nil {
		return "", false, nil
	}

	transportType, resolved := ResolveCurrentDeviceTransport(deviceId)
	if !resolved || transportType == "" {
		return "", false, nil
	}

	return client.RawCallIotToolByTransport(context.Background(), transportType, toolName, arguments)
}

func RawCallReportedToolByAgentID(agentId, toolName string, arguments map[string]interface{}) (string, bool, error) {
	if agentId == "" {
		return "", false, nil
	}

	client := mcpClientPool.GetMcpClient(agentId)
	if client == nil {
		return "", false, nil
	}

	return client.RawCallWsEndpointTool(context.Background(), toolName, arguments)
}

// GetReportedToolsByDeviceIdAndAgentId Phương thức tương thích: phân luồng rõ ràng giữa truy vấn thiết bị/agent, không còn dùng lẫn lộn
func GetReportedToolsByDeviceIdAndAgentId(deviceId string, agentId string) (map[string]tool.InvokableTool, error) {
	if deviceId != "" {
		return GetReportedToolsByDeviceID(deviceId)
	}
	if agentId != "" {
		return GetReportedToolsByAgentID(agentId)
	}
	return make(map[string]tool.InvokableTool), nil
}

// GetReportedToolByName Phương thức tương thích: phân luồng theo phạm vi, không còn dùng lẫn lộn
func GetReportedToolByName(deviceId string, agentId string, toolName string) (tool.InvokableTool, bool) {
	if deviceId != "" {
		return GetReportedToolByDeviceIDAndName(deviceId, toolName)
	}
	if agentId != "" {
		return GetReportedToolByAgentIDAndName(agentId, toolName)
	}
	return nil, false
}

func GetAudioResourceByTool(tool McpTool, resourceLink mcp_go.ResourceLink) (mcp_go.ReadResourceResult, error) {
	/*client := tool.GetClient()
	resourceRequest := mcp_go.ReadResourceRequest{
		Request: mcp_go.Request{
			Params: mcp_go.ReadResourceParams{
				URI: resourceLink.URL,
			},
		},
	}
	client.ReadResource(context.Background(), resourceRequest)*/
	return mcp_go.ReadResourceResult{}, nil
}