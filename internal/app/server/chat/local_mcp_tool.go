package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	mcp_manager "milestones-esp32-server-golang/internal/domain/mcp"
	log "milestones-esp32-server-golang/logger"

	//"github.com/scroot/music-sd/pkg/netease"
	//"github.com/scroot/music-sd/pkg/qq"
	"github.com/spf13/viper"
)

type LocalMcpTool struct {
	Name        string
	Description string
	Params      any
	Handle      mcp_manager.LocalToolHandler
}

// InitChatLocalMCPTools khởi tạo các công cụ MCP cục bộ liên quan đến trò chuyện
func InitChatLocalMCPTools() {
	manager := mcp_manager.GetLocalMCPManager()

	log.Info("Khởi tạo các công cụ MCP cục bộ liên quan đến trò chuyện...")

	localTools := map[string]LocalMcpTool{
		/*"get_current_datetime": {
			Name:        "get_current_datetime",
			Description: "Lấy thông tin thời gian và ngày tháng hiện tại",
			Params:      struct{}{},
			Handle:      getCurrentDateTimeHandler,
		},*/
		"exit_conversation": {
			Name:        "exit_conversation",
			Description: "Sử dụng khi người dùng thể hiện rõ ràng muốn kết thúc cuộc trò chuyện, thoát khỏi hệ thống hoặc chào tạm biệt; dùng để đóng phiên trò chuyện hiện tại một cách nhẹ nhàng",
			Params:      struct{}{},
			Handle:      exitConversationHandler,
		},
		"clear_conversation_history": {
			Name:        "clear_conversation_history",
			Description: "Sử dụng khi người dùng yêu cầu xóa, làm sạch hoặc đặt lại lịch sử trò chuyện; dùng để xóa toàn bộ nội dung lịch sử trò chuyện của phiên hiện tại",
			Params:      struct{}{},
			Handle:      clearConversationHistoryHandler,
		},
		"switch_device_role": {
			Name:        "switch_device_role",
			Description: "Sử dụng khi người dùng yêu cầu chuyển thiết bị hiện tại sang một vai trò (role) nào đó; tham số role_name hỗ trợ khớp mờ (fuzzy match), sẽ tìm kiếm trong các vai trò toàn cục và các vai trò của người dùng sở hữu thiết bị đó",
			Params:      SwitchDeviceRoleParams{},
			Handle:      switchDeviceRoleHandler,
		},
		"restore_device_default_role": {
			Name:        "restore_device_default_role",
			Description: "Sử dụng khi người dùng yêu cầu khôi phục vai trò mặc định của thiết bị, hủy bỏ việc ghi đè vai trò hiện tại của thiết bị",
			Params:      struct{}{},
			Handle:      restoreDeviceDefaultRoleHandler,
		},
		"search_knowledge": {
			Name:        "search_knowledge",
			Description: "Khi câu hỏi của người dùng cần căn cứ thực tế, quy tắc quy trình, chi tiết tham số hoặc điều khoản tài liệu, hãy truy xuất cơ sở tri thức liên kết với trợ lý (agent) hiện tại và trả về các đoạn nội dung liên quan; có thể truyền tùy chọn knowledge_base_ids để chỉ tìm trong các cơ sở tri thức được chỉ định; không gọi công cụ này trong các tình huống trò chuyện phiếm hoặc sáng tác thuần túy",
			Params:      SearchKnowledgeParams{},
			Handle:      searchKnowledgeHandler,
		},
		/*"play_music": {
			Name:        "play_music",
			Description: "Sử dụng khi người dùng muốn nghe nhạc, cảm thấy buồn chán hoặc muốn thư giãn đầu óc; dùng để phát bản nhạc có tên chỉ định. Khi người dùng muốn nghe ngẫu nhiên một bài nhạc, hãy đề xuất một tên bài hát cụ thể; khi có nhiều công cụ phát nhạc, ưu tiên sử dụng công cụ này. **Việc gọi công cụ này tốn khá nhiều thời gian, cần trả về lời nhắc chuyển tiếp thân thiện trước**",
			Params:      PlayMusicParams{},
			Handle:      playMusicHandler,
		},*/
	}

	for toolName, localTool := range localTools {
		// Chỉ bỏ qua khi cấu hình được đặt rõ ràng là false; nếu cấu hình không tồn tại hoặc là true thì đều được bật
		if viper.IsSet("local_mcp."+toolName) && !viper.GetBool("local_mcp."+toolName) {
			continue
		}
		err := manager.RegisterToolFunc(
			localTool.Name,
			localTool.Description,
			localTool.Params,
			localTool.Handle,
		)
		if err != nil {
			log.Errorf("Đăng ký công cụ MCP cục bộ %s thất bại: %+v", toolName, err)
		}
	}

	log.Info("Các công cụ MCP cục bộ liên quan đến trò chuyện đã được khởi tạo.")
}

