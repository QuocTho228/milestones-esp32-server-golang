package database

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"milestones/manager/backend/config"
	"milestones/manager/backend/models"
	"milestones/manager/backend/services/configprovider"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Init(cfg config.DatabaseConfig) *gorm.DB {
	var db *gorm.DB
	var err error

	storageType := cfg.GetStorageType()

	if storageType == "sqlite" {
		if cfg.SQLite == nil {
			log.Println("Cấu hình SQLite trống và sẽ chạy ở chế độ dự phòng (xác thực người dùng được mã hóa cứng)")
			return nil
		}
		// Đảm bảo thư mục chứa tệp cơ sở dữ liệu tồn tại để tránh báo cáo SQLite không thể mở tệp cơ sở dữ liệu.
		dir := filepath.Dir(cfg.SQLite.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Không tạo được thư mục cơ sở dữ liệu %s: %v", dir, err)
			return nil
		}
		log.Println("Sử dụng cơ sở dữ liệu SQLite:", cfg.SQLite.FilePath)
		db, err = gorm.Open(sqlite.Open(cfg.SQLite.FilePath), &gorm.Config{})
	} else {
		if cfg.MySQL == nil {
			log.Println("Cấu hình MySQL trống và sẽ chạy ở chế độ dự phòng (xác thực người dùng được mã hóa cứng)")
			return nil
		}
		// Kết nối cơ sở dữ liệu MySQL
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.MySQL.Username, cfg.MySQL.Password, cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	}

	if err != nil {
		log.Println("Kết nối cơ sở dữ liệu không thành công:", err)
		log.Println("Sẽ chạy bằng chế độ dự phòng (xác thực người dùng được mã hóa cứng)")
		return nil
	}

	log.Println("Kết nối cơ sở dữ liệu thành công")

	// Tự động di chuyển cấu trúc bảng cơ sở dữ liệu
	log.Println("Bắt đầu tự động di chuyển cấu trúc bảng cơ sở dữ liệu...")
	err = db.AutoMigrate(
		&models.User{},
		&models.APIToken{},
		&models.Device{},
		&models.Agent{},
		&models.KnowledgeBase{},
		&models.KnowledgeBaseDocument{},
		&models.AgentKnowledgeBase{},
		&models.Config{},
		&models.MCPMarketService{},
		&models.GlobalRole{},
		&models.Role{},
		&models.ChatMessage{},
		&models.SpeakerGroup{},
		&models.SpeakerSample{},
		&models.VoiceClone{},
		&models.VoiceCloneAudio{},
		&models.VoiceCloneTask{},
		&models.UserVoiceCloneQuota{},
	)
	if err != nil {
		log.Printf("Di chuyển cấu trúc bảng cơ sở dữ liệu không thành công: %v", err)
		log.Println("Sẽ chạy bằng chế độ dự phòng (xác thực người dùng được mã hóa cứng)")
		return nil
	}
	log.Println("Di chuyển cấu trúc bảng cơ sở dữ liệu thành công")

	if err := dropDeprecatedAgentStatusColumn(db); err != nil {
		log.Printf("Không thể xóa trường trạng thái đại lý cũ: %v", err)
	}

	// Di chuyển dữ liệu vai trò chung hiện có sang bảng vai trò mới
	log.Println("Kiểm tra xem có cần di chuyển dữ liệu vai trò chung không...")
	if err := migrateGlobalRolesToRoles(db); err != nil {
		log.Printf("Không thể di chuyển dữ liệu vai trò chung: %v", err)
		// Lỗi di chuyển không ảnh hưởng đến việc khởi động nhưng dữ liệu không được di chuyển
	}
	if err := repairConfigProviders(db); err != nil {
		log.Printf("Không thể sửa chữa các nhà cung cấp cấu hình: %v", err)
	}
	return db
}

func dropDeprecatedAgentStatusColumn(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.Agent{}) {
		return nil
	}
	hasColumn, err := hasDatabaseColumn(db, "agents", "status")
	if err != nil {
		return err
	}
	if !hasColumn {
		return nil
	}
	err = db.Exec("ALTER TABLE agents DROP COLUMN status").Error
	if err != nil {
		return err
	}
	log.Println("Trường trạng thái đại lý cũ agent.status đã bị xóa")
	return nil
}

func hasDatabaseColumn(db *gorm.DB, tableName, columnName string) (bool, error) {
	switch db.Dialector.Name() {
	case "sqlite":
		var columns []struct {
			Name string `gorm:"column:name"`
		}
		if err := db.Raw(fmt.Sprintf("PRAGMA table_info(%s)", tableName)).Scan(&columns).Error; err != nil {
			return false, err
		}
		for _, column := range columns {
			if column.Name == columnName {
				return true, nil
			}
		}
		return false, nil
	case "mysql":
		var count int64
		if err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?",
			tableName,
			columnName,
		).Scan(&count).Error; err != nil {
			return false, err
		}
		return count > 0, nil
	default:
		return db.Migrator().HasColumn(tableName, columnName), nil
	}
}

