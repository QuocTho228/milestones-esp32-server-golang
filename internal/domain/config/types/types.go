package types

type AsrConfig struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

type TtsConfig struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

type MemoryConfig struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

type LlmConfig struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

type VadConfig struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

type ConfigItem struct {
	Provider string                 `json:"provider"`
	JsonData map[string]interface{} `json:"json_data"`
}

type SpeakerGroupInfo struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Prompt      string   `json:"prompt"`
	Description string   `json:"description"`
	Uuids       []string `json:"uuids"`
	TTSConfigID *string  `json:"tts_config_id"`
	Voice       *string  `json:"voice"`
	// Khi giọng nói được tạo từ tính năng nhân bản giọng nói, hệ thống sẽ ghi đè mô hình TTS trong quá trình chạy.
	VoiceModelOverride *string `json:"voice_model_override,omitempty"`
}

type KnowledgeBaseRef struct {
	ID                 uint     `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Provider           string   `json:"provider"`
	ExternalKBID       string   `json:"external_kb_id"`
	ExternalDocID      string   `json:"external_doc_id"`
	RetrievalThreshold *float64 `json:"retrieval_threshold"`
	Status             string   `json:"status"`
}

type OpenClawConfig struct {
	Allowed       bool     `json:"allowed"`
	EnterKeywords []string `json:"enter_keywords"`
	ExitKeywords  []string `json:"exit_keywords"`
}

type UConfig struct {
	SystemPrompt    string                      `json:"system_prompt"`
	Asr             AsrConfig                   `json:"asr"`
	Tts             TtsConfig                   `json:"tts"`
	Llm             LlmConfig                   `json:"llm"`
	Vad             VadConfig                   `json:"vad"`
	Memory          MemoryConfig                `json:"memory"`
	VoiceIdentify   map[string]SpeakerGroupInfo `json:"voice_identify"`    // Cấu hình nhận diện giọng nói
	MemoryMode      string                      `json:"memory_mode"`       // Chế độ nhớ: none/short/long
	SpeakerChatMode string                      `json:"speaker_chat_mode"` // Chế độ trò chuyện giọng nói: off/identified_only
	AgentId         string                      `json:"agent_id"`          // ID agent thuộc về
	MCPServiceNames string                      `json:"mcp_service_names"` // Tên dịch vụ MCP được phân tách bằng dấu phẩy, rỗng = sử dụng tất cả các dịch vụ MCP toàn cục đã bật
	OpenClaw        OpenClawConfig              `json:"openclaw"`          // Cấu hình OpenClaw
	KnowledgeBases  []KnowledgeBaseRef          `json:"knowledge_bases"`
}

type TtsConfigItem struct {
	ConfigID  string                 `json:"config_id"`
	Name      string                 `json:"name"`
	Provider  string                 `json:"provider"`
	Config    map[string]interface{} `json:"config"`
	IsDefault bool                   `json:"is_default"`
}

type KnowledgeSearchHit struct {
	Content string  `json:"content"`
	Title   string  `json:"title,omitempty"`
	Score   float64 `json:"score,omitempty"`
}
