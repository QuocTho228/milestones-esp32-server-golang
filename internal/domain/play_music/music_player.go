package play_music

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"bytes"

	"milestones-esp32-server-golang/internal/util"
	log "milestones-esp32-server-golang/logger"
)

// Global HTTP client, dùng chung connection pool
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// Lấy HTTP client đã được cấu hình connection pool
func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		httpClient = &http.Client{
			Transport: transport,
			//Timeout:   30 * time.Second,
		}
	})
	return httpClient
}

// PlayMusicStream Phát nhạc từ URL, trả về kênh (channel) luồng âm thanh
// frameDuration: thời lượng mỗi khung (mili giây), mặc định 20ms
// audioFormat: định dạng âm thanh, hỗ trợ "mp3"
func PlayMusicStream(ctx context.Context, url string, sampleRate int, frameDuration int, audioFormat string) (outputChan chan []byte, err error) {
	// Kiểm tra tham số và thiết lập giá trị mặc định
	if frameDuration <= 0 {
		frameDuration = 20 // Mặc định thời lượng khung 20ms
	}
	if audioFormat == "" {
		audioFormat = "mp3" // Mặc định định dạng MP3
	}

	startTs := time.Now().UnixMilli()

	// Tạo HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("tạo request thất bại: %v", err)
	}

	req.Header.Set("Accept", "audio/*")
	req.Header.Set("User-Agent", "MusicPlayer/1.0")

	// Sử dụng connection pool để tạo client
	client := getHTTPClient()

	// Tạo kênh output
	outputChan = make(chan []byte, 100)

	// Khởi chạy goroutine xử lý phản hồi dạng stream
	go func() {
		// Gửi request
		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("Gửi request thất bại: %v", err)
			close(outputChan)
			return
		}
		defer func() {
			resp.Body.Close()
		}()

		// Kiểm tra mã trạng thái phản hồi
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("API request thất bại, mã trạng thái: %d, phản hồi: %s", resp.StatusCode, string(body))
			close(outputChan)
			return
		}

		// Kiểm tra loại nội dung và độ dài nội dung của phản hồi
		contentLength := resp.ContentLength

		// Ghi log độ dài phản hồi
		log.Debugf("Nhận được phản hồi luồng nhạc, Content-Length: %d", contentLength)

		// Kiểm tra Content-Length có hợp lý không
		if contentLength == 0 {
			log.Errorf("Luồng nhạc trả về phản hồi rỗng, Content-Length bằng 0")
			close(outputChan)
			return
		}

		// Header file MP3 cần tối thiểu 100 byte mới phân tích được
		// -1 nghĩa là độ dài chưa xác định (ví dụ truyền theo dạng chunked)
		if contentLength > 0 && contentLength < 100 {
			log.Errorf("Phản hồi luồng nhạc quá nhỏ, không thể phân tích thành MP3: %d byte", contentLength)
			close(outputChan)
			return
		}

		log.Infof("Bắt đầu phát nhạc: %s", url)

		// Xử lý phản hồi dạng stream theo định dạng âm thanh
		if audioFormat == "mp3" {
			// Tạo bộ giải mã MP3, truyền context thay vì kênh done
			mp3Decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, resp.Body, outputChan, frameDuration, audioFormat, sampleRate)
			if err != nil {
				log.Errorf("Tạo bộ giải mã MP3 thất bại: %v", err)
				close(outputChan)
				return
			}

			// Bắt đầu quá trình giải mã
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("Giải mã MP3 thất bại: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("Phát nhạc bị hủy, URL: %s", url)
				return
			default:
				log.Infof("Phát nhạc hoàn tất, thời gian: %d ms", time.Now().UnixMilli()-startTs)
			}
		} else {
			log.Errorf("Hiện chỉ hỗ trợ phát stream định dạng MP3, định dạng truyền vào: %s", audioFormat)
			close(outputChan)
		}
	}()

	return outputChan, nil
}

