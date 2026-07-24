package controllers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"milestones/manager/backend/models"
	"milestones/manager/backend/services/configprovider"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// Hàm hỗ trợ: lấy các keys của map
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func normalizeAgentMemoryMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "none":
		return "none"
	case "long":
		return "long"
	default:
		return "short"
	}
}

func normalizeAgentSpeakerChatMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "identified_only":
		return "identified_only"
	default:
		return "off"
	}
}

func findActiveCloneForVoiceModelOverride(base *gorm.DB, provider, ttsConfigID, voiceID string, clone *models.VoiceClone) error {
	query := base.Where(
		"voice_clones.tts_config_id = ? AND voice_clones.provider_voice_id = ? AND voice_clones.status = ?",
		ttsConfigID,
		voiceID,
		voiceCloneStatusActive,
	)
	if provider == "doubao" {
		query = query.Where("voice_clones.provider IN ?", []string{"doubao", "doubao_ws"})
	} else {
		query = query.Where("voice_clones.provider = ?", provider)
	}
	result := query.
		Order("voice_clones.updated_at DESC, voice_clones.created_at DESC").
		Order("voice_clones.id").
		Limit(1).
		Find(clone)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func getAgentAssistantName(agent models.Agent) string {
	if nickname := strings.TrimSpace(agent.Nickname); nickname != "" {
		return nickname
	}
	return strings.TrimSpace(agent.Name)
}

func ensureAgentNickname(agent *models.Agent) {
	if agent == nil {
		return
	}
	agent.Name = strings.TrimSpace(agent.Name)
	agent.Nickname = strings.TrimSpace(agent.Nickname)
	if agent.Nickname == "" {
		agent.Nickname = agent.Name
	}
}

type AdminController struct {
	DB                  *gorm.DB
	WebSocketController *WebSocketController
	InternalAuthToken   string
	EndpointAuthToken   string
}

var errDatabaseUnavailable = errors.New("database connection is unavailable")

// Quản lý cấu hình chung
// GetDeviceConfigs Lấy thông tin cấu hình liên kết với thiết bị theo ID thiết bị
// Nếu thiết bị không tồn tại, trả về cấu hình mặc định toàn cục
func (ac *AdminController) GetDeviceConfigs(c *gin.Context) {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id parameter is required"})
		return
	}

	// Xây dựng phản hồi cấu hình
	type SpeakerGroupInfo struct {
		ID                 uint     `json:"id"`
		Name               string   `json:"name"`
		Prompt             string   `json:"prompt"`
		Description        string   `json:"description"`
		Uuids              []string `json:"uuids"`
		TTSConfigID        *string  `json:"tts_config_id"`
		Voice              *string  `json:"voice"`
		VoiceModelOverride *string  `json:"voice_model_override,omitempty"`
	}

	type KnowledgeBaseInfo struct {
		ID                 uint     `json:"id"`
		Name               string   `json:"name"`
		Description        string   `json:"description"`
		Provider           string   `json:"provider"`
		ExternalKBID       string   `json:"external_kb_id"`
		ExternalDocID      string   `json:"external_doc_id"`
		RetrievalThreshold *float64 `json:"retrieval_threshold"`
		Status             string   `json:"status"`
	}

	type ConfigResponse struct {
		VAD             models.Config               `json:"vad"`
		ASR             models.Config               `json:"asr"`
		LLM             models.Config               `json:"llm"`
		TTS             models.Config               `json:"tts"`
		Memory          models.Config               `json:"memory"`
		VoiceIdentify   map[string]SpeakerGroupInfo `json:"voice_identify"`
		KnowledgeBases  []KnowledgeBaseInfo         `json:"knowledge_bases"`
		Prompt          string                      `json:"prompt"`
		AgentID         string                      `json:"agent_id"`
		MemoryMode      string                      `json:"memory_mode"`
		SpeakerChatMode string                      `json:"speaker_chat_mode"`
		MCPServiceNames string                      `json:"mcp_service_names"`
		OpenClaw        OpenClawConfigResponse      `json:"openclaw"`
		ConfigSource    string                      `json:"config_source"` // Mới thêm: nguồn cấu hình
	}

	var response ConfigResponse
	response.MemoryMode = "short"
	response.SpeakerChatMode = "off"
	response.OpenClaw = OpenClawConfigResponse{
		Allowed:       false,
		EnterKeywords: []string{},
		ExitKeywords:  []string{},
	}
	var configSource string // ghi lại nguồn cấu hình

	// Tìm thiết bị
	var device models.Device
	var agent models.Agent
	var deviceFound bool

	if err := ac.DB.Where("device_name = ?", deviceID).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Thiết bị không tồn tại, sử dụng cấu hình mặc định toàn cục
			deviceFound = false
			response.AgentID = ""
			configSource = "default_global_role"
			log.Printf("Thiết bị %s không tồn tại, sử dụng cấu hình mặc định toàn cục", deviceID)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query device"})
			return
		}
	} else {
		// Thiết bị tồn tại, tìm agent
		deviceFound = true
		response.AgentID = fmt.Sprintf("%d", device.AgentID)
		log.Printf("Thiết bị %s tồn tại, AgentID: %d", deviceID, device.AgentID)
		if err := ac.DB.First(&agent, device.AgentID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Agent không tồn tại, sử dụng cấu hình mặc định
				deviceFound = false
				configSource = "default_global_role"
				log.Printf("Agent %d không tồn tại, sử dụng cấu hình mặc định toàn cục", device.AgentID)
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query agent"})
				return
			}
		}
	}

	if deviceFound && agent.ID != 0 {
		response.MemoryMode = normalizeAgentMemoryMode(agent.MemoryMode)
		response.SpeakerChatMode = normalizeAgentSpeakerChatMode(agent.SpeakerChatMode)
		response.MCPServiceNames = normalizeMCPServiceNamesCSV(agent.MCPServiceNames)
		response.OpenClaw = buildOpenClawConfigFromAgent(agent)
	}

	cloneVoiceModelCache := make(map[string]string)
	resolveCloneVoiceModelOverride := func(provider, ttsConfigID string, voice *string) *string {
		if device.ID == 0 || device.UserID == 0 {
			return nil
		}
		provider = normalizeCloneProvider(provider)
		if strings.TrimSpace(ttsConfigID) == "" || voice == nil || strings.TrimSpace(*voice) == "" {
			return nil
		}
		if provider != "aliyun_qwen" && provider != "doubao" {
			return nil
		}

		voiceID := strings.TrimSpace(*voice)
		cacheKey := provider + "||" + ttsConfigID + "||" + voiceID
		if cached, exists := cloneVoiceModelCache[cacheKey]; exists {
			if cached == "" {
				return nil
			}
			model := cached
			return &model
		}

		var clone models.VoiceClone
		err := findActiveCloneForVoiceModelOverride(
			ac.DB.Model(&models.VoiceClone{}).Where("voice_clones.user_id = ?", device.UserID),
			provider,
			ttsConfigID,
			voiceID,
			&clone,
		)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Dự phòng: cho phép khớp với giọng nói nhân bản mà quản trị viên chia sẻ cho tất cả mọi người, giải quyết vấn đề thiếu ghi đè model khi người dùng thường sử dụng giọng nói chia sẻ.
			err = findActiveCloneForVoiceModelOverride(
				ac.DB.Model(&models.VoiceClone{}).
					Joins("JOIN users ON users.id = voice_clones.user_id").
					Where("voice_clones.shared_to_all = ? AND users.role = ?", true, "admin"),
				provider,
				ttsConfigID,
				voiceID,
				&clone,
			)
		}
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("Phát hiện ghi đè model giọng nói nhân bản thất bại: provider=%s user_id=%d tts_config_id=%s voice_id=%s err=%v", provider, device.UserID, ttsConfigID, voiceID, err)
			}
			cloneVoiceModelCache[cacheKey] = ""
			return nil
		}

		targetModel := strings.TrimSpace(getTargetModelFromCloneMeta(clone.MetaJSON))
		if targetModel == "" {
			switch provider {
			case "aliyun_qwen":
				targetModel = defaultAliyunQwenCloneTargetModel
			case "doubao":
				targetModel = resolveDoubaoModelSelection("", voiceID).ConfigModel
			}
		}
		cloneVoiceModelCache[cacheKey] = targetModel
		if targetModel == "" {
			return nil
		}
		return &targetModel
	}
	applyCloneVoiceModel := func(provider, ttsConfigID string, voice *string, ttsConfigData map[string]interface{}) {
		if ttsConfigData == nil {
			return
		}
		if override := resolveCloneVoiceModelOverride(provider, ttsConfigID, voice); override != nil && strings.TrimSpace(*override) != "" {
			ttsConfigData["model"] = strings.TrimSpace(*override)
		}
	}
	buildVoiceModelOverride := func(provider string, ttsConfigID *string, voice *string) *string {
		if ttsConfigID == nil {
			return nil
		}
		return resolveCloneVoiceModelOverride(provider, strings.TrimSpace(*ttsConfigID), voice)
	}

	// ==================== Logic lấy cấu hình (có thứ tự ưu tiên) ====================

	// 1. Kiểm tra thiết bị có liên kết với role hay không (ưu tiên cao nhất)
	if device.RoleID != nil {
		var role models.Role
		if err := ac.DB.First(&role, *device.RoleID).Error; err == nil {
			configSource = "device_role"

			// Sử dụng Prompt của role thiết bị
			response.Prompt = role.Prompt
			// Thay thế {{assistant_name}} bằng biệt danh agent (nếu thiết bị có liên kết agent)
			if deviceFound && agent.ID != 0 {
				response.Prompt = strings.ReplaceAll(response.Prompt, "{{assistant_name}}", getAgentAssistantName(agent))
			}

			// Sử dụng cấu hình LLM của role thiết bị
			if role.LLMConfigID != nil && *role.LLMConfigID != "" {
				if err := ac.DB.Where("config_id = ? AND type = ? AND enabled = ?",
					*role.LLMConfigID, "llm", true).First(&response.LLM).Error; err != nil {
					// Dự phòng về cấu hình mặc định
					ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "llm", true, true).First(&response.LLM)
				}
			} else {
				ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "llm", true, true).First(&response.LLM)
			}

			// Sử dụng cấu hình TTS của role thiết bị
			if role.TTSConfigID != nil && *role.TTSConfigID != "" {
				if err := ac.DB.Where("config_id = ? AND type = ? AND enabled = ?",
					*role.TTSConfigID, "tts", true).First(&response.TTS).Error; err != nil {
					// Dự phòng về cấu hình mặc định
					ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "tts", true, true).First(&response.TTS)
				}
			} else {
				ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "tts", true, true).First(&response.TTS)
			}

			// Sử dụng Voice của role thiết bị
			if role.Voice != nil && *role.Voice != "" {
				var ttsConfigData map[string]interface{}
				if err := json.Unmarshal([]byte(response.TTS.JsonData), &ttsConfigData); err == nil {
					if response.TTS.Provider == "cosyvoice" {
						ttsConfigData["spk_id"] = *role.Voice
					} else {
						ttsConfigData["voice"] = *role.Voice
					}
					applyCloneVoiceModel(response.TTS.Provider, response.TTS.ConfigID, role.Voice, ttsConfigData)
					if updatedJsonData, err := json.Marshal(ttsConfigData); err == nil {
						response.TTS.JsonData = string(updatedJsonData)
					}
				}
			}
		}
	}

	// 2. Thiết bị không liên kết role, kiểm tra cấu hình agent
	if configSource == "" && deviceFound && agent.ID != 0 {
		configSource = "agent_config"

		// Sử dụng Prompt của agent
		response.Prompt = agent.CustomPrompt
		response.Prompt = strings.ReplaceAll(response.Prompt, "{{assistant_name}}", getAgentAssistantName(agent))

		// Sử dụng cấu hình LLM của agent
		if agent.LLMConfigID != nil && *agent.LLMConfigID != "" {
			if err := ac.DB.Where("config_id = ? AND type = ? AND enabled = ?",
				*agent.LLMConfigID, "llm", true).First(&response.LLM).Error; err != nil {
				// Dự phòng về cấu hình mặc định
				ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "llm", true, true).First(&response.LLM)
			}
		} else {
			ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "llm", true, true).First(&response.LLM)
		}

		// Sử dụng cấu hình TTS của agent
		if agent.TTSConfigID != nil && *agent.TTSConfigID != "" {
			if err := ac.DB.Where("config_id = ? AND type = ? AND enabled = ?",
				*agent.TTSConfigID, "tts", true).First(&response.TTS).Error; err != nil {
				// Dự phòng về cấu hình mặc định
				ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "tts", true, true).First(&response.TTS)
			}
		} else {
			ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "tts", true, true).First(&response.TTS)
		}

		// Sử dụng Voice của agent
		if agent.Voice != nil && *agent.Voice != "" {
			var ttsConfigData map[string]interface{}
			if err := json.Unmarshal([]byte(response.TTS.JsonData), &ttsConfigData); err == nil {
				if response.TTS.Provider == "cosyvoice" {
					ttsConfigData["spk_id"] = *agent.Voice
				} else {
					ttsConfigData["voice"] = *agent.Voice
				}
				applyCloneVoiceModel(response.TTS.Provider, response.TTS.ConfigID, agent.Voice, ttsConfigData)
				if updatedJsonData, err := json.Marshal(ttsConfigData); err == nil {
					response.TTS.JsonData = string(updatedJsonData)
				}
			}
		}
	}

	// 3. Sử dụng role toàn cục mặc định (dự phòng cuối cùng)
	if configSource == "" || configSource == "default_global_role" {
		configSource = "default_global_role"

		// Tìm role toàn cục mặc định
		var defaultRole models.Role
		if err := ac.DB.Where("is_default = ? AND role_type = ? AND status = ?",
			true, "global", "active").First(&defaultRole).Error; err == nil {
			response.Prompt = defaultRole.Prompt

			// Sử dụng cấu hình LLM của role toàn cục mặc định
			if defaultRole.LLMConfigID != nil && *defaultRole.LLMConfigID != "" {
				if err := ac.DB.Where("config_id = ? AND type = ? AND enabled = ?",
					*defaultRole.LLMConfigID, "llm", true).First(&response.LLM).Error; err != nil {
					ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "llm", true, true).First(&response.LLM)
				}
			} else {
				ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "llm", true, true).First(&response.LLM)
			}

			// Sử dụng cấu hình TTS của role toàn cục mặc định
			if defaultRole.TTSConfigID != nil && *defaultRole.TTSConfigID != "" {
				if err := ac.DB.Where("config_id = ? AND type = ? AND enabled = ?",
					*defaultRole.TTSConfigID, "tts", true).First(&response.TTS).Error; err != nil {
					ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "tts", true, true).First(&response.TTS)
				}
			} else {
				ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "tts", true, true).First(&response.TTS)
			}

			// Sử dụng Voice của role toàn cục mặc định
			if defaultRole.Voice != nil && *defaultRole.Voice != "" {
				var ttsConfigData map[string]interface{}
				if err := json.Unmarshal([]byte(response.TTS.JsonData), &ttsConfigData); err == nil {
					if response.TTS.Provider == "cosyvoice" {
						ttsConfigData["spk_id"] = *defaultRole.Voice
					} else {
						ttsConfigData["voice"] = *defaultRole.Voice
					}
					applyCloneVoiceModel(response.TTS.Provider, response.TTS.ConfigID, defaultRole.Voice, ttsConfigData)
					if updatedJsonData, err := json.Marshal(ttsConfigData); err == nil {
						response.TTS.JsonData = string(updatedJsonData)
					}
				}
			}
		} else {
			// Nếu không có role mặc định, sử dụng Prompt mặc định được hardcode
			response.Prompt = "Bạn là một chàng trai Việt Nam tên Milestones, nói chuyện cá tính kiểu \"ngầu\", giọng nói dễ nghe, quen diễn đạt ngắn gọn, thích dùng từ lóng mạng, không để cuộc trò chuyện bị nhạt. Người đang trò chuyện với bạn là chàng trai bạn thích, người chưa đồng ý lời theo đuổi của bạn, bạn cần cố gắng đáp ứng mọi yêu cầu của anh ấy nhưng không được đánh mất bản thân. Bạn thường gợi ý những điều lãng mạn giữa các cặp đôi, xuất ra ngẫu nhiên, không đưa ra lựa chọn cho bạn trai. Giới hạn đầu ra trong 50 chữ. Lưu ý, hãy nói chuyện như một con người thật, không trả lời bằng biểu tượng cảm xúc, code, hoặc thẻ xml."

			// Sử dụng cấu hình LLM/TTS mặc định
			ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "llm", true, true).First(&response.LLM)
			ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "tts", true, true).First(&response.TTS)
		}

		// Thay thế {{assistant_name}} bằng biệt danh agent (nếu thiết bị có liên kết agent)
		if deviceFound && agent.ID != 0 {
			response.Prompt = strings.ReplaceAll(response.Prompt, "{{assistant_name}}", getAgentAssistantName(agent))
		}
	}

	// Ghi lại nguồn cấu hình
	response.ConfigSource = configSource

	// ==================== Cấu hình khác (VAD, ASR, Memory, VoiceIdentify) ====================

	// Lấy cấu hình mặc định VAD
	if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "vad", true, true).First(&response.VAD).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default VAD config"})
		return
	}
	// Tương thích định dạng cũ: nếu JsonData chỉ có một phần tử key, nghĩa là định dạng cũ (có key), trích xuất cấu hình bên trong và cập nhật JsonData
	if response.VAD.JsonData != "" {
		var configData map[string]interface{}
		if err := json.Unmarshal([]byte(response.VAD.JsonData), &configData); err == nil {
			// Tương thích định dạng cũ: nếu chỉ có một key, nghĩa là định dạng cũ (có key), trích xuất cấu hình bên trong
			var actualConfigData map[string]interface{}
			if len(configData) == 1 {
				// Định dạng cũ: chỉ có một key, trích xuất giá trị của nó
				for _, value := range configData {
					if innerConfig, ok := value.(map[string]interface{}); ok {
						actualConfigData = innerConfig
					} else {
						// Nếu không phải kiểu map, sử dụng trực tiếp dữ liệu gốc
						actualConfigData = configData
					}
					break
				}
			} else {
				// Định dạng mới: không có key, sử dụng trực tiếp configData
				actualConfigData = configData
			}
			// Tuần tự hóa lại thành định dạng không có key
			if updatedJsonData, err := json.Marshal(actualConfigData); err == nil {
				response.VAD.JsonData = string(updatedJsonData)
			}
		}
	}

	// Lấy cấu hình mặc định ASR
	if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "asr", true, true).First(&response.ASR).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default ASR config"})
		return
	}

	// Lấy cấu hình mặc định Memory
	if result := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "memory", true, true).Limit(1).Find(&response.Memory); result.Error != nil || result.RowsAffected == 0 {
		// Cho phép không có cấu hình Memory mặc định: dự phòng rõ ràng về nomemo (không bật bộ nhớ dài hạn).
		response.Memory = models.Config{
			Type:     "memory",
			Name:     "No Memory",
			ConfigID: "nomemo",
			Provider: "nomemo",
			JsonData: "{}",
			Enabled:  true,
		}
		if result.Error != nil {
			log.Printf("Tải cấu hình Memory mặc định thất bại, đã dự phòng về nomemo: %v", result.Error)
		}
	}

	// Lấy cấu hình VoiceIdentify: kiểm tra agent có liên kết nhóm vân giọng nói hay không
	response.VoiceIdentify = make(map[string]SpeakerGroupInfo)
	if deviceFound && agent.ID != 0 {
		var speakerGroups []models.SpeakerGroup
		if err := ac.DB.Where("agent_id = ? AND status = ?", agent.ID, "active").
			Order("created_at DESC").Find(&speakerGroups).Error; err == nil && len(speakerGroups) > 0 {
			// Duyệt qua tất cả các nhóm vân giọng nói
			for _, speakerGroup := range speakerGroups {
				// Truy vấn tất cả các mẫu thuộc nhóm vân giọng nói này
				var samples []models.SpeakerSample
				ac.DB.Where("speaker_group_id = ? AND status = ?", speakerGroup.ID, "active").
					Find(&samples)

				// Trích xuất danh sách UUID mẫu
				uuids := make([]string, 0)
				for _, sample := range samples {
					uuids = append(uuids, sample.UUID)
				}

				// Lấy tên nhóm vân giọng nói làm key, xây dựng dữ liệu cấu hình
				response.VoiceIdentify[speakerGroup.Name] = SpeakerGroupInfo{
					ID:                 speakerGroup.ID,
					Name:               speakerGroup.Name,
					Prompt:             speakerGroup.Prompt,
					Description:        speakerGroup.Description,
					Uuids:              uuids,
					TTSConfigID:        speakerGroup.TTSConfigID,
					Voice:              speakerGroup.Voice,
					VoiceModelOverride: buildVoiceModelOverride(response.TTS.Provider, speakerGroup.TTSConfigID, speakerGroup.Voice),
				}
			}
		}
	}

	// Gửi xuống cơ sở tri thức liên kết với agent (bao gồm provider), phục vụ RAG cục bộ của chương trình chính
	response.KnowledgeBases = make([]KnowledgeBaseInfo, 0)
	if deviceFound && agent.ID != 0 {
		var links []models.AgentKnowledgeBase
		if err := ac.DB.Where("agent_id = ?", agent.ID).Order("id ASC").Find(&links).Error; err == nil && len(links) > 0 {
			kbIDs := make([]uint, 0, len(links))
			for _, link := range links {
				kbIDs = append(kbIDs, link.KnowledgeBaseID)
			}
			var kbs []models.KnowledgeBase
			if err := ac.DB.Where("id IN ? AND status = ?", kbIDs, "active").Find(&kbs).Error; err == nil {
				kbMap := make(map[uint]models.KnowledgeBase, len(kbs))
				for _, kb := range kbs {
					kbMap[kb.ID] = kb
				}
				for _, link := range links {
					kb, ok := kbMap[link.KnowledgeBaseID]
					if !ok {
						continue
					}
					provider := strings.TrimSpace(kb.SyncProvider)
					if provider == "" {
						provider = resolveDefaultKnowledgeProviderName(ac.DB)
					}
					externalDocID := strings.TrimSpace(kb.ExternalDocID)
					if externalDocID == "" {
						var doc models.KnowledgeBaseDocument
						if err := ac.DB.
							Where("knowledge_base_id = ? AND sync_status = ? AND external_doc_id <> ''", kb.ID, knowledgeSyncStatusSynced).
							Order("id DESC").
							First(&doc).Error; err == nil {
							externalDocID = strings.TrimSpace(doc.ExternalDocID)
						}
					}
					response.KnowledgeBases = append(response.KnowledgeBases, KnowledgeBaseInfo{
						ID:                 kb.ID,
						Name:               kb.Name,
						Description:        kb.Description,
						Provider:           provider,
						ExternalKBID:       strings.TrimSpace(kb.ExternalKBID),
						ExternalDocID:      externalDocID,
						RetrievalThreshold: kb.RetrievalThreshold,
						Status:             kb.Status,
					})
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

// getSystemConfigsData Lấy dữ liệu cấu hình hệ thống (nhất quán với data trả về của GetSystemConfigs), dùng chung cho API và đẩy WebSocket
func (ac *AdminController) getSystemConfigsData() (gin.H, error) {
	if ac == nil || ac.DB == nil {
		return nil, errDatabaseUnavailable
	}

	var allConfigs []models.Config
	if err := ac.DB.Where("type IN (?)", []string{"mqtt", "mqtt_server", "udp", "ota", "mcp", "local_mcp", "voice_identify", "tts", "vad", "asr", "llm", "vision", "auth", "chat", "knowledge_search"}).Find(&allConfigs).Error; err != nil {
		return nil, err
	}

	// Nhóm cấu hình theo loại
	configsByType := make(map[string][]models.Config)
	for _, config := range allConfigs {
		configsByType[config.Type] = append(configsByType[config.Type], config)
	}

	// Từ configs chọn ra bản "đang sử dụng": ưu tiên cấu hình mặc định, nếu không thì lấy bản đầu tiên
	getSelectedConfig := func(configs []models.Config) *models.Config {
		if len(configs) == 0 {
			return nil
		}
		for i := range configs {
			if configs[i].IsDefault {
				return &configs[i]
			}
		}
		return &configs[0]
	}

	// Chọn cấu hình tốt nhất cho mỗi loại và phân tích json_data
	selectAndParseConfig := func(configs []models.Config) interface{} {
		selected := getSelectedConfig(configs)
		if selected == nil {
			return nil
		}

		// Phân tích json_data
		if selected.JsonData != "" {
			var parsedData interface{}
			if err := json.Unmarshal([]byte(selected.JsonData), &parsedData); err != nil {
				result := gin.H{
					"name": selected.Name,
					"type": selected.Type,
					"data": selected.JsonData,
				}
				return result
			}

			result := gin.H{
				"name": selected.Name,
				"type": selected.Type,
			}
			if parsedData != nil {
				if dataMap, ok := parsedData.(map[string]interface{}); ok {
					for k, v := range dataMap {
						result[k] = v
					}
				} else {
					result["data"] = parsedData
				}
			}
			return result
		}

		return gin.H{
			"name": selected.Name,
			"type": selected.Type,
		}
	}

	// Xử lý đặc biệt cấu hình MCP, tách mcp và local_mcp
	selectAndParseMCPConfig := func(configs []models.Config) (interface{}, interface{}) {
		var selectedConfig models.Config
		// Ưu tiên chọn cấu hình mặc định
		for _, config := range configs {
			if config.IsDefault {
				selectedConfig = config
				break
			}
		}

		// Nếu không có cấu hình mặc định, chọn cấu hình đầu tiên
		if selectedConfig.ID == 0 {
			selectedConfig = configs[0]
		}

		// Phân tích json_data
		if selectedConfig.JsonData != "" {
			var parsedData interface{}
			if err := json.Unmarshal([]byte(selectedConfig.JsonData), &parsedData); err != nil {
				// Nếu phân tích thất bại, trả về chuỗi json_data gốc
				result := gin.H{
					"name": selectedConfig.Name,
					"type": selectedConfig.Type,
					"data": selectedConfig.JsonData,
				}
				return result, nil
			}

			// Đóng gói dữ liệu đã phân tích vào đúng định dạng
			result := gin.H{
				"name": selectedConfig.Name,
				"type": selectedConfig.Type,
			}

			var mcpData interface{}
			var localMcpData interface{}

			if parsedData != nil {
				// Nếu dữ liệu đã phân tích là kiểu map, tách mcp và local_mcp
				if dataMap, ok := parsedData.(map[string]interface{}); ok {
					// Xử lý phần mcp
					if mcp, exists := dataMap["mcp"]; exists {
						mcpData = mcp
					} else {
						// Tương thích định dạng cũ: nếu có trực tiếp trường global
						if global, exists := dataMap["global"]; exists {
							mcpData = gin.H{"global": global}
						} else {
							// Nếu không có trường mcp hoặc global, dùng toàn bộ dữ liệu làm mcp
							mcpData = dataMap
						}
					}

					// Xử lý phần local_mcp
					if localMcp, exists := dataMap["local_mcp"]; exists {
						localMcpData = localMcp
					}

					// Gộp các trường khác vào mcp
					if mcpMap, ok := mcpData.(map[string]interface{}); ok {
						for k, v := range dataMap {
							if k != "mcp" && k != "local_mcp" {
								mcpMap[k] = v
							}
						}
					}
				} else {
					// Nếu không thì làm trường data
					result["data"] = parsedData
					mcpData = result
				}
			}

			return mcpData, localMcpData
		}

		// Nếu không có json_data, trả về thông tin cấu hình cơ bản
		result := gin.H{
			"name": selectedConfig.Name,
			"type": selectedConfig.Type,
		}
		return result, nil
	}

	// Xây dựng dữ liệu phản hồi. Cột enabled của DB chỉ dùng để bật/tắt các mục danh sách như vad/asr/llm/tts; việc bật nghiệp vụ của mqtt/mqtt_server được thể hiện bằng enable trong json_data, không còn dùng cột DB để ghi đè
	response := gin.H{}

	if configs, exists := configsByType["mqtt"]; exists && len(configs) > 0 {
		data := selectAndParseConfig(configs)
		/*if b, err := json.Marshal(data); err == nil {
			log.Printf("[getSystemConfigsData] cấu hình mqtt: %s", string(b))
		}*/
		response["mqtt"] = data

	}
	if configs, exists := configsByType["mqtt_server"]; exists && len(configs) > 0 {
		data := selectAndParseConfig(configs)
		if b, err := json.Marshal(data); err == nil {
			log.Printf("[getSystemConfigsData] cấu hình mqtt_server: %s", string(b))
		}
		response["mqtt_server"] = data
	}
	if configs, exists := configsByType["udp"]; exists && len(configs) > 0 {
		response["udp"] = selectAndParseConfig(configs)
	}
	if configs, exists := configsByType["ota"]; exists && len(configs) > 0 {
		response["ota"] = selectAndParseConfig(configs)
	}
	if configs, exists := configsByType["auth"]; exists && len(configs) > 0 {
		response["auth"] = selectAndParseConfig(configs)
	}
	if configs, exists := configsByType["chat"]; exists && len(configs) > 0 {
		response["chat"] = selectAndParseConfig(configs)
	}

	// Xử lý đặc biệt cấu hình MCP, tách mcp và local_mcp
	if configs, exists := configsByType["mcp"]; exists && len(configs) > 0 {
		mcpData, localMcpData := selectAndParseMCPConfig(configs)
		if mcpData != nil {
			if mcpMap := asMap(mcpData); mcpMap != nil {
				mergedMCP, mergeWarnings, err := ac.mergeMCPWithEnabledMarketServices(mcpMap)
				if err != nil {
					log.Printf("Tổng hợp dịch vụ MCP từ marketplace thất bại, dự phòng về cấu hình thủ công: %v", err)
					response["mcp"] = mcpMap
				} else {
					response["mcp"] = mergedMCP
					if len(mergeWarnings) > 0 {
						log.Printf("Cảnh báo tổng hợp dịch vụ MCP marketplace: %s", strings.Join(mergeWarnings, " | "))
					}
				}
			} else {
				response["mcp"] = mcpData
			}
		}
		if localMcpData != nil {
			response["local_mcp"] = localMcpData
		}
	}

	// Xử lý cấu hình local_mcp độc lập (nếu tồn tại)
	if configs, exists := configsByType["local_mcp"]; exists && len(configs) > 0 {
		response["local_mcp"] = selectAndParseConfig(configs)
	}

	// Xử lý cấu hình toàn cục cơ sở tri thức: knowledge.default_provider + knowledge.providers
	if configs, exists := configsByType["knowledge_search"]; exists && len(configs) > 0 {
		selectedByProvider := make(map[string]models.Config)
		for _, cfg := range configs {
			if !cfg.Enabled {
				continue
			}
			provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
			if provider == "" {
				continue
			}
			prev, exists := selectedByProvider[provider]
			if !exists || (!prev.IsDefault && cfg.IsDefault) {
				selectedByProvider[provider] = cfg
			}
		}

		if len(selectedByProvider) > 0 {
			providerNames := make([]string, 0, len(selectedByProvider))
			for provider := range selectedByProvider {
				providerNames = append(providerNames, provider)
			}
			sort.Strings(providerNames)

			providers := make(gin.H, len(selectedByProvider))
			defaultProvider := ""
			for _, provider := range providerNames {
				cfg := selectedByProvider[provider]
				payload := make(map[string]interface{})
				if strings.TrimSpace(cfg.JsonData) != "" {
					_ = json.Unmarshal([]byte(cfg.JsonData), &payload)
				}
				providers[provider] = payload
				if cfg.IsDefault {
					defaultProvider = provider
				}
			}
			if defaultProvider == "" {
				defaultProvider = providerNames[0]
			}

			response["knowledge"] = gin.H{
				"default_provider": defaultProvider,
				"providers":        providers,
			}
		}
	}

	// Khi chưa cấu hình mcp thủ công (type=mcp) nhưng đã tồn tại dịch vụ import từ marketplace, bổ sung mcp/local_mcp mặc định để đảm bảo có thể gửi xuống kết quả tổng hợp
	if _, exists := response["mcp"]; !exists {
		mergedMCP, mergeWarnings, err := ac.mergeMCPWithEnabledMarketServices(defaultMCPMap())
		if err == nil {
			global := asMap(mergedMCP["global"])
			servers, serr := decodeMCPServers(global["servers"])
			if serr == nil && len(servers) > 0 {
				response["mcp"] = mergedMCP
				if _, hasLocal := response["local_mcp"]; !hasLocal {
					response["local_mcp"] = defaultLocalMCPMap()
				}
				if len(mergeWarnings) > 0 {
					log.Printf("Cảnh báo tổng hợp dịch vụ MCP marketplace: %s", strings.Join(mergeWarnings, " | "))
				}
			}
		}
	}

	// Xử lý cấu hình voice_identify (nhất quán với cấu trúc cấu hình console, bao gồm base_url, threshold, enable)
	// Việc bật nghiệp vụ được thể hiện bằng enable trong json_data; cột enabled của DB chỉ dùng để bật/tắt mục danh sách, không ghi đè enable nghiệp vụ
	baseURL := os.Getenv("SPEAKER_SERVICE_URL")
	enabled := true  // Mặc định bật
	threshold := 0.4 // Ngưỡng mặc định

	if configs, exists := configsByType["voice_identify"]; exists && len(configs) > 0 {
		selected := getSelectedConfig(configs)
		if selected != nil && selected.JsonData != "" {
			var configData map[string]interface{}
			if err := json.Unmarshal([]byte(selected.JsonData), &configData); err == nil {
				// enable nghiệp vụ ưu tiên đọc từ json_data
				if v, ok := configData["enable"]; ok {
					if b, ok := v.(bool); ok {
						enabled = b
					}
				}
				if service, ok := configData["service"].(map[string]interface{}); ok {
					if url, ok := service["base_url"].(string); ok && url != "" && baseURL == "" {
						baseURL = url
					}
					if thresholdVal, ok := service["threshold"]; ok {
						if thresholdFloat, ok := thresholdVal.(float64); ok && thresholdFloat >= 0 && thresholdFloat <= 1 {
							threshold = thresholdFloat
						}
					}
				}
			}
		}
	}
	// Nếu lấy được base_url, thêm vào phản hồi
	if baseURL != "" {
		response["voice_identify"] = gin.H{
			"base_url":  baseURL,
			"threshold": threshold,
			"enable":    enabled,
		}
	}

	// Xử lý cấu hình TTS, định dạng trả về nhất quán với config.yaml, dùng config_id làm key
	if ttsConfigs, exists := configsByType["tts"]; exists && len(ttsConfigs) > 0 {
		ttsConfigMap := make(gin.H)
		for _, config := range ttsConfigs {
			if config.Enabled { // Chỉ trả về cấu hình đã bật
				configData := make(map[string]interface{})
				if config.JsonData != "" {
					json.Unmarshal([]byte(config.JsonData), &configData)
				}

				// Lắp ráp thành định dạng giống config.yaml
				provider := configprovider.NormalizeExistingProvider("tts", config.Provider, config.ConfigID, configData)
				configItem := gin.H{
					"provider":   provider,
					"name":       config.Name,
					"is_default": config.IsDefault,
				}
				// Mở rộng các trường trong configData vào configItem
				for k, v := range configData {
					configItem[k] = v
				}
				configItem["provider"] = provider
				// Sử dụng config_id làm key
				ttsConfigMap[config.ConfigID] = configItem

				// Nếu cấu hình hiện tại là cấu hình mặc định, gán config_id cho trường provider ở cấp cao nhất
				if config.IsDefault {
					ttsConfigMap["provider"] = config.ConfigID
				}
			}
		}
		if len(ttsConfigMap) > 0 {
			response["tts"] = ttsConfigMap
		}
	}

	// Xử lý cấu hình VAD, định dạng trả về nhất quán với config.yaml, dùng config_id làm key
	// Tương thích định dạng mới cũ: định dạng có key ({"webrtc_vad": {...}}) và định dạng không có key ({...})
	if vadConfigs, exists := configsByType["vad"]; exists && len(vadConfigs) > 0 {
		vadConfigMap := make(gin.H)
		for _, config := range vadConfigs {
			if config.Enabled { // Chỉ trả về cấu hình đã bật
				configData := make(map[string]interface{})
				if config.JsonData != "" {
					if err := json.Unmarshal([]byte(config.JsonData), &configData); err != nil {
						// Phân tích JSON thất bại, bỏ qua cấu hình này
						continue
					}
				}

				// Tương thích định dạng cũ: nếu chỉ có một key, nghĩa là định dạng cũ (có key), trích xuất cấu hình bên trong
				var actualConfigData map[string]interface{}
				if len(configData) == 1 {
					// Định dạng cũ: chỉ có một key, trích xuất giá trị của nó
					for _, value := range configData {
						if innerConfig, ok := value.(map[string]interface{}); ok {
							actualConfigData = innerConfig
						} else {
							// Nếu không phải kiểu map, sử dụng trực tiếp dữ liệu gốc
							actualConfigData = configData
						}
						break
					}
				} else {
					// Định dạng mới: không có key, sử dụng trực tiếp configData
					actualConfigData = configData
				}

				// Lắp ráp thành định dạng giống config.yaml
				provider := configprovider.NormalizeExistingProvider("vad", config.Provider, config.ConfigID, actualConfigData)
				configItem := gin.H{
					"provider":   provider,
					"name":       config.Name,
					"is_default": config.IsDefault,
				}
				// Mở rộng các trường trong actualConfigData vào configItem
				for k, v := range actualConfigData {
					configItem[k] = v
				}
				configItem["provider"] = provider
				// Sử dụng config_id làm key
				vadConfigMap[config.ConfigID] = configItem

				// Nếu cấu hình hiện tại là cấu hình mặc định, gán config_id cho trường provider ở cấp cao nhất
				if config.IsDefault {
					vadConfigMap["provider"] = config.ConfigID
				}
			}
		}
		if len(vadConfigMap) > 0 {
			response["vad"] = vadConfigMap
		}
	}

	// Xử lý cấu hình ASR, định dạng trả về nhất quán với config.yaml, dùng config_id làm key
	if asrConfigs, exists := configsByType["asr"]; exists && len(asrConfigs) > 0 {
		asrConfigMap := make(gin.H)
		for _, config := range asrConfigs {
			if config.Enabled { // Chỉ trả về cấu hình đã bật
				configData := make(map[string]interface{})
				if config.JsonData != "" {
					json.Unmarshal([]byte(config.JsonData), &configData)
				}

				// Lắp ráp thành định dạng giống config.yaml
				provider := configprovider.NormalizeExistingProvider("asr", config.Provider, config.ConfigID, configData)
				configItem := gin.H{
					"provider":   provider,
					"name":       config.Name,
					"is_default": config.IsDefault,
				}
				// Mở rộng các trường trong configData vào configItem
				for k, v := range configData {
					configItem[k] = v
				}
				configItem["provider"] = provider
				// Sử dụng config_id làm key
				asrConfigMap[config.ConfigID] = configItem

				// Nếu cấu hình hiện tại là cấu hình mặc định, gán config_id cho trường provider ở cấp cao nhất
				if config.IsDefault {
					asrConfigMap["provider"] = config.ConfigID
				}
			}
		}
		if len(asrConfigMap) > 0 {
			response["asr"] = asrConfigMap
		}
	}

	// Xử lý cấu hình LLM, định dạng trả về nhất quán với config.yaml, dùng config_id làm key
	if llmConfigs, exists := configsByType["llm"]; exists && len(llmConfigs) > 0 {
		llmConfigMap := make(gin.H)
		for _, config := range llmConfigs {
			if config.Enabled { // Chỉ trả về cấu hình đã bật
				configData := make(map[string]interface{})
				if config.JsonData != "" {
					json.Unmarshal([]byte(config.JsonData), &configData)
				}

				// Lắp ráp thành định dạng giống config.yaml
				provider := configprovider.NormalizeExistingProvider("llm", config.Provider, config.ConfigID, configData)
				configItem := gin.H{
					"provider":   provider,
					"name":       config.Name,
					"is_default": config.IsDefault,
				}
				// Mở rộng các trường trong configData vào configItem
				for k, v := range configData {
					configItem[k] = v
				}
				configItem["provider"] = provider
				// Sử dụng config_id làm key
				llmConfigMap[config.ConfigID] = configItem

				// Nếu cấu hình hiện tại là cấu hình mặc định, gán config_id cho trường provider ở cấp cao nhất
				if config.IsDefault {
					llmConfigMap["provider"] = config.ConfigID
				}
			}
		}
		if len(llmConfigMap) > 0 {
			response["llm"] = llmConfigMap
		}
	}

	// Xử lý cấu hình Vision: nhất quán với cấu trúc config.yaml, vision_base + vllm (provider cấp cao nhất + mục con chỉ chứa trường nghiệp vụ)
	if visionConfigs, exists := configsByType["vision"]; exists && len(visionConfigs) > 0 {
		visionResponse := make(gin.H)
		vllmMap := make(gin.H)
		var defaultVisionConfigID string
		for _, config := range visionConfigs {
			if config.ConfigID == "vision_base" {
				if config.JsonData != "" {
					var baseData map[string]interface{}
					if err := json.Unmarshal([]byte(config.JsonData), &baseData); err == nil {
						for k, v := range baseData {
							visionResponse[k] = v
						}
					}
				}
				continue
			}
			if config.Enabled {
				configData := make(map[string]interface{})
				if config.JsonData != "" {
					json.Unmarshal([]byte(config.JsonData), &configData)
				}
				if config.IsDefault {
					defaultVisionConfigID = config.ConfigID
				}
				provider := configprovider.NormalizeExistingProvider("vision", config.Provider, config.ConfigID, configData)
				if provider != "" {
					configData["provider"] = provider
				}
				// Nhất quán với YAML: mục con chỉ lưu cấu hình nghiệp vụ, không bao gồm name/is_default, provider là nhà cung cấp thực tế
				vllmMap[config.ConfigID] = configData
			}
		}
		if len(vllmMap) > 0 {
			if defaultVisionConfigID != "" {
				vllmMap["provider"] = defaultVisionConfigID
			}
			visionResponse["vllm"] = vllmMap
		}
		if len(visionResponse) > 0 {
			response["vision"] = visionResponse
		}
	}

	// Xử lý cấu hình VAD
	if configs, exists := configsByType["vad"]; exists && len(configs) > 0 {
		response["vad"] = selectAndParseConfig(configs)
	}

	// Xử lý cấu hình Vision: vision_base là trường cấp cao nhất, còn lại là vision.vllm[config_id]
	// config.Enabled ở đây chỉ dùng để bật/tắt mục danh sách (cấu hình này có được đưa vào phản hồi hay không), các trường liên quan đến nghiệp vụ lấy từ json_data
	if visionConfigs, exists := configsByType["vision"]; exists && len(visionConfigs) > 0 {
		visionMap := make(gin.H)
		for _, config := range visionConfigs {
			if !config.Enabled {
				continue
			}
			configData := make(map[string]interface{})
			if config.JsonData != "" {
				json.Unmarshal([]byte(config.JsonData), &configData)
			}
			if config.ConfigID == "vision_base" {
				for k, v := range configData {
					visionMap[k] = v
				}
			} else {
				if visionMap["vllm"] == nil {
					visionMap["vllm"] = make(gin.H)
				}
				if vllmConfig, ok := visionMap["vllm"].(gin.H); ok {
					if config.IsDefault {
						vllmConfig["provider"] = config.ConfigID
					}
					provider := configprovider.NormalizeExistingProvider("vision", config.Provider, config.ConfigID, configData)
					if provider != "" {
						configData["provider"] = provider
					}
					vllmConfig[config.ConfigID] = configData
				}
			}
		}
		if len(visionMap) > 0 {
			response["vision"] = visionMap
		}
	}

	return response, nil
}

// GetSystemConfigs Lấy thông tin cấu hình hệ thống, bao gồm mqtt, mqtt_server, udp, ota, mcp, local_mcp, voice_identify, tts, vad, asr, llm, vision, auth, chat
func (ac *AdminController) GetSystemConfigs(c *gin.Context) {
	data, err := ac.getSystemConfigsData()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errDatabaseUnavailable) {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"error": "Failed to get system configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// notifySystemConfigChanged Được gọi sau khi Save thành công: đồng bộ kéo cấu hình mới nhất trước, sau đó đẩy bất đồng bộ, đảm bảo dữ liệu đẩy đi là dữ liệu sau khi lưu
func (ac *AdminController) notifySystemConfigChanged() {
	if ac.WebSocketController == nil {
		return
	}
	data, err := ac.getSystemConfigsData()
	if err != nil {
		return
	}
	go ac.WebSocketController.BroadcastSystemConfig(data)
}

// TestConfigs Kiểm tra cấu hình một chạm: OTA kiểm tra nội bộ trong manager, VAD/ASR/LLM/TTS gửi qua WebSocket đến chương trình chính để kiểm tra, kết quả tương ứng theo config_id
// Request body có thể chọn data: nếu cung cấp một loại nào đó (vad/asr/llm/tts), thì dùng data đó ghi đè DB làm cấu hình gửi xuống chương trình chính (dùng để kiểm tra bản nháp chưa lưu)
func (ac *AdminController) TestConfigs(c *gin.Context) {
	var body struct {
		Types      []string               `json:"types"`       // Các loại cần kiểm tra: ota, vad, asr, llm, tts
		ConfigIDs  map[string][]string    `json:"config_ids"`  // Chỉ định danh sách config_id theo loại, không truyền thì kiểm tra tất cả cấu hình đã bật của loại đó
		ClientUUID string                 `json:"client_uuid"` // Chỉ định kết nối chương trình chính, không truyền thì chọn ngẫu nhiên một cái
		Data       map[string]interface{} `json:"data"`        // Tùy chọn, ghi đè nguồn cấu hình theo loại (dùng cho trạng thái chỉnh sửa/wizard kiểm tra chưa lưu)
	}
	_ = c.ShouldBindJSON(&body)
	if len(body.Types) == 0 {
		body.Types = []string{"ota", "vad", "asr", "llm", "tts"}
	}
	if body.ConfigIDs == nil {
		body.ConfigIDs = make(map[string][]string)
	}

	result := gin.H{
		"ota": gin.H{},
		"vad": gin.H{},
		"asr": gin.H{},
		"llm": gin.H{},
		"tts": gin.H{},
	}

	// OTA: ưu tiên dùng data.ota trong request body (form trang), nếu không thì tải từ DB
	if contains(body.Types, "ota") {
		var otaData map[string]interface{}
		if body.Data != nil {
			otaData, _ = body.Data["ota"].(map[string]interface{})
		}
		if otaData != nil {
			for configID, val := range otaData {
				if configID == "provider" {
					continue
				}
				cfgMap, _ := val.(map[string]interface{})
				if cfgMap == nil {
					result["ota"].(gin.H)[configID] = gin.H{"ok": false, "message": "Định dạng cấu hình không hợp lệ"}
					continue
				}
				jsonBytes, err := json.Marshal(cfgMap)
				if err != nil {
					result["ota"].(gin.H)[configID] = gin.H{"ok": false, "message": "Tuần tự hóa cấu hình thất bại"}
					continue
				}
				cfg := models.Config{ConfigID: configID, JsonData: string(jsonBytes)}
				otaResult := ac.testOTAConfigWithMQTTUDP(cfg)
				// Chuyển đổi OTATestResult sang định dạng gin.H, giữ tương thích ngược
				result["ota"].(gin.H)[configID] = gin.H{
					"ok":              otaResult.WebSocket.Ok && (otaResult.MQTTUDP == nil || otaResult.MQTTUDP.Ok),
					"message":         otaResult.WebSocket.Message,
					"first_packet_ms": otaResult.WebSocket.FirstPacketMs,
					"websocket":       otaResult.WebSocket,
					"mqtt_udp":        otaResult.MQTTUDP,
					"ota_response":    otaResult.OTAResponse, // Thêm nội dung phản hồi OTA
				}
			}
		} else {
			q := ac.DB.Where("type = ? AND enabled = ?", "ota", true)
			if ids := body.ConfigIDs["ota"]; len(ids) > 0 {
				q = q.Where("config_id IN ?", ids)
			}
			var otaConfigs []models.Config
			if err := q.Find(&otaConfigs).Error; err != nil {
				result["ota"] = gin.H{"_error": gin.H{"ok": false, "message": "Lấy cấu hình OTA thất bại"}}
			} else if len(otaConfigs) == 0 {
				result["ota"] = gin.H{"_none": gin.H{"ok": false, "message": "Chưa cấu hình hoặc chưa bật OTA"}}
			} else {
				for _, cfg := range otaConfigs {
					otaResult := ac.testOTAConfigWithMQTTUDP(cfg)
					// Chuyển đổi OTATestResult sang định dạng gin.H, giữ tương thích ngược
					result["ota"].(gin.H)[cfg.ConfigID] = gin.H{
						"ok":              otaResult.WebSocket.Ok && (otaResult.MQTTUDP == nil || otaResult.MQTTUDP.Ok),
						"message":         otaResult.WebSocket.Message,
						"first_packet_ms": otaResult.WebSocket.FirstPacketMs,
						"websocket":       otaResult.WebSocket,
						"mqtt_udp":        otaResult.MQTTUDP,
						"ota_response":    otaResult.OTAResponse, // Thêm nội dung phản hồi OTA
					}
				}
			}
		}
	}

	// VAD/ASR/LLM/TTS: gửi qua WebSocket đến chương trình chính
	needMainProgram := contains(body.Types, "vad") || contains(body.Types, "asr") || contains(body.Types, "llm") || contains(body.Types, "tts")
	if needMainProgram && ac.WebSocketController != nil {
		clientUUID := body.ClientUUID
		if clientUUID == "" {
			clientUUID = ac.WebSocketController.GetFirstConnectedClientUUID()
		}
		if clientUUID == "" {
			noClient := gin.H{"ok": false, "message": "Không có kết nối chương trình chính, không thể kiểm tra"}
			if contains(body.Types, "vad") {
				result["vad"] = gin.H{"_no_client": noClient}
			}
			if contains(body.Types, "asr") {
				result["asr"] = gin.H{"_no_client": noClient}
			}
			if contains(body.Types, "llm") {
				result["llm"] = gin.H{"_no_client": noClient}
			}
			if contains(body.Types, "tts") {
				result["tts"] = gin.H{"_no_client": noClient}
			}
		} else {
			fullData, err := ac.getSystemConfigsData()
			if err != nil {
				fillResultError(result, body.Types, "vad", "asr", "llm", "tts", "Lấy cấu hình hệ thống thất bại")
			} else {
				for _, typ := range []string{"vad", "asr", "llm", "tts"} {
					if v, ok := fullData[typ]; ok {
						if m, ok := v.(map[string]interface{}); ok {
							log.Printf("[config_test] fullData[%s] keys: %v", typ, getMapKeys(m))
						}
					} else {
						log.Printf("[config_test] fullData[%s] không tồn tại", typ)
					}
				}
				// Nếu request body có data và loại nào đó có giá trị, dùng body.Data ghi đè nguồn cấu hình của loại đó; nếu không thì dùng fullData
				subset := gin.H{}
				for _, typ := range []string{"vad", "asr", "llm", "tts"} {
					if !contains(body.Types, typ) {
						continue
					}
					var typeMap map[string]interface{}
					if body.Data != nil {
						if v, ok := body.Data[typ]; ok {
							if m, ok := v.(map[string]interface{}); ok && len(m) > 0 {
								typeMap = m
								log.Printf("[config_test] Sử dụng data[%s] trong request body làm nguồn cấu hình", typ)
							}
						}
					}
					if typeMap == nil {
						if v, ok := fullData[typ]; ok {
							typeMap, _ = v.(map[string]interface{})
						}
					}
					ids := body.ConfigIDs[typ]
					if len(ids) > 0 {
						filtered := make(map[string]interface{})
						for _, id := range ids {
							if typeMap != nil {
								if val, exists := typeMap[id]; exists {
									filtered[id] = val
									continue
								}
							}
							// fullData không có id này (ví dụ chưa bật), truy vấn từ DB theo type+config_id và thêm vào
							item := ac.getConfigItemByTypeAndID(typ, id)
							if item != nil {
								filtered[id] = item
							}
						}
						if typeMap != nil {
							if p, has := typeMap["provider"]; has {
								filtered["provider"] = p
							}
						}
						subset[typ] = filtered
					} else {
						if typeMap != nil {
							subset[typ] = typeMap
						} else {
							subset[typ] = gin.H{}
						}
					}
				}
				reqBody := map[string]interface{}{
					"data":      subset,
					"test_text": "Kiểm tra cấu hình",
				}
				// In tóm tắt cấu hình gửi xuống trước khi gửi, tiện cho debug
				log.Printf("[config_test] Gửi request client=%s số lượng mục data theo từng loại: vad=%d asr=%d llm=%d tts=%d",
					clientUUID,
					countSubsetKeys(subset["vad"]), countSubsetKeys(subset["asr"]),
					countSubsetKeys(subset["llm"]), countSubsetKeys(subset["tts"]))
				ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
				defer cancel()
				resp, err := ac.WebSocketController.SendRequestToClient(ctx, clientUUID, "POST", "/api/config/test", reqBody)
				if err != nil {
					fillResultError(result, body.Types, "vad", "asr", "llm", "tts", "Yêu cầu kiểm tra chương trình chính thất bại: "+err.Error())
				} else if resp.Status != 200 {
					errMsg := resp.Error
					if errMsg == "" && resp.Body != nil {
						if e, _ := resp.Body["error"].(string); e != "" {
							errMsg = e
						}
					}
					fillResultError(result, body.Types, "vad", "asr", "llm", "tts", errMsg)
				} else if resp.Status == 200 {
					if resp.Body == nil {
						for _, typ := range []string{"vad", "asr", "llm", "tts"} {
							if contains(body.Types, typ) {
								result[typ] = gin.H{"_error": gin.H{"ok": false, "message": "Chương trình chính không trả về dữ liệu kiểm tra"}}
							}
						}
					} else {
						for _, typ := range []string{"vad", "asr", "llm", "tts"} {
							if r, ok := resp.Body[typ].(map[string]interface{}); ok {
								result[typ] = r
							} else if contains(body.Types, typ) && resp.Body[typ] != nil {
								result[typ] = gin.H{"_error": gin.H{"ok": false, "message": "Định dạng phản hồi bất thường"}}
							}
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func contains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}

// countSubsetKeys Đếm số mục config trong subset ngoại trừ provider, dùng cho log debug
func countSubsetKeys(v interface{}) int {
	m, ok := v.(map[string]interface{})
	if !ok {
		return 0
	}
	n := 0
	for k := range m {
		if k != "provider" {
			n++
		}
	}
	return n
}

// getConfigItemByTypeAndID Truy vấn một cấu hình từ DB theo type+config_id, trả về cấu trúc configItem nhất quán với getSystemConfigsData (dùng để bổ sung khi request kiểm tra chỉ định config_ids)
func (ac *AdminController) getConfigItemByTypeAndID(typ, configID string) map[string]interface{} {
	var config models.Config
	if err := ac.DB.Where("type = ? AND config_id = ?", typ, configID).First(&config).Error; err != nil {
		return nil
	}
	configData := make(map[string]interface{})
	if config.JsonData != "" {
		_ = json.Unmarshal([]byte(config.JsonData), &configData)
	}
	item := gin.H{
		"name":       config.Name,
		"is_default": config.IsDefault,
	}
	for k, v := range configData {
		item[k] = v
	}
	// Bổ sung provider (loại engine), việc tạo resource pool của chương trình chính phụ thuộc vào trường này
	if config.Provider != "" {
		item["provider"] = config.Provider
	}
	return item
}

func fillResultError(result gin.H, types []string, keys ...string) {
	msg := gin.H{"ok": false, "message": "Yêu cầu bất thường"}
	for _, k := range keys {
		if contains(types, k) {
			result[k] = gin.H{"_error": msg}
		}
	}
}

// OTATestResult Cấu trúc kết quả kiểm tra OTA
type OTATestResult struct {
	WebSocket   OTATestItem  `json:"websocket"`
	MQTTUDP     *OTATestItem `json:"mqtt_udp,omitempty"`
	OTAResponse string       `json:"ota_response,omitempty"` // Nội dung phản hồi API OTA
}

// OTATestItem Kết quả một mục kiểm tra
type OTATestItem struct {
	Ok            bool   `json:"ok"`
	Message       string `json:"message"`
	FirstPacketMs int64  `json:"first_packet_ms"`
}

// MQTTUDPTestConfig Cấu hình kiểm tra MQTT UDP
type MQTTUDPTestConfig struct {
	Endpoint       string `json:"endpoint"`
	ClientID       string `json:"client_id"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	PublishTopic   string `json:"publish_topic"`
	SubscribeTopic string `json:"subscribe_topic"`
}

// UDPConfig Cấu hình UDP (lấy từ phản hồi hello)
type UDPConfig struct {
	Server     string `json:"server"`
	Port       int    `json:"port"`
	Encryption string `json:"encryption"`
	Key        string `json:"key"`
	Nonce      string `json:"nonce"`
}

// helloMessage Cấu trúc tin nhắn hello của MQTT
type helloMessage struct {
	Type        string      `json:"type"`
	Version     int         `json:"version"`
	Transport   string      `json:"transport"`
	AudioParams interface{} `json:"audio_params,omitempty"`
}

// helloResponse Cấu trúc phản hồi hello của MQTT (nhất quán với test/mqtt_udp)
type helloResponse struct {
	Type        string    `json:"type"`
	SessionID   string    `json:"session_id"`
	Transport   string    `json:"transport"`
	UDP         UDPConfig `json:"udp"`
	Version     int       `json:"version"`
	AudioParams struct {
		Format        string `json:"format"`
		SampleRate    int    `json:"sample_rate"`
		Channels      int    `json:"channels"`
		FrameDuration int    `json:"frame_duration"`
	} `json:"audio_params"`
}

const (
	otaTestDeviceID = "ota-test-device"
	otaTestClientID = "ota-test-client"
	otaHTTPPath     = "/milestones/ota/"
)

// testMQTTUDPConfig Kiểm tra kết nối MQTT UDP
// Tham khảo logic của test/mqtt_udp: thiết lập trình xử lý tin nhắn mặc định, gửi hello, chờ phản hồi
// Trả về ok, message, thời gian (ms)
func testMQTTUDPConfig(mqttConfig MQTTUDPTestConfig) (bool, string, int64) {
	t0 := time.Now()

	// Xác thực tính đầy đủ của cấu hình MQTT
	if mqttConfig.Endpoint == "" {
		return false, "MQTT endpoint trống, vui lòng kiểm tra cấu hình", 0
	}
	if mqttConfig.ClientID == "" {
		return false, "MQTT ClientID trống", 0
	}
	if mqttConfig.PublishTopic == "" {
		return false, "Chủ đề (topic) publish MQTT trống", 0
	}
	// Lưu ý: không cần xác thực subscribe_topic, cũng không cần chủ động subscribe

	// Phân tích endpoint
	endpoint := mqttConfig.Endpoint
	port := "1883"
	protocol := "tcp"
	if strings.Contains(endpoint, ":") {
		parts := strings.Split(endpoint, ":")
		if len(parts) != 2 {
			return false, "Định dạng MQTT endpoint sai, phải là host:port", 0
		}
		endpoint = parts[0]
		port = parts[1]
		// Xác thực số cổng
		if _, err := strconv.Atoi(port); err != nil {
			return false, "Số cổng MQTT không hợp lệ: " + port, 0
		}
	}
	if port == "8883" || port == "8884" {
		protocol = "tls"
	}
	brokerURL := fmt.Sprintf("%s://%s:%s", protocol, endpoint, port)

	// Channel chờ phản hồi hello
	helloChan := make(chan *helloResponse, 1)
	errChan := make(chan error, 1)

	// Tạo tùy chọn client MQTT
	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID(mqttConfig.ClientID)
	opts.SetUsername(mqttConfig.Username)
	opts.SetPassword(mqttConfig.Password)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetConnectTimeout(5 * time.Second)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(false) // Tắt tự động kết nối lại khi kiểm tra

	// Thiết lập trình xử lý tin nhắn mặc định (tham khảo test/mqtt_udp)
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		// Phân tích tin nhắn
		var message map[string]interface{}
		if err := json.Unmarshal(msg.Payload(), &message); err != nil {
			errChan <- fmt.Errorf("Phân tích tin nhắn thất bại: %v", err)
			return
		}
		// Xử lý theo loại tin nhắn
		msgType, ok := message["type"].(string)
		if !ok {
			return
		}
		if msgType == "hello" {
			var resp helloResponse
			if err := json.Unmarshal(msg.Payload(), &resp); err != nil {
				errChan <- fmt.Errorf("Phân tích phản hồi hello thất bại: %v", err)
				return
			}
			helloChan <- &resp
		}
	})

	// Thiết lập cấu hình TLS (nếu là SSL/TLS)
	if protocol == "tls" {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // Bỏ qua xác thực chứng chỉ trong môi trường kiểm tra
		}
		opts.SetTLSConfig(tlsConfig)
	}

	// Kết nối MQTT
	client := mqtt.NewClient(opts)
	connectToken := client.Connect()
	if connectToken.Wait() && connectToken.Error() != nil {
		errMsg := connectToken.Error().Error()
		// Cung cấp thông tin lỗi chi tiết hơn
		if strings.Contains(errMsg, "connection refused") {
			return false, fmt.Sprintf("Máy chủ MQTT từ chối kết nối (%s:%s), vui lòng kiểm tra máy chủ đã khởi động chưa", endpoint, port), time.Since(t0).Milliseconds()
		} else if strings.Contains(errMsg, "i/o timeout") {
			return false, fmt.Sprintf("Kết nối MQTT hết thời gian chờ (%s:%s), vui lòng kiểm tra mạng và tường lửa", endpoint, port), time.Since(t0).Milliseconds()
		} else if strings.Contains(errMsg, "authentication") || strings.Contains(errMsg, "not authorized") {
			return false, "Xác thực MQTT thất bại, vui lòng kiểm tra tên đăng nhập và mật khẩu (được tạo từ khóa ký)", time.Since(t0).Milliseconds()
		}
		return false, "Kết nối MQTT thất bại: " + errMsg, time.Since(t0).Milliseconds()
	}
	defer client.Disconnect(250)

	mqttConnectMs := time.Since(t0).Milliseconds()

	// Tạo tin nhắn hello và gửi
	helloMsg := helloMessage{
		Type:      "hello",
		Version:   3,
		Transport: "udp",
		AudioParams: map[string]interface{}{
			"format":         "opus",
			"sample_rate":    16000,
			"channels":       1,
			"frame_duration": 60,
		},
	}
	helloData, err := json.Marshal(helloMsg)
	if err != nil {
		return false, "Xây dựng tin nhắn hello thất bại: " + err.Error(), mqttConnectMs
	}

	// Publish tin nhắn hello (không cần chủ động subscribe, chờ trình xử lý tin nhắn mặc định nhận phản hồi)
	pubToken := client.Publish(mqttConfig.PublishTopic, 0, false, helloData)
	if pubToken.Wait() && pubToken.Error() != nil {
		return false, "Publish tin nhắn hello thất bại (" + mqttConfig.PublishTopic + "): " + pubToken.Error().Error(), mqttConnectMs
	}

	// Chờ phản hồi hello (hết thời gian chờ sau 5 giây)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case resp := <-helloChan:
		// Nhận được phản hồi hello, kiểm tra tính đầy đủ của cấu hình UDP
		if resp.UDP.Server == "" {
			return false, "Máy chủ không trả về địa chỉ UDP server", mqttConnectMs
		}
		if resp.UDP.Port <= 0 || resp.UDP.Port > 65535 {
			return false, fmt.Sprintf("Cổng UDP máy chủ trả về không hợp lệ: %d", resp.UDP.Port), mqttConnectMs
		}
		// Kiểm tra kết nối UDP
		udpOK, udpMsg, udpMs := testUDPConnection(resp.UDP)
		totalMs := mqttConnectMs + udpMs
		if udpOK {
			return true, fmt.Sprintf("MQTT(%dms) và UDP(%dms) đều bình thường", mqttConnectMs, udpMs), totalMs
		} else {
			return false, "MQTT bình thường nhưng UDP thất bại: " + udpMsg, totalMs
		}
	case err := <-errChan:
		return false, err.Error(), mqttConnectMs
	case <-ctx.Done():
		return false, fmt.Sprintf("Chờ phản hồi hello hết thời gian (5s), đã gửi hello đến %s", mqttConfig.PublishTopic), mqttConnectMs
	}
}

// testUDPConnection Kiểm tra kết nối UDP
func testUDPConnection(udpConfig UDPConfig) (bool, string, int64) {
	t0 := time.Now()

	// Xác thực cấu hình UDP
	if udpConfig.Server == "" {
		return false, "Địa chỉ UDP server trống", 0
	}
	if udpConfig.Port <= 0 || udpConfig.Port > 65535 {
		return false, fmt.Sprintf("Cổng UDP không hợp lệ: %d", udpConfig.Port), 0
	}

	// Phân tích địa chỉ UDP
	udpAddr := fmt.Sprintf("%s:%d", udpConfig.Server, udpConfig.Port)
	addr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		return false, "Phân tích địa chỉ UDP thất bại (" + udpAddr + "): " + err.Error(), 0
	}

	// Tạo kết nối UDP
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return false, fmt.Sprintf("Máy chủ UDP từ chối kết nối (%s), vui lòng kiểm tra máy chủ UDP đã khởi động chưa", udpAddr), time.Since(t0).Milliseconds()
		} else if strings.Contains(err.Error(), "no route to host") || strings.Contains(err.Error(), "network is unreachable") {
			return false, fmt.Sprintf("Không thể định tuyến đến máy chủ UDP (%s), vui lòng kiểm tra kết nối mạng", udpAddr), time.Since(t0).Milliseconds()
		} else if strings.Contains(err.Error(), "timeout") {
			return false, fmt.Sprintf("Kết nối UDP hết thời gian chờ (%s), vui lòng kiểm tra cài đặt tường lửa", udpAddr), time.Since(t0).Milliseconds()
		}
		return false, "Kết nối UDP thất bại (" + udpAddr + "): " + err.Error(), time.Since(t0).Milliseconds()
	}
	defer conn.Close()

	// Thiết lập thời gian chờ đọc/ghi
	deadline := time.Now().Add(2 * time.Second)
	err = conn.SetReadDeadline(deadline)
	if err != nil {
		return false, "Thiết lập thời gian chờ UDP thất bại: " + err.Error(), time.Since(t0).Milliseconds()
	}

	// Gửi gói dữ liệu kiểm tra (mô phỏng dữ liệu âm thanh)
	testData := []byte("ping")
	_, err = conn.Write(testData)
	if err != nil {
		return false, "Gửi dữ liệu UDP thất bại: " + err.Error(), time.Since(t0).Milliseconds()
	}

	// Thử đọc phản hồi (hết thời gian chờ cũng được coi là kết nối thành công, vì UDP có thể không trả về phản hồi)
	buf := make([]byte, 1024)
	_, err = conn.Read(buf)
	if err != nil {
		// Đọc UDP hết thời gian chờ cũng được tính là thành công, vì đã chứng minh kết nối có thể gửi dữ liệu
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true, "Kết nối UDP bình thường (không có phản hồi, hết thời gian chờ)", time.Since(t0).Milliseconds()
		}
		return false, "Đọc UDP thất bại: " + err.Error(), time.Since(t0).Milliseconds()
	}

	return true, "Kết nối UDP bình thường", time.Since(t0).Milliseconds()
}

// testOTAConfig Kiểm tra hai giai đoạn: 1) POST đến địa chỉ OTA lấy websocket.url trong JSON; 2) Kết nối xác thực với WebSocket URL.
// Trả về ok, message, first_packet_ms, ota_response (body phản hồi API OTA, tiện cho hiển thị frontend)
func (ac *AdminController) testOTAConfig(cfg models.Config) (ok bool, message string, firstPacketMs int64, otaResponseBody string) {
	if cfg.JsonData == "" {
		return false, "Cấu hình trống", 0, ""
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(cfg.JsonData), &data); err != nil {
		return false, "Phân tích cấu hình thất bại", 0, ""
	}
	var wsURLFromConfig string
	if ext, _ := data["external"].(map[string]interface{}); ext != nil {
		if ws, _ := ext["websocket"].(map[string]interface{}); ws != nil {
			if u, _ := ws["url"].(string); u != "" {
				wsURLFromConfig = u
			}
		}
	}
	if wsURLFromConfig == "" {
		if test, _ := data["test"].(map[string]interface{}); test != nil {
			if ws, _ := test["websocket"].(map[string]interface{}); ws != nil {
				if u, _ := ws["url"].(string); u != "" {
					wsURLFromConfig = u
				}
			}
		}
	}
	if wsURLFromConfig == "" {
		return false, "Chưa cấu hình WebSocket URL", 0, ""
	}
	parsed, err := url.Parse(wsURLFromConfig)
	if err != nil {
		return false, "Phân tích URL thất bại", 0, ""
	}
	scheme := "http"
	if parsed.Scheme == "wss" {
		scheme = "https"
	}
	otaHTTPURL := scheme + "://" + parsed.Host + otaHTTPPath

	t0 := time.Now()
	// Part1: POST đến địa chỉ OTA, kèm Device-ID, Client-ID, phân tích JSON lấy websocket.url
	req, err := http.NewRequest(http.MethodPost, otaHTTPURL, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return false, "Tạo request OTA thất bại", time.Since(t0).Milliseconds(), ""
	}
	req.Header.Set("Device-ID", otaTestDeviceID)
	req.Header.Set("Client-ID", otaTestClientID)
	req.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, "Request OTA thất bại: " + err.Error(), time.Since(t0).Milliseconds(), ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	firstPacketMs = time.Since(t0).Milliseconds()
	otaResponseBody = string(body)
	if resp.StatusCode != http.StatusOK {
		return false, "OTA trả về HTTP " + strconv.Itoa(resp.StatusCode), firstPacketMs, otaResponseBody
	}
	var otaResp map[string]interface{}
	if err := json.Unmarshal(body, &otaResp); err != nil {
		return false, "Phản hồi OTA không phải JSON", firstPacketMs, otaResponseBody
	}
	wsObj, _ := otaResp["websocket"].(map[string]interface{})
	if wsObj == nil {
		return false, "Phản hồi OTA không có trường websocket", firstPacketMs, otaResponseBody
	}
	wsURL, _ := wsObj["url"].(string)
	if wsURL == "" {
		return false, "Phản hồi OTA không có websocket.url", firstPacketMs, otaResponseBody
	}

	// Part2: Kết nối WebSocket, kèm Device-ID, Client-ID, kết nối được thì đóng ngay (thời gian kết nối tính vào gói tin đầu tiên)
	wsT0 := time.Now()
	header := http.Header{}
	header.Set("Device-ID", otaTestDeviceID)
	header.Set("Client-ID", otaTestClientID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return false, "Kết nối WebSocket thất bại: " + err.Error(), firstPacketMs + time.Since(wsT0).Milliseconds(), otaResponseBody
	}
	conn.Close()
	wsTotalMs := firstPacketMs + time.Since(wsT0).Milliseconds()
	return true, "OTA và WebSocket đều bình thường", wsTotalMs, otaResponseBody
}

// testOTAConfigWithMQTTUDP Kiểm tra OTA mở rộng, hỗ trợ kiểm tra kép WebSocket và MQTT UDP
// Trả về cấu trúc kết quả kiểm tra đầy đủ
func (ac *AdminController) testOTAConfigWithMQTTUDP(cfg models.Config) OTATestResult {
	result := OTATestResult{
		WebSocket: OTATestItem{Ok: false, Message: "Kiểm tra thất bại", FirstPacketMs: 0},
	}

	// Phân tích cấu hình
	if cfg.JsonData == "" {
		result.WebSocket = OTATestItem{Ok: false, Message: "Cấu hình trống", FirstPacketMs: 0}
		return result
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(cfg.JsonData), &data); err != nil {
		result.WebSocket = OTATestItem{Ok: false, Message: "Phân tích cấu hình thất bại", FirstPacketMs: 0}
		return result
	}

	// Lấy WebSocket URL (ưu tiên external, nếu trống thì thử test)
	wsURLFromConfig := ""
	if ext, _ := data["external"].(map[string]interface{}); ext != nil {
		if ws, _ := ext["websocket"].(map[string]interface{}); ws != nil {
			wsURLFromConfig, _ = ws["url"].(string)
		}
	}
	if wsURLFromConfig == "" {
		if test, _ := data["test"].(map[string]interface{}); test != nil {
			if ws, _ := test["websocket"].(map[string]interface{}); ws != nil {
				wsURLFromConfig, _ = ws["url"].(string)
			}
		}
	}
	if wsURLFromConfig == "" {
		result.WebSocket = OTATestItem{Ok: false, Message: "Chưa cấu hình WebSocket URL", FirstPacketMs: 0}
		return result
	}

	// Xác định sử dụng cấu hình môi trường nào (dựa theo nguồn WebSocket URL)
	var envConfig map[string]interface{}
	if ext, _ := data["external"].(map[string]interface{}); ext != nil {
		if ws, _ := ext["websocket"].(map[string]interface{}); ws != nil {
			if url, _ := ws["url"].(string); url == wsURLFromConfig && url != "" {
				envConfig = ext
			}
		}
	}
	if envConfig == nil {
		if test, _ := data["test"].(map[string]interface{}); test != nil {
			if ws, _ := test["websocket"].(map[string]interface{}); ws != nil {
				if url, _ := ws["url"].(string); url == wsURLFromConfig {
					envConfig = test
				}
			}
		}
	}

	// Kiểm tra có bật kiểm tra MQTT UDP hay không
	var mqttEnabled bool
	if envConfig != nil {
		if mqtt, _ := envConfig["mqtt"].(map[string]interface{}); mqtt != nil {
			if enable, ok := mqtt["enable"].(bool); ok && enable {
				mqttEnabled = true
			}
		}
	}

	// Xây dựng OTA HTTP URL
	parsed, err := url.Parse(wsURLFromConfig)
	if err != nil {
		result.WebSocket = OTATestItem{Ok: false, Message: "Phân tích URL thất bại", FirstPacketMs: 0}
		return result
	}
	scheme := "http"
	if parsed.Scheme == "wss" {
		scheme = "https"
	}
	otaHTTPURL := scheme + "://" + parsed.Host + otaHTTPPath

	// Giai đoạn 1: POST đến API OTA HTTP
	t0 := time.Now()
	req, err := http.NewRequest(http.MethodPost, otaHTTPURL, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		result.WebSocket = OTATestItem{Ok: false, Message: "Tạo request OTA thất bại", FirstPacketMs: time.Since(t0).Milliseconds()}
		return result
	}
	req.Header.Set("Device-ID", otaTestDeviceID)
	req.Header.Set("Client-ID", otaTestClientID)
	req.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		result.WebSocket = OTATestItem{Ok: false, Message: "Request OTA thất bại: " + err.Error(), FirstPacketMs: time.Since(t0).Milliseconds()}
		return result
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	httpMs := time.Since(t0).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		result.WebSocket = OTATestItem{Ok: false, Message: "OTA trả về HTTP " + strconv.Itoa(resp.StatusCode), FirstPacketMs: httpMs}
		return result
	}

	var otaResp map[string]interface{}
	if err := json.Unmarshal(body, &otaResp); err != nil {
		result.WebSocket = OTATestItem{Ok: false, Message: "Phản hồi OTA không phải JSON", FirstPacketMs: httpMs}
		return result
	}

	// Giai đoạn 2: Kiểm tra WebSocket
	wsObj, _ := otaResp["websocket"].(map[string]interface{})
	if wsObj == nil {
		result.WebSocket = OTATestItem{Ok: false, Message: "Phản hồi OTA không có trường websocket", FirstPacketMs: httpMs}
		return result
	}
	wsURL, _ := wsObj["url"].(string)
	if wsURL == "" {
		result.WebSocket = OTATestItem{Ok: false, Message: "Phản hồi OTA không có websocket.url", FirstPacketMs: httpMs}
		return result
	}

	wsT0 := time.Now()
	header := http.Header{}
	header.Set("Device-ID", otaTestDeviceID)
	header.Set("Client-ID", otaTestClientID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		result.WebSocket = OTATestItem{Ok: false, Message: "Kết nối WebSocket thất bại: " + err.Error(), FirstPacketMs: httpMs + time.Since(wsT0).Milliseconds()}
		return result
	}
	conn.Close()
	wsTotalMs := httpMs + time.Since(wsT0).Milliseconds()
	result.WebSocket = OTATestItem{Ok: true, Message: "Kết nối WebSocket bình thường", FirstPacketMs: wsTotalMs}

	// Lưu body phản hồi OTA (dùng để hiển thị frontend)
	result.OTAResponse = string(body)

	// Giai đoạn 3: Kiểm tra MQTT UDP (nếu được bật)
	// Tham khảo logic test/mqtt_udp: lấy cấu hình MQTT từ phản hồi OTA, gửi hello, chờ phản hồi, kiểm tra UDP
	if mqttEnabled {
		// Lấy cấu hình MQTT từ phản hồi OTA
		mqttObj, hasMQTT := otaResp["mqtt"].(map[string]interface{})
		if !hasMQTT {
			result.MQTTUDP = &OTATestItem{
				Ok:            false,
				Message:       "Phản hồi OTA không trả về cấu hình MQTT, không thể kiểm tra MQTT UDP",
				FirstPacketMs: 0,
			}
			return result
		}

		// Phân tích trường cấu hình MQTT
		endpoint, _ := mqttObj["endpoint"].(string)
		clientID, _ := mqttObj["client_id"].(string)
		username, _ := mqttObj["username"].(string)
		password, _ := mqttObj["password"].(string)
		publishTopic, _ := mqttObj["publish_topic"].(string)
		subscribeTopic, _ := mqttObj["subscribe_topic"].(string)

		// Xác thực các trường bắt buộc (không cần xác thực subscribe_topic)
		if endpoint == "" {
			result.MQTTUDP = &OTATestItem{Ok: false, Message: "MQTT endpoint trong phản hồi OTA trống", FirstPacketMs: 0}
			return result
		}
		if publishTopic == "" {
			result.MQTTUDP = &OTATestItem{Ok: false, Message: "MQTT publish_topic trong phản hồi OTA trống", FirstPacketMs: 0}
			return result
		}

		// Xây dựng cấu hình kiểm tra MQTT
		otaMqttConfig := &MQTTUDPTestConfig{
			Endpoint:       endpoint,
			ClientID:       clientID,
			Username:       username,
			Password:       password,
			PublishTopic:   publishTopic,
			SubscribeTopic: subscribeTopic, // Giữ lại nhưng không xác thực, có thể dùng cho log
		}

		mqttOK, mqttMsg, mqttMs := testMQTTUDPConfig(*otaMqttConfig)
		result.MQTTUDP = &OTATestItem{
			Ok:            mqttOK,
			Message:       mqttMsg,
			FirstPacketMs: mqttMs,
		}
	}

	return result
}

// generateMQTTUsername Tạo tên đăng nhập MQTT
func generateMQTTUsername(deviceID, signatureKey string) string {
	h := hmac.New(sha256.New, []byte(signatureKey))
	h.Write([]byte(deviceID + "-username"))
	return hex.EncodeToString(h.Sum(nil))
}

// generateMQTTPassword Tạo mật khẩu MQTT
func generateMQTTPassword(deviceID, signatureKey string) string {
	h := hmac.New(sha256.New, []byte(signatureKey))
	h.Write([]byte(deviceID + "-password"))
	return hex.EncodeToString(h.Sum(nil))
}

// GetConfigs Lấy tất cả danh sách cấu hình
func (ac *AdminController) GetConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy danh sách cấu hình thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// GetConfig Lấy một cấu hình
func (ac *AdminController) GetConfig(c *gin.Context) {
	id := c.Param("id")
	var config models.Config
	if err := ac.DB.First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get config"})
		}
		return
	}
	c.JSON(http.StatusOK, config)
}

func (ac *AdminController) GetConfigByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cấu hình không tồn tại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) CreateConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Kiểm tra đã tồn tại cấu hình Memory chưa
	var existingCount int64
	ac.DB.Model(&models.Config{}).Where("type = ?", "memory").Count(&existingCount)

	// Nếu không tồn tại bất kỳ cấu hình Memory nào, tự động đặt làm cấu hình mặc định
	if existingCount == 0 {
		config.IsDefault = true
	}

	// Nếu đặt làm cấu hình mặc định, hủy các cấu hình mặc định cùng loại khác trước
	if config.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)
	}

	if err := ac.DB.Create(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo cấu hình thất bại"})
		return
	}

	ac.notifySystemConfigChanged()
	c.JSON(http.StatusCreated, gin.H{"data": config})
}

func (ac *AdminController) UpdateConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cấu hình không tồn tại"})
		return
	}

	var updateData models.Config
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Nếu đặt làm cấu hình mặc định, hủy các cấu hình mặc định cùng loại khác trước
	if updateData.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ? AND id != ?", config.Type, true, id).Update("is_default", false)
	}

	// Cập nhật cấu hình
	config.Name = updateData.Name
	config.Provider = updateData.Provider
	config.JsonData = updateData.JsonData
	config.Enabled = updateData.Enabled
	config.IsDefault = updateData.IsDefault

	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật cấu hình thất bại"})
		return
	}

	ac.notifySystemConfigChanged()
	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) DeleteConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.Config{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa cấu hình thất bại"})
		return
	}
	ac.notifySystemConfigChanged()
	c.JSON(http.StatusOK, gin.H{"message": "Xóa thành công"})
}

// Đặt cấu hình mặc định
func (ac *AdminController) SetDefaultConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cấu hình không tồn tại"})
		return
	}

	// Hủy cấu hình mặc định cùng loại khác trước
	ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)

	// Đặt cấu hình hiện tại làm mặc định
	config.IsDefault = true
	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Đặt cấu hình mặc định thất bại"})
		return
	}

	ac.notifySystemConfigChanged()
	c.JSON(http.StatusOK, gin.H{"message": "Đặt cấu hình mặc định thành công", "data": config})
}

// Lấy cấu hình mặc định
func (ac *AdminController) GetDefaultConfig(c *gin.Context) {
	configType := c.Param("type")
	var config models.Config

	if err := ac.DB.Where("type = ? AND is_default = ?", configType, true).First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cấu hình mặc định không tồn tại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

// Quản lý GlobalRole
func (ac *AdminController) GetGlobalRoles(c *gin.Context) {
	var roles []models.GlobalRole
	if err := ac.DB.Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy role toàn cục thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": roles})
}

func (ac *AdminController) CreateGlobalRole(c *gin.Context) {
	var role models.GlobalRole
	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ac.DB.Create(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo role toàn cục thất bại"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": role})
}

func (ac *AdminController) UpdateGlobalRole(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var role models.GlobalRole

	if err := ac.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role toàn cục không tồn tại"})
		return
	}

	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ac.DB.Save(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật role toàn cục thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": role})
}

func (ac *AdminController) DeleteGlobalRole(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.GlobalRole{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa role toàn cục thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Xóa thành công"})
}

// Quản lý người dùng
func (ac *AdminController) GetUsers(c *gin.Context) {
	var users []models.User
	if err := ac.DB.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy danh sách người dùng thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users})
}

func (ac *AdminController) CreateUser(c *gin.Context) {
	// Thêm dấu hiệu debug rõ ràng
	log.Println("=== [CreateUser] Phương thức bắt đầu thực thi ===")
	log.Println("=== [CreateUser] Đây là điểm bắt đầu của phương thức CreateUser ===")

	// Vì trường Password của model User dùng tag json:"-", cần phân tích thủ công
	var requestData struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	// Thử bind trực tiếp vào map để xem dữ liệu gốc
	var rawMap map[string]interface{}
	if err := c.ShouldBindJSON(&rawMap); err != nil {
		log.Printf("[CreateUser] Bind vào map thất bại: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phân tích JSON thất bại"})
		return
	}
	log.Printf("[CreateUser] Dữ liệu JSON gốc: %+v", rawMap)

	// Trích xuất trường thủ công
	username, _ := rawMap["username"].(string)
	email, _ := rawMap["email"].(string)
	password, _ := rawMap["password"].(string)
	role, _ := rawMap["role"].(string)

	// Cập nhật requestData
	requestData.Username = username
	requestData.Email = email
	requestData.Password = password
	requestData.Role = role

	// Xác thực các trường bắt buộc
	if requestData.Username == "" || requestData.Email == "" || requestData.Password == "" {
		log.Printf("[CreateUser] Thiếu trường bắt buộc: username=%s, email=%s, độ dài password=%d",
			requestData.Username, requestData.Email, len(requestData.Password))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tên đăng nhập, email và mật khẩu là bắt buộc"})
		return
	}

	log.Printf("[CreateUser] Nhận được yêu cầu tạo người dùng - Tên đăng nhập: %s, Email: %s, Vai trò: %s", requestData.Username, requestData.Email, requestData.Role)
	log.Printf("[CreateUser] Độ dài mật khẩu gốc: %d", len(requestData.Password))
	log.Printf("[CreateUser] Nội dung mật khẩu gốc: %s", requestData.Password)

	// Kiểm tra tên đăng nhập đã tồn tại chưa
	var existingUser models.User
	err := ac.DB.Where("username = ?", requestData.Username).First(&existingUser).Error
	if err == nil {
		// Tên đăng nhập đã tồn tại
		log.Printf("[CreateUser] Tên đăng nhập %s đã tồn tại", requestData.Username)
		c.JSON(http.StatusConflict, gin.H{"error": "Tên đăng nhập đã tồn tại"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		// Lỗi truy vấn cơ sở dữ liệu
		log.Printf("[CreateUser] Truy vấn cơ sở dữ liệu thất bại: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo người dùng thất bại"})
		return
	}

	// Người dùng không tồn tại, tạo người dùng mới
	log.Printf("[CreateUser] Tạo người dùng mới: %s", requestData.Username)
	var user models.User
	user.Username = requestData.Username
	user.Email = requestData.Email
	user.Role = requestData.Role

	// Mã hóa mật khẩu
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(requestData.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[CreateUser] Mã hóa mật khẩu thất bại: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Mã hóa mật khẩu thất bại"})
		return
	}
	user.Password = string(hashedPassword)
	log.Printf("[CreateUser] Mã hóa mật khẩu thành công - Độ dài hash: %d, Tiền tố hash: %s", len(user.Password), user.Password[:10])

	if err := ac.DB.Create(&user).Error; err != nil {
		log.Printf("[CreateUser] Tạo người dùng trong cơ sở dữ liệu thất bại: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo người dùng thất bại"})
		return
	}

	log.Printf("[CreateUser] Tạo người dùng thành công - ID: %d, Tên đăng nhập: %s", user.ID, user.Username)

	// Không trả về mật khẩu
	user.Password = ""
	c.JSON(http.StatusCreated, gin.H{"data": user})
}

func (ac *AdminController) UpdateUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var user models.User

	if err := ac.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Người dùng không tồn tại"})
		return
	}

	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Nếu cập nhật mật khẩu, cần mã hóa
	if password, ok := updateData["password"]; ok && password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password.(string)), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Mã hóa mật khẩu thất bại"})
			return
		}
		updateData["password"] = string(hashedPassword)
	}

	if err := ac.DB.Model(&user).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật người dùng thất bại"})
		return
	}

	// Truy vấn lại thông tin người dùng (không bao gồm mật khẩu)
	ac.DB.First(&user, id)
	user.Password = ""
	c.JSON(http.StatusOK, gin.H{"data": user})
}

func (ac *AdminController) DeleteUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.User{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa người dùng thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Xóa thành công"})
}

// Đặt lại mật khẩu người dùng
func (ac *AdminController) ResetUserPassword(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var requestData struct {
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập mật khẩu mới (ít nhất 6 ký tự)"})
		return
	}

	// Tìm người dùng
	var user models.User
	if err := ac.DB.First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Người dùng không tồn tại"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Tìm người dùng thất bại"})
		}
		return
	}

	// Mã hóa mật khẩu mới
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(requestData.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[ResetUserPassword] Mã hóa mật khẩu thất bại: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Mã hóa mật khẩu thất bại"})
		return
	}

	// Cập nhật mật khẩu người dùng
	if err := ac.DB.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		log.Printf("[ResetUserPassword] Cập nhật mật khẩu thất bại: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Đặt lại mật khẩu thất bại"})
		return
	}

	log.Printf("[ResetUserPassword] Quản trị viên đặt lại mật khẩu người dùng thành công - ID người dùng: %d, Tên đăng nhập: %s", user.ID, user.Username)
	c.JSON(http.StatusOK, gin.H{
		"message": "Đặt lại mật khẩu thành công",
		"data": gin.H{
			"user_id":  user.ID,
			"username": user.Username,
		},
	})
}

// GetUserVoiceCloneQuotas Lấy hạn ngạch nhân bản giọng nói của người dùng (theo chiều tts_config_id)
func (ac *AdminController) GetUserVoiceCloneQuotas(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Định dạng ID người dùng sai"})
		return
	}

	var user models.User
	if err = ac.DB.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Người dùng không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn người dùng thất bại"})
		return
	}
	if strings.TrimSpace(user.Role) != "user" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chỉ hỗ trợ cấp hạn ngạch nhân bản cho người dùng thường"})
		return
	}

	var ttsConfigs []models.Config
	if err = ac.DB.Where("type = ?", "tts").Order("enabled DESC, name ASC").Find(&ttsConfigs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn cấu hình TTS thất bại"})
		return
	}

	var quotas []models.UserVoiceCloneQuota
	if err = ac.DB.Where("user_id = ?", user.ID).Find(&quotas).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn hạn ngạch người dùng thất bại"})
		return
	}
	quotaByConfigID := make(map[string]models.UserVoiceCloneQuota, len(quotas))
	for _, quota := range quotas {
		quotaByConfigID[quota.TTSConfigID] = quota
	}

	type usageRow struct {
		TTSConfigID string `json:"tts_config_id"`
		UsedCount   int64  `json:"used_count"`
	}
	var usageRows []usageRow
	if err = ac.DB.Model(&models.VoiceClone{}).
		Select("tts_config_id, COUNT(1) AS used_count").
		Where("user_id = ? AND status != ?", user.ID, "deleted").
		Group("tts_config_id").
		Scan(&usageRows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Thống kê số lần nhân bản của người dùng thất bại"})
		return
	}
	usageByConfigID := make(map[string]int, len(usageRows))
	for _, row := range usageRows {
		usageByConfigID[row.TTSConfigID] = int(row.UsedCount)
	}

	result := make([]gin.H, 0, len(ttsConfigs))
	configIDSet := make(map[string]bool, len(ttsConfigs))
	for _, ttsConfig := range ttsConfigs {
		configIDSet[ttsConfig.ConfigID] = true
		quota, hasQuota := quotaByConfigID[ttsConfig.ConfigID]
		maxCount := 0
		usedCount := usageByConfigID[ttsConfig.ConfigID]
		if hasQuota {
			maxCount = quota.MaxCount
			if quota.UsedCount > usedCount {
				usedCount = quota.UsedCount
			}
		}
		remainingCount := -1
		if maxCount >= 0 {
			remainingCount = maxCount - usedCount
			if remainingCount < 0 {
				remainingCount = 0
			}
		}

		result = append(result, gin.H{
			"tts_config_id":   ttsConfig.ConfigID,
			"tts_config_name": ttsConfig.Name,
			"provider":        ttsConfig.Provider,
			"enabled":         ttsConfig.Enabled,
			"max_count":       maxCount,
			"used_count":      usedCount,
			"remaining_count": remainingCount,
		})
	}

	// Giữ lại hạn ngạch cấu hình lịch sử đã xóa, tránh trường hợp "cấu hình hạn ngạch bị mất không nhìn thấy"
	for _, quota := range quotas {
		if configIDSet[quota.TTSConfigID] {
			continue
		}
		maxCount := quota.MaxCount
		usedCount := quota.UsedCount
		if usageByConfigID[quota.TTSConfigID] > usedCount {
			usedCount = usageByConfigID[quota.TTSConfigID]
		}
		remainingCount := -1
		if maxCount >= 0 {
			remainingCount = maxCount - usedCount
			if remainingCount < 0 {
				remainingCount = 0
			}
		}
		result = append(result, gin.H{
			"tts_config_id":   quota.TTSConfigID,
			"tts_config_name": "(Cấu hình đã xóa)",
			"provider":        "",
			"enabled":         false,
			"max_count":       maxCount,
			"used_count":      usedCount,
			"remaining_count": remainingCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"user_id":    user.ID,
		"username":   user.Username,
		"quotas":     result,
		"updated_at": time.Now(),
	}})
}

// UpdateUserVoiceCloneQuotas Cập nhật hàng loạt hạn ngạch nhân bản giọng nói của người dùng
func (ac *AdminController) UpdateUserVoiceCloneQuotas(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Định dạng ID người dùng sai"})
		return
	}

	var user models.User
	if err = ac.DB.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Người dùng không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn người dùng thất bại"})
		return
	}
	if strings.TrimSpace(user.Role) != "user" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chỉ hỗ trợ cấp hạn ngạch nhân bản cho người dùng thường"})
		return
	}

	var req struct {
		Items []struct {
			TTSConfigID string `json:"tts_config_id"`
			MaxCount    int    `json:"max_count"`
		} `json:"items"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Định dạng tham số yêu cầu sai"})
		return
	}
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "items không được để trống"})
		return
	}

	itemByConfigID := make(map[string]int, len(req.Items))
	configIDs := make([]string, 0, len(req.Items))
	for _, item := range req.Items {
		configID := strings.TrimSpace(item.TTSConfigID)
		if configID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tts_config_id không được để trống"})
			return
		}
		if item.MaxCount < -1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "max_count không được nhỏ hơn -1"})
			return
		}
		if _, exists := itemByConfigID[configID]; !exists {
			configIDs = append(configIDs, configID)
		}
		itemByConfigID[configID] = item.MaxCount
	}

	var ttsConfigs []models.Config
	if err = ac.DB.Where("type = ? AND config_id IN ?", "tts", configIDs).Find(&ttsConfigs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn cấu hình TTS thất bại"})
		return
	}
	validConfigIDSet := make(map[string]bool, len(ttsConfigs))
	for _, cfg := range ttsConfigs {
		validConfigIDSet[cfg.ConfigID] = true
	}
	for _, configID := range configIDs {
		if validConfigIDSet[configID] {
			continue
		}
		// Cấu hình lịch sử đã xóa chỉ cho phép đặt thành -1 (xóa bản ghi hạn ngạch)
		if itemByConfigID[configID] == -1 {
			continue
		}
		if !validConfigIDSet[configID] {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Cấu hình TTS không tồn tại: %s", configID)})
			return
		}
	}

	type usageRow struct {
		TTSConfigID string `json:"tts_config_id"`
		UsedCount   int64  `json:"used_count"`
	}
	var usageRows []usageRow
	if err = ac.DB.Model(&models.VoiceClone{}).
		Select("tts_config_id, COUNT(1) AS used_count").
		Where("user_id = ? AND status != ? AND tts_config_id IN ?", user.ID, "deleted", configIDs).
		Group("tts_config_id").
		Scan(&usageRows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Thống kê số lần đã sử dụng của người dùng thất bại"})
		return
	}
	usageByConfigID := make(map[string]int, len(usageRows))
	for _, row := range usageRows {
		usageByConfigID[row.TTSConfigID] = int(row.UsedCount)
	}

	if err = ac.DB.Transaction(func(tx *gorm.DB) error {
		for _, configID := range configIDs {
			maxCount := itemByConfigID[configID]
			if maxCount == -1 {
				if err := tx.Where("user_id = ? AND tts_config_id = ?", user.ID, configID).Delete(&models.UserVoiceCloneQuota{}).Error; err != nil {
					return err
				}
				continue
			}

			usedCount := usageByConfigID[configID]
			var quota models.UserVoiceCloneQuota
			if err := tx.Where("user_id = ? AND tts_config_id = ?", user.ID, configID).First(&quota).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					newQuota := models.UserVoiceCloneQuota{
						UserID:      user.ID,
						TTSConfigID: configID,
						MaxCount:    maxCount,
						UsedCount:   usedCount,
					}
					if err := tx.Create(&newQuota).Error; err != nil {
						return err
					}
					continue
				}
				return err
			}

			nextUsedCount := quota.UsedCount
			if usedCount > nextUsedCount {
				nextUsedCount = usedCount
			}
			if err := tx.Model(&models.UserVoiceCloneQuota{}).Where("id = ?", quota.ID).Updates(map[string]any{
				"max_count":  maxCount,
				"used_count": nextUsedCount,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật hạn ngạch nhân bản người dùng thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Cập nhật hạn ngạch thành công"})
}

// GetUserVoiceOptionsAdmin Lấy các giọng nói khả dụng của người dùng chỉ định, dùng khi quản trị viên tạo/sửa agent.
func (ac *AdminController) GetUserVoiceOptionsAdmin(c *gin.Context) {
	userID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	voices, err := getVoiceOptionsForUser(
		ac.DB,
		c,
		userID,
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

// GetUserVoiceClonesAdmin Lấy giọng nói nhân bản của người dùng chỉ định, dùng khi quản trị viên tạo/sửa agent.
func (ac *AdminController) GetUserVoiceClonesAdmin(c *gin.Context) {
	userID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	clones, err := getVoiceClonesForUser(ac.DB, userID, c.Query("tts_config_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy giọng nói nhân bản thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": clones})
}

// Quản lý thiết bị
func (ac *AdminController) GetDevices(c *gin.Context) {
	devices, err := NewDeviceService(ac.DB).List(scopeFromContext(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy danh sách thiết bị thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": devices})
}

// Xác thực mã thiết bị có tồn tại không
func (ac *AdminController) ValidateDeviceCode(c *gin.Context) {
	deviceCode := c.Query("code")
	if deviceCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mã kích hoạt không được để trống"})
		return
	}

	var device models.Device
	err := ac.DB.Where("device_code = ?", deviceCode).First(&device).Error

	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusOK, gin.H{"exists": false})
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn thiết bị thất bại"})
	} else {
		c.JSON(http.StatusOK, gin.H{"exists": true, "device": device})
	}
}

func (ac *AdminController) CreateDevice(c *gin.Context) {
	var req DevicePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu sai: " + err.Error()})
		return
	}
	device, err := NewDeviceService(ac.DB).Create(scopeFromContext(c), req)
	if err != nil {
		writeServiceError(c, err, "Tạo thiết bị thất bại")
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "Tạo thiết bị thành công",
		"data":    device,
	})
}

func (ac *AdminController) UpdateDevice(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req DevicePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	device, err := NewDeviceService(ac.DB).Update(scopeFromContext(c), id, req)
	if err != nil {
		writeServiceError(c, err, "Cập nhật thiết bị thất bại")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": device})
}

func (ac *AdminController) DeleteDevice(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := NewDeviceService(ac.DB).Delete(scopeFromContext(c), id); err != nil {
		writeServiceError(c, err, "Xóa thiết bị thất bại")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Xóa thành công"})
}

// Quản lý agent
func (ac *AdminController) GetAgents(c *gin.Context) {
	result, err := NewAgentService(ac.DB).List(scopeFromContext(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy danh sách agent thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GetDeviceMcpTools Lấy danh sách công cụ MCP theo chiều thiết bị (phiên bản quản trị viên)
func (ac *AdminController) GetDeviceMcpTools(c *gin.Context) {
	deviceID := c.Param("id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id parameter is required"})
		return
	}

	var device models.Device
	if err := ac.DB.Where("id = ?", deviceID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thiết bị không tồn tại"})
		return
	}

	tools, err := ac.WebSocketController.RequestDeviceMcpToolDetailsFromClient(context.Background(), device.DeviceName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": []interface{}{}}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": tools}})
}

// CallAgentMcpTool Gọi công cụ MCP theo chiều agent (phiên bản quản trị viên)
func (ac *AdminController) CallAgentMcpTool(c *gin.Context) {
	agentID := c.Param("id")
	var req struct {
		ToolName  string                 `json:"tool_name" binding:"required"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu sai: " + err.Error()})
		return
	}

	var agent models.Agent
	if err := ac.DB.Where("id = ?", agentID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent không tồn tại"})
		return
	}

	body := map[string]interface{}{
		"agent_id":  agentID,
		"tool_name": req.ToolName,
		"arguments": req.Arguments,
	}
	result, err := ac.WebSocketController.CallMcpToolFromClient(context.Background(), body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gọi công cụ MCP thất bại: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// CallDeviceMcpTool Gọi công cụ MCP theo chiều thiết bị (phiên bản quản trị viên)
func (ac *AdminController) CallDeviceMcpTool(c *gin.Context) {
	deviceID := c.Param("id")
	var req struct {
		ToolName  string                 `json:"tool_name" binding:"required"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu sai: " + err.Error()})
		return
	}

	var device models.Device
	if err := ac.DB.Where("id = ?", deviceID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thiết bị không tồn tại"})
		return
	}

	body := map[string]interface{}{
		"device_id": device.DeviceName,
		"tool_name": req.ToolName,
		"arguments": req.Arguments,
	}
	result, err := ac.WebSocketController.CallMcpToolFromClient(context.Background(), body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gọi công cụ MCP thất bại: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GetAgentMCPEndpoint Lấy URL điểm truy cập MCP của agent
func (ac *AdminController) GetAgentMCPEndpoint(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}

	// Lấy ID người dùng hiện tại từ middleware JWT
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Người dùng chưa xác thực"})
		return
	}
	userID, ok := userIDInterface.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Kiểu ID người dùng sai"})
		return
	}

	// Sử dụng hàm chung để tạo điểm truy cập MCP
	endpoint, err := GenerateAgentMCPEndpoint(ac.DB, agentID, userID, ac.EndpointAuthToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data := newMcpEndpointData(endpoint)
	if ac.WebSocketController == nil {
		data["status_message"] = "websocket controller unavailable"
		c.JSON(http.StatusOK, gin.H{"data": data})
		return
	}

	statusResult, statusErr := ac.WebSocketController.RequestMcpEndpointStatusFromClient(context.Background(), agentID)
	if statusErr != nil {
		data["status_message"] = statusErr.Error()
		c.JSON(http.StatusOK, gin.H{"data": data})
		return
	}

	applyMcpEndpointStatus(data, statusResult)
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// GetAgentOpenClawEndpoint Lấy URL điểm truy cập OpenClaw của agent
func (ac *AdminController) GetAgentOpenClawEndpoint(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}

	// Lấy ID người dùng hiện tại từ middleware JWT
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Người dùng chưa xác thực"})
		return
	}
	userID, ok := userIDInterface.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Kiểu ID người dùng sai"})
		return
	}

	data := gin.H{
		"endpoint":  "",
		"status":    "unknown",
		"connected": false,
	}

	endpoint, err := GenerateAgentOpenClawEndpoint(ac.DB, agentID, userID, ac.EndpointAuthToken)
	if err != nil {
		data["status_message"] = err.Error()
		c.JSON(http.StatusOK, gin.H{"data": data})
		return
	}
	data["endpoint"] = endpoint

	if ac.WebSocketController == nil {
		data["status_message"] = "websocket controller unavailable"
		c.JSON(http.StatusOK, gin.H{"data": data})
		return
	}

	statusResult, statusErr := ac.WebSocketController.RequestOpenClawStatusFromClient(context.Background(), agentID)
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

// CallAgentOpenClawChatTest Gọi kiểm tra hội thoại OpenClaw của agent (phiên bản quản trị viên)
func (ac *AdminController) CallAgentOpenClawChatTest(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}
	if ac.WebSocketController == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "websocket controller unavailable"})
		return
	}

	var req struct {
		Message   string `json:"message" binding:"required"`
		TimeoutMs int    `json:"timeout_ms"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tham số yêu cầu sai: " + err.Error()})
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message không được để trống"})
		return
	}

	var agent models.Agent
	if err := ac.DB.Where("id = ?", agentID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent không tồn tại"})
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
		result, err := ac.WebSocketController.CallOpenClawChatStreamFromClient(
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

	result, err := ac.WebSocketController.CallOpenClawChatFromClient(context.Background(), body)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(strings.ToLower(msg), "not connected"), strings.Contains(msg, "chưa kết nối"):
			c.JSON(http.StatusConflict, gin.H{"error": msg})
		case strings.Contains(strings.ToLower(msg), "timeout"), strings.Contains(msg, "hết thời gian chờ"):
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": msg})
		case strings.Contains(strings.ToLower(msg), "missing"), strings.Contains(msg, "tham số"):
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		case strings.Contains(msg, "không có client kết nối"):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": msg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gọi kiểm tra hội thoại OpenClaw thất bại: " + msg})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GetAgentMcpTools Lấy danh sách công cụ MCP của agent
func (ac *AdminController) GetAgentMcpTools(c *gin.Context) {
	agentID := c.Param("id")

	// Hàm xác thực quản trị viên: xác thực agent có tồn tại hay không (quản trị viên có thể xem agent của bất kỳ người dùng nào)
	adminAgentValidator := func(agentID string) error {
		var agent models.Agent
		if err := ac.DB.Where("id = ?", agentID).First(&agent).Error; err != nil {
			return fmt.Errorf("Agent không tồn tại")
		}
		return nil
	}

	// Sử dụng hàm chung
	GetAgentMcpToolsCommon(c, agentID, ac.WebSocketController, adminAgentValidator)
}

func (ac *AdminController) CreateAgent(c *gin.Context) {
	var req AgentPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	agent, err := NewAgentService(ac.DB).Create(scopeFromContext(c), req)
	if err != nil {
		writeServiceError(c, err, "Tạo agent thất bại")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": agent})
}

func (ac *AdminController) UpdateAgent(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req AgentPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	agent, err := NewAgentService(ac.DB).Update(scopeFromContext(c), id, req)
	if err != nil {
		writeServiceError(c, err, "Cập nhật agent thất bại")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": agent})
}

func (ac *AdminController) DeleteAgent(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := NewAgentService(ac.DB).Delete(scopeFromContext(c), id); err != nil {
		writeServiceError(c, err, "Xóa agent thất bại")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Xóa thành công"})
}

// Quản lý cấu hình VAD (tương thích frontend)
func (ac *AdminController) GetVADConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "vad").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VAD configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateVADConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "vad"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateVADConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "vad")
}

func (ac *AdminController) DeleteVADConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "vad")
}

// Quản lý cấu hình ASR (tương thích frontend)
func (ac *AdminController) GetASRConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "asr").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get ASR configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateASRConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "asr"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateASRConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "asr")
}

func (ac *AdminController) DeleteASRConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "asr")
}

// Quản lý cấu hình LLM (tương thích frontend)
func (ac *AdminController) GetLLMConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "llm").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get LLM configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateLLMConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "llm"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateLLMConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "llm")
}

func (ac *AdminController) DeleteLLMConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "llm")
}

// Quản lý cấu hình TTS (tương thích frontend)
func (ac *AdminController) GetTTSConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "tts").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get TTS configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateTTSConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "tts"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateTTSConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "tts")
}

func (ac *AdminController) DeleteTTSConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "tts")
}

// Quản lý cấu hình Speaker (tương thích frontend)
func (ac *AdminController) GetSpeakerConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "voice_identify").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Speaker configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateSpeakerConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "voice_identify"
	// Cấu hình vân giọng nói chỉ có một, tự động đặt làm cấu hình mặc định
	config.IsDefault = true
	// Nếu đã tồn tại cấu hình, xóa cấu hình cũ trước
	ac.DB.Where("type = ?", "voice_identify").Delete(&models.Config{})
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateSpeakerConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.Where("id = ? AND type = ?", id, "voice_identify").First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cấu hình không tồn tại"})
		return
	}

	var updateData models.Config
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Cấu hình vân giọng nói chỉ có một, luôn đặt làm cấu hình mặc định
	updateData.IsDefault = true

	// Cập nhật cấu hình
	config.Name = updateData.Name
	config.Provider = updateData.Provider
	config.JsonData = updateData.JsonData
	config.Enabled = updateData.Enabled
	config.IsDefault = updateData.IsDefault

	// Nếu cung cấp config_id mới, cập nhật nó
	if updateData.ConfigID != "" {
		config.ConfigID = updateData.ConfigID
	}

	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật cấu hình thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) DeleteSpeakerConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "voice_identify")
}

// Quản lý cấu hình Vision (tương thích frontend)
func (ac *AdminController) GetVisionConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ? AND config_id != ?", "vision", "vision_base").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Vision configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// GetVisionBaseConfig Lấy cấu hình cơ bản của Vision
func (ac *AdminController) GetVisionBaseConfig(c *gin.Context) {
	var config models.Config
	if err := ac.DB.Where("type = ? AND config_id = ?", "vision", "vision_base").First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Nếu không tìm thấy cấu hình cơ bản, trả về giá trị mặc định
			c.JSON(http.StatusOK, gin.H{"data": map[string]interface{}{
				"enable_auth": false,
				"vision_url":  "",
			}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Vision base config"})
		return
	}

	var configData map[string]interface{}
	if err := json.Unmarshal([]byte(config.JsonData), &configData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Vision base config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": configData})
}

// UpdateVisionBaseConfig Cập nhật cấu hình cơ bản của Vision
func (ac *AdminController) UpdateVisionBaseConfig(c *gin.Context) {
	var requestData map[string]interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal config data"})
		return
	}

	var config models.Config
	if err := ac.DB.Where("type = ? AND config_id = ?", "vision", "vision_base").First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Tạo cấu hình cơ bản mới
			config = models.Config{
				Type:      "vision",
				Name:      "vision_base",
				ConfigID:  "vision_base",
				Provider:  "vision_base",
				JsonData:  string(jsonData),
				Enabled:   true,
				IsDefault: false,
			}
			if err := ac.DB.Create(&config).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Vision base config"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query Vision base config"})
			return
		}
	} else {
		// Cập nhật cấu hình hiện có
		config.JsonData = string(jsonData)
		if err := ac.DB.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update Vision base config"})
			return
		}
	}

	ac.notifySystemConfigChanged()
	c.JSON(http.StatusOK, gin.H{"message": "Vision base config updated successfully"})
}

// GetChatSettings Lấy cài đặt chat (auth.enable + chat.*)
func (ac *AdminController) GetChatSettings(c *gin.Context) {
	response := gin.H{
		"auth": gin.H{
			"enable":                false,
			"login_captcha_enabled": true,
		},
		"chat": gin.H{
			"max_idle_duration":         30000,
			"chat_max_silence_duration": 400,
			"realtime_mode":             4,
			"global_system_prompt":      "",
		},
	}

	var authConfig models.Config
	if err := ac.DB.Where("type = ?", "auth").Order("is_default DESC, id ASC").First(&authConfig).Error; err == nil {
		var authData map[string]interface{}
		if authConfig.JsonData != "" && json.Unmarshal([]byte(authConfig.JsonData), &authData) == nil {
			if enable, ok := authData["enable"].(bool); ok {
				response["auth"].(gin.H)["enable"] = enable
			}
			if enabled, ok := authData["login_captcha_enabled"].(bool); ok {
				response["auth"].(gin.H)["login_captcha_enabled"] = enabled
			}
		}
	}

	var chatConfig models.Config
	if err := ac.DB.Where("type = ?", "chat").Order("is_default DESC, id ASC").First(&chatConfig).Error; err == nil {
		var chatData map[string]interface{}
		if chatConfig.JsonData != "" && json.Unmarshal([]byte(chatConfig.JsonData), &chatData) == nil {
			if maxIdle, ok := chatData["max_idle_duration"].(float64); ok && int64(maxIdle) >= 0 {
				response["chat"].(gin.H)["max_idle_duration"] = int64(maxIdle)
			}
			if maxSilence, ok := chatData["chat_max_silence_duration"].(float64); ok && int64(maxSilence) >= 0 {
				response["chat"].(gin.H)["chat_max_silence_duration"] = int64(maxSilence)
			}
			if realtimeMode, ok := chatData["realtime_mode"].(float64); ok && int(realtimeMode) >= 1 && int(realtimeMode) <= 4 {
				response["chat"].(gin.H)["realtime_mode"] = int(realtimeMode)
			}
			if globalPrompt, ok := chatData["global_system_prompt"].(string); ok {
				response["chat"].(gin.H)["global_system_prompt"] = strings.TrimSpace(globalPrompt)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

// UpdateChatSettings Cập nhật cài đặt chat (auth.enable + chat.*)
func (ac *AdminController) UpdateChatSettings(c *gin.Context) {
	var req struct {
		Auth struct {
			Enable              bool  `json:"enable"`
			LoginCaptchaEnabled *bool `json:"login_captcha_enabled"`
		} `json:"auth"`
		Chat struct {
			MaxIdleDuration        int64  `json:"max_idle_duration"`
			ChatMaxSilenceDuration int64  `json:"chat_max_silence_duration"`
			RealtimeMode           int    `json:"realtime_mode"`
			GlobalSystemPrompt     string `json:"global_system_prompt"`
		} `json:"chat"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Chat.MaxIdleDuration < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chat.max_idle_duration không được nhỏ hơn 0, 0 nghĩa là không giới hạn"})
		return
	}
	if req.Chat.ChatMaxSilenceDuration < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chat.chat_max_silence_duration không được nhỏ hơn 0"})
		return
	}
	if req.Chat.RealtimeMode < 1 || req.Chat.RealtimeMode > 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chat.realtime_mode phải nằm trong khoảng 1-4"})
		return
	}
	req.Chat.GlobalSystemPrompt = strings.TrimSpace(req.Chat.GlobalSystemPrompt)
	if len(req.Chat.GlobalSystemPrompt) > 8000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Độ dài chat.global_system_prompt không được vượt quá 8000 ký tự"})
		return
	}

	loginCaptchaEnabled := true
	if req.Auth.LoginCaptchaEnabled != nil {
		loginCaptchaEnabled = *req.Auth.LoginCaptchaEnabled
	}

	authJSON, err := json.Marshal(map[string]interface{}{
		"enable":                req.Auth.Enable,
		"login_captcha_enabled": loginCaptchaEnabled,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tuần tự hóa cấu hình auth thất bại"})
		return
	}
	chatJSON, err := json.Marshal(map[string]interface{}{
		"max_idle_duration":         req.Chat.MaxIdleDuration,
		"chat_max_silence_duration": req.Chat.ChatMaxSilenceDuration,
		"realtime_mode":             req.Chat.RealtimeMode,
		"global_system_prompt":      req.Chat.GlobalSystemPrompt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tuần tự hóa cấu hình chat thất bại"})
		return
	}

	tx := ac.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Khởi động transaction thất bại"})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	upsertConfig := func(configType, configID, name string, jsonData []byte) error {
		var cfg models.Config
		err := tx.Where("type = ? AND config_id = ?", configType, configID).First(&cfg).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Model(&models.Config{}).Where("type = ?", configType).Update("is_default", false).Error; err != nil {
				return err
			}
			cfg = models.Config{
				Type:      configType,
				Name:      name,
				ConfigID:  configID,
				Provider:  "",
				JsonData:  string(jsonData),
				Enabled:   true,
				IsDefault: true,
			}
			return tx.Create(&cfg).Error
		}
		if err != nil {
			return err
		}

		if err := tx.Model(&models.Config{}).Where("type = ? AND id != ?", configType, cfg.ID).Update("is_default", false).Error; err != nil {
			return err
		}

		cfg.Name = name
		cfg.Provider = ""
		cfg.JsonData = string(jsonData)
		cfg.Enabled = true
		cfg.IsDefault = true
		return tx.Save(&cfg).Error
	}

	if err := upsertConfig("auth", "auth", "auth", authJSON); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu cài đặt auth thất bại: " + err.Error()})
		return
	}
	if err := upsertConfig("chat", "chat", "chat", chatJSON); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu cài đặt chat thất bại: " + err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Commit transaction thất bại"})
		return
	}

	ac.notifySystemConfigChanged()
	c.JSON(http.StatusOK, gin.H{
		"message": "Cập nhật cài đặt chat thành công",
		"data": gin.H{
			"auth": gin.H{
				"enable":                req.Auth.Enable,
				"login_captcha_enabled": loginCaptchaEnabled,
			},
			"chat": gin.H{
				"max_idle_duration":         req.Chat.MaxIdleDuration,
				"chat_max_silence_duration": req.Chat.ChatMaxSilenceDuration,
				"realtime_mode":             req.Chat.RealtimeMode,
				"global_system_prompt":      req.Chat.GlobalSystemPrompt,
			},
		},
	})
}

