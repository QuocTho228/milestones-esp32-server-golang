package mqtt_server

import (
	"fmt"
	"strings"
	"time"

	mqttServer "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"

	client "milestones-esp32-server-golang/internal/data/msg"
	log "milestones-esp32-server-golang/logger"
)

// DeviceHook: Hook kiểm soát quyền thiết bị và tự động đăng ký chủ đề.
// Người dùng chỉ được phép đăng ký chủ đề theo quy định, không được đăng ký tùy ý.
// Khi kết nối, hệ thống sẽ tự động đăng ký chủ đề /p2p/device_sub/{mac}.
type DeviceHook struct {
	mqttServer.HookBase
	server           *mqttServer.Server
	publishLifecycle func(event client.MqttLifecycleEvent) error
}

func (h *DeviceHook) ID() string {
	return "custom-device-hook"
}

func (h *DeviceHook) Provides(b byte) bool {
	return b == mqttServer.OnDisconnect || b == mqttServer.OnACLCheck || b == mqttServer.OnSessionEstablished || b == mqttServer.OnSubscribe || b == mqttServer.OnPublish
}

// OnACLCheck: Kiểm soát quyền xuất bản và đăng ký chủ đề.
func (h *DeviceHook) OnACLCheck(cl *mqttServer.Client, topic string, write bool) bool {
	isAdmin := isAdminUser(cl)

	if isAdmin {
		return true // Quản trị viên không bị giới hạn.
	}

	if write {
		// Chỉ cho phép người dùng xuất bản tới topic "device-server".
		if topic == client.MDeviceMockPubTopicPrefix {
			return true
		}
		log.Warnf("Người dùng bị cấm đăng bài lên %s", topic)
		return false
	}

	mac := parseMacFromClientId(cl.ID)
	if mac == "" {
		log.Warnf("Người dùng không được phép đăng ký %s: Không thể phân giải MAC từ ID máy khách, clientID=%s", topic, cl.ID)
		return false
	}

	allowedTopic := deviceSubTopic(mac)
	if topic == allowedTopic {
		return true
	}

	log.Warnf("Cấm người dùng đăng ký %s: Chỉ cho phép đăng ký chủ đề %s của riêng bạn.", topic, allowedTopic)
	return false
}

func (h *DeviceHook) OnConnect(cl *mqttServer.Client, pk packets.Packet) error {
	isAdmin := isAdminUser(cl)
	if isAdmin {
		return nil
	}
	pk.Connect.Clean = true
	return nil
}

func (h *DeviceHook) OnDisconnect(cl *mqttServer.Client, err error, ok bool) {
	if cl == nil {
		log.Warnf("OnDisconnect: Máy khách trống, err=%v, ok=%v", err, ok)
		return
	}
	isAdmin := isAdminUser(cl)
	mac := parseMacFromClientId(cl.ID)
	deviceID := deviceIDFromClientId(cl.ID)
	takenOver := cl.IsTakenOver()

	log.Infof("OnDisconnect: clientID=%s, deviceID=%s, mac=%s, ok=%v, err=%v, takenOver=%v, isAdmin=%v",
		cl.ID, deviceID, mac, ok, err, takenOver, isAdmin)

	if isAdmin {
		return
	}
	if takenOver {
		log.Infof("Máy khách %s đã được tiếp quản bởi một kết nối mới có cùng ID, bỏ qua bước hủy đăng ký và xuất bản vòng đời ngoại tuyến.", cl.ID)
		return
	}
	if mac == "" {
		log.Infof("OnDisconnect: Không thể phân giải địa chỉ MAC từ ID máy khách, clientID=%s, err=%v, ok=%v", cl.ID, err, ok)
		return
	}

	log.Infof("OnDisconnect: Chuẩn bị phát hành vòng đời ngoại tuyến, clientID=%s, deviceID=%s", cl.ID, deviceID)
	h.publishLifecycleEvent(cl.ID, client.MqttLifecycleStateOffline)
	topic := deviceSubTopic(mac)

	action := h.server.Topics.Unsubscribe(topic, cl.ID)
	log.Infof("OnDisconnect: Hủy đăng ký khỏi ứng dụng khách %s theo chủ đề %s, action=%v", cl.ID, topic, action)

	return
}

