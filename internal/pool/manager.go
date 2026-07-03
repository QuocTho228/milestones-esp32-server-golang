package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"milestones-esp32-server-golang/internal/domain/asr"
	"milestones-esp32-server-golang/internal/domain/llm"
	"milestones-esp32-server-golang/internal/domain/tts"
	"milestones-esp32-server-golang/internal/domain/vad"
	vad_inter "milestones-esp32-server-golang/internal/domain/vad/inter"
	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"

	"github.com/mitchellh/hashstructure/v2"
	"github.com/spf13/viper"
)

var (
	globalManager *UniversalResourcePoolManager
	once          sync.Once
)

// UniversalResourcePoolManager Người quản lý nhóm tài nguyên public
type UniversalResourcePoolManager struct {
	pools        map[string]*util.ResourcePool // Định nghĩa key: "resourceType:provider"
	creators     map[string]interface{}        // Đã đăng ký chức năng tạo
	closeFuncs   map[string]func(interface{}) error
	isValidFuncs map[string]func(interface{}) bool
	resetFuncs   map[string]func(interface{}) error
	mu           sync.RWMutex
}

// GetGlobalResourcePoolManager Nhận trình quản lý nhóm tài nguyên toàn cục (singleton)
func GetGlobalResourcePoolManager() *UniversalResourcePoolManager {
	once.Do(func() {
		globalManager = &UniversalResourcePoolManager{
			pools:        make(map[string]*util.ResourcePool),
			creators:     make(map[string]interface{}),
			closeFuncs:   make(map[string]func(interface{}) error),
			isValidFuncs: make(map[string]func(interface{}) bool),
			resetFuncs:   make(map[string]func(interface{}) error),
		}
		log.Info("Đã khởi tạo trình quản lý nhóm tài nguyên chung")
	})
	return globalManager
}

// ResourceTypeOption Tùy chọn đăng ký loại tài nguyên
type ResourceTypeOption func(*ResourceTypeConfig)

// ResourceTypeConfig Cấu hình loại tài nguyên
type ResourceTypeConfig struct {
	CloseFunc   func(interface{}) error
	IsValidFunc func(interface{}) bool
	ResetFunc   func(interface{}) error
}

// WithCloseFunc Đặt chức năng tắt máy
func WithCloseFunc(fn func(interface{}) error) ResourceTypeOption {
	return func(c *ResourceTypeConfig) {
		c.CloseFunc = fn
	}
}

// WithIsValidFunc Đặt hàm xác thực
func WithIsValidFunc(fn func(interface{}) bool) ResourceTypeOption {
	return func(c *ResourceTypeConfig) {
		c.IsValidFunc = fn
	}
}

// WithResetFunc Đặt chức năng đặt lại
func WithResetFunc(fn func(interface{}) error) ResourceTypeOption {
	return func(c *ResourceTypeConfig) {
		c.ResetFunc = fn
	}
}

// RegisterResourceType Đăng ký loại tài nguyên (cuộc gọi bên ngoài)
// resourceType: Tên loại tài nguyên (chẳng hạn như "vad", "asr", "custom_type", v.v.)
// creator: Hàm tạo tài nguyên
// opts: Cấu hình tùy chọn (closeFunc, isValidFunc, resetFunc)
func RegisterResourceType[T any](
	resourceType string,
	creator CreatorFunc[T],
	opts ...ResourceTypeOption,
) error {
	mgr := GetGlobalResourcePoolManager()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// Kiểm tra xem loại tài nguyên đã được đăng ký chưa
	if _, exists := mgr.creators[resourceType]; exists {
		return fmt.Errorf("Loại tài nguyên %s đã được đăng ký", resourceType)
	}

	// đăng ký creator
	mgr.creators[resourceType] = creator

	// Tùy chọn ứng dụng
	config := &ResourceTypeConfig{}
	for _, opt := range opts {
		opt(config)
	}

	if config.CloseFunc != nil {
		mgr.closeFuncs[resourceType] = config.CloseFunc
	}
	if config.IsValidFunc != nil {
		mgr.isValidFuncs[resourceType] = config.IsValidFunc
	}
	if config.ResetFunc != nil {
		mgr.resetFuncs[resourceType] = config.ResetFunc
	}

	log.Infof("Đã đăng ký loại tài nguyên: %s", resourceType)
	return nil
}

