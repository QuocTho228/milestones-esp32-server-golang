//go:build asr_server

package main

import (
	"context"
	"net/http"
	"time"

	"voice_server/server"
	log "milestones-esp32-server-golang/logger"
)

const (
	defaultAsrServerConfigPath = "asr_server.json"
)

var (
	asrHTTPServer *http.Server // Handle của dịch vụ HTTP asr_server được nhúng trong tiến trình này, dùng để tắt (shutdown) một cách an toàn
)

// StartAsrServerHTTP khởi động dịch vụ HTTP của asr_server ngay trong tiến trình này (cổng riêng biệt). Việc có gọi hàm này hay không do main quyết định dựa trên tham số -asr-enable.
// configPath: đường dẫn tệp cấu hình của asr_server, nếu để trống sẽ dùng đường dẫn mặc định asr_server/config.json
func StartAsrServerHTTP(configPath string) {
	if configPath == "" {
		configPath = defaultAsrServerConfigPath
	}
	log.Infof("Đang khởi động dịch vụ HTTP asr_server được nhúng sẵn, tệp cấu hình: %s", configPath)

	handler, addr, readTimeout, err := server.Setup(configPath)
	if err != nil {
		log.Warnf("Khởi tạo asr_server thất bại, bỏ qua việc khởi động: %v", err)
		return
	}

	asrHTTPServer = &http.Server{
		Addr:        addr,
		Handler:     handler,
		ReadTimeout: readTimeout,
	}

	go func() {
		log.Infof("Dịch vụ HTTP asr_server đã khởi động tại %s", addr)
		if err := asrHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("Dịch vụ HTTP asr_server thoát bất thường: %v", err)
		}
	}()
}

// StopAsrServerHTTP tắt (shutdown) một cách an toàn dịch vụ HTTP asr_server được nhúng trong tiến trình này
func StopAsrServerHTTP() {
	if asrHTTPServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := asrHTTPServer.Shutdown(ctx); err != nil {
			log.Warnf("Đóng dịch vụ HTTP asr_server bị timeout hoặc gặp lỗi: %v", err)
		}
		asrHTTPServer = nil
		log.Info("Dịch vụ HTTP asr_server đã được đóng")
	}
}