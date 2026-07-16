package websocket

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"time"
	"milestones-esp32-server-golang/internal/app/server/types"
	log "milestones-esp32-server-golang/logger"

	"github.com/gorilla/websocket"
)

// WebSocketConn triển khai interface types.IConn, thích ứng cho kết nối WebSocket
type WebSocketConn struct {
	ctx    context.Context
	cancel context.CancelFunc

	onCloseCbList []func(deviceId string)

	conn     *websocket.Conn
	deviceID string

	isMqttUdpBridge bool
	recvCmdChan     chan []byte
	recvAudioChan   chan []byte

	closed bool
	sync.RWMutex
}

// NewWebSocketConn tạo một instance WebSocketConn mới
func NewWebSocketConn(conn *websocket.Conn, deviceID string, isMqttUdpBridge bool) *WebSocketConn {
	ctx, cancel := context.WithCancel(context.Background())
	instance := &WebSocketConn{
		ctx:             ctx,
		cancel:          cancel,
		conn:            conn,
		deviceID:        deviceID,
		isMqttUdpBridge: isMqttUdpBridge,
		recvCmdChan:     make(chan []byte, 100),
		recvAudioChan:   make(chan []byte, 100),
	}

	// Thiết lập bộ xử lý pong (pong handler)
	conn.SetPongHandler(func(appData string) error {
		log.Debugf("Đã nhận thông điệp pong, Device ID: %s", deviceID)
		return nil
	})

	// Khởi động goroutine kiểm tra heartbeat
	go func() {
		ticker := time.NewTicker(30 * time.Second) // Gửi ping mỗi 30 giây
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := instance.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
					log.Errorf("Gửi thông điệp ping thất bại, Device ID: %s, lỗi: %v", deviceID, err)
					// Heartbeat thất bại, đóng kết nối
					for _, cb := range instance.onCloseCbList {
						cb(instance.deviceID)
					}
					return
				}
				log.Debugf("Gửi thông điệp ping thành công, Device ID: %s", deviceID)
			case <-instance.ctx.Done():
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-instance.ctx.Done():
				return
			default:
				msgType, audio, err := instance.conn.ReadMessage()
				if err != nil {
					log.Errorf("read message error: %v", err)
					for _, cb := range instance.onCloseCbList {
						cb(instance.deviceID) //Thông báo cho bên đăng ký thoát
					}
					return
				}

				if msgType == websocket.TextMessage {
					select {
					case instance.recvCmdChan <- audio:
					default:
						log.Errorf("recv cmd channel is full")
					}
				} else if msgType == websocket.BinaryMessage {
					if instance.isMqttUdpBridge {
						audio = instance.tryUnpackUdpBridgeAudioPacket(audio)
					}
					select {
					case instance.recvAudioChan <- audio:
					default:
						log.Errorf("recv audio channel is full")
					}
				}
			}
		}
	}()

	return instance
}

// Thích ứng định dạng dữ liệu của mqtt udp bridge
// 8 byte đầu tiên là 0, byte 12-16 là độ dài dữ liệu âm thanh, sau byte 16 là dữ liệu âm thanh
func (c *WebSocketConn) tryUnpackUdpBridgeAudioPacket(buffer []byte) []byte {
	if len(buffer) < 16 {
		return buffer
	}
	// Kiểm tra 8 byte đầu tiên có phải toàn là 0 hay không
	for i := 0; i < 8; i++ {
		if buffer[i] != 0 {
			return buffer
		}
	}
	dataLen := binary.BigEndian.Uint32(buffer[12:16])
	if int(dataLen) != len(buffer)-16 {
		return buffer
	}
	audioData := buffer[16:]
	return audioData
}

func (c *WebSocketConn) packUdpBridgeAudioPacket(buffer []byte) []byte {
	header := make([]byte, 16)
	// 8 byte đầu toàn là 0, đã được khởi tạo
	// Byte 9~12 ghi timestamp hiện tại (giây)
	timestamp := uint32(time.Now().Unix())
	binary.BigEndian.PutUint32(header[8:12], timestamp)
	// Byte 13~16 ghi độ dài âm thanh
	binary.BigEndian.PutUint32(header[12:16], uint32(len(buffer)))
	// Ghép header và dữ liệu âm thanh
	return append(header, buffer...)
}

func (w *WebSocketConn) SendCmd(msg []byte) error {
	w.Lock()
	defer w.Unlock()

	if w.closed {
		return errors.New("connection is closed")
	}

	log.Debugf("send cmd: %s", string(msg))

	err := w.conn.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		log.Errorf("send cmd error: %v", err)
		return err
	}
	return nil
}

func (w *WebSocketConn) SendAudio(audio []byte) error {
	w.Lock()
	defer w.Unlock()

	if w.closed {
		return errors.New("connection is closed")
	}

	if w.isMqttUdpBridge {
		audio = w.packUdpBridgeAudioPacket(audio)
	}
	err := w.conn.WriteMessage(websocket.BinaryMessage, audio)
	if err != nil {
		log.Errorf("send audio error: %v", err)
		return err
	}
	return nil
}

func (w *WebSocketConn) RecvCmd(ctx context.Context, timeout int) ([]byte, error) {
	for {
		select {
		case <-ctx.Done():
			log.Debugf("recv cmd context done")
			return nil, ctx.Err()
		case msg, ok := <-w.recvCmdChan:
			if !ok {
				return nil, errors.New("connection is closed")
			}
			return msg, nil
		case <-time.After(time.Duration(timeout) * time.Second):
			return nil, errors.New("timeout")
		}
	}
}

func (w *WebSocketConn) RecvAudio(ctx context.Context, timeout int) ([]byte, error) {
	for {
		select {
		case <-ctx.Done():
			log.Debugf("recv audio context done")
			return nil, ctx.Err()
		case audio, ok := <-w.recvAudioChan:
			if !ok {
				return nil, errors.New("connection is closed")
			}
			return audio, nil
		case <-time.After(time.Duration(timeout) * time.Second):
			return nil, errors.New("timeout")
		}
	}
}

func (w *WebSocketConn) Close() error {
	w.Lock()
	defer w.Unlock()

	if w.closed {
		return nil // Already closed
	}

	w.closed = true
	w.cancel()
	w.conn.Close()
	close(w.recvCmdChan)
	close(w.recvAudioChan)
	return nil
}

func (w *WebSocketConn) OnClose(cb func(deviceId string)) {
	w.onCloseCbList = append(w.onCloseCbList, cb)
}

func (w *WebSocketConn) GetDeviceID() string {
	return w.deviceID
}

func (w *WebSocketConn) GetTransportType() string {
	return types.TransportTypeWebsocket
}

func (w *WebSocketConn) GetData(key string) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (w *WebSocketConn) CloseAudioChannel() error {
	return nil
}