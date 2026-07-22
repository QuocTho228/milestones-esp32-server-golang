package storage

import (
	"context"
	"milestones/manager/backend/models"
)

// Storage interface lưu trữ dùng chung
type Storage interface {
	// Quản lý kết nối
	Connect(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error

	// Quản lý transaction (giao dịch)
	BeginTx(ctx context.Context) (Transaction, error)

	// Quản lý người dùng
	UserStorage
	// Quản lý thiết bị
	DeviceStorage
	// Quản lý agent (smart agent/trợ lý ảo)
	AgentStorage
	// Quản lý cấu hình
	ConfigStorage
}

// Transaction interface transaction (giao dịch)
type Transaction interface {
	Commit() error
	Rollback() error
	// Thực thi các thao tác lưu trữ trong transaction
	UserStorage
	DeviceStorage
	AgentStorage
	ConfigStorage
}

// UserStorage interface lưu trữ người dùng
type UserStorage interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByID(ctx context.Context, id uint) (*models.User, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUsers(ctx context.Context, offset, limit int) ([]*models.User, int64, error)
	UpdateUser(ctx context.Context, id uint, updates map[string]interface{}) error
	DeleteUser(ctx context.Context, id uint) error
}

// DeviceStorage interface lưu trữ thiết bị
type DeviceStorage interface {
	CreateDevice(ctx context.Context, device *models.Device) error
	GetDeviceByID(ctx context.Context, id uint) (*models.Device, error)
	GetDeviceByCode(ctx context.Context, deviceCode string) (*models.Device, error)
	GetDevicesByUserID(ctx context.Context, userID uint, offset, limit int) ([]*models.Device, int64, error)
	UpdateDevice(ctx context.Context, id uint, updates map[string]interface{}) error
	DeleteDevice(ctx context.Context, id uint) error
}

// AgentStorage interface lưu trữ agent (smart agent/trợ lý ảo)
type AgentStorage interface {
	CreateAgent(ctx context.Context, agent *models.Agent) error
	GetAgentByID(ctx context.Context, id uint) (*models.Agent, error)
	GetAgentsByUserID(ctx context.Context, userID uint, offset, limit int) ([]*models.Agent, int64, error)
	UpdateAgent(ctx context.Context, id uint, updates map[string]interface{}) error
	DeleteAgent(ctx context.Context, id uint) error
}

// ConfigStorage interface lưu trữ cấu hình
type ConfigStorage interface {
	// Các thao tác cấu hình dùng chung
	CreateConfig(ctx context.Context, config *models.Config) error
	GetConfigs(ctx context.Context, configType string) ([]*models.Config, error)
	GetConfigByID(ctx context.Context, id uint) (*models.Config, error)
	GetConfigByTypeAndName(ctx context.Context, configType, name string) (*models.Config, error)
	GetDefaultConfig(ctx context.Context, configType string) (*models.Config, error)
	UpdateConfig(ctx context.Context, id uint, updates map[string]interface{}) error
	DeleteConfig(ctx context.Context, id uint) error
	SetDefaultConfig(ctx context.Context, configType string, id uint) error

	// Cấu hình vai trò toàn cục (global role)
	CreateGlobalRole(ctx context.Context, role *models.GlobalRole) error
	GetGlobalRoles(ctx context.Context) ([]*models.GlobalRole, error)
	GetGlobalRoleByID(ctx context.Context, id uint) (*models.GlobalRole, error)
	UpdateGlobalRole(ctx context.Context, id uint, updates map[string]interface{}) error
	DeleteGlobalRole(ctx context.Context, id uint) error
}

// StorageConfig interface cấu hình lưu trữ
type StorageConfig interface {
	GetType() string
	Validate() error
}