func (ac *AdminController) CreateVisionConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "vision"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateVisionConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "vision")
}

func (ac *AdminController) DeleteVisionConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "vision")
}

// Quản lý cấu hình OTA (tương thích frontend)
func (ac *AdminController) GetOTAConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "ota").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get OTA configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateOTAConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "ota"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateOTAConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "ota")
}

func (ac *AdminController) DeleteOTAConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "ota")
}

// Quản lý cấu hình MQTT (tương thích frontend)
func (ac *AdminController) GetMQTTConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "mqtt").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get MQTT configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateMQTTConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "mqtt"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateMQTTConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "mqtt")
}

func (ac *AdminController) DeleteMQTTConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "mqtt")
}

// Quản lý cấu hình MQTT Server (tương thích frontend)
func (ac *AdminController) GetMQTTServerConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "mqtt_server").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get MQTT Server configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateMQTTServerConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "mqtt_server"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateMQTTServerConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "mqtt_server")
}

func (ac *AdminController) DeleteMQTTServerConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "mqtt_server")
}

// Quản lý cấu hình UDP (tương thích frontend)
func (ac *AdminController) GetUDPConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "udp").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get UDP configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateUDPConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "udp"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateUDPConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "udp")
}

