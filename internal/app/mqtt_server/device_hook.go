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

// DeviceHook 设备权限与自动订阅钩子
// 普通用户禁止随意订阅，只允许发布指定 topic，连接时自动订阅 /p2p/device_sub/{mac}
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

// OnACLCheck 发布/订阅权限控制
func (h *DeviceHook) OnACLCheck(cl *mqttServer.Client, topic string, write bool) bool {
	isAdmin := isAdminUser(cl)

	if isAdmin {
		return true // 超级管理员无限制
	}

	if write {
		// 只允许普通用户发布到 "device-server"
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

// OnSessionEstablished 连接建立后自动订阅
func (h *DeviceHook) OnSessionEstablished(cl *mqttServer.Client, pk packets.Packet) {
	isAdmin := isAdminUser(cl)
	mac := parseMacFromClientId(cl.ID)
	deviceID := deviceIDFromClientId(cl.ID)
	if isAdmin {
		return // 超级管理员不做限制
	}
	if mac == "" {
		log.Info("Cảnh báo: Không thể phân giải địa chỉ MAC từ ID máy khách:", cl.ID)
		return
	}
	log.Infof("OnSessionEstablished: clientID=%s, deviceID=%s, mac=%s, clean=%v", cl.ID, deviceID, mac, pk.Connect.Clean)
	h.publishLifecycleEvent(cl.ID, client.MqttLifecycleStateOnline)

	topic := deviceSubTopic(mac)

	// 使用服务器的API直接订阅，而不是注入数据包
	clientID := cl.ID
	exists := h.server.Topics.Subscribe(clientID, packets.Subscription{
		Filter: topic,
		Qos:    0,
	})

	log.Infof("Đăng ký theo dõi chủ đề %s thông qua máy khách %s, exists: %v", topic, clientID, exists)
}

// OnSubscribe 打印订阅包
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

// OnPublish 打印发布包
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
			// 如果消息太长，只显示前100个字节
			log.Infof("Nội dung tin nhắn (100 byte đầu tiên): %s...", pk.Payload[:100])
		} else {
			log.Infof("Nội dung tin nhắn: %s", pk.Payload)
		}
	} else {
		log.Info("Nội dung tin nhắn: <trống>")
	}

	//从cl中找到mac地址
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

// 判断是否超级管理员
func isAdminUser(cl *mqttServer.Client) bool {
	if cl == nil {
		return false
	}
	return string(cl.Properties.Username) == configuredAdminUsername()
}

// 解析 clientId，获取 mac 地址
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

// 启动周期性打印订阅主题的任务
func (h *DeviceHook) StartPeriodicSubscriptionPrinter(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			h.PrintAllClientSubscriptions()
		}
	}()
}

// 打印所有客户端的订阅主题
func (h *DeviceHook) PrintAllClientSubscriptions() {
	log.Info("=== Danh sách chủ đề do khách hàng đăng ký ===")
	clients := h.server.Clients.GetAll()
	if len(clients) == 0 {
		log.Info("Hiện không có máy khách nào kết nối.")
		return
	}

	for clientID, _ := range clients {
		log.Infof("Các chủ đề mà khách hàng %s đăng ký theo dõi: ", clientID)

		// 使用server.Topics.Subscribers("+")获取所有主题的订阅者
		// 然后过滤出与当前clientID匹配的订阅
		allSubs := h.server.Topics.Subscribers("+")
		foundTopics := false

		// 检查客户端的订阅
		if subs, ok := allSubs.Subscriptions[clientID]; ok {
			log.Infof("  - %s (QoS: %d)", subs.Filter, subs.Qos)
			foundTopics = true
		}

		// 检查更多可能的主题订阅
		allSubs = h.server.Topics.Subscribers("#")
		if subs, ok := allSubs.Subscriptions[clientID]; ok {
			log.Infof("  - %s (QoS: %d)", subs.Filter, subs.Qos)
			foundTopics = true
		}

		// 再检查一下特定主题
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
