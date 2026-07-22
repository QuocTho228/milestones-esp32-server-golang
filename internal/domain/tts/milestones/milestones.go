package milestones

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "milestones-esp32-server-golang/logger"

	"github.com/gorilla/websocket"
)

var deviceIdList = []string{
	"ba:8f:17:de:94:94",
	"f2:85:44:27:7b:51",
	"4f:57:fb:d4:69:fa",
	"b3:1e:1c:80:cc:78",
	"32:a5:cc:b7:c0:e4",
	"2b:60:6a:5a:72:10",
	"ca:a6:8b:20:f1:6f",
	"26:1a:d7:27:9f:f8",
	"03:02:26:58:2b:06",
	"5f:f3:85:8b:5d:da",
}

// Ghi nhận danh sách các deviceId gần đây bị lỗi và thời gian hết hạn cấm sử dụng
var (
	deviceIdBlocklist     = make(map[string]time.Time)
	deviceIdBlocklistLock sync.Mutex
	// Thời gian cấm sử dụng thiết bị (khoảng thời gian không sử dụng sau khi bị lỗi)
	deviceIdBlockDuration = 5 * time.Second
)

// MilestonesProvider là Provider WebSocket chuyển văn bản thành giọng nói (TTS) của Milestones
// Hỗ trợ chuyển văn bản thành giọng nói theo dạng stream (luồng)
type MilestonesProvider struct {
	ServerAddr  string
	DeviceID    string
	AudioFormat map[string]interface{}
	Header      http.Header
}

// Định kỳ dọn dẹp danh sách deviceId bị cấm đã hết hạn
func init() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Dọn dẹp danh sách deviceId bị cấm đã hết hạn
			deviceIdBlocklistLock.Lock()
			now := time.Now()
			for id, expireTime := range deviceIdBlocklist {
				if now.After(expireTime) {
					delete(deviceIdBlocklist, id)
					log.Debugf("Lệnh cấm deviceId đã hết hạn, kích hoạt lại: %s", id)
				}
			}
			deviceIdBlocklistLock.Unlock()
		}
	}()
}

// Thêm deviceId vào danh sách bị cấm
func blockDeviceId(deviceId string) {
	deviceIdBlocklistLock.Lock()
	defer deviceIdBlocklistLock.Unlock()

	deviceIdBlocklist[deviceId] = time.Now().Add(deviceIdBlockDuration)
	log.Warnf("Device ID %s đã được thêm vào danh sách cấm, sẽ được kích hoạt lại sau %v", deviceId, deviceIdBlockDuration)
}

// Kiểm tra xem deviceId có nằm trong danh sách bị cấm hay không
func isDeviceIdBlocked(deviceId string) bool {
	deviceIdBlocklistLock.Lock()
	defer deviceIdBlocklistLock.Unlock()

	expireTime, exists := deviceIdBlocklist[deviceId]
	if !exists {
		return false
	}

	// Nếu thời gian hết hạn đã qua thì xóa khỏi danh sách cấm
	if time.Now().After(expireTime) {
		delete(deviceIdBlocklist, deviceId)
		log.Debugf("Lệnh cấm deviceId đã hết hạn, kích hoạt lại: %s", deviceId)
		return false
	}

	return true
}

// NewMilestonesProvider tạo mới một Provider TTS Milestones
func NewMilestonesProvider(config map[string]interface{}) *MilestonesProvider {
	serverAddr, _ := config["server_addr"].(string)
	deviceID, _ := config["device_id"].(string)
	clientID, _ := config["client_id"].(string)
	token, _ := config["token"].(string)
	format := map[string]interface{}{
		"sample_rate":    16000,
		"channels":       1,
		"frame_duration": 20,
		"format":         "opus",
	}

	header := http.Header{}
	header.Set("Device-Id", deviceID)
	header.Set("Content-Type", "application/json")
	header.Set("Authorization", "Bearer "+token)
	header.Set("Protocol-Version", "1")
	header.Set("Client-Id", clientID)

	return &MilestonesProvider{
		ServerAddr:  serverAddr,
		DeviceID:    deviceID,
		AudioFormat: format,
		Header:      header,
	}
}

