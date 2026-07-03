package asr

import (
	"context"
	"fmt"

	"milestones-esp32-server-golang/constants"
	"milestones-esp32-server-golang/internal/domain/asr/doubao"
	"milestones-esp32-server-golang/internal/domain/asr/types"
	log "milestones-esp32-server-golang/logger"
)

// Asr interface nhận dạng giọng nói
type AsrProvider interface {
	// Process xử lý toàn bộ đoạn âm thanh một lần, trả về kết quả nhận dạng hoàn chỉnh
	Process(pcmData []float32) (string, error)

	// StreamingRecognize interface nhận dạng dạng streaming
	// Dữ liệu âm thanh đầu vào được truyền qua channel audioStream, kết quả nhận dạng được lấy qua channel trả về
	// Khi audioStream bị đóng, nghĩa là đầu vào đã kết thúc, kết quả cuối cùng sẽ được gửi qua channel trả về, sau đó channel này sẽ được đóng
	// Có thể dùng ctx để kiểm soát việc hủy (cancel) và timeout của quá trình nhận dạng
	StreamingRecognize(ctx context.Context, audioStream <-chan []float32) (chan types.StreamingResult, error)
	// Close đóng tài nguyên, giải phóng kết nối, v.v.
	Close() error
	// IsValid kiểm tra tài nguyên có hợp lệ hay không
	IsValid() bool
}

// NewAsrProvider tạo một instance ASR mới
// asrType: loại engine ASR, hiện hỗ trợ "funasr"
// config: cấu hình engine ASR, kiểu map[string]interface{}
func NewAsrProvider(asrType string, config map[string]interface{}) (AsrProvider, error) {
	// Ưu tiên sử dụng provider trong config, nếu không có thì dùng provider truyền vào tham số
	if configProvider, ok := config["provider"].(string); ok && configProvider != "" {
		asrType = configProvider
	}
	switch asrType {
	case constants.AsrTypeFunAsr:
		return NewFunasrAdapter(config)
	case constants.AsrTypeAliyunFunASR:
		return NewAliyunFunASRAdapter(config)
	case constants.AsrTypeDoubao:
		log.Info("Sử dụng nhà cung cấp ASR Doubao (豆包)")
		provider, err := doubao.NewDoubaoV2Adapter(config)
		if err != nil {
			log.Errorf("Khởi tạo adapter ASR Doubao thất bại: %v", err)
		} else {
			log.Info("Khởi tạo adapter ASR Doubao thành công")
		}
		return provider, err
	case constants.AsrTypeAliyunQwen3:
		log.Info("Sử dụng nhà cung cấp ASR Aliyun Qwen3")
		provider, err := NewAliyunQwen3Adapter(config)
		if err != nil {
			log.Errorf("Khởi tạo adapter ASR Aliyun Qwen3 thất bại: %v", err)
		} else {
			log.Info("Khởi tạo adapter ASR Aliyun Qwen3 thành công")
		}
		return provider, err
	case constants.AsrTypeXunfei:
		log.Info("Sử dụng nhà cung cấp ASR Xunfei (讯飞)")
		provider, err := NewXunfeiAdapter(config)
		if err != nil {
			log.Errorf("Khởi tạo adapter ASR Xunfei thất bại: %v", err)
		} else {
			log.Info("Khởi tạo adapter ASR Xunfei thành công")
		}
		return provider, err
	default:
		return nil, fmt.Errorf("Không hỗ trợ loại engine ASR: %s, hiện chỉ hỗ trợ 'funasr', 'aliyun_funasr', 'doubao', 'aliyun_qwen3', 'xunfei'", asrType)
	}
}