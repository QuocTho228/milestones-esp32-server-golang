package eino_llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	log "milestones-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
)

func (p *EinoLLMProvider) ResponseWithVllm(ctx context.Context, file []byte, text string, mimeType string) (string, error) {
	log.Infof("[Eino-LLM] Bắt đầu thực hiện request VLLM - MIMEType: %s, độ dài file: %d", mimeType, len(file))

	// Mã hóa file hình ảnh bằng base64, ghép thành data url
	base64Str := base64.StdEncoding.EncodeToString(file)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Str)

	msg := &schema.Message{
		Role: schema.User,
		MultiContent: []schema.ChatMessagePart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: text,
			},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				ImageURL: &schema.ChatMessageImageURL{
					URL: dataURL,
				},
			},
		},
	}

	dialogue := []*schema.Message{
		&schema.Message{
			Role:    schema.System,
			Content: "Bạn là chuyên gia nhận diện hình ảnh chuyên nghiệp, hãy dựa vào nội dung hình ảnh để trả lời câu hỏi của người dùng bằng tiếng Việt.",
		},
		msg,
	}
	responseChan := p.ResponseWithContext(ctx, "", dialogue, []*schema.ToolInfo{})
	if responseChan == nil {
		log.Errorf("[Eino-VLLM] Gọi xử lý request API thị giác (vision api) thất bại - responseChan là nil")
		return "", fmt.Errorf("gọi xử lý request API thị giác (vision api) thất bại - responseChan là nil")
	}

	var result bytes.Buffer
	for {
		select {
		case <-ctx.Done():
			log.Errorf("[Eino-VLLM] context done")
			return "", nil
		case response, ok := <-responseChan:
			if !ok {
				if response != nil && response.Content != "" {
					result.WriteString(response.Content)
				}
				responseText := result.String()
				return responseText, nil
			}
			result.WriteString(response.Content)
		}
	}
}