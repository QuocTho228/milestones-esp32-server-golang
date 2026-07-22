package util

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Resource giao diện tài nguyên, tất cả tài nguyên được quản lý bởi pool đều phải triển khai giao diện này
type Resource interface {
	// Close đóng tài nguyên
	Close() error
	// IsValid kiểm tra tài nguyên có còn hợp lệ hay không
	IsValid() bool
}

// ResourceFactory giao diện factory tạo tài nguyên, dùng để tạo và xác thực tài nguyên
type ResourceFactory interface {
	// Create tạo một instance tài nguyên mới
	Create() (Resource, error)
	// Validate xác thực tài nguyên có hợp lệ hay không (tùy chọn, nếu trả về false thì tài nguyên sẽ bị hủy)
	Validate(resource Resource) bool
	// Reset đặt lại trạng thái tài nguyên (tùy chọn, dùng để dọn dẹp trước khi tái sử dụng tài nguyên)
	Reset(resource Resource) error
}

// PoolConfig cấu hình resource pool
type PoolConfig struct {
	// MaxSize số lượng tài nguyên tối đa
	MaxSize int
	// MinSize số lượng tài nguyên tối thiểu (được tạo trước)
	MinSize int
	// MaxIdle số lượng tài nguyên rảnh tối đa
	MaxIdle int
	// AcquireTimeout thời gian chờ tối đa khi lấy tài nguyên
	AcquireTimeout time.Duration
	// IdleTimeout thời gian tài nguyên được phép ở trạng thái rảnh trước khi bị hủy
	IdleTimeout time.Duration
	// ValidateOnBorrow có xác thực tài nguyên khi lấy ra hay không
	ValidateOnBorrow bool
	// ValidateOnReturn có xác thực tài nguyên khi trả lại hay không
	ValidateOnReturn bool
}

// DefaultConfig trả về cấu hình mặc định
func DefaultConfig() *PoolConfig {
	return &PoolConfig{
		MaxSize:          1000,
		MinSize:          1,
		MaxIdle:          5,
		AcquireTimeout:   30 * time.Second,
		IdleTimeout:      5 * time.Minute,
		ValidateOnBorrow: true,
		ValidateOnReturn: false,
	}
}

// pooledResource lớp bao bọc tài nguyên trong pool
type pooledResource struct {
	resource   Resource
	createTime time.Time
	lastUsed   time.Time
	inUse      bool
}

// ResourcePool resource pool dùng chung
type ResourcePool struct {
	config  *PoolConfig
	factory ResourceFactory

	// available hàng đợi tài nguyên khả dụng
	available chan *pooledResource
	// resources bản đồ tất cả tài nguyên (bao gồm cả đang dùng và khả dụng)
	resources map[Resource]*pooledResource
	// mu khóa đọc/ghi
	mu sync.RWMutex
	// closed cờ đánh dấu đã đóng
	closed bool
	// ctx context để hủy
	ctx    context.Context
	cancel context.CancelFunc
	// cleanupWg WaitGroup cho goroutine dọn dẹp
	cleanupWg sync.WaitGroup
}

