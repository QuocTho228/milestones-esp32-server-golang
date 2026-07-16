package websocket

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"milestones-esp32-server-golang/internal/data/client"
	user_config "milestones-esp32-server-golang/internal/domain/config"
	ctypes "milestones-esp32-server-golang/internal/domain/config/types"
	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"

	"github.com/spf13/viper"
)

type ActivationRequest struct {
	Payload ctypes.ActivationPayload `json:"Payload"`
}

func (s *WebSocketServer) handleOta(w http.ResponseWriter, r *http.Request) {
	//Lấy IP của client
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}

	//Lấy Device-Id và Client-Id từ header
	deviceId := r.Header.Get("Device-Id")
	clientId := r.Header.Get("Client-Id")

	if deviceId == "" || clientId == "" {
		log.Errorf("Device-Id hoặc Client-Id bị thiếu")
		http.Error(w, "Device-Id hoặc Client-Id bị thiếu", http.StatusBadRequest)
		return
	}

	//deviceId = strings.ReplaceAll(deviceId, ":", "_")

	//Chọn cấu hình khác nhau dựa trên IP
	clientIp := r.Header.Get("X-Real-IP")
	if clientIp == "" {
		clientIp = r.Header.Get("X-Forwarded-For")
	}
	if clientIp == "" {
		clientIp = r.RemoteAddr
	}

	var activationInfo *ActivationInfo
	authEnable := viper.GetBool("auth.enable")
	log.Debugf("authEnable: %v", authEnable)
	if authEnable {
		configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
		//Kiểm tra xem deviceId này đã được xác thực hay chưa
		isActivited, err := configProvider.IsDeviceActivated(r.Context(), deviceId, clientId)
		if err != nil {
			log.Errorf("Kiểm tra xác thực thiết bị thất bại: %v", err)
			http.Error(w, "Lỗi máy chủ nội bộ", http.StatusInternalServerError)
			return
		}
		if !isActivited {
			code, challenge, msg, timeoutMs := configProvider.GetActivationInfo(r.Context(), deviceId, clientId)
			activationInfo = &ActivationInfo{
				Code:      code,
				Message:   msg,
				Challenge: challenge,
				TimeoutMs: timeoutMs,
			}
			log.Infof("Thông tin kích hoạt: &{Code:%s Message:%s Challenge:%s TimeoutMs:%d}", code, msg, challenge, timeoutMs)
		}
	}

	otaConfigPrefix := "ota.external."
	//Nếu IP bắt đầu bằng 192.168 thì chọn cấu hình test
	if strings.HasPrefix(clientIp, "192.168") || strings.HasPrefix(clientIp, "10.") || strings.HasPrefix(clientIp, "127.0.0.1") {
		otaConfigPrefix = "ota.test."
	} else {
		otaConfigPrefix = "ota.external."
	}

	mqttInfo := getMqttInfo(deviceId, clientId, otaConfigPrefix, ip)
	//Mật khẩu
	respData := &OtaResponse{
		Websocket: WebsocketInfo{
			Url:   viper.GetString(otaConfigPrefix + "websocket.url"),
			Token: viper.GetString(otaConfigPrefix + "websocket.token"),
		},
		Mqtt: mqttInfo,
		ServerTime: ServerTimeInfo{
			Timestamp:      time.Now().UnixMilli(),
			TimezoneOffset: 480,
		},
		Activation: activationInfo,
		Firmware: FirmwareInfo{
			Version: "0.9.9",
			Url:     "",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(respData); err != nil {
		log.Errorf("Serialize phản hồi OTA thất bại: %v", err)
		http.Error(w, "Lỗi máy chủ nội bộ", http.StatusInternalServerError)
		return
	}
	return
}

func getMqttInfo(deviceId, clientId, otaConfigPrefix, ip string) *MqttInfo {
	if !viper.GetBool(otaConfigPrefix + "mqtt.enable") {
		return nil
	}

	// Tạo thông tin xác thực (credentials) cho MQTT
	signatureKey := viper.GetString("ota.signature_key")
	credentials, err := util.GenerateMqttCredentials(deviceId, clientId, ip, signatureKey)
	if err != nil {
		log.Errorf("Tạo thông tin xác thực MQTT thất bại: %v", err)
		return nil
	}

	return &MqttInfo{
		Endpoint:       viper.GetString(otaConfigPrefix + "mqtt.endpoint"),
		ClientId:       credentials.ClientId,
		Username:       credentials.Username,
		Password:       credentials.Password,
		PublishTopic:   client.DeviceMockPubTopicPrefix,
		SubscribeTopic: client.DeviceMockSubTopicPrefix,
	}
}

// handleOtaActivate API kích hoạt thiết bị
func (s *WebSocketServer) handleOtaActivate(w http.ResponseWriter, r *http.Request) {
	deviceId := r.Header.Get("Device-Id")
	clientId := r.Header.Get("Client-Id")
	if deviceId == "" || clientId == "" {
		log.Errorf("Device-Id hoặc Client-Id bị thiếu")
		http.Error(w, "Device-Id hoặc Client-Id bị thiếu", http.StatusBadRequest)
		return
	}
	var req ActivationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Errorf("Giải mã yêu cầu kích hoạt thất bại: %v", err)
		http.Error(w, "Giải mã nội dung yêu cầu thất bại", http.StatusBadRequest)
		return
	}
	// Kiểm tra thuật toán
	if req.Payload.Algorithm != "hmac-sha256" {
		http.Error(w, "Thuật toán không được hỗ trợ", http.StatusBadRequest)
		return
	}

	// Gọi Config Provider để kiểm tra liên kết (binding)
	configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		log.Errorf("Lấy Config Provider thất bại: %v", err)
		http.Error(w, "Lỗi máy chủ nội bộ", http.StatusInternalServerError)
		return
	}
	ok, err := configProvider.VerifyChallenge(r.Context(), deviceId, clientId, req.Payload)
	if err != nil {
		log.Errorf("Kiểm tra kích hoạt thiết bị thất bại: %v", err)
		http.Error(w, "Kiểm tra kích hoạt thiết bị thất bại", http.StatusInternalServerError)
		return
	}
	if !ok {
		log.Warnf("Kiểm tra kích hoạt thiết bị không đạt: deviceId=%s, clientId=%s", deviceId, clientId)
		http.Error(w, "Kiểm tra kích hoạt thiết bị không đạt", http.StatusAccepted)
		return
	}
	// Kích hoạt thành công, trả về 200
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Kích hoạt thành công"))
}