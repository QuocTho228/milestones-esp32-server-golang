package aliyun_funasr

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultWsURL          = "wss://dashscope-intl.aliyuncs.com/api-ws/v1/inference/"
	defaultModel          = "fun-asr-realtime"
	defaultFormat         = "pcm"
	defaultSampleRate     = 16000
	defaultTimeoutSeconds = 30
)

// Config: Cấu hình Aliyun FunASR.
type Config struct {
	APIKey                     string
	WsURL                      string
	Model                      string
	Format                     string
	SampleRate                 int
	LanguageHints              []string
	VocabularyID               string
	DisfluencyRemovalEnabled   bool
	SemanticPunctuationEnabled bool
	Timeout                    time.Duration
}

// DefaultConfig: Trả về cấu hình mặc định.
func DefaultConfig() Config {
	return Config{
		WsURL:      defaultWsURL,
		Model:      defaultModel,
		Format:     defaultFormat,
		SampleRate: defaultSampleRate,
		Timeout:    time.Duration(defaultTimeoutSeconds) * time.Second,
	}
}

// ConfigFromMap: Tạo cấu hình bằng cách hợp nhất dữ liệu từ map cấu hình
// (hỗ trợ cả tệp cấu hình và hệ thống quản lý nội bộ).
func ConfigFromMap(cfg map[string]interface{}) Config {
	conf := DefaultConfig()

	// Trước tiên, hợp nhất các giá trị mặc định từ tệp cấu hình.
	applyViperDefaults(&conf)

	// Tương thích với định dạng cũ: nếu truyền vào { aliyun_funasr: { ... } },
	// sẽ ưu tiên sử dụng map bên trong.
	if nested, ok := cfg["aliyun_funasr"].(map[string]interface{}); ok {
		cfg = nested
	}

	applyMapOverrides(&conf, cfg)

	// Nếu api_key để trống, sẽ sử dụng giá trị từ biến môi trường.
	if conf.APIKey == "" {
		conf.APIKey = os.Getenv("DASHSCOPE_API_KEY")
	}

	return conf
}

func applyViperDefaults(conf *Config) {
	const prefix = "asr.aliyun_funasr."
	if viper.IsSet(prefix + "api_key") {
		conf.APIKey = viper.GetString(prefix + "api_key")
	}
	if viper.IsSet(prefix + "ws_url") {
		conf.WsURL = viper.GetString(prefix + "ws_url")
	}
	if viper.IsSet(prefix + "model") {
		conf.Model = viper.GetString(prefix + "model")
	}
	if viper.IsSet(prefix + "format") {
		conf.Format = viper.GetString(prefix + "format")
	}
	if viper.IsSet(prefix + "sample_rate") {
		if sr := viper.GetInt(prefix + "sample_rate"); sr > 0 {
			conf.SampleRate = sr
		}
	}
	if viper.IsSet(prefix + "language_hints") {
		conf.LanguageHints = parseLanguageHints(viper.Get(prefix + "language_hints"))
	} else if viper.IsSet(prefix + "language") {
		conf.LanguageHints = parseLanguageHints(viper.GetString(prefix + "language"))
	}
	if viper.IsSet(prefix + "vocabulary_id") {
		conf.VocabularyID = viper.GetString(prefix + "vocabulary_id")
	}
	if viper.IsSet(prefix + "disfluency_removal_enabled") {
		conf.DisfluencyRemovalEnabled = viper.GetBool(prefix + "disfluency_removal_enabled")
	}
	if viper.IsSet(prefix + "semantic_punctuation_enabled") {
		conf.SemanticPunctuationEnabled = viper.GetBool(prefix + "semantic_punctuation_enabled")
	}
	if viper.IsSet(prefix + "timeout") {
		if t := viper.GetInt(prefix + "timeout"); t > 0 {
			conf.Timeout = time.Duration(t) * time.Second
		}
	}
}

func applyMapOverrides(conf *Config, cfg map[string]interface{}) {
	if v, ok := cfg["api_key"].(string); ok && v != "" {
		conf.APIKey = v
	}
	if v, ok := cfg["ws_url"].(string); ok && v != "" {
		conf.WsURL = v
	}
	if v, ok := cfg["model"].(string); ok && v != "" {
		conf.Model = v
	}
	if v, ok := cfg["format"].(string); ok && v != "" {
		conf.Format = v
	}
	if v, ok := cfg["sample_rate"].(int); ok && v > 0 {
		conf.SampleRate = v
	} else if v, ok := cfg["sample_rate"].(float64); ok && v > 0 {
		conf.SampleRate = int(v)
	}
	if v, ok := cfg["language_hints"]; ok {
		conf.LanguageHints = parseLanguageHints(v)
	} else if v, ok := cfg["language"]; ok {
		conf.LanguageHints = parseLanguageHints(v)
	}
	if v, ok := cfg["vocabulary_id"].(string); ok && v != "" {
		conf.VocabularyID = v
	}
	if v, ok := cfg["disfluency_removal_enabled"].(bool); ok {
		conf.DisfluencyRemovalEnabled = v
	}
	if v, ok := cfg["semantic_punctuation_enabled"].(bool); ok {
		conf.SemanticPunctuationEnabled = v
	}
	if v, ok := cfg["timeout"].(int); ok && v > 0 {
		conf.Timeout = time.Duration(v) * time.Second
	} else if v, ok := cfg["timeout"].(float64); ok && v > 0 {
		conf.Timeout = time.Duration(int(v)) * time.Second
	}
}

func parseLanguageHints(value interface{}) []string {
	switch v := value.(type) {
	case []string:
		return cleanLanguageHints(v)
	case []interface{}:
		hints := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				hints = append(hints, s)
			}
		}
		return cleanLanguageHints(hints)
	case string:
		return splitLanguageHints(v)
	default:
		return nil
	}
}

func splitLanguageHints(value string) []string {
	normalized := strings.NewReplacer("，", ",", ";", ",", "；", ",").Replace(value)
	return cleanLanguageHints(strings.Split(normalized, ","))
}

func cleanLanguageHints(values []string) []string {
	hints := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			hints = append(hints, value)
		}
	}
	return hints
}
