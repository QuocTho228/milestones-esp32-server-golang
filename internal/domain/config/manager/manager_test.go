package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestConfigManager_GetSystemConfig(t *testing.T) {
	// Tạo trình quản lý cấu hình
	config := map[string]interface{}{
		"backend_url": "http://192.168.1.97:8080", // điều chỉnh theo địa chỉ backend thực tế
	}

	manager, err := NewManagerUserConfigProvider(config)
	if err != nil {
		t.Fatalf("Tạo trình quản lý cấu hình thất bại: %v", err)
	}

	// Tạo context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Lấy cấu hình hệ thống
	configJSON, err := manager.GetSystemConfig(ctx)
	if err != nil {
		t.Fatalf("Lấy cấu hình hệ thống thất bại: %v", err)
	}

	// Xác thực định dạng JSON trả về
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		t.Fatalf("Phân tích JSON cấu hình thất bại: %v", err)
	}

	// Kiểm tra có chứa các mục cấu hình mong đợi hay không
	expectedKeys := []string{"mqtt", "mqtt_server", "udp", "ota"}
	for _, key := range expectedKeys {
		if _, exists := configMap[key]; !exists {
			t.Errorf("Cấu hình thiếu key mong đợi: %s", key)
		}
	}

	fmt.Printf("Cấu hình hệ thống đã lấy được: %s\n", configJSON)
	t.Logf("Kích thước cấu hình: %d byte", len(configJSON))
}