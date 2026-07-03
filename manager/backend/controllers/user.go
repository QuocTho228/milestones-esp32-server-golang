package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"milestones/manager/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserController struct {
	DB                  *gorm.DB
	InternalAuthToken   string
	EndpointAuthToken   string
	WebSocketController interface {
		RequestMcpToolDetailsFromClient(ctx context.Context, agentID string) ([]MCPTool, error)
		RequestMcpEndpointStatusFromClient(ctx context.Context, agentID string) (map[string]interface{}, error)
		RequestDeviceMcpToolDetailsFromClient(ctx context.Context, deviceID string) ([]MCPTool, error)
		CallMcpToolFromClient(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error)
		RequestOpenClawStatusFromClient(ctx context.Context, agentID string) (map[string]interface{}, error)
		CallOpenClawChatFromClient(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error)
		CallOpenClawChatStreamFromClient(ctx context.Context, body map[string]interface{}, onResponse func(*WebSocketResponse) error) (map[string]interface{}, error)
		InjectMessageToDevice(ctx context.Context, deviceID, message string, skipLlm bool, autoListen bool) error
	}
}

// UserConfigResponse Phản hồi cấu hình hiển thị cho người dùng (không bao gồm các trường nhạy cảm như json_data)
type UserConfigResponse struct {
	ID        uint      `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	ConfigID  string    `json:"config_id"`
	Provider  string    `json:"provider"`
	Enabled   bool      `json:"enabled"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toUserConfigResponse(cfg *models.Config) *UserConfigResponse {
	if cfg == nil {
		return nil
	}

	return &UserConfigResponse{
		ID:        cfg.ID,
		Type:      cfg.Type,
		Name:      cfg.Name,
		ConfigID:  cfg.ConfigID,
		Provider:  cfg.Provider,
		Enabled:   cfg.Enabled,
		IsDefault: cfg.IsDefault,
		CreatedAt: cfg.CreatedAt,
		UpdatedAt: cfg.UpdatedAt,
	}
}

func toUserConfigResponseList(configs []models.Config) []UserConfigResponse {
	result := make([]UserConfigResponse, 0, len(configs))
	for i := range configs {
		result = append(result, *toUserConfigResponse(&configs[i]))
	}
	return result
}

func normalizeMemoryMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "none":
		return "none"
	case "long":
		return "long"
	default:
		return "short"
	}
}

