package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"milestones/manager/backend/config"
	"milestones/manager/backend/models"
	"milestones/manager/backend/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SpeakerGroupController Controller nhóm vân giọng nói
type SpeakerGroupController struct {
	DB            *gorm.DB
	ServiceURL    string
	HTTPClient    *http.Client
	AudioStorage  *storage.AudioStorage
	HistoryConfig *config.HistoryConfig // Cấu hình lịch sử tin nhắn trò chuyện
}

// NewSpeakerGroupController Tạo controller nhóm vân giọng nói
func NewSpeakerGroupController(db *gorm.DB, cfg *config.Config) *SpeakerGroupController {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	audioStorage := storage.NewAudioStorage(
		cfg.Storage.SpeakerAudioPath,
		cfg.Storage.MaxFileSize,
	)

	return &SpeakerGroupController{
		DB:            db,
		ServiceURL:    cfg.SpeakerService.URL,
		HTTPClient:    httpClient,
		AudioStorage:  audioStorage,
		HistoryConfig: &cfg.History,
	}
}

// CreateSpeakerGroup Tạo nhóm vân giọng nói
func (sgc *SpeakerGroupController) CreateSpeakerGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu thông tin xác thực"})
		return
	}

	var req struct {
		AgentID     uint    `json:"agent_id" binding:"required"`
		Name        string  `json:"name" binding:"required,min=1,max=100"`
		Prompt      string  `json:"prompt"`
		Description string  `json:"description"`
		TTSConfigID *string `json:"tts_config_id"`
		Voice       *string `json:"voice"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu không hợp lệ: " + err.Error()})
		return
	}

	// Xác minh agent có tồn tại và thuộc về người dùng hiện tại không
	var agent models.Agent
	if err := sgc.DB.Where("id = ? AND user_id = ?", req.AgentID, userID).First(&agent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Agent không tồn tại hoặc không có quyền truy cập"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn agent thất bại"})
		return
	}

	// Kiểm tra xem đã tồn tại nhóm vân giọng nói cùng tên dưới cùng người dùng chưa
	var existingGroup models.SpeakerGroup
	if err := sgc.DB.Where("user_id = ? AND name = ?", userID, req.Name).First(&existingGroup).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tên nhóm vân giọng nói này đã tồn tại, vui lòng sử dụng tên khác"})
		return
	} else if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn nhóm vân giọng nói thất bại"})
		return
	}

	// Tạo nhóm vân giọng nói
	speakerGroup := models.SpeakerGroup{
		UserID:      userID.(uint),
		AgentID:     req.AgentID,
		Name:        req.Name,
		Prompt:      req.Prompt,
		Description: req.Description,
		TTSConfigID: req.TTSConfigID,
		Voice:       req.Voice,
		Status:      "active",
		SampleCount: 0,
	}

	if err := sgc.DB.Create(&speakerGroup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo nhóm vân giọng nói thất bại: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":           speakerGroup.ID,
			"agent_id":     speakerGroup.AgentID,
			"name":         speakerGroup.Name,
			"prompt":       speakerGroup.Prompt,
			"description":  speakerGroup.Description,
			"sample_count": speakerGroup.SampleCount,
			"created_at":   speakerGroup.CreatedAt,
		},
	})
}

// GetSpeakerGroups Lấy danh sách nhóm vân giọng nói
func (sgc *SpeakerGroupController) GetSpeakerGroups(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu thông tin xác thực"})
		return
	}

	// Lấy tham số truy vấn
	agentIDStr := c.Query("agent_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	// Xây dựng truy vấn
	query := sgc.DB.Model(&models.SpeakerGroup{}).Where("user_id = ?", userID)

	// Lọc theo agent
	if agentIDStr != "" {
		agentID, err := strconv.ParseUint(agentIDStr, 10, 32)
		if err == nil {
			query = query.Where("agent_id = ?", uint(agentID))
		}
	}

	// Lấy tổng số
	var total int64
	query.Count(&total)

	// Lấy dữ liệu
	var speakerGroups []models.SpeakerGroup
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&speakerGroups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn nhóm vân giọng nói thất bại"})
		return
	}

	// Lấy thông tin agent (dùng để hiển thị tên agent)
	agentIDs := make([]uint, 0)
	for _, sg := range speakerGroups {
		agentIDs = append(agentIDs, sg.AgentID)
	}

	var agents []models.Agent
	if len(agentIDs) > 0 {
		sgc.DB.Where("id IN ?", agentIDs).Find(&agents)
	}

	agentMap := make(map[uint]string)
	for _, agent := range agents {
		agentMap[agent.ID] = agent.Name
	}

	// Xây dựng phản hồi
	result := make([]gin.H, 0)
	for _, sg := range speakerGroups {
		result = append(result, gin.H{
			"id":            sg.ID,
			"agent_id":      sg.AgentID,
			"agent_name":    agentMap[sg.AgentID],
			"name":          sg.Name,
			"prompt":        sg.Prompt,
			"description":   sg.Description,
			"tts_config_id": sg.TTSConfigID,
			"voice":         sg.Voice,
			"sample_count":  sg.SampleCount,
			"created_at":    sg.CreatedAt,
			"updated_at":    sg.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": total,
	})
}

// GetSpeakerGroup Lấy chi tiết nhóm vân giọng nói (bao gồm danh sách mẫu)
func (sgc *SpeakerGroupController) GetSpeakerGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu thông tin xác thực"})
		return
	}

	id := c.Param("id")
	speakerGroupID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID nhóm vân giọng nói không hợp lệ"})
		return
	}

	// Truy vấn nhóm vân giọng nói
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ? AND user_id = ?", speakerGroupID, userID).First(&speakerGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Nhóm vân giọng nói không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn nhóm vân giọng nói thất bại"})
		return
	}

	// Truy vấn thông tin agent
	var agent models.Agent
	sgc.DB.Where("id = ?", speakerGroup.AgentID).First(&agent)

	// Truy vấn danh sách mẫu
	var samples []models.SpeakerSample
	sgc.DB.Where("speaker_group_id = ?", speakerGroupID).Order("created_at DESC").Find(&samples)

	// Xây dựng phản hồi mẫu
	sampleList := make([]gin.H, 0)
	for _, sample := range samples {
		sampleList = append(sampleList, gin.H{
			"id":         sample.ID,
			"uuid":       sample.UUID,
			"file_name":  sample.FileName,
			"file_size":  sample.FileSize,
			"duration":   sample.Duration,
			"file_path":  sample.FilePath,
			"created_at": sample.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":            speakerGroup.ID,
			"agent_id":      speakerGroup.AgentID,
			"agent_name":    agent.Name,
			"name":          speakerGroup.Name,
			"prompt":        speakerGroup.Prompt,
			"description":   speakerGroup.Description,
			"tts_config_id": speakerGroup.TTSConfigID,
			"voice":         speakerGroup.Voice,
			"sample_count":  speakerGroup.SampleCount,
			"samples":       sampleList,
			"created_at":    speakerGroup.CreatedAt,
		},
	})
}

// UpdateSpeakerGroup Cập nhật nhóm vân giọng nói
func (sgc *SpeakerGroupController) UpdateSpeakerGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu thông tin xác thực"})
		return
	}

	id := c.Param("id")
	speakerGroupID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID nhóm vân giọng nói không hợp lệ"})
		return
	}

	var req struct {
		AgentID     *uint   `json:"agent_id"`
		Name        string  `json:"name"`
		Prompt      string  `json:"prompt"`
		Description string  `json:"description"`
		TTSConfigID *string `json:"tts_config_id"`
		Voice       *string `json:"voice"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu không hợp lệ: " + err.Error()})
		return
	}

	// Truy vấn nhóm vân giọng nói
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ? AND user_id = ?", speakerGroupID, userID).First(&speakerGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Nhóm vân giọng nói không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn nhóm vân giọng nói thất bại"})
		return
	}

	// Nếu agent ID được cập nhật, cần xác minh agent mới có tồn tại không
	if req.AgentID != nil && *req.AgentID != speakerGroup.AgentID {
		var agent models.Agent
		if err := sgc.DB.Where("id = ? AND user_id = ?", *req.AgentID, userID).First(&agent).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Agent không tồn tại hoặc không có quyền truy cập"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn agent thất bại"})
			return
		}
		speakerGroup.AgentID = *req.AgentID
	}

	// Cập nhật các trường
	if req.Name != "" && req.Name != speakerGroup.Name {
		// Kiểm tra xem đã tồn tại nhóm vân giọng nói cùng tên dưới cùng người dùng chưa (loại trừ nhóm hiện tại)
		var existingGroup models.SpeakerGroup
		if err := sgc.DB.Where("user_id = ? AND name = ? AND id != ?", userID, req.Name, speakerGroupID).First(&existingGroup).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Tên nhóm vân giọng nói này đã tồn tại, vui lòng sử dụng tên khác"})
			return
		} else if err != gorm.ErrRecordNotFound {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn nhóm vân giọng nói thất bại"})
			return
		}
		speakerGroup.Name = req.Name
	}
	if req.Prompt != "" {
		speakerGroup.Prompt = req.Prompt
	}
	speakerGroup.Description = req.Description // Cho phép xóa trống mô tả
	speakerGroup.TTSConfigID = req.TTSConfigID
	speakerGroup.Voice = req.Voice

	if err := sgc.DB.Save(&speakerGroup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật nhóm vân giọng nói thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    speakerGroup,
	})
}