// OnSessionEstablished: Tự động đăng ký chủ đề sau khi kết nối được thiết lập.
func (h *DeviceHook) OnSessionEstablished(cl *mqttServer.Client, pk packets.Packet) {
	isAdmin := isAdminUser(cl)
	mac := parseMacFromClientId(cl.ID)
	deviceID := deviceIDFromClientId(cl.ID)
	if isAdmin {
		return // Quản trị viên không bị giới hạn.
	}
	if mac == "" {
		log.Info("Cảnh báo: Không thể phân giải địa chỉ MAC từ ID máy khách:", cl.ID)
		return
	}
	log.Infof("OnSessionEstablished: clientID=%s, deviceID=%s, mac=%s, clean=%v", cl.ID, deviceID, mac, pk.Connect.Clean)
	h.publishLifecycleEvent(cl.ID, client.MqttLifecycleStateOnline)

	topic := deviceSubTopic(mac)

	// Đăng ký chủ đề trực tiếp thông qua API của máy chủ, thay vì chèn gói dữ liệu.
	clientID := cl.ID
	exists := h.server.Topics.Subscribe(clientID, packets.Subscription{
		Filter: topic,
		Qos:    0,
	})

	log.Infof("Đăng ký theo dõi chủ đề %s thông qua máy khách %s, exists: %v", topic, clientID, exists)
}

// OnSubscribe: Ghi nhật ký gói đăng ký chủ đề.
func (h *DeviceHook) OnSubscribe(cl *mqttServer.Client, pk packets.Packet) packets.Packet {
	log.Info("=== Đã nhận được gói đăng ký ===")
	log.Infof("ID khách hàng: %s", cl.ID)
	log.Infof("Loại gói: %v", pk.FixedHeader.Type)
	log.Infof("ID gói: %d", pk.PacketID)

	if len(pk.Filters) > 0 {
		log.Info("Thông tin đăng ký:")
		for i, sub := range pk.Filters {
			log.Infof("  %d. chủ đề: %s, QoS: %d", i+1, sub.Filter, sub.Qos)
		}
	}

	log.Info("==================")
	return pk
}

// OnPublish: Ghi nhật ký gói xuất bản.
func (h *DeviceHook) OnPublish(cl *mqttServer.Client, pk packets.Packet) (packets.Packet, error) {
	if cl == nil {
		return pk, nil
	}

	log.Info("=== Đã nhận được gói phát hành ===")
	log.Infof("ID khách hàng: %s", cl.ID)
	log.Infof("Loại gói: %v", pk.FixedHeader.Type)
	log.Infof("ID gói: %d", pk.PacketID)
	log.Infof("Tên chủ đề: %s", pk.TopicName)

	if isAdminUser(cl) {
		return pk, nil
	}

	if len(pk.Payload) > 0 {
		if len(pk.Payload) > 100 {
			// Nếu nội dung tin nhắn quá dài, chỉ hiển thị 100 byte đầu.
			log.Infof("Nội dung tin nhắn (100 byte đầu tiên): %s...", pk.Payload[:100])
		} else {
			log.Infof("Nội dung tin nhắn: %s", pk.Payload)
		}
	} else {
		log.Info("Nội dung tin nhắn: <trống>")
	}

	// Tìm địa chỉ MAC từ client.
	mac := parseMacFromClientId(cl.ID)
	if mac == "" {
		log.Info("Cảnh báo: Không thể phân giải địa chỉ MAC từ ID máy khách:", cl.ID)
		return pk, nil
	}
	forwardTopic := fmt.Sprintf("%s%s", client.MDevicePubTopicPrefix, mac)

	pk.TopicName = forwardTopic

	log.Info("==================")
	return pk, nil
}

