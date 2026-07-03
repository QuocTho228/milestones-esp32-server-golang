package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"milestones-esp32-server-golang/internal/components/http"
	"milestones-esp32-server-golang/internal/domain/config/types"
	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"
)

var (
	defaultManagerOpenClawEnterKeywords = []string{"Mở OpenClaw", "Khởi động OpenClaw"}
	defaultManagerOpenClawExitKeywords  = []string{"Đóng OpenClaw", "Thoát OpenClaw"}
)

func cloneOpenClawKeywords(keywords []string) []string {
	if len(keywords) == 0 {
		return []string{}
	}
	cloned := make([]string, len(keywords))
	copy(cloned, keywords)
	return cloned
}

func normalizeSpeakerChatMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "identified_only":
		return "identified_only"
	default:
		return "off"
	}
}

// ConfigManager - Trình quản lý cấu hình
// Cung cấp các chức năng quản lý cấu hình cấp cao, bao gồm bộ nhớ đệm, cập nhật nóng, xác thực cấu hình, v.v.
type ConfigManager struct {
	// HTTP client
	client *http.ManagerClient
}

// NewConfigManager - Tạo trình quản lý cấu hình mới
func NewManagerUserConfigProvider(config map[string]interface{}) (*ConfigManager, error) {
	// Lấy URL cơ sở của hệ thống quản trị backend từ cấu hình
	var baseURL string
	if backendUrl := config["backend_url"]; backendUrl != nil {
		baseURL = backendUrl.(string)
	}
	// Nếu không có trong cấu hình, sử dụng giá trị mặc định
	if baseURL == "" {
		baseURL = "http://localhost:8080" // giá trị mặc định
	}

	// Tạo Manager HTTP client
	authToken := util.GetManagerAuthToken()
	if token, ok := config["auth_token"].(string); ok && strings.TrimSpace(token) != "" {
		authToken = strings.TrimSpace(token)
	}
	managerClient := http.NewManagerClient(http.ManagerClientConfig{
		BaseURL:    baseURL,
		AuthToken:  authToken,
		Timeout:    10 * time.Second,
		MaxRetries: 3,
	})

	manager := &ConfigManager{
		client: managerClient,
	}

	//log.Log().Debug("Khởi tạo trình quản lý cấu hình thành công", "backend_url", baseURL)
	return manager, nil
}