// DeleteSpeakerGroup Xóa nhóm vân giọng nói
func (sgc *SpeakerGroupController) DeleteSpeakerGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu thông tin xác thực"})
		return
	}

	id := c.Param("id")
	speakerGroupID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID nhóm vân giọng nói không hợp lệ"})
		return
	}

	// Truy vấn nhóm vân giọng nói
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ? AND user_id = ?", speakerGroupID, userID).First(&speakerGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Nhóm vân giọng nói không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn nhóm vân giọng nói thất bại"})
		return
	}

	// Truy vấn tất cả mẫu (dùng để xóa file cục bộ và bản ghi cơ sở dữ liệu)
	var samples []models.SpeakerSample
	sgc.DB.Where("speaker_group_id = ?", speakerGroupID).Find(&samples)

	// Gọi API xóa của asr_server (thông qua speaker_id, tức ID khóa chính của nhóm vân giọng nói, xóa toàn bộ mẫu một lần)
	err = sgc.callDeleteAPI(fmt.Sprintf("%d", speakerGroup.ID), speakerGroup.AgentID, userID)
	if err != nil {
		log.Printf("asr_server xóa nhóm vân giọng nói thất bại (speaker_id: %d): %v", speakerGroup.ID, err)
		// Tiếp tục thực hiện xóa cục bộ, không dừng luồng xử lý
	}

	// Xóa file cục bộ và bản ghi cơ sở dữ liệu của tất cả mẫu
	for _, sample := range samples {
		// Xóa file cục bộ
		sgc.AudioStorage.DeleteAudioFile(sample.FilePath)

		// Xóa bản ghi cơ sở dữ liệu
		sgc.DB.Delete(&sample)
	}

	// Xóa nhóm vân giọng nói
	if err := sgc.DB.Delete(&speakerGroup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa nhóm vân giọng nói thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Xóa nhóm vân giọng nói thành công",
	})
}

