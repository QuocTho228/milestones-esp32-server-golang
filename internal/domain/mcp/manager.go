package mcp

import (
	"fmt"
	"sync"

	log "milestones-esp32-server-golang/logger"
)

// MCPManager 统一的MCP管理器，负责协调所有子管理器
type MCPManager struct {
	localManager  *LocalMCPManager
	globalManager *GlobalMCPManager
	// deviceManager 将来可以在这里管理设备管理器池

	mu      sync.RWMutex
	started bool
}

var (
	mcpManager *MCPManager
	mcpOnce    sync.Once
)

// GetMCPManager 获取统一MCP管理器单例
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

// Start 启动所有MCP管理器
func (m *MCPManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		log.Warn("Trình quản lý MCP đã được khởi động.")
		return nil
	}

	log.Info("=== Khởi động cụm MCP Manager ===")

	// 1. 首先启动本地管理器
	log.Info("Khởi chạy trình quản lý MCP cục bộ...")
	if err := m.localManager.Start(); err != nil {
		log.Errorf("Không thể khởi động trình quản lý MCP cục bộ.: %v", err)
		return fmt.Errorf("Khởi động trình quản lý MCP cục bộ thất bại: %v", err)
	}

	// 2. 然后启动全局管理器
	log.Info("Khởi chạy trình quản lý MCP toàn cục...")
	if err := m.globalManager.Start(); err != nil {
		log.Errorf("Không thể khởi động trình quản lý MCP toàn cục: %v", err)
		return fmt.Errorf("Không thể khởi động trình quản lý MCP toàn cục: %v", err)
	}

	// 3. 设备管理器通过连接时动态创建，这里不需要启动
	log.Info("Trình quản lý MCP của thiết bị sẽ tự động tạo thiết bị dựa trên kết nối.")

	m.started = true
	log.Info("=== Quá trình khởi động cụm MCP Manager hoàn tất ===")

	// 输出启动状态统计
	m.printStartupStats()

	return nil
}

// Stop 停止所有MCP管理器
func (m *MCPManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		log.Info("MCP Manager hiện không chạy và không cần phải dừng lại")
		return nil
	}

	log.Info("=== Dừng cụm quản lý MCP ===")

	// 按相反顺序停止管理器
	// 1. 停止全局管理器
	log.Info("Dừng trình quản lý MCP toàn cục...")
	if err := m.globalManager.Stop(); err != nil {
		log.Errorf("Không thể dừng trình quản lý MCP toàn cục: %v", err)
	}

	// 2. 停止本地管理器
	log.Info("Dừng việc quản lý MCP cục bộ...")
	if err := m.localManager.Stop(); err != nil {
		log.Errorf("Dừng trình quản lý MCP cục bộ thất bại: %v", err)
	}

	// 3. 设备管理器通过连接断开自动清理
	log.Info("Kết nối MCP của thiết bị sẽ được tự động làm sạch.")

	m.started = false
	log.Info("=== Cụm máy chủ MCP Manager đã bị dừng hoạt động ===")
	return nil
}

// IsStarted 检查管理器是否已启动
func (m *MCPManager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// GetLocalManager 获取本地管理器
func (m *MCPManager) GetLocalManager() *LocalMCPManager {
	return m.localManager
}

// GetGlobalManager 获取全局管理器
func (m *MCPManager) GetGlobalManager() *GlobalMCPManager {
	return m.globalManager
}

// printStartupStats 输出启动状态统计
func (m *MCPManager) printStartupStats() {
	localToolCount := m.localManager.GetToolCount()
	globalToolCount := len(m.globalManager.GetAllTools())

	log.Infof("Thống kê khởi động trình quản lý MCP:")
	log.Infof("  - Số lượng công cụ cục bộ: %d", localToolCount)
	log.Infof("  - Số lượng công cụ toàn cục: %d", globalToolCount)
	log.Infof("  - Quản lý thiết bị: Quản lý động")
	log.Infof("  - Tổng số lượng công cụ: %d", localToolCount+globalToolCount)
}

// GetAllManagersStatus 获取所有管理器的状态信息
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

// RestartManager 重启指定的管理器
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

// 为了向后兼容，提供便捷函数

// StartMCPManagers 启动所有MCP管理器（便捷函数）
func StartMCPManagers() error {
	return GetMCPManager().Start()
}

// StopMCPManagers 停止所有MCP管理器（便捷函数）
func StopMCPManagers() error {
	return GetMCPManager().Stop()
}

// GetMCPManagerStatus 获取MCP管理器状态（便捷函数）
func GetMCPManagerStatus() map[string]interface{} {
	return GetMCPManager().GetAllManagersStatus()
}