// GenerateConfigKey 生成配置键（用于区分不同配置的资源池）
// 使用 hashstructure 做与 map key 顺序无关的指纹，同一语义配置得到相同 key，避免重复建池。
func GenerateConfigKey(provider string, config map[string]interface{}) string {
	input := map[string]interface{}{"provider": provider, "config": config}
	h, err := hashstructure.Hash(input, hashstructure.FormatV2, nil)
	if err != nil {
		log.Warnf("Không thể định cấu hình tính toán dấu vân tay, sử dụng nhà cung cấp làm khóa: %v", err)
		return provider
	}
	return fmt.Sprintf("%016x", h)
}

// getOrCreatePool 获取或创建资源池（泛型版本）
// 使用配置指纹作为 poolKey，同一 config_id 在 host 等配置变更后会使用新池、新配置实例。
func getOrCreatePool[T any](
	resourceType, provider string,
	config map[string]interface{},
) (*util.ResourcePool, error) {
	mgr := GetGlobalResourcePoolManager()
	// 资源池 key 格式统一为：类型:配置指纹（provider+config 的 MD5）
	configKey := GenerateConfigKey(provider, config)
	poolKey := fmt.Sprintf("%s:%s", resourceType, configKey)

	mgr.mu.RLock()
	pool, exists := mgr.pools[poolKey]
	mgr.mu.RUnlock()

	if exists {
		return pool, nil
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// 双重检查
	if pool, exists := mgr.pools[poolKey]; exists {
		return pool, nil
	}

	// 获取已注册的 creator
	creatorInterface, exists := mgr.creators[resourceType]
	if !exists {
		return nil, fmt.Errorf("Loại tài nguyên chưa được đăng ký: %s (vui lòng gọi RegisterResourceType để đăng ký trước)", resourceType)
	}

	// 类型断言获取泛型 creator
	creator, ok := creatorInterface.(CreatorFunc[T])
	if !ok {
		return nil, fmt.Errorf("Loại người tạo không khớp với loại tài nguyên %s", resourceType)
	}

	// 创建泛型资源工厂
	factory := &ResourceFactory[T]{
		resourceType: resourceType,
		provider:     provider,
		config:       config,
		configKey:    configKey,
		creator:      creator,
		closeFunc: func(p T) error {
			if closeFunc := mgr.closeFuncs[resourceType]; closeFunc != nil {
				return closeFunc(any(p))
			}
			return nil
		},
		isValidFunc: func(p T) bool {
			if isValidFunc := mgr.isValidFuncs[resourceType]; isValidFunc != nil {
				return isValidFunc(any(p))
			}
			return true
		},
		resetFunc: func(p T) error {
			if resetFunc := mgr.resetFuncs[resourceType]; resetFunc != nil {
				return resetFunc(any(p))
			}
			return nil
		},
	}

	// 获取资源池配置（所有资源类型共享默认配置）
	poolConfig := getPoolConfig()

	// 创建资源池
	pool, err := util.NewResourcePool(poolConfig, factory)
	if err != nil {
		return nil, fmt.Errorf("Không tạo được nhóm tài nguyên [%s:%s]: %w", resourceType, configKey, err)
	}

	mgr.pools[poolKey] = pool
	fpShort := configKey
	if len(configKey) > 8 {
		fpShort = configKey[:8] + "..."
	}
	log.Infof("Tạo nhóm tài nguyên: type=%s, provider=%s, fingerprint=%s", resourceType, provider, fpShort)

	return pool, nil
}

// Acquire 获取资源（泛型版本，类型安全，支持懒加载）
// T: 资源类型
// resourceType: 资源类型字符串（vad/asr/llm/tts等）
// provider: 提供者名称
// config: 配置信息
func Acquire[T any](
	resourceType, provider string,
	config map[string]interface{},
) (*ResourceWrapper[T], error) {
	pool, err := getOrCreatePool[T](resourceType, provider, config)
	if err != nil {
		return nil, err
	}

	resource, err := pool.Acquire()
	if err != nil {
		return nil, fmt.Errorf("Không lấy được tài nguyên [%s:%s]: %w", resourceType, provider, err)
	}

	wrapper, ok := resource.(*ResourceWrapper[T])
	if !ok {
		pool.Release(resource)
		return nil, fmt.Errorf("Lỗi loại tài nguyên: dự kiến ResourceWrapper[%T]", *new(T))
	}

	return wrapper, nil
}

// Release 归还资源（泛型版本，类型安全）
func Release[T any](wrapper *ResourceWrapper[T]) error {
	if wrapper == nil {
		return nil
	}

	mgr := GetGlobalResourcePoolManager()
	// 所有资源池的 key 格式统一为：类型:provider
	poolKey := fmt.Sprintf("%s:%s", wrapper.resourceType, wrapper.configKey)

	mgr.mu.RLock()
	pool, exists := mgr.pools[poolKey]
	mgr.mu.RUnlock()

	if !exists {
		log.Warnf("Nhóm tài nguyên không tồn tại: %s", poolKey)
		return nil
	}

	return pool.Release(wrapper)
}

// GetStats 获取所有资源池的统计信息
func GetStats() map[string]interface{} {
	mgr := GetGlobalResourcePoolManager()
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	stats := make(map[string]interface{})

	for poolKey, pool := range mgr.pools {
		stats[poolKey] = pool.Stats()
	}

	return stats
}

// StartStatsMonitor 启动资源池统计监控，每 interval 输出一次统计信息到日志
func StartStatsMonitor(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Debugf("Nhóm tài nguyên thống kê đã dừng")
				return
			case <-ticker.C:
				stats := GetStats()
				if len(stats) > 0 {
					statsJSON, err := json.MarshalIndent(stats, "", "  ")
					if err != nil {
						log.Errorf("Không thể tuần tự hóa số liệu thống kê nhóm tài nguyên: %v", err)
						continue
					}
					log.Infof("========== Thống kê nhóm tài nguyên toàn cục ==========")
					log.Infof("Thời gian thống kê: %s", time.Now().Format("2006-01-02 15:04:05"))
					log.Infof("Số lượng nguồn tài nguyên: %d", len(stats))
					log.Infof("Chi tiết:\n%s", string(statsJSON))
					log.Infof("========================================")
				} else {
					log.Infof("========== Thống kê nhóm tài nguyên toàn cục ==========")
					log.Infof("Thời gian thống kê: %s", time.Now().Format("2006-01-02 15:04:05"))
					log.Infof("Hiện tại không có nhóm tài nguyên hoạt động")
					log.Infof("========================================")
				}
			}
		}
	}()
	log.Infof("Giám sát thống kê nhóm tài nguyên đã được bắt đầu và số liệu thống kê được xuất ra nhật ký mỗi %v", interval)
}

