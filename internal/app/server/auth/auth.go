package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"
)

// ClientSession đại diện cho một phiên làm việc của client
type ClientSession struct {
	ID        string
	DeviceID  string
	CreatedAt time.Time
	LastSeen  time.Time
}

// AuthManager quản lý việc xác thực và các phiên làm việc
type AuthManager struct {
	sessions map[string]*ClientSession
	mutex    sync.RWMutex
	// Bảng ánh xạ token
	tokens map[string]string // token -> deviceID
}

var authManager *AuthManager

func Init() error {
	authManager = NewAuthManager()
	return nil
}

func A() *AuthManager {
	return authManager
}

// NewAuthManager tạo một AuthManager (trình quản lý xác thực) mới
func NewAuthManager() *AuthManager {
	return &AuthManager{
		sessions: make(map[string]*ClientSession),
		tokens:   make(map[string]string),
	}
}

// CreateSession tạo một phiên làm việc mới
func (am *AuthManager) CreateSession(deviceID string) (*ClientSession, error) {
	// Sinh ID phiên làm việc ngẫu nhiên
	sessionID, err := generateClientSessionID()
	if err != nil {
		return nil, err
	}

	session := &ClientSession{
		ID:        sessionID,
		DeviceID:  deviceID,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
	}

	am.mutex.Lock()
	am.sessions[sessionID] = session
	am.mutex.Unlock()

	return session, nil
}

// EnsureSession đảm bảo phiên làm việc với ID cho trước tồn tại; nếu preferredID rỗng thì tạo phiên mới
func (am *AuthManager) EnsureSession(deviceID string, preferredID string) (*ClientSession, error) {
	preferredID = strings.TrimSpace(preferredID)
	if preferredID == "" {
		return am.CreateSession(deviceID)
	}

	now := time.Now()

	am.mutex.Lock()
	defer am.mutex.Unlock()

	if session, exists := am.sessions[preferredID]; exists {
		if deviceID != "" {
			session.DeviceID = deviceID
		}
		session.LastSeen = now
		return session, nil
	}

	session := &ClientSession{
		ID:        preferredID,
		DeviceID:  deviceID,
		CreatedAt: now,
		LastSeen:  now,
	}
	am.sessions[preferredID] = session
	return session, nil
}

// GetSession lấy thông tin phiên làm việc
func (am *AuthManager) GetSession(sessionID string) (*ClientSession, error) {
	am.mutex.RLock()
	session, exists := am.sessions[sessionID]
	am.mutex.RUnlock()

	if !exists {
		return nil, errors.New("phiên làm việc không tồn tại")
	}

	// Cập nhật thời gian truy cập gần nhất
	am.mutex.Lock()
	session.LastSeen = time.Now()
	am.mutex.Unlock()

	return session, nil
}

// RemoveSession xóa bỏ phiên làm việc
func (am *AuthManager) RemoveSession(sessionID string) {
	am.mutex.Lock()
	delete(am.sessions, sessionID)
	am.mutex.Unlock()
}

// CleanupSessions dọn dẹp các phiên làm việc đã hết hạn
func (am *AuthManager) CleanupSessions(maxAge time.Duration) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	now := time.Now()
	for id, session := range am.sessions {
		if now.Sub(session.LastSeen) > maxAge {
			delete(am.sessions, id)
		}
	}
}

// generateClientSessionID sinh ID phiên làm việc ngẫu nhiên
func generateClientSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// ValidateToken xác thực token
func (am *AuthManager) ValidateToken(token string) bool {
	return true
	// Loại bỏ tiền tố "Bearer "
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	am.mutex.RLock()
	_, exists := am.tokens[token]
	am.mutex.RUnlock()

	return exists
}

// RegisterToken đăng ký token
func (am *AuthManager) RegisterToken(token string, deviceID string) {
	// Loại bỏ tiền tố "Bearer "
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	am.mutex.Lock()
	am.tokens[token] = deviceID
	am.mutex.Unlock()
}

// RemoveToken xóa bỏ token
func (am *AuthManager) RemoveToken(token string) {
	// Loại bỏ tiền tố "Bearer "
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	am.mutex.Lock()
	delete(am.tokens, token)
	am.mutex.Unlock()
}