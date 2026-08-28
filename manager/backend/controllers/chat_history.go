package controllers

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"milestones/manager/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ChatHistoryController struct {
	DB            *gorm.DB
	AudioBasePath string // Đường dẫn cơ sở lưu trữ file âm thanh
	MaxFileSize   int64  // Kích thước file tối đa (10MB)
}

// SaveMessageRequest Yêu cầu lưu tin nhắn
type SaveMessageRequest struct {
	MessageID     string                 `json:"message_id" binding:"required"`
	DeviceID      string                 `json:"device_id" binding:"required"`
	AgentID       string                 `json:"agent_id" binding:"required"`
	SessionID     string                 `json:"session_id,omitempty"`
	Role          string                 `json:"role" binding:"required,oneof=user assistant system tool"`
	Content       string                 `json:"content" binding:"required"`
	ToolCallID    string                 `json:"tool_call_id,omitempty"`    // ID gọi công cụ (dùng cho role Tool)
	ToolCallsJSON *string                `json:"tool_calls_json,omitempty"` // JSON danh sách gọi công cụ (dùng cho role Assistant), nil nghĩa là NULL
	AudioData     string                 `json:"audio_data,omitempty"`      // Mã hóa base64
	AudioFormat   string                 `json:"audio_format,omitempty"`    // Định dạng âm thanh (client truyền vào, backend cố định dùng wav)
	AudioDuration int                    `json:"audio_duration,omitempty"`
	AudioSize     int                    `json:"audio_size,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// SaveMessage Lưu tin nhắn
func (c *ChatHistoryController) SaveMessage(ctx *gin.Context) {
	var req SaveMessageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Xác thực thiết bị tồn tại (truy vấn bằng trường device_name)
	var device models.Device
	if err := c.DB.Where("device_name = ?", req.DeviceID).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Thiết bị không tồn tại"})
			return
		}
		// Lỗi cơ sở dữ liệu khác
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn thiết bị thất bại: " + err.Error()})
		return
	}

	// Nếu yêu cầu không cung cấp AgentID, sử dụng AgentID liên kết với thiết bị
	agentID := req.AgentID
	if agentID == "" && device.AgentID > 0 {
		agentID = fmt.Sprintf("%d", device.AgentID)
	}

	// Nếu AgentID vẫn trống, bỏ qua việc lưu
	if agentID == "" {
		ctx.JSON(http.StatusOK, gin.H{"message": "Bỏ qua lưu: không có AgentID liên kết"})
		return
	}

	message := &models.ChatMessage{
		MessageID:     req.MessageID,
		DeviceID:      req.DeviceID,
		AgentID:       agentID,
		UserID:        device.UserID,
		SessionID:     req.SessionID,
		Role:          req.Role,
		Content:       req.Content,
		ToolCallID:    req.ToolCallID,
		ToolCallsJSON: req.ToolCallsJSON,
		Metadata:      req.Metadata,
	}

	// Kiểm tra tin nhắn đã tồn tại chưa (tránh tạo trùng lặp)
	var existingMessage models.ChatMessage
	err := c.DB.Where("message_id = ?", req.MessageID).First(&existingMessage).Error
	if err == nil {
		// Tin nhắn đã tồn tại, cập nhật dữ liệu âm thanh (nếu được cung cấp)
		if req.AudioData != "" {
			audioPath, err := c.saveAudioFile(req.MessageID, req.AudioData)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu file âm thanh thất bại: " + err.Error()})
				return
			}

			// Nếu trước đó có file âm thanh, xóa trước
			if existingMessage.AudioPath != "" {
				c.deleteAudioFile(existingMessage.AudioPath)
			}

			// Cập nhật tin nhắn
			updates := map[string]interface{}{
				"audio_path":   audioPath,
				"audio_format": "wav",
			}
			if req.AudioSize > 0 {
				updates["audio_size"] = req.AudioSize
			}
			if req.AudioDuration > 0 {
				updates["audio_duration"] = req.AudioDuration
			}

			// Cập nhật metadata (gộp)
			if existingMessage.Metadata == nil {
				existingMessage.Metadata = make(map[string]interface{})
			}
			if req.Metadata != nil {
				for k, v := range req.Metadata {
					existingMessage.Metadata[k] = v
				}
			}
			// Tuần tự hóa metadata thủ công vào MetadataJSON (vì Updates không kích hoạt BeforeSave hook)
			metadataJSONBytes, err := json.Marshal(existingMessage.Metadata)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Tuần tự hóa metadata thất bại: " + err.Error()})
				return
			}
			updates["metadata"] = string(metadataJSONBytes)

			if err := c.DB.Model(&existingMessage).Updates(updates).Error; err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật tin nhắn thất bại"})
				return
			}
			ctx.JSON(http.StatusOK, existingMessage)
			return
		}
		// Tin nhắn đã tồn tại và không có dữ liệu âm thanh, trả về ngay
		ctx.JSON(http.StatusOK, existingMessage)
		return
	} else if err != gorm.ErrRecordNotFound {
		// Lỗi truy vấn (không phải "bản ghi không tồn tại")
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn tin nhắn thất bại: " + err.Error()})
		return
	}

	// Tin nhắn không tồn tại, tạo tin nhắn mới
	// Xử lý dữ liệu âm thanh - lưu vào hệ thống file (cố định định dạng wav, phân tán hai cấp hash)
	if req.AudioData != "" {
		audioPath, err := c.saveAudioFile(req.MessageID, req.AudioData)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu file âm thanh thất bại: " + err.Error()})
			return
		}
		message.AudioPath = audioPath
		message.AudioFormat = "wav" // Cố định định dạng wav
		if req.AudioSize > 0 {
			message.AudioSize = &req.AudioSize
		}
		if req.AudioDuration > 0 {
			message.AudioDuration = &req.AudioDuration
		}
	}

	if err := c.DB.Create(message).Error; err != nil {
		// Nếu lưu cơ sở dữ liệu thất bại, xóa file âm thanh đã lưu
		if message.AudioPath != "" {
			c.deleteAudioFile(message.AudioPath)
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu tin nhắn thất bại: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, message)
}

// GetMessages Lấy danh sách tin nhắn (tổng hợp theo agentId)
func (c *ChatHistoryController) GetMessages(ctx *gin.Context) {
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Chưa được ủy quyền"})
		return
	}

	agentID := ctx.Query("agent_id")
	deviceID := ctx.Query("device_id")
	sessionID := ctx.Query("session_id")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "50"))
	role := ctx.Query("role") // user/assistant

	// Xây dựng truy vấn
	query := c.DB.Model(&models.ChatMessage{}).
		Where("user_id = ? AND is_deleted = ?", userID, false)

	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}
	if sessionID != "" {
		query = query.Where("session_id = ?", sessionID)
	}
	if role != "" {
		query = query.Where("role = ?", role)
	}

	var total int64
	query.Count(&total)

	var messages []models.ChatMessage
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Limit(pageSize).Offset(offset).
		Find(&messages).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn thất bại"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"data":      messages,
	})
}

// DeleteMessage Xóa tin nhắn (xóa mềm, xóa ngay file âm thanh)
func (c *ChatHistoryController) DeleteMessage(ctx *gin.Context) {
	id := ctx.Param("id")

	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Chưa được ủy quyền"})
		return
	}

	// Lấy thông tin tin nhắn
	var message models.ChatMessage
	if err := c.DB.Where("id = ? AND user_id = ?", id, userID).First(&message).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Tin nhắn không tồn tại"})
		return
	}

	// Xóa file âm thanh trước (nếu tồn tại)
	if message.AudioPath != "" {
		if err := c.deleteAudioFile(message.AudioPath); err != nil {
			// Ghi log, nhưng không ảnh hưởng đến thao tác xóa
			log.Printf("Xóa file âm thanh thất bại: %v", err)
		}
	}

	// Xóa mềm tin nhắn
	if err := c.DB.Model(&models.ChatMessage{}).
		Where("id = ?", id).
		Update("is_deleted", true).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa thất bại"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Xóa thành công"})
}

// GetMessagesByAgent Lấy tổng hợp tin nhắn theo AgentID (hỗ trợ lọc)
func (c *ChatHistoryController) GetMessagesByAgent(ctx *gin.Context) {
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Chưa được ủy quyền"})
		return
	}

	agentID := ctx.Param("agent_id")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "50"))
	role := ctx.Query("role")            // user/assistant
	deviceID := ctx.Query("device_id")   // Lọc theo ID thiết bị
	startDate := ctx.Query("start_date") // Ngày bắt đầu YYYY-MM-DD
	endDate := ctx.Query("end_date")     // Ngày kết thúc YYYY-MM-DD

	// Xây dựng truy vấn
	query := c.DB.Model(&models.ChatMessage{}).
		Where("user_id = ? AND agent_id = ? AND is_deleted = ?", userID, agentID, false)

	// Lọc theo vai trò
	if role != "" {
		query = query.Where("role = ?", role)
	}

	// Lọc theo thiết bị
	if deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}

	// Lọc theo khoảng ngày
	if startDate != "" {
		if startTime, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("created_at >= ?", startTime)
		}
	}
	if endDate != "" {
		if endTime, err := time.Parse("2006-01-02", endDate); err == nil {
			// Ngày kết thúc bao gồm cả ngày
			endTime = endTime.Add(24 * time.Hour)
			query = query.Where("created_at < ?", endTime)
		}
	}

	// Tính tổng số
	var total int64
	query.Count(&total)

	// Truy vấn phân trang (theo thời gian giảm dần, mới nhất ở đầu, frontend sẽ đảo mảng để mới nhất ở cuối)
	var messages []models.ChatMessage
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Limit(pageSize).Offset(offset).
		Find(&messages).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn thất bại"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"data":      messages,
	})
}

// ExportMessages Xuất lịch sử chat (định dạng JSON)
func (c *ChatHistoryController) ExportMessages(ctx *gin.Context) {
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Chưa được ủy quyền"})
		return
	}

	agentID := ctx.Query("agent_id")
	deviceID := ctx.Query("device_id")
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")

	// Xây dựng truy vấn
	query := c.DB.Model(&models.ChatMessage{}).
		Where("user_id = ? AND is_deleted = ?", userID, false)

	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}
	if startDate != "" {
		if startTime, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("created_at >= ?", startTime)
		}
	}
	if endDate != "" {
		if endTime, err := time.Parse("2006-01-02", endDate); err == nil {
			// Ngày kết thúc bao gồm cả ngày
			endTime = endTime.Add(24 * time.Hour)
			query = query.Where("created_at < ?", endTime)
		}
	}

	var messages []models.ChatMessage
	if err := query.Order("created_at ASC").Find(&messages).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Xuất thất bại"})
		return
	}

	// Thiết lập header phản hồi, gợi ý tải xuống
	ctx.Header("Content-Type", "application/json")
	ctx.Header("Content-Disposition", "attachment; filename=chat_history_"+time.Now().Format("20060102_150405")+".json")
	ctx.JSON(http.StatusOK, gin.H{
		"export_time": time.Now().Format("2006-01-02 15:04:05"),
		"total":       len(messages),
		"messages":    messages,
	})
}

// saveAudioFile Lưu file âm thanh vào hệ thống file (phân tán hai cấp hash)
func (c *ChatHistoryController) saveAudioFile(messageID, audioDataBase64 string) (string, error) {
	// Giải mã dữ liệu âm thanh base64
	audioData, err := base64.StdEncoding.DecodeString(audioDataBase64)
	if err != nil {
		return "", fmt.Errorf("Giải mã dữ liệu âm thanh thất bại: %v", err)
	}

	// Kiểm tra kích thước file
	if int64(len(audioData)) > c.MaxFileSize {
		return "", fmt.Errorf("Kích thước file âm thanh vượt quá giới hạn: %d > %d", len(audioData), c.MaxFileSize)
	}

	// Tính MD5 của message_id làm tên file (không bao gồm phần mở rộng)
	fileNameHash := fmt.Sprintf("%x", md5.Sum([]byte(messageID)))

	// Tính hai cấp hash để phân tán thư mục
	hash1 := fileNameHash[0:2] // 2 ký tự đầu
	hash2 := fileNameHash[2:4] // Ký tự thứ 3-4

	// Xây dựng đường dẫn file: {base_path}/{hash1}/{hash2}/{md5(message_id)}.wav
	relativePath := fmt.Sprintf("%s/%s/%s.wav", hash1, hash2, fileNameHash)
	fullPath := filepath.Join(c.AudioBasePath, relativePath)

	// Tạo thư mục
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("Tạo thư mục thất bại: %v", err)
	}

	// Ghi file
	if err := os.WriteFile(fullPath, audioData, 0644); err != nil {
		return "", fmt.Errorf("Ghi file thất bại: %v", err)
	}

	// Trả về đường dẫn tương đối (dùng để lưu vào cơ sở dữ liệu)
	return relativePath, nil
}

// deleteAudioFile Xóa file âm thanh
func (c *ChatHistoryController) deleteAudioFile(relativePath string) error {
	fullPath := filepath.Join(c.AudioBasePath, relativePath)
	return os.Remove(fullPath)
}

// GetAudioFile Lấy file âm thanh (chuyển tiếp qua Golang)
func (c *ChatHistoryController) GetAudioFile(ctx *gin.Context) {
	id := ctx.Param("id")

	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Chưa được ủy quyền"})
		return
	}

	// Lấy thông tin tin nhắn
	var message models.ChatMessage
	if err := c.DB.Where("id = ? AND user_id = ? AND is_deleted = ?", id, userID, false).First(&message).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Tin nhắn không tồn tại"})
		return
	}

	if message.AudioPath == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "File âm thanh không tồn tại"})
		return
	}

	// Đọc file
	fullPath := filepath.Join(c.AudioBasePath, message.AudioPath)
	audioData, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "File âm thanh không tồn tại"})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Đọc file âm thanh thất bại"})
		}
		return
	}

	// Thiết lập header phản hồi (định dạng wav)
	ctx.Header("Content-Type", "audio/wav")
	ctx.Header("Content-Length", strconv.Itoa(len(audioData)))
	ctx.Header("Content-Disposition", fmt.Sprintf("inline; filename=%s", filepath.Base(message.AudioPath)))

	// Chuyển tiếp dữ liệu âm thanh
	ctx.Data(http.StatusOK, "audio/wav", audioData)
}

// GetMessagesForInit Lấy danh sách tin nhắn (dùng để tải khởi tạo, API dịch vụ nội bộ, không cần xác thực)
func (c *ChatHistoryController) GetMessagesForInit(ctx *gin.Context) {
	deviceID := ctx.Query("device_id")
	agentID := ctx.Query("agent_id")
	sessionID := ctx.Query("session_id")
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))

	if deviceID == "" || agentID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "device_id và agent_id không được để trống"})
		return
	}

	// Xây dựng truy vấn (không lọc theo user_id, vì đây là API dịch vụ nội bộ)
	query := c.DB.Model(&models.ChatMessage{}).
		Where("device_id = ? AND agent_id = ? AND is_deleted = ?", deviceID, agentID, false)

	if sessionID != "" {
		query = query.Where("session_id = ?", sessionID)
	}

	var messages []models.ChatMessage
	// Lấy N bản ghi mới nhất trước, sau đó đảo ngược thành thứ tự thời gian tăng dần (cũ -> mới) để LLM sử dụng
	if err := query.Order("created_at DESC").
		Order("id DESC").
		Limit(limit).
		Find(&messages).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn thất bại"})
		return
	}

	// Sau khi đảo ngược đảm bảo thứ tự trả về là cũ -> mới
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	// Chuyển đổi sang định dạng phản hồi (chỉ bao gồm văn bản, không bao gồm âm thanh)
	messageItems := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		item := map[string]interface{}{
			"message_id": msg.MessageID,
			"role":       msg.Role,
			"content":    msg.Content,
			"created_at": msg.CreatedAt.Format(time.RFC3339),
		}
		// Trả về trực tiếp tool_call_id (nếu tồn tại)
		if msg.ToolCallID != "" {
			item["tool_call_id"] = msg.ToolCallID
		}
		// Trả về trực tiếp tool_calls (nếu tồn tại)
		if msg.ToolCallsJSON != nil && *msg.ToolCallsJSON != "" {
			var toolCalls []interface{}
			if err := json.Unmarshal([]byte(*msg.ToolCallsJSON), &toolCalls); err == nil {
				item["tool_calls"] = toolCalls
			}
		}
		messageItems = append(messageItems, item)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"messages": messageItems,
	})
}

// UpdateMessageAudioRequest Yêu cầu cập nhật âm thanh tin nhắn
type UpdateMessageAudioRequest struct {
	AudioData   string                 `json:"audio_data" binding:"required"`
	AudioFormat string                 `json:"audio_format"`
	AudioSize   int                    `json:"audio_size"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateMessageAudio Cập nhật âm thanh tin nhắn
func (c *ChatHistoryController) UpdateMessageAudio(ctx *gin.Context) {
	messageID := ctx.Param("message_id")
	if messageID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "message_id không được để trống"})
		return
	}

	var req UpdateMessageAudioRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Tìm tin nhắn
	var message models.ChatMessage
	if err := c.DB.Where("message_id = ?", messageID).First(&message).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Tin nhắn không tồn tại, bỏ qua cập nhật (có thể vì SaveMessage bị bỏ qua do không có AgentID)
			ctx.JSON(http.StatusOK, gin.H{"message": "Bỏ qua cập nhật: tin nhắn không tồn tại"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn tin nhắn thất bại"})
		return
	}

	// Nếu tin nhắn không có AgentID liên kết, bỏ qua cập nhật
	if message.AgentID == "" {
		ctx.JSON(http.StatusOK, gin.H{"message": "Bỏ qua cập nhật: không có AgentID liên kết"})
		return
	}

	// Lưu file âm thanh
	if req.AudioData != "" {
		audioPath, err := c.saveAudioFile(messageID, req.AudioData)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu file âm thanh thất bại: " + err.Error()})
			return
		}

		// Nếu trước đó có file âm thanh, xóa trước
		if message.AudioPath != "" {
			c.deleteAudioFile(message.AudioPath)
		}

		// Cập nhật tin nhắn
		updates := map[string]interface{}{
			"audio_path":   audioPath,
			"audio_format": "wav",
		}
		if req.AudioSize > 0 {
			updates["audio_size"] = req.AudioSize
		}

		// Cập nhật metadata
		if message.Metadata == nil {
			message.Metadata = make(map[string]interface{})
		}
		for k, v := range req.Metadata {
			message.Metadata[k] = v
		}
		// Tuần tự hóa metadata thủ công vào MetadataJSON (vì Updates không kích hoạt BeforeSave hook)
		metadataJSONBytes, err := json.Marshal(message.Metadata)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Tuần tự hóa metadata thất bại: " + err.Error()})
			return
		}
		updates["metadata"] = string(metadataJSONBytes)

		if err := c.DB.Model(&message).Updates(updates).Error; err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật tin nhắn thất bại"})
			return
		}
	}

	ctx.JSON(http.StatusOK, message)
}