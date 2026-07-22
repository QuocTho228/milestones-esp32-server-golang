//go:build !manager

package main

import log "milestones-esp32-server-golang/logger"

// StartManagerHTTP là phần triển khai rỗng khi bản build không bật `manager`. Biên dịch với `-tags manager` để bật HTTP Manager nhúng.
func StartManagerHTTP(configPath string) {
	log.Warn("manager 内嵌未编译进本二进制，请使用 -tags manager 重新编译以启用")
}

// StopManagerHTTP là phần triển khai rỗng khi bản build không bật manager.
func StopManagerHTTP() {}
