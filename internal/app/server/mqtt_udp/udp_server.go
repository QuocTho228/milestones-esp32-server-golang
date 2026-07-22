package mqtt_udp

import (
	"crypto/aes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	. "milestones-esp32-server-golang/logger"
)

// UDPServer Cấu trúc UDP server
/*
type UDPServer struct {
	conn       *net.UDPConn
	sessions   map[string]*Session
	mqttServer *MqttServer
	udpPort    int
	sync.RWMutex
}*/

type UdpServer struct {
	conn           *net.UDPConn
	udpPort        int      //udp server listen port
	externalHost   string   //udp server external host
	externalPort   int      //udp server external port
	connId2Session sync.Map //connId => UdpSession
	mqttAdapter    *MqttUdpAdapter
	sync.RWMutex
}

const maxConnIDGenerateAttempts = 16

var udpRandReader io.Reader = rand.Reader

// NewUDPServer Tạo mới UDP server
func NewUDPServer(udpPort int, externalHost string, externalPort int) *UdpServer {
	return &UdpServer{
		udpPort:        udpPort,
		externalHost:   externalHost,
		externalPort:   externalPort,
		connId2Session: sync.Map{},
	}
}

// Start Khởi động UDP server
func (s *UdpServer) Start() error {
	addr := &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: s.udpPort,
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("Lỗi lắng nghe UDP: %v", err)
	}

	s.conn = conn
	Infof("Máy chủ UDP đã khởi động trong %s:%d", "0.0.0.0", s.udpPort)

	// Khởi động dọn dẹp session
	//go s.cleanupSessions()

	// Khởi động xử lý gói tin
	go s.handlePackets()

	return nil
}

// Close Đóng UDP server, làm cho handlePackets thoát
func (s *UdpServer) Close() error {
	s.Lock()
	conn := s.conn
	s.conn = nil
	s.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close()
}

// handlePackets Xử lý gói tin nhận được
func (s *UdpServer) handlePackets() {
	buffer := make([]byte, 4096) // Dùng kích thước buffer mặc định
	for {
		s.RLock()
		conn := s.conn
		s.RUnlock()
		if conn == nil {
			return
		}
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			s.RLock()
			closed := s.conn == nil
			s.RUnlock()
			if closed {
				return
			}
			Errorf("Không thể đọc dữ liệu UDP: %v", err)
			continue
		}

		// Sao chép dữ liệu, tránh bị sửa đổi đồng thời (concurrent)
		data := make([]byte, n)
		copy(data, buffer[:n])

		// Xử lý gói tin
		s.processPacket(addr, data)
	}
}

func (s *UdpServer) getSessionByConnID(connID string) *UdpSession {
	val, ok := s.connId2Session.Load(connID)
	if ok {
		return val.(*UdpSession)
	}
	return nil
}

// processPacket Xử lý một gói tin đơn lẻ
func (s *UdpServer) processPacket(addr *net.UDPAddr, data []byte) {
	// Kiểm tra kích thước gói tin
	if len(data) < 16 {
		Warn("Gói dữ liệu quá nhỏ.")
		return
	}

	fullNonce := data[:16]
	connID := fullNonce[4:8] // Lấy byte thứ 5-8 làm connection id
	strConnID := hex.EncodeToString(connID)
	udpSession := s.getSessionByConnID(strConnID)
	if udpSession == nil {
		//Warnf("session không tồn tại addr: %s, connID: %s", addr, strConnID)
		return
	}

	// Cập nhật thời gian hoạt động cuối cùng
	udpSession.LastActive = time.Now()

	decrypted, err := udpSession.Decrypt(data)
	if err != nil {
		Errorf("addr: %s Giải mã không thành công: %v", addr, err)
		return
	}
	currentAddr := udpSession.GetRemoteAddr()
	if currentAddr == nil || currentAddr.String() != addr.String() {
		udpSession.SetRemoteAddr(addr)
	}
	Debugf("Dữ liệu âm thanh đã nhận được, addr: %s, kích thước: %d byte", addr, len(decrypted))
	ok, err := udpSession.RecvData(decrypted)
	if err != nil {
		Errorf("addr: %s Không nhận được dữ liệu: %v", addr, err)
		return
	}
	if !ok {
		Warnf("addr: %s Không nhận được dữ liệu, kênh đã đầy", addr)
		return
	}
	/*select {
	case udpSession.RecvChannel <- decrypted:
		return
	default:
		Warnf("udpSession.RecvChannel is full, addr: %s", addr)
	}*/
}

// cleanupSessions Dọn dẹp các session đã hết hạn
func (s *UdpServer) cleanupSessions() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		now := time.Now()
		s.connId2Session.Range(func(key, value interface{}) bool {
			session := value.(*UdpSession)
			if now.Sub(session.LastActive) > 5*time.Minute {
				s.connId2Session.Delete(key)
				Infof("Dọn dẹp các phiên hết hạn: %s", key)
			}
			return true
		})
	}
}