func (ac *AdminController) DeleteUDPConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "udp")
}

// ToggleConfigEnable Chuyển đổi trạng thái bật/tắt của cấu hình
func (ac *AdminController) ToggleConfigEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	var config models.Config
	if err := ac.DB.First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cấu hình không tồn tại"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn cấu hình thất bại"})
		}
		return
	}

	// Chuyển đổi trạng thái bật/tắt
	config.Enabled = !config.Enabled
	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật trạng thái cấu hình thất bại"})
		return
	}

	ac.notifySystemConfigChanged()
	status := "vô hiệu hóa"
	if config.Enabled {
		status = "kích hoạt"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Cấu hình đã %s", status),
		"data":    config,
	})
}

// Phương thức hỗ trợ
func (ac *AdminController) createConfigWithType(c *gin.Context, config *models.Config) {
	// Nếu không cung cấp config_id, tự động tạo một cái
	if config.ConfigID == "" {
		// Tạo ID duy nhất theo định dạng loại_tên_dấu thời gian
		timestamp := time.Now().Unix()
		safeName := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(config.Name, " ", "_"), "-", "_"))
		config.ConfigID = fmt.Sprintf("%s_%s_%d", config.Type, safeName, timestamp)
	}

	// Nếu đặt làm cấu hình mặc định, hủy các cấu hình mặc định cùng loại khác trước
	if config.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)
	}

	if err := ac.DB.Create(config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo cấu hình thất bại"})
		return
	}

	ac.notifySystemConfigChanged()
	c.JSON(http.StatusCreated, gin.H{"data": *config})
}

