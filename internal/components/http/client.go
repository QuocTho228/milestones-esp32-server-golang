package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client HTTP client dùng chung
type Client struct {
	httpClient *http.Client
	baseURL    string
	authToken  string
	maxRetries int
}

// NewClient Tạo mới HTTP client
func NewClient(cfg ClientConfig) *Client {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 1 // Mặc định thử lại 1 lần
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL:    cfg.BaseURL,
		authToken:  cfg.AuthToken,
		maxRetries: cfg.MaxRetries,
	}
}

// DoRequest Thực hiện HTTP request
func (c *Client) DoRequest(ctx context.Context, opts RequestOptions) error {
	return c.doRequestOnce(ctx, opts)
}

// doRequestOnce Thực hiện một lần HTTP request
func (c *Client) doRequestOnce(ctx context.Context, opts RequestOptions) error {
	// Xây dựng URL
	reqURL := c.baseURL + opts.Path

	// Thêm query parameter
	if len(opts.QueryParams) > 0 {
		params := url.Values{}
		for k, v := range opts.QueryParams {
			params.Set(k, v)
		}
		reqURL += "?" + params.Encode()
	}

	// Xây dựng request body
	var bodyReader io.Reader
	if opts.Body != nil {
		data, err := json.Marshal(opts.Body)
		if err != nil {
			return fmt.Errorf("Serialize request body thất bại: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	// Tạo HTTP request
	req, err := http.NewRequestWithContext(ctx, opts.Method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("Tạo request thất bại: %w", err)
	}

	// Thiết lập header mặc định
	req.Header.Set("Content-Type", "application/json")

	// Thiết lập Token xác thực
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	// Thiết lập header tùy chỉnh
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	// Gửi request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Request thất bại: %w", err)
	}
	defer resp.Body.Close()

	// Đọc response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Đọc response thất bại: %w", err)
	}

	// Kiểm tra mã trạng thái HTTP
	/*if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}*/

	// Parse response body
	if opts.Response != nil {
		if err := json.Unmarshal(body, opts.Response); err != nil {
			return fmt.Errorf("Parse response thất bại: %w, response body: %s", err, string(body))
		}
	}

	return nil
}

// DoRequestRaw Thực hiện HTTP request và trả về response gốc (không tự động parse JSON)
func (c *Client) DoRequestRaw(ctx context.Context, opts RequestOptions) ([]byte, error) {
	var responseBody []byte
	var err error

	operation := func() error {
		// Xây dựng URL
		reqURL := c.baseURL + opts.Path

		// Thêm query parameter
		if len(opts.QueryParams) > 0 {
			params := url.Values{}
			for k, v := range opts.QueryParams {
				params.Set(k, v)
			}
			reqURL += "?" + params.Encode()
		}

		// Xây dựng request body
		var bodyReader io.Reader
		if opts.Body != nil {
			data, marshalErr := json.Marshal(opts.Body)
			if marshalErr != nil {
				return fmt.Errorf("Serialize request body thất bại: %w", marshalErr)
			}
			bodyReader = bytes.NewReader(data)
		}

		// Tạo HTTP request
		req, createErr := http.NewRequestWithContext(ctx, opts.Method, reqURL, bodyReader)
		if createErr != nil {
			return fmt.Errorf("Tạo request thất bại: %w", createErr)
		}

		// Thiết lập header mặc định
		req.Header.Set("Content-Type", "application/json")

		// Thiết lập Token xác thực
		if c.authToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.authToken)
		}

		// Thiết lập header tùy chỉnh
		for k, v := range opts.Headers {
			req.Header.Set(k, v)
		}

		// Gửi request
		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return fmt.Errorf("Request thất bại: %w", doErr)
		}
		defer resp.Body.Close()

		// Đọc response body
		responseBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("Đọc response thất bại: %w", err)
		}

		// Kiểm tra mã trạng thái HTTP
		if resp.StatusCode >= 400 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(responseBody))
		}

		return nil
	}

	if err := operation(); err != nil {
		return nil, err
	}

	return responseBody, nil
}