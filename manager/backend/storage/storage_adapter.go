package storage

import (
	"context"
	"milestones/manager/backend/models"
)

// StorageAdapter adapter lưu trữ, dùng để cầu nối (bridge) sự khác biệt giữa các interface
type StorageAdapter struct {
	*GormBaseStorage
	userStorage   *GormUserStorage
	deviceStorage *GormDeviceStorage
	agentStorage  *GormAgentStorage
	configAdapter *ConfigAdapter
}

// NewStorageAdapter tạo mới adapter lưu trữ
func NewStorageAdapter(base *GormBaseStorage) *StorageAdapter {
	configStorage := NewGormConfigStorage(base.DB)
	return &StorageAdapter{
		GormBaseStorage: base,
		userStorage:     NewGormUserStorage(base.DB),
		deviceStorage:   NewGormDeviceStorage(base.DB),
		agentStorage:    NewGormAgentStorage(base.DB),
		configAdapter:   NewConfigAdapter(configStorage),
	}
}

// Connect kết nối cơ sở dữ liệu (phương thức của adapter)
func (a *StorageAdapter) Connect(ctx context.Context) error {
	// Lớp cơ sở (base class) đã kết nối sẵn, ở đây chỉ là để thích ứng (adapt) interface
	return nil
}

// Ping kiểm tra kết nối cơ sở dữ liệu (phương thức của adapter)
func (a *StorageAdapter) Ping(ctx context.Context) error {
	return a.GormBaseStorage.Ping()
}

// UserStorage trả về interface lưu trữ người dùng
func (a *StorageAdapter) UserStorage() UserStorage {
	return a.userStorage
}

// CreateUser tạo mới người dùng
func (a *StorageAdapter) CreateUser(ctx context.Context, user *models.User) error {
	return a.userStorage.CreateUser(ctx, user)
}

// GetUsers lấy tất cả người dùng
func (a *StorageAdapter) GetUsers(ctx context.Context, offset, limit int) ([]*models.User, int64, error) {
	return a.userStorage.GetUsers(ctx, offset, limit)
}

// GetUserByID lấy người dùng theo ID
func (a *StorageAdapter) GetUserByID(ctx context.Context, id uint) (*models.User, error) {
	return a.userStorage.GetUserByID(ctx, id)
}

// GetUserByUsername lấy người dùng theo tên đăng nhập (username)
func (a *StorageAdapter) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return a.userStorage.GetUserByUsername(ctx, username)
}

// GetUserByEmail lấy người dùng theo email
func (a *StorageAdapter) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return a.userStorage.GetUserByEmail(ctx, email)
}

// UpdateUser cập nhật người dùng
func (a *StorageAdapter) UpdateUser(ctx context.Context, id uint, updates map[string]interface{}) error {
	return a.userStorage.UpdateUser(ctx, id, updates)
}

// DeleteUser xóa người dùng
func (a *StorageAdapter) DeleteUser(ctx context.Context, id uint) error {
	return a.userStorage.DeleteUser(ctx, id)
}

// DeviceStorage trả về interface lưu trữ thiết bị
func (a *StorageAdapter) DeviceStorage() DeviceStorage {
	return a.deviceStorage
}

// CreateDevice tạo mới thiết bị
func (a *StorageAdapter) CreateDevice(ctx context.Context, device *models.Device) error {
	return a.deviceStorage.CreateDevice(ctx, device)
}

// GetDeviceByID lấy thiết bị theo ID
func (a *StorageAdapter) GetDeviceByID(ctx context.Context, id uint) (*models.Device, error) {
	return a.deviceStorage.GetDeviceByID(ctx, id)
}

// GetDeviceByCode lấy thiết bị theo mã thiết bị (device code)
func (a *StorageAdapter) GetDeviceByCode(ctx context.Context, deviceCode string) (*models.Device, error) {
	return a.deviceStorage.GetDeviceByCode(ctx, deviceCode)
}

// GetDevicesByUserID lấy danh sách thiết bị theo ID người dùng
func (a *StorageAdapter) GetDevicesByUserID(ctx context.Context, userID uint, offset, limit int) ([]*models.Device, int64, error) {
	return a.deviceStorage.GetDevicesByUserID(ctx, userID, offset, limit)
}

// UpdateDevice cập nhật thiết bị
func (a *StorageAdapter) UpdateDevice(ctx context.Context, id uint, updates map[string]interface{}) error {
	return a.deviceStorage.UpdateDevice(ctx, id, updates)
}

// DeleteDevice xóa thiết bị
func (a *StorageAdapter) DeleteDevice(ctx context.Context, id uint) error {
	return a.deviceStorage.DeleteDevice(ctx, id)
}

// AgentStorage trả về interface lưu trữ agent (smart agent/trợ lý ảo)
func (a *StorageAdapter) AgentStorage() AgentStorage {
	return a.agentStorage
}

// CreateAgent tạo mới agent
func (a *StorageAdapter) CreateAgent(ctx context.Context, agent *models.Agent) error {
	return a.agentStorage.CreateAgent(ctx, agent)
}

// GetAgentByID lấy agent theo ID
func (a *StorageAdapter) GetAgentByID(ctx context.Context, id uint) (*models.Agent, error) {
	return a.agentStorage.GetAgentByID(ctx, id)
}

// GetAgentsByUserID lấy danh sách agent theo ID người dùng
func (a *StorageAdapter) GetAgentsByUserID(ctx context.Context, userID uint, offset, limit int) ([]*models.Agent, int64, error) {
	return a.agentStorage.GetAgentsByUserID(ctx, userID, offset, limit)
}

// UpdateAgent cập nhật agent
func (a *StorageAdapter) UpdateAgent(ctx context.Context, id uint, updates map[string]interface{}) error {
	return a.agentStorage.UpdateAgent(ctx, id, updates)
}

// DeleteAgent xóa agent
func (a *StorageAdapter) DeleteAgent(ctx context.Context, id uint) error {
	return a.agentStorage.DeleteAgent(ctx, id)
}

// ConfigStorage trả về interface lưu trữ cấu hình
func (a *StorageAdapter) ConfigStorage() ConfigStorage {
	return a.configAdapter
}