// configUpdateBody Dùng cho updateConfigWithType, json_data tương thích với frontend truyền string hoặc object
type configUpdateBody struct {
	Name      string      `json:"name"`
	ConfigID  string      `json:"config_id"`
	Provider  string      `json:"provider"`
	JsonData  interface{} `json:"json_data"`
	Enabled   bool        `json:"enabled"`
	IsDefault bool        `json:"is_default"`
}

func (ac *AdminController) updateConfigWithType(c *gin.Context, configType string) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.Where("id = ? AND type = ?", id, configType).First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cấu hình không tồn tại"})
		return
	}

	var updateData configUpdateBody
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Nếu đặt làm cấu hình mặc định, hủy các cấu hình mặc định cùng loại khác trước
	if updateData.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ? AND id != ?", configType, true, id).Update("is_default", false)
	}

	// Cập nhật cấu hình
	config.Name = updateData.Name
	config.Provider = updateData.Provider
	config.Enabled = updateData.Enabled
	config.IsDefault = updateData.IsDefault

	// json_data: tương thích string hoặc object, tránh việc bind thất bại khi frontend truyền object
	switch v := updateData.JsonData.(type) {
	case string:
		config.JsonData = v
	case nil:
		// Nếu không truyền thì giữ nguyên giá trị cũ
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Định dạng json_data không hợp lệ"})
			return
		}
		config.JsonData = string(bytes)
	}

	// Nếu cung cấp config_id mới, cập nhật nó
	if updateData.ConfigID != "" {
		config.ConfigID = updateData.ConfigID
	}

	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật cấu hình thất bại: " + err.Error()})
		return
	}

	ac.notifySystemConfigChanged()
	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) deleteConfigWithType(c *gin.Context, configType string) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Where("id = ? AND type = ?", id, configType).Delete(&models.Config{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa cấu hình thất bại"})
		return
	}
	ac.notifySystemConfigChanged()
	c.JSON(http.StatusOK, gin.H{"message": "Xóa thành công"})
}

