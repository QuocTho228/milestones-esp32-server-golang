package sqlite

import (
	"fmt"
	"path/filepath"
	"milestones/manager/backend/config"
)

// Config: Cấu hình SQLite.
type Config struct {
	// FilePath: Đường dẫn đến tệp cơ sở dữ liệu (ví dụ: ./data/milestones.db hoặc /path/to/milestones.db).
	FilePath string `json:"file_path"`

	// Cấu hình nhóm kết nối (đối với SQLite, một kết nối thường là đủ).
	MaxIdleConns    int `json:"max_idle_conns"`
	MaxOpenConns    int `json:"max_open_conns"`
	ConnMaxLifetime int `json:"conn_max_lifetime"`
}

// NewConfigFromDatabase: Tạo cấu hình SQLite từ cấu hình cơ sở dữ liệu.
func NewConfigFromDatabase(cfg *config.SQLiteConfig) *Config {
	filePath := cfg.FilePath
	if filePath == "" {
		filePath = "./data/milestones.db"
	}

	return &Config{
		FilePath:       filePath,
		MaxIdleConns:   1,
		MaxOpenConns:   1,
		ConnMaxLifetime: 3600,
	}
}

// DSN: Tạo tên nguồn dữ liệu (định dạng GORM SQLite).
func (c *Config) DSN() string {
	// Đảm bảo sử dụng tiền tố file: để hỗ trợ thêm nhiều tùy chọn.
	return "file:" + c.FilePath + "?_foreign_keys=on&_journal_mode=WAL"
}

// Validate: Kiểm tra tính hợp lệ của cấu hình.
func (c *Config) Validate() error {
	if c.FilePath == "" {
		return fmt.Errorf("SQLite file path is required")
	}

	// Kiểm tra phần mở rộng của tệp.
	ext := filepath.Ext(c.FilePath)
	if ext != ".db" && ext != ".sqlite" && ext != ".sqlite3" {
		return fmt.Errorf("SQLite file must have .db, .sqlite or .sqlite3 extension")
	}

	return nil
}

// ValidateConfig: Kiểm tra tính hợp lệ của cấu hình SQLite.
func ValidateConfig(cfg *config.SQLiteConfig) error {
	if cfg == nil {
		return fmt.Errorf("SQLite config is required")
	}
	if cfg.FilePath == "" {
		return fmt.Errorf("SQLite file path is required")
	}

	// Kiểm tra phần mở rộng của tệp.
	ext := filepath.Ext(cfg.FilePath)
	if ext != ".db" && ext != ".sqlite" && ext != ".sqlite3" {
		return fmt.Errorf("SQLite file must have .db, .sqlite or .sqlite3 extension")
	}

	return nil
}