// NewResourcePool tạo resource pool mới
func NewResourcePool(config *PoolConfig, factory ResourceFactory) (*ResourcePool, error) {
	if config == nil {
		config = DefaultConfig()
	}
	if factory == nil {
		return nil, errors.New("factory cannot be nil")
	}
	if config.MaxSize <= 0 {
		return nil, errors.New("max size must be positive")
	}
	if config.MinSize < 0 {
		return nil, errors.New("min size cannot be negative")
	}
	if config.MinSize > config.MaxSize {
		return nil, errors.New("min size cannot be greater than max size")
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &ResourcePool{
		config:    config,
		factory:   factory,
		available: make(chan *pooledResource, config.MaxSize),
		resources: make(map[Resource]*pooledResource),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Tạo trước số lượng tài nguyên tối thiểu
	if err := pool.preCreateResources(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to pre-create resources: %w", err)
	}

	// Khởi động goroutine dọn dẹp
	pool.startCleanupRoutine()

	return pool, nil
}

// preCreateResources tạo trước tài nguyên
func (p *ResourcePool) preCreateResources() error {
	for i := 0; i < p.config.MinSize; i++ {
		resource, err := p.factory.Create()
		if err != nil {
			return fmt.Errorf("failed to create resource %d: %w", i, err)
		}

		pooled := &pooledResource{
			resource:   resource,
			createTime: time.Now(),
			lastUsed:   time.Now(),
			inUse:      false,
		}

		p.resources[resource] = pooled
		p.available <- pooled
	}
	return nil
}

// Acquire lấy tài nguyên
func (p *ResourcePool) Acquire() (Resource, error) {
	return p.AcquireWithTimeout(p.config.AcquireTimeout)
}

// AcquireWithTimeout lấy tài nguyên trong khoảng thời gian chờ chỉ định
func (p *ResourcePool) AcquireWithTimeout(timeout time.Duration) (Resource, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, errors.New("pool is closed")
	}
	p.mu.RUnlock()

	ctx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("acquire timeout after %v", timeout)
		case pooled := <-p.available:
			// Xác thực tính hợp lệ của tài nguyên
			if p.config.ValidateOnBorrow && pooled.resource != nil {
				if !pooled.resource.IsValid() || !p.factory.Validate(pooled.resource) {
					// Tài nguyên không hợp lệ, hủy và thử tạo tài nguyên mới
					p.destroyResource(pooled)
					if newResource, err := p.tryCreateResource(); err == nil {
						return newResource, nil
					}
					continue
				}
			}

			// Đặt lại trạng thái tài nguyên
			if err := p.factory.Reset(pooled.resource); err != nil {
				p.destroyResource(pooled)
				continue
			}

			// Đánh dấu là đang sử dụng
			p.mu.Lock()
			pooled.inUse = true
			pooled.lastUsed = time.Now()
			p.mu.Unlock()

			return pooled.resource, nil
		default:
			// Không có tài nguyên khả dụng, thử tạo mới
			if resource, err := p.tryCreateResource(); err == nil {
				return resource, nil
			}
			// Tạo thất bại, chờ tài nguyên được giải phóng
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// tryCreateResource thử tạo tài nguyên mới
func (p *ResourcePool) tryCreateResource() (Resource, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.resources) >= p.config.MaxSize {
		return nil, errors.New("pool is full")
	}

	resource, err := p.factory.Create()
	if err != nil {
		return nil, err
	}

	pooled := &pooledResource{
		resource:   resource,
		createTime: time.Now(),
		lastUsed:   time.Now(),
		inUse:      true,
	}

	p.resources[resource] = pooled
	return resource, nil
}

// Release giải phóng tài nguyên trả về pool
func (p *ResourcePool) Release(resource Resource) error {
	if resource == nil {
		return errors.New("resource cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return errors.New("pool is closed")
	}

	pooled, exists := p.resources[resource]
	if !exists {
		return errors.New("resource not managed by this pool")
	}

	if !pooled.inUse {
		return errors.New("resource is not in use")
	}

	// Xác thực tính hợp lệ của tài nguyên
	if p.config.ValidateOnReturn {
		if !resource.IsValid() || !p.factory.Validate(resource) {
			p.destroyResourceUnsafe(pooled)
			return nil
		}
	}

	// Kiểm tra xem có vượt quá số lượng rảnh tối đa không
	if len(p.available) >= p.config.MaxIdle {
		p.destroyResourceUnsafe(pooled)
		return nil
	}

	// Đánh dấu là khả dụng
	pooled.inUse = false
	pooled.lastUsed = time.Now()

	// Thử đưa trở lại hàng đợi khả dụng
	select {
	case p.available <- pooled:
		return nil
	default:
		// Hàng đợi đã đầy, hủy tài nguyên
		p.destroyResourceUnsafe(pooled)
		return nil
	}
}

// destroyResource hủy tài nguyên (có khóa)
func (p *ResourcePool) destroyResource(pooled *pooledResource) {
	p.mu.Lock()
	p.destroyResourceUnsafe(pooled)
	p.mu.Unlock()
}

// destroyResourceUnsafe hủy tài nguyên (không khóa)
func (p *ResourcePool) destroyResourceUnsafe(pooled *pooledResource) {
	if pooled.resource != nil {
		pooled.resource.Close()
		delete(p.resources, pooled.resource)
	}
}

// Stats lấy thông tin thống kê của resource pool
func (p *ResourcePool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	inUseCount := 0
	for _, pooled := range p.resources {
		if pooled.inUse {
			inUseCount++
		}
	}

	return map[string]interface{}{
		"total_resources":     len(p.resources),
		"available_resources": len(p.available),
		"in_use_resources":    inUseCount,
		"max_size":            p.config.MaxSize,
		"min_size":            p.config.MinSize,
		"max_idle":            p.config.MaxIdle,
		"is_closed":           p.closed,
	}
}

// Resize điều chỉnh kích thước pool
func (p *ResourcePool) Resize(newMaxSize int) error {
	if newMaxSize <= 0 {
		return errors.New("new max size must be positive")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return errors.New("pool is closed")
	}

	oldMaxSize := p.config.MaxSize
	p.config.MaxSize = newMaxSize

	// Nếu thu nhỏ kích thước pool, cần loại bỏ tài nguyên dư thừa
	if newMaxSize < oldMaxSize {
		excess := len(p.resources) - newMaxSize
		for excess > 0 {
			select {
			case pooled := <-p.available:
				p.destroyResourceUnsafe(pooled)
				excess--
			default:
				// Không còn tài nguyên khả dụng nào để loại bỏ
				break
			}
		}
	}

	return nil
}

// startCleanupRoutine khởi động goroutine dọn dẹp
func (p *ResourcePool) startCleanupRoutine() {
	if p.config.IdleTimeout <= 0 {
		return
	}

	p.cleanupWg.Add(1)
	go func() {
		defer p.cleanupWg.Done()
		ticker := time.NewTicker(p.config.IdleTimeout / 2)
		defer ticker.Stop()

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				p.cleanupIdleResources()
			}
		}
	}()
}