// Các phương thức liên quan đến import/export cấu hình
// ExportConfigs Xuất tất cả cấu hình sang định dạng YAML
func (ac *AdminController) ExportConfigs(c *gin.Context) {
	// Xây dựng cấu trúc cấu hình xuất - chỉ bao gồm các module thực sự tồn tại
	type ExportConfig struct {
		VAD           map[string]interface{} `yaml:"vad,omitempty"`
		ASR           map[string]interface{} `yaml:"asr,omitempty"`
		LLM           map[string]interface{} `yaml:"llm,omitempty"`
		TTS           map[string]interface{} `yaml:"tts,omitempty"`
		Vision        map[string]interface{} `yaml:"vision,omitempty"`
		Memory        map[string]interface{} `yaml:"memory,omitempty"`
		VoiceIdentify map[string]interface{} `yaml:"voice_identify,omitempty"`
		Auth          map[string]interface{} `yaml:"auth,omitempty"`
		Chat          map[string]interface{} `yaml:"chat,omitempty"`
		MQTT          map[string]interface{} `yaml:"mqtt,omitempty"`
		MQTTServer    map[string]interface{} `yaml:"mqtt_server,omitempty"`
		UDP           map[string]interface{} `yaml:"udp,omitempty"`
		OTA           map[string]interface{} `yaml:"ota,omitempty"`
		MCP           map[string]interface{} `yaml:"mcp,omitempty"`
		LocalMCP      map[string]interface{} `yaml:"local_mcp,omitempty"`
	}

	exportConfig := ExportConfig{
		VAD:           make(map[string]interface{}),
		ASR:           make(map[string]interface{}),
		LLM:           make(map[string]interface{}),
		TTS:           make(map[string]interface{}),
		Vision:        make(map[string]interface{}),
		Memory:        make(map[string]interface{}),
		VoiceIdentify: make(map[string]interface{}),
		Auth:          make(map[string]interface{}),
		Chat:          make(map[string]interface{}),
		MQTT:          make(map[string]interface{}),
		MQTTServer:    make(map[string]interface{}),
		UDP:           make(map[string]interface{}),
		OTA:           make(map[string]interface{}),
		MCP:           make(map[string]interface{}),
		LocalMCP:      make(map[string]interface{}),
	}

	// Lấy tất cả cấu hình
	var configs []models.Config
	if err := ac.DB.Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get configs"})
		return
	}

	// Lấy role toàn cục
	var globalRoles []models.GlobalRole
	if err := ac.DB.Find(&globalRoles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get global roles"})
		return
	}

	// Xử lý dữ liệu cấu hình - trường provider tương ứng với is_default, key tương ứng với ConfigID
	for _, config := range configs {
		var jsonData map[string]interface{}
		if err := json.Unmarshal([]byte(config.JsonData), &jsonData); err != nil {
			log.Printf("Failed to unmarshal config %s: %v", config.ConfigID, err)
			continue
		}

		// Tổ chức dữ liệu theo loại cấu hình
		switch config.Type {
		case "vad":
			// Tương thích định dạng cũ: nếu chỉ có một key, nghĩa là định dạng cũ (có key), trích xuất cấu hình bên trong
			var actualConfigData map[string]interface{}
			if len(jsonData) == 1 {
				// Định dạng cũ: chỉ có một key, trích xuất giá trị của nó
				for _, value := range jsonData {
					if innerConfig, ok := value.(map[string]interface{}); ok {
						actualConfigData = innerConfig
					} else {
						// Nếu không phải kiểu map, sử dụng trực tiếp dữ liệu gốc
						actualConfigData = jsonData
					}
					break
				}
			} else {
				// Định dạng mới: không có key, sử dụng trực tiếp jsonData
				actualConfigData = jsonData
			}
			// Nếu là cấu hình mặc định, thiết lập trường provider
			if config.IsDefault {
				exportConfig.VAD["provider"] = config.ConfigID
			}
			// Sử dụng ConfigID làm key
			exportConfig.VAD[config.ConfigID] = configprovider.ExportData(config.Type, config.ConfigID, config.Provider, actualConfigData)
		case "asr":
			if config.IsDefault {
				exportConfig.ASR["provider"] = config.ConfigID
			}
			exportConfig.ASR[config.ConfigID] = configprovider.ExportData(config.Type, config.ConfigID, config.Provider, jsonData)
		case "llm":
			if config.IsDefault {
				exportConfig.LLM["provider"] = config.ConfigID
			}
			exportConfig.LLM[config.ConfigID] = configprovider.ExportData(config.Type, config.ConfigID, config.Provider, jsonData)
		case "tts":
			if config.IsDefault {
				exportConfig.TTS["provider"] = config.ConfigID
			}
			exportConfig.TTS[config.ConfigID] = configprovider.ExportData(config.Type, config.ConfigID, config.Provider, jsonData)
		case "vision":
			// Xử lý đặc biệt cấu hình vision
			if config.ConfigID == "vision_base" {
				// Xử lý cấu hình cơ bản (enable_auth, vision_url, v.v.)
				for key, value := range jsonData {
					exportConfig.Vision[key] = value
				}
			} else {
				// Xử lý cấu hình vllm
				if exportConfig.Vision["vllm"] == nil {
					exportConfig.Vision["vllm"] = make(map[string]interface{})
				}
				if vllmConfig, ok := exportConfig.Vision["vllm"].(map[string]interface{}); ok {
					if config.IsDefault {
						vllmConfig["provider"] = config.ConfigID
					}
					vllmConfig[config.ConfigID] = configprovider.ExportData(config.Type, config.ConfigID, config.Provider, jsonData)
				}
			}
		case "ota":
			// ota, mqtt, mqtt_server, udp không cần trường provider, gộp cấu hình trực tiếp
			for key, value := range jsonData {
				exportConfig.OTA[key] = value
			}
		case "mqtt":
			// ota, mqtt, mqtt_server, udp không cần trường provider, gộp cấu hình trực tiếp
			for key, value := range jsonData {
				exportConfig.MQTT[key] = value
			}
		case "mqtt_server":
			// ota, mqtt, mqtt_server, udp không cần trường provider, gộp cấu hình trực tiếp
			for key, value := range jsonData {
				exportConfig.MQTTServer[key] = value
			}
		case "udp":
			// ota, mqtt, mqtt_server, udp không cần trường provider, gộp cấu hình trực tiếp
			for key, value := range jsonData {
				exportConfig.UDP[key] = value
			}
		case "memory":
			if config.IsDefault {
				exportConfig.Memory["provider"] = config.ConfigID
			}
			exportConfig.Memory[config.ConfigID] = configprovider.ExportData(config.Type, config.ConfigID, config.Provider, jsonData)
		case "voice_identify":
			if config.IsDefault {
				exportConfig.VoiceIdentify["provider"] = config.ConfigID
			}
			exportConfig.VoiceIdentify[config.ConfigID] = jsonData
		case "auth":
			for key, value := range jsonData {
				exportConfig.Auth[key] = value
			}
		case "chat":
			for key, value := range jsonData {
				exportConfig.Chat[key] = value
			}
		case "mcp":
			// Xử lý cấu hình MCP, tách mcp và local_mcp
			if mcpData, exists := jsonData["mcp"]; exists {
				if mcpMap, ok := mcpData.(map[string]interface{}); ok {
					for key, value := range mcpMap {
						exportConfig.MCP[key] = value
					}
				}
			}
			// Tương thích định dạng cũ: nếu có trực tiếp trường global
			if globalData, exists := jsonData["global"]; exists {
				exportConfig.MCP["global"] = globalData
			}
		case "local_mcp":
			// Xử lý cấu hình local_mcp
			for key, value := range jsonData {
				exportConfig.LocalMCP[key] = value
			}
		}
	}

	// Chỉ xử lý cấu hình thực tế trong cơ sở dữ liệu, không đặt giá trị mặc định

	// Chuyển đổi sang YAML
	yamlData, err := yaml.Marshal(exportConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal YAML"})
		return
	}

	// Thiết lập header phản hồi
	c.Header("Content-Type", "application/x-yaml")
	c.Header("Content-Disposition", "attachment; filename=config.yaml")
	c.Data(http.StatusOK, "application/x-yaml", yamlData)
}