// CreateSession Tạo session mới
func (s *UdpServer) CreateSession(deviceId, clientId string) *UdpSession {
	// Tạo session ID
	sessionID, err := generateSessionID()
	if err != nil {
		Errorf("Không tạo được ID phiên: %v", err)
		return nil
	}

	// Tạo khóa AES
	key := make([]byte, 16)
	if err := fillRandomBytes(key); err != nil {
		Errorf("Không tạo được khóa AES: %v", err)
		return nil
	}

	// Tạo khối (block) AES
	block, err := aes.NewCipher(key)
	if err != nil {
		Errorf("Không tạo được khối AES: %v", err)
		return nil
	}

	// Chuyển key thành [16]byte
	aesKey := [16]byte{}
	copy(aesKey[:], key)

	for attempt := 0; attempt < maxConnIDGenerateAttempts; attempt++ {
		// Tạo connection id 4 byte
		connID := make([]byte, 4)
		if err := fillRandomBytes(connID); err != nil {
			Errorf("Không tạo được ID kết nối: %v", err)
			return nil
		}
		strConnID := hex.EncodeToString(connID)

		// Timestamp 4 byte
		timestamp := make([]byte, 4)
		binary.BigEndian.PutUint32(timestamp, uint32(time.Now().Unix()))

		// Ghép nonce: 4 byte connection id + 4 byte timestamp
		nonce := append(connID, timestamp...)

		// Chuyển nonce thành [8]byte
		nonceBytes := [8]byte{}
		copy(nonceBytes[:], nonce)

		// Tạo session
		session := &UdpSession{
			ID:          sessionID,
			ConnId:      strConnID,
			ClientId:    clientId,
			DeviceId:    deviceId,
			AesKey:      aesKey,
			Nonce:       nonceBytes, // Lưu template nonce gốc
			CreatedAt:   time.Now(),
			LastActive:  time.Now(),
			Block:       block,
			RecvChannel: make(chan []byte, 100),
			SendChannel: make(chan []byte, 100),
			Status:      UdpSessionStatusActive,
			Lock:        sync.Mutex{},
		}

		if _, loaded := s.connId2Session.LoadOrStore(strConnID, session); loaded {
			Warnf("Xung đột connID UDP, đang thử lại: device=%s, connID=%s, attempt=%d", deviceId, strConnID, attempt+1)
			continue
		}

		s.startSessionSender(session)
		return session
	}

	Errorf("Tạo UDP duy nhất connID thất bại: device=%s", deviceId)
	return nil
}

func (s *UdpServer) startSessionSender(session *UdpSession) {
	go func() {
		for data := range session.SendChannel {
			remoteAddr := session.WaitRemoteAddr(2 * time.Second)
			if remoteAddr == nil {
				dropped := 1 + session.DrainPendingAudio()
				Warnf("Địa chỉ từ xa UDP không được thiết lập, âm thanh TTS bị loại bỏ: device=%s, connId=%s, dropped=%d", session.DeviceId, session.ConnId, dropped)
				continue
			}
			encrypted, err := session.Encrypt(data)
			if err != nil {
				Errorf("Mã hóa thất bại: %v", err)
				continue
			}
			_, err = s.writeToUDP(encrypted, remoteAddr)
			if err != nil {
				Errorf("Gửi dữ liệu âm thanh thất bại: %v", err)
				continue
			}
		}
	}()
}

func (s *UdpServer) writeToUDP(data []byte, remoteAddr *net.UDPAddr) (int, error) {
	s.RLock()
	conn := s.conn
	s.RUnlock()
	if conn == nil {
		return 0, fmt.Errorf("udp server is closed")
	}
	return conn.WriteToUDP(data, remoteAddr)
}

// CloseSession Đóng session
func (s *UdpServer) CloseSession(connID string) {
	session := s.getSessionByConnID(connID)
	s.CloseSessionByRef(session)
}

// ClearSessionAddrBinding Xóa binding địa chỉ UDP của session tương ứng với connID, không hủy (destroy) session
func (s *UdpServer) ClearSessionAddrBinding(connID string) {
	session := s.getSessionByConnID(connID)
	if session == nil {
		return
	}
	session.SetRemoteAddr(nil)
}

func (s *UdpServer) SetConnId2Session(connID string, session *UdpSession) {
	Debugf("SetConnId2Session, connID: %s, session: %+v", connID, session)
	s.connId2Session.Store(connID, session)
}

// GetSessionByConnID Lấy thông tin session
func (s *UdpServer) GetSessionByConnID(connID string) *UdpSession {
	val, ok := s.connId2Session.Load(connID)
	if ok {
		return val.(*UdpSession)
	}
	return nil
}

// generateSessionID Tạo session ID
func generateSessionID() (string, error) {
	b := make([]byte, 8)
	if err := fillRandomBytes(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func fillRandomBytes(buffer []byte) error {
	_, err := io.ReadFull(udpRandReader, buffer)
	return err
}

func (s *UdpServer) CloseSessionByRef(session *UdpSession) {
	if session == nil {
		return
	}
	s.connId2Session.Delete(session.ConnId)
	session.SetRemoteAddr(nil)
	session.Destroy()
}