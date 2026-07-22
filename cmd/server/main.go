package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"milestones-esp32-server-golang/internal/app/server"
	user_config "milestones-esp32-server-golang/internal/domain/config"
	log "milestones-esp32-server-golang/logger"

	"github.com/spf13/viper"
)

func main() {
	// Phân tích các tham số dòng lệnh
	configFile := flag.String("c", defaultConfigFilePath, "Đường dẫn tệp cấu hình")
	managerEnable := flag.Bool("manager-enable", defaultManagerEnable, "Có bật manager nhúng hay không")
	managerConfig := flag.String("manager-config", "", "Đường dẫn đến tệp cấu hình của manager. Không bắt buộc khi bật, mặc định là manager/backend/config/config.json")
	asrEnable := flag.Bool("asr-enable", defaultAsrEnable, "Có bật asr_server nhúng hay không")
	asrConfig := flag.String("asr-config", "", "Đường dẫn tệp cấu hình của asr_server. Tùy chọn khi bật, mặc định: asr_server/config.json")
	flag.Parse()

	if *configFile == "" {
		fmt.Println("Đường dẫn đến tệp cấu hình không được để trống.")
		return
	}

	// Khởi động trình quản lý trước, sau đó chạy Init; nếu không, hàm updateConfigFromAPI trong Init sẽ không thể kết nối với trình quản lý và khiến tiến trình bị treo.
	if *managerEnable {
		StartManagerHTTP(*managerConfig)
	}
	if *asrEnable {
		StartAsrServerHTTP(*asrConfig)
	}
	err := Init(*configFile)
	if err != nil {
		return
	}

	// Khởi động dịch vụ pprof theo cấu hình đã thiết lập.
	if viper.GetBool("server.pprof.enable") {
		pprofPort := viper.GetInt("server.pprof.port")
		go func() {
			log.Infof("Khởi động dịch vụ pprof, cổng: %d", pprofPort)
			if err := http.ListenAndServe(fmt.Sprintf(":%d", pprofPort), nil); err != nil {
				log.Errorf("Dịch vụ pprof không khởi động được: %v", err)
			}
		}()
		log.Infof("Địa chỉ pprof: http://localhost:%d/debug/pprof/", pprofPort)
	} else {
		log.Info("Dịch vụ pprof bị vô hiệu hóa")
	}

	// Tạo máy chủ
	appInstance := server.NewApp()

	var lock sync.RWMutex
	// Đăng ký cập nhật nóng cho system_config: so sánh cấu hình hiện tại của Viper với cấu hình được đẩy lên, chỉ hợp nhất và kích hoạt cập nhật nóng khi nội dung thay đổi.
	user_config.RegisterManagerSystemConfigHandler(func(data map[string]interface{}) {
		lock.Lock()
		defer lock.Unlock()
		current := viper.AllSettings()
		oldMqttServer := current["mqtt_server"]
		oldMqtt := current["mqtt"]
		oldUdp := current["udp"]
		oldMcp := current["mcp"]
		oldLocalMcp := current["local_mcp"]

		var doMqttServer, doMqttReload, doUdpReload, doMcpReload bool
		if data["mqtt_server"] != nil {
			if !SystemConfigEqual(data["mqtt_server"], oldMqttServer) {
				doMqttServer = true
			}
		}
		if data["mqtt"] != nil {
			if !SystemConfigEqual(data["mqtt"], oldMqtt) {
				doMqttReload = true
			}
		}
		if data["udp"] != nil {
			if udpListenChanged(data["udp"], oldUdp) {
				doUdpReload = true
			}
		}
		if data["mcp"] != nil {
			if !SystemConfigEqual(data["mcp"], oldMcp) {
				doMcpReload = true
			}
		}
		if data["local_mcp"] != nil {
			if !SystemConfigEqual(data["local_mcp"], oldLocalMcp) {
				doMcpReload = true
			}
		}

		ApplySystemConfigToViper(data)

		var wg sync.WaitGroup
		if doMqttServer {
			wg.Add(1)
			go func() {
				defer wg.Done()
				appInstance.ReloadMqttServer()
			}()
		}
		if doMqttReload || doUdpReload {
			wg.Add(1)
			go func() {
				defer wg.Done()
				appInstance.ReloadMqttUdpWithFlags(doMqttReload, doUdpReload)
			}()
		}
		if doMcpReload {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := appInstance.ReloadMCP(); err != nil {
					log.Errorf("ReloadMCP failed: %v", err)
				}
			}()
		}
		wg.Wait()
	})
	appInstance.Run()

	// Chặn và lắng nghe tín hiệu thoát ra.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Info("Máy chủ đang hoạt động. Nhấn Ctrl+C để thoát.")
	<-quit

	log.Info("Đang tắt máy chủ...")

	// Dừng dịch vụ cập nhật cấu hình định kỳ
	StopPeriodicConfigUpdate()
	if *managerEnable {
		StopManagerHTTP()
	}
	if *asrEnable {
		StopAsrServerHTTP()
	}

	log.Info("Máy chủ đã đóng.")
}

func udpListenChanged(newUdpCfg interface{}, oldUdpCfg interface{}) bool {
	newListenHost, newListenPort := udpListenHostPort(newUdpCfg)
	oldListenHost, oldListenPort := udpListenHostPort(oldUdpCfg)
	if newListenHost == "" && newListenPort == 0 {
		return false
	}
	return newListenHost != oldListenHost || newListenPort != oldListenPort
}

func udpListenHostPort(cfg interface{}) (string, int) {
	if cfg == nil {
		return "", 0
	}
	type udpListen struct {
		ListenHost string `json:"listen_host"`
		ListenPort int    `json:"listen_port"`
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", 0
	}
	var parsed udpListen
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", 0
	}
	return parsed.ListenHost, parsed.ListenPort
}