// AddSample Thêm mẫu vân giọng nói
func (sgc *SpeakerGroupController) AddSample(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu thông tin xác thực"})
		return
	}

	groupIDStr := c.Param("id") // Đổi thành sử dụng tham số :id
	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID nhóm vân giọng nói không hợp lệ"})
		return
	}

	// Xác minh nhóm vân giọng nói có tồn tại và thuộc về người dùng hiện tại không
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ? AND user_id = ?", groupID, userID).First(&speakerGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Nhóm vân giọng nói không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn nhóm vân giọng nói thất bại"})
		return
	}

	var file multipart.File
	var header *multipart.FileHeader
	var fileName string

	// Kiểm tra xem có lấy âm thanh từ lịch sử tin nhắn trò chuyện không
	messageID := c.PostForm("message_id")
	if messageID != "" {
		// Lấy âm thanh từ lịch sử tin nhắn trò chuyện
		var chatMessage models.ChatMessage
		if err := sgc.DB.Where("message_id = ? AND user_id = ? AND role = ? AND is_deleted = ?",
			messageID, userID, "user", false).First(&chatMessage).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Lịch sử tin nhắn trò chuyện không tồn tại hoặc không phải tin nhắn của người dùng"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn lịch sử tin nhắn trò chuyện thất bại"})
			return
		}

		if chatMessage.AudioPath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Tin nhắn này không có dữ liệu âm thanh"})
			return
		}

		// Đọc file âm thanh
		audioBasePath := sgc.HistoryConfig.AudioBasePath
		if audioBasePath == "" {
			audioBasePath = "./storage/chat_history/audio"
		}
		fullPath := filepath.Join(audioBasePath, chatMessage.AudioPath)

		audioData, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "File âm thanh không tồn tại"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Đọc file âm thanh thất bại: " + err.Error()})
			return
		}

		// Tạo file tạm dùng cho multipart
		tempFile, err := os.CreateTemp("", "audio_*.wav")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo file tạm thất bại: " + err.Error()})
			return
		}
		defer os.Remove(tempFile.Name()) // Dọn dẹp file tạm
		defer tempFile.Close()

		if _, err := tempFile.Write(audioData); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ghi file tạm thất bại: " + err.Error()})
			return
		}
		tempFile.Seek(0, 0)

		// Tạo multipart.File và FileHeader
		file = tempFile
		fileInfo, _ := tempFile.Stat()
		header = &multipart.FileHeader{
			Filename: fmt.Sprintf("history_%s.wav", messageID),
			Size:     fileInfo.Size(),
		}
		fileName = header.Filename
	} else {
		// Lấy âm thanh từ file đã tải lên
		file, header, err = c.Request.FormFile("audio")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Thiếu file âm thanh: " + err.Error()})
			return
		}
		defer file.Close()
		fileName = header.Filename
	}

	// Tạo UUID
	sampleUUID := uuid.New().String()

	// Lưu file âm thanh vào cục bộ
	filePath, savedFileSize, err := sgc.AudioStorage.SaveAudioFile(
		userID.(uint),
		uint(groupID),
		sampleUUID,
		fileName,
		file,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu file âm thanh thất bại: " + err.Error()})
		return
	}

	// Gọi API đăng ký của asr_server
	file.Seek(0, 0) // Đặt lại con trỏ file
	err = sgc.callRegisterAPI(
		fmt.Sprintf("%d", speakerGroup.ID), // speaker_id sử dụng ID khóa chính của nhóm vân giọng nói
		speakerGroup.Name,                  // speaker_name sử dụng tên nhóm
		sampleUUID,
		speakerGroup.AgentID, // agent_id
		file,
		header,
		userID,
	)
	if err != nil {
		// Nếu đăng ký thất bại, xóa file đã lưu
		sgc.AudioStorage.DeleteAudioFile(filePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Đăng ký vân giọng nói thất bại: " + err.Error()})
		return
	}

	// Tạo bản ghi mẫu
	sample := models.SpeakerSample{
		SpeakerGroupID: uint(groupID),
		UserID:         userID.(uint),
		UUID:           sampleUUID,
		FilePath:       filePath,
		FileName:       fileName,
		FileSize:       savedFileSize,
		Status:         "active",
	}

	if err := sgc.DB.Create(&sample).Error; err != nil {
		// Nếu lưu cơ sở dữ liệu thất bại, xóa file và bản ghi trong asr_server
		sgc.AudioStorage.DeleteAudioFile(filePath)
		sgc.callDeleteAPI(sampleUUID, speakerGroup.AgentID, userID, sampleUUID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu bản ghi mẫu thất bại"})
		return
	}

	// Cập nhật số lượng mẫu của nhóm vân giọng nói
	sgc.DB.Model(&speakerGroup).Update("sample_count", gorm.Expr("sample_count + 1"))

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":         sample.ID,
			"uuid":       sample.UUID,
			"file_name":  sample.FileName,
			"file_size":  sample.FileSize,
			"file_path":  sample.FilePath,
			"created_at": sample.CreatedAt,
		},
	})
}