// cleanupIdleResources dọn dẹp tài nguyên đã hết thời gian rảnh
func (p *ResourcePool) cleanupIdleResources() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	now := time.Now()
	var toRemove []*pooledResource

	// Kiểm tra tài nguyên rảnh trong hàng đợi khả dụng
	for {
		select {
		case pooled := <-p.available:
			if now.Sub(pooled.lastUsed) > p.config.IdleTimeout {
				toRemove = append(toRemove, pooled)
			} else {
				// Đưa trở lại hàng đợi
				p.available <- pooled
				goto cleanup
			}
		default:
			goto cleanup
		}
	}

cleanup:
	// Hủy các tài nguyên đã hết thời gian
	for _, pooled := range toRemove {
		p.destroyResourceUnsafe(pooled)
	}
}

// Close đóng resource pool
func (p *ResourcePool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// Hủy context
	p.cancel()

	// Chờ goroutine dọn dẹp kết thúc
	p.cleanupWg.Wait()

	// Đóng tất cả tài nguyên
	p.mu.Lock()
	defer p.mu.Unlock()

	// Làm rỗng hàng đợi khả dụng
	close(p.available)
	for pooled := range p.available {
		p.destroyResourceUnsafe(pooled)
	}

	// Đóng tất cả tài nguyên
	for _, pooled := range p.resources {
		p.destroyResourceUnsafe(pooled)
	}

	return nil
}