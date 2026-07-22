package storage

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// GormBaseStorage lớp cơ sở (base class) lưu trữ dùng chung bằng GORM
// Chứa phần triển khai dùng chung cho tất cả các thao tác lưu trữ dựa trên GORM
type GormBaseStorage struct {
	DB *gorm.DB // Trường được export, cho phép các lớp con truy cập
}

// NewGormBaseStorage tạo mới instance lưu trữ nền tảng (base storage) bằng GORM
func NewGormBaseStorage(db *gorm.DB) *GormBaseStorage {
	return &GormBaseStorage{
		DB: db,
	}
}

// Ping kiểm tra kết nối cơ sở dữ liệu
func (s *GormBaseStorage) Ping() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	return sqlDB.Ping()
}

// Close đóng kết nối cơ sở dữ liệu
func (s *GormBaseStorage) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}

// BeginTx bắt đầu transaction (giao dịch)
func (s *GormBaseStorage) BeginTx(ctx context.Context) (Transaction, error) {
	tx := s.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	transaction := &GormTransaction{
		DB: tx,
	}
	transaction.init()
	return transaction, nil
}

// GormTransaction triển khai transaction dùng chung bằng GORM
type GormTransaction struct {
	DB *gorm.DB
	*GormUserStorage
	*GormDeviceStorage
	*GormAgentStorage
	*GormConfigStorage
}

// init khởi tạo các thành phần lưu trữ trong transaction
func (t *GormTransaction) init() {
	t.GormUserStorage = &GormUserStorage{db: t.DB}
	t.GormDeviceStorage = &GormDeviceStorage{db: t.DB}
	t.GormAgentStorage = &GormAgentStorage{db: t.DB}
	t.GormConfigStorage = &GormConfigStorage{db: t.DB}
}

// Commit xác nhận (commit) transaction
func (t *GormTransaction) Commit() error {
	if err := t.DB.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Rollback hoàn tác (rollback) transaction
func (t *GormTransaction) Rollback() error {
	if err := t.DB.Rollback().Error; err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	return nil
}