package redis_config

import (
	"context"
	"encoding/json"
	"fmt"

	log "milestones-esp32-server-golang/logger"

	i_redis "milestones-esp32-server-golang/internal/db/redis"
	"milestones-esp32-server-golang/internal/domain/config/types"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

// RedisUserConfigProvider - Trình cung cấp cấu hình người dùng Redis
// Triển khai giao diện UserConfigProvider, hỗ trợ lưu trữ Redis
type RedisUserConfigProvider struct {
	UserConfig
}

type UserConfig struct {
	redisInstance *redis.Client
	prefix        string
}

// NewRedisUserConfigProvider - Tạo trình cung cấp cấu hình người dùng Redis
// config: map tham số cấu hình, bao gồm host, port, password, db, prefix, v.v.
func NewRedisUserConfigProvider(config interface{}) (*RedisUserConfigProvider, error) {
	provider := &RedisUserConfigProvider{
		UserConfig: UserConfig{
			redisInstance: i_redis.GetClient(),
			prefix:        viper.GetString("redis.key_prefix"),
		},
	}

	log.Log().Info("Khởi tạo trình cung cấp cấu hình người dùng Redis thành công")
	return provider, nil
}

func (u *UserConfig) GetUserConfig(ctx context.Context, userID string) (types.UConfig, error) {
	redisConfig := map[string]string{}

	if u.redisInstance != nil {
		key := u.GetUserConfigKey(userID)

		// hgetall lấy tất cả
		var err error
		redisConfig, err = u.redisInstance.HGetAll(ctx, key).Result()
		if err != nil {
			return types.UConfig{}, err
		}
	}

	ret := types.UConfig{
		SystemPrompt:    u.getSystemPrompt(ctx, userID),
		MemoryMode:      "short",
		SpeakerChatMode: "off",
	}
	// Chuyển đổi UserConfig thành cấu trúc UConfig
	kv := map[string]string{
		"llm":    "",
		"asr":    "",
		"tts":    "",
		"memory": "",
	}
	for k, _ := range kv {
		var redisPerConfig map[string]interface{}
		if rv, ok := redisConfig[k]; ok {
			if rv != "" {
				err := json.Unmarshal([]byte(rv), &redisPerConfig)
				if err != nil {
					log.Log().Errorf("redis config unmarshal error: %+v", err)
				}
			}
		}

		if k == "llm" {
			config, err := u.getLlmConfig(ctx, redisPerConfig)
			if err != nil {
				return types.UConfig{}, err
			}
			ret.Llm = config
		} else if k == "tts" {
			config, err := u.getTtsConfig(ctx, redisPerConfig)
			if err != nil {
				return types.UConfig{}, err
			}
			ret.Tts = config
		} else if k == "asr" {
			config, err := u.getAsrConfig(ctx, redisPerConfig)
			if err != nil {
				return types.UConfig{}, err
			}
			ret.Asr = config
		} else if k == "memory" {
			config, err := u.getMemoryConfig(ctx, redisPerConfig)
			if err != nil {
				return types.UConfig{}, err
			}
			ret.Memory = config
		}
	}
	ret.Vad = u.getVadConfig(ctx)

	log.Log().Infof("userconfig: %+v", ret)
	return ret, nil
}

func (u *UserConfig) getVadConfig(ctx context.Context) types.VadConfig {
	provider := viper.GetString("vad.provider")
	return types.VadConfig{
		Provider: provider,
		Config:   viper.GetStringMap(fmt.Sprintf("vad.%s", provider)),
	}
}

func (u *UserConfig) getConfigByType(ctx context.Context, config map[string]interface{}, prefix string) (string, map[string]interface{}, error) {
	provider := viper.GetString(prefix + ".provider")
	if _, ok := config[provider]; !ok {
		if iProvider, ok := config["provider"].(string); ok {
			provider = iProvider
		}
	}

	commonConfig := viper.GetStringMap(prefix + "." + provider)

	for k, v := range config {
		if k == "provider" {
			continue
		}
		commonConfig[k] = v
	}
	return provider, commonConfig, nil
}

func (u *UserConfig) getLlmConfig(ctx context.Context, config map[string]interface{}) (types.LlmConfig, error) {
	provider, commonConfig, err := u.getConfigByType(ctx, config, "llm")
	if err != nil {
		return types.LlmConfig{}, err
	}
	return types.LlmConfig{
		Provider: provider,
		Config:   commonConfig,
	}, nil
}
func (u *UserConfig) getAsrConfig(ctx context.Context, config map[string]interface{}) (types.AsrConfig, error) {
	provider, commonConfig, err := u.getConfigByType(ctx, config, "asr")
	if err != nil {
		return types.AsrConfig{}, err
	}
	return types.AsrConfig{
		Provider: provider,
		Config:   commonConfig,
	}, nil
}
func (u *UserConfig) getTtsConfig(ctx context.Context, config map[string]interface{}) (types.TtsConfig, error) {
	provider, commonConfig, err := u.getConfigByType(ctx, config, "tts")
	if err != nil {
		return types.TtsConfig{}, err
	}
	return types.TtsConfig{
		Provider: provider,
		Config:   commonConfig,
	}, nil
}

func (u *UserConfig) getMemoryConfig(ctx context.Context, config map[string]interface{}) (types.MemoryConfig, error) {
	provider, commonConfig, err := u.getConfigByType(ctx, config, "memory")
	if err != nil {
		return types.MemoryConfig{}, err
	}
	return types.MemoryConfig{
		Provider: provider,
		Config:   commonConfig,
	}, nil
}

func (u *UserConfig) GetUserConfigKey(deviceId string) string {
	return fmt.Sprintf("%s:userconfig:%s", u.prefix, deviceId)
}

// getSystemPromptKey - Tạo Redis key của system prompt tương ứng với thiết bị
func (u *UserConfig) getSystemPrompt(ctx context.Context, deviceID string) string {
	key := fmt.Sprintf("%s:llm:system:%s", u.prefix, deviceID)

	configPrompt := viper.GetString("system_prompt")

	if u.redisInstance != nil {
		systemPrompt, err := u.redisInstance.Get(ctx, key).Result()
		if err != nil {
			return configPrompt
		}
		return systemPrompt
	}
	return configPrompt
}

// Lấy cấu hình mqtt, mqtt_server, udp, ota, vision
func (u *UserConfig) GetSystemConfig(ctx context.Context) (string, error) {
	// Mặc định không ghi đè
	return "", nil
}

// SwitchDeviceRoleByName - Chế độ Redis không hỗ trợ chuyển đổi vai trò thiết bị
func (u *UserConfig) SwitchDeviceRoleByName(ctx context.Context, deviceID string, roleName string) (string, error) {
	return "", fmt.Errorf("Trình cung cấp cấu hình redis không hỗ trợ chuyển đổi vai trò thiết bị theo tên vai trò")
}

// RestoreDeviceDefaultRole - Chế độ Redis không hỗ trợ khôi phục vai trò mặc định
func (u *UserConfig) RestoreDeviceDefaultRole(ctx context.Context, deviceID string) error {
	return fmt.Errorf("Trình cung cấp cấu hình redis không hỗ trợ khôi phục vai trò mặc định của thiết bị")
}

func (u *UserConfig) NotifyDeviceEvent(ctx context.Context, eventType string, eventData map[string]interface{}) {
	// Triển khai logic thông báo sự kiện thiết bị
	return
}

func (u *UserConfig) RegisterMessageEventHandler(ctx context.Context, deviceID string, handler types.EventHandler) {
	// Triển khai logic xử lý sự kiện tin nhắn
	return
}

// Init - Khởi tạo trình cung cấp cấu hình Redis
func Init(ctx context.Context) error {
	log.Log().Info("Redis config provider initialized successfully")
	return nil
}

// Close - Đóng trình cung cấp cấu hình Redis, dọn dẹp tài nguyên
func Close() error {
	log.Log().Info("Redis config provider closed")
	return nil
}

// IsConnected - Kiểm tra trình cung cấp cấu hình Redis đã kết nối hay chưa
func IsConnected() bool {
	// Trạng thái kết nối Redis được quản lý bởi Redis client toàn cục
	return true
}