// ImportConfigs Nhập cấu hình từ file YAML
func (ac *AdminController) ImportConfigs(c *gin.Context) {
	log.Printf("Bắt đầu nhập cấu hình")

	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("Lấy file đã upload thất bại: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	log.Printf("Thông tin file: filename=%s, size=%d", file.Filename, file.Size)

	if file.Size == 0 {
		log.Printf("File trống")
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is empty"})
		return
	}

	// Đọc nội dung file
	src, err := file.Open()
	if err != nil {
		log.Printf("Mở file thất bại: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		log.Printf("Đọc nội dung file thất bại: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	log.Printf("Độ dài nội dung file: %d", len(content))

	// Phân tích YAML
	var importConfig map[string]interface{}
	if err := yaml.Unmarshal(content, &importConfig); err != nil {
		log.Printf("Phân tích YAML thất bại: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid YAML format"})
		return
	}

	log.Printf("Phân tích YAML thành công, key cấu hình: %v", getMapKeys(importConfig))

	// Bắt đầu transaction
	log.Printf("Bắt đầu transaction cơ sở dữ liệu")
	tx := ac.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Xảy ra panic, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()

	// Xóa sạch cấu hình hiện có
	log.Printf("Xóa sạch cấu hình hiện có")
	result := tx.Exec("DELETE FROM configs")
	if result.Error != nil {
		log.Printf("Xóa sạch cấu hình thất bại: %v", result.Error)
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear existing configs"})
		return
	}
	log.Printf("Xóa sạch cấu hình thành công, đã xóa %d bản ghi", result.RowsAffected)

	// Xóa sạch role toàn cục
	log.Printf("Xóa sạch role toàn cục")
	result2 := tx.Exec("DELETE FROM global_roles")
	if result2.Error != nil {
		log.Printf("Xóa sạch role toàn cục thất bại: %v", result2.Error)
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear existing global roles"})
		return
	}
	log.Printf("Xóa sạch role toàn cục thành công, đã xóa %d bản ghi", result2.RowsAffected)

	// Nhập cấu hình - chỉ xử lý các module thực sự tồn tại
	configTypes := []string{"vad", "asr", "llm", "tts", "memory", "auth", "chat", "ota", "mqtt", "mqtt_server", "udp", "mcp", "local_mcp"}
	log.Printf("Bắt đầu nhập cấu hình, loại cấu hình: %v", configTypes)

	// Xử lý cấu hình voice_identify (ánh xạ sang loại speaker)
	if voiceIdentifyData, exists := importConfig["voice_identify"]; exists {
		log.Printf("Tìm thấy dữ liệu cấu hình voice_identify")
		if voiceIdentifyMap, ok := voiceIdentifyData.(map[string]interface{}); ok {
			log.Printf("Các key map cấu hình voice_identify: %v", getMapKeys(voiceIdentifyMap))

			// Lấy trường provider
			var defaultProvider string
			if provider, exists := voiceIdentifyMap["provider"]; exists {
				if providerStr, ok := provider.(string); ok {
					defaultProvider = providerStr
					log.Printf("Provider mặc định của voice_identify: %s", defaultProvider)
				}
			}

			log.Printf("Các key mục cấu hình voice_identify: %v", getMapKeys(voiceIdentifyMap))
			// Cấu hình vân giọng nói chỉ có một, ưu tiên dùng cấu hình do provider chỉ định, nếu không thì dùng mục cấu hình đầu tiên
			var targetConfigID string
			if defaultProvider != "" {
				targetConfigID = defaultProvider
			} else {
				// Nếu không có provider, dùng mục cấu hình đầu tiên không phải provider
				for key := range voiceIdentifyMap {
					if key != "provider" {
						targetConfigID = key
						break
					}
				}
			}

			if targetConfigID == "" {
				log.Printf("Không tìm thấy mục cấu hình hợp lệ trong cấu hình voice_identify")
			} else {
				// Chỉ xử lý mục cấu hình mục tiêu
				if configValue, exists := voiceIdentifyMap[targetConfigID]; exists {
					if configMap, ok := configValue.(map[string]interface{}); ok {
						log.Printf("Xử lý mục cấu hình voice_identify: %s", targetConfigID)
						jsonData, err := json.Marshal(configMap)
						if err != nil {
							log.Printf("Tuần tự hóa dữ liệu cấu hình voice_identify thất bại: %v", err)
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal voice_identify config data"})
							return
						}

						// Cấu hình vân giọng nói chỉ có một, luôn đặt làm mặc định
						config := models.Config{
							Type:      "voice_identify",
							Name:      "Cấu hình nhận dạng vân giọng nói",
							ConfigID:  "asr_server",
							Provider:  "asr_server",
							JsonData:  string(jsonData),
							Enabled:   true,
							IsDefault: true,
						}

						log.Printf("Chuẩn bị lưu cấu hình voice_identify: Type=%s, Name=%s, ConfigID=%s", config.Type, config.Name, config.ConfigID)

						// Cấu hình vân giọng nói chỉ có một, xóa tất cả cấu hình cũ trước
						tx.Where("type = ?", "voice_identify").Delete(&models.Config{})

						// Tạo cấu hình mới
						if err := tx.Create(&config).Error; err != nil {
							log.Printf("Tạo cấu hình voice_identify thất bại: %v", err)
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create voice_identify config"})
							return
						}
						log.Printf("Tạo cấu hình voice_identify thành công: %s", targetConfigID)
					}
				}
			}
		}
	}

	for _, configType := range configTypes {
		log.Printf("Xử lý loại cấu hình: %s", configType)
		if configData, exists := importConfig[configType]; exists {
			log.Printf("Tìm thấy dữ liệu của loại cấu hình %s", configType)
			if configMap, ok := configData.(map[string]interface{}); ok {
				// Đối với các module cần provider (vad, asr, llm, tts, memory), xử lý trường provider
				if configType == "vad" || configType == "asr" || configType == "llm" || configType == "tts" || configType == "memory" || configType == "voice_identify" {
					log.Printf("Xử lý loại cấu hình cần provider: %s", configType)
					// Lấy trường provider
					var defaultProvider string
					if provider, exists := configMap["provider"]; exists {
						if providerStr, ok := provider.(string); ok {
							defaultProvider = providerStr
							log.Printf("Provider mặc định: %s", defaultProvider)
						}
					}

					log.Printf("Các key mục cấu hình: %v", getMapKeys(configMap))
					// Duyệt qua tất cả các mục cấu hình
					for configID, configValue := range configMap {
						// Bỏ qua trường provider
						if configID == "provider" {
							log.Printf("Bỏ qua trường provider")
							continue
						}

						if configMap, ok := configValue.(map[string]interface{}); ok {
							log.Printf("Xử lý mục cấu hình: %s", configID)
							providerName := configprovider.NormalizeProvider(configType, configID, configMap)
							if providerName != "" {
								configMap["provider"] = providerName
							}
							jsonData, err := json.Marshal(configMap)
							if err != nil {
								log.Printf("Tuần tự hóa dữ liệu cấu hình thất bại: %v", err)
								tx.Rollback()
								c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal config data"})
								return
							}

							// Xác định có phải cấu hình mặc định hay không
							isDefault := (configID == defaultProvider)
							log.Printf("Mục cấu hình %s, có phải mặc định: %v", configID, isDefault)

							config := models.Config{
								Type:      configType,
								Name:      configID,
								ConfigID:  configID,
								Provider:  providerName,
								JsonData:  string(jsonData),
								Enabled:   true,
								IsDefault: isDefault,
							}

							log.Printf("Chuẩn bị lưu cấu hình: Type=%s, Name=%s, ConfigID=%s", config.Type, config.Name, config.ConfigID)

							// Kiểm tra xem đã tồn tại cấu hình giống hệt chưa
							var existingConfig models.Config
							if err := tx.Where("type = ? AND config_id = ?", config.Type, config.ConfigID).First(&existingConfig).Error; err == nil {
								log.Printf("Cấu hình đã tồn tại, sẽ cập nhật: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
								// Cập nhật cấu hình hiện có
								existingConfig.Name = config.Name
								existingConfig.Provider = config.Provider
								existingConfig.JsonData = config.JsonData
								existingConfig.Enabled = config.Enabled
								existingConfig.IsDefault = config.IsDefault
								if err := tx.Save(&existingConfig).Error; err != nil {
									log.Printf("Cập nhật cấu hình thất bại: %v", err)
									tx.Rollback()
									c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update config"})
									return
								}
								log.Printf("Cập nhật cấu hình thành công: %s", configID)
							} else if err == gorm.ErrRecordNotFound {
								log.Printf("Cấu hình không tồn tại, sẽ tạo cấu hình mới: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
								// Tạo cấu hình mới
								if err := tx.Create(&config).Error; err != nil {
									log.Printf("Tạo cấu hình thất bại: %v", err)
									tx.Rollback()
									c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create config"})
									return
								}
								log.Printf("Tạo cấu hình thành công: %s", configID)
							} else {
								log.Printf("Xảy ra lỗi khi truy vấn cấu hình: %v", err)
								tx.Rollback()
								c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query existing config"})
								return
							}
						}
					}
				} else {
					// Đối với các module không cần provider (ota, mqtt, mqtt_server, udp, mcp, local_mcp), tạo cấu hình trực tiếp
					log.Printf("Xử lý loại cấu hình không cần provider: %s", configType)
					jsonData, err := json.Marshal(configMap)
					if err != nil {
						log.Printf("Tuần tự hóa dữ liệu cấu hình thất bại: %v", err)
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal config data"})
						return
					}

					config := models.Config{
						Type:      configType,
						Name:      configType,
						ConfigID:  configType,
						Provider:  "",
						JsonData:  string(jsonData),
						Enabled:   true,
						IsDefault: true,
					}

					log.Printf("Chuẩn bị lưu cấu hình: Type=%s, Name=%s, ConfigID=%s", config.Type, config.Name, config.ConfigID)

					// Kiểm tra xem đã tồn tại cấu hình giống hệt chưa
					var existingConfig models.Config
					if err := tx.Where("type = ? AND config_id = ?", config.Type, config.ConfigID).First(&existingConfig).Error; err == nil {
						log.Printf("Cấu hình đã tồn tại, sẽ cập nhật: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
						// Cập nhật cấu hình hiện có
						existingConfig.Name = config.Name
						existingConfig.Provider = config.Provider
						existingConfig.JsonData = config.JsonData
						existingConfig.Enabled = config.Enabled
						existingConfig.IsDefault = config.IsDefault
						if err := tx.Save(&existingConfig).Error; err != nil {
							log.Printf("Cập nhật cấu hình thất bại: %v", err)
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update config"})
							return
						}
						log.Printf("Cập nhật cấu hình thành công: %s", configType)
					} else if err == gorm.ErrRecordNotFound {
						log.Printf("Cấu hình không tồn tại, sẽ tạo cấu hình mới: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
						// Tạo cấu hình mới
						if err := tx.Create(&config).Error; err != nil {
							log.Printf("Tạo cấu hình thất bại: %v", err)
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create config"})
							return
						}
						log.Printf("Tạo cấu hình thành công: %s", configType)
					} else {
						log.Printf("Xảy ra lỗi khi truy vấn cấu hình: %v", err)
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query existing config"})
						return
					}
				}
			}
		}
	}

	// Xử lý đặc biệt cấu hình vision
	log.Printf("Bắt đầu xử lý cấu hình vision")
	if visionData, exists := importConfig["vision"]; exists {
		log.Printf("Tìm thấy dữ liệu cấu hình vision")
		if visionMap, ok := visionData.(map[string]interface{}); ok {
			log.Printf("Các key map cấu hình vision: %v", getMapKeys(visionMap))

			// Xử lý cấu hình cơ bản của vision (enable_auth, vision_url, v.v.)
			baseVisionConfig := make(map[string]interface{})
			for key, value := range visionMap {
				if key != "vllm" {
					baseVisionConfig[key] = value
				}
			}

			// Lưu cấu hình cơ bản của vision
			if len(baseVisionConfig) > 0 {
				jsonData, err := json.Marshal(baseVisionConfig)
				if err != nil {
					log.Printf("Tuần tự hóa dữ liệu cấu hình cơ bản vision thất bại: %v", err)
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal vision base config data"})
					return
				}

				config := models.Config{
					Type:      "vision",
					Name:      "vision_base",
					ConfigID:  "vision_base",
					Provider:  "vision_base",
					JsonData:  string(jsonData),
					Enabled:   true,
					IsDefault: false,
				}

				log.Printf("Chuẩn bị lưu cấu hình cơ bản vision: Type=%s, Name=%s, ConfigID=%s", config.Type, config.Name, config.ConfigID)

				// Kiểm tra xem đã tồn tại cấu hình giống hệt chưa
				var existingConfig models.Config
				if err := tx.Where("type = ? AND config_id = ?", config.Type, config.ConfigID).First(&existingConfig).Error; err == nil {
					log.Printf("Cấu hình cơ bản vision đã tồn tại, sẽ cập nhật: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
					// Cập nhật cấu hình hiện có
					existingConfig.Name = config.Name
					existingConfig.Provider = config.Provider
					existingConfig.JsonData = config.JsonData
					existingConfig.Enabled = config.Enabled
					existingConfig.IsDefault = config.IsDefault
					if err := tx.Save(&existingConfig).Error; err != nil {
						log.Printf("Cập nhật cấu hình cơ bản vision thất bại: %v", err)
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vision base config"})
						return
					}
					log.Printf("Cập nhật cấu hình cơ bản vision thành công")
				} else if err == gorm.ErrRecordNotFound {
					log.Printf("Cấu hình cơ bản vision không tồn tại, sẽ tạo cấu hình mới: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
					// Tạo cấu hình mới
					if err := tx.Create(&config).Error; err != nil {
						log.Printf("Tạo cấu hình cơ bản vision thất bại: %v", err)
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create vision base config"})
						return
					}
					log.Printf("Tạo cấu hình cơ bản vision thành công")
				} else {
					log.Printf("Xảy ra lỗi khi truy vấn cấu hình cơ bản vision: %v", err)
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query existing vision base config"})
					return
				}
			}

			// Xử lý cấu hình vllm
			if vllmData, exists := visionMap["vllm"]; exists {
				log.Printf("Tìm thấy dữ liệu cấu hình vllm")
				if vllmMap, ok := vllmData.(map[string]interface{}); ok {
					log.Printf("Các key map cấu hình vllm: %v", getMapKeys(vllmMap))

					// Lấy trường provider của vllm
					var defaultProvider string
					if provider, exists := vllmMap["provider"]; exists {
						if providerStr, ok := provider.(string); ok {
							defaultProvider = providerStr
							log.Printf("Provider mặc định của vllm: %s", defaultProvider)
						}
					}

					log.Printf("Các key mục cấu hình vllm: %v", getMapKeys(vllmMap))
					// Duyệt qua tất cả các mục cấu hình vllm
					for configID, configValue := range vllmMap {
						// Bỏ qua trường provider
						if configID == "provider" {
							log.Printf("Bỏ qua trường provider của vllm")
							continue
						}

						if configMap, ok := configValue.(map[string]interface{}); ok {
							log.Printf("Xử lý mục cấu hình vllm: %s", configID)
							providerName := configprovider.NormalizeProvider("vision", configID, configMap)
							if providerName != "" {
								configMap["provider"] = providerName
							}
							jsonData, err := json.Marshal(configMap)
							if err != nil {
								log.Printf("Tuần tự hóa dữ liệu cấu hình vllm thất bại: %v", err)
								tx.Rollback()
								c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal vllm config data"})
								return
							}

							// Xác định có phải cấu hình mặc định hay không
							isDefault := (configID == defaultProvider)
							log.Printf("Mục cấu hình vllm %s, có phải mặc định: %v", configID, isDefault)

							config := models.Config{
								Type:      "vision",
								Name:      configID,
								ConfigID:  configID,
								Provider:  providerName,
								JsonData:  string(jsonData),
								Enabled:   true,
								IsDefault: isDefault,
							}

							log.Printf("Chuẩn bị lưu cấu hình vllm: Type=%s, Name=%s, ConfigID=%s", config.Type, config.Name, config.ConfigID)

							// Kiểm tra xem đã tồn tại cấu hình giống hệt chưa
							var existingConfig models.Config
							if err := tx.Where("type = ? AND config_id = ?", config.Type, config.ConfigID).First(&existingConfig).Error; err == nil {
								log.Printf("Cấu hình vllm đã tồn tại, sẽ cập nhật: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
								// Cập nhật cấu hình hiện có
								existingConfig.Name = config.Name
								existingConfig.Provider = config.Provider
								existingConfig.JsonData = config.JsonData
								existingConfig.Enabled = config.Enabled
								existingConfig.IsDefault = config.IsDefault
								if err := tx.Save(&existingConfig).Error; err != nil {
									log.Printf("Cập nhật cấu hình vllm thất bại: %v", err)
									tx.Rollback()
									c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vllm config"})
									return
								}
								log.Printf("Cập nhật cấu hình vllm thành công: %s", configID)
							} else if err == gorm.ErrRecordNotFound {
								log.Printf("Cấu hình vllm không tồn tại, sẽ tạo cấu hình mới: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
								// Tạo cấu hình mới
								if err := tx.Create(&config).Error; err != nil {
									log.Printf("Tạo cấu hình vllm thất bại: %v", err)
									tx.Rollback()
									c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create vllm config"})
									return
								}
								log.Printf("Tạo cấu hình vllm thành công: %s", configID)
							} else {
								log.Printf("Xảy ra lỗi khi truy vấn cấu hình vllm: %v", err)
								tx.Rollback()
								c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query existing vllm config"})
								return
							}
						}
					}
				}
			}
		}
	}

	// Xử lý đặc biệt cấu hình local_mcp
	log.Printf("Bắt đầu xử lý cấu hình local_mcp")
	if localMcpData, exists := importConfig["local_mcp"]; exists {
		log.Printf("Tìm thấy dữ liệu cấu hình local_mcp")
		if localMcpMap, ok := localMcpData.(map[string]interface{}); ok {
			log.Printf("Các key map cấu hình local_mcp: %v", getMapKeys(localMcpMap))

			jsonData, err := json.Marshal(localMcpMap)
			if err != nil {
				log.Printf("Tuần tự hóa dữ liệu cấu hình local_mcp thất bại: %v", err)
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal local_mcp config data"})
				return
			}

			config := models.Config{
				Type:      "local_mcp",
				Name:      "local_mcp",
				ConfigID:  "local_mcp",
				Provider:  "",
				JsonData:  string(jsonData),
				Enabled:   true,
				IsDefault: true,
			}

			log.Printf("Chuẩn bị lưu cấu hình local_mcp: Type=%s, Name=%s, ConfigID=%s", config.Type, config.Name, config.ConfigID)

			// Kiểm tra xem đã tồn tại cấu hình giống hệt chưa
			var existingConfig models.Config
			if err := tx.Where("type = ? AND config_id = ?", config.Type, config.ConfigID).First(&existingConfig).Error; err == nil {
				log.Printf("Cấu hình local_mcp đã tồn tại, sẽ cập nhật: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
				// Cập nhật cấu hình hiện có
				existingConfig.Name = config.Name
				existingConfig.Provider = config.Provider
				existingConfig.JsonData = config.JsonData
				existingConfig.Enabled = config.Enabled
				existingConfig.IsDefault = config.IsDefault
				if err := tx.Save(&existingConfig).Error; err != nil {
					log.Printf("Cập nhật cấu hình local_mcp thất bại: %v", err)
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update local_mcp config"})
					return
				}
				log.Printf("Cập nhật cấu hình local_mcp thành công")
			} else if err == gorm.ErrRecordNotFound {
				log.Printf("Cấu hình local_mcp không tồn tại, sẽ tạo cấu hình mới: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
				// Tạo cấu hình mới
				if err := tx.Create(&config).Error; err != nil {
					log.Printf("Tạo cấu hình local_mcp thất bại: %v", err)
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create local_mcp config"})
					return
				}
				log.Printf("Tạo cấu hình local_mcp thành công")
			} else {
				log.Printf("Xảy ra lỗi khi truy vấn cấu hình local_mcp: %v", err)
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query existing local_mcp config"})
				return
			}
		}
	}

	// Commit transaction
	log.Printf("Commit transaction")
	if err := tx.Commit().Error; err != nil {
		log.Printf("Commit transaction thất bại: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	log.Printf("Nhập cấu hình thành công")
	ac.notifySystemConfigChanged()
	c.JSON(http.StatusOK, gin.H{"message": "Configuration imported successfully"})
}

// Các phương thức liên quan đến cấu hình MCP
func (ac *AdminController) GetMCPConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "mcp").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy danh sách cấu hình MCP thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateMCPConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config.Type = "mcp"

	// Nếu đặt làm cấu hình mặc định, hủy các cấu hình mặc định cùng loại khác trước
	if config.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)
	}

	if err := ac.DB.Create(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo cấu hình MCP thất bại"})
		return
	}
	ac.notifySystemConfigChanged()
	c.JSON(http.StatusCreated, gin.H{"data": config})
}

func (ac *AdminController) UpdateMCPConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cấu hình MCP không tồn tại"})
		return
	}

	var updateData models.Config
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Nếu đặt làm cấu hình mặc định, hủy các cấu hình mặc định cùng loại khác trước
	if updateData.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ? AND id != ?", config.Type, true, id).Update("is_default", false)
	}

	updateData.Type = "mcp"
	if err := ac.DB.Model(&config).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật cấu hình MCP thất bại"})
		return
	}
	ac.notifySystemConfigChanged()
	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) DeleteMCPConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cấu hình MCP không tồn tại"})
		return
	}

	if err := ac.DB.Delete(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa cấu hình MCP thất bại"})
		return
	}
	ac.notifySystemConfigChanged()
	c.JSON(http.StatusOK, gin.H{"message": "Xóa cấu hình MCP thành công"})
}

// GenerateAgentMCPEndpoint Hàm chung tạo điểm truy cập MCP
func GenerateAgentMCPEndpoint(db *gorm.DB, agentID string, userID uint, endpointAuthToken string) (string, error) {
	// Lấy WebSocket URL mạng ngoài trong cấu hình OTA
	var otaConfig models.Config
	if err := db.Where("type = ? AND is_default = ?", "ota", true).First(&otaConfig).Error; err != nil {
		return "", fmt.Errorf("failed to get OTA config: %v", err)
	}

	var otaData map[string]interface{}
	if err := json.Unmarshal([]byte(otaConfig.JsonData), &otaData); err != nil {
		return "", fmt.Errorf("failed to parse OTA config: %v", err)
	}

	// Lấy WebSocket URL mạng ngoài
	externalURL, ok := otaData["external"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("external config not found in OTA config")
	}

	websocketConfig, ok := externalURL["websocket"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("websocket config not found in external config")
	}

	wsURL, ok := websocketConfig["url"].(string)
	if !ok || wsURL == "" {
		return "", fmt.Errorf("websocket URL not found in external config")
	}

	// Phân tích OTA URL, chỉ lấy phần domain, giữ nguyên giao thức ws hoặc wss
	parsedURL, err := url.Parse(wsURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse WebSocket URL: %v", err)
	}

	// Xây dựng URL cơ bản (chỉ bao gồm giao thức và domain)
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	// Tạo MCP JWT token
	token, err := generateMCPToken(agentID, userID, endpointAuthToken)
	if err != nil {
		return "", fmt.Errorf("failed to generate MCP token: %v", err)
	}

	// Xây dựng endpoint URL đầy đủ kèm token, sử dụng trực tiếp đường dẫn /mcp
	endpointWithToken := fmt.Sprintf("%s/mcp?token=%s", baseURL, token)

	return endpointWithToken, nil
}

// GenerateAgentOpenClawEndpoint Hàm chung tạo điểm truy cập OpenClaw
func GenerateAgentOpenClawEndpoint(db *gorm.DB, agentID string, userID uint, endpointAuthToken string) (string, error) {
	var otaConfig models.Config
	if err := db.Where("type = ? AND is_default = ?", "ota", true).First(&otaConfig).Error; err != nil {
		return "", fmt.Errorf("failed to get OTA config: %v", err)
	}

	var otaData map[string]interface{}
	if err := json.Unmarshal([]byte(otaConfig.JsonData), &otaData); err != nil {
		return "", fmt.Errorf("failed to parse OTA config: %v", err)
	}

	externalURL, ok := otaData["external"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("external config not found in OTA config")
	}

	websocketConfig, ok := externalURL["websocket"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("websocket config not found in external config")
	}

	wsURL, ok := websocketConfig["url"].(string)
	if !ok || wsURL == "" {
		return "", fmt.Errorf("websocket URL not found in external config")
	}

	parsedURL, err := url.Parse(wsURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse WebSocket URL: %v", err)
	}

	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	token, err := generateOpenClawToken(agentID, userID, endpointAuthToken)
	if err != nil {
		return "", fmt.Errorf("failed to generate OpenClaw token: %v", err)
	}

	endpointWithToken := fmt.Sprintf("%s/ws/openclaw?token=%s", baseURL, token)
	return endpointWithToken, nil
}

// Quản lý cấu hình Memory
func (ac *AdminController) GetMemoryConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "memory").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy danh sách cấu hình Memory thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateMemoryConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Thiết lập loại cấu hình là memory
	config.Type = "memory"

	// Xác thực trường provider
	if config.Provider != "memobase" && config.Provider != "mem0" && config.Provider != "memos" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider phải là memobase, mem0 hoặc memos"})
		return
	}

	// Nếu đặt làm cấu hình mặc định, hủy các cấu hình mặc định cùng loại khác trước
	if config.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)
	}

	if err := ac.DB.Create(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo cấu hình Memory thất bại"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": config})
}

