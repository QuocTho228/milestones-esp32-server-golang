package chat

import (
	"context"
	"testing"

	client "milestones-esp32-server-golang/internal/data/client"
	"milestones-esp32-server-golang/internal/domain/play_music"
)

func TestDetectRealtimeMcpAudioControlAction(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{text: "cho tôi tiếp tục phát", want: "resume"},
		{text: "tạm dừng một chút.", want: "pause"},
		{text: "dừng phát nhé", want: "stop"},
		{text: "bài tiếp theo", want: "next"},
		{text: "bài trước", want: "prev"},
		{text: "phát các bài trong danh sách phát", want: "play_playlist"},
		{text: "thêm bài đang phát vào danh sách nghe", want: "enqueue_current"},
		{text: "kể cho tôi nghe một câu chuyện cười", want: ""},
	}

	for _, tc := range cases {
		got := detectRealtimeMcpAudioControlAction(tc.text)
		if got != tc.want {
			t.Fatalf("detectRealtimeMcpAudioControlAction(%q)=%q, want %q", tc.text, got, tc.want)
		}
	}
}

func TestIsRealtimeMcpAudioExitCommand(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: "tạm biệt", want: true},
		{text: "vậy thì thoát cuộc trò chuyện", want: true},
		{text: "bye bye nhé", want: true},
		{text: "tiếp tục phát", want: false},
		{text: "hôm nay thời tiết thế nào", want: false},
	}

	for _, tc := range cases {
		got := isRealtimeMcpAudioExitCommand(tc.text)
		if got != tc.want {
			t.Fatalf("isRealtimeMcpAudioExitCommand(%q)=%v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestIsRealtimeMcpAudioPlaybackState(t *testing.T) {
	if !isRealtimeMcpAudioPlaybackState(MediaPlayerState{
		Status:            play_music.StatusPlaying,
		CurrentSourceType: MediaSourceTypeMCPResource,
	}) {
		t.Fatal("expected mcp playing state to be gated")
	}

	if isRealtimeMcpAudioPlaybackState(MediaPlayerState{
		Status:            play_music.StatusPaused,
		CurrentSourceType: MediaSourceTypeInlineAudio,
	}) {
		t.Fatal("expected inline paused state not to gate general ASR")
	}

	if isRealtimeMcpAudioPlaybackState(MediaPlayerState{
		Status:            play_music.StatusStopped,
		CurrentSourceType: MediaSourceTypeMCPResource,
	}) {
		t.Fatal("expected stopped state not to be gated")
	}

	if isRealtimeMcpAudioPlaybackState(MediaPlayerState{
		Status:            play_music.StatusPlaying,
		CurrentSourceType: MediaSourceTypeHTTPURL,
	}) {
		t.Fatal("expected non-mcp source not to be gated")
	}
}

func TestTryHandleRealtimeMcpAudioASRAllowsNormalChatWhenPlaybackPaused(t *testing.T) {
	session, runtime := newRealtimeGateTestSession(t)
	active := newActiveMediaPlayback(context.Background())
	defer active.cancel()
	defer active.closeDone()
	active.setPaused(true)

	source := MediaSourceDescriptor{SourceType: MediaSourceTypeMCPResource}
	runtime.mu.Lock()
	runtime.active = active
	runtime.attachment = &mediaSessionAttachment{}
	runtime.currentSource = &source
	runtime.state.Status = play_music.StatusPaused
	runtime.state.CurrentSourceType = MediaSourceTypeMCPResource
	runtime.mu.Unlock()

	handled, err := session.tryHandleRealtimeMcpAudioASR(context.Background(), "bạn đang làm gì vậy")
	if err != nil {
		t.Fatalf("tryHandleRealtimeMcpAudioASR returned error: %v", err)
	}
	if handled {
		t.Fatal("expected paused media context to allow normal chat through")
	}
}

func TestTryHandleRealtimeMcpAudioASRSwallowsNormalChatWhenPlaybackActive(t *testing.T) {
	session, runtime := newRealtimeGateTestSession(t)
	active := newActiveMediaPlayback(context.Background())
	defer active.cancel()
	defer active.closeDone()

	source := MediaSourceDescriptor{SourceType: MediaSourceTypeMCPResource}
	runtime.mu.Lock()
	runtime.active = active
	runtime.attachment = &mediaSessionAttachment{}
	runtime.currentSource = &source
	runtime.state.Status = play_music.StatusPlaying
	runtime.state.CurrentSourceType = MediaSourceTypeMCPResource
	runtime.mu.Unlock()

	handled, err := session.tryHandleRealtimeMcpAudioASR(context.Background(), "bạn đang làm gì vậy")
	if err != nil {
		t.Fatalf("tryHandleRealtimeMcpAudioASR returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected active media playback to gate normal ASR text")
	}
}

func newRealtimeGateTestSession(t *testing.T) (*ChatSession, *deviceMediaRuntime) {
	t.Helper()

	clientState := &client.ClientState{
		Ctx:        context.Background(),
		DeviceID:   t.Name(),
		ListenMode: "realtime",
	}
	session := &ChatSession{
		clientState: clientState,
	}
	session.mediaPlayer = NewSessionMediaPlayer(session)
	return session, session.mediaPlayer.runtime
}