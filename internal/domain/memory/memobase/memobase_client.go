package memobase

import (
	"context"
	"fmt"
	"strings"
	"sync"

	log "milestones-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/memodb-io/memobase/src/client/memobase-go/blob"
	"github.com/memodb-io/memobase/src/client/memobase-go/core"
)

var (
	clientInstance *MemobaseClient
	once           sync.Once
	configOnce     sync.Once
	// Sử dụng namespace UUID cố định, dùng để sinh UUID v5 cho device ID
	// Nhờ vậy cùng một device ID sẽ luôn ánh xạ về cùng một UUID
	deviceNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // Namespace DNS
)

// MemobaseClient Trình quản lý client của Memobase
type MemobaseClient struct {
	client *core.MemoBaseClient
	users  sync.Map // Cache đối tượng user
	sync.RWMutex
	EnableSearch    bool
	SearchThreshold float64
	SearchTopk      int
}

// GetWithConfig Sử dụng cấu hình để lấy instance của Memobase client (theo mô hình singleton)
func GetWithConfig(config map[string]interface{}) (*MemobaseClient, error) {
	var initErr error
	configOnce.Do(func() {
		iClient := &MemobaseClient{
			users: sync.Map{},
		}
		// Đọc cấu hình liên quan đến memobase từ config		// Đọc các mục cấu hình bắt buộc
		projectUrlInterface, ok := config["base_url"]
		if !ok {
			initErr = fmt.Errorf("thiếu cấu hình memobase.base_url")
			return
		}
		baseUrl, ok := projectUrlInterface.(string)
		if !ok {
			initErr = fmt.Errorf("memobase.base_url phải là chuỗi (string)")
			return
		}

		apiKeyInterface, ok := config["api_key"]
		if !ok {
			initErr = fmt.Errorf("thiếu cấu hình memobase.api_key")
			return
		}
		apiKey, ok := apiKeyInterface.(string)
		if !ok {
			initErr = fmt.Errorf("memobase.api_key phải là chuỗi (string)")
			return
		}

		if baseUrl == "" || apiKey == "" {
			initErr = fmt.Errorf("cấu hình Memobase không đầy đủ: base_url hoặc api_key trống")
			log.Log().Errorf("Khởi tạo Memobase thất bại: %v", initErr)
			return
		}

		// Đọc cấu hình tìm kiếm (search) tùy chọn
		enableSearchInterface, ok := config["enable_search"]
		if ok {
			enableSearch, ok := enableSearchInterface.(bool)
			if ok {
				iClient.EnableSearch = enableSearch
			}
		}

		thresholdInterface, ok := config["search_threshold"]
		if ok {
			threshold, ok := thresholdInterface.(float64)
			if ok {
				iClient.SearchThreshold = threshold
			}
		}

		topKInterface, ok := config["search_topk"]
		if ok {
			topK, ok := topKInterface.(int)
			if ok {
				iClient.SearchTopk = topK
			}
		}

		// Tạo client
		client, err := core.NewMemoBaseClient(baseUrl, apiKey)
		if err != nil {
			initErr = fmt.Errorf("tạo Memobase client thất bại: %v", err)
			log.Log().Errorf("Khởi tạo Memobase thất bại: %v", initErr)
			return
		}

		iClient.client = client
		clientInstance = iClient

		log.Log().Infof("Khởi tạo Memobase client thành công, project_url: %s", baseUrl)
	})

	if initErr != nil {
		return nil, initErr
	}
	return clientInstance, nil
}

// deviceIDToUUID Chuyển đổi device ID sang định dạng UUID v5
// Sử dụng UUID v5 để đảm bảo cùng một device ID luôn sinh ra cùng một UUID
func deviceIDToUUID(deviceID string) string {
	return uuid.NewSHA1(deviceNamespace, []byte(deviceID)).String()
}

func IsEnableSearch() bool {
	return clientInstance.EnableSearch
}