// GetSamples Lấy tất cả mẫu thuộc nhóm vân giọng nói
func (sgc *SpeakerGroupController) GetSamples(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu thông tin xác thực"})
		return
	}

	groupIDStr := c.Param("id") // Đổi thành sử dụng tham số :id
	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID nhóm vân giọng nói không hợp lệ"})
		return
	}

	// Xác minh nhóm vân giọng nói có tồn tại và thuộc về người dùng hiện tại không
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ? AND user_id = ?", groupID, userID).First(&speakerGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Nhóm vân giọng nói không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn nhóm vân giọng nói thất bại"})
		return
	}

	// Truy vấn danh sách mẫu
	var samples []models.SpeakerSample
	if err := sgc.DB.Where("speaker_group_id = ?", groupID).Order("created_at DESC").Find(&samples).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn mẫu thất bại"})
		return
	}

	// Xây dựng phản hồi
	result := make([]gin.H, 0)
	for _, sample := range samples {
		result = append(result, gin.H{
			"id":         sample.ID,
			"uuid":       sample.UUID,
			"file_name":  sample.FileName,
			"file_size":  sample.FileSize,
			"duration":   sample.Duration,
			"file_path":  sample.FilePath,
			"created_at": sample.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}

// DeleteSample Xóa mẫu vân giọng nói
func (sgc *SpeakerGroupController) DeleteSample(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu thông tin xác thực"})
		return
	}

	groupIDStr := c.Param("id") // Đổi thành sử dụng tham số :id
	sampleIDStr := c.Param("sample_id")

	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID nhóm vân giọng nói không hợp lệ"})
		return
	}

	sampleID, err := strconv.ParseUint(sampleIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID mẫu không hợp lệ"})
		return
	}

	// Xác minh mẫu có tồn tại và thuộc về người dùng hiện tại không
	var sample models.SpeakerSample
	if err := sgc.DB.Where("id = ? AND speaker_group_id = ? AND user_id = ?", sampleID, groupID, userID).First(&sample).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mẫu không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn mẫu thất bại"})
		return
	}

	// Truy vấn nhóm vân giọng nói để lấy AgentID
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ?", groupID).First(&speakerGroup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn nhóm vân giọng nói thất bại"})
		return
	}

	// Gọi API xóa của asr_server (thông qua UUID)
	sgc.callDeleteAPI(sample.UUID, speakerGroup.AgentID, userID, sample.UUID)

	// Xóa file cục bộ
	sgc.AudioStorage.DeleteAudioFile(sample.FilePath)

	// Xóa bản ghi cơ sở dữ liệu
	if err := sgc.DB.Delete(&sample).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa mẫu thất bại"})
		return
	}

	// Cập nhật số lượng mẫu của nhóm vân giọng nói
	sgc.DB.Model(&models.SpeakerGroup{}).Where("id = ?", groupID).Update("sample_count", gorm.Expr("sample_count - 1"))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Xóa mẫu thành công",
	})
}

