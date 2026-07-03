package user_config

import (
	"context"
	"fmt"
	log "milestones-esp32-server-golang/logger"

	"milestones-esp32-server-golang/internal/domain/config/manager"
	"milestones-esp32-server-golang/internal/domain/config/memory"
	redis_config "milestones-esp32-server-golang/internal/domain/config/redis"

	"github.com/spf13/viper"
)

var (
	// managerSystemConfigHandlers - danh sách callback khi nhận được thông báo đẩy WebSocket system_config, chương trình chính có thể đăng ký nhiều lần (ví dụ hợp nhất vào viper, dịch vụ cập nhật nóng)
	managerSystemConfigHandlers []func(map[string]interface{})
)

// RegisterManagerSystemConfigHandler - Đăng ký callback cho việc đẩy cấu hình hệ thống ở chế độ manager, nên được gọi trước InitConfigSystem; có thể gọi nhiều lần để thêm nhiều callback
func RegisterManagerSystemConfigHandler(fn func(map[string]interface{})) {
	managerSystemConfigHandlers = append(managerSystemConfigHandlers, fn)
}

// InitConfigSystem - Khởi tạo hệ thống cấu hình
// Gọi phương thức Init của package cấu hình tương ứng dựa trên giá trị config_provider.type
func InitConfigSystem(ctx context.Context) error {
	// Lấy loại trình cung cấp cấu hình
	providerType := viper.GetString("config_provider.type")
	if providerType == "" {
		providerType = "redis" // Mặc định sử dụng redis
		log.Infof("config_provider.type not set, using default: redis")
	}

	log.Infof("Initializing config system with provider: %s", providerType)

	// Gọi phương thức Init tương ứng dựa trên loại trình cung cấp cấu hình
	switch providerType {
	case "manager":
		manager.SetSystemConfigPushHandler(func(data map[string]interface{}) {
			for _, h := range managerSystemConfigHandlers {
				h(data)
			}
		})
		return manager.Init(ctx)
	case "redis":
		return redis_config.Init(ctx)
	case "memory":
		return memory.Init(ctx)
	default:
		return fmt.Errorf("unsupported config provider type: %s", providerType)
	}
}