// selectDeviceId chọn một deviceId khả dụng
func (p *MilestonesProvider) selectDeviceId() string {
	// Tìm trong deviceIdList một deviceId chưa bị cấm
	for _, deviceId := range deviceIdList {
		if !isDeviceIdBlocked(deviceId) {
			log.Debugf("Chọn device ID chưa bị cấm: %s", deviceId)
			return deviceId
		}
	}

	// Nếu tất cả deviceId đều bị cấm, chọn theo vòng xoay (round-robin) từ toàn bộ danh sách
	if len(deviceIdList) > 0 {
		// Sử dụng chiến lược round-robin đơn giản (dựa trên thời gian)
		selectedIndex := int(time.Now().Unix()) % len(deviceIdList)
		selectedDeviceId := deviceIdList[selectedIndex]
		log.Warnf("Tất cả deviceId đều bị cấm, chọn theo vòng xoay device ID: %s (chỉ số: %d)", selectedDeviceId, selectedIndex)
		return selectedDeviceId
	}

	// Nếu deviceIdList trống, sử dụng deviceId được truyền vào
	if p.DeviceID != "" {
		log.Warnf("deviceIdList trống, sử dụng device ID hiện tại: %s", p.DeviceID)
		return p.DeviceID
	}

	// Nếu không có gì cả, trả về deviceId đầu tiên (nếu tồn tại)
	if len(deviceIdList) > 0 {
		return deviceIdList[0]
	}

	return ""
}

// createWSConnection tạo một kết nối WebSocket mới
func (p *MilestonesProvider) createWSConnection(ctx context.Context) (*websocket.Conn, string, error) {
	// Chọn một deviceId khả dụng
	selectedDeviceId := p.selectDeviceId()
	if selectedDeviceId == "" {
		return nil, "", fmt.Errorf("không thể chọn được device ID")
	}

	// Cập nhật p.DeviceID và Header hiện tại
	p.DeviceID = selectedDeviceId
	p.Header.Set("Device-Id", selectedDeviceId)

	// Tạo kết nối mới
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, p.ServerAddr, p.Header)
	if err != nil {
		log.Errorf("Tạo kết nối WebSocket thất bại: %v, device ID: %s", err, selectedDeviceId)
		blockDeviceId(selectedDeviceId) // Thêm deviceId bị lỗi vào danh sách cấm
		return nil, "", err
	}

	// Thiết lập giữ kết nối (keep-alive)
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
	})

	// Gửi thông điệp hello khi tạo kết nối mới
	helloMsg := map[string]interface{}{
		"type":         "hello",
		"device_id":    selectedDeviceId,
		"transport":    "websocket",
		"version":      1,
		"audio_params": p.AudioFormat,
	}
	log.Debugf("Tạo kết nối mới và gửi thông điệp hello, device ID: %s", selectedDeviceId)
	if err := conn.WriteJSON(helloMsg); err != nil {
		conn.Close()
		return nil, "", fmt.Errorf("gửi thông điệp hello thất bại: %v", err)
	}

	return conn, selectedDeviceId, nil
}

type RecvMsg struct {
	Type    string `json:"type"`
	State   string `json:"state"`
	Text    string `json:"text"`
	Version int    `json:"version"`
}

// sendStopMessage gửi thông điệp stop và đóng kết nối
func sendStopMessage(conn *websocket.Conn, deviceId string) {
	stopMsg := map[string]interface{}{
		"type":      "listen",
		"device_id": deviceId,
		"state":     "stop",
	}
	if err := conn.WriteJSON(stopMsg); err != nil {
		log.Warnf("Gửi thông điệp stop thất bại: %v, device ID: %s", err, deviceId)
	} else {
		log.Debugf("Gửi thông điệp stop thành công, device ID: %s", deviceId)
	}
}

