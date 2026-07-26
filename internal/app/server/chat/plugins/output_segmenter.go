package plugins

import (
	"regexp"
	"strings"

	"github.com/cloudwego/eino/schema"
	"milestones-esp32-server-golang/internal/domain/chat/streamtransform"
	"milestones-esp32-server-golang/internal/util"
)

const (
	defaultOutputSegmenterMinLen = 2
	defaultOutputSegmenterMaxLen = 100
)

// emotionTagRe khớp tag cảm xúc dạng "[happy] " ở đầu câu, ví dụ: "[happy] Xin chào!"
var emotionTagRe = regexp.MustCompile(`^\s*\[([a-z_]+)\]\s*`)

// strayBracketTagRe bắt MỌI cụm dạng [xxx] hoặc [/xxx] còn sót lại ở bất kỳ đâu trong câu
// (không chỉ đầu câu) — phòng trường hợp model tự sinh tag đóng như "[/thinking]" khi đang
// suy luận trước khi gọi tool, khiến cụm này lọt qua extractEmotionTag (chỉ bắt tag mở ở đầu
// câu) và bị đọc thành tiếng nguyên văn qua TTS. Strip vô điều kiện, không cần validEmotions,
// vì đây là tag rác không nên xuất hiện trong lời nói dù đúng hay sai định dạng.
var strayBracketTagRe = regexp.MustCompile(`\[/?[a-zA-Z_]*\]`)

// stripStrayTags loại bỏ mọi cụm ngoặc vuông còn sót (bao gồm tag đóng, tag rác giữa câu)
// khỏi text SAU KHI đã tách tag emotion hợp lệ ở đầu câu qua extractEmotionTag.
func stripStrayTags(text string) string {
	if !strings.ContainsAny(text, "[]") {
		return text
	}
	return strings.TrimSpace(strayBracketTagRe.ReplaceAllString(text, ""))
}

// validEmotions danh sách 21 emotion hợp lệ, khớp chính xác với font_awesome.py (firmware)
var validEmotions = map[string]bool{
	"neutral": true, "happy": true, "laughing": true, "funny": true,
	"sad": true, "angry": true, "crying": true, "loving": true,
	"embarrassed": true, "surprised": true, "shocked": true, "thinking": true,
	"winking": true, "cool": true, "relaxed": true, "delicious": true,
	"kissy": true, "confident": true, "sleepy": true, "silly": true,
	"confused": true,
}

// extractEmotionTag tách tag [emotion] ở đầu câu nếu có và hợp lệ.
// Trả về (text đã bỏ tag, tên emotion hoặc "" nếu không có/không hợp lệ).
// Nếu tag không nằm trong danh sách hợp lệ, KHÔNG strip để tránh mất nghĩa câu do LLM viết nhầm định dạng.
func extractEmotionTag(sentence string) (string, string) {
	m := emotionTagRe.FindStringSubmatch(sentence)
	if m == nil {
		return sentence, ""
	}
	emotion := m[1]
	if !validEmotions[emotion] {
		return sentence, ""
	}
	return strings.TrimPrefix(sentence, m[0]), emotion
}

// cloneMetaWithEmotion tạo bản sao Meta (tránh sửa map gốc đang được nhiều Item dùng chung)
// rồi gắn thêm key "emotion" nếu tách được tag hợp lệ.
func cloneMetaWithEmotion(meta map[string]any, emotion string) map[string]any {
	out := make(map[string]any, len(meta)+1)
	for k, v := range meta {
		out[k] = v
	}
	if emotion != "" {
		out["emotion"] = emotion
	}
	return out
}

type outputSegmenterFactory struct{}

func (f outputSegmenterFactory) Name() string {
	return "output_segmenter"
}

func (f outputSegmenterFactory) Priority() int {
	return 200
}

func (f outputSegmenterFactory) New(ctx streamtransform.Context) (streamtransform.Transformer, error) {
	return &outputSegmenterTransformer{
		minLen:  defaultOutputSegmenterMinLen,
		maxLen:  defaultOutputSegmenterMaxLen,
		isFirst: true,
	}, nil
}

type outputSegmenterTransformer struct {
	textBuffer       strings.Builder
	pendingToolCalls []schema.ToolCall
	minLen           int
	maxLen           int
	isFirst          bool
}

