package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	log "milestones-esp32-server-golang/logger"
)

const localMcpMusicControlToolName = "control_music_playback"

func init() {
	if err := RegisterLocalMcpFunc(
		localMcpMusicControlToolName,
		"Phải sử dụng khi người dùng muốn điều khiển nhạc hoặc âm thanh đang phát trên thiết bị hiện tại. Đối với các lệnh như “tiếp tục phát”, “khôi phục phát”, “tiếp tục nghe”, “phát tiếp”, “tạm dừng”, “dừng lại”, “bài trước”, “bài tiếp theo”, “phát danh sách phát”, “phát bài hát trong danh sách phát”, “phát playlist”, “thêm bài đang phát vào danh sách phát”, v.v., bắt buộc phải gọi công cụ này, không được chỉ trả lời bằng văn bản. Chỉ khi người dùng muốn phát bài hát mới, tìm kiếm bài hát hoặc yêu cầu phát một bản nhạc cụ thể thì mới không sử dụng công cụ này.",
		MusicPlaybackControlParams{},
		musicPlaybackControlHandler,
	); err != nil {
		log.Errorf("Đăng ký công cụ MCP cục bộ điều khiển media thất bại: %v", err)
	}
}

func musicPlaybackControlHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Infof("Thực thi công cụ điều khiển media, args=%s", argumentsInJSON)

	var params MusicPlaybackControlParams
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
			response := NewErrorResponse(localMcpMusicControlToolName, "Phân tích tham số thất bại", "PARSE_ERROR", "Vui lòng kiểm tra định dạng tham số action có chính xác không")
			return response.ToJSON()
		}
	}

	chatSessionOperatorValue := ctx.Value("chat_session_operator")
	if chatSessionOperatorValue == nil {
		return "", fmt.Errorf("Không tìm thấy chat_session_operator trong context")
	}

	chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator)
	if !ok {
		return "", fmt.Errorf("chat_session_operator lấy được từ context không phải kiểu ChatSessionOperator")
	}

	result, err := chatSessionOperator.LocalMcpControlMusicPlayback(ctx, &params)
	if err != nil {
		log.Errorf("Điều khiển media thất bại: %v", err)
		response := NewErrorResponse(localMcpMusicControlToolName, fmt.Sprintf("Điều khiển media thất bại: %v", err), "MEDIA_CONTROL_FAILED", "Vui lòng kiểm tra trạng thái phát hiện tại rồi thử lại")
		return response.ToJSON()
	}
	if result == nil {
		result = &MusicPlaybackControlResult{
			Action:          normalizeMusicPlaybackAction(params.Action),
			Status:          "unknown",
			SilenceResponse: true,
		}
	}

	action := normalizeMusicPlaybackAction(params.Action)
	if result != nil && result.Action != "" {
		action = result.Action
	}

	response := NewActionResponse(
		localMcpMusicControlToolName,
		action,
		buildMusicPlaybackControlMessage(result),
		result.Status,
		false,
	)
	response.NoFurtherResponse = result.SilenceResponse
	response.SilenceLLM = result.SilenceResponse
	response.Metadata = buildMusicPlaybackControlMetadata(result)

	return response.ToJSON()
}

func buildMusicPlaybackControlMessage(result *MusicPlaybackControlResult) string {
	if result == nil {
		return "Điều khiển media đã hoàn tất"
	}

	switch result.Action {
	case "resume":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("Đã tiếp tục phát: %s", result.CurrentTitle)
		}
		return "Đã tiếp tục phát"
	case "pause":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("Đã tạm dừng: %s", result.CurrentTitle)
		}
		return "Đã tạm dừng phát"
	case "stop":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("Đã dừng: %s", result.CurrentTitle)
		}
		return "Đã dừng phát"
	case "prev":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("Đã chuyển về bài trước: %s", result.CurrentTitle)
		}
		return "Đã chuyển về bài trước"
	case "next":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("Đã chuyển sang bài tiếp theo: %s", result.CurrentTitle)
		}
		return "Đã chuyển sang bài tiếp theo"
	case "play_playlist":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("Đã bắt đầu phát danh sách phát: %s", result.CurrentTitle)
		}
		return "Đã bắt đầu phát danh sách phát"
	case "enqueue_current":
		if result.AddedTitle != "" {
			return fmt.Sprintf("Đã thêm nguồn phát hiện tại vào danh sách phát: %s", result.AddedTitle)
		}
		return "Đã thêm nguồn phát hiện tại vào danh sách phát"
	default:
		return "Điều khiển media đã hoàn tất"
	}
}

func buildMusicPlaybackControlMetadata(result *MusicPlaybackControlResult) map[string]string {
	if result == nil {
		return nil
	}

	metadata := map[string]string{
		"action":          result.Action,
		"status":          result.Status,
		"current_title":   result.CurrentTitle,
		"current_index":   strconv.Itoa(result.CurrentIndex),
		"playlist_length": strconv.Itoa(result.PlaylistLength),
		"current_source":  result.CurrentSource,
		"position_ms":     strconv.FormatInt(result.PositionMs, 10),
	}
	if result.AddedTitle != "" {
		metadata["added_title"] = result.AddedTitle
	}
	return metadata
}