package controllers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"

	"milestones/manager/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DeviceActivationController struct {
	DB *gorm.DB
}

// Sinh mã số ngẫu nhiên gồm 6 chữ số
func generateCode() string {
	randomBytes := make([]byte, 3)
	rand.Read(randomBytes)
	code := 0
	for i, b := range randomBytes {
		code += int(b) << (8 * i)
	}
	return fmt.Sprintf("%06d", code%1000000)
}

// Sinh mã thử thách (challenge) theo định dạng UUID
func generateChallenge() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)

	// Thiết lập version (4) và các bit variant
	randomBytes[6] = (randomBytes[6] & 0x0f) | 0x40
	randomBytes[8] = (randomBytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		randomBytes[0:4],
		randomBytes[4:6],
		randomBytes[6:8],
		randomBytes[8:10],
		randomBytes[10:16])
}

// 1. Kiểm tra thiết bị đã được kích hoạt hay chưa
// GET /api/internal/device/check-activation?device_id=xxx&client_id=xxx
func (dac *DeviceActivationController) CheckDeviceActivation(c *gin.Context) {
	deviceId := c.Query("device_id")
	//clientId := c.Query("client_id")

	if deviceId == "" /*|| clientId == ""*/ {
		c.JSON(http.StatusOK, gin.H{
			"activated": false,
			"error":     "Tham số device_id là bắt buộc",
		})
		return
	}

	var device models.Device
	// Sử dụng device_id (tương ứng với trường device_name) để tìm thiết bị
	if err := dac.DB.Where("device_name = ?", deviceId).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{
				"activated": false,
				"message":   "Thiết bị không tồn tại",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"activated": false,
			"error":     "Truy vấn thiết bị thất bại",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"activated": device.Activated,
		"message": func() string {
			if device.Activated {
				return "Thiết bị đã được kích hoạt"
			}
			return "Thiết bị chưa được kích hoạt"
		}(),
	})
}

// 2. Lấy thông tin kích hoạt
// GET /api/internal/device/activation-info?device_id=xxx&client_id=xxx
func (dac *DeviceActivationController) GetActivationInfo(c *gin.Context) {
	deviceId := c.Query("device_id")
	//clientId := c.Query("client_id")

	if deviceId == "" /*|| clientId == ""*/ {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số device_id và client_id là bắt buộc"})
		return
	}

	var device models.Device
	var isNewDevice bool

	// Sử dụng device_id (tương ứng với trường device_name) để tìm thiết bị
	if err := dac.DB.Where("device_name = ?", deviceId).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Thiết bị không tồn tại, tạo bản ghi thiết bị mới
			device = models.Device{
				DeviceName: deviceId,
				UserID:     0, // đặt user_id về 0
				DeviceCode: generateCode(),
				Challenge:  generateChallenge(),
				Activated:  false,
			}

			if err := dac.DB.Create(&device).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo bản ghi thiết bị thất bại"})
				return
			}
			isNewDevice = true
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn thiết bị thất bại"})
			return
		}
	}

	// Nếu thiết bị đã được kích hoạt, trả về trạng thái ngay
	if device.Activated {
		c.JSON(http.StatusOK, gin.H{
			"activated": true,
			"message":   "Thiết bị đã được kích hoạt",
		})
		return
	}

	// Nếu thiết bị chưa được kích hoạt, sinh hoặc trả về thông tin kích hoạt
	needUpdate := false

	// Nếu chưa có mã kích hoạt, sinh mã kích hoạt mới
	if device.DeviceCode == "" {
		device.DeviceCode = generateCode()
		needUpdate = true
	}

	// Nếu chưa có mã thử thách, sinh mã thử thách mới
	if device.Challenge == "" {
		device.Challenge = generateChallenge()
		needUpdate = true
	}

	// Đảm bảo user_id bằng 0 (nếu không phải thiết bị mới và chưa kích hoạt)
	if !isNewDevice && device.UserID != 0 {
		device.UserID = 0
		needUpdate = true
	}

	// Cập nhật cơ sở dữ liệu
	if needUpdate {
		updates := map[string]interface{}{}
		if device.DeviceCode != "" {
			updates["device_code"] = device.DeviceCode
		}
		if device.Challenge != "" {
			updates["challenge"] = device.Challenge
		}
		if !isNewDevice && device.UserID == 0 {
			updates["user_id"] = device.UserID
		}
		if err := updateDeviceColumns(dac.DB, device.ID, updates); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật thông tin thiết bị thất bại"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"activated": false,
		"code":      device.DeviceCode,
		"challenge": device.Challenge,
		"message":   "Vui lòng vào trang quản trị để liên kết và kích hoạt thiết bị, mã kích hoạt:" + device.DeviceCode,
	})
}

// Xác thực HMAC-SHA256
func verifyHMAC(challenge, secretKey, providedHmac string) bool {
	if secretKey == "" {
		return true // Nếu pre_secret_key trống, bỏ qua xác thực và cho phép luôn
	}

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(challenge))
	expectedHmac := hex.EncodeToString(mac.Sum(nil))

	return expectedHmac == providedHmac
}

// 3. API kích hoạt thiết bị
// POST /api/internal/device/activate
func (dac *DeviceActivationController) ActivateDevice(c *gin.Context) {
	var req struct {
		DeviceId     string `json:"device_id" binding:"required"`
		ClientId     string `json:"client_id" binding:"required"`
		Challenge    string `json:"challenge" binding:"required"`
		Algorithm    string `json:"algorithm" binding:"required"`
		SerialNumber string `json:"serial_number" binding:"required"`
		Hmac         string `json:"hmac" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Lỗi tham số: " + err.Error()})
		return
	}

	var device models.Device
	// Sử dụng device_id (tương ứng với trường device_name) để tìm thiết bị
	if err := dac.DB.Where("device_name = ?", req.DeviceId).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "Thiết bị không tồn tại",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Truy vấn thiết bị thất bại",
		})
		return
	}

	// Kiểm tra thiết bị đã được kích hoạt chưa
	if device.Activated {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Thiết bị đã được kích hoạt",
		})
		return
	}

	if device.UserID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "Thiết bị chưa được liên kết với người dùng",
		})
		return
	}

	if device.Challenge != req.Challenge {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "Mã thử thách không đúng",
		})
		return
	}

	// Xác thực HMAC (nếu pre_secret_key trống thì bỏ qua và cho phép luôn)
	if !verifyHMAC(req.Challenge, device.PreSecretKey, req.Hmac) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "Xác thực HMAC thất bại",
		})
		return
	}

	// Kích hoạt thiết bị
	device.Activated = true
	if err := updateDeviceColumns(dac.DB, device.ID, map[string]interface{}{
		"activated": device.Activated,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Kích hoạt thiết bị thất bại",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Kích hoạt thiết bị thành công",
		"data": gin.H{
			"device_id": device.DeviceName,
			"activated": device.Activated,
		},
	})
}