func (t *outputSegmenterTransformer) Transform(item streamtransform.Item) (streamtransform.Result, error) {
	switch item.Kind {
	case streamtransform.ItemKindTextDelta:
		return t.transformText(item), nil
	case streamtransform.ItemKindToolCalls:
		return t.transformToolCalls(item), nil
	default:
		out := t.flushPendingText(item.Meta, true, false)
		out = append(out, t.flushPendingToolCalls(item.Meta, false)...)
		out = append(out, item)
		return streamtransform.Result{Items: out}, nil
	}
}

func (t *outputSegmenterTransformer) Close() error {
	t.textBuffer.Reset()
	t.pendingToolCalls = nil
	return nil
}

func (t *outputSegmenterTransformer) transformText(item streamtransform.Item) streamtransform.Result {
	out := t.flushPendingToolCalls(item.Meta, false)

	if item.Text != "" {
		t.textBuffer.WriteString(item.Text)
	}

	if item.Text != "" && util.ContainsSentenceSeparator(item.Text, t.isFirst) {
		sentences, remaining := util.ExtractSmartSentences(t.textBuffer.String(), t.minLen, t.maxLen, t.isFirst)
		t.textBuffer.Reset()
		t.textBuffer.WriteString(remaining)
		for _, sentence := range sentences {
			if strings.TrimSpace(sentence) == "" {
				continue
			}
			cleanText, emotion := extractEmotionTag(sentence)
			cleanText = stripStrayTags(cleanText)
			if strings.TrimSpace(cleanText) == "" {
				continue
			}
			out = append(out, streamtransform.Item{
				Kind: streamtransform.ItemKindTextSegment,
				Text: cleanText,
				Meta: cloneMetaWithEmotion(item.Meta, emotion),
			})
			t.isFirst = false
		}
	}

	if !item.IsEnd {
		return streamtransform.Result{Items: out}
	}

	out = append(out, t.flushPendingText(item.Meta, true, true)...)
	if len(out) > 0 {
		last := len(out) - 1
		out[last].IsEnd = true
		return streamtransform.Result{Items: out}
	}

	out = append(out, streamtransform.Item{
		Kind:  streamtransform.ItemKindTextSegment,
		IsEnd: true,
		Meta:  item.Meta,
	})
	return streamtransform.Result{Items: out}
}

func (t *outputSegmenterTransformer) transformToolCalls(item streamtransform.Item) streamtransform.Result {
	out := t.flushPendingText(item.Meta, true, false)
	if len(item.ToolCalls) > 0 {
		t.pendingToolCalls = append(t.pendingToolCalls, item.ToolCalls...)
	}
	if item.IsEnd {
		out = append(out, t.flushPendingToolCalls(item.Meta, true)...)
		if len(out) > 0 {
			out[len(out)-1].IsEnd = true
			return streamtransform.Result{Items: out}
		}
	}
	return streamtransform.Result{Items: out}
}

func (t *outputSegmenterTransformer) flushPendingText(meta map[string]any, force bool, isEnd bool) []streamtransform.Item {
	buffered := strings.TrimSpace(t.textBuffer.String())
	if buffered == "" {
		if isEnd {
			t.textBuffer.Reset()
		}
		return nil
	}

	if !force {
		return nil
	}

	t.textBuffer.Reset()
	t.isFirst = false
	cleanText, emotion := extractEmotionTag(buffered)
	cleanText = stripStrayTags(cleanText)
	if cleanText == "" {
		// buffered chỉ chứa tag rác (ví dụ "[/thinking]"), không còn nội dung để nói.
		// Trả về nil để nơi gọi (transformText dòng 153) rơi vào fallback emit item rỗng có IsEnd=true.
		return nil
	}
	return []streamtransform.Item{{
		Kind:  streamtransform.ItemKindTextSegment,
		Text:  cleanText,
		IsEnd: isEnd,
		Meta:  cloneMetaWithEmotion(meta, emotion),
	}}
}

func (t *outputSegmenterTransformer) flushPendingToolCalls(meta map[string]any, isEnd bool) []streamtransform.Item {
	if len(t.pendingToolCalls) == 0 {
		return nil
	}

	toolCalls := append([]schema.ToolCall(nil), t.pendingToolCalls...)
	t.pendingToolCalls = nil
	return []streamtransform.Item{{
		Kind:      streamtransform.ItemKindToolCalls,
		ToolCalls: toolCalls,
		IsEnd:     isEnd,
		Meta:      meta,
	}}
}

func RegisterOutputSegmenter(registry *streamtransform.Registry) {
	if registry == nil {
		return
	}
	registry.Register(outputSegmenterFactory{})
}