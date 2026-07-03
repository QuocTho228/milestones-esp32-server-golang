package user_config

import (
	"fmt"

	"milestones-esp32-server-golang/internal/domain/config/manager"
	userconfig_redis "milestones-esp32-server-golang/internal/domain/config/redis"
	"milestones-esp32-server-golang/internal/util"
)

// Config - Cấu trúc cấu hình trình cung cấp cấu hình người dùng
type Config struct {
	Type       string                 `json:"type"`       // Loại lưu trữ: "redis", "memory", "file"
	Parameters map[string]interface{} `json:"parameters"` // Tham số cấu hình liên quan đến lưu trữ
}

func GetProvider(sType string) (UserConfigProvider, error) {
	config := make(map[string]interface{})
	if sType == "manager" {
		// Ưu tiên lấy địa chỉ backend từ biến môi trường, nếu biến môi trường không tồn tại thì lấy từ cấu hình
		backendUrl := util.GetBackendURL()
		config = map[string]interface{}{
			"backend_url": backendUrl,
			"auth_token":  util.GetManagerAuthToken(),
		}
	}

	provider, err := GetUserConfigProvider(sType, config)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// GetUserConfigProvider - Tạo trình cung cấp cấu hình người dùng
// Tạo instance trình cung cấp tương ứng dựa trên loại lưu trữ và tham số cấu hình truyền vào
// providerType: loại trình cung cấp, hỗ trợ "redis", "memory", "file"
// config: tham số cấu hình của trình cung cấp
// Trả về interface UserConfigProvider, hỗ trợ đầy đủ các thao tác CRUD
func GetUserConfigProvider(providerType string, config map[string]interface{}) (UserConfigProvider, error) {
	if config == nil {
		config = make(map[string]interface{})
	}

	switch providerType {
	case "redis":
		// Tạo trình cung cấp cấu hình người dùng Redis
		provider, err := userconfig_redis.NewRedisUserConfigProvider(config)
		if err != nil {
			return nil, fmt.Errorf("Tạo trình cung cấp cấu hình người dùng Redis thất bại: %v", err)
		}
		return provider, nil
	case "manager":
		// Tạo trình cung cấp cấu hình người dùng của hệ thống quản trị backend
		provider, err := manager.NewManagerUserConfigProvider(config)
		if err != nil {
			return nil, fmt.Errorf("Tạo trình cung cấp cấu hình người dùng của hệ thống quản trị backend thất bại: %v", err)
		}
		return provider, nil
	default:
		return nil, fmt.Errorf("Trình cung cấp cấu hình người dùng không được hỗ trợ: %s", providerType)
	}
}