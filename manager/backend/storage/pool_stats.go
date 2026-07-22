package storage

import (
	"sync"
	"time"
)

// PoolStatsData dữ liệu thống kê resource pool (bể tài nguyên)
type PoolStatsData struct {
	Timestamp time.Time              `json:"timestamp"`
	Stats     map[string]interface{} `json:"stats"`
}

// PoolStatsStorage lưu trữ thống kê resource pool (lưu trong bộ nhớ, chỉ giữ dữ liệu mới nhất)
type PoolStatsStorage struct {
	mu     sync.RWMutex
	latest *PoolStatsData // Chỉ lưu dữ liệu thống kê mới nhất
}

var (
	globalPoolStatsStorage *PoolStatsStorage
	once                   sync.Once
)

// GetPoolStatsStorage lấy storage thống kê resource pool toàn cục (singleton)
func GetPoolStatsStorage() *PoolStatsStorage {
	once.Do(func() {
		globalPoolStatsStorage = &PoolStatsStorage{
			latest: nil,
		}
	})
	return globalPoolStatsStorage
}

// AddStats thêm dữ liệu thống kê (chỉ lưu dữ liệu mới nhất, ghi đè dữ liệu cũ)
func (s *PoolStatsStorage) AddStats(stats map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ghi đè trực tiếp dữ liệu mới nhất
	s.latest = &PoolStatsData{
		Timestamp: time.Now(),
		Stats:     stats,
	}
}

// GetLatestStats lấy dữ liệu thống kê mới nhất
func (s *PoolStatsStorage) GetLatestStats() *PoolStatsData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.latest == nil {
		return nil
	}

	// Trả về bản sao (copy) của dữ liệu mới nhất
	latest := *s.latest
	return &latest
}

// GetAllStats lấy toàn bộ dữ liệu thống kê (chỉ trả về bản ghi mới nhất)
func (s *PoolStatsStorage) GetAllStats() []PoolStatsData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.latest == nil {
		return []PoolStatsData{}
	}

	// Chỉ trả về bản ghi mới nhất
	return []PoolStatsData{*s.latest}
}

// GetStatsByTimeRange lấy dữ liệu thống kê theo khoảng thời gian (chỉ trả về dữ liệu mới nhất, nếu nằm trong khoảng thời gian đó)
func (s *PoolStatsStorage) GetStatsByTimeRange(start, end time.Time) []PoolStatsData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.latest == nil {
		return []PoolStatsData{}
	}

	// Kiểm tra xem dữ liệu mới nhất có nằm trong khoảng thời gian hay không
	if s.latest.Timestamp.After(start) && s.latest.Timestamp.Before(end) {
		return []PoolStatsData{*s.latest}
	}

	return []PoolStatsData{}
}

// GetStatsCount lấy số lượng bản ghi hiện đang được lưu trữ (chỉ lưu dữ liệu mới nhất, nên trả về 0 hoặc 1)
func (s *PoolStatsStorage) GetStatsCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.latest == nil {
		return 0
	}
	return 1
}