// handleTTSConnection đóng gói logic lấy kết nối, gửi thông điệp và nhận thông điệp
func (p *MilestonesProvider) handleTTSConnection(ctx context.Context, text string, outputChan chan []byte) error {
	// Tạo kết nối mới
	conn, deviceId, err := p.createWSConnection(ctx)
	if err != nil {
		return fmt.Errorf("tạo kết nối TTS Milestones thất bại: %v", err)
	}
	defer func() {
		// Gửi thông điệp stop và đóng kết nối
		sendStopMessage(conn, deviceId)
		conn.Close()
	}()

	// Gửi thông điệp listen detect
	sendText := fmt.Sprintf("`%s`", text)
	listenMsg := map[string]interface{}{
		"type":      "listen",
		"device_id": deviceId,
		"state":     "detect",
		"text":      sendText,
	}
	log.Debugf("Gửi thông điệp đến server milestones: %v", listenMsg)

	if err := conn.WriteJSON(listenMsg); err != nil {
		log.Errorf("Gửi thông điệp listen thất bại: %v, device ID: %s", err, deviceId)
		blockDeviceId(deviceId) // Thêm deviceId bị lỗi vào danh sách cấm
		return fmt.Errorf("gửi thông điệp thất bại: %v", err)
	}

	// Đọc và xử lý thông điệp
	startTs := time.Now().UnixMilli()
	var firstFrameTs bool
	i := 0
	receivedFrames := false

	for {
		select {
		case <-ctx.Done():
			log.Debugf("ctx.Done() khi nhận thông điệp từ server milestones, device ID: %s", deviceId)
			return nil
		default:
		}
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			// Kết nối bị lỗi
			log.Errorf("Lỗi khi đọc thông điệp: %v, device ID: %s", err, deviceId)

			// Nếu chưa nhận được bất kỳ khung âm thanh nào, nghĩa là kết nối có thể có vấn đề, thêm deviceId vào danh sách cấm
			if !receivedFrames {
				blockDeviceId(deviceId)
			}

			return fmt.Errorf("lỗi khi đọc thông điệp: %v", err)
		}
		if msgType == websocket.TextMessage {
			log.Debugf("Nhận được thông điệp từ server milestones: %s", string(msg))
			var recvMsg RecvMsg
			err := json.Unmarshal(msg, &recvMsg)
			if err != nil {
				continue
			}
			if recvMsg.Type == "tts" {
				if recvMsg.State == "stop" {
					log.Debugf("Nhận được thông điệp tts stop từ server milestones")
					return nil
				}
			}
		} else if msgType == websocket.BinaryMessage {
			receivedFrames = true
			if !firstFrameTs {
				firstFrameTs = true
				log.Debugf("Thống kê thời gian TTS: thời điểm nhận khung âm thanh đầu tiên từ server milestones: %d", time.Now().UnixMilli()-startTs)
			}
			outputChan <- msg
			if i%20 == 0 {
				log.Debugf("Thông điệp âm thanh từ server milestones, đã nhận %d khung âm thanh", i)
			}
			i++
		}
	}
}

// TextToSpeechStream triển khai TTS dạng stream, trả về channel chứa các khung âm thanh opus
func (p *MilestonesProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (chan []byte, error) {
	outputChan := make(chan []byte, 1000)

	// Thử xử lý kết nối TTS, hỗ trợ thử lại (retry)
	go func() {
		defer close(outputChan)

		retryCount := 0
		maxRetries := 2
		var lastError error

		// Thử tối đa maxRetries lần
		for retryCount <= maxRetries {
			if retryCount > 0 {
				log.Infof("Đang thử lấy lại kết nối, lần thử lại %d/%d", retryCount, maxRetries)

				// Kiểm tra context đã bị hủy trước khi thử lại chưa
				select {
				case <-ctx.Done():
					log.Debugf("Context đã bị hủy, dừng việc thử lại")
					return
				default:
					// Tiếp tục thử lại
				}
			}

			// Xử lý kết nối TTS
			err := p.handleTTSConnection(ctx, text, outputChan)

			if err == nil {
				// Xử lý kết nối thành công, không cần thử lại
				return
			}

			lastError = err
			log.Errorf("Xử lý kết nối TTS thất bại: %v (thử lại: %d/%d)", err, retryCount, maxRetries)

			retryCount++
		}

		if retryCount > maxRetries {
			log.Warnf("Đã đạt số lần thử lại tối đa %d, bỏ cuộc, lỗi cuối cùng: %v", maxRetries, lastError)
		}
	}()

	return outputChan, nil
}

// GetVoiceInfo lấy thông tin cấu hình TTS
func (p *MilestonesProvider) GetVoiceInfo() map[string]interface{} {
	return map[string]interface{}{
		"type":         "milestones_ws",
		"server_addr":  p.ServerAddr,
		"device_id":    p.DeviceID,
		"audio_format": p.AudioFormat,
	}
}

// SetVoice thiết lập tham số giọng nói (Milestones Provider không hỗ trợ thiết lập giọng nói động)
func (p *MilestonesProvider) SetVoice(voiceConfig map[string]interface{}) error {
	return fmt.Errorf("Milestones TTS Provider không hỗ trợ thiết lập giọng nói động")
}

// Close đóng tài nguyên (Provider không trạng thái, không cần đóng)
func (p *MilestonesProvider) Close() error {
	return nil
}

// IsValid kiểm tra tài nguyên có hợp lệ hay không
func (p *MilestonesProvider) IsValid() bool {
	return p != nil
}

// TextToSpeech triển khai interface BaseTTSProvider, gộp trực tiếp các khung dạng stream
func (p *MilestonesProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	ch, err := p.TextToSpeechStream(ctx, text, sampleRate, channels, frameDuration)
	if err != nil {
		return nil, err
	}
	var frames [][]byte
	for frame := range ch {
		frames = append(frames, frame)
	}
	return frames, nil
}