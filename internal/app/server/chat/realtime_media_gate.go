package chat

import (
	"context"
	"strings"
	"time"

	"milestones-esp32-server-golang/internal/domain/eventbus"
	"milestones-esp32-server-golang/internal/domain/play_music"
	log "milestones-esp32-server-golang/logger"
)

type realtimeMusicControlRule struct {
	action   string
	keywords []string
}

var realtimeMcpAudioControlRules = []realtimeMusicControlRule{
	{
		action: "play_playlist",
		keywords: []string{
			"phát danh sách phát",
			"phát các bài trong danh sách phát",
			"phát playlist",
			"danh sách phát",
		},
	},
	{
		action: "enqueue_current",
		keywords: []string{
			"vào danh sách nghe",
			"vào playlist",
			"thêm vào danh sách nghe",
			"thêm vào playlist",
		},
	},
	{
		action: "resume",
		keywords: []string{
			"tiếp tục phát",
			"phát lại",
			"tiếp tục nghe",
			"phát tiếp",
			"chơi tiếp",
		},
	},
	{
		action: "pause",
		keywords: []string{
			"tạm dừng",
			"dừng một chút",
		},
	},
	{
		action: "stop",
		keywords: []string{
			"dừng phát",
			"dừng lại",
			"ngừng phát",
			"đừng phát nữa",
		},
	},
	{
		action: "next",
		keywords: []string{
			"bài tiếp theo",
			"bài kế tiếp",
			"chuyển sang bài tiếp theo",
			"đổi bài",
		},
	},
	{
		action: "prev",
		keywords: []string{
			"bài trước đó",
			"bài trước",
			"chuyển về bài trước",
		},
	},
}

var realtimeMcpAudioExitKeywords = []string{
	"tạm biệt",
	"bye bye",
	"chào nhé",
	"hẹn gặp lại",
	"thoát cuộc trò chuyện",
	"thoát",
	"thôi nhé",
}

func normalizeRealtimeMcpAudioText(text string) string {
	return removePunctuation(strings.ToLower(strings.TrimSpace(text)))
}

func detectRealtimeMcpAudioControlAction(text string) string {
	normalizedText := normalizeRealtimeMcpAudioText(text)
	if normalizedText == "" {
		return ""
	}

	for _, rule := range realtimeMcpAudioControlRules {
		for _, keyword := range rule.keywords {
			normalizedKeyword := normalizeRealtimeMcpAudioText(keyword)
			if normalizedKeyword == "" {
				continue
			}
			if strings.Contains(normalizedText, normalizedKeyword) {
				return rule.action
			}
		}
	}

	return ""
}

func isRealtimeMcpAudioExitCommand(text string) bool {
	normalizedText := normalizeRealtimeMcpAudioText(text)
	if normalizedText == "" {
		return false
	}

	for _, keyword := range realtimeMcpAudioExitKeywords {
		normalizedKeyword := normalizeRealtimeMcpAudioText(keyword)
		if normalizedKeyword == "" {
			continue
		}
		if strings.Contains(normalizedText, normalizedKeyword) {
			return true
		}
	}

	return false
}

func isRealtimeMcpAudioSourceType(sourceType MediaSourceType) bool {
	return sourceType == MediaSourceTypeMCPResource || sourceType == MediaSourceTypeInlineAudio
}

func isRealtimeMcpAudioPlaybackState(state MediaPlayerState) bool {
	if !isRealtimeMcpAudioSourceType(state.CurrentSourceType) {
		return false
	}

	return state.Status == play_music.StatusPlaying
}

func (s *ChatSession) hasRealtimeMcpAudioControlContext() bool {
	if s == nil || s.clientState == nil || !s.clientState.IsRealTime() || s.mediaPlayer == nil {
		return false
	}

	return s.mediaPlayer.HasRealtimeMcpAudioControlContext()
}

func (s *ChatSession) isRealtimeMcpAudioGateActive() bool {
	if s == nil || s.clientState == nil || !s.clientState.IsRealTime() || s.mediaPlayer == nil {
		return false
	}

	return s.mediaPlayer.ShouldGateRealtimeMcpAudioASR()
}

func (s *ChatSession) tryHandleRealtimeMcpAudioASR(ctx context.Context, text string) (bool, error) {
	if !s.hasRealtimeMcpAudioControlContext() {
		return false, nil
	}

	if isRealtimeMcpAudioExitCommand(text) {
		eventbus.Get().Publish(eventbus.TopicExitChat, &eventbus.ExitChatEvent{
			ClientState: s.clientState,
			Reason:      "Người dùng thoát trong khi đang phát media thời gian thực",
			TriggerType: "realtime_media_exit_words",
			UserText:    text,
			Timestamp:   time.Now(),
		})
		log.Infof("Thiết bị %s: cổng chặn phát media thời gian thực nhận được lệnh thoát: %s", s.clientState.DeviceID, text)
		return true, nil
	}

	action := detectRealtimeMcpAudioControlAction(text)
	if action != "" {
		_, err := controlMusicPlayback(ctx, s, &MusicPlaybackControlParams{Action: action})
		if err != nil {
			log.Warnf("Thiết bị %s: cổng chặn phát media thời gian thực thực thi hành động điều khiển thất bại: action=%s, text=%s, err=%v", s.clientState.DeviceID, action, text, err)
			return true, nil
		}
		log.Infof("Thiết bị %s: cổng chặn phát media thời gian thực thực thi hành động điều khiển: action=%s, text=%s", s.clientState.DeviceID, action, text)
		return true, nil
	}

	if !s.isRealtimeMcpAudioGateActive() {
		return false, nil
	}

	log.Debugf("Thiết bị %s: cổng chặn phát media thời gian thực bỏ qua văn bản ASR: %s", s.clientState.DeviceID, text)
	return true, nil
}