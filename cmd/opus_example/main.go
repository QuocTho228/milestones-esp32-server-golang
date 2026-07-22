package main

import (
	"fmt"
	"os"

	"github.com/hraban/opus"
)

func main() {
	// Thiết lập tham số âm thanh
	channels := 1
	sampleRate := 16000 // 16kHz
	fmt.Printf("Số kênh: %d, Tần số lấy mẫu: %d Hz\n", channels, sampleRate)

	// Tạo một bộ mã hóa (encoder), chỉ định loại ứng dụng là VoIP (thoại độ trễ thấp)
	enc, err := opus.NewEncoder(sampleRate, channels, opus.AppVoIP)
	if err != nil {
		fmt.Printf("Tạo bộ mã hóa thất bại: %v\n", err)
		os.Exit(1)
	}

	// Thiết lập bitrate là 16kbps
	if err = enc.SetBitrate(16000); err != nil {
		fmt.Printf("Thiết lập bitrate thất bại: %v\n", err)
		os.Exit(1)
	}

	// Thiết lập độ phức tạp (complexity), trong khoảng 0-10, giá trị càng cao chất lượng càng tốt nhưng tốn nhiều CPU hơn
	if err = enc.SetComplexity(5); err != nil {
		fmt.Printf("Thiết lập độ phức tạp thất bại: %v\n", err)
		os.Exit(1)
	}

	// Tạo dữ liệu PCM thử nghiệm dài 20ms (mỗi khung 20ms, tần số lấy mẫu 16kHz = 320 mẫu)
	frameSize := 320
	pcm := make([]int16, frameSize*channels)

	// Tạo một sóng sin đơn giản để kiểm thử
	for i := 0; i < frameSize; i++ {
		// Sóng sin đơn giản, tần số khoảng 440Hz
		value := int16(10000.0 * float64(i%36) / 36.0)
		pcm[i] = value
	}

	// Dùng để lưu trữ dữ liệu sau khi mã hóa
	data := make([]byte, 1000)

	// Mã hóa dữ liệu PCM thành Opus
	n, err := enc.Encode(pcm, data)
	if err != nil {
		fmt.Printf("Mã hóa thất bại: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Đã mã hóa %d mẫu thành %d byte dữ liệu Opus, tỷ lệ nén: %.2f%%\n",
		frameSize*channels, n, float64(n)/float64(frameSize*channels*2)*100)

	// Tạo bộ giải mã (decoder) để kiểm thử giải mã
	dec, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		fmt.Printf("Tạo bộ giải mã thất bại: %v\n", err)
		os.Exit(1)
	}

	// Dùng để lưu trữ dữ liệu PCM sau khi giải mã
	decodedPCM := make([]int16, frameSize*channels)

	// Giải mã dữ liệu Opus thành PCM
	samplesDecoded, err := dec.Decode(data[:n], decodedPCM)
	if err != nil {
		fmt.Printf("Giải mã thất bại: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Đã giải mã %d byte dữ liệu Opus thành %d mẫu\n", n, samplesDecoded)

	// Tính toán sự chênh lệch giữa PCM gốc và PCM sau khi giải mã
	var sumDiff int64
	for i := 0; i < frameSize; i++ {
		diff := int64(pcm[i]) - int64(decodedPCM[i])
		if diff < 0 {
			diff = -diff
		}
		sumDiff += diff
	}
	avgDiff := float64(sumDiff) / float64(frameSize)

	fmt.Printf("Chênh lệch trung bình giữa PCM gốc và PCM đã giải mã: %.2f\n", avgDiff)
	fmt.Println("Ví dụ mã hóa/giải mã Opus đã hoàn tất!")
}