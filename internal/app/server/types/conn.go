package types

import "context"

// IConn là giao diện kết nối độc lập với giao thức, được triển khai bởi các bộ điều hợp như websocket, mqtt_udp...
// Có thể mở rộng thêm các phương thức theo nhu cầu thực tế.

const (
	TransportTypeWebsocket = "websocket"
	TransportTypeMqttUdp   = "udp"
)

type IConn interface {
	// Gửi dữ liệu lệnh/tín hiệu.
	SendCmd(msg []byte) error
	// Nhận dữ liệu lệnh/tín hiệu.
	RecvCmd(ctx context.Context, timeout int) ([]byte, error)
	// Gửi dữ liệu âm thanh.
	SendAudio(audio []byte) error
	// Nhận dữ liệu âm thanh.
	RecvAudio(ctx context.Context, timeout int) ([]byte, error)

	GetDeviceID() string

	Close() error
	OnClose(func(deviceId string))

	CloseAudioChannel() error

	GetTransportType() string

	// Lấy dữ liệu riêng tư.
	GetData(key string) (interface{}, error)
}

type OnNewConnection func(conn IConn)
