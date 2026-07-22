package pool

import (
	"milestones-esp32-server-golang/internal/util"
)

// ResourceWrapper wrapper (lớp bọc) resource dạng generic
// T: kiểu resource cụ thể (ví dụ vad.VAD, asr.AsrProvider, v.v.)
type ResourceWrapper[T any] struct {
	provider     T             // Provider resource thực tế (type-safe)
	configKey    string        // Khóa cấu hình, dùng để định danh resource pool
	resourceType string        // Loại resource (vad/asr/llm/tts...)
	closeFunc    func(T) error // Hàm đóng resource
	isValidFunc  func(T) bool  // Hàm kiểm tra resource có hợp lệ hay không
	resetFunc    func(T) error // Hàm đặt lại trạng thái resource (tùy chọn)
}

// Close đóng resource
func (r *ResourceWrapper[T]) Close() error {
	if r.closeFunc != nil {
		return r.closeFunc(r.provider)
	}
	return nil
}

// IsValid kiểm tra resource có hợp lệ hay không
func (r *ResourceWrapper[T]) IsValid() bool {
	if r.isValidFunc != nil {
		return r.isValidFunc(r.provider)
	}
	var zero T
	return any(r.provider) != any(zero)
}

// GetProvider lấy provider resource thực tế (type-safe, không cần type assertion)
func (r *ResourceWrapper[T]) GetProvider() T {
	return r.provider
}

// GetConfigKey lấy khóa cấu hình
func (r *ResourceWrapper[T]) GetConfigKey() string {
	return r.configKey
}

// GetResourceType lấy loại resource
func (r *ResourceWrapper[T]) GetResourceType() string {
	return r.resourceType
}

// Reset đặt lại trạng thái resource
func (r *ResourceWrapper[T]) Reset() error {
	if r.resetFunc != nil {
		return r.resetFunc(r.provider)
	}
	return nil
}

// CreatorFunc kiểu hàm tạo resource dạng generic
// T: kiểu resource
// Tham số: resourceType, provider, config
// Trả về: thực thể resource (kiểu T) và lỗi (nếu có)
type CreatorFunc[T any] func(resourceType, provider string, config map[string]interface{}) (T, error)

// ResourceFactory factory (nhà máy tạo instance) resource dạng generic
type ResourceFactory[T any] struct {
	resourceType string
	provider     string
	config       map[string]interface{}
	configKey    string
	creator      CreatorFunc[T]
	closeFunc    func(T) error
	isValidFunc  func(T) bool
	resetFunc    func(T) error
}

// Create tạo resource
func (f *ResourceFactory[T]) Create() (util.Resource, error) {
	provider, err := f.creator(f.resourceType, f.provider, f.config)
	if err != nil {
		return nil, err
	}

	return &ResourceWrapper[T]{
		provider:     provider,
		configKey:    f.configKey,
		resourceType: f.resourceType,
		closeFunc:    f.closeFunc,
		isValidFunc:  f.isValidFunc,
		resetFunc:    f.resetFunc,
	}, nil
}

// Validate kiểm tra resource
func (f *ResourceFactory[T]) Validate(resource util.Resource) bool {
	if wrapper, ok := resource.(*ResourceWrapper[T]); ok {
		if f.isValidFunc != nil {
			return f.isValidFunc(wrapper.provider)
		}
		return wrapper.IsValid()
	}
	return resource != nil && resource.IsValid()
}

// Reset đặt lại resource
func (f *ResourceFactory[T]) Reset(resource util.Resource) error {
	if wrapper, ok := resource.(*ResourceWrapper[T]); ok {
		if wrapper.resetFunc != nil {
			return wrapper.resetFunc(wrapper.provider)
		}
		return wrapper.Reset()
	}
	return nil
}