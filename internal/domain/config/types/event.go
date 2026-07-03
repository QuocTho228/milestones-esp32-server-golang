package types

import "context"

type EventHandler func(ctx context.Context, eventType string, eventData map[string]interface{}) (string, error)

// Sự kiện Push hướng lên: Chương trình chính → Hệ thống quản lý nội bộ
const (
	EventDeviceOnline  = "/api/device/active"   //Thiết bị trực tuyến
	EventDeviceOffline = "/api/device/inactive" //Thiết bị ngoại tuyến
)

// Sự kiện Pull hướng xuống: Hệ thống quản lý nội bộ => Chương trình chính
const (
	EventHandleMessageInject = "/api/device/inject_msg" // Xử lý việc chèn thông báo
)