func RegisterLocalMcpFunc(name string, description string, params any, handle mcp_manager.LocalToolHandler) error {
	manager := mcp_manager.GetLocalMCPManager()

	err := manager.RegisterToolFunc(
		name,
		description,
		params,
		handle,
	)
	if err != nil {
		log.Errorf("Đăng ký công cụ MCP cục bộ %s thất bại: %+v", name, err)
		return err
	}
	return nil
}

type SwitchDeviceRoleParams struct {
	RoleName string `json:"role_name" description:"Tên vai trò đích, hỗ trợ khớp mờ" required:"true"`
}

type SearchKnowledgeParams struct {
	Query            string `json:"query" description:"Nội dung truy vấn cần tìm kiếm" required:"true"`
	TopK             int    `json:"top_k,omitempty" description:"Số lượng kết quả trả về, mặc định là 5"`
	KnowledgeBaseIDs []uint `json:"knowledge_base_ids,omitempty" description:"Tùy chọn: chỉ tìm kiếm trong các cơ sở tri thức có ID này (đã được liên kết với trợ lý hiện tại)"`
}

// playMusicHandler hàm xử lý phát nhạc
func playMusicHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("Chạy công cụ phát nhạc")

	// Phân tích tham số
	var params PlayMusicParams

	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
			response := NewErrorResponse("play_music", "Phân tích tham số thất bại", "PARSE_ERROR", "Vui lòng kiểm tra lại định dạng tham số")
			return response.ToJSON()
		}
	}

	log.Infof("Đối tượng ChatSessionOperator được phát hiện đang gọi phương thức LocalMcpPlayMusic để phát nhạc: %s", params.Name)
	audioData, realMusicName, err := GetMusicAudioData(ctx, &params)
	if err != nil {
		log.Errorf("Không thể truy xuất dữ liệu âm nhạc: %v", err)
		response := NewErrorResponse("play_music", fmt.Sprintf("Lấy dữ liệu âm nhạc thất bại: %v", err), "PLAYBACK_ERROR", "Vui lòng kiểm tra lại tên bài hát hoặc kết nối mạng")
		return response.ToJSON()
	} else {
		// Phát thành công - phản hồi loại hành động, dừng các bước xử lý tiếp theo
		response := NewAudioResponse("play_music", "play_music", fmt.Sprintf("Bắt đầu phát nhạc: %s", realMusicName), true, audioData)
		response.MusicName = realMusicName
		return response.ToJSON()
	}

}

