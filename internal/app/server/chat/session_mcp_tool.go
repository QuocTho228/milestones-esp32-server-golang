package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	user_config "milestones-esp32-server-golang/internal/domain/config"
	config_types "milestones-esp32-server-golang/internal/domain/config/types"
	llm_memory "milestones-esp32-server-golang/internal/domain/memory/llm_memory"
	"milestones-esp32-server-golang/internal/domain/rag"
	log "milestones-esp32-server-golang/logger"

	"github.com/spf13/viper"
)

//Tệp này xử lý các lệnh gọi công cụ (tool) được gắn (bind) với session cho local mcp tool

// Cấu trúc phản hồi API tìm kiếm nhạc
type MusicSearchResponse struct {
	Data  []MusicItem `json:"data"`
	Code  int         `json:"code"`
	Error string      `json:"error"`
}

type MusicItem struct {
	Type   string `json:"type"`
	Link   string `json:"link"`
	SongID string `json:"songid"`
	Title  string `json:"title"`
	Author string `json:"author"`
	LRC    bool   `json:"lrc"`
	URL    string `json:"url"`
	Pic    string `json:"pic"`
}

// HTTP client toàn cục
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// Lấy HTTP client đã được cấu hình connection pool
func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}
	})
	return httpClient
}

// Đóng phiên (session)
func (c *ChatManager) LocalMcpCloseChat() error {
	return c.ExitChat()
}

// Xóa lịch sử hội thoại
func (c *ChatManager) LocalMcpClearHistory() error {
	llm_memory.Get().ResetMemory(c.ctx, c.DeviceID)
	return nil
}

type PlayMusicParams struct {
	Name string `json:"name,omitempty" description:"Tên bài hát"`
	//Welcome string `json:"welcome" description:"Tìm kiếm nhạc có thể tốn nhiều thời gian, dùng để trấn an người dùng" required:"true"`
}

type MusicPlaybackControlParams struct {
	Action string `json:"action" description:"Hành động điều khiển: resume (tiếp tục phát/khôi phục phát/nghe tiếp/phát tiếp), pause, stop, prev, next, play_playlist (phát danh sách phát/phát các bài trong danh sách phát), enqueue_current; play và continue cũng sẽ được chuẩn hóa thành resume" required:"true"`
}

type MusicPlaybackControlResult struct {
	Action          string `json:"action"`
	Status          string `json:"status"`
	CurrentTitle    string `json:"current_title,omitempty"`
	CurrentIndex    int    `json:"current_index"`
	PlaylistLength  int    `json:"playlist_length"`
	CurrentSource   string `json:"current_source,omitempty"`
	PositionMs      int64  `json:"position_ms"`
	AddedTitle      string `json:"added_title,omitempty"`
	SilenceResponse bool   `json:"silence_response"`
}

// Phát nhạc
func (c *ChatManager) LocalMcpPlayMusic(ctx context.Context, musicParams *PlayMusicParams) error {
	musicName := musicParams.Name
	//welcome := musicParams.Welcome
	welcome := ""
	log.Infof("Đang tìm nhạc: %s, welcome: %s", musicName, welcome)
	var musicURL, realMusicName string
	var wg sync.WaitGroup
	var ierr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		// Ở đây có thể lấy URL nhạc theo tên bài hát
		// Hiện tại đơn giản hóa cách triển khai, giả định musicName chính là URL hoặc lấy từ cấu hình
		musicURL, realMusicName, ierr = getMusicURL(musicName)
		if ierr != nil {
			log.Errorf("Lấy URL nhạc thất bại: %v", ierr)
			return
		}

		return
	}()
	go func() {
		defer wg.Done()
		//c.session.ttsManager.handleTts(ctx, common.LLMResponseStruct{Text: welcome, IsStart: true})
	}()

	wg.Wait()

	if musicURL == "" {
		log.Errorf("Không tìm thấy nhạc: %s", musicName)
		return fmt.Errorf("Không tìm thấy nhạc: %s", musicName)
	}

	log.Infof("Đã tìm thấy nhạc: %s, URL: %s", realMusicName, musicURL)

	return nil
}

