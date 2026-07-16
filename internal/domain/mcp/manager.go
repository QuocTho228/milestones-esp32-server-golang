package mcp

import (
	"fmt"
	"sync"

	log "milestones-esp32-server-golang/logger"
)

// MCPManager là người quản lý MCP thống nhất, chịu trách nhiệm điều phối tất cả các người quản lý chương trình con.
type MCPManager struct {
	localManager  *LocalMCPManager
	globalManager *GlobalMCPManager
	// Trong tương lai, deviceManager sẽ có khả năng quản lý các nhóm trình quản lý thiết bị tại đây.

	mu      sync.RWMutex
	started bool
}

var (
	mcpManager *MCPManager
	mcpOnce    sync.Once
)

// GetMCPManager Tải xuống Unified MCP Manager Singleton
func GetMCPManager() *MCPManager {
	mcpOnce.Do(func() {
		mcpManager = &MCPManager{
			localManager:  GetLocalMCPManager(),
			globalManager: GetGlobalMCPManager(),
			started:       false,
		}
	})
	return mcpManager
}

// Start Khởi động tất cả các trình quản lý MCP
func (m *MCPManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		log.Warn("Trình quản lý MCP đã được khởi động.")
		return nil
	}

	log.Info("=== Khởi động cụm MCP Manager ===")

	// 1. Trước tiên, hãy khởi động Trình quản lý cục bộ.
	log.Info("Khởi chạy trình quản lý MCP cục bộ...")
	if err := m.localManager.Start(); err != nil {
		log.Errorf("Không thể khởi động trình quản lý MCP cục bộ.: %v", err)
		return fmt.Errorf("Khởi động trình quản lý MCP cục bộ thất bại: %v", err)
	}

	// 2. Sau đó khởi chạy trình quản lý toàn cầu.
	log.Info("Khởi chạy trình quản lý MCP toàn cục...")
	if err := m.globalManager.Start(); err != nil {
		log.Errorf("Không thể khởi động trình quản lý MCP toàn cục: %v", err)
		return fmt.Errorf("Không thể khởi động trình quản lý MCP toàn cục: %v", err)
	}

	// 3. Trình quản lý thiết bị được tạo động khi kết nối; không cần phải khởi động nó ở đây.
	log.Info("Trình quản lý MCP của thiết bị sẽ tự động tạo thiết bị dựa trên kết nối.")

	m.started = true
	log.Info("=== Quá trình khởi động cụm MCP Manager hoàn tất ===")

	// Thống kê trạng thái khởi động đầu ra
	m.printStartupStats()

	return nil
}

// Stop Dừng tất cả các trình quản lý MCP
func (m *MCPManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		log.Info("MCP Manager hiện không chạy và không cần phải dừng lại")
		return nil
	}

	log.Info("=== Dừng cụm quản lý MCP ===")

	// Dừng người quản lý theo thứ tự ngược lại.
	// 1. Dừng quản lý toàn cầu
	log.Info("Dừng trình quản lý MCP toàn cục...")
	if err := m.globalManager.Stop(); err != nil {
		log.Errorf("Không thể dừng trình quản lý MCP toàn cục: %v", err)
	}

	// 2. Dừng quản lý cục bộ
	log.Info("Dừng việc quản lý MCP cục bộ...")
	if err := m.localManager.Stop(); err != nil {
		log.Errorf("Dừng trình quản lý MCP cục bộ thất bại: %v", err)
	}

	// 3. Trình quản lý thiết bị tự động dọn dẹp khi mất kết nối.
	log.Info("Kết nối MCP của thiết bị sẽ được tự động làm sạch.")

	m.started = false
	log.Info("=== Cụm máy chủ MCP Manager đã bị dừng hoạt động ===")
	return nil
}

// IsStarted kiểm tra xem quản lý có đang chạy không
func (m *MCPManager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// GetLocalManager Tìm người quản lý cục bộ
func (m *MCPManager) GetLocalManager() *LocalMCPManager {
	return m.localManager
}

// GetGlobalManager Lấy Quản lý Toàn cầu
func (m *MCPManager) GetGlobalManager() *GlobalMCPManager {
	return m.globalManager
}

// printStartupStats Thống kê trạng thái khởi động đầu ra
func (m *MCPManager) printStartupStats() {
	localToolCount := m.localManager.GetToolCount()
	globalToolCount := len(m.globalManager.GetAllTools())

	log.Infof("Thống kê khởi động trình quản lý MCP:")
	log.Infof("  - Số lượng công cụ cục bộ: %d", localToolCount)
	log.Infof("  - Số lượng công cụ toàn cục: %d", globalToolCount)
	log.Infof("  - Quản lý thiết bị: Quản lý động")
	log.Infof("  - Tổng số lượng công cụ: %d", localToolCount+globalToolCount)
}

// GetAllManagersStatus Lấy thông tin trạng thái của tất cả các quản lý.
func (m *MCPManager) GetAllManagersStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := map[string]interface{}{
		"mcp_manager": map[string]interface{}{
			"started": m.started,
		},
		"local_manager": map[string]interface{}{
			"tool_count": m.localManager.GetToolCount(),
			"tool_names": m.localManager.GetToolNames(),
		},
		"global_manager": map[string]interface{}{
			"tool_count": len(m.globalManager.GetAllTools()),
		},
		"device_manager": map[string]interface{}{
			"active_devices": mcpClientPool.device2McpClient.Count(),
		},
	}

	return status
}

// RestartManager Khởi động lại trình quản lý được chỉ định
func (m *MCPManager) RestartManager(managerType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return fmt.Errorf("Cụm MCP Manager chưa được khởi động")
	}

	switch managerType {
	case "local":
		log.Info("Khởi động lại trình quản lý MCP cục bộ...")
		if err := m.localManager.Stop(); err != nil {
			log.Errorf("Dừng trình quản lý MCP cục bộ thất bại: %v", err)
		}
		if err := m.localManager.Start(); err != nil {
			return fmt.Errorf("Khởi động lại trình quản lý cục bộ thất bại: %v", err)
		}
		log.Info("Quá trình khởi động lại Trình quản lý MCP cục bộ đã hoàn tất.")

	case "global":
		log.Info("Khởi động lại trình quản lý MCP toàn cục...")
		if err := m.globalManager.Stop(); err != nil {
			log.Errorf("Không thể ngăn chặn người quản lý toàn cục: %v", err)
		}
		if err := m.globalManager.Start(); err != nil {
			return fmt.Errorf("Khởi động lại trình quản lý toàn cục đã thất bại: %v", err)
		}
		log.Info("Quá trình khởi động lại Trình quản lý MCP toàn cục đã hoàn tất.")

	default:
		return fmt.Errorf("Các loại quản trị viên không được hỗ trợ: %s", managerType)
	}

	return nil
}

// Để đảm bảo khả năng tương thích ngược, các chức năng tiện lợi được cung cấp.

// StartMCPManagers Khởi chạy tất cả các trình quản lý MCP (chức năng tiện ích)
func StartMCPManagers() error {
	return GetMCPManager().Start()
}

// StopMCPManagers Dừng tất cả các trình quản lý MCP (chức năng tiện ích)
func StopMCPManagers() error {
	return GetMCPManager().Stop()
}

// GetMCPManagerStatus Lấy trạng thái của trình quản lý MCP (chức năng tiện ích)
func GetMCPManagerStatus() map[string]interface{} {
	return GetMCPManager().GetAllManagersStatus()
}