/*
// getCurrentDateTimeHandler hàm xử lý lấy thời gian và ngày tháng hiện tại (đã bị comment trong bản gốc)
// Giữ nguyên phần code mẫu bị vô hiệu hóa này, chỉ dịch phần chú thích tiêu đề, phần thân bên trong giữ nguyên do đã bị tắt trong mã nguồn gốc.
*/
// exitConversationHandler hàm xử lý thoát cuộc trò chuyện
func exitConversationHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("Thực thi công cụ hộp thoại thoát")

	// Phân tích tham số
	var params map[string]interface{}
	reason := "Người dùng chủ động thoát" // Lý do mặc định

	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err == nil {
			if r, ok := params["reason"].(string); ok && r != "" {
				reason = r
			}
		}
	}

	// Tạo phản hồi loại hành động - thao tác kết thúc
	response := NewActionResponse("exit_conversation", "exit_conversation", "Cuộc trò chuyện sắp kết thúc, cảm ơn bạn đã sử dụng!", "exiting", true)
	response.UserState = "conversation_ended"
	response.Instruction = "Cuộc trò chuyện đã kết thúc, vui lòng không tạo thêm phản hồi văn bản"
	response.Metadata = map[string]string{
		"reason":           reason,
		"exit_code":        "0",
		"farewell_chinese": "Tạm biệt! Tôi mong sẽ được liên lạc lại với bạn lần sau.",
		"farewell_english": "Goodbye! Looking forward to our next conversation.",
	}

	log.Infof("Quá trình thoát hộp thoại đã hoàn tất. Lý do:: %s", reason)

	// Lấy ChatSessionOperator từ context và gọi phương thức Close
	if chatSessionOperatorValue := ctx.Value("chat_session_operator"); chatSessionOperatorValue != nil {
		if chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator); ok {
			log.Info("Đối tượng ChatSessionOperator được phát hiện đang gọi phương thức Close để đóng phiên làm việc.")
			defer chatSessionOperator.LocalMcpCloseChat()
		} else {
			log.Warn("Đối tượng chat_session_operator nhận được từ ngữ cảnh không thuộc kiểu ChatSessionOperator.")
		}
	} else {
		log.Warn("Đối tượng chat_session_operator không được tìm thấy trong ngữ cảnh.")
	}

	responseStr, err := response.ToJSON()
	if err != nil {
		return "", err
	}

	return responseStr, nil
}

// clearConversationHistoryHandler hàm xử lý xóa lịch sử trò chuyện
func clearConversationHistoryHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("Công cụ xóa lịch sử hộp thoại")

	// Phân tích tham số
	var params map[string]interface{}
	reason := "Người dùng chủ động xóa lịch sử" // Lý do mặc định

	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err == nil {
			if r, ok := params["reason"].(string); ok && r != "" {
				reason = r
			}
		}
	}

	// Lấy ChatSessionOperator từ context và gọi phương thức LocalMcpClearHistory
	if chatSessionOperatorValue := ctx.Value("chat_session_operator"); chatSessionOperatorValue != nil {
		if chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator); ok {
			log.Info("Đối tượng ChatSessionOperator được phát hiện đang gọi phương thức LocalMcpClearHistory để xóa lịch sử.")
			if err := chatSessionOperator.LocalMcpClearHistory(); err != nil {
				log.Errorf("Xóa lịch sử không thành công: %v", err)
				return "", err
			} else {
				// Xóa thành công - phản hồi loại hành động, nhưng không kết thúc cuộc trò chuyện
				response := NewActionResponse("clear_conversation_history", "clear_history", "Lịch sử trò chuyện đã được xóa thành công, bạn có thể bắt đầu một cuộc trò chuyện hoàn toàn mới.", "completed", false)
				response.Metadata = map[string]string{
					"reason": reason,
					"status": "cleared",
				}
				log.Info("Cuộc đối thoại lịch sử đã được hoàn tất thành công.")

				return response.ToJSON()
			}
		} else {
			log.Warn("Đối tượng chat_session_operator nhận được từ ngữ cảnh không thuộc kiểu ChatSessionOperator.")
			return "", fmt.Errorf("Đối tượng chat_session_operator nhận được từ ngữ cảnh không thuộc kiểu ChatSessionOperator.")
		}
	}
	log.Warn("Không tìm thấy chat_session_operator trong ngữ cảnh")
	return "", fmt.Errorf("Không tìm thấy chat_session_operator trong ngữ cảnh")
}

