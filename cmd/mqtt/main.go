package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	mqtt_server "milestones-esp32-server-golang/internal/app/mqtt_server"
	log "milestones-esp32-server-golang/logger"
)

// Hàm khởi tạo
func Init(configFile string) error {
	err := initConfig(configFile)
	if err != nil {
		return err
	}

	err = initLog()
	if err != nil {
		return err
	}

	return nil
}

func initLog() error {
	// Không còn kiểm tra cấu hình stdout nữa, thống nhất ghi ra file
	// Ghi ra file
	binPath, _ := os.Executable()
	baseDir := filepath.Dir(binPath)
	logPath := fmt.Sprintf("%s/%s%s", baseDir, viper.GetString("log.path"), viper.GetString("log.file"))
	/* Các hàm liên quan đến xoay vòng (rotate) nhật ký
	`WithLinkName` tạo liên kết mềm (symlink) trỏ tới file nhật ký mới nhất
	`WithRotationTime` thiết lập thời gian phân tách (rotate) nhật ký, cách bao lâu thì xoay vòng một lần
	WithMaxAge và WithRotationCount chỉ được thiết lập một trong hai:
		`WithMaxAge` thiết lập thời gian lưu trữ tối đa của file trước khi bị dọn dẹp
		`WithRotationCount` thiết lập số lượng file tối đa được lưu trước khi bị dọn dẹp
	*/
	// Cấu hình dưới đây sẽ xoay vòng tạo file mới mỗi 1 phút, giữ lại nhật ký trong 3 phút gần nhất,
	// các file dư thừa sẽ được tự động dọn dẹp.
	writer, err := rotatelogs.New(
		logPath+".%Y%m%d",
		rotatelogs.WithLinkName(logPath),
		rotatelogs.WithRotationCount(uint(viper.GetInt("log.max_age"))),
		rotatelogs.WithRotationTime(time.Duration(86400)*time.Second),
	)
	if err != nil {
		fmt.Printf("Lỗi khởi tạo nhật ký: %v\n", err)
		os.Exit(1)
		return err
	}
	logrus.SetOutput(writer)
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000", // Định dạng thời gian, thêm mili giây
		ForceColors:     false,                     // Không bật màu khi xuất ra file
	})

	// Vô hiệu hóa báo cáo caller mặc định, sử dụng trường caller tùy chỉnh
	logrus.SetReportCaller(false)
	logLevel, _ := logrus.ParseLevel(viper.GetString("log.level"))
	logrus.SetLevel(logLevel)

	return nil

}

func initConfig(configFile string) error {
	basePath, file := filepath.Split(configFile)

	// Lấy tên tệp và phần mở rộng
	fileName, fileExt := func(file string) (string, string) {
		if pos := strings.LastIndex(file, "."); pos != -1 {
			return file[:pos], strings.ToLower(file[pos+1:])
		}
		return file, ""
	}(file)

	// Thiết lập tên tệp cấu hình (không có phần mở rộng)
	viper.SetConfigName(fileName)
	viper.AddConfigPath(basePath)

	// Thiết lập loại cấu hình dựa theo phần mở rộng của tệp
	switch fileExt {
	case "json":
		viper.SetConfigType("json")
	case "yaml", "yml":
		viper.SetConfigType("yaml")
	default:
		return fmt.Errorf("unsupported config file type: %s", fileExt)
	}

	return viper.ReadInConfig()
}

func main() {
	// Phân tích tham số dòng lệnh
	configFile := flag.String("c", "config/mqtt_config.json", "Đường dẫn tệp cấu hình")
	flag.Parse()

	if *configFile == "" {
		fmt.Println("Đường dẫn tệp cấu hình không được để trống")
		return
	}

	// Khởi tạo cấu hình và nhật ký
	err := Init(*configFile)
	if err != nil {
		fmt.Printf("Khởi tạo thất bại: %v\n", err)
		return
	}

	// Khởi động máy chủ MQTT
	err = mqtt_server.StartMqttServer()
	if err != nil {
		log.Errorf("Khởi động máy chủ MQTT thất bại: %v", err)
		return
	}

	fmt.Println("Máy chủ MQTT đã khởi động")

	// Chặn (block) và lắng nghe tín hiệu thoát
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Info("Máy chủ MQTT đã khởi động, nhấn Ctrl+C để thoát")
	<-quit

	log.Info("Đang tắt máy chủ MQTT...")
	log.Info("Máy chủ MQTT đã tắt")
}