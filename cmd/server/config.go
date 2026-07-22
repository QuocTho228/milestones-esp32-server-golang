package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"milestones-esp32-server-golang/internal/app/server/auth"
	redisdb "milestones-esp32-server-golang/internal/db/redis"
	user_config "milestones-esp32-server-golang/internal/domain/config"

	log "milestones-esp32-server-golang/logger"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/mitchellh/hashstructure/v2"
	logrus "github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

// Các biến toàn cục dùng để điều khiển việc cập nhật cấu hình định kỳ
var (
	configUpdateTicker *time.Ticker
	configUpdateStop   chan struct{}
	configUpdateWg     sync.WaitGroup
)

func Init(configFile string) error {
	// Khởi tạo cấu hình
	err := initConfig(configFile)
	if err != nil {
		fmt.Printf("initConfig err: %+v", err)
		os.Exit(1)
		return err
	}

	// Khởi tạo nhật ký
	initLog()

	// Khởi tạo hệ thống cấu hình (bao gồm cả kết nối WebSocket)
	// Lưu ý: không đăng ký ApplySystemConfigToViper riêng lẻ ở đây, nếu không nó sẽ được thực thi trước callback của main,
	// khiến "cấu hình hiện tại" được đọc trong main đã là cấu hình mới sau khi hợp nhất; việc hợp nhất cần được thực hiện
	// trong callback của main, sau khi đã đọc current và so sánh xong.
	ctx := context.Background()
	if err := user_config.InitConfigSystem(ctx); err != nil {
		fmt.Printf("Không thể khởi tạo hệ thống cấu hình: %v\n", err)
	}

	// Lấy cấu hình từ API và cập nhật
	if err := updateConfigFromAPI(); err != nil {
		fmt.Printf("Không lấy được cấu hình từ giao diện, hãy sử dụng cấu hình cục bộ: %v\n", err)
	}

	// Bắt đầu cập nhật cấu hình định kỳ
	startPeriodicConfigUpdate()

	// Khởi tạo VAD
	initVad()

	// Khởi tạo Redis
	initRedis()

	// Mô-đun memory sử dụng cơ chế tải chậm (lazy load), sẽ tự động khởi tạo khi được sử dụng, không cần khởi tạo tường minh

	// Khởi tạo auth
	err = initAuthManager()
	if err != nil {
		fmt.Printf("initAuthManager err: %+v", err)
		os.Exit(1)
		return err
	}

	return nil
}

// startPeriodicConfigUpdate khởi động việc cập nhật cấu hình định kỳ
func startPeriodicConfigUpdate() {
	// Lấy khoảng thời gian cập nhật từ cấu hình, mặc định 5 phút
	updateInterval := viper.GetDuration("config_provider.update_interval")
	if updateInterval <= 0 {
		updateInterval = 30 * time.Second
	}

	// Kiểm tra xem cập nhật định kỳ có được bật hay không
	if !viper.GetBool("config_provider.enable_periodic_update") {
		log.Info("Cập nhật cấu hình định kỳ bị vô hiệu hóa")
		return
	}

	configUpdateStop = make(chan struct{})
	configUpdateTicker = time.NewTicker(updateInterval)

	configUpdateWg.Add(1)
	go func() {
		defer configUpdateWg.Done()
		defer configUpdateTicker.Stop()

		for {
			select {
			case <-configUpdateTicker.C:
				if err := updateConfigFromAPI(); err != nil {
					log.Warnf("Cập nhật cấu hình định kỳ thất bại: %v", err)
				} else {
					//log.Debug("Cập nhật cấu hình định kỳ thành công")
				}
			case <-configUpdateStop:
				log.Info("Cập nhật cấu hình định kỳ đã dừng")
				return
			}
		}
	}()

	log.Infof("Cập nhật cấu hình định kỳ đã được bắt đầu, khoảng thời gian cập nhật: %v", updateInterval)
}

// StopPeriodicConfigUpdate dừng việc cập nhật cấu hình định kỳ
func StopPeriodicConfigUpdate() {
	if configUpdateStop != nil {
		close(configUpdateStop)
		configUpdateWg.Wait()
		logrus.Info("Cập nhật cấu hình định kỳ đã dừng")
	}
}

func initConfig(configFile string) error {
	viper.SetConfigFile(configFile)

	// Đọc tệp cấu hình
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	return nil
}

// ApplySystemConfigToViper hợp nhất cấu hình hệ thống vào viper, dùng cho việc cập nhật thời gian thực system_config
// được đẩy xuống qua WebSocket (callback không có giá trị trả về)
func ApplySystemConfigToViper(data map[string]interface{}) {
	if err := viper.MergeConfigMap(data); err != nil {
		log.Warnf("Hợp nhất cấu hình đẩy vào viper không thành công: %v", err)
		return
	}
	log.Info("Cấu hình hệ thống đã hợp nhất được đẩy từ WebSocket sang viper")
}

