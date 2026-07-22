package http

import "time"

// ClientConfig Cấu hình HTTP client
type ClientConfig struct {
	BaseURL   string        // URL cơ sở (base URL)
	AuthToken string        // Token xác thực (tùy chọn)
	Timeout   time.Duration // Thời gian timeout của request
	MaxRetries int          // Số lần thử lại tối đa (mặc định 3 lần)
}

// RequestOptions Các tùy chọn cho request
type RequestOptions struct {
	Method      string                 // Phương thức HTTP
	Path        string                 // Đường dẫn request
	QueryParams map[string]string      // Tham số truy vấn (query)
	Headers     map[string]string      // Header tùy chỉnh
	Body        interface{}             // Request body (sẽ tự động serialize thành JSON)
	Response    interface{}             // Response body (sẽ tự động deserialize)
}