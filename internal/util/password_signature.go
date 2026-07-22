package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// GeneratePasswordSignature tạo chữ ký mật khẩu
// Dựa trên clientId + '|' + username và khóa ký để tạo chữ ký HMAC-SHA256
func GeneratePasswordSignature(data, key string) string {
	// Sử dụng HMAC-SHA256 để tạo chữ ký
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	signature := h.Sum(nil)

	// Trả về chữ ký đã mã hóa base64
	return base64.StdEncoding.EncodeToString(signature)
}

// ValidateMqttCredentials xác thực thông tin đăng nhập MQTT
// Được triển khai dựa trên logic xác thực JavaScript đã cung cấp
func ValidateMqttCredentials(clientId, username, password, signatureKey string) (*MqttCredentialInfo, error) {
	// Kiểm tra khóa ký
	if signatureKey == "" {
		return nil, fmt.Errorf("thiếu cấu hình khóa ký")
	}

	// Kiểm tra clientId
	if clientId == "" {
		return nil, fmt.Errorf("clientId phải là chuỗi không rỗng")
	}

	// Kiểm tra định dạng clientId (phải chứa dấu phân cách @@@)
	clientIdParts := strings.Split(clientId, "@@@")
	if len(clientIdParts) != 3 {
		return nil, fmt.Errorf("định dạng clientId sai, phải chứa dấu phân cách @@@")
	}

	// Kiểm tra username
	if username == "" {
		return nil, fmt.Errorf("username phải là chuỗi không rỗng")
	}

	// Thử giải mã username (phải là JSON được mã hóa base64)
	var userData map[string]interface{}
	decodedUsername, err := base64.StdEncoding.DecodeString(username)
	if err != nil {
		return nil, fmt.Errorf("username không phải là chuỗi được mã hóa base64 hợp lệ: %v", err)
	}

	if err := json.Unmarshal(decodedUsername, &userData); err != nil {
		return nil, fmt.Errorf("username không phải là JSON mã hóa base64 hợp lệ: %v", err)
	}

	// Kiểm tra chữ ký mật khẩu
	signatureData := clientId + "|" + username
	expectedSignature := GeneratePasswordSignature(signatureData, signatureKey)
	if password != expectedSignature {
		return nil, fmt.Errorf("xác thực chữ ký mật khẩu thất bại")
	}

	// Phân tích thông tin trong clientId
	groupId := clientIdParts[0]
	macAddress := strings.ReplaceAll(clientIdParts[1], "_", ":")
	uuid := clientIdParts[2]

	// Nếu xác thực thành công, trả về thông tin hữu ích đã phân tích
	return &MqttCredentialInfo{
		GroupId:    groupId,
		MacAddress: macAddress,
		UUID:       uuid,
		UserData:   userData,
	}, nil
}

// MqttCredentialInfo thông tin đăng nhập MQTT
type MqttCredentialInfo struct {
	GroupId    string                 `json:"groupId"`
	MacAddress string                 `json:"macAddress"`
	UUID       string                 `json:"uuid"`
	UserData   map[string]interface{} `json:"userData"`
}

// GenerateMqttCredentials tạo thông tin đăng nhập MQTT
// Dùng cho API OTA để tạo thông tin kết nối MQTT
func GenerateMqttCredentials(deviceId, clientId, ip, signatureKey string) (*MqttCredentials, error) {
	// Xử lý deviceId (thay dấu hai chấm bằng dấu gạch dưới)
	deviceId = strings.ReplaceAll(deviceId, ":", "_")

	// Xây dựng dữ liệu username (bao gồm thông tin IP)
	userName := struct {
		Ip string `json:"ip"`
	}{
		Ip: ip,
	}
	userNameJson, err := json.Marshal(userName)
	if err != nil {
		return nil, fmt.Errorf("chuyển đổi username sang JSON thất bại: %v", err)
	}
	base64UserName := base64.StdEncoding.EncodeToString(userNameJson)

	// Xây dựng clientId, định dạng: GID_test@@@deviceId@@@clientId
	mqttClientId := fmt.Sprintf("GID_test@@@%s@@@%s", deviceId, clientId)

	// Tạo chữ ký mật khẩu
	var pwd string
	if signatureKey != "" {
		// Sử dụng khóa ký để tạo mật khẩu
		signatureData := mqttClientId + "|" + base64UserName
		pwd = GeneratePasswordSignature(signatureData, signatureKey)
	} else {
		// Nếu không cấu hình khóa ký, dùng logic cũ làm phương án dự phòng
		pwd = Sha256Digest([]byte(mqttClientId))
	}

	return &MqttCredentials{
		ClientId: mqttClientId,
		Username: base64UserName,
		Password: pwd,
	}, nil
}

// MqttCredentials thông tin đăng nhập MQTT
type MqttCredentials struct {
	ClientId string `json:"client_id"`
	Username string `json:"username"`
	Password string `json:"password"`
}