// Close 关闭所有资源池
func Close() error {
	mgr := GetGlobalResourcePoolManager()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	var errs []error

	for poolKey, pool := range mgr.pools {
		if err := pool.Close(); err != nil {
			errs = append(errs, fmt.Errorf("Đóng nhóm tài nguyên %s thất bại: %w", poolKey, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("Đã xảy ra lỗi khi đóng nhóm tài nguyên: %v", errs)
	}

	return nil
}

// getPoolConfig 从配置中获取资源池配置（所有资源类型共享默认配置）
func getPoolConfig() *util.PoolConfig {
	// 使用默认配置
	config := util.DefaultConfig()

	// 如果配置了 resource_pools，则覆盖默认值
	if viper.IsSet("resource_pools.max_size") {
		config.MaxSize = viper.GetInt("resource_pools.max_size")
	}
	if viper.IsSet("resource_pools.min_size") {
		config.MinSize = viper.GetInt("resource_pools.min_size")
	}
	if viper.IsSet("resource_pools.max_idle") {
		config.MaxIdle = viper.GetInt("resource_pools.max_idle")
	}
	if viper.IsSet("resource_pools.acquire_timeout") {
		config.AcquireTimeout = viper.GetDuration("resource_pools.acquire_timeout")
	}
	if viper.IsSet("resource_pools.idle_timeout") {
		config.IdleTimeout = viper.GetDuration("resource_pools.idle_timeout")
	}
	if viper.IsSet("resource_pools.validate_on_borrow") {
		config.ValidateOnBorrow = viper.GetBool("resource_pools.validate_on_borrow")
	}
	if viper.IsSet("resource_pools.validate_on_return") {
		config.ValidateOnReturn = viper.GetBool("resource_pools.validate_on_return")
	}

	return config
}

// init 初始化内置资源类型
func init() {
	// 注册 VAD 资源类型
	RegisterResourceType[vad_inter.VAD](
		"vad",
		func(rt, p string, cfg map[string]interface{}) (vad_inter.VAD, error) {
			vadProvider, err := vad.AcquireVAD(p, cfg)
			if err != nil {
				return nil, err
			}
			if vadProvider != nil {
				vadProvider.Reset()
			}
			return vadProvider, nil
		},
		WithCloseFunc(func(p interface{}) error {
			if vadProvider, ok := p.(vad_inter.VAD); ok && vadProvider != nil {
				return vadProvider.Close()
			}
			return nil
		}),
		WithIsValidFunc(func(p interface{}) bool {
			if vadProvider, ok := p.(vad_inter.VAD); ok && vadProvider != nil {
				return vadProvider.IsValid()
			}
			return false
		}),
		WithResetFunc(func(p interface{}) error {
			if vadProvider, ok := p.(vad_inter.VAD); ok && vadProvider != nil {
				return vadProvider.Reset()
			}
			return nil
		}),
	)

	// 注册 ASR 资源类型
	RegisterResourceType[asr.AsrProvider](
		"asr",
		func(rt, p string, cfg map[string]interface{}) (asr.AsrProvider, error) {
			return asr.NewAsrProvider(p, cfg)
		},
		WithIsValidFunc(func(p interface{}) bool {
			if asrProvider, ok := p.(asr.AsrProvider); ok && asrProvider != nil {
				return asrProvider.IsValid()
			}
			return false
		}),
		WithCloseFunc(func(p interface{}) error {
			if asrProvider, ok := p.(asr.AsrProvider); ok && asrProvider != nil {
				return asrProvider.Close()
			}
			return nil
		}),
	)

	// 注册 LLM 资源类型
	RegisterResourceType[llm.LLMProvider](
		"llm",
		func(rt, p string, cfg map[string]interface{}) (llm.LLMProvider, error) {
			providerName, ok := cfg["provider"].(string)
			if !ok || providerName == "" {
				providerName = p
			}
			return llm.GetLLMProvider(providerName, cfg)
		},
		WithIsValidFunc(func(p interface{}) bool {
			if llmProvider, ok := p.(llm.LLMProvider); ok && llmProvider != nil {
				return llmProvider.IsValid()
			}
			return false
		}),
		WithCloseFunc(func(p interface{}) error {
			if llmProvider, ok := p.(llm.LLMProvider); ok && llmProvider != nil {
				return llmProvider.Close()
			}
			return nil
		}),
	)

	// 注册 TTS 资源类型
	RegisterResourceType[tts.TTSProvider](
		"tts",
		func(rt, p string, cfg map[string]interface{}) (tts.TTSProvider, error) {
			return tts.GetTTSProvider(p, cfg)
		},
		WithIsValidFunc(func(p interface{}) bool {
			if ttsProvider, ok := p.(tts.TTSProvider); ok && ttsProvider != nil {
				return ttsProvider.IsValid()
			}
			return false
		}),
		WithCloseFunc(func(p interface{}) error {
			if ttsProvider, ok := p.(tts.TTSProvider); ok && ttsProvider != nil {
				return ttsProvider.Close()
			}
			return nil
		}),
	)
}
