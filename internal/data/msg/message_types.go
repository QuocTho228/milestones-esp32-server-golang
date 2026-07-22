package msg

import (
	"encoding/json"

	types_audio "milestones-esp32-server-golang/internal/data/audio"
)

const (
	MDeviceMockPubTopicPrefix = "device-server"
	MDeviceMockSubTopicPrefix = "null"
	MDeviceSubTopicPrefix     = "/p2p/device_sub/"
	MDevicePubTopicPrefix     = "/p2p/device_public/"
	MDeviceLifecycleTopic     = MDevicePubTopicPrefix + "_server/lifecycle"
	MServerSubTopicPrefix     = "/p2p/device_public/#"
	MServerPubTopicPrefix     = MDeviceSubTopicPrefix
)

const (
	MqttLifecycleType         = "mqtt_lifecycle"
	MqttLifecycleStateOnline  = "online"
	MqttLifecycleStateOffline = "offline"
)

// Các hằng số loại tin nhắn.
const (
	MessageTypeHello      = "hello"       // Tin nhắn bắt tay.
	MessageTypeAbort      = "abort"       // Tin nhắn hủy.
	MessageTypeListen     = "listen"      // Tin nhắn lắng nghe.
	MessageTypeIot        = "iot"         // Tin nhắn IoT.
	MessageTypeMcp        = "mcp"         // Tin nhắn MCP.
	MessageTypeGoodBye    = "goodbye"     // Tin nhắn tạm biệt.
	MessageTypeSpeakReady = "speak_ready" // Thiết bị đã sẵn sàng nhận yêu cầu phát thông báo chủ động.
)

// Các hằng số loại tin nhắn từ máy chủ.
const (
	ServerMessageTypeHello        = "hello"         // Tin nhắn bắt tay.
	ServerMessageTypeStt          = "stt"           // Chuyển giọng nói thành văn bản.
	ServerMessageTypeTts          = "tts"           // Chuyển văn bản thành giọng nói.
	ServerMessageTypeIot          = "iot"           // Tin nhắn IoT.
	ServerMessageTypeLlm          = "llm"           // Mô hình ngôn ngữ lớn.
	ServerMessageTypeText         = "text"          // Tin nhắn văn bản.
	ServerMessageTypeGoodBye      = "goodbye"       // Tin nhắn tạm biệt.
	ServerMessageTypeSpeakRequest = "speak_request" // Yêu cầu phát thông báo chủ động.
)

// Các hằng số trạng thái tin nhắn.
const (
	MessageStateStart         = "start"          // Trạng thái bắt đầu.
	MessageStateSentenceStart = "sentence_start" // Trạng thái bắt đầu câu.
	MessageStateSentenceEnd   = "sentence_end"   // Trạng thái kết thúc câu.
	MessageStateStop          = "stop"           // Trạng thái dừng.
	MessageStateDetect        = "detect"         // Trạng thái phát hiện.
	MessageStateAbort         = "abort"          // Trạng thái hủy.
	MessageStateSuccess       = "success"        // Trạng thái thành công.
	MessageStateReady         = "ready"          // Thiết bị đã sẵn sàng.
)

type UdpConfig struct {
	Server string `json:"server"`
	Port   int    `json:"port"`
	Key    string `json:"key"`
	Nonce  string `json:"nonce"`
}

type MqttLifecycleEvent struct {
	Type     string `json:"type"`
	DeviceID string `json:"device_id"`
	State    string `json:"state"`
	ClientID string `json:"client_id,omitempty"`
	Ts       int64  `json:"ts"`
}

// ServerMessage biểu diễn một tin nhắn từ máy chủ.
type ServerMessage struct {
	Type        string                   `json:"type"`
	Text        string                   `json:"text,omitempty"`
	SessionID   string                   `json:"session_id,omitempty"`
	Version     int                      `json:"version"`
	State       string                   `json:"state,omitempty"`
	Transport   string                   `json:"transport,omitempty"`
	AudioFormat *types_audio.AudioFormat `json:"audio_params,omitempty"`
	Emotion     string                   `json:"emotion,omitempty"`
	AutoListen  *bool                    `json:"auto_listen,omitempty"`
	Udp         *UdpConfig               `json:"udp,omitempty"`
	PayLoad     json.RawMessage          `json:"payload,omitempty"`
}