// Đẩy giọng nói đến thiết bị
func (uc *UserController) InjectMessage(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		DeviceID   string `json:"device_id" binding:"required"`
		Message    string `json:"message" binding:"required"`
		SkipLlm    bool   `json:"skip_llm"`
		AutoListen *bool  `json:"auto_listen"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu không hợp lệ: " + err.Error()})
		return
	}

	// Xác minh thiết bị có thuộc về người dùng hiện tại không
	var device models.Device

	if err := uc.DB.Where("device_name = ? AND user_id = ?", req.DeviceID, userID).First(&device).Error; err != nil {
		log.Printf("[InjectMessage] Truy vấn thiết bị thất bại: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Thiết bị không tồn tại hoặc không thuộc về người dùng hiện tại"})
		return
	}

	autoListen := true
	if req.AutoListen != nil {
		autoListen = *req.AutoListen
	}

	// Gửi yêu cầu đẩy giọng nói đến server chính qua WebSocket
	ctx := context.Background()
	err := uc.WebSocketController.InjectMessageToDevice(ctx, device.DeviceName, req.Message, req.SkipLlm, autoListen)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Đẩy giọng nói thất bại: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Yêu cầu đẩy giọng nói đã được gửi",
		"data": gin.H{
			"device_id":   req.DeviceID,
			"message":     req.Message,
			"skip_llm":    req.SkipLlm,
			"auto_listen": autoListen,
		},
	})
}

// Người dùng tạo thiết bị trực tiếp (không cần mã xác minh)
func (uc *UserController) CreateDevice(c *gin.Context) {
	var req DevicePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu không hợp lệ: " + err.Error()})
		return
	}
	device, err := NewDeviceService(uc.DB).Create(scopeFromContext(c), req)
	if err != nil {
		writeServiceError(c, err, "Tạo thiết bị thất bại")
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Tạo thiết bị thành công",
		"data": gin.H{
			"device_code": device.DeviceCode,
			"device":      device,
		},
	})
}

// Tạo mã số ngẫu nhiên 6 chữ số
func generateRandomCode() string {
	// Tạo số ngẫu nhiên 6 chữ số
	code := fmt.Sprintf("%06d", rand.Intn(1000000))
	return code
}

func isSixDigitCode(value string) bool {
	if len(value) != 6 {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func normalizeDeviceNameCandidate(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", ":"))
}

func normalizeDeviceNickName(value string) (string, error) {
	nickName := strings.TrimSpace(value)
	if len([]rune(nickName)) > 50 {
		return "", fmt.Errorf("Tên hiển thị thiết bị tối đa 50 ký tự")
	}
	return nickName, nil
}

func generateUniqueDeviceCode(db *gorm.DB) string {
	for i := 0; i < 10; i++ { // Thử tối đa 10 lần
		code := generateRandomCode()

		var count int64
		if err := db.Model(&models.Device{}).Where("device_code = ?", code).Count(&count).Error; err == nil && count == 0 {
			return code
		}
	}

	return fmt.Sprintf("%06d", time.Now().Unix()%1000000)
}

// Lấy tổng quan tất cả thiết bị của người dùng (chỉ đọc)
func (uc *UserController) GetMyDevices(c *gin.Context) {
	result, err := NewDeviceService(uc.DB).List(scopeFromContext(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy danh sách thiết bị thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// UpdateDevice Cập nhật tên hiển thị thiết bị của người dùng hiện tại. device_name là định danh phía thiết bị, không được sửa ở đây.
func (uc *UserController) UpdateDevice(c *gin.Context) {
	deviceID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req DevicePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu không hợp lệ: " + err.Error()})
		return
	}
	device, err := NewDeviceService(uc.DB).Update(scopeFromContext(c), deviceID, req)
	if err != nil {
		writeServiceError(c, err, "Cập nhật thiết bị thất bại")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": device})
}

// DeleteDevice Xóa thiết bị của người dùng hiện tại khỏi hệ thống. Sau khi xóa, thiết bị cần thực hiện lại quy trình kích hoạt để có thể vào hệ thống lần nữa.
func (uc *UserController) DeleteDevice(c *gin.Context) {
	deviceID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := NewDeviceService(uc.DB).Delete(scopeFromContext(c), deviceID); err != nil {
		writeServiceError(c, err, "Xóa thiết bị thất bại")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Thiết bị đã bị xóa khỏi hệ thống, cần kích hoạt lại để có thể sử dụng lần nữa"})
}

// Quản lý agent
func (uc *UserController) GetAgents(c *gin.Context) {
	result, err := NewAgentService(uc.DB).List(scopeFromContext(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy danh sách agent thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (uc *UserController) CreateAgent(c *gin.Context) {
	var req AgentPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu không hợp lệ"})
		return
	}
	agent, err := NewAgentService(uc.DB).Create(scopeFromContext(c), req)
	if err != nil {
		writeServiceError(c, err, "Tạo agent thất bại")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"agent": agent, "knowledge_base_ids": agent.KnowledgeBaseIDs}})
}

func (uc *UserController) GetAgent(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	result, err := NewAgentService(uc.DB).Get(scopeFromContext(c), id)
	if err != nil {
		writeServiceError(c, err, "Lấy agent thất bại")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (uc *UserController) UpdateAgent(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req AgentPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu không hợp lệ"})
		return
	}
	agent, err := NewAgentService(uc.DB).Update(scopeFromContext(c), id, req)
	if err != nil {
		writeServiceError(c, err, "Cập nhật agent thất bại")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"agent": agent, "knowledge_base_ids": agent.KnowledgeBaseIDs}})
}

func (uc *UserController) DeleteAgent(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := NewAgentService(uc.DB).Delete(scopeFromContext(c), id); err != nil {
		writeServiceError(c, err, "Xóa agent thất bại")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Xóa thành công"})
}

// Lấy thiết bị liên kết với agent
func (uc *UserController) GetAgentDevices(c *gin.Context) {
	agentID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	devices, err := NewDeviceService(uc.DB).ListByAgent(scopeFromContext(c), agentID)
	if err != nil {
		writeServiceError(c, err, "Lấy danh sách thiết bị thất bại")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": devices})
}

// Thêm thiết bị vào agent
func (uc *UserController) AddDeviceToAgent(c *gin.Context) {
	agentID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req DevicePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu không hợp lệ"})
		return
	}
	device, err := NewDeviceService(uc.DB).BindToAgent(scopeFromContext(c), agentID, req)
	if err != nil {
		writeServiceError(c, err, "Liên kết thiết bị thất bại")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": device})
}

// Gỡ thiết bị khỏi agent
func (uc *UserController) RemoveDeviceFromAgent(c *gin.Context) {
	agentID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	deviceID, ok := parseUintParam(c, "device_id")
	if !ok {
		return
	}
	if err := NewDeviceService(uc.DB).UnbindFromAgent(scopeFromContext(c), agentID, deviceID); err != nil {
		writeServiceError(c, err, "Gỡ thiết bị thất bại")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Gỡ thiết bị thành công"})
}

// Lấy mẫu vai trò
func (uc *UserController) GetRoleTemplates(c *gin.Context) {
	var roles []models.GlobalRole
	if err := uc.DB.Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy mẫu vai trò thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": roles})
}

func trimSuffixFoldForURL(s string, suffix string) string {
	if len(s) < len(suffix) {
		return s
	}
	start := len(s) - len(suffix)
	if strings.EqualFold(s[start:], suffix) {
		return s[:start]
	}
	return s
}

func normalizeIndexTTSVoiceOptionsBaseURL(raw string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	baseURL = trimSuffixFoldForURL(baseURL, "/audio/speech")
	baseURL = trimSuffixFoldForURL(baseURL, "/audio/voices")
	return strings.TrimRight(baseURL, "/")
}

func (uc *UserController) fetchIndexTTSVoices(c *gin.Context, configID, overrideURL, overrideAPIKey string) ([]VoiceOption, error) {
	baseURL := "http://127.0.0.1:7860"
	apiKey := ""
	if strings.TrimSpace(configID) != "" {
		var cfg models.Config
		if err := uc.DB.Where("type = ? AND config_id = ?", "tts", configID).First(&cfg).Error; err == nil {
			var cfgMap map[string]any
			if strings.TrimSpace(cfg.JsonData) != "" && json.Unmarshal([]byte(cfg.JsonData), &cfgMap) == nil {
				if v, ok := cfgMap["api_url"].(string); ok && strings.TrimSpace(v) != "" {
					baseURL = strings.TrimSpace(v)
				}
				if v, ok := cfgMap["api_key"].(string); ok {
					apiKey = strings.TrimSpace(v)
				}
			}
		}
	}
	if strings.TrimSpace(overrideURL) != "" {
		baseURL = strings.TrimSpace(overrideURL)
	}
	if strings.TrimSpace(overrideAPIKey) != "" {
		apiKey = strings.TrimSpace(overrideAPIKey)
	}
	baseURL = normalizeIndexTTSVoiceOptionsBaseURL(baseURL)
	if baseURL == "" {
		baseURL = "http://127.0.0.1:7860"
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, baseURL+indexTTSVoicesEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("IndexTTS lấy giọng nói thất bại: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	voiceMap := map[string]any{}
	if err = json.Unmarshal(body, &voiceMap); err != nil {
		return nil, err
	}
	result := make([]VoiceOption, 0, len(voiceMap))
	normalizedConfigPrefix := strings.ToLower(strings.TrimSpace(configID))
	if normalizedConfigPrefix != "" {
		normalizedConfigPrefix += "_"
	}
	for voice := range voiceMap {
		v := strings.TrimSpace(voice)
		if v == "" {
			continue
		}
		// Lọc bỏ các giọng có tiền tố nội bộ do phiên bản cấu hình IndexTTS hiện tại sinh ra, tránh hiển thị trùng với giọng nhân bản.
		if normalizedConfigPrefix != "" && strings.HasPrefix(strings.ToLower(v), normalizedConfigPrefix) {
			continue
		}
		result = append(result, VoiceOption{Value: v, Label: v})
	}
	return result, nil
}

// Lấy tùy chọn giọng nói
func (uc *UserController) GetVoiceOptions(c *gin.Context) {
	scope := scopeFromContext(c)
	voices, err := getVoiceOptionsForUser(
		uc.DB,
		c,
		scope.ActorUserID,
		c.Query("provider"),
		c.Query("config_id"),
		c.Query("api_url"),
		c.Query("api_key"),
	)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "IndexTTS") {
			status = http.StatusBadGateway
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": voices})
}

// Lấy danh sách cấu hình LLM
func (uc *UserController) GetLLMConfigs(c *gin.Context) {
	var configs []models.Config
	// Lấy tất cả cấu hình LLM đã bật từ cấu hình toàn cục, cấu hình mặc định xếp trước
	if err := uc.DB.Where("type = ? AND enabled = ?", "llm", true).Order("is_default DESC, name ASC").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy cấu hình LLM thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": toUserConfigResponseList(configs)})
}

// Lấy danh sách cấu hình TTS
func (uc *UserController) GetTTSConfigs(c *gin.Context) {
	var configs []models.Config
	// Lấy tất cả cấu hình TTS đã bật từ cấu hình toàn cục, cấu hình mặc định xếp trước
	if err := uc.DB.Where("type = ? AND enabled = ?", "tts", true).Order("is_default DESC, name ASC").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy cấu hình TTS thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": toUserConfigResponseList(configs)})
}

// GetDeviceMcpTools Lấy danh sách công cụ MCP theo thiết bị (phiên bản người dùng)
func (uc *UserController) GetDeviceMcpTools(c *gin.Context) {
	userID, _ := c.Get("user_id")
	deviceID := c.Param("id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id parameter is required"})
		return
	}

	var device models.Device
	if err := uc.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thiết bị không tồn tại hoặc không thuộc về người dùng hiện tại"})
		return
	}

	tools, err := uc.WebSocketController.RequestDeviceMcpToolDetailsFromClient(context.Background(), device.DeviceName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": []interface{}{}}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": tools}})
}

// CallAgentMcpTool Gọi công cụ MCP theo agent (phiên bản người dùng)
func (uc *UserController) CallAgentMcpTool(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")

	var req struct {
		ToolName  string                 `json:"tool_name" binding:"required"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu không hợp lệ: " + err.Error()})
		return
	}

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent không tồn tại hoặc không thuộc về người dùng hiện tại"})
		return
	}

	body := map[string]interface{}{
		"agent_id":  agentID,
		"tool_name": req.ToolName,
		"arguments": req.Arguments,
	}
	result, err := uc.WebSocketController.CallMcpToolFromClient(context.Background(), body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gọi công cụ MCP thất bại: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (uc *UserController) GetAgentMCPServiceOptions(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent không tồn tại"})
		return
	}

	options, err := listEnabledGlobalMCPServiceNames(uc.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Lấy tùy chọn dịch vụ MCP thất bại: %v", err)})
		return
	}

	normalized := normalizeMCPServiceNamesCSV(agent.MCPServiceNames)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"options":           options,
		"selected":          splitMCPServiceNames(normalized),
		"mcp_service_names": normalized,
	}})
}