func PlayMusicFromAudioData(ctx context.Context, audioData []byte, sampleRate int, frameDuration int, audioFormat string) (outputChan chan []byte, err error) {
	// Kiểm tra tham số và thiết lập giá trị mặc định
	if frameDuration <= 0 {
		frameDuration = 20 // Mặc định thời lượng khung 20ms
	}
	if audioFormat == "" {
		audioFormat = "mp3" // Mặc định định dạng MP3
	}

	// Thêm thông tin debug
	log.Debugf("PlayMusicFromAudioData: độ dài dữ liệu âm thanh=%d byte, tần số lấy mẫu=%d, thời lượng khung=%dms, định dạng=%s",
		len(audioData), sampleRate, frameDuration, audioFormat)

	// Kiểm tra dữ liệu âm thanh có rỗng không
	if len(audioData) == 0 {
		log.Errorf("Dữ liệu âm thanh rỗng, không thể phát")
		return nil, fmt.Errorf("dữ liệu âm thanh rỗng")
	}

	startTs := time.Now().UnixMilli()

	// Tạo kênh output
	outputChan = make(chan []byte, 100)

	// Khởi chạy goroutine xử lý phản hồi dạng stream
	go func() {
		// Tạo một io.ReadCloser từ audioData
		audioReader := io.NopCloser(bytes.NewReader(audioData))

		// Xử lý phản hồi dạng stream theo định dạng âm thanh
		if audioFormat == "mp3" {
			// Tạo bộ giải mã MP3, truyền context thay vì kênh done
			mp3Decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, audioReader, outputChan, frameDuration, audioFormat, sampleRate)
			if err != nil {
				log.Errorf("Tạo bộ giải mã MP3 thất bại: %v", err)
				return
			}

			// Bắt đầu quá trình giải mã
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("Giải mã MP3 thất bại: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("Phát nhạc bị hủy")
				return
			default:
				log.Infof("Phát nhạc hoàn tất, thời gian: %d ms", time.Now().UnixMilli()-startTs)
			}
		} else {
			log.Errorf("Hiện chỉ hỗ trợ phát stream định dạng MP3, định dạng truyền vào: %s", audioFormat)
		}
	}()

	return outputChan, nil
}

func PlayMusicFromPipe(ctx context.Context, pipeReader *io.PipeReader, sampleRate int, frameDuration int, audioFormat string) (outputChan chan []byte, err error) {
	// Kiểm tra tham số và thiết lập giá trị mặc định
	if frameDuration <= 0 {
		frameDuration = 20 // Mặc định thời lượng khung 20ms
	}
	if audioFormat == "" {
		audioFormat = "mp3" // Mặc định định dạng MP3
	}

	// Thêm thông tin debug
	log.Debugf("PlayMusicFromPipe: tần số lấy mẫu=%d, thời lượng khung=%dms, định dạng=%s",
		sampleRate, frameDuration, audioFormat)

	startTs := time.Now().UnixMilli()

	// Tạo kênh output
	outputChan = make(chan []byte, 100)

	// Khởi chạy goroutine xử lý phản hồi dạng stream
	go func() {
		// Xử lý phản hồi dạng stream theo định dạng âm thanh
		if audioFormat == "mp3" {
			// Tạo bộ giải mã MP3, truyền context thay vì kênh done
			mp3Decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, audioFormat, sampleRate)
			if err != nil {
				log.Errorf("Tạo bộ giải mã MP3 thất bại: %v", err)
				return
			}

			// Bắt đầu quá trình giải mã
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("Giải mã MP3 thất bại: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("Phát nhạc bị hủy")
				return
			default:
				log.Infof("Phát nhạc hoàn tất, thời gian: %d ms", time.Now().UnixMilli()-startTs)
			}
		} else {
			log.Errorf("Hiện chỉ hỗ trợ phát stream định dạng MP3, định dạng truyền vào: %s", audioFormat)
		}
	}()

	return outputChan, nil
}