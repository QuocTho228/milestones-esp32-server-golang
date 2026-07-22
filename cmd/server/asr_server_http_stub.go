//go:build !asr_server

package main

import log "milestones-esp32-server-golang/logger"

// StartAsrServerHTTP biên dịch với việc triển khai asr_server trống không được bật. Yêu cầu biên dịch với -tags asr_server để kích hoạt asr_server được nhúng.
func StartAsrServerHTTP(configPath string) {
	log.Warn("asr_server 内嵌未编译进本二进制，请使用 -tags asr_server 重新编译以启用")
}

// StopAsrServerHTTP Việc triển khai asr_server trống khi được biên dịch mà không bật.
func StopAsrServerHTTP() {}