func (uc *UserController) GetMCPServiceOptions(c *gin.Context) {
	options, err := listEnabledGlobalMCPServiceNames(uc.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Lấy tùy chọn dịch vụ MCP thất bại: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"options": options,
	}})
}

// CallDeviceMcpTool Gọi công cụ MCP theo thiết bị (phiên bản người dùng)
func (uc *UserController) CallDeviceMcpTool(c *gin.Context) {
	userID, _ := c.Get("user_id")
	deviceID := c.Param("id")

	var req struct {
		ToolName  string                 `json:"tool_name" binding:"required"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu không hợp lệ: " + err.Error()})
		return
	}

	var device models.Device
	if err := uc.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thiết bị không tồn tại hoặc không thuộc về người dùng hiện tại"})
		return
	}

	body := map[string]interface{}{
		"device_id": device.DeviceName,
		"tool_name": req.ToolName,
		"arguments": req.Arguments,
	}
	result, err := uc.WebSocketController.CallMcpToolFromClient(context.Background(), body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gọi công cụ MCP thất bại: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GetAgentMCPEndpoint Lấy URL điểm truy cập MCP của agent (phiên bản người dùng)
func (uc *UserController) GetAgentMCPEndpoint(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}

	// Xác minh agent có tồn tại và thuộc về người dùng hiện tại không
	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent không tồn tại hoặc không thuộc về người dùng hiện tại"})
		return
	}

	// Sử dụng hàm chung để tạo điểm truy cập MCP
	endpoint, err := GenerateAgentMCPEndpoint(uc.DB, agentID, userID.(uint), uc.EndpointAuthToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data := newMcpEndpointData(endpoint)
	if uc.WebSocketController == nil {
		data["status_message"] = "websocket controller unavailable"
		c.JSON(http.StatusOK, gin.H{"data": data})
		return
	}

	statusResult, statusErr := uc.WebSocketController.RequestMcpEndpointStatusFromClient(context.Background(), agentID)
	if statusErr != nil {
		data["status_message"] = statusErr.Error()
		c.JSON(http.StatusOK, gin.H{"data": data})
		return
	}

	applyMcpEndpointStatus(data, statusResult)
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// GetAgentOpenClawEndpoint Lấy URL điểm truy cập OpenClaw của agent (phiên bản người dùng)
func (uc *UserController) GetAgentOpenClawEndpoint(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent không tồn tại hoặc không thuộc về người dùng hiện tại"})
		return
	}

	data := gin.H{
		"endpoint":  "",
		"status":    "unknown",
		"connected": false,
	}

	endpoint, err := GenerateAgentOpenClawEndpoint(uc.DB, agentID, userID.(uint), uc.EndpointAuthToken)
	if err != nil {
		data["status_message"] = err.Error()
		c.JSON(http.StatusOK, gin.H{"data": data})
		return
	}
	data["endpoint"] = endpoint

	if uc.WebSocketController == nil {
		data["status_message"] = "websocket controller unavailable"
		c.JSON(http.StatusOK, gin.H{"data": data})
		return
	}

	statusResult, statusErr := uc.WebSocketController.RequestOpenClawStatusFromClient(context.Background(), agentID)
	if statusErr != nil {
		data["status_message"] = statusErr.Error()
		c.JSON(http.StatusOK, gin.H{"data": data})
		return
	}

	connected, _ := statusResult["connected"].(bool)
	status, _ := statusResult["status"].(string)
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		if connected {
			status = "online"
		} else {
			status = "offline"
		}
	}

	data["connected"] = connected
	data["status"] = status
	if msg, ok := statusResult["status_message"].(string); ok && strings.TrimSpace(msg) != "" {
		data["status_message"] = msg
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// CallAgentOpenClawChatTest Gọi kiểm tra hội thoại OpenClaw của agent (phiên bản người dùng)
func (uc *UserController) CallAgentOpenClawChatTest(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}
	if uc.WebSocketController == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "websocket controller unavailable"})
		return
	}

	var req struct {
		Message   string `json:"message" binding:"required"`
		TimeoutMs int    `json:"timeout_ms"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu không hợp lệ: " + err.Error()})
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message không được để trống"})
		return
	}

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent không tồn tại hoặc không thuộc về người dùng hiện tại"})
		return
	}

	body := map[string]interface{}{
		"agent_id": agentID,
		"message":  req.Message,
	}
	if req.TimeoutMs > 0 {
		body["timeout_ms"] = req.TimeoutMs
	}

	if wantsOpenClawSSE(c) {
		if !prepareOpenClawSSE(c) {
			return
		}
		_ = writeOpenClawSSE(c, "start", map[string]interface{}{
			"agent_id": agentID,
		})

		terminalErrorSent := false
		result, err := uc.WebSocketController.CallOpenClawChatStreamFromClient(
			c.Request.Context(),
			body,
			func(resp *WebSocketResponse) error {
				if resp == nil {
					return nil
				}
				payload := map[string]interface{}{
					"status": resp.Status,
				}
				if resp.Body != nil {
					payload["data"] = resp.Body
				}
				if msg := strings.TrimSpace(resp.Error); msg != "" {
					payload["error"] = msg
				}

				switch resp.Status {
				case http.StatusPartialContent:
					return writeOpenClawSSE(c, "chunk", payload)
				case http.StatusOK:
					return writeOpenClawSSE(c, "result", payload)
				default:
					terminalErrorSent = true
					return writeOpenClawSSE(c, "error", payload)
				}
			},
		)
		if err != nil {
			if !terminalErrorSent {
				_ = writeOpenClawSSE(c, "error", map[string]interface{}{
					"error": err.Error(),
				})
			}
			_ = writeOpenClawSSE(c, "done", map[string]interface{}{
				"ok": false,
			})
			return
		}

		_ = writeOpenClawSSE(c, "done", map[string]interface{}{
			"ok":   true,
			"data": result,
		})
		return
	}

	result, err := uc.WebSocketController.CallOpenClawChatFromClient(context.Background(), body)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(strings.ToLower(msg), "not connected"), strings.Contains(msg, "未连接"):
			c.JSON(http.StatusConflict, gin.H{"error": msg})
		case strings.Contains(strings.ToLower(msg), "timeout"), strings.Contains(msg, "超时"):
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": msg})
		case strings.Contains(strings.ToLower(msg), "missing"), strings.Contains(msg, "参数"):
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		case strings.Contains(msg, "没有连接的客户端"):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": msg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gọi kiểm tra hội thoại OpenClaw thất bại: " + msg})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GetAgentMcpTools Lấy danh sách công cụ MCP của agent (phiên bản người dùng)
func (uc *UserController) GetAgentMcpTools(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")

	// Hàm xác minh của người dùng: xác minh agent có tồn tại và thuộc về người dùng hiện tại không
	userAgentValidator := func(agentID string) error {
		var agent models.Agent
		if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
			return fmt.Errorf("Agent không tồn tại hoặc không thuộc về người dùng hiện tại")
		}
		return nil
	}

	// Sử dụng hàm chung
	GetAgentMcpToolsCommon(c, agentID, uc.WebSocketController, userAgentValidator)
}

