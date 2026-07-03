package edge

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"

	"github.com/difyz9/edge-tts-go/pkg/communicate"
)

// EdgeTTSProvider nhà cung cấp Edge TTS
// Hỗ trợ TTS một lần và TTS dạng luồng, đầu ra là khung Opus
// Tham số cấu hình: voice, rate, volume, pitch, connectTimeout, receiveTimeout
type EdgeTTSProvider struct {
	Voice          string
	Rate           string
	Volume         string
	Pitch          string
	ConnectTimeout int
	ReceiveTimeout int
}

// NewEdgeTTSProvider tạo EdgeTTSProvider
func NewEdgeTTSProvider(config map[string]interface{}) *EdgeTTSProvider {
	voice, _ := config["voice"].(string)
	rate, _ := config["rate"].(string)
	volume, _ := config["volume"].(string)
	pitch, _ := config["pitch"].(string)
	connectTimeout, _ := config["connect_timeout"].(int)
	receiveTimeout, _ := config["receive_timeout"].(int)
	if rate == "" {
		rate = "+0%"
	}
	if volume == "" {
		volume = "+0%"
	}
	if pitch == "" {
		pitch = "+0Hz"
	}
	if connectTimeout == 0 {
		connectTimeout = 10
	}
	if receiveTimeout == 0 {
		receiveTimeout = 60
	}
	return &EdgeTTSProvider{
		Voice:          voice,
		Rate:           rate,
		Volume:         volume,
		Pitch:          pitch,
		ConnectTimeout: connectTimeout,
		ReceiveTimeout: receiveTimeout,
	}
}

// TextToSpeech tổng hợp một lần, trả về khung Opus
func (p *EdgeTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	startTs := time.Now().UnixMilli()
	// File MP3 tạm thời
	tmpFile := fmt.Sprintf("/tmp/edge-tts-%d.mp3", time.Now().UnixNano())
	defer os.Remove(tmpFile)

	comm, err := communicate.NewCommunicate(
		text,
		p.Voice,
		p.Rate,
		p.Volume,
		p.Pitch,
		"", // proxy
		p.ConnectTimeout,
		p.ReceiveTimeout,
	)
	if err != nil {
		log.Errorf("Tạo EdgeTTS Communicate thất bại: %v", err)
		return nil, err
	}
	// Lưu MP3
	err = comm.Save(ctx, tmpFile, "")
	if err != nil {
		log.Errorf("EdgeTTS lưu MP3 thất bại: %v", err)
		return nil, err
	}
	// Chuyển MP3 sang Opus
	f, err := os.Open(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("Mở MP3 thất bại: %v", err)
	}
	defer f.Close()
	pipeReader, pipeWriter := io.Pipe()
	outputChan := make(chan []byte, 1000)
	// Ghi dữ liệu MP3 vào pipe
	go func() {
		_, _ = io.Copy(pipeWriter, f)
		pipeWriter.Close()
	}()
	mp3Decoder, err := util.CreateAudioDecoder(ctx, pipeReader, outputChan, frameDuration, "mp3")
	if err != nil {
		return nil, fmt.Errorf("Tạo bộ giải mã MP3 thất bại: %v", err)
	}
	var opusFrames [][]byte
	done := make(chan struct{})
	go func() {
		for frame := range outputChan {
			opusFrames = append(opusFrames, frame)
		}
		done <- struct{}{}
	}()
	if err := mp3Decoder.Run(startTs); err != nil {
		return nil, fmt.Errorf("Giải mã MP3 thất bại: %v", err)
	}
	<-done
	return opusFrames, nil
}

// TextToSpeechStream tổng hợp dạng luồng, trả về channel khung Opus
func (p *EdgeTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (chan []byte, error) {
	startTs := time.Now().UnixMilli()
	comm, err := communicate.NewCommunicate(
		text,
		p.Voice,
		p.Rate,
		p.Volume,
		p.Pitch,
		"", // proxy
		p.ConnectTimeout,
		p.ReceiveTimeout,
	)
	if err != nil {
		log.Errorf("Tạo EdgeTTS Communicate thất bại: %v", err)
		return nil, err
	}

	chunkChan, errChan := comm.Stream(ctx)
	outputChan := make(chan []byte, 100)
	pipeReader, pipeWriter := io.Pipe()
	// Bộ giải mã chuyển MP3 sang Opus
	go func() {
		defer func() {
			pipeWriter.Close()
			log.Debugf("EdgeTTS tổng hợp dạng luồng kết thúc, thời gian xử lý: %d ms", time.Now().UnixMilli()-startTs)
			if err := <-errChan; err != nil {
				log.Errorf("EdgeTTS tổng hợp dạng luồng gặp lỗi: %v", err)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				log.Debugf("EdgeTTS Stream context done, exit")
				return
			default:
				select {
				case chunk, ok := <-chunkChan:
					if !ok {
						log.Debugf("EdgeTTS Stream channel closed, exit")
						return
					}
					if chunk.Type == "audio" {
						_, _ = pipeWriter.Write(chunk.Data)
					}
				}
			}
		}

	}()
	// Khởi động giải mã MP3→Opus
	go func() {
		mp3Decoder, err := util.CreateAudioDecoder(ctx, pipeReader, outputChan, frameDuration, "mp3")
		if err != nil {
			log.Errorf("Tạo bộ giải mã MP3 của EdgeTTS thất bại: %v", err)
			return
		}
		if err := mp3Decoder.Run(startTs); err != nil {
			log.Errorf("EdgeTTS giải mã MP3 thất bại: %v", err)
		}
		log.Debugf("EdgeTTS giải mã MP3 kết thúc, thời gian xử lý: %d ms", time.Now().UnixMilli()-startTs)
	}()
	return outputChan, nil
}

// SetVoice thiết lập tham số âm sắc
func (p *EdgeTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	if voice, ok := voiceConfig["voice"].(string); ok && voice != "" {
		p.Voice = voice
		return nil
	}
	return fmt.Errorf("Cấu hình âm sắc không hợp lệ: thiếu voice")
}

// Close đóng tài nguyên (Provider không trạng thái, không cần đóng)
func (p *EdgeTTSProvider) Close() error {
	return nil
}

// IsValid kiểm tra tài nguyên có hợp lệ hay không
func (p *EdgeTTSProvider) IsValid() bool {
	return p != nil
}