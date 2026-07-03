package controllers

import (
	"net/http"
	"time"
	"milestones/manager/backend/storage"

	"github.com/gin-gonic/gin"
)

// PoolStatsController Controller thống kê pool tài nguyên
type PoolStatsController struct {
	storage *storage.PoolStatsStorage
}

// NewPoolStatsController Tạo controller thống kê pool tài nguyên
func NewPoolStatsController() *PoolStatsController {
	return &PoolStatsController{
		storage: storage.GetPoolStatsStorage(),
	}
}

// ReportPoolStats Nhận dữ liệu thống kê do dịch vụ chính báo lên (API nội bộ, không cần xác thực)
func (c *PoolStatsController) ReportPoolStats(ctx *gin.Context) {
	var request struct {
		Stats map[string]interface{} `json:"stats" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu không hợp lệ: " + err.Error()})
		return
	}

	// Lưu dữ liệu thống kê
	c.storage.AddStats(request.Stats)

	ctx.JSON(http.StatusOK, gin.H{
		"message":   "Báo cáo dữ liệu thống kê thành công",
		"timestamp": time.Now().Unix(),
	})
}

// GetPoolStats Lấy dữ liệu thống kê pool tài nguyên (API quản trị viên)
func (c *PoolStatsController) GetPoolStats(ctx *gin.Context) {
	// Lấy tham số truy vấn
	queryType := ctx.DefaultQuery("type", "latest") // latest, all, range

	switch queryType {
	case "latest":
		// Lấy dữ liệu mới nhất
		latest := c.storage.GetLatestStats()
		if latest == nil {
			ctx.JSON(http.StatusOK, gin.H{
				"data":    nil,
				"message": "Chưa có dữ liệu thống kê",
			})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{
			"data": latest,
		})

	case "all":
		// Lấy toàn bộ dữ liệu (24 giờ gần nhất)
		allStats := c.storage.GetAllStats()
		ctx.JSON(http.StatusOK, gin.H{
			"data":  allStats,
			"count": len(allStats),
		})

	case "range":
		// Lấy dữ liệu theo khoảng thời gian
		startStr := ctx.Query("start")
		endStr := ctx.Query("end")

		if startStr == "" || endStr == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Tham số khoảng thời gian start và end không được để trống"})
			return
		}

		start, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Sai định dạng thời gian bắt đầu, vui lòng dùng định dạng RFC3339"})
			return
		}

		end, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Sai định dạng thời gian kết thúc, vui lòng dùng định dạng RFC3339"})
			return
		}

		stats := c.storage.GetStatsByTimeRange(start, end)
		ctx.JSON(http.StatusOK, gin.H{
			"data":  stats,
			"count": len(stats),
		})

	default:
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Loại truy vấn không hợp lệ, hỗ trợ: latest, all, range"})
	}
}

// GetPoolStatsSummary Lấy thông tin tóm tắt thống kê
func (c *PoolStatsController) GetPoolStatsSummary(ctx *gin.Context) {
	latest := c.storage.GetLatestStats()

	summary := gin.H{
		"total_records":    0,
		"storage_duration": "Chỉ lưu dữ liệu mới nhất",
		"oldest_timestamp": nil,
		"newest_timestamp": nil,
	}

	if latest != nil {
		summary["total_records"] = 1
		summary["newest_timestamp"] = latest.Timestamp
		summary["oldest_timestamp"] = latest.Timestamp
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": summary,
	})
}