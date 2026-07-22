package mqtt_server

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"encoding/json"

	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"

	mqttServer "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/spf13/viper"
)

// AuthHook triển khai logic xác thực (auth) tùy chỉnh
// Hỗ trợ người dùng thông thường và quản trị viên cấp cao (super admin)
// Người dùng thông thường: tên đăng nhập là chuỗi base64 của {"ip":"1.202.193.194"}, mật khẩu là chữ ký HMAC-SHA256
// Quản trị viên cấp cao: tên đăng nhập admin, mật khẩu shijingbo!@#
type AuthHook struct {
	mqttServer.HookBase
}

func (h *AuthHook) ID() string {
	return "custom-auth-hook"
}

func (h *AuthHook) Provides(b byte) bool {
	return b == mqttServer.OnConnectAuthenticate
}

func (h *AuthHook) OnConnectAuthenticate(cl *mqttServer.Client, pk packets.Packet) bool {
	// Kiểm tra xem xác thực có được bật hay không
	enableAuth := viper.GetBool("mqtt_server.enable_auth")
	if !enableAuth {
		//log.Infof("Xác thực MQTT đã bị vô hiệu hóa, cho phép tất cả các kết nối")
		return true
	}

	username := string(pk.Connect.Username)
	password := string(pk.Connect.Password)
	clientId := string(pk.Connect.ClientIdentifier)

	// Kiểm tra thông tin quản trị viên cấp cao
	adminUsername := configuredAdminUsername()
	adminPassword := configuredAdminPassword()
	if username == adminUsername && password == adminPassword {
		log.Infof("Quản trị viên cấp cao đăng nhập thành công: %s", username)
		return true
	}
	if username == adminUsername {
		log.Warnf("Quản trị viên MQTT đăng nhập thất bại: username=%s, clientId=%s, nguyên nhân=sai mật khẩu", username, clientId)
		return false
	}

	// Kiểm tra người dùng thông thường - sử dụng logic xác minh chữ ký mới
	signatureKey := viper.GetString("mqtt_server.signature_key")
	if signatureKey != "" {
		credentialInfo, err := util.ValidateMqttCredentials(clientId, username, password, signatureKey)
		//log.Infof("Bắt đầu xác minh người dùng MQTT: clientId=%s, username=%s, password=%s, signatureKey=%s",
		//	clientId, username, password, signatureKey)
		//log.Infof("Bắt đầu xác minh người dùng MQTT: credentialInfo=%+v", credentialInfo)

		if err != nil {
			log.Warnf("Xác minh thông tin đăng nhập MQTT thất bại: username=%s, clientId=%s, err=%v", username, clientId, err)
			return false
		}

		log.Infof("Xác minh người dùng MQTT thành công: groupId=%s, macAddress=%s, uuid=%s",
			credentialInfo.GroupId, credentialInfo.MacAddress, credentialInfo.UUID)
		return true
	}

	// Nếu không cấu hình khóa chữ ký, quay lại sử dụng logic xác minh AES cũ
	log.Warnf("Thiếu cấu hình khóa chữ ký OTA, sử dụng phương thức xác minh AES")
	return h.validateWithAes(username, password)
}

// validateWithAes xác minh mật khẩu bằng phương thức AES (tương thích ngược)
func (h *AuthHook) validateWithAes(username, password string) bool {
	// Kiểm tra người dùng thông thường
	decoded, err := base64.StdEncoding.DecodeString(username)
	if err != nil {
		return false
	}
	var userInfo map[string]string
	if err := json.Unmarshal(decoded, &userInfo); err != nil {
		return false
	}
	if _, ok := userInfo["ip"]; !ok {
		return false
	}
	// Kiểm tra xem password có phải là username đã được mã hóa AES hay không
	if !checkAesPassword(username, password) {
		return false
	}
	return true
}

// checkAesPassword kiểm tra xem password có phải là base64(username) sau khi mã hóa AES-ECB hay không
func checkAesPassword(username, password string) bool {
	key := []byte("milestones_aes_key_1") // Khóa 16 byte, khuyến nghị nên đưa ra cấu hình thực tế
	ciphertext, err := aesEncryptECB([]byte(username), key)
	if err != nil {
		return false
	}
	cipherBase64 := base64.StdEncoding.EncodeToString(ciphertext)
	return cipherBase64 == password
}

// aesEncryptECB triển khai mã hóa AES-ECB
func aesEncryptECB(src, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	// Đệm (padding) theo chuẩn PKCS7
	padding := blockSize - len(src)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	src = append(src, padtext...)
	encrypted := make([]byte, len(src))
	for bs, be := 0, blockSize; bs < len(src); bs, be = bs+blockSize, be+blockSize {
		block.Encrypt(encrypted[bs:be], src[bs:be])
	}
	return encrypted, nil
}