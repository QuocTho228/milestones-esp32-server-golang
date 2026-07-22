package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/hackers365/silero-vad-go/speech"
	"gopkg.in/hraban/opus.v2"
)

// readCloserWrapper cung cấp phương thức Close cho bytes.Reader để hiện thực interface ReadCloser
type readCloserWrapper struct {
	*bytes.Reader
}

// Close hiện thực interface io.Closer
func (r *readCloserWrapper) Close() error {
	return nil
}

// newReadCloserWrapper tạo một ReadCloser wrapper mới
func newReadCloserWrapper(data []byte) *readCloserWrapper {
	return &readCloserWrapper{bytes.NewReader(data)}
}

// WavToOpus chuyển đổi dữ liệu audio WAV sang định dạng Opus chuẩn
// Trả về tập hợp các slice khung Opus, mỗi slice là một khung được mã hóa Opus
func WavToOpus(wavData []byte, sampleRate int, channels int, bitRate int) ([][]byte, error) {

	sd, err := speech.NewDetector(speech.DetectorConfig{
		ModelPath:            "silero_vad.onnx",
		SampleRate:           16000,
		Threshold:            0.5,
		MinSilenceDurationMs: 250,
		SpeechPadMs:          150,
	})
	if err != nil {
		log.Fatalf("failed to create speech detector: %s", err)
	}

	// Tạo bộ giải mã WAV
	wavReader := bytes.NewReader(wavData)
	wavDecoder := wav.NewDecoder(wavReader)
	if !wavDecoder.IsValidFile() {
		return nil, fmt.Errorf("file WAV không hợp lệ")
	}

	// Đọc thông tin file WAV
	wavDecoder.ReadInfo()
	format := wavDecoder.Format()
	wavSampleRate := int(format.SampleRate)
	wavChannels := int(format.NumChannels)

	// Nếu tham số truyền vào không khớp với tham số của file, dùng tham số trong file
	if sampleRate == 0 {
		sampleRate = wavSampleRate
	}
	if channels == 0 {
		channels = wavChannels
	}

	// In thông tin wavDecoder
	fmt.Println("Định dạng WAV:", format)

	enc, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
	if err != nil {
		return nil, fmt.Errorf("tạo bộ mã hóa Opus thất bại: %v", err)
	}

	// Lưu ý: thông báo lỗi bên dưới trong bản gốc ghi nhầm là "tạo bộ mã hóa" dù đang tạo bộ GIẢI mã (decoder).
	// Đây là lỗi có sẵn trong code gốc, được giữ nguyên logic, chỉ dịch nội dung.
	dec, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("tạo bộ mã hóa Opus thất bại: %v", err)
	}

	// Thiết lập bitrate
	if bitRate > 0 {
		if err := enc.SetBitrate(bitRate); err != nil {
			return nil, fmt.Errorf("thiết lập bitrate thất bại: %v", err)
		}
	}

	// Tạo mảng slice khung đầu ra
	opusFrames := make([][]byte, 0)

	perFrameDuration := 60
	// Buffer PCM - kích thước khung Opus (60ms)
	frameSize := sampleRate * perFrameDuration / 1000
	pcmBuffer := make([]int16, frameSize*channels)
	pcmBufferFloat32 := make([]float32, frameSize*channels)
	opusBuffer := make([]byte, 1000) // Buffer đủ lớn để lưu dữ liệu đã mã hóa

	// Đọc buffer audio
	audioBuf := &audio.IntBuffer{Data: make([]int, frameSize*channels), Format: format}

	fmt.Println("Bắt đầu chuyển đổi...")

	pcmAllData := make([]float32, 0)
	for {
		// Đọc dữ liệu WAV
		n, err := wavDecoder.PCMBuffer(audioBuf)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("đọc dữ liệu WAV thất bại: %v", err)
		}

		// Chuyển int sang int16
		for i := 0; i < len(audioBuf.Data); i++ {
			if i < len(pcmBuffer) {
				pcmBuffer[i] = int16(audioBuf.Data[i])
			}
		}

		// Mã hóa sang định dạng Opus
		n, err = enc.Encode(pcmBuffer, opusBuffer)
		if err != nil {
			return nil, fmt.Errorf("mã hóa thất bại: %v", err)
		}

		// Sao chép khung hiện tại vào slice mới và thêm vào mảng khung
		frameData := make([]byte, n)
		copy(frameData, opusBuffer[:n])
		opusFrames = append(opusFrames, frameData)

		// Giải mã Opus về PCM
		n, err = dec.DecodeFloat32(frameData, pcmBufferFloat32)
		if err != nil {
			return nil, fmt.Errorf("giải mã thất bại: %v", err)
		}

		fmt.Printf("độ dài pcmBufferFloat32: %d\n", len(pcmBufferFloat32[:n]))

		segments, err := sd.Detect(pcmBufferFloat32[:n])
		if err != nil {
			//log.Fatalf("Detect failed: %s", err)
		}
		fmt.Printf("phát hiện giọng nói: %v\n", segments)

		pcmAllData = append(pcmAllData, pcmBufferFloat32[:n]...)
	}

	segments, err := sd.Detect(pcmAllData)
	if err != nil {
		log.Fatalf("Detect failed: %s", err)
	}
	fmt.Printf("phát hiện giọng nói: %v\n", segments)

	// Ghi frameData ra file test.opus
	opusFile, err := os.OpenFile("output.opus", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to create opus file: %s", err)
	}
	opusFile.Write(opusFrames[0])
	opusFile.Close()

	/*
		// Ghi dữ liệu pcm ra file test.pcm
		pcmFile, err := os.OpenFile("test.pcm", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("failed to create pcm file: %s", err)
		}

		defer pcmFile.Close()
		dec, err := opus.NewDecoder(sampleRate, channels)
		if err != nil {
			return nil, fmt.Errorf("tạo bộ giải mã Opus thất bại: %v", err)
		}

		pcmBuffer = make([]int16, 10240)
		for _, data := range opusFrames {
			// Giải mã dữ liệu opus thành pcm
			n, err := dec.Decode(data, pcmBuffer)
			if err != nil {
				return nil, fmt.Errorf("giải mã thất bại: %v", err)
			}
			frameData := make([]int16, len(pcmBuffer)*2)
			copy(frameData, pcmBuffer[:n])
			_, err = pcmFile.Write(frameData)
			if err != nil {
				log.Fatalf("failed to write to pcm file: %s", err)
			}
		}*/

	return opusFrames, nil
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("invalid arguments provided: expecting one file path")
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("failed to open sample audio file: %s", err)
	}
	defer f.Close()

	// Đọc toàn bộ nội dung file
	mp3Data, err := io.ReadAll(f)
	if err != nil {
		log.Fatalf("failed to read mp3 file: %s", err)
	}

	// Chuyển đổi mp3 sang opus
	opusData, err := WavToOpus(mp3Data, 16000, 1, 0)
	if err != nil {
		log.Fatalf("failed to convert mp3 to opus: %s", err)
	}

	// In dữ liệu opus
	fmt.Printf("opusData: %d\n", len(opusData))

	// Giải mã dữ liệu Opus thành pcm

	// Ghi tất cả dữ liệu ra file test.opus
	/*opusFile, err := os.OpenFile("test.opus", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to create opus file: %s", err)
	}
	defer opusFile.Close()

	for _, data := range opusData {
		_, err := opusFile.Write(data)
		if err != nil {
			log.Fatalf("failed to write to opus file: %s", err)
		}
	}*/
}