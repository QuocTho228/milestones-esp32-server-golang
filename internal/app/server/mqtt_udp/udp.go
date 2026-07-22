package mqtt_udp

import (
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	UdpSessionStatusActive = "active"
	UdpSessionStatusClosed = "closed"
)

// Session Đại diện cho một phiên (session) UDP
type UdpSession struct {
	ID          string
	Conn        *net.UDPConn //udp conn
	ConnId      string
	ClientId    string
	DeviceId    string
	AesKey      [16]byte // Khóa ngẫu nhiên 32 bit
	Nonce       [8]byte  // Lưu trữ template nonce gốc 16 bit
	CreatedAt   time.Time
	LastActive  time.Time
	RemoteAddr  *net.UDPAddr //remote addr
	LocalSeq    uint32
	Block       cipher.Block
	RemoteSeq   uint32
	RecvChannel chan []byte //Dữ liệu audio gửi đi
	SendChannel chan []byte //Dữ liệu audio nhận về
	Status      string
	Lock        sync.Mutex
}

func (s *UdpSession) SetRemoteAddr(addr *net.UDPAddr) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if addr == nil {
		s.RemoteAddr = nil
		return
	}
	addrCopy := *addr
	s.RemoteAddr = &addrCopy
}

func (s *UdpSession) GetRemoteAddr() *net.UDPAddr {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if s.RemoteAddr == nil {
		return nil
	}
	addrCopy := *s.RemoteAddr
	return &addrCopy
}

func (s *UdpSession) IsClosed() bool {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	return s.Status == UdpSessionStatusClosed
}

func (s *UdpSession) WaitRemoteAddr(timeout time.Duration) *net.UDPAddr {
	if timeout <= 0 {
		return s.GetRemoteAddr()
	}

	deadline := time.Now().Add(timeout)
	for {
		if addr := s.GetRemoteAddr(); addr != nil {
			return addr
		}
		if s.IsClosed() || !time.Now().Before(deadline) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (s *UdpSession) DrainPendingAudio() int {
	drained := 0
	for {
		select {
		case <-s.SendChannel:
			drained++
		default:
			return drained
		}
	}
}

// decrypt Giải mã dữ liệu
func (s *UdpSession) Decrypt(data []byte) ([]byte, error) {
	// Tách nonce và ciphertext
	nonce := data[:16] // Dùng nonce 16 byte
	ciphertext := data[16:]

	// Trích xuất số thứ tự (sequence number)
	seqNum := binary.BigEndian.Uint32(data[12:16])

	// Kiểm tra số thứ tự (sequence number)
	/*if seqNum < s.RemoteSeq {
		return nil, fmt.Errorf("Số thứ tự đã hết hạn: got %d, expected >= %d", seqNum, s.RemoteSeq)
	}*/
	s.RemoteSeq = seqNum

	// Giải mã dữ liệu
	stream := cipher.NewCTR(s.Block, nonce)
	decrypted := make([]byte, len(ciphertext))
	stream.XORKeyStream(decrypted, ciphertext)

	return decrypted, nil
}

// encrypt Mã hóa dữ liệu
func (s *UdpSession) Encrypt(data []byte) ([]byte, error) {
	// Cấp phát bộ nhớ trước, tránh phải mở rộng
	encrypted := make([]byte, 16+len(data))

	// Xây dựng nonce (16 byte)
	encrypted[0] = 0x01                                          // Loại gói tin
	binary.BigEndian.PutUint16(encrypted[2:], uint16(len(data))) // Độ dài dữ liệu
	copy(encrypted[4:12], s.Nonce[:])                            // Nonce 8 byte
	s.LocalSeq++
	binary.BigEndian.PutUint32(encrypted[12:], s.LocalSeq) // Số thứ tự (sequence number)

	// Mã hóa dữ liệu
	stream := cipher.NewCTR(s.Block, encrypted[:16]) // Dùng 16 byte làm IV
	stream.XORKeyStream(encrypted[16:], data)

	return encrypted, nil
}

func (s *UdpSession) GetAesKeyAndNonce() (string, string) {
	//Xử lý
	strAesKey := hex.EncodeToString(s.AesKey[:])

	// Xây dựng fullNonce: tiền tố 2 byte 0100 + độ dài 2 byte 0000 + nonce thật (8 byte) + seq (4 byte 00000000)
	prefix := []byte{0x01, 0x00}
	length := []byte{0x00, 0x00}
	seq := []byte{0x00, 0x00, 0x00, 0x00}
	fullNonce := append(append(append(prefix, length...), s.Nonce[:]...), seq...)
	strFullNonce := hex.EncodeToString(fullNonce)

	return strAesKey, strFullNonce
}

func (s *UdpSession) RecvData(data []byte) (bool, error) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if s.Status == UdpSessionStatusClosed {
		return false, nil
	}
	select {
	case s.RecvChannel <- data:
		return true, nil
	default:
		return false, fmt.Errorf("recv channel is full")
	}
}

// SendAudioData Gửi dữ liệu audio
func (s *UdpSession) SendAudioData(data []byte) (bool, error) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if s.Status == UdpSessionStatusClosed {
		return false, nil
	}
	select {
	case s.SendChannel <- data:
		return true, nil
	default:
		return false, fmt.Errorf("send channel is full")
	}
}

func (s *UdpSession) Destroy() {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if s.Status == UdpSessionStatusClosed {
		return
	}
	s.Status = UdpSessionStatusClosed
	close(s.RecvChannel)
	close(s.SendChannel)
}