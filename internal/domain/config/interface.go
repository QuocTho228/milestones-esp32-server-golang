package user_config

import (
	"context"
	"milestones-esp32-server-golang/internal/domain/config/types"
)

// UserConfigProvider - Giao diện trình cung cấp cấu hình người dùng
// Đây là một giao diện mở rộng, hỗ trợ nhiều thao tác hơn, khác với giao diện UserConfig ban đầu
type UserConfigProvider interface {
	// auth
	// Lấy thông tin kích hoạt dựa trên deviceId và clientId
	IsDeviceActivated(ctx context.Context, deviceId string, clientId string) (bool, error)
	GetActivationInfo(ctx context.Context, deviceId string, clientId string) (string, string, string, int)
	VerifyChallenge(ctx context.Context, deviceId string, clientId string, activationPayload types.ActivationPayload) (bool, error)

	// llm memory

	// GetUserConfig - Lấy cấu hình người dùng (tương thích với giao diện gốc)
	GetUserConfig(ctx context.Context, userID string) (types.UConfig, error)

	// SwitchDeviceRoleByName - Chuyển đổi vai trò thiết bị theo tên vai trò (hỗ trợ khớp mờ)
	SwitchDeviceRoleByName(ctx context.Context, deviceID string, roleName string) (string, error)

	// RestoreDeviceDefaultRole - Khôi phục vai trò mặc định của thiết bị (xóa vai trò đã gán cho thiết bị)
	RestoreDeviceDefaultRole(ctx context.Context, deviceID string) error

	// Lấy cấu hình mqtt, mqtt_server, udp, ota, vision
	GetSystemConfig(ctx context.Context) (string, error)

	// Đăng ký hàm xử lý sự kiện chiều lên (ví dụ thiết bị online/offline, v.v.)
	NotifyDeviceEvent(ctx context.Context, eventType string, eventData map[string]interface{})
	// Đăng ký hàm xử lý sự kiện chiều xuống (ví dụ inject tin nhắn, v.v.)
	RegisterMessageEventHandler(ctx context.Context, eventType string, eventHandler types.EventHandler)
}