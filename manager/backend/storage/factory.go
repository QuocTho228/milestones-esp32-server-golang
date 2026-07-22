package storage

import (
	"fmt"

	"milestones/manager/backend/config"
	"milestones/manager/backend/storage/mysql"
	"milestones/manager/backend/storage/sqlite"
)

// StorageType loại lưu trữ
type StorageType string

const (
	StorageTypeMySQL  StorageType = "mysql"
	StorageTypeSQLite StorageType = "sqlite"
)

// Factory factory (xưởng tạo) lưu trữ
type Factory struct{}

// NewFactory tạo mới factory lưu trữ
func NewFactory() *Factory {
	return &Factory{}
}

// CreateStorage tạo mới instance lưu trữ
func CreateStorage(dbConfig config.DatabaseConfig) (*StorageAdapter, error) {
	// Xác định loại lưu trữ dựa theo cấu hình
	storageType := dbConfig.GetStorageType()

	switch StorageType(storageType) {
	case StorageTypeSQLite:
		if dbConfig.SQLite == nil {
			return nil, fmt.Errorf("SQLite config is required")
		}
		// Kiểm tra hợp lệ cấu hình SQLite
		if err := sqlite.ValidateConfig(dbConfig.SQLite); err != nil {
			return nil, fmt.Errorf("invalid SQLite config: %w", err)
		}
		// Tạo cấu hình SQLite
		sqliteConfig := sqlite.NewConfigFromDatabase(dbConfig.SQLite)
		// Tạo storage SQLite
		sqliteStorage, err := sqlite.NewStorage(sqliteConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create SQLite storage: %w", err)
		}
		// Tạo storage nền tảng (base storage)
		baseStorage := NewGormBaseStorage(sqliteStorage.DB)
		// Trả về adapter
		return NewStorageAdapter(baseStorage), nil

	case StorageTypeMySQL:
		if dbConfig.MySQL == nil {
			return nil, fmt.Errorf("MySQL config is required")
		}
		// Kiểm tra hợp lệ cấu hình MySQL
		if err := mysql.ValidateConfig(dbConfig.MySQL); err != nil {
			return nil, fmt.Errorf("invalid MySQL config: %w", err)
		}
		// Tạo cấu hình MySQL
		mysqlConfig := mysql.NewConfigFromDatabase(dbConfig.MySQL)
		// Tạo storage MySQL
		mysqlStorage, err := mysql.NewStorage(mysqlConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create MySQL storage: %w", err)
		}
		// Tạo storage nền tảng (base storage)
		baseStorage := NewGormBaseStorage(mysqlStorage.DB)
		// Trả về adapter
		return NewStorageAdapter(baseStorage), nil

	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

// GetSupportedTypes lấy danh sách các loại lưu trữ được hỗ trợ
func (f *Factory) GetSupportedTypes() []StorageType {
	return []StorageType{
		StorageTypeMySQL,
		StorageTypeSQLite,
	}
}