// VerifySpeakerGroup Xác minh nhóm vân giọng nói
func (sgc *SpeakerGroupController) VerifySpeakerGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu thông tin xác thực"})
		return
	}

	id := c.Param("id")
	speakerGroupID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID nhóm vân giọng nói không hợp lệ"})
		return
	}

	// Xác minh nhóm vân giọng nói có tồn tại và thuộc về người dùng hiện tại không
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ? AND user_id = ?", speakerGroupID, userID).First(&speakerGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Nhóm vân giọng nói không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn nhóm vân giọng nói thất bại"})
		return
	}

	// Lấy file âm thanh đã tải lên
	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Thiếu file âm thanh: " + err.Error()})
		return
	}
	defer file.Close()

	// Gọi API xác minh của asr_server
	result, err := sgc.callVerifyAPI(fmt.Sprintf("%d", speakerGroup.ID), speakerGroup.AgentID, file, header, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xác minh thất bại: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"verified":     result.Verified,
			"confidence":   result.Confidence,
			"threshold":    result.Threshold,
			"speaker_id":   fmt.Sprintf("%d", speakerGroup.ID),
			"speaker_name": speakerGroup.Name,
			"message":      sgc.getVerifyMessage(result.Verified, result.Confidence),
		},
	})
}

// getVerifyMessage Tạo thông báo kết quả xác minh
func (sgc *SpeakerGroupController) getVerifyMessage(verified bool, confidence float32) string {
	if verified {
		return fmt.Sprintf("Xác minh thành công, độ tương đồng: %.1f%%", confidence*100)
	}
	return fmt.Sprintf("Xác minh không thành công, độ tương đồng: %.1f%%", confidence*100)
}