// SystemConfigEqual so sánh xem hai đoạn cấu hình hệ thống có tương đương về mặt ngữ nghĩa hay không
// (sử dụng fingerprint hashstructure, không phụ thuộc vào thứ tự key trong map)
func SystemConfigEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		log.Debugf("[SystemConfigEqual] Kết quả: true (tất cả các giá trị đều là nil)")
		return true
	}
	if a == nil || b == nil {
		log.Debugf("[SystemConfigEqual] Kết quả: false (một phía là nil)")
		return false
	}
	ha, err1 := hashstructure.Hash(a, hashstructure.FormatV2, nil)
	hb, err2 := hashstructure.Hash(b, hashstructure.FormatV2, nil)
	if err1 != nil || err2 != nil {
		log.Debugf("[SystemConfigEqual] Kết quả: false (Hash thất bại err1=%v err2=%v)", err1, err2)
		return false
	}
	equal := ha == hb
	log.Debugf("[SystemConfigEqual] kết quả: %t (ha=%d hb=%d), a: %+v, b: %+v", equal, ha, hb, a, b)
	return equal
}

// updateConfigFromAPI lấy cấu hình từ API và cập nhật vào cấu hình viper.
// Bên trong sẽ liên tục thử lại cho đến khi thành công thì mới trả về.
func updateConfigFromAPI() error {
	configProviderType := viper.GetString("config_provider.type")
	retryInterval := 10 * time.Second // Khoảng thời gian giữa các lần thử lại
	retryCount := 0

	for {
		// Lấy địa chỉ hệ thống quản lý backend từ tệp cấu hình
		configProvider, err := user_config.GetProvider(configProviderType)
		if err != nil {
			retryCount++
			log.Warnf("Không thể truy xuất cấu hình hệ thống (%d lần thử lại): %v, thử lại sau %v", retryCount, err, retryInterval)
			time.Sleep(retryInterval)
			continue
		}

		// Tạo context
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		// Lấy chuỗi JSON cấu hình hệ thống
		configJSON, err := configProvider.GetSystemConfig(ctx)
		cancel()

		if err != nil {
			retryCount++
			log.Warnf("Không thể truy xuất cấu hình hệ thống (%d lần thử lại): %v, thử lại sau %v", retryCount, err, retryInterval)
			time.Sleep(retryInterval)
			continue
		}

		if configJSON == "" {
			// Cấu hình trống, coi như thành công (có thể dịch vụ trả về cấu hình rỗng)
			if retryCount > 0 {
				log.Infof("Cấu hình đã được truy xuất thành công (cấu hình trống, sau %d lần thử lại)", retryCount)
			}
			return nil
		}

		// Phân tích chuỗi JSON thành map
		var configMap map[string]interface{}
		if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
			retryCount++
			log.Warnf("Không thể phân tích cú pháp JSON cấu hình (%d lần thử lại): %v, thử lại sau %v", retryCount, err, retryInterval)
			time.Sleep(retryInterval)
			continue
		}

		//log.Debugf("Load config from API: %+v", configMap)

		// Sử dụng viper.MergeConfigMap để thiết lập vào viper
		if err := viper.MergeConfigMap(configMap); err != nil {
			retryCount++
			log.Warnf("Hợp nhất cấu hình vào Viper thất bại (thử lại %d): %v, thử lại sau %v", retryCount, err, retryInterval)
			time.Sleep(retryInterval)
			continue
		}

		// Thành công
		if retryCount > 0 {
			log.Infof("Cấu hình đã được thiết lập thành công (sau %d lần thử lại).", retryCount)
		} else {
			log.Debug("Đã nhận được cấu hình thành công")
		}
		return nil
	}
}

func initLog() error {
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
		fmt.Printf("init log error: %v\n", err)
		os.Exit(1)
		return err
	}

	// Xác định đích xuất log dựa theo cấu hình
	if viper.GetBool("log.stdout") {
		// Xuất đồng thời ra cả file và standard output
		multiWriter := io.MultiWriter(writer, os.Stdout)
		logrus.SetOutput(multiWriter)
		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000", // Định dạng thời gian, thêm mili giây
			ForceColors:     true,                      // Bật màu khi xuất ra standard output
		})
	} else {
		// Chỉ xuất ra file
		logrus.SetOutput(writer)
		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000", // Định dạng thời gian, thêm mili giây
			ForceColors:     false,                     // Không bật màu khi xuất ra file
		})
	}

	// Vô hiệu hóa báo cáo caller mặc định, sử dụng trường caller tùy chỉnh
	logrus.SetReportCaller(false)
	logLevel, _ := logrus.ParseLevel(viper.GetString("log.level"))
	logrus.SetLevel(logLevel)

	return nil
}

func initVad() error {
	log.Infof("Đang bắt đầu khởi tạo mô-đun VAD...")
	vadProvider := viper.GetString("vad.provider")
	log.Infof("Nhà cung cấp VAD: %s", vadProvider)

	// VAD sử dụng chế độ tải chậm (lazy load), sẽ tự động khởi tạo thông qua resource pool toàn cục trong lần sử dụng đầu tiên
	log.Infof("Mô-đun VAD sẽ sử dụng chế độ tải chậm và tự động khởi tạo trong lần sử dụng đầu tiên")
	return nil
}

func initRedis() error {
	// Khởi tạo mô-đun Redis thống nhất của chúng ta
	redisConfig := &redisdb.Config{
		Host:     viper.GetString("redis.host"),
		Port:     viper.GetInt("redis.port"),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
	}

	err := redisdb.Init(redisConfig)
	if err != nil {
		fmt.Printf("init redis error: %v\n", err)
		return err
	}

	return nil
}

func initAuthManager() error {
	return auth.Init()
}