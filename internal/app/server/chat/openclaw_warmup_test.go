package chat

import (
	"strings"
	"testing"
)

func TestParseOpenClawWarmupPlanObjects(t *testing.T) {
	got := parseOpenClawWarmupPlan(`[{"text":"Để tôi xem đã"},{"text":"Vẫn theo dõi đây"},{"text":"Việc này tôi lo"},{"text":"Kết quả đang tới"},{"text":"Tôi vẫn đang xem"},{"text":"Đang kiểm tra nè"},{"text":"Đang xác nhận nè"},{"text":"Số liệu sắp có"},{"text":"Tôi vẫn đang chờ"},{"text":"Xong sẽ báo ngay"},{"text":"Tôi xem lại chút"}]`)

	if len(got) != openClawWarmupPlanSize {
		t.Fatalf("unexpected plan size: got %d want %d", len(got), openClawWarmupPlanSize)
	}
	if got[0] != "Để tôi xem đã" {
		t.Fatalf("unexpected first line: %q", got[0])
	}
	if got[4] != "Tôi vẫn đang xem" {
		t.Fatalf("unexpected last line: %q", got[4])
	}
	if got[9] != "Xong sẽ báo ngay" {
		t.Fatalf("unexpected tenth line: %q", got[9])
	}
	if got[10] != "Tôi xem lại chút" {
		t.Fatalf("unexpected eleventh line: %q", got[10])
	}
}

func TestParseOpenClawWarmupPlanReturnsEmptyOnInvalidJSON(t *testing.T) {
	got := parseOpenClawWarmupPlan("not-json")

	for idx, line := range got {
		if line != "" {
			t.Fatalf("expected empty line at %d, got %q", idx, line)
		}
	}
}

func TestBuildOpenClawWarmupHint(t *testing.T) {
	got := buildOpenClawWarmupHint("giúp tôi kiểm tra thời tiết Thượng Hải hôm nay thế nào?")
	if got == "" {
		t.Fatal("expected non-empty hint")
	}
	if strings.Contains(got, "giúp tôi") {
		t.Fatalf("hint should not contain user command: %q", got)
	}
	if len([]rune(got)) > openClawWarmupHintMaxRunes {
		t.Fatalf("hint too long: %q", got)
	}
}

func TestBuildOpenClawWarmupHintWeatherTopic(t *testing.T) {
	got := buildOpenClawWarmupHint("thời tiết Thiên Tân ngày mốt thế nào?")
	if got != "thời tiết Thiên Tân ngày mốt" {
		t.Fatalf("unexpected weather hint: %q", got)
	}
}

func TestBuildOpenClawWarmupUserPromptIncludesTimeline(t *testing.T) {
	got := buildOpenClawWarmupUserPrompt("thời tiết Thiên Tân ngày mốt thế nào?")
	if !strings.Contains(got, "Nhiệm vụ của người dùng trong lượt này:") {
		t.Fatalf("task label missing from prompt: %q", got)
	}
	if !strings.Contains(got, "chỉ được rút gọn thành cụm danh từ \"thời tiết Thiên Tân ngày mốt\"") {
		t.Fatalf("topic hint missing from prompt: %q", got)
	}
	if !strings.Contains(got, "giây thứ 1, giây thứ 10, giây thứ 20, giây thứ 30, giây thứ 40, giây thứ 50, giây thứ 60, giây thứ 70, giây thứ 80, giây thứ 90, giây thứ 100") {
		t.Fatalf("timeline missing from prompt: %q", got)
	}
}

func TestFormatOpenClawWarmupTopicWeather(t *testing.T) {
	got := formatOpenClawWarmupTopic("thời tiết Thiên Tân ngày mốt")
	if got != "thời tiết Thiên Tân ngày mốt" {
		t.Fatalf("unexpected formatted topic: %q", got)
	}
}

func TestSanitizeOpenClawWarmupTextRejectsUserCommandEcho(t *testing.T) {
	got := sanitizeOpenClawWarmupText("Để tôi xem giúp tôi kiểm tra một chút.")
	if got != "" {
		t.Fatalf("expected invalid warmup text to be rejected, got %q", got)
	}
}

func TestTakeWarmupSegmentStartFlagOnlyMarksFirstWarmupSentence(t *testing.T) {
	task := &openClawWarmupTask{nextWarmupSegmentIsStart: true}

	if !task.takeWarmupSegmentStartFlag() {
		t.Fatal("expected first warmup sentence to carry start flag")
	}
	if task.takeWarmupSegmentStartFlag() {
		t.Fatal("expected subsequent warmup sentence to clear start flag")
	}
}