// AddMessage Thêm tin nhắn vào Memobase
func (m *MemobaseClient) AddMessage(ctx context.Context, agentID string, msg schema.Message) error {
	memobaseUserID := deviceIDToUUID(agentID)
	// Xây dựng tin nhắn
	messages := []blob.OpenAICompatibleMessage{
		{
			Role:    string(msg.Role),
			Content: msg.Content,
		},
	}

	// Nếu có tool call, thêm vào tin nhắn
	if len(msg.ToolCalls) > 0 {
		return nil
		/*for _, toolCall := range msg.ToolCalls {
			messages = append(messages, blob.OpenAICompatibleMessage{
				Role:    "tool",
				Content: fmt.Sprintf("Tool: %s, Args: %v", toolCall.Function.Name, toolCall.Function.Arguments),
			})
		}*/
	}

	// Tạo ChatBlob
	chatBlob := &blob.ChatBlob{
		BaseBlob: blob.BaseBlob{
			Type: blob.ChatType,
		},
		Messages: messages,
	}

	// Lấy hoặc tạo user instance (sử dụng userID dạng UUID)
	user, err := m.getUser(memobaseUserID)
	if err != nil {
		log.Log().Errorf("Lấy hoặc tạo user thất bại, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return fmt.Errorf("lấy hoặc tạo user thất bại: %v", err)
	}

	// Chèn (insert) tin nhắn (bất đồng bộ)
	blobID, err := user.Insert(chatBlob, false)
	if err != nil {
		log.Log().Errorf("Thêm tin nhắn vào Memobase thất bại, deviceID: %s, error: %v", agentID, err)
		return fmt.Errorf("thêm tin nhắn vào Memobase thất bại: %v", err)
	}

	//user.Flush(blob.ChatType, false)

	log.Log().Debugf("Thêm tin nhắn vào Memobase thành công, deviceID: %s, blobID: %s", agentID, blobID)
	return nil
}

func (m *MemobaseClient) Flush(ctx context.Context, agentID string) error {
	memobaseUserID := deviceIDToUUID(agentID)
	user, err := m.getUser(memobaseUserID)
	if err != nil {
		log.Log().Errorf("Làm mới (flush) bộ nhớ user thất bại, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return fmt.Errorf("làm mới bộ nhớ user thất bại: %v", err)
	}
	user.Flush(blob.ChatType, false)
	return nil
}

// GetContext Lấy context của người dùng
func (m *MemobaseClient) GetContext(ctx context.Context, agentID string, maxToken int) (string, error) {

	// Chuyển device ID sang định dạng UUID (Memobase yêu cầu)
	memobaseUserID := deviceIDToUUID(agentID)

	// Lấy user instance (không thực hiện HTTP GET request, chỉ tạo instance)
	user, err := m.getUser(memobaseUserID)
	if err != nil {
		log.Log().Errorf("Lấy user instance thất bại, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return "", fmt.Errorf("lấy user instance thất bại: %v", err)
	}

	// Lấy context, sử dụng các tùy chọn mặc định
	context, err := user.Context(&core.ContextOptions{
		MaxTokenSize: maxToken,
	})
	if err != nil {
		log.Log().Errorf("Lấy context từ Memobase thất bại, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return "", fmt.Errorf("lấy context từ Memobase thất bại: %v", err)
	}

	log.Log().Debugf("Lấy context từ Memobase thành công, agentID: %s, độ dài context: %d", agentID, len(context))
	return context, nil
}

func (m *MemobaseClient) Search(ctx context.Context, agentID string, query string, topK int, timeRangeDays int64) (string, error) {
	if !m.EnableSearch {
		return "", nil
	}
	topK = m.SearchTopk
	// Chuyển device ID sang định dạng UUID (Memobase yêu cầu)
	memobaseUserID := deviceIDToUUID(agentID)

	// Lấy user instance (không thực hiện HTTP GET request, chỉ tạo instance)
	user, err := m.getUser(memobaseUserID)
	if err != nil {
		log.Log().Errorf("Lấy user instance thất bại, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return "", fmt.Errorf("lấy user instance thất bại: %v", err)
	}

	topK = 2

	// Tìm kiếm event
	userEventList, err := user.SearchEvent(query, topK, 0.2, int(timeRangeDays))
	if err != nil {
		log.Log().Errorf("Tìm kiếm event từ Memobase thất bại, agentID: %s, error: %v", agentID, err)
		return "", fmt.Errorf("tìm kiếm event từ Memobase thất bại: %v", err)
	}

	var eventList []string
	for _, event := range userEventList {
		eventList = append(eventList, fmt.Sprintf("- %s: %s", event.CreatedAt, event.EventData.EventTip))
	}

	// Chuyển thành chuỗi
	userEventStr := strings.Join(eventList, "\n")

	log.Log().Debugf("Tìm kiếm event từ Memobase thành công, agentID: %s, số lượng event: %d", agentID, len(eventList))
	return userEventStr, nil
}

// AddBatchMessages Thêm hàng loạt tin nhắn vào Memobase
func (m *MemobaseClient) AddBatchMessages(ctx context.Context, userID string, messages []schema.Message) error {
	m.Lock()
	defer m.Unlock()

	if len(messages) == 0 {
		return nil
	}

	// Chuyển đổi định dạng tin nhắn
	blobMessages := make([]blob.OpenAICompatibleMessage, 0, len(messages))
	for _, msg := range messages {
		blobMessages = append(blobMessages, blob.OpenAICompatibleMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	// Tạo ChatBlob
	chatBlob := &blob.ChatBlob{
		BaseBlob: blob.BaseBlob{
			Type: blob.ChatType,
		},
		Messages: blobMessages,
	}

	// Chuyển device ID sang định dạng UUID (Memobase yêu cầu)
	memobaseUserID := deviceIDToUUID(userID)

	// Lấy hoặc tạo user instance (sử dụng userID dạng UUID)
	user, err := m.getUser(userID)
	if err != nil {
		log.Log().Errorf("Thêm hàng loạt tin nhắn: lấy hoặc tạo user thất bại, deviceID: %s, memobaseUserID: %s, error: %v", userID, memobaseUserID, err)
		return fmt.Errorf("lấy hoặc tạo user thất bại: %v", err)
	}

	// Chèn (insert) tin nhắn (bất đồng bộ)
	blobID, err := user.Insert(chatBlob, false)
	if err != nil {
		log.Log().Errorf("Thêm hàng loạt tin nhắn vào Memobase thất bại, deviceID: %s, error: %v", userID, err)
		return fmt.Errorf("thêm hàng loạt tin nhắn vào Memobase thất bại: %v", err)
	}

	log.Log().Debugf("Thêm hàng loạt %d tin nhắn vào Memobase thành công, deviceID: %s, blobID: %s", len(messages), userID, blobID)
	return nil
}

// GetMessages Lấy tin nhắn lịch sử của người dùng
// Triển khai interface BaseMemoryProvider
// Lưu ý: Memobase chủ yếu dùng cho bộ nhớ dài hạn (long-term memory) và tăng cường context, không cung cấp chức năng truy xuất tin nhắn lịch sử
func (m *MemobaseClient) GetMessages(ctx context.Context, agentID string, count int) ([]*schema.Message, error) {
	return []*schema.Message{}, nil
}

// ResetMemory Đặt lại (reset) bộ nhớ của người dùng
// Triển khai interface MemoryProvider
// Lưu ý: Việc reset bộ nhớ của Memobase cần thực hiện thông qua API xóa dữ liệu người dùng
func (m *MemobaseClient) ResetMemory(ctx context.Context, userID string) error {
	// TODO: Nếu Memobase SDK cung cấp interface xóa dữ liệu người dùng, gọi ở đây
	// Hiện tại trả về nil nghĩa là thao tác thành công (dù chưa thực sự xóa)
	log.Log().Infof("Yêu cầu reset bộ nhớ Memobase: userID=%s (lưu ý: Memobase không hỗ trợ reset trực tiếp)", userID)
	return nil
}

// Close Đóng client (nếu cần)
func (m *MemobaseClient) Close() error {
	log.Log().Info("Memobase client đã đóng")
	return nil
}

// todo thêm cache đối tượng user
func (m *MemobaseClient) getUser(userID string) (*core.User, error) {
	if user, ok := m.users.Load(userID); ok {
		return user.(*core.User), nil
	}

	memobaseUserID := deviceIDToUUID(userID)
	user, err := m.client.GetOrCreateUser(memobaseUserID)
	if err != nil {
		return nil, fmt.Errorf("lấy user instance thất bại: %v", err)
	}

	m.users.Store(userID, user)
	return user, nil
}