// Kiểm tra có phải là quản trị viên cấp cao hay không.
func isAdminUser(cl *mqttServer.Client) bool {
	if cl == nil {
		return false
	}
	return string(cl.Properties.Username) == configuredAdminUsername()
}

// Phân tích clientId để lấy địa chỉ MAC.
func parseMacFromClientId(clientId string) string {
	parts := strings.Split(clientId, "@@@")
	if len(parts) >= 3 {
		return parts[1]
	}
	return ""
}

func deviceIDFromClientId(clientID string) string {
	mac := parseMacFromClientId(clientID)
	if mac == "" {
		return ""
	}
	return strings.ReplaceAll(mac, "_", ":")
}

func (h *DeviceHook) publishLifecycleEvent(clientID string, state string) {
	if h == nil || h.publishLifecycle == nil {
		return
	}
	deviceID := deviceIDFromClientId(clientID)
	if deviceID == "" {
		log.Warnf("Bỏ qua các sự kiện vòng đời MQTT: Không thể phân giải deviceID, clientID=%s, state=%s", clientID, state)
		return
	}
	event := client.MqttLifecycleEvent{
		Type:     client.MqttLifecycleType,
		DeviceID: deviceID,
		State:    state,
		ClientID: clientID,
		Ts:       time.Now().UnixMilli(),
	}
	log.Infof("Phát sự kiện vòng đời MQTT: device=%s, clientID=%s, state=%s, ts=%d", deviceID, clientID, state, event.Ts)
	if err := h.publishLifecycle(event); err != nil {
		log.Warnf("Phát sự kiện vòng đời MQTT thất bại: device=%s state=%s err=%v", deviceID, state, err)
	}
}

func deviceSubTopic(mac string) string {
	return fmt.Sprintf("%s%s", client.MDeviceSubTopicPrefix, mac)
}

// Khởi động tác vụ định kỳ in danh sách các chủ đề đã đăng ký.
func (h *DeviceHook) StartPeriodicSubscriptionPrinter(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			h.PrintAllClientSubscriptions()
		}
	}()
}

// In danh sách các chủ đề mà tất cả client đã đăng ký.
func (h *DeviceHook) PrintAllClientSubscriptions() {
	log.Info("=== Danh sách chủ đề do khách hàng đăng ký ===")
	clients := h.server.Clients.GetAll()
	if len(clients) == 0 {
		log.Info("Hiện không có máy khách nào kết nối.")
		return
	}

	for clientID, _ := range clients {
		log.Infof("Các chủ đề mà khách hàng %s đăng ký theo dõi: ", clientID)

		// Lấy danh sách người đăng ký của tất cả chủ đề bằng server.Topics.Subscribers("+").
		// Sau đó lọc ra các đăng ký tương ứng với clientID hiện tại.
		allSubs := h.server.Topics.Subscribers("+")
		foundTopics := false

		// Kiểm tra các chủ đề mà client đã đăng ký.
		if subs, ok := allSubs.Subscriptions[clientID]; ok {
			log.Infof("  - %s (QoS: %d)", subs.Filter, subs.Qos)
			foundTopics = true
		}

		// Kiểm tra thêm các chủ đề có thể đã được đăng ký.
		allSubs = h.server.Topics.Subscribers("#")
		if subs, ok := allSubs.Subscriptions[clientID]; ok {
			log.Infof("  - %s (QoS: %d)", subs.Filter, subs.Qos)
			foundTopics = true
		}

		// Kiểm tra lại chủ đề cụ thể.
		mac := parseMacFromClientId(clientID)
		if mac != "" {
			topic := deviceSubTopic(mac)
			topicSubs := h.server.Topics.Subscribers(topic)
			if subs, ok := topicSubs.Subscriptions[clientID]; ok {
				log.Infof("  - %s (QoS: %d)", subs.Filter, subs.Qos)
				foundTopics = true
			}
		}

		if !foundTopics {
			log.Info("  Các chủ đề đã hủy đăng ký hoặc không thể truy cập")
		}
	}
	log.Info("=====================")
}
