package mqtt_server

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	mqttServer "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/spf13/viper"

	msg "milestones-esp32-server-golang/internal/data/msg"
	log "milestones-esp32-server-golang/logger"
)

var (
	currentServer *mqttServer.Server
	serverMu      sync.Mutex
)

// StartMqttServer: Khởi động máy chủ MQTT (có thể được gọi lại sau khi StopMqttServer để cập nhật nóng).
func StartMqttServer() error {
	serverMu.Lock()
	defer serverMu.Unlock()
	if currentServer != nil {
		return errors.New("Máy chủ mqtt_server đang chạy. Vui StopMqttServer trước.")
	}
	srv := mqttServer.New(&mqttServer.Options{
		InlineClient: true,
	})

	if err := srv.AddHook(&AuthHook{}, nil); err != nil {
		log.Errorf("Thêm AuthHook không thành công: %v", err)
		return err
	}
	deviceHook := &DeviceHook{
		server: srv,
		publishLifecycle: func(event msg.MqttLifecycleEvent) error {
			payload, err := json.Marshal(event)
			if err != nil {
				return err
			}
			return srv.Publish(msg.MDeviceLifecycleTopic, payload, false, 0)
		},
	}
	if err := srv.AddHook(deviceHook, nil); err != nil {
		log.Errorf("Thêm DeviceHook không thành công: %v", err)
		return err
	}

	if viper.GetBool("mqtt_server.tls.enable") {
		pemFile := viper.GetString("mqtt_server.tls.pem")
		keyFile := viper.GetString("mqtt_server.tls.key")
		cert, err := tls.LoadX509KeyPair(pemFile, keyFile)
		if err != nil {
			log.Errorf("Tải chứng chỉ không thành công: %v", err)
			return err
		}
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		ssltcp := listeners.NewTCP(listeners.Config{
			ID:        "ssl",
			Address:   fmt.Sprintf(":%d", viper.GetInt("mqtt_server.tls.port")),
			TLSConfig: tlsConfig,
		})
		if err := srv.AddListener(ssltcp); err != nil {
			return err
		}
	}

	host := viper.GetString("mqtt_server.listen_host")
	port := viper.GetInt("mqtt_server.listen_port")
	if port == 0 {
		return errors.New("Cấu hình mqtt_server.port không chính xác. Vui lòng kiểm tra lại tệp cấu hình.")
	}
	address := fmt.Sprintf("%s:%d", host, port)
	tcp := listeners.NewTCP(listeners.Config{Type: "tcp", ID: "t1", Address: address})
	if err := srv.AddListener(tcp); err != nil {
		return err
	}

	currentServer = srv
	log.Infof("Máy chủ MQTT khởi động và lắng nghe trên địa chỉ %s...", address)
	go func() {
		// Serve() sẽ trả về ngay sau khi khởi động goroutine listener trong thư viện, không chặn luồng thực thi.
		// Vì vậy, không đặt currentServer về nil tại đây.
		if err := srv.Serve(); err != nil {
			log.Warnf("Thoát MQTT Server Serve: %v", err)
		}
	}()
	return nil
}

// StopMqttServer: Dừng máy chủ MQTT hiện tại để có thể khởi động lại bằng StartMqttServer sau khi cập nhật nóng.
func StopMqttServer() error {
	log.Infof("enter StopMqttServer ")
	defer log.Infof("exit StopMqttServer ")
	serverMu.Lock()
	defer serverMu.Unlock()
	srv := currentServer
	if srv == nil {
		return nil
	}
	// Đưa thao tác Close vào cùng vùng tới hạn để tránh nhiều lệnh Stop đồng thời gọi Close trên cùng một đối tượng.
	if err := srv.Close(); err != nil {
		log.Warnf("StopMqttServer Close: %v", err)
		return err
	}
	currentServer = nil
	log.Info("Máy chủ MQTT đã bị dừng hoạt động.")
	return nil
}
