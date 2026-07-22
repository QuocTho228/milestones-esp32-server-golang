package storage

import (
	"context"
	"gorm.io/gorm"
	"milestones/manager/backend/models"
)

// GormAgentStorage triển khai lưu trữ agent dùng chung bằng GORM
type GormAgentStorage struct {
	db *gorm.DB
}

// NewGormAgentStorage tạo mới instance lưu trữ agent bằng GORM
func NewGormAgentStorage(db *gorm.DB) *GormAgentStorage {
	return &GormAgentStorage{
		db: db,
	}
}

// CreateAgent tạo mới agent
func (s *GormAgentStorage) CreateAgent(ctx context.Context, agent *models.Agent) error {
	return s.db.WithContext(ctx).Create(agent).Error
}

// GetAgentByID lấy agent theo ID
func (s *GormAgentStorage) GetAgentByID(ctx context.Context, id uint) (*models.Agent, error) {
	var agent models.Agent
	err := s.db.WithContext(ctx).First(&agent, id).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// GetAgentsByUserID lấy danh sách agent theo ID người dùng
func (s *GormAgentStorage) GetAgentsByUserID(ctx context.Context, userID uint, offset, limit int) ([]*models.Agent, int64, error) {
	var agents []*models.Agent
	var total int64

	// Lấy tổng số lượng
	err := s.db.WithContext(ctx).Model(&models.Agent{}).Where("user_id = ?", userID).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Lấy dữ liệu theo trang (phân trang)
	err = s.db.WithContext(ctx).Where("user_id = ?", userID).Offset(offset).Limit(limit).Find(&agents).Error
	return agents, total, err
}

// UpdateAgent cập nhật agent
func (s *GormAgentStorage) UpdateAgent(ctx context.Context, id uint, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).Model(&models.Agent{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteAgent xóa agent
func (s *GormAgentStorage) DeleteAgent(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.Agent{}, id).Error
}