func (c *ConfigManager) GetUserConfig(ctx context.Context, deviceID string) (types.UConfig, error) {
	// Phân tích phản hồi
	var response struct {
		Data struct {
			VAD struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"vad"`
			ASR struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"asr"`
			LLM struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"llm"`
			TTS struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"tts"`
			Memory struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"memory"`
			VoiceIdentify map[string]struct {
				ID                 uint     `json:"id"`
				Name               string   `json:"name"`
				Prompt             string   `json:"prompt"`
				Description        string   `json:"description"`
				Uuids              []string `json:"uuids"`
				TTSConfigID        *string  `json:"tts_config_id"`
				Voice              *string  `json:"voice"`
				VoiceModelOverride *string  `json:"voice_model_override"`
			} `json:"voice_identify"`
			KnowledgeBases  []types.KnowledgeBaseRef `json:"knowledge_bases"`
			Prompt          string                   `json:"prompt"`
			AgentId         string                   `json:"agent_id"`
			MemoryMode      string                   `json:"memory_mode"`
			SpeakerChatMode string                   `json:"speaker_chat_mode"`
			MCPServiceNames string                   `json:"mcp_service_names"`
			OpenClaw        struct {
				Allowed       bool     `json:"allowed"`
				EnterKeywords []string `json:"enter_keywords"`
				ExitKeywords  []string `json:"exit_keywords"`
			} `json:"openclaw"`
		} `json:"data"`
	}

	// Gửi yêu cầu HTTP
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method: "GET",
		Path:   "/api/configs",
		QueryParams: map[string]string{
			"device_id": deviceID,
		},
		Response: &response,
	})
	if err != nil {
		log.Log().Error("Lấy cấu hình người dùng thất bại", "error", err, "device_id", deviceID)
		return types.UConfig{}, err
	}

	// Hàm hỗ trợ phân tích dữ liệu cấu hình JSON
	parseJsonData := func(jsonStr string) map[string]interface{} {
		var data map[string]interface{}
		if jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
				log.Log().Warn("Phân tích dữ liệu JSON thất bại", "error", err, "json", jsonStr)
				return make(map[string]interface{})
			}
		}
		return data
	}

	// Lấy thông tin nhóm vân giọng nói từ cấu hình thiết bị (chỉ lấy cấu hình nhóm vân giọng nói, không lấy địa chỉ dịch vụ)
	// VoiceIdentify là một map, key là tên nhóm vân giọng nói, value bao gồm prompt, description và uuids
	voiceIdentifyData := make(map[string]types.SpeakerGroupInfo)
	if len(response.Data.VoiceIdentify) > 0 {
		// Chuyển đổi thông tin nhóm vân giọng nói dạng map sang định dạng cấu hình
		for groupName, groupInfo := range response.Data.VoiceIdentify {
			groupData := types.SpeakerGroupInfo{
				ID:                 groupInfo.ID,
				Name:               groupInfo.Name,
				Prompt:             groupInfo.Prompt,
				Description:        groupInfo.Description,
				Uuids:              groupInfo.Uuids,
				TTSConfigID:        groupInfo.TTSConfigID,
				Voice:              groupInfo.Voice,
				VoiceModelOverride: groupInfo.VoiceModelOverride,
			}
			voiceIdentifyData[groupName] = groupData
		}
	}

	// Xây dựng kết quả cấu hình
	enterKeywords := response.Data.OpenClaw.EnterKeywords
	if len(enterKeywords) == 0 {
		enterKeywords = cloneOpenClawKeywords(defaultManagerOpenClawEnterKeywords)
	}
	exitKeywords := response.Data.OpenClaw.ExitKeywords
	if len(exitKeywords) == 0 {
		exitKeywords = cloneOpenClawKeywords(defaultManagerOpenClawExitKeywords)
	}

	config := types.UConfig{
		SystemPrompt: response.Data.Prompt, // Sử dụng prompt tùy chỉnh của agent
		Asr: types.AsrConfig{
			Provider: response.Data.ASR.Provider,
			Config:   parseJsonData(response.Data.ASR.JsonData),
		},
		Tts: types.TtsConfig{
			Provider: response.Data.TTS.Provider,
			Config:   parseJsonData(response.Data.TTS.JsonData),
		},
		Llm: types.LlmConfig{
			Provider: response.Data.LLM.Provider,
			Config:   parseJsonData(response.Data.LLM.JsonData),
		},
		Vad: types.VadConfig{
			Provider: response.Data.VAD.Provider,
			Config:   parseJsonData(response.Data.VAD.JsonData),
		},
		Memory: types.MemoryConfig{
			Provider: response.Data.Memory.Provider,
			Config:   parseJsonData(response.Data.Memory.JsonData),
		},
		KnowledgeBases:  response.Data.KnowledgeBases,
		VoiceIdentify:   voiceIdentifyData,
		MemoryMode:      response.Data.MemoryMode,
		SpeakerChatMode: response.Data.SpeakerChatMode,
		AgentId:         response.Data.AgentId,
		MCPServiceNames: strings.TrimSpace(response.Data.MCPServiceNames),
		OpenClaw: types.OpenClawConfig{
			Allowed:       response.Data.OpenClaw.Allowed,
			EnterKeywords: enterKeywords,
			ExitKeywords:  exitKeywords,
		},
	}
	if strings.TrimSpace(config.MemoryMode) == "" {
		config.MemoryMode = "short"
	}
	config.SpeakerChatMode = normalizeSpeakerChatMode(config.SpeakerChatMode)

	log.Log().Infof("Lấy cấu hình thiết bị thành công: deviceId: %s, config: %+v", deviceID, config)
	return config, nil
}

// Lấy cấu hình mqtt, mqtt_server, udp, ota, vision
func (c *ConfigManager) GetSystemConfig(ctx context.Context) (string, error) {
	// Phân tích JSON phản hồi
	var apiResponse struct {
		Data map[string]interface{} `json:"data"`
	}

	// Gửi yêu cầu HTTP
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method:   "GET",
		Path:     "/api/system/configs",
		Response: &apiResponse,
	})
	if err != nil {
		return "", fmt.Errorf("Lấy cấu hình hệ thống thất bại: %w", err)
	}

	// Xử lý cấu hình voice_identify, đảm bảo bao gồm trường threshold
	if voiceIdentifyData, exists := apiResponse.Data["voice_identify"]; exists {
		if voiceIdentifyMap, ok := voiceIdentifyData.(map[string]interface{}); ok {
			// Nếu cấu hình voice_identify tồn tại nhưng không có trường threshold, thêm giá trị mặc định
			if _, hasThreshold := voiceIdentifyMap["threshold"]; !hasThreshold {
				voiceIdentifyMap["threshold"] = 0.4
				log.Log().Info("Cấu hình voice_identify thiếu trường threshold, đã thêm giá trị mặc định 0.4")
			} else {
				// Xác thực phạm vi ngưỡng
				if thresholdVal, ok := voiceIdentifyMap["threshold"].(float64); ok {
					if thresholdVal < 0 || thresholdVal > 1 {
						log.Log().Warnf("Giá trị voice_identify.threshold %.4f vượt quá phạm vi hợp lệ [0.0, 1.0], sử dụng giá trị mặc định 0.4", thresholdVal)
						voiceIdentifyMap["threshold"] = 0.4
					}
				}
			}
			// Cập nhật dữ liệu cấu hình
			apiResponse.Data["voice_identify"] = voiceIdentifyMap
		}
	}
	//log.Debugf("Đã lấy cấu hình hệ thống từ internal control: %+v", apiResponse.Data)

	// Chuyển đổi phản hồi API thành chuỗi JSON cấu hình
	configJSON, err := json.Marshal(apiResponse.Data)
	if err != nil {
		return "", fmt.Errorf("Serialize cấu hình thất bại: %w", err)
	}

	return string(configJSON), nil
}

// LoadSystemConfigToViper - Tải cấu hình hệ thống từ backend API và thiết lập vào viper
func (c *ConfigManager) LoadSystemConfigToViper(ctx context.Context) error {
	// Lấy chuỗi JSON cấu hình hệ thống
	configJSON, err := c.GetSystemConfig(ctx)
	if err != nil {
		return fmt.Errorf("Lấy cấu hình hệ thống thất bại: %w", err)
	}

	// Sử dụng viper.MergeConfigMap để thiết lập cấu hình vào viper
	// Đầu tiên phân tích chuỗi JSON thành map
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		return fmt.Errorf("Phân tích JSON cấu hình thất bại: %w", err)
	}

	// Thiết lập vào viper (cần import package viper)
	// viper.MergeConfigMap(configMap)

	log.Log().Info("Đã tải cấu hình hệ thống vào viper thành công", "config_size", len(configJSON))
	return nil
}

// SwitchDeviceRoleByName - Chuyển đổi vai trò thiết bị theo tên vai trò (hỗ trợ khớp mờ)
func (c *ConfigManager) SwitchDeviceRoleByName(ctx context.Context, deviceID string, roleName string) (string, error) {
	deviceID = strings.TrimSpace(deviceID)
	roleName = strings.TrimSpace(roleName)
	if deviceID == "" {
		return "", fmt.Errorf("deviceID không được để trống")
	}
	if roleName == "" {
		return "", fmt.Errorf("roleName không được để trống")
	}

	var response struct {
		Data struct {
			RoleName string `json:"role_name"`
		} `json:"data"`
		Error string `json:"error"`
	}

	path := fmt.Sprintf("/api/internal/devices/%s/switch-role", url.PathEscape(deviceID))
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method: "POST",
		Path:   path,
		Body: map[string]string{
			"role_name": roleName,
		},
		Response: &response,
	})
	if err != nil {
		return "", fmt.Errorf("Chuyển đổi vai trò thiết bị thất bại: %w", err)
	}
	if response.Error != "" {
		return "", fmt.Errorf(response.Error)
	}
	if strings.TrimSpace(response.Data.RoleName) == "" {
		return "", fmt.Errorf("Chuyển đổi vai trò thiết bị thất bại: không trả về vai trò phù hợp")
	}
	return response.Data.RoleName, nil
}

// RestoreDeviceDefaultRole - Khôi phục vai trò mặc định của thiết bị (xóa vai trò đã gán cho thiết bị)
func (c *ConfigManager) RestoreDeviceDefaultRole(ctx context.Context, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return fmt.Errorf("deviceID không được để trống")
	}

	var response struct {
		Error string `json:"error"`
	}

	path := fmt.Sprintf("/api/internal/devices/%s/restore-default-role", url.PathEscape(deviceID))
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method:   "POST",
		Path:     path,
		Response: &response,
	})
	if err != nil {
		return fmt.Errorf("Khôi phục vai trò mặc định thất bại: %w", err)
	}
	if response.Error != "" {
		return fmt.Errorf(response.Error)
	}
	return nil
}

// SearchKnowledge - Tìm kiếm tri thức thống nhất qua backend quản trị (console chuyển tiếp theo provider)
func (c *ConfigManager) NotifyDeviceEvent(ctx context.Context, eventType string, eventData map[string]interface{}) {
	_, err := SendDeviceRequest(ctx, eventType, eventData)
	if err != nil {
		log.Log().Error("Gửi sự kiện thiết bị thất bại", "error", err)
	}
}

func (c *ConfigManager) RegisterMessageEventHandler(ctx context.Context, eventType string, handler types.EventHandler) {
	GetDefaultClient().RegisterMessageHandler(ctx, eventType, handler)
}