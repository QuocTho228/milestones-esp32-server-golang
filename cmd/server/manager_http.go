//go:build manager

package main

import (
	"context"
	"net/http"
	"time"

	log "milestones-esp32-server-golang/logger"
	mbconfig "milestones/manager/backend/config"
	"milestones/manager/backend/database"
	"milestones/manager/backend/router"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	defaultManagerHTTPPort   = "9000"
	defaultManagerConfigPath = "manager.json"
)

var (
	managerHTTPServer *http.Server // Handle của dịch vụ HTTP manager được nhúng trong tiến trình này, dùng để tắt (shutdown) một cách an toàn
	managerDB         *gorm.DB     // DB mà manager sử dụng, sẽ được đóng khi thoát chương trình
)

// StartManagerHTTP khởi động dịch vụ HTTP của manager ngay trong tiến trình này (hai cổng). Việc có gọi hàm này hay không do main quyết định dựa trên tham số -manager-enable.
// configPath: đường dẫn tệp cấu hình của manager, nếu để trống sẽ dùng đường dẫn mặc định
func StartManagerHTTP(configPath string) {
	if configPath == "" {
		configPath = defaultManagerConfigPath
	}
	log.Infof("Đang khởi động dịch vụ HTTP manager được nhúng sẵn, tệp cấu hình: %s", configPath)

	cfg := mbconfig.LoadWithPath(configPath)
	port := cfg.Server.Port
	if port == "" {
		port = defaultManagerHTTPPort
	}
	cfg.Server.Port = port

	db := database.Init(cfg.Database)
	if db == nil {
		log.Warn("Khởi tạo cơ sở dữ liệu của manager thất bại, bỏ qua việc khởi động manager HTTP")
		return
	}
	managerDB = db

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := router.Setup(db, cfg)

	managerHTTPServer = &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		log.Infof("Dịch vụ HTTP manager đã khởi động tại cổng: %s", port)
		if err := managerHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("Dịch vụ HTTP manager thoát bất thường: %v", err)
		}
	}()
}

// StopManagerHTTP tắt (shutdown) một cách an toàn dịch vụ HTTP manager được nhúng trong tiến trình này và đóng kết nối cơ sở dữ liệu
func StopManagerHTTP() {
	if managerHTTPServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := managerHTTPServer.Shutdown(ctx); err != nil {
			log.Warnf("Đóng dịch vụ HTTP manager bị timeout hoặc gặp lỗi: %v", err)
		}
		managerHTTPServer = nil
		log.Info("Dịch vụ HTTP manager đã được đóng")
	}
	if managerDB != nil {
		database.Close(managerDB)
		managerDB = nil
	}
}