// LocalMcpSwitchDeviceRole Chuyển đổi vai trò thiết bị theo tên vai trò (hỗ trợ khớp mờ - fuzzy match)
func (c *ChatManager) LocalMcpSwitchDeviceRole(ctx context.Context, roleName string) (string, error) {
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return "", fmt.Errorf("role_name không được để trống")
	}

	configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		return "", fmt.Errorf("Lấy config provider thất bại: %w", err)
	}

	matchedRoleName, err := configProvider.SwitchDeviceRoleByName(ctx, c.DeviceID, roleName)
	if err != nil {
		return "", err
	}

	if err := c.ReloadDeviceConfig(ctx); err != nil {
		return "", fmt.Errorf("Đã chuyển vai trò, nhưng làm mới cấu hình phiên thất bại: %w", err)
	}

	log.Infof("Thiết bị %s chuyển vai trò thành công, yêu cầu=%s, khớp=%s", c.DeviceID, roleName, matchedRoleName)
	return matchedRoleName, nil
}

// LocalMcpRestoreDeviceDefaultRole Khôi phục vai trò mặc định của thiết bị
func (c *ChatManager) LocalMcpRestoreDeviceDefaultRole(ctx context.Context) error {
	configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		return fmt.Errorf("Lấy config provider thất bại: %w", err)
	}

	if err := configProvider.RestoreDeviceDefaultRole(ctx, c.DeviceID); err != nil {
		return err
	}

	if err := c.ReloadDeviceConfig(ctx); err != nil {
		return fmt.Errorf("Đã khôi phục vai trò mặc định, nhưng làm mới cấu hình phiên thất bại: %w", err)
	}

	log.Infof("Thiết bị %s khôi phục vai trò mặc định thành công", c.DeviceID)
	return nil
}

// LocalMcpSearchKnowledge Truy vấn cơ sở tri thức (knowledge base) được gắn với agent hiện tại
func (c *ChatManager) LocalMcpSearchKnowledge(ctx context.Context, query string, topK int, knowledgeBaseIDs []uint) ([]config_types.KnowledgeSearchHit, error) {
	if c == nil || c.clientState == nil {
		return nil, fmt.Errorf("Trạng thái phiên (session) không khả dụng")
	}
	return rag.Search(ctx, query, topK, c.clientState.DeviceConfig.KnowledgeBases, knowledgeBaseIDs)
}

func (c *ChatManager) LocalMcpControlMusicPlayback(ctx context.Context, params *MusicPlaybackControlParams) (*MusicPlaybackControlResult, error) {
	if c == nil {
		return nil, fmt.Errorf("chat manager không khả dụng")
	}
	return controlMusicPlayback(ctx, c.GetSession(), params)
}

