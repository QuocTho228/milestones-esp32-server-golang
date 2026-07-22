package chat

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/viper"

	"milestones-esp32-server-golang/internal/domain/llm"
	log "milestones-esp32-server-golang/logger"
)

func HandleVllm(deviceId string, file []byte, text string) (string, error) {
	// Sử dụng VLLM Provider của deviceId.
	provider := viper.GetString("vision.vllm.provider")
	vllmConfig := viper.GetStringMap(fmt.Sprintf("vision.vllm.%s", provider))

	mimeType := http.DetectContentType(file[:512])

	llmProvider, err := llm.GetLLMProvider(provider, vllmConfig)
	if err != nil {
		log.Errorf("Không thể lấy VLLM Provider: %v", err)
		return "", err
	}
	responseText, err := llmProvider.ResponseWithVllm(context.Background(), file, text, mimeType)
	if err != nil {
		log.Errorf("Nhận dạng hình ảnh thất bại: %v", err)
		return "", err
	}

	return responseText, nil
}

func GetVisionUrl() string {
	return viper.GetString("vision.vision_url")
}

func GenToken(clientId string) string {
	return clientId + "_" + time.Now().Format("20060102150405")
}

func VisvionAuth(token string) error {
	return nil
}
