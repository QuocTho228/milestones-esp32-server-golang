package pool

import (
	"context"
	"sync"
	"time"
	"milestones-esp32-server-golang/internal/components/http"
	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"

	"github.com/spf13/viper"
)

// StatsReporter bộ báo cáo thống kê của resource pool
type StatsReporter struct {
	client  *http.ManagerClient
	enabled bool
}

var (
	globalReporter *StatsReporter
	reporterOnce   sync.Once
)

// GetStatsReporter lấy bộ báo cáo thống kê toàn cục (singleton)
func GetStatsReporter() *StatsReporter {
	reporterOnce.Do(func() {
		// Lấy URL backend của manager, ưu tiên lấy từ biến môi trường, nếu không có thì lấy từ cấu hình
		baseURL := util.GetBackendURL()
		if baseURL == "" {
			baseURL = "http://localhost:8080" // Giá trị mặc định
		}

		// Kiểm tra xem có bật báo cáo hay không
		enabled := viper.GetBool("pool_stats.report_enabled")
		if !enabled {
			// Mặc định bật
			enabled = true
		}

		// Tạo HTTP client
		managerClient := http.NewManagerClient(http.ManagerClientConfig{
			BaseURL:    baseURL,
			AuthToken:  util.GetManagerAuthToken(),
			Timeout:    5 * time.Second,
			MaxRetries: 2,
		})

		globalReporter = &StatsReporter{
			client:  managerClient,
			enabled: enabled,
		}

		log.Infof("Trình báo cáo thống kê nhóm tài nguyên đã được khởi tạo, backend_url=%s, enabled=%v", baseURL, enabled)
	})
	return globalReporter
}

// StartReporting khởi động báo cáo thống kê (báo cáo mỗi 5 giây)
func (r *StatsReporter) StartReporting(ctx context.Context) {
	if !r.enabled {
		log.Info("Báo cáo thống kê nhóm tài nguyên bị vô hiệu hóa")
		return
	}

	// Khoảng thời gian báo cáo (5 giây)
	interval := viper.GetDuration("pool_stats.report_interval")
	if interval == 0 {
		interval = 5 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		//log.Infof("Báo cáo thống kê nhóm tài nguyên đã được khởi động, mỗi %v báo cáo một lần", interval)

		for {
			select {
			case <-ctx.Done():
				log.Debugf("Báo cáo thống kê nhóm tài nguyên đã dừng")
				return
			case <-ticker.C:
				r.reportStats(ctx)
			}
		}
	}()
}

// reportStats báo cáo dữ liệu thống kê
func (r *StatsReporter) reportStats(ctx context.Context) {
	// Lấy dữ liệu thống kê
	stats := GetStats()

	// Nếu không có dữ liệu, bỏ qua việc báo cáo
	if len(stats) == 0 {
		//log.Debugf("Hiện không có resource pool nào đang hoạt động, bỏ qua báo cáo")
		return
	}

	// Xây dựng request body
	requestBody := map[string]interface{}{
		"stats": stats,
	}

	// Gửi yêu cầu báo cáo
	err := r.client.DoRequest(ctx, http.RequestOptions{
		Method: "POST",
		Path:   "/api/internal/pool/stats",
		Body:   requestBody,
	})

	if err != nil {
		log.Warnf("Không thể báo cáo số liệu thống kê nhóm tài nguyên: %v", err)
	} else {
		//log.Debugf("Báo cáo thống kê resource pool thành công, số lượng resource pool: %d", len(stats))
	}
}

// StartStatsReporter khởi động bộ báo cáo thống kê toàn cục (hàm tiện ích)
func StartStatsReporter(ctx context.Context) {
	reporter := GetStatsReporter()
	reporter.StartReporting(ctx)
}