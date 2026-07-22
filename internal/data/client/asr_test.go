package client

import (
	"context"
	"errors"
	"testing"

	asr_types "milestones-esp32-server-golang/internal/domain/asr/types"
)

func TestRetireAsrResult_RetryableErrorUsesRetryReason(t *testing.T) {
	retryErr := errors.New("provider retryable error")
	a := &Asr{
		AsrResultChannel: make(chan asr_types.StreamingResult, 1),
	}
	a.AsrResultChannel <- asr_types.StreamingResult{
		Error:       retryErr,
		RetryReason: asr_types.RetryReasonXunfeiServiceInstanceInvalid,
		IsFinal:     true,
	}

	result, shouldContinue, err := a.RetireAsrResult(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !shouldContinue {
		t.Fatalf("expected shouldContinue to be true")
	}
	if result.RetryReason != asr_types.RetryReasonXunfeiServiceInstanceInvalid {
		t.Fatalf("expected retry reason %q, got %q", asr_types.RetryReasonXunfeiServiceInstanceInvalid, result.RetryReason)
	}
	if !errors.Is(result.Error, retryErr) {
		t.Fatalf("expected original error to be preserved")
	}
}

func TestRetireAsrResult_NonRetryableErrorReturnsError(t *testing.T) {
	fatalErr := errors.New("fatal provider error")
	a := &Asr{
		AsrResultChannel: make(chan asr_types.StreamingResult, 1),
	}
	a.AsrResultChannel <- asr_types.StreamingResult{
		Error: fatalErr,
	}

	_, shouldContinue, err := a.RetireAsrResult(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, fatalErr) {
		t.Fatalf("expected original error, got %v", err)
	}
	if shouldContinue {
		t.Fatalf("expected shouldContinue to be false")
	}
}

func TestRetireAsrResult_NonFinalResultsOnlyTriggerFirstTextOnce(t *testing.T) {
	var firstTexts []string
	a := &Asr{
		AsrResultChannel: make(chan asr_types.StreamingResult, 4),
		ClientState: &ClientState{
			OnAsrFirstTextCallback: func(text string, isFinal bool) {
				firstTexts = append(firstTexts, text)
			},
		},
	}
	a.AsrResultChannel <- asr_types.StreamingResult{Text: "Bạn đang", IsFinal: false}
	a.AsrResultChannel <- asr_types.StreamingResult{Text: "Bạn đang làm gì đấy", IsFinal: false}
	a.AsrResultChannel <- asr_types.StreamingResult{Text: "Bạn đang làm gì đấy", IsFinal: false}
	a.AsrResultChannel <- asr_types.StreamingResult{Text: "Bạn đang làm gì đấy?", IsFinal: true}

	result, shouldContinue, err := a.RetireAsrResult(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !shouldContinue {
		t.Fatalf("expected shouldContinue to be true")
	}
	if result.Text != "Bạn đang làm gì đấy?" {
		t.Fatalf("expected final text %q, got %q", "Bạn đang làm gì đấy?", result.Text)
	}
	if len(firstTexts) != 1 || firstTexts[0] != "Bạn đang" {
		t.Fatalf("unexpected first text callbacks: %v", firstTexts)
	}
}

func TestRetireAsrResult_FinalOnlyStillTriggersFirstText(t *testing.T) {
	var firstText string
	var firstIsFinal bool
	a := &Asr{
		AsrResultChannel: make(chan asr_types.StreamingResult, 1),
		ClientState: &ClientState{
			OnAsrFirstTextCallback: func(text string, isFinal bool) {
				firstText = text
				firstIsFinal = isFinal
			},
		},
	}
	a.AsrResultChannel <- asr_types.StreamingResult{Text: "Văn bản cuối cùng", IsFinal: true}

	result, shouldContinue, err := a.RetireAsrResult(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !shouldContinue {
		t.Fatalf("expected shouldContinue to be true")
	}
	if result.Text != "Văn bản cuối cùng" {
		t.Fatalf("expected final text %q, got %q", "Văn bản cuối cùng", result.Text)
	}
	if firstText != "Văn bản cuối cùng" || !firstIsFinal {
		t.Fatalf("unexpected first text callback: text=%q, isFinal=%v", firstText, firstIsFinal)
	}
}

func TestRetireAsrResult_Funasr2PassOnlineMarkedFinalStillWaitsForOfflineFinal(t *testing.T) {
	var firstTexts []string
	a := &Asr{
		AsrType:          "funasr",
		Mode:             "2pass",
		AsrResultChannel: make(chan asr_types.StreamingResult, 2),
		ClientState: &ClientState{
			OnAsrFirstTextCallback: func(text string, isFinal bool) {
				firstTexts = append(firstTexts, text)
			},
		},
	}
	a.AsrResultChannel <- asr_types.StreamingResult{Text: "Bạn đang", IsFinal: true, Mode: "2pass-online"}
	a.AsrResultChannel <- asr_types.StreamingResult{Text: "Bạn đang làm gì đấy.", IsFinal: true, Mode: "2pass-offline"}

	result, shouldContinue, err := a.RetireAsrResult(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !shouldContinue {
		t.Fatalf("expected shouldContinue to be true")
	}
	if result.Text != "Bạn đang làm gì đấy." {
		t.Fatalf("expected final text %q, got %q", "Bạn đang làm gì đấy.", result.Text)
	}
	if len(firstTexts) != 1 || firstTexts[0] != "Bạn đang" {
		t.Fatalf("unexpected first text callbacks: %v", firstTexts)
	}
}