package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config Thể hiện cấu hình server
type Config struct {
	Server struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"server"`
	MQTT struct {
		Broker   string `json:"broker"`
		ClientID string `json:"client_id"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"mqtt"`
	// Cấu hình liên quan đến từ đánh thức (wakeup word)
	WakeupWords    []string `json:"wakeup_words"`
	EnableGreeting bool     `json:"enable_greeting"`
}

// ServerAddress Trả về địa chỉ server
func (c *Config) ServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// LoadConfig Nạp cấu hình từ file
func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig Lưu cấu hình ra file
func (c *Config) SaveConfig(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}