func Close(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Println("Không thể kết nối cơ sở dữ liệu:", err)
		return
	}
	sqlDB.Close()
}
// migrateGlobalRolesToRoles di chuyển dữ liệu vai trò chung hiện có sang bảng vai trò mới
func migrateGlobalRolesToRoles(db *gorm.DB) error {
	// Kiểm tra xem bảng vai trò đã có dữ liệu chưa
	var count int64
	if err := db.Table("roles").Count(&count).Error; err != nil {
		return fmt.Errorf("Kiểm tra bảng vai trò không thành công: %w", err)
	}

	// Nếu bảng vai trò đã có dữ liệu, hãy bỏ qua quá trình di chuyển
	if count > 0 {
		log.Println("Bảng vai trò đã có dữ liệu, bỏ qua việc di chuyển")
		return nil
	}

	// Kiểm tra xem bảng global_roles có dữ liệu không
	var globalRoleCount int64
	if err := db.Table("global_roles").Count(&globalRoleCount).Error; err != nil {
		// Bảng global_roles có thể không tồn tại, đó không phải là lỗi
		log.Println("Bảng global_roles không tồn tại, bỏ qua việc di chuyển")
		return nil
	}

	if globalRoleCount == 0 {
		log.Println("Bảng global_roles không có dữ liệu và quá trình di chuyển bị bỏ qua.")
		return nil
	}

	log.Printf("Bắt đầu di chuyển dữ liệu vai trò chung %d sang bảng vai trò...", globalRoleCount)

	// Truy vấn tất cả các vai trò chung
	var globalRoles []models.GlobalRole
	if err := db.Table("global_roles").Find(&globalRoles).Error; err != nil {
		return fmt.Errorf("Truy vấn global_roles không thành công: %w", err)
	}

	// Chuyển đổi và chèn vào bảng roles
	for _, gr := range globalRoles {
		role := models.Role{
			UserID:      nil, // Vai trò chung user_id là NULL
			Name:        gr.Name,
			Description: gr.Description,
			Prompt:      gr.Prompt,
			RoleType:    "global",
			Status:      "active",
			SortOrder:   0,
			IsDefault:   gr.IsDefault,
			CreatedAt:   gr.CreatedAt,
			UpdatedAt:   gr.UpdatedAt,
		}
		if err := db.Create(&role).Error; err != nil {
			log.Printf("Chèn vai trò %s thất bại: %v", gr.Name, err)
			continue
		}
		log.Printf("Các vai trò chung đã được di chuyển: %s", gr.Name)
	}

	log.Println("Quá trình di chuyển dữ liệu vai trò toàn cục đã hoàn tất")
	return nil
}

func repairConfigProviders(db *gorm.DB) error {
	var configs []models.Config
	if err := db.Where("type IN ?", []string{"vad", "asr", "llm", "tts", "memory", "vision"}).Find(&configs).Error; err != nil {
		return err
	}

	repaired := 0
	for _, cfg := range configs {
		var data map[string]interface{}
		if cfg.JsonData != "" {
			if err := json.Unmarshal([]byte(cfg.JsonData), &data); err != nil {
				log.Printf("Bỏ qua sửa chữa nhà cung cấp, phân tích cú pháp json_data không thành công type=%s config_id=%s: %v", cfg.Type, cfg.ConfigID, err)
				continue
			}
		}
		if data == nil {
			data = map[string]interface{}{}
		}

		provider := configprovider.NormalizeExistingProvider(cfg.Type, cfg.Provider, cfg.ConfigID, data)
		if provider == "" || provider == cfg.Provider {
			if jsonProvider, _ := data["provider"].(string); strings.TrimSpace(jsonProvider) == "" || strings.EqualFold(strings.TrimSpace(jsonProvider), provider) {
				continue
			}
		}

		updates := map[string]interface{}{}
		if provider != "" && provider != cfg.Provider {
			updates["provider"] = provider
		}
		if provider != "" {
			if jsonProvider, _ := data["provider"].(string); !strings.EqualFold(strings.TrimSpace(jsonProvider), provider) {
				data["provider"] = provider
				bytes, err := json.Marshal(data)
				if err != nil {
					return err
				}
				updates["json_data"] = string(bytes)
			}
		}
		if len(updates) == 0 {
			continue
		}
		if err := db.Model(&models.Config{}).Where("id = ?", cfg.ID).Updates(updates).Error; err != nil {
			return err
		}
		repaired++
	}

	if repaired > 0 {
		log.Printf("Đã sửa %d nhà cung cấp cấu hình", repaired)
	}
	return nil
}
