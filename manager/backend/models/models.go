package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// User model người dùng
type User struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Username  string    `json:"username" gorm:"type:varchar(50);uniqueIndex:idx_users_username;not null"`
	Password  string    `json:"-" gorm:"type:varchar(255);not null"`
	Email     string    `json:"email" gorm:"type:varchar(100);uniqueIndex:idx_users_email"`
	Role      string    `json:"role" gorm:"type:varchar(20);not null;default:'user'"` // admin, user
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIToken token truy cập OpenAPI ra bên ngoài (chỉ lưu bản băm (hash), không lưu văn bản gốc)
type APIToken struct {
	ID          uint       `json:"id" gorm:"primarykey"`
	UserID      uint       `json:"user_id" gorm:"not null;index"`
	Name        string     `json:"name" gorm:"type:varchar(100);not null"`
	TokenPrefix string     `json:"token_prefix" gorm:"type:varchar(20);index"`
	TokenHash   string     `json:"-" gorm:"type:char(64);uniqueIndex;not null"`
	IsActive    bool       `json:"is_active" gorm:"default:true;index"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at" gorm:"index"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Device model thiết bị
type Device struct {
	ID           uint       `json:"id" gorm:"primarykey"`
	UserID       uint       `json:"user_id" gorm:"not null"`
	AgentID      uint       `json:"agent_id" gorm:"not null;default:0"`                                       // ID agent (smart agent), một thiết bị chỉ thuộc về một agent
	RoleID       *uint      `json:"role_id" gorm:"index"`                                                     // ID vai trò (role, tùy chọn, ghi đè cấu hình của agent)
	NickName     string     `json:"nick_name" gorm:"type:varchar(100)"`                                       // Biệt danh thiết bị, người dùng có thể chỉnh sửa
	DeviceCode   string     `json:"device_code" gorm:"type:varchar(100);uniqueIndex:idx_devices_device_code"` // Mã kích hoạt gồm 6 ký tự
	DeviceName   string     `json:"device_name" gorm:"type:varchar(100)"`                                     // Định danh thiết bị/Device-ID, dùng khi thiết bị báo cáo lên server
	Challenge    string     `json:"challenge" gorm:"type:varchar(128)"`                                       // Mã thử thách (challenge) kích hoạt
	PreSecretKey string     `json:"pre_secret_key" gorm:"type:varchar(128)"`                                  // Khóa bí mật tiền kích hoạt
	Activated    bool       `json:"activated" gorm:"default:false"`                                           // Thiết bị đã được kích hoạt hay chưa
	LastActiveAt *time.Time `json:"last_active_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Agent model agent (smart agent/trợ lý ảo)
type Agent struct {
	ID              uint    `json:"id" gorm:"primarykey"`
	UserID          uint    `json:"user_id" gorm:"not null"`
	Name            string  `json:"name" gorm:"type:varchar(100);not null"`                  // Tên gọi, dùng để nhận diện phía quản trị (admin)
	Nickname        string  `json:"nickname" gorm:"type:varchar(100)"`                       // Biệt danh, dùng cho LLM/Prompt
	CustomPrompt    string  `json:"custom_prompt" gorm:"type:text"`                          // Phần giới thiệu vai trò (prompt)
	LLMConfigID     *string `json:"llm_config_id" gorm:"type:varchar(100)"`                  // ID cấu hình mô hình ngôn ngữ (LLM)
	TTSConfigID     *string `json:"tts_config_id" gorm:"type:varchar(100)"`                  // ID cấu hình âm sắc (TTS)
	Voice           *string `json:"voice" gorm:"type:varchar(200)"`                          // Giá trị âm sắc (voice)
	ASRSpeed        string  `json:"asr_speed" gorm:"type:varchar(20);default:'normal'"`      // Tốc độ nhận diện giọng nói: normal/patient/fast
	MemoryMode      string  `json:"memory_mode" gorm:"type:varchar(20);default:'short'"`     // Chế độ bộ nhớ (memory): none/short/long
	SpeakerChatMode string  `json:"speaker_chat_mode" gorm:"type:varchar(32);default:'off'"` // Chế độ trò chuyện theo vân giọng nói: off/identified_only
	MCPServiceNames string  `json:"mcp_service_names" gorm:"type:text"`                      // Danh sách tên dịch vụ MCP phân tách bằng dấu phẩy, rỗng = sử dụng toàn bộ dịch vụ MCP toàn cục đã bật
	// Cấu hình OpenClaw, dạng chuỗi JSON, cấu trúc:
	// {"allowed":true,"enter_keywords":["进入openclaw"],"exit_keywords":["退出openclaw"]}
	OpenClawConfig string    `json:"openclaw_config" gorm:"type:text"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// KnowledgeBase kho tri thức (knowledge base) của người dùng (độc lập theo từng người dùng)
type KnowledgeBase struct {
	ID                 uint       `json:"id" gorm:"primarykey"`
	UserID             uint       `json:"user_id" gorm:"not null;index"`
	Name               string     `json:"name" gorm:"type:varchar(100);not null"`
	Description        string     `json:"description" gorm:"type:text"`
	Content            string     `json:"content" gorm:"type:text"`
	RetrievalThreshold *float64   `json:"retrieval_threshold" gorm:"type:double"`         // Ngưỡng truy hồi (retrieval threshold) — để trống nghĩa là kế thừa cấu hình toàn cục
	ExternalKBID       string     `json:"external_kb_id" gorm:"type:varchar(255);index"`  // ID kho tri thức bên ngoài (Dify dataset_id)
	ExternalDocID      string     `json:"external_doc_id" gorm:"type:varchar(255);index"` // ID tài liệu bên ngoài (Dify document_id)
	AutoDataset        bool       `json:"auto_dataset" gorm:"default:false"`              // Có phải do hệ thống tự động tạo dataset hay không
	SyncProvider       string     `json:"sync_provider" gorm:"type:varchar(50);index"`    // Nhà cung cấp đồng bộ (sync provider) — hiện tại là dify
	SyncStatus         string     `json:"sync_status" gorm:"type:varchar(20);default:'pending';index"`
	SyncError          string     `json:"sync_error" gorm:"type:text"`
	LastSyncedAt       *time.Time `json:"last_synced_at"`
	Status             string     `json:"status" gorm:"type:varchar(20);default:'active';index"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// KnowledgeBaseDocument tài liệu trong kho tri thức (một kho tri thức có thể chứa nhiều tài liệu)
type KnowledgeBaseDocument struct {
	ID              uint       `json:"id" gorm:"primarykey"`
	KnowledgeBaseID uint       `json:"knowledge_base_id" gorm:"not null;index"`
	Name            string     `json:"name" gorm:"type:varchar(200);not null"`
	Content         string     `json:"content" gorm:"type:text"`
	ExternalDocID   string     `json:"external_doc_id" gorm:"type:varchar(255);index"` // Dify document_id
	SyncStatus      string     `json:"sync_status" gorm:"type:varchar(20);default:'pending';index"`
	SyncError       string     `json:"sync_error" gorm:"type:text"`
	LastSyncedAt    *time.Time `json:"last_synced_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// AgentKnowledgeBase quan hệ nhiều-nhiều giữa agent và kho tri thức
type AgentKnowledgeBase struct {
	ID              uint      `json:"id" gorm:"primarykey"`
	AgentID         uint      `json:"agent_id" gorm:"not null;index;uniqueIndex:idx_agent_kb_unique,priority:1"`
	KnowledgeBaseID uint      `json:"knowledge_base_id" gorm:"not null;index;uniqueIndex:idx_agent_kb_unique,priority:2"`
	CreatedAt       time.Time `json:"created_at"`
}

// Config model cấu hình dùng chung
type Config struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Type      string    `json:"type" gorm:"type:varchar(50);not null;uniqueIndex:type_config_id,priority:1"` // vad, asr, llm, tts, ota, mqtt, udp, mqtt_server, vision
	Name      string    `json:"name" gorm:"type:varchar(100);not null"`
	ConfigID  string    `json:"config_id" gorm:"type:varchar(100);not null;uniqueIndex:type_config_id,priority:2"` // ID cấu hình, dùng để liên kết
	Provider  string    `json:"provider" gorm:"type:varchar(50)"`                                                  // Một số loại cấu hình cần trường provider
	JsonData  string    `json:"json_data" gorm:"type:text"`                                                        // Dữ liệu cấu hình dạng JSON
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	IsDefault bool      `json:"is_default" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MCPMarketService cấu hình dịch vụ MCP được nhập từ chợ ứng dụng (marketplace)
// Cấu hình thủ công vẫn được lưu trong Config(type=mcp).json_data, cấu hình từ marketplace được tách ra bảng riêng.
type MCPMarketService struct {
	ID               uint   `json:"id" gorm:"primarykey"`
	Name             string `json:"name" gorm:"type:varchar(150);not null"`
	Enabled          bool   `json:"enabled" gorm:"default:true;index"`
	Transport        string `json:"transport" gorm:"type:varchar(32);not null"` // sse / streamablehttp
	URL              string `json:"url" gorm:"type:text;not null"`
	URLHash          string `json:"url_hash" gorm:"type:char(64);not null;uniqueIndex:idx_mcp_market_services_url_hash"` // sha256(url) dạng hex
	HeadersJSON      string `json:"headers_json" gorm:"type:text"`
	AllowedToolsJSON string `json:"allowed_tools_json" gorm:"type:text"`

	MarketID    *uint  `json:"market_id" gorm:"index"` // Liên kết tới configs(type=mcp_market).id
	ProviderID  string `json:"provider_id" gorm:"type:varchar(50);index"`
	ServiceID   string `json:"service_id" gorm:"type:varchar(255);index"`
	ServiceName string `json:"service_name" gorm:"type:varchar(255)"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Role model vai trò (quản lý tập trung cả vai trò toàn cục và vai trò của người dùng)
type Role struct {
	ID          uint   `json:"id" gorm:"primarykey"`
	UserID      *uint  `json:"user_id" gorm:"index"` // ID người dùng sở hữu, NULL nghĩa là vai trò toàn cục
	Name        string `json:"name" gorm:"type:varchar(100);not null"`
	Description string `json:"description" gorm:"type:text"`
	Prompt      string `json:"prompt" gorm:"type:text"` // Prompt hệ thống (system prompt)

	// Cấu hình LLM/TTS (giữ nhất quán với các trường của Agent)
	LLMConfigID *string `json:"llm_config_id" gorm:"type:varchar(100)"` // ID cấu hình LLM

	TTSConfigID *string `json:"tts_config_id" gorm:"type:varchar(100)"` // ID cấu hình TTS
	Voice       *string `json:"voice" gorm:"type:varchar(200)"`         // Giá trị âm sắc (voice)

	// Loại vai trò và trạng thái
	RoleType string `json:"role_type" gorm:"type:varchar(20);default:'user';index"` // global/system/user
	Status   string `json:"status" gorm:"type:varchar(20);default:'active';index"`  // active/inactive

	// Thứ tự sắp xếp và mặc định
	SortOrder int  `json:"sort_order" gorm:"default:0"`           // Thứ tự hiển thị
	IsDefault bool `json:"is_default" gorm:"default:false;index"` // Có phải vai trò mặc định hay không (chỉ áp dụng cho vai trò toàn cục)

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName chỉ định tên bảng
func (Role) TableName() string {
	return "roles"
}

// GlobalRole model vai trò toàn cục (giữ lại để tương thích ngược, sau này có thể chuyển sang dùng Role)
type GlobalRole struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	Name        string    `json:"name" gorm:"type:varchar(100);not null"`
	Description string    `json:"description" gorm:"type:text"`
	Prompt      string    `json:"prompt" gorm:"type:text"`
	IsDefault   bool      `json:"is_default" gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SpeakerGroup model nhóm vân giọng nói (speaker group)
type SpeakerGroup struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	UserID      uint      `json:"user_id" gorm:"not null;index;uniqueIndex:idx_speaker_groups_user_name,priority:1"`
	AgentID     uint      `json:"agent_id" gorm:"not null;index"`
	Name        string    `json:"name" gorm:"type:varchar(100);not null;uniqueIndex:idx_speaker_groups_user_name,priority:2"`
	Prompt      string    `json:"prompt" gorm:"type:text"`
	Description string    `json:"description" gorm:"type:text"`
	TTSConfigID *string   `json:"tts_config_id" gorm:"type:varchar(100)"` // ID cấu hình TTS
	Voice       *string   `json:"voice" gorm:"type:varchar(200)"`         // Giá trị âm sắc (voice)
	Status      string    `json:"status" gorm:"type:varchar(20);default:'active'"`
	SampleCount int       `json:"sample_count" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SpeakerSample model mẫu vân giọng nói (speaker sample)
type SpeakerSample struct {
	ID             uint      `json:"id" gorm:"primarykey"`
	SpeakerGroupID uint      `json:"speaker_group_id" gorm:"not null;index"`
	UserID         uint      `json:"user_id" gorm:"not null;index"`
	UUID           string    `json:"uuid" gorm:"type:varchar(36);not null;uniqueIndex"`
	FilePath       string    `json:"file_path" gorm:"type:varchar(500);not null"`
	FileName       string    `json:"file_name" gorm:"type:varchar(255)"`
	FileSize       int64     `json:"file_size"`
	Duration       float32   `json:"duration"`
	Status         string    `json:"status" gorm:"type:varchar(20);default:'active'"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// VoiceClone model âm sắc nhân bản (voice clone)
type VoiceClone struct {
	ID                 uint      `json:"id" gorm:"primarykey"`
	UserID             uint      `json:"user_id" gorm:"not null;index"`
	Name               string    `json:"name" gorm:"type:varchar(100);not null"`
	Provider           string    `json:"provider" gorm:"type:varchar(50);not null;index"`
	ProviderVoiceID    string    `json:"provider_voice_id" gorm:"type:varchar(200);not null;index"`
	TTSConfigID        string    `json:"tts_config_id" gorm:"type:varchar(100);not null;index"`
	SharedToAll        bool      `json:"shared_to_all" gorm:"default:false;index"`
	Status             string    `json:"status" gorm:"type:varchar(20);default:'active';index"`
	TranscriptRequired bool      `json:"transcript_required" gorm:"default:false"`
	MetaJSON           string    `json:"meta_json" gorm:"type:json"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// VoiceCloneAudio model tài sản âm thanh gốc dùng để nhân bản (giữ lại dữ liệu đã upload/ghi âm)
type VoiceCloneAudio struct {
	ID             uint      `json:"id" gorm:"primarykey"`
	VoiceCloneID   *uint     `json:"voice_clone_id" gorm:"index"`
	UserID         uint      `json:"user_id" gorm:"not null;index"`
	SourceType     string    `json:"source_type" gorm:"type:varchar(20);not null"` // upload/record
	FilePath       string    `json:"file_path" gorm:"type:varchar(500);not null"`
	FileName       string    `json:"file_name" gorm:"type:varchar(255)"`
	FileSize       int64     `json:"file_size"`
	ContentType    string    `json:"content_type" gorm:"type:varchar(100)"`
	Transcript     string    `json:"transcript" gorm:"type:text"`
	TranscriptLang string    `json:"transcript_lang" gorm:"type:varchar(20)"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// VoiceCloneTask model tác vụ nhân bản giọng nói dạng bất đồng bộ (async)
type VoiceCloneTask struct {
	ID           uint       `json:"id" gorm:"primarykey"`
	TaskID       string     `json:"task_id" gorm:"type:varchar(64);not null;uniqueIndex"`
	UserID       uint       `json:"user_id" gorm:"not null;index"`
	VoiceCloneID uint       `json:"voice_clone_id" gorm:"not null;index"`
	Provider     string     `json:"provider" gorm:"type:varchar(50);not null;index"`
	Status       string     `json:"status" gorm:"type:varchar(20);not null;default:'queued';index"` // queued/processing/succeeded/failed
	Attempts     int        `json:"attempts" gorm:"not null;default:0"`
	LastError    string     `json:"last_error" gorm:"type:text"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	MetaJSON     string     `json:"meta_json" gorm:"type:json"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// UserVoiceCloneQuota hạn mức nhân bản giọng nói của người dùng (tính theo tts_config_id)
type UserVoiceCloneQuota struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	UserID      uint      `json:"user_id" gorm:"not null;index;uniqueIndex:idx_user_tts_quota,priority:1"`
	TTSConfigID string    `json:"tts_config_id" gorm:"type:varchar(100);not null;index;uniqueIndex:idx_user_tts_quota,priority:2"`
	MaxCount    int       `json:"max_count" gorm:"not null;default:-1"` // -1 nghĩa là không giới hạn, 0 nghĩa là cấm tạo mới
	UsedCount   int       `json:"used_count" gorm:"not null;default:0"` // Mỗi lần gửi tác vụ nhân bản sẽ được tính vào bộ đếm này
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ChatMessage model tin nhắn chat
type ChatMessage struct {
	ID        uint   `json:"id" gorm:"primarykey"`
	MessageID string `json:"message_id" gorm:"type:varchar(64);uniqueIndex:idx_chat_messages_message_id;not null"`

	// Thông tin liên kết (không dùng khóa ngoại - foreign key)
	DeviceID  string `json:"device_id" gorm:"type:varchar(100);index:idx_device_id;not null"`
	AgentID   string `json:"agent_id" gorm:"type:varchar(64);index:idx_agent_id;not null"`
	UserID    uint   `json:"user_id" gorm:"index:idx_user_id;not null"`
	SessionID string `json:"session_id" gorm:"type:varchar(64);index:idx_session_id"` // Chỉ dùng để đánh dấu nhóm (grouping)

	// Nội dung tin nhắn
	Role    string `json:"role" gorm:"type:varchar(20);index;not null;comment:user|assistant|system|tool"`
	Content string `json:"content" gorm:"type:text;not null"`

	// Thông tin gọi công cụ (tool call)
	ToolCallID    string  `json:"tool_call_id,omitempty" gorm:"type:varchar(64);index;comment:ID lượt gọi công cụ (dùng cho vai trò Tool)"`
	ToolCallsJSON *string `json:"tool_calls_json,omitempty" gorm:"type:json;column:tool_calls;comment:Danh sách lượt gọi công cụ dạng JSON (dùng cho vai trò Assistant)"`

	// Thông tin file âm thanh (lưu trên hệ thống file, phân tán theo 2 cấp hash)
	AudioPath     string `json:"audio_path,omitempty" gorm:"type:varchar(512);comment:Đường dẫn tương đối của file âm thanh (phân tán theo 2 cấp hash)"`
	AudioDuration *int   `json:"audio_duration,omitempty" gorm:"comment:Đơn vị mili-giây"`
	AudioSize     *int   `json:"audio_size,omitempty" gorm:"comment:Đơn vị byte"`
	AudioFormat   string `json:"audio_format,omitempty" gorm:"type:varchar(20);default:'wav';comment:Định dạng âm thanh (cố định là wav)"`

	// Metadata
	MetadataJSON string                 `json:"-" gorm:"type:json;column:metadata"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" gorm:"-"`

	// Trạng thái
	IsDeleted bool      `json:"is_deleted" gorm:"default:false;index"`
	CreatedAt time.Time `json:"created_at" gorm:"index:idx_created_at"`
}

// TableName chỉ định tên bảng
func (ChatMessage) TableName() string {
	return "chat_messages"
}

// BeforeSave GORM hook - chuyển đổi (serialize) metadata
func (m *ChatMessage) BeforeSave(tx *gorm.DB) error {
	if m.Metadata != nil {
		data, err := json.Marshal(m.Metadata)
		if err != nil {
			return err
		}
		m.MetadataJSON = string(data)
	}
	return nil
}

// AfterFind GORM hook - chuyển đổi ngược (deserialize) metadata
func (m *ChatMessage) AfterFind(tx *gorm.DB) error {
	if m.MetadataJSON != "" {
		return json.Unmarshal([]byte(m.MetadataJSON), &m.Metadata)
	}
	return nil
}