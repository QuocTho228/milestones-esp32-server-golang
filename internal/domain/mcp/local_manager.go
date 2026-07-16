package mcp

import (
	"fmt"
	"sync"

	log "milestones-esp32-server-golang/logger"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"

	mcp_protocol "github.com/ThinkInAIXYZ/go-mcp/protocol"
)

// LocalMCPManager Trình quản lý công cụ MCP cục bộ
type LocalMCPManager struct {
	tools map[string]*McpTool // Tên công cụ -> Định nghĩa công cụ
	mu    sync.RWMutex        // Khóa đọc-ghi bảo vệ quyền truy cập đồng thời.
}

var (
	localManager *LocalMCPManager
	localOnce    sync.Once
)

// GetLocalMCPManager Tìm người quản lý MCP địa phương duy nhất
func GetLocalMCPManager() *LocalMCPManager {
	localOnce.Do(func() {
		localManager = &LocalMCPManager{
			tools: make(map[string]*McpTool),
		}
		// Khởi tạo các công cụ cục bộ mặc định
		localManager.initDefaultTools()
	})
	return localManager
}

// initDefaultTools Khởi tạo các công cụ cục bộ mặc định
func (l *LocalMCPManager) initDefaultTools() {

	log.Info("Quá trình khởi tạo công cụ mặc định của trình quản lý MCP cục bộ đã hoàn tất.")
}

// RegisterTool Đăng ký các công cụ địa phương
func (l *LocalMCPManager) RegisterTool(tool *McpTool) error {
	if tool == nil {
		return fmt.Errorf("Tên công cụ không được để trống.")
	}

	if tool.info.Name == "" {
		return fmt.Errorf("Tên công cụ không được để trống.")
	}

	if !tool.isLocal || tool.localHandler == nil {
		return fmt.Errorf("Chức năng xử lý công cụ không được để trống.")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Kiểm tra xem công cụ đó đã tồn tại chưa.
	if _, exists := l.tools[tool.info.Name]; exists {
		log.Warnf("Công cụ cục bộ %s đã tồn tại và sẽ bị ghi đè", tool.info.Name)
	}

	l.tools[tool.info.Name] = tool
	log.Infof("Đã đăng ký thành công các công cụ cục bộ: %s - %s", tool.info.Name, tool.info.Desc)
	return nil
}

func (l *LocalMCPManager) convertStructToOpenaipi3Schema(inputParams any) (*openapi3.Schema, error) {
	//Công cụ này được tạo ra bằng cách sử dụng github.com/ThinkInAIXYZ/go-mcp thông qua struct, sau đó được chuyển đổi thành openapi3.Schema.
	toolInstance, err := mcp_protocol.NewTool("get_system_info", "Nhận thông tin hệ thống cơ bản", inputParams)
	if err != nil {
		return nil, err
	}

	marshaledInputSchema, err := sonic.Marshal(toolInstance.InputSchema)
	if err != nil {
		return nil, err
	}

	inputSchema := &openapi3.Schema{}
	err = sonic.Unmarshal(marshaledInputSchema, inputSchema)
	if err != nil {
		return nil, err
	}
	return inputSchema, nil
}

// RegisterToolFunc Chức năng tiện ích đăng ký (phiên bản đơn giản)
func (l *LocalMCPManager) RegisterToolFunc(name, description string, inputParams any, handler LocalToolHandler) error {
	inputSchema, err := l.convertStructToOpenaipi3Schema(inputParams)
	if err != nil {
		log.Errorf("Failed to convert struct to openapi3 schema: %v", err)
		return err
	}
	tool := &McpTool{
		info: &schema.ToolInfo{
			Name:        name,
			Desc:        description,
			ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(inputSchema),
		},
		isLocal:      true,
		localHandler: handler,
	}
	return l.RegisterTool(tool)
}

// UnregisterTool Hủy đăng ký công cụ
func (l *LocalMCPManager) UnregisterTool(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.tools[name]; !exists {
		return fmt.Errorf("Công cụ %s không tồn tại", name)
	}

	delete(l.tools, name)
	log.Infof("Đã hủy đăng ký thành công công cụ cục bộ: %s", name)
	return nil
}

// GetAllTools Truy xuất tất cả các công cụ cục bộ và trả về định dạng giao diện công cụ Eino.
func (l *LocalMCPManager) GetAllTools() map[string]tool.InvokableTool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]tool.InvokableTool)
	for name, mcpTool := range l.tools {
		result[name] = mcpTool
	}
	return result
}

// GetToolByName Tìm công cụ theo tên
func (l *LocalMCPManager) GetToolByName(name string) (tool.InvokableTool, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	mcpTool, exists := l.tools[name]
	if !exists {
		return nil, false
	}

	return mcpTool, true
}

// GetToolNames Lấy danh sách tất cả tên công cụ
func (l *LocalMCPManager) GetToolNames() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.tools))
	for name := range l.tools {
		names = append(names, name)
	}
	return names
}

// GetToolCount Lấy số lượng công cụ
func (l *LocalMCPManager) GetToolCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.tools)
}

// Start Khởi chạy trình quản lý cục bộ (giao diện dành riêng).
func (l *LocalMCPManager) Start() error {
	log.Info("Người quản lý MCP cục bộ đã bắt đầu")
	return nil
}

// Stop Dừng trình quản lý cục bộ (giao diện dành riêng)
func (l *LocalMCPManager) Stop() error {
	// Lưu ý: Chúng tôi không xóa các công cụ vì các công cụ của người quản lý cục bộ cần phải luôn khả dụng trong suốt vòng đời của ứng dụng.
	// Nếu cần xóa công cụ, bạn nên gọi phương thức UnregisterTool một cách rõ ràng.
	log.Info("Trình quản lý MCP cục bộ đã dừng")
	return nil
}