func controlMusicPlayback(ctx context.Context, session *ChatSession, params *MusicPlaybackControlParams) (*MusicPlaybackControlResult, error) {
	if session == nil || session.mediaPlayer == nil {
		return nil, fmt.Errorf("Trình phát media không khả dụng")
	}
	if params == nil {
		return nil, fmt.Errorf("Tham số điều khiển không được để trống")
	}

	action := normalizeMusicPlaybackAction(params.Action)
	if action == "" {
		return nil, fmt.Errorf("Hành động điều khiển không được hỗ trợ: %s", params.Action)
	}

	result := &MusicPlaybackControlResult{
		Action:          action,
		SilenceResponse: true,
	}

	switch action {
	case "resume":
		if err := session.mediaPlayer.Play(ctx); err != nil {
			return nil, err
		}
	case "pause":
		if err := session.mediaPlayer.Pause(); err != nil {
			return nil, err
		}
		flushQueuedMediaAudio(session, action)
	case "stop":
		if err := session.mediaPlayer.Stop(ctx); err != nil {
			return nil, err
		}
		flushQueuedMediaAudio(session, action)
	case "prev":
		if err := session.mediaPlayer.Prev(ctx); err != nil {
			return nil, err
		}
	case "next":
		if err := session.mediaPlayer.Next(ctx); err != nil {
			return nil, err
		}
	case "play_playlist":
		if err := session.mediaPlayer.PlayAgentPlaylist(ctx); err != nil {
			return nil, err
		}
	case "enqueue_current":
		appendResult, err := session.mediaPlayer.AppendCurrentToPlaylist()
		if err != nil {
			return nil, err
		}
		result.AddedTitle = appendResult.AddedTitle
		if _, err := session.mediaPlayer.ResumeIfInterruptedPause(); err != nil {
			log.Warnf("enqueue_current tự động khôi phục phát thất bại: %v", err)
		}
	}

	state := session.mediaPlayer.GetState()
	result.Status = state.Status.String()
	result.CurrentTitle = state.CurrentTitle
	result.CurrentIndex = state.CurrentIndex
	result.PlaylistLength = len(state.Playlist)
	result.CurrentSource = string(state.CurrentSourceType)
	result.PositionMs = state.PositionMs
	return result, nil
}

func flushQueuedMediaAudio(session *ChatSession, action string) {
	if session == nil || session.ttsManager == nil {
		return
	}

	interruptCtx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := session.ttsManager.InterruptAndClearQueueSync(interruptCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Warnf("Dọn hàng đợi gửi audio sau khi điều khiển media bị timeout: action=%s", action)
			return
		}
		if !errors.Is(err, context.Canceled) {
			log.Warnf("Dọn hàng đợi gửi audio sau khi điều khiển media thất bại: action=%s, err=%v", action, err)
		}
	}
}

func normalizeMusicPlaybackAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "play", "resume", "continue":
		return "resume"
	case "pause":
		return "pause"
	case "stop":
		return "stop"
	case "prev", "previous":
		return "prev"
	case "next":
		return "next"
	case "play_playlist", "play_agent_playlist", "play_playlist_songs", "playlist":
		return "play_playlist"
	case "enqueue_current", "append_current", "add_current_to_playlist":
		return "enqueue_current"
	default:
		return ""
	}
}

// searchMusicFromAPI Tìm nhạc từ API
func getMusicURL(musicName string) (string, string, error) {
	client := getHTTPClient()

	// Xây dựng request body
	data := fmt.Sprintf("input=%s&filter=name&type=migu&page=1",
		url.QueryEscape(musicName))

	req, err := http.NewRequest("POST", "https://music.txqq.pro/",
		strings.NewReader(data))
	if err != nil {
		return "", "", fmt.Errorf("Tạo request thất bại: %v", err)
	}

	// Thiết lập request header, giả lập request từ trình duyệt
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Origin", "https://music.txqq.pro")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", "https://music.txqq.pro/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("sec-ch-ua", `"Not)A;Brand";v="8", "Chromium";v="138", "Google Chrome";v="138"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)

	// Thiết lập timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("Yêu cầu API thất bại: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("Yêu cầu API thất bại, mã trạng thái: %d", resp.StatusCode)
	}

	// Phân tích response
	var searchResp MusicSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return "", "", fmt.Errorf("Phân tích response thất bại: %v", err)
	}

	if searchResp.Code != 200 {
		return "", "", fmt.Errorf("API trả về lỗi: %s", searchResp.Error)
	}

	if len(searchResp.Data) == 0 {
		return "", "", fmt.Errorf("Không tìm thấy nhạc: %s", musicName)
	}
	musicItem := searchResp.Data[0]
	// Trả về URL của kết quả tìm kiếm đầu tiên
	return musicItem.URL, musicItem.Title, nil
}