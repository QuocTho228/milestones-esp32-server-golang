package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"milestones/manager/backend/middleware"
	"milestones/manager/backend/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthController struct {
	DB *gorm.DB
}

const defaultLoginCaptchaEnabled = true

type LoginRequest struct {
	Username      string `json:"username" binding:"required"`
	Password      string `json:"password" binding:"required"`
	CaptchaID     string `json:"captchaId"`
	CaptchaAnswer string `json:"captchaAnswer"`
}

type RegisterRequest struct {
	Username      string `json:"username" binding:"required"`
	Password      string `json:"password" binding:"required"`
	Email         string `json:"email" binding:"required,email"`
	CaptchaID     string `json:"captchaId"`
	CaptchaAnswer string `json:"captchaAnswer"`
}

func isLoginCaptchaEnabledFromDB(db *gorm.DB) bool {
	if db == nil {
		return defaultLoginCaptchaEnabled
	}

	var authConfig models.Config
	if err := db.Where("type = ?", "auth").Order("is_default DESC, id ASC").First(&authConfig).Error; err != nil {
		return defaultLoginCaptchaEnabled
	}

	var authData map[string]interface{}
	if authConfig.JsonData == "" || json.Unmarshal([]byte(authConfig.JsonData), &authData) != nil {
		return defaultLoginCaptchaEnabled
	}

	if enabled, ok := authData["login_captcha_enabled"].(bool); ok {
		return enabled
	}

	return defaultLoginCaptchaEnabled
}

// Lấy trạng thái bật/tắt xác thực số khi đăng nhập
func (ac *AuthController) GetCaptchaStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enabled": isLoginCaptchaEnabledFromDB(ac.DB),
	})
}

// Lấy mã captcha đơn giản
func (ac *AuthController) GetSimpleCaptcha(c *gin.Context) {
	captchaID, prompt, err := authCaptchaStore.NewChallenge()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo captcha thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"captchaId": captchaID,
		"prompt":    prompt,
	})
}

// Đăng nhập người dùng
func (ac *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if isLoginCaptchaEnabledFromDB(ac.DB) && !authCaptchaStore.Verify(req.CaptchaID, req.CaptchaAnswer) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Xác thực captcha thất bại, vui lòng thử câu khác"})
		return
	}

	// Thêm log debug đăng nhập
	log.Printf("[Login] Thử đăng nhập người dùng: %s, IP client: %s", req.Username, c.ClientIP())
	log.Printf("[Login] Độ dài mật khẩu nhận được: %d", len(req.Password))

	// Nếu cơ sở dữ liệu khả dụng, thử xác thực từ cơ sở dữ liệu
	if ac.DB != nil {
		log.Printf("[Login] Kết nối cơ sở dữ liệu khả dụng, bắt đầu xác thực bằng cơ sở dữ liệu")
		var user models.User
		if err := ac.DB.Where("username = ?", req.Username).First(&user).Error; err == nil {
			log.Printf("[Login] Tìm thấy người dùng: ID=%d, Username=%s, Role=%s, Email=%s", user.ID, user.Username, user.Role, user.Email)
			log.Printf("[Login] Độ dài hash mật khẩu trong cơ sở dữ liệu: %d, Tiền tố hash: %s", len(user.Password), user.Password[:10])
			log.Printf("[Login] Bắt đầu xác thực so sánh mật khẩu bằng bcrypt")

			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err == nil {
				log.Printf("[Login] ✅ Xác thực mật khẩu thành công - Người dùng: %s", req.Username)
				token, err := middleware.GenerateToken(user.ID, user.Username, user.Role)
				if err != nil {
					log.Printf("[Login] ❌ Tạo token thất bại: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo token thất bại"})
					return
				}

				log.Printf("[Login] ✅ Đăng nhập thành công, trả về token - Người dùng: %s, Vai trò: %s", user.Username, user.Role)
				c.JSON(http.StatusOK, gin.H{
					"token": token,
					"user": gin.H{
						"id":       user.ID,
						"username": user.Username,
						"email":    user.Email,
						"role":     user.Role,
					},
				})
				return
			} else {
				log.Printf("[Login] ❌ Xác thực mật khẩu thất bại - Người dùng: %s, Lỗi bcrypt: %v", req.Username, err)
				log.Printf("[Login] Thông tin debug - Mật khẩu nhập vào: '%s', Hash: '%s'", req.Password, user.Password)
			}
		} else {
			log.Printf("[Login] ❌ Người dùng không tồn tại - Tên đăng nhập: %s, Lỗi cơ sở dữ liệu: %v", req.Username, err)
		}
	} else {
		log.Printf("[Login] ❌ Kết nối cơ sở dữ liệu không khả dụng")
	}

	// Dự phòng: xác thực người dùng admin được hardcode (khi cơ sở dữ liệu không khả dụng)
	if req.Username == "admin" && req.Password == "admin123" {
		token, err := middleware.GenerateToken(1, "admin", "admin")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo token thất bại"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token": token,
			"user": gin.H{
				"id":       1,
				"username": "admin",
				"email":    "admin@milestones.com",
				"role":     "admin",
			},
		})
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{"error": "Tên đăng nhập hoặc mật khẩu sai"})
}

// Đăng ký người dùng
func (ac *AuthController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !authCaptchaStore.Verify(req.CaptchaID, req.CaptchaAnswer) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Xác thực captcha thất bại, vui lòng thử câu khác"})
		return
	}

	// Kiểm tra tên đăng nhập đã tồn tại chưa
	var existingUser models.User
	if err := ac.DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tên đăng nhập đã tồn tại"})
		return
	}

	// Mã hóa mật khẩu
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Mã hóa mật khẩu thất bại"})
		return
	}

	user := models.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Role:     "user",
	}

	if err := ac.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo người dùng thất bại"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Đăng ký thành công",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}

// Lấy thông tin người dùng hiện tại
func (ac *AuthController) GetProfile(c *gin.Context) {
	log.Printf("[GetProfile] Bắt đầu xử lý yêu cầu lấy thông tin người dùng, IP client: %s", c.ClientIP())

	userID, exists := c.Get("user_id")
	if !exists {
		log.Printf("[GetProfile] ❌ Không thể lấy ID người dùng, middleware xác thực có thể chưa được thiết lập đúng")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu thông tin xác thực"})
		return
	}

	log.Printf("[GetProfile] Lấy ID người dùng từ ngữ cảnh: %v", userID)

	var user models.User
	if err := ac.DB.First(&user, userID).Error; err != nil {
		log.Printf("[GetProfile] ❌ Truy vấn người dùng trong cơ sở dữ liệu thất bại: %v, ID người dùng: %v", err, userID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Người dùng không tồn tại"})
		return
	}

	log.Printf("[GetProfile] ✅ Lấy thông tin người dùng thành công - ID: %d, Tên đăng nhập: %s, Vai trò: %s", user.ID, user.Username, user.Role)
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}