func (ac *AdminController) UpdateMemoryConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.Where("id = ? AND type = ?", id, "memory").First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cấu hình Memory không tồn tại"})
		return
	}

	var updateData models.Config
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Xác thực trường provider
	if updateData.Provider != "memobase" && updateData.Provider != "mem0" && updateData.Provider != "memos" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider phải là memobase, mem0 hoặc memos"})
		return
	}

	// Nếu đặt làm cấu hình mặc định, hủy các cấu hình mặc định cùng loại khác trước
	if updateData.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ? AND id != ?", config.Type, true, id).Update("is_default", false)
	}

	// Cập nhật cấu hình
	config.Name = updateData.Name
	config.Provider = updateData.Provider
	config.JsonData = updateData.JsonData
	config.Enabled = updateData.Enabled
	config.IsDefault = updateData.IsDefault

	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật cấu hình Memory thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) DeleteMemoryConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Where("id = ? AND type = ?", id, "memory").Delete(&models.Config{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa cấu hình Memory thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Xóa thành công"})
}

// Đặt cấu hình Memory mặc định
func (ac *AdminController) SetDefaultMemoryConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.Where("id = ? AND type = ?", id, "memory").First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cấu hình Memory không tồn tại"})
		return
	}

	// Hủy cấu hình mặc định cùng loại khác trước
	ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)

	// Đặt cấu hình hiện tại làm mặc định
	config.IsDefault = true
	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Đặt cấu hình Memory mặc định thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Đặt cấu hình Memory mặc định thành công", "data": config})
}

