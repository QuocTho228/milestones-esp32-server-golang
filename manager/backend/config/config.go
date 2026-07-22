package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Config struct {
	Server            ServerConfig         `json:"server"`
	Database          DatabaseConfig       `json:"database"`
	JWT               JWTConfig            `json:"jwt"`
	InternalAuthToken string               `json:"internal_auth_token"`
	EndpointAuthToken string               `json:"endpoint_auth_token"`
	SpeakerService    SpeakerServiceConfig `json:"speaker_service"`
	Storage           StorageConfig        `json:"storage"`
	History           HistoryConfig        `json:"history"`
}

type ServerConfig struct {
	Port string `json:"port"`
	Mode string `json:"mode"`
}

type DatabaseConfig struct {
	Type   string        `json:"type"` // "mysql" hoặc "sqlite", quyết định dùng loại cơ sở dữ liệu nào
	MySQL  *MySQLConfig  `json:"mysql,omitempty"`
	SQLite *SQLiteConfig `json:"sqlite,omitempty"`
}

// GetStorageType lấy loại lưu trữ đang được cấu hình hiện tại
func (c *DatabaseConfig) GetStorageType() string {
	if c.Type == "sqlite" || c.Type == "mysql" {
		return c.Type
	}
	// Khi chưa thiết lập type, suy luận dựa trên cấu hình hiện có
	if c.SQLite != nil {
		return "sqlite"
	}
	if c.MySQL != nil {
		return "mysql"
	}
	return "mysql"
}

// MySQLConfig cấu hình cơ sở dữ liệu MySQL
type MySQLConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// SQLiteConfig cấu hình cơ sở dữ liệu SQLite
type SQLiteConfig struct {
	FilePath string `json:"file_path"` // Đường dẫn file cơ sở dữ liệu, ví dụ ./data/milestones.db
}

type JWTConfig struct {
	Secret     string `json:"secret"`
	ExpireHour int    `json:"expire_hour"`
}

type SpeakerServiceConfig struct {
	URL string `json:"url"` // Địa chỉ dịch vụ của asr_server
}

type StorageConfig struct {
	SpeakerAudioPath string `json:"speaker_audio_path"` // Đường dẫn lưu trữ file âm thanh
	MaxFileSize      int64  `json:"max_file_size"`      // Kích thước file tối đa (byte), mặc định 10MB
}

type HistoryConfig struct {
	Enabled       bool   `json:"enabled"`
	AudioBasePath string `json:"audio_base_path"` // Đường dẫn gốc lưu trữ âm thanh
	MaxFileSize   int64  `json:"max_file_size"`   // Kích thước file tối đa (byte), mặc định 10MB
}

func Load() *Config {
	return LoadWithPath("config/config.json")
}

func LoadWithPath(configPath string) *Config {
	config := LoadFromFile(configPath)

	// Chỉ khi sử dụng MySQL mới đảm bảo có cấu hình MySQL và áp dụng ghi đè từ biến môi trường
	if config.Database.GetStorageType() == "mysql" {
		if config.Database.MySQL == nil {
			config.Database.MySQL = &MySQLConfig{}
		}
		if host := os.Getenv("DB_HOST"); host != "" {
			config.Database.MySQL.Host = host
		}
		if port := os.Getenv("DB_PORT"); port != "" {
			var p int
			fmt.Sscanf(port, "%d", &p)
			config.Database.MySQL.Port = p
		}
		if username := os.Getenv("DB_USER"); username != "" {
			config.Database.MySQL.Username = username
		}
		if password := os.Getenv("DB_PASSWORD"); password != "" {
			config.Database.MySQL.Password = password
		}
		if database := os.Getenv("DB_NAME"); database != "" {
			config.Database.MySQL.Database = database
		}
	}

	// Ưu tiên dùng biến môi trường để ghi đè cấu hình dịch vụ nhận diện giọng nói (speaker service)
	if serviceURL := os.Getenv("SPEAKER_SERVICE_URL"); serviceURL != "" {
		config.SpeakerService.URL = serviceURL
	}
	// Ưu tiên dùng biến môi trường để ghi đè đường dẫn lưu trữ âm thanh
	if audioBasePath := os.Getenv("AUDIO_BASE_PATH"); audioBasePath != "" {
		config.History.AudioBasePath = audioBasePath
	}

	fmt.Println("config", config)

	return config
}

func LoadFromFile(configPath string) *Config {
	file, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("không thể mở file cấu hình %s: %v", configPath, err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		log.Fatalf("phân tích file cấu hình thất bại %s: %v", configPath, err)
	}

	return &config
}