package database

import (
	"fmt"
	"log"
	"milestones/manager/backend/config"
	"milestones/manager/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// InitWithReset khởi tạo cơ sở dữ liệu và đặt lại tất cả các bảng (chỉ dành cho môi trường phát triển)
func InitWithReset(cfg config.DatabaseConfig) *gorm.DB {
	storageType := cfg.GetStorageType()
	var db *gorm.DB
	var err error

	if storageType == "sqlite" {
		if cfg.SQLite == nil {
			log.Fatal("Cấu hình SQLite trống")
		}
		db, err = gorm.Open(sqlite.Open(cfg.SQLite.FilePath), &gorm.Config{})
	} else {
		if cfg.MySQL == nil {
			log.Fatal("Cấu hình MySQL trống")
		}
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.MySQL.Username, cfg.MySQL.Password, cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	}

	if err != nil {
		log.Fatal("Kết nối cơ sở dữ liệu không thành công:", err)
	}

	log.Println("Cảnh báo: Các bảng cơ sở dữ liệu đang được thiết lập lại, tất cả dữ liệu sẽ bị xóa!")

	// Xóa tất cả các bảng
	err = db.Migrator().DropTable(
		&models.User{},
		&models.Device{},
		&models.Agent{},
		&models.Config{},
		&models.MCPMarketService{},
		&models.GlobalRole{},
		&models.Role{},
		&models.SpeakerGroup{},
		&models.SpeakerSample{},
		&models.VoiceClone{},
		&models.VoiceCloneAudio{},
	)
	if err != nil {
		log.Printf("Đã xảy ra lỗi khi xóa bảng (bảng có thể không tồn tại): %v", err)
	}

	log.Println("Các bảng cơ sở dữ liệu đã được xóa hoàn tất!")
	return db
}
