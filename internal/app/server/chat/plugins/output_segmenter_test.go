package plugins

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"milestones-esp32-server-golang/internal/domain/chat/streamtransform"
)

func TestOutputSegmenterFlushesOnEnd(t *testing.T) {
	transformer, err := outputSegmenterFactory{}.New(streamtransform.Context{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	out, err := transformer.Transform(streamtransform.Item{
		Kind: streamtransform.ItemKindTextDelta,
		Text: "Xin chào, thế giới",
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("len(out.Items) = %d, want 1", len(out.Items))
	}
	if got := out.Items[0].Text; got != "Xin chào," {
		t.Fatalf("out.Items[0].Text = %q, want %q", got, "Xin chào,")
	}
	if out.Items[0].IsEnd {
		t.Fatalf("out.Items[0].IsEnd = true, want false")
	}

	out, err = transformer.Transform(streamtransform.Item{
		Kind:  streamtransform.ItemKindTextDelta,
		IsEnd: true,
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("len(out.Items) = %d, want 1", len(out.Items))
	}
	if got := out.Items[0].Text; got != " thế giới" {
		t.Fatalf("out.Items[0].Text = %q, want %q", got, " thế giới")
	}
	if !out.Items[0].IsEnd {
		t.Fatalf("out.Items[0].IsEnd = false, want true")
	}
}

func TestOutputSegmenterEmitsEmptyEndWhenNoRemainder(t *testing.T) {
	transformer, err := outputSegmenterFactory{}.New(streamtransform.Context{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	out, err := transformer.Transform(streamtransform.Item{
		Kind: streamtransform.ItemKindTextDelta,
		Text: "Xin chào.",
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("len(out.Items) = %d, want 1", len(out.Items))
	}
	if got := out.Items[0].Text; got != "Xin chào." {
		t.Fatalf("out.Items[0].Text = %q, want %q", got, "Xin chào.")
	}

	out, err = transformer.Transform(streamtransform.Item{
		Kind:  streamtransform.ItemKindTextDelta,
		IsEnd: true,
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("len(out.Items) = %d, want 1", len(out.Items))
	}
	if got := out.Items[0].Text; got != "" {
		t.Fatalf("out.Items[0].Text = %q, want empty", got)
	}
	if !out.Items[0].IsEnd {
		t.Fatalf("out.Items[0].IsEnd = false, want true")
	}
}

func TestOutputSegmenterFlushesBufferedTextBeforeToolCalls(t *testing.T) {
	transformer, err := outputSegmenterFactory{}.New(streamtransform.Context{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	out, err := transformer.Transform(streamtransform.Item{
		Kind: streamtransform.ItemKindTextDelta,
		Text: "Nửa câu chưa xong",
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}
	if len(out.Items) != 0 {
		t.Fatalf("len(out.Items) = %d, want 0", len(out.Items))
	}

	out, err = transformer.Transform(streamtransform.Item{
		Kind: streamtransform.ItemKindToolCalls,
		ToolCalls: []schema.ToolCall{
			{ID: "call_1", Type: "function", Function: schema.FunctionCall{Name: "weather", Arguments: `{"city":"shanghai"}`}},
		},
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("len(out.Items) = %d, want 1", len(out.Items))
	}
	if out.Items[0].Kind != streamtransform.ItemKindTextSegment {
		t.Fatalf("out.Items[0].Kind = %q, want %q", out.Items[0].Kind, streamtransform.ItemKindTextSegment)
	}
	if got := out.Items[0].Text; got != "Nửa câu chưa xong" {
		t.Fatalf("out.Items[0].Text = %q, want %q", got, "Nửa câu chưa xong")
	}

	out, err = transformer.Transform(streamtransform.Item{
		Kind:  streamtransform.ItemKindTextDelta,
		IsEnd: true,
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("len(out.Items) = %d, want 1", len(out.Items))
	}
	if out.Items[0].Kind != streamtransform.ItemKindToolCalls {
		t.Fatalf("out.Items[0].Kind = %q, want %q", out.Items[0].Kind, streamtransform.ItemKindToolCalls)
	}
	if len(out.Items[0].ToolCalls) != 1 {
		t.Fatalf("len(out.Items[0].ToolCalls) = %d, want 1", len(out.Items[0].ToolCalls))
	}
	if !out.Items[0].IsEnd {
		t.Fatalf("out.Items[0].IsEnd = false, want true")
	}
}

func TestOutputSegmenterAggregatesToolCallsUntilBoundary(t *testing.T) {
	transformer, err := outputSegmenterFactory{}.New(streamtransform.Context{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for _, tc := range []schema.ToolCall{
		{ID: "call_1", Type: "function", Function: schema.FunctionCall{Name: "weather", Arguments: `{"city":"shanghai"}`}},
		{ID: "call_2", Type: "function", Function: schema.FunctionCall{Name: "clock", Arguments: `{"timezone":"Asia/Shanghai"}`}},
	} {
		out, err := transformer.Transform(streamtransform.Item{
			Kind:      streamtransform.ItemKindToolCalls,
			ToolCalls: []schema.ToolCall{tc},
		})
		if err != nil {
			t.Fatalf("Transform() error = %v", err)
		}
		if len(out.Items) != 0 {
			t.Fatalf("len(out.Items) = %d, want 0", len(out.Items))
		}
	}

	out, err := transformer.Transform(streamtransform.Item{
		Kind: streamtransform.ItemKindTextDelta,
		Text: "Tiếp tục trả lời.",
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("len(out.Items) = %d, want 2", len(out.Items))
	}
	if out.Items[0].Kind != streamtransform.ItemKindToolCalls {
		t.Fatalf("out.Items[0].Kind = %q, want %q", out.Items[0].Kind, streamtransform.ItemKindToolCalls)
	}
	if len(out.Items[0].ToolCalls) != 2 {
		t.Fatalf("len(out.Items[0].ToolCalls) = %d, want 2", len(out.Items[0].ToolCalls))
	}
	if out.Items[1].Kind != streamtransform.ItemKindTextSegment {
		t.Fatalf("out.Items[1].Kind = %q, want %q", out.Items[1].Kind, streamtransform.ItemKindTextSegment)
	}
	if got := out.Items[1].Text; got != "Tiếp tục trả lời." {
		t.Fatalf("out.Items[1].Text = %q, want %q", got, "Tiếp tục trả lời.")
	}
}