// VerifyResult Kết quả xác minh
type VerifyResult struct {
	SpeakerID   string  `json:"speaker_id"`
	SpeakerName string  `json:"speaker_name"`
	Verified    bool    `json:"verified"`
	Confidence  float32 `json:"confidence"`
	Threshold   float32 `json:"threshold"`
}

// callVerifyAPI Gọi API xác minh của asr_server
func (sgc *SpeakerGroupController) callVerifyAPI(speakerID string, agentID uint, file multipart.File, header *multipart.FileHeader, userID interface{}) (*VerifyResult, error) {
	// Chuẩn bị multipart form data
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Thêm file
	part, err := writer.CreateFormFile("audio", header.Filename)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("Tạo trường file thất bại: %v", err)
	}

	// Đặt lại con trỏ file
	file.Seek(0, 0)
	if _, err := io.Copy(part, file); err != nil {
		writer.Close()
		return nil, fmt.Errorf("Sao chép nội dung file thất bại: %v", err)
	}

	writer.Close()

	// Tạo yêu cầu
	apiURL := fmt.Sprintf("%s/api/v1/speaker/verify/%s", sgc.ServiceURL, url.PathEscape(speakerID))
	req, err := http.NewRequest("POST", apiURL, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("Tạo yêu cầu thất bại: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", fmt.Sprintf("%v", userID))
	req.Header.Set("X-Agent-ID", fmt.Sprintf("%d", agentID)) // Thêm header agent_id

	// Gửi yêu cầu
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := sgc.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("Gửi yêu cầu thất bại: %v", err)
	}
	defer resp.Body.Close()

	// Đọc phản hồi
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Đọc phản hồi thất bại: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("asr_server trả về lỗi (mã trạng thái: %d): %s", resp.StatusCode, string(body))
	}

	// Phân tích phản hồi
	var result VerifyResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("Phân tích phản hồi thất bại: %v", err)
	}

	return &result, nil
}

// GetSampleFile Lấy file âm thanh mẫu
func (sgc *SpeakerGroupController) GetSampleFile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu thông tin xác thực"})
		return
	}

	groupIDStr := c.Param("id")
	sampleIDStr := c.Param("sample_id")

	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID nhóm vân giọng nói không hợp lệ"})
		return
	}

	sampleID, err := strconv.ParseUint(sampleIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID mẫu không hợp lệ"})
		return
	}

	// Xác minh mẫu có tồn tại và thuộc về người dùng hiện tại không
	var sample models.SpeakerSample
	if err := sgc.DB.Where("id = ? AND speaker_group_id = ? AND user_id = ?", sampleID, groupID, userID).First(&sample).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mẫu không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn mẫu thất bại"})
		return
	}

	// Kiểm tra xem file có tồn tại không
	if !sgc.AudioStorage.FileExists(sample.FilePath) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File âm thanh không tồn tại"})
		return
	}

	// Mở file
	file, err := sgc.AudioStorage.GetAudioFile(sample.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Đọc file thất bại"})
		return
	}
	defer file.Close()

	// Lấy thông tin file
	fileInfo, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy thông tin file thất bại"})
		return
	}

	// Thiết lập header phản hồi
	c.Header("Content-Type", "audio/wav")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", sample.FileName))
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Trả về nội dung file
	c.File(sample.FilePath)
}

