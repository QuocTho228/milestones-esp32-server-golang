package controllers

import (
	"log"
	"net/http"
	"milestones/manager/backend/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type SetupController struct {
	DB *gorm.DB
}

type SetupRequest struct {
	AdminUsername string `json:"admin_username" binding:"required,min=3,max=50"`
	AdminPassword string `json:"admin_password" binding:"required,min=6,max=100"`
	AdminEmail    string `json:"admin_email" binding:"required,email"`
}

// Kiểm tra xem cơ sở dữ liệu có cần khởi tạo không
func (sc *SetupController) CheckSetupStatus(c *gin.Context) {
	if sc.DB == nil {
		c.JSON(http.StatusOK, gin.H{
			"needs_setup": true,
			"message":     "Kết nối cơ sở dữ liệu không khả dụng",
		})
		return
	}

	// Kiểm tra xem bảng người dùng đã tồn tại chưa
	if !sc.DB.Migrator().HasTable(&models.User{}) {
		c.JSON(http.StatusOK, gin.H{
			"needs_setup": true,
			"message":     "Cấu trúc bảng cơ sở dữ liệu chưa được khởi tạo",
		})
		return
	}

	// Kiểm tra xem đã có người dùng quản trị viên chưa
	var count int64
	sc.DB.Model(&models.User{}).Where("role = ?", "admin").Count(&count)

	if count == 0 {
		c.JSON(http.StatusOK, gin.H{
			"needs_setup": true,
			"message":     "Cần tạo tài khoản quản trị viên",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"needs_setup": false,
		"message":     "Hệ thống đã được khởi tạo",
	})
}

// Khởi tạo cơ sở dữ liệu
func (sc *SetupController) InitializeDatabase(c *gin.Context) {
	var req SetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if sc.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Kết nối cơ sở dữ liệu không khả dụng"})
		return
	}

	// Bắt đầu transaction
	tx := sc.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Khởi động transaction cơ sở dữ liệu thất bại"})
		return
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Tự động migrate cấu trúc bảng
	log.Println("Bắt đầu tự động migrate cấu trúc bảng cơ sở dữ liệu...")
	err := tx.AutoMigrate(
		&models.User{},
		&models.Device{},
		&models.Agent{},
		&models.Config{},
		&models.MCPMarketService{},
		&models.GlobalRole{},
		&models.SpeakerGroup{},
		&models.SpeakerSample{},
		&models.ChatMessage{},
	)
	if err != nil {
		tx.Rollback()
		log.Printf("Migrate cấu trúc bảng cơ sở dữ liệu thất bại: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Migrate cấu trúc bảng cơ sở dữ liệu thất bại: " + err.Error()})
		return
	}
	log.Println("Migrate cấu trúc bảng cơ sở dữ liệu thành công")

	// 2. Kiểm tra xem đã tồn tại người dùng quản trị viên chưa
	var existingAdmin models.User
	if err := tx.Where("role = ?", "admin").First(&existingAdmin).Error; err == nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Người dùng quản trị viên đã tồn tại, không thể khởi tạo lại"})
		return
	}

	// 3. Kiểm tra xem tên đăng nhập đã tồn tại chưa
	var existingUser models.User
	if err := tx.Where("username = ?", req.AdminUsername).First(&existingUser).Error; err == nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tên đăng nhập đã tồn tại"})
		return
	}

	// 4. Kiểm tra xem email đã tồn tại chưa
	if err := tx.Where("email = ?", req.AdminEmail).First(&existingUser).Error; err == nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email đã tồn tại"})
		return
	}

	// 5. Mã hóa mật khẩu
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Mã hóa mật khẩu thất bại"})
		return
	}

	// 6. Tạo người dùng quản trị viên
	admin := models.User{
		Username: req.AdminUsername,
		Password: string(hashedPassword),
		Email:    req.AdminEmail,
		Role:     "admin",
	}

	if err := tx.Create(&admin).Error; err != nil {
		tx.Rollback()
		log.Printf("Tạo người dùng quản trị viên thất bại: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo người dùng quản trị viên thất bại: " + err.Error()})
		return
	}

	// 7. Tạo một số vai trò toàn cục mặc định
	defaultRoles := []models.GlobalRole{
		{
			Name:        "Trợ lý",
			Description: "Một trợ lý AI thân thiện, có thể giúp người dùng giải quyết nhiều vấn đề khác nhau",
			Prompt:      "Bạn là một trợ lý AI thân thiện, chuyên nghiệp. Hãy trả lời câu hỏi của người dùng bằng ngôn ngữ ngắn gọn, rõ ràng và đưa ra những gợi ý hữu ích.",
			IsDefault:   true,
		},
		{
			Name:        "Giáo viên",
			Description: "Một giáo viên kiên nhẫn, có thể giải thích chi tiết các khái niệm phức tạp",
			Prompt:      "Bạn là một giáo viên giàu kinh nghiệm. Hãy giải thích các khái niệm phức tạp theo cách dễ hiểu và đưa ra các ví dụ cụ thể để giúp người dùng nắm bắt.",
			IsDefault:   false,
		},
		{
			Name:        "Bạn bè",
			Description: "Một người bạn chu đáo, biết lắng nghe và đồng hành",
			Prompt:      "Bạn là một người bạn chu đáo. Hãy giao tiếp với người dùng bằng thái độ ấm áp, thấu hiểu, và mang lại sự hỗ trợ, động viên về mặt cảm xúc.",
			IsDefault:   false,
		},
	}

	for _, role := range defaultRoles {
		if err := tx.Create(&role).Error; err != nil {
			log.Printf("Tạo vai trò mặc định thất bại: %v", err)
			// Không dừng quá trình khởi tạo, tiếp tục thực hiện
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Commit transaction cơ sở dữ liệu thất bại"})
		return
	}

	log.Printf("Khởi tạo cơ sở dữ liệu thành công, người dùng quản trị viên: %s", req.AdminUsername)
	c.JSON(http.StatusOK, gin.H{
		"message": "Khởi tạo cơ sở dữ liệu thành công",
		"admin": gin.H{
			"username": admin.Username,
			"email":    admin.Email,
			"role":     admin.Role,
		},
	})
}