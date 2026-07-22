package storage

import (
	"context"
	"gorm.io/gorm"
	"milestones/manager/backend/models"
)

// GormUserStorage triển khai lưu trữ người dùng dùng chung bằng GORM
type GormUserStorage struct {
	db *gorm.DB
}

// NewGormUserStorage tạo mới instance lưu trữ người dùng bằng GORM
func NewGormUserStorage(db *gorm.DB) *GormUserStorage {
	return &GormUserStorage{
		db: db,
	}
}

// CreateUser tạo mới người dùng
func (s *GormUserStorage) CreateUser(ctx context.Context, user *models.User) error {
	return s.db.WithContext(ctx).Create(user).Error
}

// GetUserByID lấy người dùng theo ID
func (s *GormUserStorage) GetUserByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	err := s.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername lấy người dùng theo tên đăng nhập (username)
func (s *GormUserStorage) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByEmail lấy người dùng theo email
func (s *GormUserStorage) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUsers lấy danh sách người dùng
func (s *GormUserStorage) GetUsers(ctx context.Context, offset, limit int) ([]*models.User, int64, error) {
	var users []*models.User
	var total int64

	// Lấy tổng số lượng
	err := s.db.WithContext(ctx).Model(&models.User{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Lấy dữ liệu theo trang (phân trang)
	err = s.db.WithContext(ctx).Offset(offset).Limit(limit).Find(&users).Error
	return users, total, err
}

// UpdateUser cập nhật người dùng
func (s *GormUserStorage) UpdateUser(ctx context.Context, id uint, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteUser xóa người dùng
func (s *GormUserStorage) DeleteUser(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.User{}, id).Error
}