// Lấy dữ liệu thống kê bảng điều khiển
func (uc *UserController) GetDashboardStats(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("role")

	type DashboardStats struct {
		TotalUsers       int64  `json:"totalUsers"`
		TotalDevices     int64  `json:"totalDevices"`
		TotalAgents      int64  `json:"totalAgents"`
		OnlineDevices    int64  `json:"onlineDevices"`
		ProgramStartedAt string `json:"programStartedAt"`
	}

	stats := DashboardStats{
		ProgramStartedAt: programStartedAt.Format(time.RFC3339),
	}

	if userRole == "admin" {
		// Quản trị viên xem toàn bộ dữ liệu
		uc.DB.Model(&models.User{}).Count(&stats.TotalUsers)
		uc.DB.Model(&models.Device{}).Count(&stats.TotalDevices)
		uc.DB.Model(&models.Agent{}).Count(&stats.TotalAgents)
		// Thiết bị trực tuyến: các thiết bị hoạt động trong 5 phút gần nhất
		fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
		uc.DB.Model(&models.Device{}).Where("last_active_at > ?", fiveMinutesAgo).Count(&stats.OnlineDevices)
	} else {
		// Người dùng chỉ xem dữ liệu của chính mình
		stats.TotalUsers = 0 // Người dùng không hiển thị số lượng người dùng
		uc.DB.Model(&models.Device{}).Where("user_id = ?", userID).Count(&stats.TotalDevices)
		uc.DB.Model(&models.Agent{}).Where("user_id = ?", userID).Count(&stats.TotalAgents)
		// Thiết bị trực tuyến: các thiết bị của người dùng hoạt động trong 5 phút gần nhất
		fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
		uc.DB.Model(&models.Device{}).Where("user_id = ? AND last_active_at > ?", userID, fiveMinutesAgo).Count(&stats.OnlineDevices)
	}

	c.JSON(http.StatusOK, stats)
}