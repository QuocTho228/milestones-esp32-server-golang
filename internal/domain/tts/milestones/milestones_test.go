package milestones

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/hraban/opus.v2"

	"milestones-esp32-server-golang/internal/util/workqueue"
)

func OpusToWav(opusData [][]byte, sampleRate int, channels int, fileName string) ([][]int16, error) {
	opusDecoder, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("tạo bộ giải mã Opus thất bại: %v", err)
	}

	wavOut, err := os.Create(fileName)
	if err != nil {
		return nil, fmt.Errorf("tạo file WAV thất bại: %v", err)
	}

	pcmDataList := make([][]int16, 0)
	pcmBuffer := make([]int16, 4096)

	wavEncoder := wav.NewEncoder(wavOut, sampleRate, 16, channels, 1)
	wavBuffer := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: channels, // Sử dụng số kênh (channels) được truyền vào
			SampleRate:  sampleRate,
		},
		SourceBitDepth: 16,
		Data:           make([]int, 4096),
	}

	for _, frame := range opusData {
		n, err := opusDecoder.Decode(frame, pcmBuffer)
		if err != nil {
			return nil, fmt.Errorf("giải mã thất bại: %v", err)
		}
		copyData := make([]int16, len(pcmBuffer[:n]))
		copy(copyData, pcmBuffer[:n])
		pcmDataList = append(pcmDataList, copyData)

		//fmt.Println("pcmData len: ", len(copyData))

		// Chuyển đổi dữ liệu PCM sang định dạng int
		for i := 0; i < len(copyData); i++ {
			wavBuffer.Data = append(wavBuffer.Data, int(copyData[i]))
		}
	}

	// Ghi vào file WAV
	err = wavEncoder.Write(&wavBuffer)
	if err != nil {
		return nil, fmt.Errorf("ghi file WAV thất bại: %v", err)
	}

	wavEncoder.Close()

	return pcmDataList, nil
}

func initLog() error {
	// Sử dụng đầu ra chuẩn (stdout) thay vì ghi vào file
	logrus.SetOutput(os.Stdout)

	// Tắt báo cáo caller mặc định, sử dụng field caller tùy chỉnh
	logrus.SetReportCaller(false)
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000", // Định dạng thời gian, thêm phần mili giây
		ForceColors:     true,                      // Bật hiển thị màu sắc
	})
	logLevel, _ := logrus.ParseLevel(viper.GetString("log.level"))
	if logLevel == 0 {
		logLevel = logrus.DebugLevel // Mặc định thiết lập ở mức Debug
	}
	logrus.SetLevel(logLevel)
	return nil
}

func TestNewMilestonesProviderConfigAndVoiceInfo(t *testing.T) {
	provider := NewMilestonesProvider(map[string]interface{}{
		"server_addr": "wss://example.test/milestones",
		"device_id":   "device-1",
		"client_id":   "client-1",
		"token":       "token-1",
	})

	if provider.ServerAddr != "wss://example.test/milestones" {
		t.Fatalf("ServerAddr = %q", provider.ServerAddr)
	}
	if provider.DeviceID != "device-1" {
		t.Fatalf("DeviceID = %q", provider.DeviceID)
	}
	if provider.Header.Get("Device-Id") != "device-1" {
		t.Fatalf("Device-Id header = %q", provider.Header.Get("Device-Id"))
	}
	if provider.Header.Get("Client-Id") != "client-1" {
		t.Fatalf("Client-Id header = %q", provider.Header.Get("Client-Id"))
	}
	if provider.Header.Get("Authorization") != "Bearer token-1" {
		t.Fatalf("Authorization header = %q", provider.Header.Get("Authorization"))
	}

	info := provider.GetVoiceInfo()
	if info["type"] != "milestones_ws" {
		t.Fatalf("voice info type = %#v", info["type"])
	}
	if info["server_addr"] != "wss://example.test/milestones" {
		t.Fatalf("voice info server_addr = %#v", info["server_addr"])
	}
	if info["device_id"] != "device-1" {
		t.Fatalf("voice info device_id = %#v", info["device_id"])
	}
	if _, ok := info["audio_format"].(map[string]interface{}); !ok {
		t.Fatalf("voice info audio_format missing: %#v", info["audio_format"])
	}
}

func TestMilestonesProviderUnsupportedSetVoiceAndLifecycle(t *testing.T) {
	provider := NewMilestonesProvider(map[string]interface{}{})

	if !provider.IsValid() {
		t.Fatal("provider should be valid")
	}
	if err := provider.Close(); err != nil {
		t.Fatalf("Close error = %v", err)
	}
	if err := provider.SetVoice(map[string]interface{}{"voice": "demo"}); err == nil {
		t.Fatal("expected SetVoice to be unsupported")
	}
}

func TestTextToSpeechStream(t *testing.T) {
	if os.Getenv("RUN_MILESTONES_TEST") != "1" {
		t.Skip("Bỏ qua bài test TTS trực tuyến của Milestones, đặt RUN_MILESTONES_TEST=1 để bật")
	}

	// Khởi tạo log, xuất ra đầu ra chuẩn (stdout)
	//initLog()
	provider := NewMilestonesProvider(map[string]interface{}{
		"server_addr": "wss://api.tenclass.net/milestones/v1/",
		"device_id":   "ba:8f:17:de:94:94",
	})

	// LƯU Ý: Danh sách văn bản test dưới đây được giữ nguyên bằng tiếng Trung vì đây là
	// dữ liệu đầu vào thực tế dùng để kiểm thử engine TTS tiếng Trung (Milestones).
	// Nếu dịch sang tiếng Việt, bài test sẽ không còn kiểm tra đúng chức năng
	// chuyển văn bản tiếng Trung thành giọng nói như thiết kế ban đầu.
	textList := []string{
		"你好，小智TTS单元测试",
		"讲个笑话",
		"今天天气怎么样",
		"你叫什么名字",
		"你今年几岁",
		"你住在哪里",
		"你喜欢吃什么",
		"你最喜欢什么颜色",
		"你最喜欢什么食物",
		"你最喜欢什么动物",
	}

	workqueue.ParallelizeUntil(context.Background(), 3, len(textList), func(piece int) {
		text := textList[piece]
		fmt.Println("Bắt đầu speech text: ", text)
		ch, err := provider.TextToSpeechStream(context.Background(), text, 16000, 1, 20)
		if err != nil {
			fmt.Println("TextToSpeechStream kết nối thất bại: ", err)
			return
		}
		opusDataList := [][]byte{}
		for frame := range ch {
			opusDataList = append(opusDataList, frame)
			if len(frame) == 0 {
				t.Error("nhận được khung âm thanh rỗng")
			}
		}
		fmt.Printf("text: %s, đã nhận %d khung âm thanh\n", text, len(opusDataList))
	})

	/*
		for _, text := range textList {
			fmt.Println("开始 speech text: ", text)
			ch, err := provider.TextToSpeechStream(context.Background(), text)
			if err != nil {
				fmt.Println("TextToSpeechStream 连接失败: ", err)
				return
			}
			opusDataList := [][]byte{}
			for frame := range ch {
				opusDataList = append(opusDataList, frame)
				if len(frame) == 0 {
					t.Error("收到空音频帧")
				}
			}
			//OpusToWav(opusDataList, 24000, 1, "output_24000.wav")
		}*/

}