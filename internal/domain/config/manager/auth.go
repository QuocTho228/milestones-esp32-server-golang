package manager

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"milestones-esp32-server-golang/internal/components/http"
	"milestones-esp32-server-golang/internal/domain/config/types"
	log "milestones-esp32-server-golang/logger"
)

// Cấu trúc response của HTTP API

// CheckActivationResponse Response kiểm tra trạng thái kích hoạt
type CheckActivationResponse struct {
	Activated bool   `json:"activated"`
	Message   string `json:"message"`
}

// GetActivationInfoResponse Response lấy thông tin kích hoạt
type GetActivationInfoResponse struct {
	Activated bool   `json:"activated"`
	Code      string `json:"code,omitempty"` // Đổi thành kiểu string để khớp với API backend
	Challenge string `json:"challenge,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ActivateDeviceRequest Request kích hoạt thiết bị
type ActivateDeviceRequest struct {
	DeviceId     string `json:"device_id"`
	ClientId     string `json:"client_id"`
	Code         string `json:"code"`
	Challenge    string `json:"challenge"`
	Algorithm    string `json:"algorithm"`
	SerialNumber string `json:"serial_number"`
	Hmac         string `json:"hmac"`
}

// ActivateDeviceResponse Response kích hoạt thiết bị
type ActivateDeviceResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// IsDeviceActivated Kiểm tra thiết bị đã được kích hoạt hay chưa
func (am *ConfigManager) IsDeviceActivated(ctx context.Context, deviceId string, clientId string) (bool, error) {
	// Gọi trực tiếp HTTP API của hệ thống quản trị backend
	activated, err := am.callCheckActivationAPI(ctx, deviceId, clientId)
	if err != nil {
		log.Log().Errorf("Kiểm tra trạng thái kích hoạt thiết bị %s thất bại: %v", deviceId, err)
		return false, err
	}

	log.Log().Debugf("Trạng thái kích hoạt thiết bị %s: %v", deviceId, activated)
	return activated, nil
}

// GetActivationInfo Lấy thông tin kích hoạt thiết bị
func (am *ConfigManager) GetActivationInfo(ctx context.Context, deviceId string, clientId string) (string, string, string, int) {
	// Gọi trực tiếp HTTP API của hệ thống quản trị backend
	activated, codeStr, challenge, message, err := am.callGetActivationInfoAPI(ctx, deviceId, clientId)
	if err != nil {
		log.Log().Errorf("Lấy thông tin kích hoạt thiết bị %s thất bại: %v", deviceId, err)
		return "", "", "", 0
	}

	// Nếu thiết bị đã kích hoạt, trả về ngay
	if activated {
		log.Log().Debugf("Thiết bị %s đã được kích hoạt", deviceId)
		return "", "", message, 0
	}

	// Kiểm tra Challenge có rỗng không
	if challenge == "" {
		log.Log().Errorf("Trường Challenge của thiết bị %s bị rỗng", deviceId)
		return "", "", "Trường Challenge bị rỗng, vui lòng liên hệ quản trị viên", 0
	}

	// Thiết bị chưa kích hoạt, trả về thông tin kích hoạt
	timeoutMs := 300 // Mặc định timeout 5 phút
	log.Log().Debugf("Lấy thông tin kích hoạt thiết bị %s: code=%s, challenge=%s", deviceId, codeStr, challenge)
	if codeStr == "" {
		log.Log().Warnf("Mã kích hoạt của thiết bị %s bị rỗng", deviceId)
	}

	return codeStr, challenge, message, timeoutMs
}

// VerifyChallenge Xác minh mã challenge và HMAC
func (am *ConfigManager) VerifyChallenge(ctx context.Context, deviceId string, clientId string, activationPayload types.ActivationPayload) (bool, error) {
	// Xác minh HMAC (nếu có cung cấp HMAC)
	if activationPayload.HMAC != "" {
		if !am.verifyHMAC(activationPayload.Challenge, activationPayload.HMAC) {
			log.Log().Warnf("Xác minh HMAC của thiết bị %s thất bại", deviceId)
			return false, fmt.Errorf("Xác minh HMAC thất bại")
		}
	}

	// Gọi trực tiếp API kích hoạt của hệ thống quản trị backend
	verified, err := am.callActivateDeviceAPI(ctx, deviceId, clientId, activationPayload)
	if err != nil {
		log.Log().Errorf("Kích hoạt thiết bị thất bại: %v", err)
		return false, err
	}

	if verified {
		log.Log().Infof("Xác minh kích hoạt thiết bị %s thành công", deviceId)
	}

	return verified, nil
}

// verifyHMAC Xác minh chữ ký HMAC
func (am *ConfigManager) verifyHMAC(challenge, providedHmac string) bool {
	// Có thể cấu hình khóa bí mật tùy theo nhu cầu thực tế tại đây
	// Tạm thời dùng khóa rỗng, trong thực tế nên lấy từ cấu hình
	secretKey := ""

	if secretKey == "" {
		// Nếu không cấu hình khóa, bỏ qua xác minh và cho thông qua
		return true
	}

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(challenge))
	expectedHmac := hex.EncodeToString(mac.Sum(nil))

	return expectedHmac == providedHmac
}

// Các phương thức gọi HTTP API

// callCheckActivationAPI Gọi API kiểm tra trạng thái kích hoạt
func (am *ConfigManager) callCheckActivationAPI(ctx context.Context, deviceId, clientId string) (bool, error) {
	var response CheckActivationResponse

	// Gửi HTTP request
	err := am.client.DoRequest(ctx, http.RequestOptions{
		Method: "GET",
		Path:   "/api/internal/device/check-activation",
		QueryParams: map[string]string{
			"device_id": deviceId,
			"client_id": clientId,
		},
		Response: &response,
	})
	if err != nil {
		return false, fmt.Errorf("Request thất bại: %w", err)
	}

	log.Log().Debugf("Response kiểm tra trạng thái kích hoạt: %+v", response)
	return response.Activated, nil
}

// callGetActivationInfoAPI Gọi API lấy thông tin kích hoạt
func (am *ConfigManager) callGetActivationInfoAPI(ctx context.Context, deviceId, clientId string) (bool, string, string, string, error) {
	var response GetActivationInfoResponse

	// Gửi HTTP request
	err := am.client.DoRequest(ctx, http.RequestOptions{
		Method: "GET",
		Path:   "/api/internal/device/activation-info",
		QueryParams: map[string]string{
			"device_id": deviceId,
			"client_id": clientId,
		},
		Response: &response,
	})
	if err != nil {
		return false, "", "", "", fmt.Errorf("Request thất bại: %w", err)
	}

	log.Log().Debugf("Response lấy thông tin kích hoạt: %+v", response)

	if response.Activated {
		return true, "", "", response.Message, nil
	}

	return false, response.Code, response.Challenge, response.Message, nil
}

// callActivateDeviceAPI Gọi API kích hoạt thiết bị
func (am *ConfigManager) callActivateDeviceAPI(ctx context.Context, deviceId, clientId string, activationPayload types.ActivationPayload) (bool, error) {
	// Xây dựng body của request
	request := ActivateDeviceRequest{
		DeviceId:     deviceId,
		ClientId:     clientId,
		Challenge:    activationPayload.Challenge,
		Algorithm:    activationPayload.Algorithm,
		SerialNumber: activationPayload.SerialNumber,
		Hmac:         activationPayload.HMAC,
	}

	var response ActivateDeviceResponse

	// Gửi HTTP request
	err := am.client.DoRequest(ctx, http.RequestOptions{
		Method:   "POST",
		Path:     "/api/internal/device/activate",
		Body:     request,
		Response: &response,
	})
	if err != nil {
		return false, fmt.Errorf("Request thất bại: %w", err)
	}

	log.Log().Debugf("Response kích hoạt thiết bị: %+v", response)

	if !response.Success {
		return false, nil
	}

	return response.Success, nil
}