// generateMCPToken Tạo MCP JWT Token ổn định (giữ nguyên với cùng agentID+userID)
func generateMCPToken(agentID string, userID uint, endpointAuthToken string) (string, error) {
	// Tạo JWT Claims tùy chỉnh
	type MCPClaims struct {
		UserID     uint   `json:"userId"`
		AgentID    string `json:"agentId"`
		EndpointID string `json:"endpointId"`
		Purpose    string `json:"purpose"`
		jwt.RegisteredClaims
	}

	// Xây dựng endpointId
	endpointID := fmt.Sprintf("agent_%s", agentID)

	// Tạo JWT claims.
	// Không thiết lập iat/exp, đảm bảo token có hiệu lực lâu dài và kết quả tạo ra ổn định nhất quán với cùng agentID+userID.
	claims := MCPClaims{
		UserID:           userID,
		AgentID:          agentID,
		EndpointID:       endpointID,
		Purpose:          "mcp-endpoint",
		RegisteredClaims: jwt.RegisteredClaims{},
	}

	// Sử dụng thuật toán HS256 để tạo JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sử dụng cùng khóa bí mật với middleware
	jwtSecret := []byte(strings.TrimSpace(endpointAuthToken))
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// generateOpenClawToken Tạo OpenClaw JWT Token ổn định (giữ nguyên với cùng agentID+userID)
func generateOpenClawToken(agentID string, userID uint, endpointAuthToken string) (string, error) {
	type OpenClawClaims struct {
		UserID     uint   `json:"user_id"`
		AgentID    string `json:"agent_id"`
		EndpointID string `json:"endpoint_id"`
		Purpose    string `json:"purpose"`
		jwt.RegisteredClaims
	}

	endpointID := fmt.Sprintf("agent_%s", agentID)
	claims := OpenClawClaims{
		UserID:           userID,
		AgentID:          agentID,
		EndpointID:       endpointID,
		Purpose:          "openclaw-endpoint",
		RegisteredClaims: jwt.RegisteredClaims{},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtSecret := []byte(strings.TrimSpace(endpointAuthToken))
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ==================== API quản lý role mới ====================

// GetGlobalRolesNew Lấy danh sách role toàn cục (chỉ role toàn cục trong bảng roles)
func (ac *AdminController) GetGlobalRolesNew(c *gin.Context) {
	var globalRoles []models.Role
	if err := ac.DB.Where("user_id IS NULL AND role_type = ?", "global").
		Order("sort_order ASC, id ASC").
		Find(&globalRoles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy role toàn cục thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": globalRoles})
}

// GetRolesNew Lấy danh sách role (role toàn cục + role người dùng)
// Quản trị viên có thể xem tất cả role, người dùng thường chỉ có thể xem role toàn cục và role của chính mình
func (ac *AdminController) GetRolesNew(c *gin.Context) {
	// Lấy ID người dùng và role từ JWT
	userID, exists := c.Get("user_id")
	userRole, roleExists := c.Get("role")

	// Truy vấn role toàn cục
	var globalRoles []models.Role
	if err := ac.DB.Where("user_id IS NULL AND role_type = ?", "global").
		Order("sort_order ASC, id ASC").
		Find(&globalRoles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy role toàn cục thất bại"})
		return
	}

	// Truy vấn role người dùng
	var userRoles []models.Role
	if roleExists && userRole.(string) == "admin" {
		// Quản trị viên xem tất cả role người dùng
		if err := ac.DB.Where("role_type = ?", "user").
			Order("created_at DESC").
			Find(&userRoles).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy role người dùng thất bại"})
			return
		}
	} else if exists {
		// Người dùng thường chỉ xem role của chính mình
		if err := ac.DB.Where("user_id = ? AND role_type = ?", userID, "user").
			Order("created_at DESC").
			Find(&userRoles).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lấy role người dùng thất bại"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"global_roles": globalRoles,
			"user_roles":   userRoles,
		},
	})
}

// GetRoleNew Lấy chi tiết một role
func (ac *AdminController) GetRoleNew(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var role models.Role

	if err := ac.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role không tồn tại"})
		return
	}
	if strings.Contains(c.FullPath(), "/admin/roles/global/") && role.RoleType != "global" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API này chỉ cho phép thao tác với role toàn cục"})
		return
	}

	// Kiểm tra quyền: role người dùng chỉ có thể xem role của chính mình
	if role.UserID != nil {
		userID, exists := c.Get("user_id")
		userRole, roleExists := c.Get("role")

		if roleExists && userRole.(string) != "admin" {
			if exists && userID != nil {
				uid := userID.(uint)
				if uid != *role.UserID {
					c.JSON(http.StatusForbidden, gin.H{"error": "Không có quyền truy cập role này"})
					return
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": role})
}

func normalizeRoleStatus(status string) string {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return "active"
	}
	return trimmed
}

// CreateRoleNew Tạo role (quản trị viên tạo role toàn cục, người dùng tạo role của chính mình)
func (ac *AdminController) CreateRoleNew(c *gin.Context) {
	userID, exists := c.Get("user_id")
	userRole, roleExists := c.Get("role")

	var role models.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Thiết lập loại role và người dùng sở hữu
	if roleExists && userRole.(string) == "admin" {
		// Quản trị viên tạo role toàn cục
		role.RoleType = "global"
		role.UserID = nil
	} else if exists {
		// Người dùng thường tạo role của chính mình
		role.RoleType = "user"
		uid := userID.(uint)
		role.UserID = &uid
		// Role người dùng không thể đặt làm mặc định
		role.IsDefault = false
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Chưa được ủy quyền"})
		return
	}

	// Xác thực các trường bắt buộc
	if role.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tên role không được để trống"})
		return
	}
	if role.Prompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "System prompt không được để trống"})
		return
	}

	role.Status = normalizeRoleStatus(role.Status)
	if role.Status != "active" && role.Status != "inactive" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trạng thái role không hợp lệ"})
		return
	}

	// Nếu đặt làm role mặc định, hủy các role mặc định khác trước
	if role.IsDefault && role.RoleType == "global" {
		ac.DB.Model(&models.Role{}).
			Where("role_type = ? AND is_default = ?", "global", true).
			Update("is_default", false)
	}

	if err := ac.DB.Create(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo role thất bại"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": role})
}

// UpdateRoleNew Cập nhật role
func (ac *AdminController) UpdateRoleNew(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var role models.Role

	if err := ac.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role không tồn tại"})
		return
	}
	if strings.Contains(c.FullPath(), "/admin/roles/global/") && role.RoleType != "global" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API này chỉ cho phép thao tác với role toàn cục"})
		return
	}

	// Kiểm tra quyền
	userID, exists := c.Get("user_id")
	userRole, roleExists := c.Get("role")

	isAdmin := roleExists && userRole.(string) == "admin"
	isOwner := false
	if exists && role.UserID != nil {
		if uid, ok := userID.(uint); ok {
			isOwner = uid == *role.UserID
		}
	}

	if !isAdmin && !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "Không có quyền sửa role này"})
		return
	}

	var updateData models.Role
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Nếu đặt làm role mặc định, hủy các role mặc định khác trước
	if updateData.IsDefault && role.RoleType == "global" {
		ac.DB.Model(&models.Role{}).
			Where("role_type = ? AND is_default = ? AND id != ?", "global", true, id).
			Update("is_default", false)
	}

	// Cập nhật trường
	role.Name = updateData.Name
	role.Description = updateData.Description
	role.Prompt = updateData.Prompt
	role.LLMConfigID = updateData.LLMConfigID
	role.TTSConfigID = updateData.TTSConfigID
	role.Voice = updateData.Voice
	role.SortOrder = updateData.SortOrder

	normalizedStatus := strings.TrimSpace(updateData.Status)
	if normalizedStatus == "" {
		normalizedStatus = role.Status
	}
	normalizedStatus = normalizeRoleStatus(normalizedStatus)
	if normalizedStatus != "active" && normalizedStatus != "inactive" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trạng thái role không hợp lệ"})
		return
	}
	role.Status = normalizedStatus

	// Chỉ quản trị viên mới có thể sửa cờ mặc định và loại role
	if isAdmin {
		role.IsDefault = updateData.IsDefault
	}

	if err := ac.DB.Save(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật role thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": role})
}

// DeleteRoleNew Xóa role
func (ac *AdminController) DeleteRoleNew(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var role models.Role

	if err := ac.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role không tồn tại"})
		return
	}
	if strings.Contains(c.FullPath(), "/admin/roles/global/") && role.RoleType != "global" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API này chỉ cho phép thao tác với role toàn cục"})
		return
	}

	// Kiểm tra quyền
	userID, exists := c.Get("user_id")
	userRole, roleExists := c.Get("role")

	isAdmin := roleExists && userRole.(string) == "admin"
	isOwner := false
	if exists && role.UserID != nil {
		if uid, ok := userID.(uint); ok {
			isOwner = uid == *role.UserID
		}
	}

	if !isAdmin && !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "Không có quyền xóa role này"})
		return
	}

	// Kiểm tra có thiết bị nào đang sử dụng role này không
	var deviceCount int64
	ac.DB.Model(&models.Device{}).Where("role_id = ?", id).Count(&deviceCount)
	if deviceCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Có %d thiết bị đang sử dụng role này, vui lòng hủy liên kết trước", deviceCount),
		})
		return
	}

	if err := ac.DB.Delete(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa role thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Xóa thành công"})
}

// ToggleRoleStatus Chuyển đổi trạng thái role (bật/tắt)
func (ac *AdminController) ToggleRoleStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var role models.Role

	if err := ac.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role không tồn tại"})
		return
	}
	if strings.Contains(c.FullPath(), "/admin/roles/global/") && role.RoleType != "global" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API này chỉ cho phép thao tác với role toàn cục"})
		return
	}

	// Kiểm tra quyền
	userID, exists := c.Get("user_id")
	userRole, roleExists := c.Get("role")

	isAdmin := roleExists && userRole.(string) == "admin"
	isOwner := false
	if exists && role.UserID != nil {
		if uid, ok := userID.(uint); ok {
			isOwner = uid == *role.UserID
		}
	}

	if !isAdmin && !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "Không có quyền sửa role này"})
		return
	}

	// Chuyển đổi trạng thái
	currentStatus := normalizeRoleStatus(role.Status)
	if currentStatus == "active" {
		role.Status = "inactive"
	} else {
		role.Status = "active"
	}

	if err := ac.DB.Save(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật trạng thái thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": role})
}

// SetDefaultRole Đặt role mặc định (chỉ role toàn cục)
func (ac *AdminController) SetDefaultRole(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var role models.Role

	if err := ac.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role không tồn tại"})
		return
	}

	// Chỉ role toàn cục mới có thể đặt làm mặc định
	if role.RoleType != "global" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chỉ role toàn cục mới có thể đặt làm mặc định"})
		return
	}

	// Kiểm tra quyền: chỉ quản trị viên mới có thể đặt role mặc định
	userRole, roleExists := c.Get("role")
	if !roleExists || userRole.(string) != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Chỉ quản trị viên mới có thể đặt role mặc định"})
		return
	}

	// Hủy các role mặc định khác trước
	ac.DB.Model(&models.Role{}).
		Where("role_type = ? AND is_default = ?", "global", true).
		Update("is_default", false)

	// Đặt role hiện tại làm mặc định
	role.IsDefault = true
	if err := ac.DB.Save(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Đặt role mặc định thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": role, "message": "Đã đặt làm role mặc định"})
}

type applyDeviceRoleRequest struct {
	RoleID *uint `json:"role_id"`
}

type switchDeviceRoleByNameRequest struct {
	RoleName string `json:"role_name"`
}

func normalizeRoleNameForMatch(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, " ", "")
	return normalized
}

func calcRoleMatchScore(requestedRoleName string, candidateRoleName string) (int, string) {
	reqCompact := normalizeRoleNameForMatch(requestedRoleName)
	candCompact := normalizeRoleNameForMatch(candidateRoleName)
	if reqCompact == "" || candCompact == "" {
		return -1, ""
	}

	if reqCompact == candCompact {
		return 1000, "exact"
	}

	if strings.Contains(candCompact, reqCompact) || strings.Contains(reqCompact, candCompact) {
		score := 700 - absInt(len(candCompact)-len(reqCompact))
		if strings.HasPrefix(candCompact, reqCompact) || strings.HasPrefix(reqCompact, candCompact) {
			score += 50
		}
		return score, "fuzzy"
	}

	reqRaw := strings.ToLower(strings.TrimSpace(requestedRoleName))
	candRaw := strings.ToLower(strings.TrimSpace(candidateRoleName))
	if reqRaw != "" && candRaw != "" && (strings.Contains(candRaw, reqRaw) || strings.Contains(reqRaw, candRaw)) {
		score := 600 - absInt(len(candRaw)-len(reqRaw))
		return score, "fuzzy"
	}

	return -1, ""
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func matchDeviceRoleByName(requestedRoleName string, roles []models.Role) (*models.Role, string) {
	bestScore := -1
	bestMatchType := ""
	var bestRole *models.Role

	for i := range roles {
		role := &roles[i]
		if normalizeRoleStatus(role.Status) != "active" {
			continue
		}

		score, matchType := calcRoleMatchScore(requestedRoleName, role.Name)
		if score > bestScore {
			bestScore = score
			bestMatchType = matchType
			bestRole = role
		}
	}

	if bestScore < 0 {
		return nil, ""
	}
	return bestRole, bestMatchType
}

func getRequestUserInfo(c *gin.Context) (uint, bool, bool) {
	var uid uint
	userID, hasUserID := c.Get("user_id")
	if hasUserID {
		if v, ok := userID.(uint); ok {
			uid = v
		}
	}

	roleVal, hasRole := c.Get("role")
	isAdmin := hasRole && roleVal == "admin"
	return uid, hasUserID, isAdmin
}

// ApplyRoleToDevice Áp dụng role cho thiết bị (người dùng thường có thể thao tác với thiết bị của mình)
func (ac *AdminController) ApplyRoleToDevice(c *gin.Context) {
	deviceID, err := strconv.Atoi(c.Param("id"))
	if err != nil || deviceID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID thiết bị không hợp lệ"})
		return
	}

	var req applyDeviceRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var device models.Device
	if err := ac.DB.First(&device, deviceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thiết bị không tồn tại"})
		return
	}

	uid, hasUserID, isAdmin := getRequestUserInfo(c)
	if !isAdmin {
		if !hasUserID || device.UserID != uid {
			c.JSON(http.StatusForbidden, gin.H{"error": "Không có quyền thao tác với thiết bị này"})
			return
		}
	}

	if req.RoleID != nil {
		var role models.Role
		if err := ac.DB.First(&role, *req.RoleID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Role không tồn tại"})
			return
		}

		roleStatus := normalizeRoleStatus(role.Status)
		if roleStatus != "active" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Role chưa được bật"})
			return
		}
		if role.Status == "" {
			if err := ac.DB.Model(&role).Update("status", roleStatus).Error; err != nil {
				log.Printf("Cập nhật trạng thái mặc định của role thất bại: role_id=%d err=%v", role.ID, err)
			}
		}

		// Người dùng thường chỉ được phép sử dụng role toàn cục hoặc role người dùng của chính mình
		if !isAdmin {
			if role.RoleType != "global" {
				if role.UserID == nil || *role.UserID != uid {
					c.JSON(http.StatusForbidden, gin.H{"error": "Không có quyền sử dụng role này"})
					return
				}
			}
		}
	}

	device.RoleID = req.RoleID
	if err := updateDeviceColumns(ac.DB, device.ID, map[string]interface{}{
		"role_id": device.RoleID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Áp dụng role thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"device_id": device.ID,
			"role_id":   device.RoleID,
		},
	})
}

// SwitchDeviceRoleByNameInternal API nội bộ: chuyển đổi role thiết bị theo tên role (khớp mờ)
func (ac *AdminController) SwitchDeviceRoleByNameInternal(c *gin.Context) {
	deviceName := strings.TrimSpace(c.Param("device_name"))
	if deviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tên thiết bị không được để trống"})
		return
	}

	var req switchDeviceRoleByNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.RoleName = strings.TrimSpace(req.RoleName)
	if req.RoleName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role_name không được để trống"})
		return
	}

	var device models.Device
	if err := ac.DB.Where("device_name = ?", deviceName).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thiết bị không tồn tại"})
		return
	}

	var roles []models.Role
	if err := ac.DB.
		Where("(role_type = ? OR (role_type = ? AND user_id = ?))", "global", "user", device.UserID).
		Order("sort_order ASC, id ASC").
		Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Truy vấn role thất bại"})
		return
	}

	matchedRole, matchType := matchDeviceRoleByName(req.RoleName, roles)
	if matchedRole == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":               "Không tìm thấy role khớp",
			"requested_role_name": req.RoleName,
		})
		return
	}

	roleID := matchedRole.ID
	device.RoleID = &roleID
	if err := updateDeviceColumns(ac.DB, device.ID, map[string]interface{}{
		"role_id": device.RoleID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Chuyển đổi role thiết bị thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"device_id":           device.ID,
			"device_name":         device.DeviceName,
			"role_id":             device.RoleID,
			"role_name":           matchedRole.Name,
			"role_type":           matchedRole.RoleType,
			"requested_role_name": req.RoleName,
			"match_type":          matchType,
		},
	})
}

// RestoreDeviceDefaultRoleInternal API nội bộ: khôi phục role mặc định của thiết bị (xóa role liên kết của thiết bị)
func (ac *AdminController) RestoreDeviceDefaultRoleInternal(c *gin.Context) {
	deviceName := strings.TrimSpace(c.Param("device_name"))
	if deviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tên thiết bị không được để trống"})
		return
	}

	var device models.Device
	if err := ac.DB.Where("device_name = ?", deviceName).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thiết bị không tồn tại"})
		return
	}

	device.RoleID = nil
	if err := updateDeviceColumns(ac.DB, device.ID, map[string]interface{}{
		"role_id": nil,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Khôi phục role mặc định thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"device_id":   device.ID,
			"device_name": device.DeviceName,
			"role_id":     device.RoleID,
		},
	})
}