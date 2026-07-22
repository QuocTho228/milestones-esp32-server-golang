package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"

	"milestones-esp32-server-golang/internal/domain/asr/funasr"
)

// readWavFile đọc file WAV và chuyển đổi thành dữ liệu PCM []float32
func readWavFile(filePath string) ([]float32, error) {
	// Mở file WAV
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("mở file WAV thất bại: %v", err)
	}
	defer file.Close()

	// Tạo bộ giải mã (decoder) WAV
	wavDecoder := wav.NewDecoder(file)
	if !wavDecoder.IsValidFile() {
		return nil, fmt.Errorf("file WAV không hợp lệ")
	}

	// Đọc thông tin file WAV
	wavDecoder.ReadInfo()
	format := wavDecoder.Format()

	fmt.Printf("Định dạng WAV: tần số lấy mẫu=%dHz, số kênh=%d\n",
		int(format.SampleRate), format.NumChannels)

	// Đọc toàn bộ dữ liệu PCM
	var allPcmData []float32

	// Dùng kích thước frame 20ms làm buffer
	perFrameDuration := 20
	frameSize := int(format.SampleRate) * perFrameDuration / 1000
	audioBuf := &audio.IntBuffer{
		Format:         format,
		SourceBitDepth: 16,
		Data:           make([]int, frameSize*format.NumChannels),
	}

	fmt.Printf("Sử dụng kích thước frame: %d điểm lấy mẫu (%.1fms)\n", frameSize, float64(perFrameDuration))
	fmt.Println("Bắt đầu đọc dữ liệu WAV...")

	for {
		// Đọc dữ liệu WAV
		n, err := wavDecoder.PCMBuffer(audioBuf)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("đọc dữ liệu WAV thất bại: %v", err)
		}

		// Chuyển đổi dữ liệu int sang float32 (khoảng -1.0 đến 1.0)
		for i := 0; i < n; i++ {
			// Chuyển int sang float32, từ khoảng [-32768, 32767] sang [-1.0, 1.0]
			floatSample := float32(audioBuf.Data[i]) / 32767.0
			allPcmData = append(allPcmData, floatSample)
		}
	}

	fmt.Printf("Đọc file WAV thành công, tổng số điểm lấy mẫu: %d, thời lượng: %.2f giây\n",
		len(allPcmData), float64(len(allPcmData))/float64(format.SampleRate))

	return allPcmData, nil
}

func main() {
	// Định nghĩa các tham số dòng lệnh
	var (
		host = flag.String("host", "192.168.1.97", "Địa chỉ IP của server FunASR")
		port = flag.String("port", "10096", "Cổng của server FunASR")
		mode = flag.String("mode", "offline", "Chế độ nhận dạng (online/offline)")
		file = flag.String("file", "test.wav", "Đường dẫn file WAV cần nhận dạng")
	)

	// Phân tích (parse) tham số dòng lệnh
	flag.Parse()

	// Hiển thị hướng dẫn sử dụng
	if len(os.Args) < 2 {
		fmt.Println("Cách dùng: ./streaming_example [tùy chọn]")
		fmt.Println("Tùy chọn:")
		flag.PrintDefaults()
		fmt.Println("\nVí dụ:")
		fmt.Println("  ./streaming_example -host=192.168.1.100 -port=10095 -file=audio.wav")
		fmt.Println("  ./streaming_example -mode=online -file=test.wav")
		return
	}

	config := funasr.FunasrConfig{
		Host:          *host,
		Port:          *port,
		Mode:          *mode,
		SampleRate:    16000,
		ChunkSize:     []int{5, 10, 5},
		ChunkInterval: 10,
		Timeout:       30,
		AutoEnd:       false,
	}

	// Dùng cấu hình để tạo instance ASR
	asr, err := funasr.NewFunasr(config)
	if err != nil {
		fmt.Printf("Tạo instance ASR thất bại: %v\n", err)
		return
	}

	fmt.Printf("Server đích: %s:%s, chế độ: %s\n", config.Host, config.Port, config.Mode)

	// Dùng đường dẫn file audio được chỉ định qua tham số dòng lệnh
	audioFilePath := *file

	// Kiểm tra xem file audio có tồn tại hay không
	if _, err := os.Stat(audioFilePath); os.IsNotExist(err) {
		fmt.Printf("File audio %s không tồn tại\n", audioFilePath)
		fmt.Println("Vui lòng cung cấp đường dẫn file audio hợp lệ")
		return
	}

	// Đọc file WAV và chuyển đổi thành dữ liệu PCM
	pcmData, err := readWavFile(audioFilePath)
	if err != nil {
		fmt.Printf("Đọc file WAV thất bại: %v\n", err)
		return
	}

	// Thực hiện nhận dạng
	result, err := asr.Process(pcmData)
	if err != nil {
		fmt.Printf("Nhận dạng thất bại: %v\n", err)
		return
	}

	// Định dạng và in kết quả
	fmt.Println("Kết quả nhận dạng:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println(result)
	fmt.Println(strings.Repeat("-", 40))
}