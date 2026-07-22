package http

import (
	"context"
	"time"
)

// ManagerClient HTTP client chuyên dụng cho backend Manager
type ManagerClient struct {
	client *Client
}

// ManagerClientConfig Cấu hình client Manager
type ManagerClientConfig struct {
	BaseURL   string        // Địa chỉ backend Manager
	AuthToken string        // Token xác thực (tùy chọn)
	Timeout   time.Duration // Thời gian timeout của request
	MaxRetries int          // Số lần thử lại tối đa
}

// NewManagerClient Tạo HTTP client cho backend Manager
func NewManagerClient(cfg ManagerClientConfig) *ManagerClient {
	client := NewClient(ClientConfig{
		BaseURL:    cfg.BaseURL,
		AuthToken:  cfg.AuthToken,
		Timeout:    cfg.Timeout,
		MaxRetries: cfg.MaxRetries,
	})

	return &ManagerClient{
		client: client,
	}
}

// DoRequest Thực hiện HTTP request (bọc DoRequest của client dùng chung)
func (m *ManagerClient) DoRequest(ctx context.Context, opts RequestOptions) error {
	return m.client.DoRequest(ctx, opts)
}

// DoRequestRaw Thực hiện HTTP request và trả về response gốc
func (m *ManagerClient) DoRequestRaw(ctx context.Context, opts RequestOptions) ([]byte, error) {
	return m.client.DoRequestRaw(ctx, opts)
}