package types

const (
	EmptyReasonNone               = ""
	EmptyReasonNoServerResponse   = "no_server_response"
	EmptyReasonProviderEmptyFinal = "provider_empty_final"

	RetryReasonNone                           = ""
	RetryReasonDoubaoResponseCode45000081     = "doubao_response_code_45000081"
	RetryReasonDoubaoWaitingNextPacketTimeout = "doubao_waiting_next_packet_timeout"
	RetryReasonXunfeiServiceInstanceInvalid   = "xunfei_service_instance_invalid"
	RetryReasonAliyunQwen3ConnectionClosed    = "aliyun_qwen3_connection_closed"
)

// StreamingResult: Kết quả nhận dạng theo luồng.
type StreamingResult struct {
	Text        string // Văn bản được nhận dạng.
	IsFinal     bool   // Có phải kết quả cuối cùng hay không.
	Error       error  // Thông tin lỗi.
	AsrType     string // Loại ASR.
	Mode        string // Chế độ.
	EmptyReason string // Lý do kết quả rỗng; chỉ dùng khi Text rỗng để phân biệt kết quả rỗng từ phía nguồn hoặc do xử lý không tạo ra kết quả.
	RetryReason string // Lý do có thể thử lại; chỉ dùng khi cần giải phóng tài nguyên hiện tại và thực hiện lại.
}
