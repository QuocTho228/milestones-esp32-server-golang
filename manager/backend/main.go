package main

import (
	"flag"
	"log"
	"milestones/manager/backend/config"
	"milestones/manager/backend/database"
	"milestones/manager/backend/router"

	"github.com/gin-gonic/gin"
)

func main() {
	// Xác định tham số dòng lệnh
	var configFile string
	flag.StringVar(&configFile, "config", "config/config.json", "Đường dẫn tệp cấu hình")
	flag.StringVar(&configFile, "c", "config/config.json", "Đường dẫn tệp cấu hình (viết tắt)")
	flag.Parse()

	// Tải cấu hình
	cfg := config.LoadWithPath(configFile)

	// Khởi tạo cơ sở dữ liệu
	db := database.Init(cfg.Database)
	if db == nil {
		log.Fatal("Khởi tạo cơ sở dữ liệu không thành công và dịch vụ đã thoát")
	}
	defer database.Close(db)

	// Thiết lập chế độ Gin
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Khởi tạo định tuyến
	r := router.Setup(db, cfg)

	// Khởi động máy chủ
	log.Printf("Sử dụng tệp cấu hình: %s", configFile)
	log.Printf("Máy chủ khởi động trên cổng: %s", cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal("Khởi động máy chủ không thành công:", err)
	}
}