// switchDeviceRoleHandler hàm xử lý chuyển đổi vai trò thiết bị
func switchDeviceRoleHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("Thực thi công cụ chuyển đổi vai trò thiết bị")

	var params SwitchDeviceRoleParams
	if argumentsInJSON == "" {
		response := NewErrorResponse("switch_device_role", "Thiếu tham số role_name", "MISSING_ROLE_NAME", "Vui lòng cung cấp tên vai trò cần chuyển đổi")
		return response.ToJSON()
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		response := NewErrorResponse("switch_device_role", "Phân tích tham số thất bại", "PARSE_ERROR", "Vui lòng kiểm tra lại định dạng tham số role_name")
		return response.ToJSON()
	}
	params.RoleName = strings.TrimSpace(params.RoleName)
	if params.RoleName == "" {
		response := NewErrorResponse("switch_device_role", "Tên vai trò không được để trống", "INVALID_ROLE_NAME", "Vui lòng cung cấp role_name hợp lệ")
		return response.ToJSON()
	}

	if chatSessionOperatorValue := ctx.Value("chat_session_operator"); chatSessionOperatorValue != nil {
		if chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator); ok {
			matchedRoleName, err := chatSessionOperator.LocalMcpSwitchDeviceRole(ctx, params.RoleName)
			if err != nil {
				log.Errorf("Việc chuyển đổi vai trò thiết bị đã thất bại: %v", err)
				response := NewErrorResponse("switch_device_role", fmt.Sprintf("Chuyển đổi vai trò thất bại: %v", err), "SWITCH_ROLE_FAILED", "Vui lòng thử đổi tên vai trò khác hoặc thử lại sau")
				return response.ToJSON()
			}

			response := NewActionResponse(
				"switch_device_role",
				"switch_device_role",
				fmt.Sprintf("Đã chuyển sang vai trò: %s", matchedRoleName),
				"completed",
				false,
			)
			response.Metadata = map[string]string{
				"requested_role_name": params.RoleName,
				"matched_role_name":   matchedRoleName,
			}
			return response.ToJSON()
		}
		return "", fmt.Errorf("chat_session_operator lấy từ context không thuộc kiểu ChatSessionOperator")
	}

	return "", fmt.Errorf("không tìm thấy chat_session_operator trong context")
}

// restoreDeviceDefaultRoleHandler hàm xử lý khôi phục vai trò mặc định của thiết bị
func restoreDeviceDefaultRoleHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("Chạy công cụ để khôi phục vai trò mặc định của thiết bị.")

	if chatSessionOperatorValue := ctx.Value("chat_session_operator"); chatSessionOperatorValue != nil {
		if chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator); ok {
			if err := chatSessionOperator.LocalMcpRestoreDeviceDefaultRole(ctx); err != nil {
				log.Errorf("Khôi phục vai trò mặc định của thiết bị không thành công: %v", err)
				response := NewErrorResponse("restore_device_default_role", fmt.Sprintf("Khôi phục vai trò mặc định không thành công: %v", err), "RESTORE_ROLE_FAILED", "Vui lòng thử lại sau")
				return response.ToJSON()
			}

			response := NewActionResponse(
				"restore_device_default_role",
				"restore_device_default_role",
				"Đã khôi phục vai trò mặc định của thiết bị",
				"completed",
				false,
			)
			return response.ToJSON()
		}
		return "", fmt.Errorf("chat_session_operator lấy từ context không thuộc kiểu ChatSessionOperator")
	}

	return "", fmt.Errorf("không tìm thấy chat_session_operator trong context")
}

func searchKnowledgeHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("Thực thi các công cụ truy xuất cơ sở tri thức")

	var params SearchKnowledgeParams
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
			response := NewErrorResponse("search_knowledge", "Phân tích tham số thất bại", "PARSE_ERROR", "Vui lòng kiểm tra lại định dạng tham số query")
			return response.ToJSON()
		}
	}
	params.Query = strings.TrimSpace(params.Query)
	if params.Query == "" {
		response := NewErrorResponse("search_knowledge", "query không được để trống", "INVALID_QUERY", "Vui lòng cung cấp nội dung cần truy vấn")
		return response.ToJSON()
	}
	if params.TopK <= 0 {
		params.TopK = 5
	}

	chatSessionOperatorValue := ctx.Value("chat_session_operator")
	if chatSessionOperatorValue == nil {
		return "", fmt.Errorf("không tìm thấy chat_session_operator trong context")
	}
	chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator)
	if !ok {
		return "", fmt.Errorf("chat_session_operator lấy từ context không thuộc kiểu ChatSessionOperator")
	}

	hits, err := chatSessionOperator.LocalMcpSearchKnowledge(ctx, params.Query, params.TopK, params.KnowledgeBaseIDs)
	if err != nil {
		response := NewErrorResponse("search_knowledge", fmt.Sprintf("Truy xuất thông tin thất bại: %v", err), "SEARCH_FAILED", "Vui lòng thử lại sau")
		return response.ToJSON()
	}

	data := map[string]interface{}{
		"query": params.Query,
		"hits":  hits,
		"count": len(hits),
	}
	if len(hits) == 0 {
		response := NewContentResponse("search_knowledge", data, "Không tìm thấy thông tin liên quan đủ mức độ")
		return response.ToJSON()
	}

	var builder strings.Builder
	for i, hit := range hits {
		content := strings.TrimSpace(hit.Content)
		if content == "" {
			continue
		}
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, content))
	}
	msg := strings.TrimSpace(builder.String())
	if msg == "" {
		msg = "Đã lấy được thông tin liên quan"
	}
	response := NewContentResponse("search_knowledge", data, msg)
	return response.ToJSON()
}

