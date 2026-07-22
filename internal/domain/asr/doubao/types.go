package doubao

import "strings"

const (
	legacyDoubaoNonstreamPath = "bigmodel_nostream"
	doubaoStreamingPath       = "bigmodel_async"
)

// DoubaoV2Config struct cấu hình ASR Doubao
type DoubaoV2Config struct {
	AppID             string // ID ứng dụng
	AccessToken       string // Token truy cập
	WsURL             string // WebSocket URL
	ResourceID        string // ID tài nguyên
	ModelName         string // Tên mô hình
	EndWindowSize     int    // Kích thước cửa sổ kết thúc
	EnablePunc        bool   // Có bật dấu câu (punctuation) hay không
	EnableITN         bool   // Có bật ITN hay không
	EnableDDC         bool   // Có bật DDC hay không
	ResultType        string // Chế độ trả kết quả
	ShowUtterances    bool   // Có trả về thông tin phân câu (utterance) hay không
	ForceToSpeechTime int    // Thời lượng tối thiểu trước khi bắt buộc chuyển sang nhận dạng giọng nói
	EnableNonstream   bool   // Có bật phiên bản tối ưu streaming hai chiều hay không
	ChunkDuration     int    // Thời lượng mỗi chunk (mili giây)
	Timeout           int    // Thời gian timeout (giây)
}

// DefaultConfig cấu hình mặc định
var DefaultConfig = DoubaoV2Config{
	WsURL:             "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async",
	ResourceID:        "volc.bigasr.sauc.duration",
	ModelName:         "bigmodel",
	EndWindowSize:     800,
	EnablePunc:        true,
	EnableITN:         true,
	EnableDDC:         false,
	ResultType:        "full",
	ShowUtterances:    true,
	ForceToSpeechTime: 1000,
	EnableNonstream:   false,
	ChunkDuration:     200,
	Timeout:           30,
}

func normalizeDoubaoWsURL(wsURL string) string {
	if wsURL == "" || !strings.Contains(wsURL, legacyDoubaoNonstreamPath) {
		return wsURL
	}
	return strings.ReplaceAll(wsURL, legacyDoubaoNonstreamPath, doubaoStreamingPath)
}

// DoubaoV2Request struct request ASR Doubao
type DoubaoV2Request struct {
	User struct {
		UID string `json:"uid"`
	} `json:"user"`
	Audio struct {
		Format   string `json:"format"`
		Rate     int    `json:"rate"`
		Bits     int    `json:"bits"`
		Channel  int    `json:"channel"`
		Language string `json:"language"`
	} `json:"audio"`
	Request struct {
		ModelName         string `json:"model_name"`
		EndWindowSize     int    `json:"end_window_size"`
		EnablePunc        bool   `json:"enable_punc"`
		EnableITN         bool   `json:"enable_itn"`
		EnableDDC         bool   `json:"enable_ddc"`
		ResultType        string `json:"result_type"`
		ShowUtterances    bool   `json:"show_utterances"`
		ForceToSpeechTime int    `json:"force_to_speech_time"`
		EnableNonstream   bool   `json:"enable_nonstream"`
	} `json:"request"`
}

// DoubaoV2Response struct response ASR Doubao
type DoubaoV2Response struct {
	Code   int `json:"code"`
	Result struct {
		Text string `json:"text"`
	} `json:"result,omitempty"`
	Error string `json:"error,omitempty"`
}