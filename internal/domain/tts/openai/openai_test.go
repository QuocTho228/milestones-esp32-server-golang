package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"milestones-esp32-server-golang/internal/util"
)

func TestOpenAITTS(t *testing.T) {
	// Bỏ qua bài test gửi request thực tế qua mạng, trừ khi biến môi trường được thiết lập
	if os.Getenv("RUN_OPENAI_TEST") != "1" {
		t.Skip("Bỏ qua bài test OpenAI API, đặt biến môi trường RUN_OPENAI_TEST=1 để bật")
	}

	// Lấy API key từ biến môi trường
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Bỏ qua bài test OpenAI API, cần thiết lập biến môi trường OPENAI_API_KEY")
	}

	config := map[string]interface{}{
		"api_key":         apiKey,
		"api_url":         "https://api.openai.com/v1/audio/speech",
		"model":           "tts-1",
		"voice":           "alloy",
		"response_format": "mp3",
		"speed":           1.0,
		"frame_duration":  float64(60),
	}

	provider := NewOpenAITTSProvider(config)

	// Kiểm thử chuyển văn bản thành giọng nói
	t.Run("TestTextToSpeech", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		frames, err := provider.TextToSpeech(ctx, "Hello, this is a test of OpenAI text to speech.", 16000, 1, 60)
		if err != nil {
			t.Fatalf("TextToSpeech thất bại: %v", err)
		}

		if len(frames) == 0 {
			t.Error("không trả về bất kỳ khung âm thanh nào")
		}

		t.Logf("Tạo thành công %d khung âm thanh", len(frames))
	})

	// Kiểm thử chuyển văn bản thành giọng nói dạng stream
	t.Run("TestTextToSpeechStream", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outputChan, err := provider.TextToSpeechStream(ctx, "Hello, this is a test of OpenAI streaming text to speech.", 16000, 1, 60)
		if err != nil {
			t.Fatalf("TextToSpeechStream thất bại: %v", err)
		}

		// Nhận tất cả các khung
		var receivedFrames [][]byte
		timeout := time.After(20 * time.Second)

	receiveLoop:
		for {
			select {
			case frame, ok := <-outputChan:
				if !ok {
					break receiveLoop
				}
				receivedFrames = append(receivedFrames, frame)
			case <-timeout:
				t.Error("hết thời gian chờ nhận khung âm thanh")
				break receiveLoop
			}
		}

		if len(receivedFrames) == 0 {
			t.Error("không nhận được bất kỳ khung âm thanh nào")
		}

		t.Logf("Nhận thành công %d khung âm thanh", len(receivedFrames))
	})

	// Kiểm thử các giọng nói khác nhau
	t.Run("TestDifferentVoices", func(t *testing.T) {
		voices := []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"}

		for _, voice := range voices {
			t.Run(voice, func(t *testing.T) {
				config := map[string]interface{}{
					"api_key":         apiKey,
					"model":           "tts-1",
					"voice":           voice,
					"response_format": "mp3",
					"speed":           1.0,
				}

				provider := NewOpenAITTSProvider(config)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				frames, err := provider.TextToSpeech(ctx, "Testing voice: "+voice, 16000, 1, 60)
				if err != nil {
					t.Errorf("sử dụng giọng nói %s thất bại: %v", voice, err)
					return
				}

				if len(frames) == 0 {
					t.Errorf("giọng nói %s không trả về bất kỳ khung âm thanh nào", voice)
				}

				t.Logf("Giọng nói %s tạo thành công %d khung âm thanh", voice, len(frames))
			})
		}
	})

	// Kiểm thử các tốc độ khác nhau
	t.Run("TestDifferentSpeeds", func(t *testing.T) {
		speeds := []float64{0.5, 1.0, 1.5, 2.0}

		for _, speed := range speeds {
			t.Run(string(rune(speed)), func(t *testing.T) {
				config := map[string]interface{}{
					"api_key":         apiKey,
					"model":           "tts-1",
					"voice":           "alloy",
					"response_format": "mp3",
					"speed":           speed,
				}

				provider := NewOpenAITTSProvider(config)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				frames, err := provider.TextToSpeech(ctx, "Testing speed", 16000, 1, 60)
				if err != nil {
					t.Errorf("sử dụng tốc độ %.1f thất bại: %v", speed, err)
					return
				}

				if len(frames) == 0 {
					t.Errorf("tốc độ %.1f không trả về bất kỳ khung âm thanh nào", speed)
				}

				t.Logf("Tốc độ %.1f tạo thành công %d khung âm thanh", speed, len(frames))
			})
		}
	})
}

// TestOpenAITTSProviderDefaults kiểm thử các giá trị mặc định
func TestOpenAITTSProviderDefaults(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	provider := NewOpenAITTSProvider(config)

	if provider.APIURL != "https://api.openai.com/v1/audio/speech" {
		t.Errorf("kỳ vọng API URL mặc định là https://api.openai.com/v1/audio/speech, thực tế là %s", provider.APIURL)
	}

	if provider.Model != "tts-1" {
		t.Errorf("kỳ vọng model mặc định là tts-1, thực tế là %s", provider.Model)
	}

	if provider.Voice != "alloy" {
		t.Errorf("kỳ vọng giọng nói mặc định là alloy, thực tế là %s", provider.Voice)
	}

	if provider.ResponseFormat != "mp3" {
		t.Errorf("kỳ vọng định dạng phản hồi mặc định là mp3, thực tế là %s", provider.ResponseFormat)
	}

	if provider.Speed != 1.0 {
		t.Errorf("kỳ vọng tốc độ mặc định là 1.0, thực tế là %.1f", provider.Speed)
	}
}

func TestOpenAITTSProviderSupportsOpusResponse(t *testing.T) {
	sampleRate := 16000
	pcm := make([]int16, sampleRate/2)
	for i := range pcm {
		if i%32 < 16 {
			pcm[i] = 2400
		} else {
			pcm[i] = -2400
		}
	}

	opusBytes, err := util.PCM16ToOggOpus(pcm, sampleRate, 1, 20)
	if err != nil {
		t.Fatalf("tạo dữ liệu test Ogg Opus thất bại: %v", err)
	}

	requestErrCh := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			requestErrCh <- err
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.ResponseFormat != "opus" {
			requestErrCh <- fmt.Errorf("kỳ vọng response_format=opus, thực tế là %s", req.ResponseFormat)
			http.Error(w, "unexpected response_format", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "audio/ogg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(opusBytes)
	}))
	defer server.Close()

	provider := NewOpenAITTSProvider(map[string]interface{}{
		"api_url":         server.URL,
		"model":           "tts-1",
		"voice":           "alloy",
		"response_format": "opus",
		"speed":           1.0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	outputChan, err := provider.TextToSpeechStream(ctx, "测试 opus 输出", sampleRate, 1, 60)
	if err != nil {
		t.Fatalf("TextToSpeechStream trả về lỗi: %v", err)
	}

	frameCount := 0
	for frame := range outputChan {
		if len(frame) == 0 {
			t.Fatal("nhận được khung Opus rỗng")
		}
		frameCount++
	}

	if frameCount == 0 {
		t.Fatal("không nhận được bất kỳ khung Opus nào")
	}

	select {
	case err := <-requestErrCh:
		t.Fatalf("kiểm tra mock server thất bại: %v", err)
	default:
	}
}