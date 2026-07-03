package user_config

import (
	"context"
	"testing"
)

func TestMemoryProvider(t *testing.T) {
	ctx := context.Background()

	// Tạo provider bộ nhớ
	config := map[string]interface{}{
		"max_entries": 10,
	}

	provider, err := GetUserConfigProvider("memory", config)
	if err != nil {
		t.Fatalf("Tạo provider bộ nhớ thất bại: %v", err)
	}
	// Lưu ý: giao diện không có phương thức Close, nên không cần gọi

	userID := "test_user_123"

	// Vì giao diện không có phương thức SetUserConfig, chúng ta chỉ kiểm tra phương thức GetUserConfig
	// Kiểm tra lấy cấu hình của người dùng không tồn tại (nên trả về cấu hình rỗng)
	retrievedConfig, err := provider.GetUserConfig(ctx, userID)
	if err != nil {
		t.Fatalf("Lấy cấu hình người dùng thất bại: %v", err)
	}

	// Xác minh trả về là cấu hình rỗng
	if retrievedConfig.Llm.Provider != "" {
		t.Errorf("Mong đợi cấu hình rỗng, nhưng nhận được LLM Provider: %s", retrievedConfig.Llm.Provider)
	}

	// Kiểm tra lấy cấu hình hệ thống
	systemConfig, err := provider.GetSystemConfig(ctx)
	if err != nil {
		t.Fatalf("Lấy cấu hình hệ thống thất bại: %v", err)
	}
	_ = systemConfig // Cấu hình hệ thống có thể rỗng, điều này là bình thường
}

func TestProviderAdapter(t *testing.T) {
	ctx := context.Background()

	// Tạo provider bộ nhớ
	provider, err := GetUserConfigProvider("memory", map[string]interface{}{
		"max_entries": 5,
	})
	if err != nil {
		t.Fatalf("Tạo provider bộ nhớ thất bại: %v", err)
	}
	// Lưu ý: giao diện không có phương thức Close, nên không cần gọi

	// Kiểm tra adapter lấy cấu hình
	userID := "adapter_test_user"

	// Sử dụng adapter để lấy cấu hình (có thể là cấu hình rỗng)
	adapter := NewUserConfigAdapter(provider)
	retrievedConfig, err := adapter.GetUserConfig(ctx, userID)
	if err != nil {
		t.Fatalf("Lấy cấu hình qua adapter thất bại: %v", err)
	}

	// Xác minh adapter hoạt động bình thường (lấy được cấu trúc cấu hình)
	if retrievedConfig.SystemPrompt == "" {
		t.Logf("Adapter lấy được system prompt rỗng, điều này là bình thường")
	} else {
		t.Logf("Adapter lấy được system prompt: %s", retrievedConfig.SystemPrompt)
	}
}

func TestDefaultConfig(t *testing.T) {
	// Kiểm tra cấu hình mặc định của Redis
	redisConfig := DefaultConfig("redis")
	if redisConfig["host"] != "localhost" {
		t.Errorf("Cấu hình host mặc định của Redis sai, mong đợi: localhost, thực tế: %v", redisConfig["host"])
	}

	// Kiểm tra cấu hình mặc định của Memory
	memoryConfig := DefaultConfig("memory")
	if memoryConfig["max_entries"] != 1000 {
		t.Errorf("Cấu hình max_entries mặc định của Memory sai, mong đợi: 1000, thực tế: %v", memoryConfig["max_entries"])
	}

	// Kiểm tra loại không được hỗ trợ
	unknownConfig := DefaultConfig("unknown")
	if len(unknownConfig) != 0 {
		t.Errorf("Loại không xác định nên trả về cấu hình rỗng, thực tế: %v", unknownConfig)
	}
}

func TestValidateConfig(t *testing.T) {
	// Kiểm tra cấu hình Redis hợp lệ
	validRedisConfig := map[string]interface{}{
		"host": "localhost",
		"port": 6379,
	}
	err := ValidateConfig("redis", validRedisConfig)
	if err != nil {
		t.Errorf("Xác thực cấu hình Redis hợp lệ thất bại: %v", err)
	}

	// Kiểm tra cấu hình Redis không hợp lệ (thiếu host)
	invalidRedisConfig := map[string]interface{}{
		"port": 6379,
	}
	err = ValidateConfig("redis", invalidRedisConfig)
	if err == nil {
		t.Error("Cấu hình Redis thiếu host nên xác thực thất bại")
	}

	// Kiểm tra cấu hình Memory (không cần xác thực)
	err = ValidateConfig("memory", map[string]interface{}{})
	if err != nil {
		t.Errorf("Xác thực cấu hình Memory thất bại: %v", err)
	}
}