// callRegisterAPI Gọi API đăng ký của asr_server
func (sgc *SpeakerGroupController) callRegisterAPI(speakerID, speakerName, uuid string, agentID uint, file multipart.File, header *multipart.FileHeader, userID interface{}) error {
	// Chuẩn bị multipart form data
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Thêm các trường
	writer.WriteField("speaker_id", speakerID)
	writer.WriteField("speaker_name", speakerName)
	writer.WriteField("uuid", uuid)
	writer.WriteField("agent_id", fmt.Sprintf("%d", agentID)) // Thêm trường agent_id
	writer.WriteField("uid", fmt.Sprintf("%v", userID))

	// Thêm file
	part, err := writer.CreateFormFile("audio", header.Filename)
	if err != nil {
		writer.Close()
		return fmt.Errorf("Tạo trường file thất bại: %v", err)
	}

	// Đặt lại con trỏ file
	file.Seek(0, 0)
	if _, err := io.Copy(part, file); err != nil {
		writer.Close()
		return fmt.Errorf("Sao chép nội dung file thất bại: %v", err)
	}

	writer.Close()

	// Tạo yêu cầu
	url := fmt.Sprintf("%s/api/v1/speaker/register", sgc.ServiceURL)
	req, err := http.NewRequest("POST", url, &requestBody)
	if err != nil {
		return fmt.Errorf("Tạo yêu cầu thất bại: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", fmt.Sprintf("%v", userID))
	req.Header.Set("X-Agent-ID", fmt.Sprintf("%d", agentID)) // Thêm header agent_id

	// Gửi yêu cầu
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := sgc.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("Gửi yêu cầu thất bại: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("asr_server trả về lỗi: %s", string(body))
	}

	return nil
}

// callDeleteAPI Gọi API xóa của asr_server
// speakerID: dùng làm tham số đường dẫn (speaker_id hoặc uuid)
// agentID: Agent ID
// uuid: tùy chọn, nếu được cung cấp sẽ dùng làm tham số truy vấn (dùng để xóa một mẫu cụ thể)
func (sgc *SpeakerGroupController) callDeleteAPI(speakerID string, agentID uint, userID interface{}, uuid ...string) error {
	// Xây dựng URL: tham số đường dẫn sử dụng speakerID
	apiURL := fmt.Sprintf("%s/api/v1/speaker/%s", sgc.ServiceURL, url.PathEscape(speakerID))

	// Xây dựng tham số truy vấn
	queryParams := make([]string, 0)
	if len(uuid) > 0 && uuid[0] != "" {
		queryParams = append(queryParams, fmt.Sprintf("uuid=%s", url.QueryEscape(uuid[0])))
	}
	queryParams = append(queryParams, fmt.Sprintf("agent_id=%d", agentID))

	if len(queryParams) > 0 {
		apiURL += "?" + strings.Join(queryParams, "&")
	}

	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("Tạo yêu cầu thất bại: %v", err)
	}

	req.Header.Set("X-User-ID", fmt.Sprintf("%v", userID))
	req.Header.Set("X-Agent-ID", fmt.Sprintf("%d", agentID)) // Thêm header agent_id

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := sgc.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("Gửi yêu cầu thất bại: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if len(uuid) > 0 && uuid[0] != "" {
			log.Printf("asr_server xóa thất bại (speaker_id: %s, uuid: %s): %s", speakerID, uuid[0], string(body))
		} else {
			log.Printf("asr_server xóa thất bại (speaker_id: %s): %s", speakerID, string(body))
		}
		// Nếu đã cung cấp uuid, không trả về lỗi (có thể đã bị xóa hoặc không tồn tại)
		// Nếu xóa thông qua speaker_id, trả về lỗi
		if len(uuid) == 0 || uuid[0] == "" {
			return fmt.Errorf("asr_server trả về lỗi: %s", string(body))
		}
	}

	return nil
}