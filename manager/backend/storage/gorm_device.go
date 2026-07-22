package storage

import (
	"context"
	"gorm.io/gorm"
	"milestones/manager/backend/models"
)

// GormDeviceStorage triển khai lưu trữ thiết bị dùng chung bằng GORM
type GormDeviceStorage struct {
	db *gorm.DB
}

// NewGormDeviceStorage tạo mới instance lưu trữ thiết bị bằng GORM
func NewGormDeviceStorage(db *gorm.DB) *GormDeviceStorage {
	return &GormDeviceStorage{
		db: db,
	}
}

// CreateDevice tạo mới thiết bị
func (s *GormDeviceStorage) CreateDevice(ctx context.Context, device *models.Device) error {
	return s.db.WithContext(ctx).Create(device).Error
}

// GetDeviceByID lấy thiết bị theo ID
func (s *GormDeviceStorage) GetDeviceByID(ctx context.Context, id uint) (*models.Device, error) {
	var device models.Device
	err := s.db.WithContext(ctx).First(&device, id).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

// GetDeviceByCode lấy thiết bị theo mã thiết bị (device code)
func (s *GormDeviceStorage) GetDeviceByCode(ctx context.Context, deviceCode string) (*models.Device, error) {
	var device models.Device
	err := s.db.WithContext(ctx).Where("device_code = ?", deviceCode).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

// GetDevicesByUserID lấy danh sách thiết bị theo ID người dùng
func (s *GormDeviceStorage) GetDevicesByUserID(ctx context.Context, userID uint, offset, limit int) ([]*models.Device, int64, error) {
	var devices []*models.Device
	var total int64

	// Lấy tổng số lượng
	err := s.db.WithContext(ctx).Model(&models.Device{}).Where("user_id = ?", userID).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Lấy dữ liệu theo trang (phân trang)
	err = s.db.WithContext(ctx).Where("user_id = ?", userID).Offset(offset).Limit(limit).Find(&devices).Error
	return devices, total, err
}

// UpdateDevice cập nhật thiết bị
func (s *GormDeviceStorage) UpdateDevice(ctx context.Context, id uint, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).Model(&models.Device{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteDevice xóa thiết bị
func (s *GormDeviceStorage) DeleteDevice(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.Device{}, id).Error
}