// getWeekNumber lấy số thứ tự tuần trong năm
func getWeekNumber(t time.Time) int {
	_, week := t.ISOWeek()
	return week
}

// formatVietnameseDateTime định dạng ngày giờ theo kiểu tiếng Việt
func formatVietnameseDateTime(t time.Time) string {
	weekdays := map[time.Weekday]string{
		time.Sunday:    "Chủ nhật",
		time.Monday:    "Thứ hai",
		time.Tuesday:   "Thứ ba",
		time.Wednesday: "Thứ tư",
		time.Thursday:  "Thứ năm",
		time.Friday:    "Thứ sáu",
		time.Saturday:  "Thứ bảy",
	}

	return fmt.Sprintf("Ngày %d tháng %d năm %d, %s, %02d:%02d:%02d",
		t.Day(), int(t.Month()), t.Year(),
		weekdays[t.Weekday()],
		t.Hour(), t.Minute(), t.Second(),
	)
}

// getWeekdayVietnamese lấy tên thứ trong tuần bằng tiếng Việt
func getWeekdayVietnamese(weekday time.Weekday) string {
	weekdays := map[time.Weekday]string{
		time.Sunday:    "Chủ nhật",
		time.Monday:    "Thứ hai",
		time.Tuesday:   "Thứ ba",
		time.Wednesday: "Thứ tư",
		time.Thursday:  "Thứ năm",
		time.Friday:    "Thứ sáu",
		time.Saturday:  "Thứ bảy",
	}
	return weekdays[weekday]
}

// RegisterChatMCPTools hàm công khai để bên ngoài gọi nhằm đăng ký các công cụ MCP cho trò chuyện
func RegisterChatMCPTools() {
	InitChatLocalMCPTools()
}

// Phát nhạc
func GetMusicAudioData(ctx context.Context, musicParams *PlayMusicParams) ([]byte, string, error) {
	musicName := musicParams.Name
	//welcome := musicParams.Welcome
	welcome := ""
	log.Infof("Đang tìm nhạc: %s, welcome: %s", musicName, welcome)
	// Ở đây có thể lấy URL nhạc dựa theo tên bài hát
	// Hiện tại triển khai đơn giản, giả định musicName chính là URL hoặc lấy từ cấu hình
	musicURL, realMusicName, ierr := getMusicURL(musicName)
	if ierr != nil {
		log.Errorf("Không thể truy xuất URL nhạc: %v", ierr)
		return nil, "", fmt.Errorf("Không thể truy xuất URL nhạc: %v", ierr)
	}

	log.Infof("Tìm kiếm nhạc thành công. URL: %s, Tên bài hát: %s", musicURL, realMusicName)

	client := getHTTPClient()
	req, err := http.NewRequest("GET", musicURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("tạo request thất bại: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("gọi API thất bại: %v", err)
	}
	defer resp.Body.Close()

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("đọc phản hồi thất bại: %v", err)
	}

	log.Infof("Đã truy xuất dữ liệu âm nhạc thành công (%s). Độ dài dữ liệu âm thanh: %d", realMusicName, len(audioData))

	return audioData, realMusicName, nil
}