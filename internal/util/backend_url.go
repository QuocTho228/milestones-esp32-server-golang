package util

import (
	"os"

	"github.com/spf13/viper"
)

// GetBackendURL lấy URL backend, ưu tiên lấy từ biến môi trường, nếu biến môi trường không tồn tại thì lấy từ cấu hình
func GetBackendURL() string {
	// Ưu tiên lấy từ biến môi trường
	if backendURL := os.Getenv("BACKEND_URL"); backendURL != "" {
		return backendURL
	}
	// Lấy từ file cấu hình
	return viper.GetString("manager.backend_url")
}