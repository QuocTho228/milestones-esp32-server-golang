package util

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"

	log "milestones-esp32-server-golang/logger"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"gopkg.in/hraban/opus.v2"
)

// min returns the smaller of x or y.
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// readCloserWrapper cung cấp phương thức Close cho bytes.Reader để hiện thực interface ReadCloser
type readCloserWrapper struct {
	*bytes.Reader
}

// Close hiện thực interface io.Closer
func (r *readCloserWrapper) Close() error {
	return nil
}

// newReadCloserWrapper tạo một wrapper ReadCloser mới
func newReadCloserWrapper(data []byte) *readCloserWrapper {
	return &readCloserWrapper{bytes.NewReader(data)}
}

// WavToOpus chuyển dữ liệu âm thanh WAV sang định dạng Opus chuẩn
// Trả về tập hợp slice khung Opus, mỗi slice là một khung đã mã hóa Opus
func WavToOpus(wavData []byte, sampleRate int, channels int, bitRate int) ([][]byte, error) {
	// Tạo bộ giải mã WAV
	wavReader := bytes.NewReader(wavData)
	wavDecoder := wav.NewDecoder(wavReader)
	if !wavDecoder.IsValidFile() {
		return nil, fmt.Errorf("File WAV không hợp lệ")
	}

	// Đọc thông tin file WAV
	wavDecoder.ReadInfo()
	format := wavDecoder.Format()
	wavSampleRate := int(format.SampleRate)
	wavChannels := int(format.NumChannels)

	// Nếu tham số cung cấp không khớp với tham số trong file, dùng tham số trong file
	if sampleRate == 0 {
		sampleRate = wavSampleRate
	}
	if channels == 0 {
		channels = wavChannels
	}

	//In thông tin wavDecoder
	fmt.Println("Định dạng WAV:", format)

	enc, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
	if err != nil {
		return nil, fmt.Errorf("Tạo bộ mã hóa Opus thất bại: %v", err)
	}

	// Thiết lập bitrate
	if bitRate > 0 {
		if err := enc.SetBitrate(bitRate); err != nil {
			return nil, fmt.Errorf("Thiết lập bitrate thất bại: %v", err)
		}
	}

	// Tạo mảng slice khung đầu ra
	opusFrames := make([][]byte, 0)

	perFrameDuration := 20
	// Bộ đệm PCM - kích thước khung Opus (60ms)
	frameSize := sampleRate * perFrameDuration / 1000
	pcmBuffer := make([]int16, frameSize*channels)
	opusBuffer := make([]byte, 1000) // Bộ đệm đủ lớn để lưu dữ liệu sau khi mã hóa

	// Đọc bộ đệm âm thanh
	audioBuf := &audio.IntBuffer{Data: make([]int, frameSize*channels), Format: format}

	fmt.Println("Bắt đầu chuyển đổi...")
	for {
		// Đọc dữ liệu WAV
		n, err := wavDecoder.PCMBuffer(audioBuf)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Đọc dữ liệu WAV thất bại: %v", err)
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
			return nil, fmt.Errorf("Mã hóa thất bại: %v", err)
		}

		// Sao chép khung hiện tại vào slice mới và thêm vào mảng khung
		frameData := make([]byte, n)
		copy(frameData, opusBuffer[:n])
		opusFrames = append(opusFrames, frameData)
	}

	return opusFrames, nil
}

type AudioDecoder struct {
	streamer           beep.StreamSeekCloser
	format             beep.Format
	enc                *opus.Encoder
	pipeReader         io.ReadCloser
	perFrameDurationMs int
	AudioFormat        string
	targetSampleRate   int
	TargetAudioFormat  string

	outputOpusChan chan []byte     //đầu ra opus từng khung một
	ctx            context.Context // Bổ sung: điều khiển context
}

// CreateMP3Decoder tạo một bộ giải mã MP3 được điều khiển qua kênh Done
// Giữ lại phương thức này để tương thích với code cũ
func CreateAudioDecoder(ctx context.Context, pipeReader io.ReadCloser, outputOpusChan chan []byte, perFrameDurationMs int, AudioFormat string) (*AudioDecoder, error) {
	return &AudioDecoder{
		pipeReader:         pipeReader,
		outputOpusChan:     outputOpusChan,
		perFrameDurationMs: perFrameDurationMs,
		AudioFormat:        AudioFormat,
		ctx:                ctx,
		TargetAudioFormat:  "opus",
	}, nil
}

// CreateMP3Decoder tạo một bộ giải mã MP3 được điều khiển qua kênh Done
// Giữ lại phương thức này để tương thích với code cũ
func CreateAudioDecoderWithSampleRate(ctx context.Context, pipeReader io.ReadCloser, outputOpusChan chan []byte, perFrameDurationMs int, AudioFormat string, targetSampleRate int) (*AudioDecoder, error) {
	return &AudioDecoder{
		pipeReader:         pipeReader,
		outputOpusChan:     outputOpusChan,
		perFrameDurationMs: perFrameDurationMs,
		AudioFormat:        AudioFormat,
		targetSampleRate:   targetSampleRate,
		ctx:                ctx,
		TargetAudioFormat:  "opus",
	}, nil
}

func (d *AudioDecoder) WithFormat(format beep.Format) *AudioDecoder {
	d.format = format
	return d
}

func (d *AudioDecoder) WithTargetAudioFormat(targetAudioFormat string) *AudioDecoder {
	d.TargetAudioFormat = targetAudioFormat
	return d
}

func (d *AudioDecoder) Run(startTs int64) error {
	if d.AudioFormat == "wav" {
		d.RunWavDecoder(startTs, false)
	} else if d.AudioFormat == "pcm" {
		d.RunWavDecoder(startTs, true)
	} else if d.AudioFormat == "mp3" {
		return d.RunMp3Decoder(startTs)
	} else if d.AudioFormat == "opus" {
		return d.RunOpusDecoder(startTs)
	} else if d.AudioFormat == "ogg_opus" {
		return d.RunOggOpusDecoder(startTs)
	}
	return nil
}

// WriteLengthPrefixedFrame ghi một khung dữ liệu âm thanh theo định dạng "header độ dài 4 byte + payload", tiện cho việc truyền dạng luồng đến bộ giải mã dùng chung.
func WriteLengthPrefixedFrame(writer io.Writer, frame []byte) error {
	if writer == nil {
		return fmt.Errorf("writer không được để trống")
	}
	if len(frame) == 0 {
		return fmt.Errorf("frame không được để trống")
	}

	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(frame)))
	if _, err := writer.Write(header[:]); err != nil {
		return fmt.Errorf("Ghi độ dài khung thất bại: %v", err)
	}
	if _, err := writer.Write(frame); err != nil {
		return fmt.Errorf("Ghi dữ liệu khung thất bại: %v", err)
	}
	return nil
}

func readLengthPrefixedFrame(reader io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}

	frameLen := binary.LittleEndian.Uint32(header[:])
	if frameLen == 0 {
		return nil, fmt.Errorf("Độ dài khung không được bằng 0")
	}
	if frameLen > 64*1024 {
		return nil, fmt.Errorf("Độ dài khung quá lớn: %d", frameLen)
	}

	frame := make([]byte, int(frameLen))
	if _, err := io.ReadFull(reader, frame); err != nil {
		return nil, err
	}
	return frame, nil
}

func (d *AudioDecoder) RunOpusDecoder(startTs int64) error {
	defer func() {
		close(d.outputOpusChan)
		if d.pipeReader != nil {
			d.pipeReader.Close()
		}
	}()

	sourceSampleRate := int(d.format.SampleRate)
	if sourceSampleRate < 1 {
		sourceSampleRate = 16000
		log.Warnf("Tần số lấy mẫu đầu vào Opus bằng 0, xử lý theo 16000 Hz")
	}

	channels := d.format.NumChannels
	if channels < 1 {
		channels = 1
		log.Warnf("Số kênh đầu vào Opus bằng 0, xử lý như đơn kênh")
	}

	return d.runOpusPacketStream(startTs, sourceSampleRate, channels, func() ([]byte, error) {
		packet, err := readLengthPrefixedFrame(d.pipeReader)
		if err == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("Đọc khung Opus thất bại: dữ liệu không đầy đủ")
		}
		if err != nil {
			return nil, err
		}
		return packet, nil
	})
}

func (d *AudioDecoder) RunOggOpusDecoder(startTs int64) error {
	defer func() {
		close(d.outputOpusChan)
		if d.pipeReader != nil {
			d.pipeReader.Close()
		}
	}()

	packetReader := &oggOpusPacketReader{reader: d.pipeReader}
	info, err := packetReader.Prepare()
	if err != nil {
		return fmt.Errorf("Phân tích header Ogg Opus thất bại: %v", err)
	}

	log.Debugf("Bộ giải mã Ogg Opus bắt đầu, tần số lấy mẫu gốc: %d, kênh gốc: %d, tần số lấy mẫu đích: %d, định dạng đích: %s", info.SampleRate, info.Channels, d.getTargetSampleRate(info.SampleRate), d.TargetAudioFormat)

	return d.runOpusPacketStream(startTs, info.SampleRate, info.Channels, packetReader.NextPacket)
}

func (d *AudioDecoder) runOpusPacketStream(startTs int64, sourceSampleRate int, channels int, nextPacket func() ([]byte, error)) error {
	firstPacket, err := nextPacket()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}

	if d.canPassthroughOpusPacket(sourceSampleRate, channels, firstPacket) {
		return d.passThroughOpusPackets(startTs, firstPacket, nextPacket)
	}
	if d.canRepacketizeOpusPacket(sourceSampleRate, channels, firstPacket) {
		return d.repacketizeOpusPackets(startTs, sourceSampleRate, firstPacket, nextPacket)
	}

	return d.transcodeOpusPackets(startTs, sourceSampleRate, channels, firstPacket, nextPacket)
}

func (d *AudioDecoder) passThroughOpusPackets(startTs int64, firstPacket []byte, nextPacket func() ([]byte, error)) error {
	var firstFrame bool
	emitPacket := func(packet []byte) error {
		if len(packet) == 0 {
			return nil
		}
		if !firstFrame {
			firstFrame = true
			log.Infof("tts đám mây->thời gian hoàn thành truyền thẳng khung đầu tiên: %d ms", time.Now().UnixMilli()-startTs)
		}
		frameData := make([]byte, len(packet))
		copy(frameData, packet)
		select {
		case <-d.ctx.Done():
			log.Debugf("opus passthrough context done, exit")
			return nil
		case d.outputOpusChan <- frameData:
		}
		return nil
	}

	if err := emitPacket(firstPacket); err != nil {
		return err
	}

	for {
		select {
		case <-d.ctx.Done():
			log.Debugf("opus passthrough context done, exit")
			return nil
		default:
		}

		packet, err := nextPacket()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := emitPacket(packet); err != nil {
			return err
		}
	}
}

func (d *AudioDecoder) transcodeOpusPackets(startTs int64, sourceSampleRate int, channels int, firstPacket []byte, nextPacket func() ([]byte, error)) error {
	targetSampleRate := d.getTargetSampleRate(sourceSampleRate)
	frameDurationMs := d.perFrameDurationMs
	if frameDurationMs <= 0 {
		frameDurationMs = 60
	}
	sourceFrameSize := sourceSampleRate * frameDurationMs / 1000
	if sourceFrameSize <= 0 {
		return fmt.Errorf("Thời lượng khung Opus không hợp lệ: %d ms", frameDurationMs)
	}

	outputChannels := 1
	var enc *opus.Encoder
	var err error
	if d.TargetAudioFormat == "opus" {
		enc, err = opus.NewEncoder(targetSampleRate, outputChannels, opus.AppAudio)
		if err != nil {
			return fmt.Errorf("Tạo bộ mã hóa Opus thất bại: %v", err)
		}
		d.enc = enc
	}

	opusDecoder, err := opus.NewDecoder(sourceSampleRate, channels)
	if err != nil {
		return fmt.Errorf("Tạo bộ giải mã Opus thất bại: %v", err)
	}

	maxDecodeSamples := channels * sourceSampleRate * 120 / 1000
	if maxDecodeSamples < channels*sourceSampleRate/50 {
		maxDecodeSamples = channels * sourceSampleRate / 50
	}
	decodedBuffer := make([]int16, maxDecodeSamples)
	pcmBuffer := make([]int16, 0, sourceFrameSize*2)
	opusBuffer := make([]byte, 1000)
	var firstFrame bool

	log.Debugf("Chuyển mã Opus bắt đầu, tần số lấy mẫu gốc: %d, tần số lấy mẫu đích: %d, kênh gốc: %d, kích thước khung: %d, định dạng đích: %s", sourceSampleRate, targetSampleRate, channels, sourceFrameSize, d.TargetAudioFormat)

	emitFrame := func(frame []int16) error {
		if len(frame) == 0 {
			return nil
		}

		outputPCM := append([]int16(nil), frame...)
		if targetSampleRate > 0 && targetSampleRate != sourceSampleRate {
			pcmBytes := Int16SliceToBytes(outputPCM)
			pcmFloat32 := PCM16BytesToFloat32(pcmBytes)
			pcmFloat32 = ResampleLinearFloat32(pcmFloat32, sourceSampleRate, targetSampleRate)
			outputPCM = Float32SliceToInt16Slice(pcmFloat32)
		}

		if !firstFrame {
			firstFrame = true
			log.Infof("tts đám mây->thời gian hoàn thành giải mã khung đầu tiên: %d ms", time.Now().UnixMilli()-startTs)
		}

		switch d.TargetAudioFormat {
		case "opus":
			n, encodeErr := enc.Encode(outputPCM, opusBuffer)
			if encodeErr != nil {
				return fmt.Errorf("Mã hóa lại Opus thất bại: %v", encodeErr)
			}
			frameData := make([]byte, n)
			copy(frameData, opusBuffer[:n])
			select {
			case <-d.ctx.Done():
				log.Debugf("opusDecoder context done, exit")
				return nil
			case d.outputOpusChan <- frameData:
			}
		case "pcm":
			pcmData := Int16SliceToBytes(outputPCM)
			select {
			case <-d.ctx.Done():
				log.Debugf("opusDecoder context done, exit")
				return nil
			case d.outputOpusChan <- pcmData:
			}
		default:
			return fmt.Errorf("Định dạng âm thanh đích không được hỗ trợ: %s", d.TargetAudioFormat)
		}

		return nil
	}

	flushFrames := func(flushLast bool) error {
		for len(pcmBuffer) >= sourceFrameSize {
			frame := append([]int16(nil), pcmBuffer[:sourceFrameSize]...)
			if err := emitFrame(frame); err != nil {
				return err
			}
			pcmBuffer = pcmBuffer[sourceFrameSize:]
		}
		if flushLast && len(pcmBuffer) > 0 {
			padded := make([]int16, sourceFrameSize)
			copy(padded, pcmBuffer)
			if err := emitFrame(padded); err != nil {
				return err
			}
			pcmBuffer = pcmBuffer[:0]
		}
		return nil
	}

	processPacket := func(packet []byte) error {
		n, err := opusDecoder.Decode(packet, decodedBuffer)
		if err != nil {
			return fmt.Errorf("Giải mã khung Opus thất bại: %v", err)
		}
		if n <= 0 {
			return nil
		}

		if channels == 1 {
			pcmBuffer = append(pcmBuffer, decodedBuffer[:n]...)
		} else {
			for i := 0; i < n; i++ {
				base := i * channels
				var sampleSum int32
				for ch := 0; ch < channels; ch++ {
					sampleSum += int32(decodedBuffer[base+ch])
				}
				pcmBuffer = append(pcmBuffer, int16(sampleSum/int32(channels)))
			}
		}
		return flushFrames(false)
	}

	if err := processPacket(firstPacket); err != nil {
		return err
	}

	for {
		select {
		case <-d.ctx.Done():
			log.Debugf("opusDecoder context done, exit")
			return nil
		default:
		}

		packet, err := nextPacket()
		if err == io.EOF {
			log.Debugf("Đọc luồng Opus kết thúc, xử lý dữ liệu còn lại")
			return flushFrames(true)
		}
		if err != nil {
			return err
		}
		if err := processPacket(packet); err != nil {
			return err
		}
	}
}

func (d *AudioDecoder) repacketizeOpusPackets(startTs int64, sourceSampleRate int, firstPacket []byte, nextPacket func() ([]byte, error)) error {
	targetDurationMs := d.perFrameDurationMs
	if targetDurationMs <= 0 {
		return fmt.Errorf("Thời lượng khung Opus đích không hợp lệ: %d ms", targetDurationMs)
	}

	rp, err := newOpusRepacketizer()
	if err != nil {
		return err
	}
	defer rp.close()

	currentDurationMs := 0
	prevTOC := byte(0)
	var firstFrame bool

	emitCurrent := func() error {
		if rp.nbFrames() == 0 {
			return nil
		}
		packet, err := rp.out()
		if err != nil {
			return fmt.Errorf("Xuất Opus packet sau khi ghép lại thất bại: %v", err)
		}
		if len(packet) == 0 {
			rp.reset()
			currentDurationMs = 0
			prevTOC = 0
			return nil
		}
		if !firstFrame {
			firstFrame = true
			log.Infof("tts đám mây->thời gian hoàn thành ghép lại khung đầu tiên: %d ms", time.Now().UnixMilli()-startTs)
		}
		frameData := make([]byte, len(packet))
		copy(frameData, packet)
		select {
		case <-d.ctx.Done():
			log.Debugf("opus repacketize context done, exit")
			return nil
		case d.outputOpusChan <- frameData:
		}
		rp.reset()
		currentDurationMs = 0
		prevTOC = 0
		return nil
	}

	appendPacket := func(packet []byte) error {
		if len(packet) == 0 {
			return nil
		}
		packetDurationMs, err := opusPacketDurationMs(packet, sourceSampleRate)
		if err != nil {
			return err
		}
		if packetDurationMs <= 0 {
			return fmt.Errorf("Thời lượng Opus packet không hợp lệ: %d ms", packetDurationMs)
		}
		if packetDurationMs > targetDurationMs {
			return fmt.Errorf("Thời lượng Opus packet %d ms lớn hơn độ dài khung đích %d ms, không thể chỉ xử lý bằng cách ghép lại", packetDurationMs, targetDurationMs)
		}

		needFlush := rp.nbFrames() > 0 && (((prevTOC & 0xFC) != (packet[0] & 0xFC)) || currentDurationMs+packetDurationMs > targetDurationMs)
		if needFlush {
			if err := emitCurrent(); err != nil {
				return err
			}
		}

		if err := rp.cat(packet); err != nil {
			return fmt.Errorf("Gửi Opus packet đến repacketizer thất bại: %v", err)
		}
		prevTOC = packet[0]
		currentDurationMs += packetDurationMs
		if currentDurationMs == targetDurationMs {
			return emitCurrent()
		}
		return nil
	}

	if err := appendPacket(firstPacket); err != nil {
		return err
	}

	for {
		select {
		case <-d.ctx.Done():
			log.Debugf("opus repacketize context done, exit")
			return nil
		default:
		}

		packet, err := nextPacket()
		if err == io.EOF {
			return emitCurrent()
		}
		if err != nil {
			return err
		}
		if err := appendPacket(packet); err != nil {
			return err
		}
	}
}

func (d *AudioDecoder) getTargetSampleRate(sourceSampleRate int) int {
	targetSampleRate := sourceSampleRate
	if d.targetSampleRate > 0 {
		targetSampleRate = d.targetSampleRate
	}
	return targetSampleRate
}

func (d *AudioDecoder) canPassthroughOpusPacket(sourceSampleRate int, channels int, firstPacket []byte) bool {
	if d.TargetAudioFormat != "opus" {
		return false
	}
	if channels != 1 {
		return false
	}
	if d.getTargetSampleRate(sourceSampleRate) != sourceSampleRate {
		return false
	}
	if d.perFrameDurationMs <= 0 {
		return true
	}

	packetDurationMs, err := opusPacketDurationMs(firstPacket, sourceSampleRate)
	if err != nil {
		log.Debugf("Phân tích thời lượng Opus packet thất bại, chuyển sang chuyển mã: %v", err)
		return false
	}
	if packetDurationMs != d.perFrameDurationMs {
		log.Debugf("Thời lượng Opus packet không khớp, chuyển sang chuyển mã: packet=%dms target=%dms", packetDurationMs, d.perFrameDurationMs)
		return false
	}
	return true
}

func (d *AudioDecoder) canRepacketizeOpusPacket(sourceSampleRate int, channels int, firstPacket []byte) bool {
	if d.TargetAudioFormat != "opus" {
		return false
	}
	if channels != 1 {
		return false
	}
	if d.getTargetSampleRate(sourceSampleRate) != sourceSampleRate {
		return false
	}
	targetDurationMs := d.perFrameDurationMs
	if targetDurationMs <= 0 || targetDurationMs > 120 {
		return false
	}

	packetDurationMs, err := opusPacketDurationMs(firstPacket, sourceSampleRate)
	if err != nil {
		log.Debugf("Phân tích thời lượng Opus packet thất bại, chuyển sang chuyển mã: %v", err)
		return false
	}
	if packetDurationMs <= 0 || packetDurationMs >= targetDurationMs {
		return false
	}
	return true
}

func opusPacketDurationMs(packet []byte, sampleRate int) (int, error) {
	if len(packet) == 0 {
		return 0, fmt.Errorf("Opus packet rỗng")
	}
	if sampleRate <= 0 {
		sampleRate = 48000
	}

	samplesPerFrame := opusPacketSamplesPerFrame(packet[0], sampleRate)
	frameCount, err := opusPacketFrameCount(packet)
	if err != nil {
		return 0, err
	}
	totalSamples := samplesPerFrame * frameCount
	return totalSamples * 1000 / sampleRate, nil
}

func opusPacketSamplesPerFrame(toc byte, sampleRate int) int {
	if toc&0x80 != 0 {
		return (sampleRate << ((toc >> 3) & 0x03)) / 400
	}
	if toc&0x60 == 0x60 {
		if toc&0x08 != 0 {
			return sampleRate / 50
		}
		return sampleRate / 100
	}

	audioSize := (toc >> 3) & 0x03
	if audioSize == 3 {
		return sampleRate * 60 / 1000
	}
	return (sampleRate << audioSize) / 100
}

func opusPacketFrameCount(packet []byte) (int, error) {
	if len(packet) == 0 {
		return 0, fmt.Errorf("Opus packet rỗng")
	}

	switch packet[0] & 0x03 {
	case 0:
		return 1, nil
	case 1, 2:
		return 2, nil
	default:
		if len(packet) < 2 {
			return 0, fmt.Errorf("Độ dài Opus packet không đủ, không thể phân tích frame count")
		}
		return int(packet[1] & 0x3F), nil
	}
}

type opusStreamInfo struct {
	SampleRate int
	Channels   int
}

type oggPage struct {
	HeaderType byte
	Segments   []byte
	Body       []byte
}

type oggOpusPacketReader struct {
	reader   io.Reader
	queue    [][]byte
	carry    []byte
	info     opusStreamInfo
	headSeen bool
	tagsSeen bool
}

func (r *oggOpusPacketReader) Prepare() (opusStreamInfo, error) {
	for !r.headSeen || !r.tagsSeen {
		if err := r.readNextPage(); err != nil {
			if err == io.EOF {
				return opusStreamInfo{}, fmt.Errorf("Luồng Ogg Opus thiếu header bắt buộc")
			}
			return opusStreamInfo{}, err
		}
	}

	if r.info.SampleRate <= 0 {
		r.info.SampleRate = 48000
	}
	if r.info.Channels <= 0 {
		r.info.Channels = 1
	}
	return r.info, nil
}

func (r *oggOpusPacketReader) NextPacket() ([]byte, error) {
	for len(r.queue) == 0 {
		if err := r.readNextPage(); err != nil {
			if err == io.EOF {
				if len(r.carry) > 0 {
					return nil, io.ErrUnexpectedEOF
				}
				return nil, io.EOF
			}
			return nil, err
		}
	}

	packet := r.queue[0]
	r.queue = r.queue[1:]
	return packet, nil
}

func (r *oggOpusPacketReader) readNextPage() error {
	page, err := readOggPage(r.reader)
	if err != nil {
		return err
	}

	packet := r.carry
	if len(packet) == 0 && page.HeaderType&0x01 != 0 {
		return fmt.Errorf("Nhận được Ogg continuation page thiếu dữ liệu trước đó")
	}

	offset := 0
	for _, segmentLen := range page.Segments {
		end := offset + int(segmentLen)
		if end > len(page.Body) {
			return fmt.Errorf("Độ dài dữ liệu Ogg page không đầy đủ")
		}
		packet = append(packet, page.Body[offset:end]...)
		offset = end
		if segmentLen < 255 {
			completePacket := append([]byte(nil), packet...)
			if err := r.handlePacket(completePacket); err != nil {
				return err
			}
			packet = nil
		}
	}

	if offset != len(page.Body) {
		return fmt.Errorf("Dữ liệu Ogg page còn phần đuôi chưa xử lý: offset=%d total=%d", offset, len(page.Body))
	}

	r.carry = packet
	return nil
}

func (r *oggOpusPacketReader) handlePacket(packet []byte) error {
	switch {
	case !r.headSeen:
		info, err := parseOpusHeadPacket(packet)
		if err != nil {
			return err
		}
		r.info = info
		r.headSeen = true
	case !r.tagsSeen:
		if !bytes.HasPrefix(packet, []byte("OpusTags")) {
			return fmt.Errorf("Thiếu gói OpusTags")
		}
		r.tagsSeen = true
	default:
		if len(packet) > 0 {
			r.queue = append(r.queue, packet)
		}
	}
	return nil
}

func parseOpusHeadPacket(packet []byte) (opusStreamInfo, error) {
	if len(packet) < 19 {
		return opusStreamInfo{}, fmt.Errorf("Độ dài gói OpusHead không đủ: %d", len(packet))
	}
	if !bytes.HasPrefix(packet, []byte("OpusHead")) {
		return opusStreamInfo{}, fmt.Errorf("Thiếu gói OpusHead")
	}

	channels := int(packet[9])
	if channels < 1 {
		channels = 1
	}
	sampleRate := int(binary.LittleEndian.Uint32(packet[12:16]))
	if sampleRate <= 0 {
		sampleRate = 48000
	}

	return opusStreamInfo{
		SampleRate: NormalizeOpusSampleRate(sampleRate),
		Channels:   channels,
	}, nil
}

func readOggPage(reader io.Reader) (oggPage, error) {
	header := make([]byte, 27)
	n, err := io.ReadFull(reader, header)
	if err != nil {
		if err == io.EOF && n == 0 {
			return oggPage{}, io.EOF
		}
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			return oggPage{}, io.ErrUnexpectedEOF
		}
		return oggPage{}, err
	}

	if !bytes.Equal(header[:4], []byte("OggS")) {
		return oggPage{}, fmt.Errorf("Header OggS không hợp lệ")
	}
	if header[4] != 0 {
		return oggPage{}, fmt.Errorf("Phiên bản Ogg không được hỗ trợ: %d", header[4])
	}

	segmentCount := int(header[26])
	segments := make([]byte, segmentCount)
	if _, err := io.ReadFull(reader, segments); err != nil {
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			return oggPage{}, io.ErrUnexpectedEOF
		}
		return oggPage{}, err
	}

	bodyLen := 0
	for _, segmentLen := range segments {
		bodyLen += int(segmentLen)
	}

	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(reader, body); err != nil {
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			return oggPage{}, io.ErrUnexpectedEOF
		}
		return oggPage{}, err
	}

	return oggPage{
		HeaderType: header[5],
		Segments:   segments,
		Body:       body,
	}, nil
}

func (d *AudioDecoder) RunWavDecoder(startTs int64, isRaw bool) error {
	defer func() {
		close(d.outputOpusChan)
		if d.pipeReader != nil {
			d.pipeReader.Close()
		}
	}()
	var sampleRate int
	var channels int

	if !isRaw {
		// Header file WAV cố định là 44 byte
		headerSize := 44
		header := make([]byte, headerSize)
		_, err := io.ReadFull(d.pipeReader, header)
		if err != nil {
			return fmt.Errorf("Đọc header WAV thất bại: %v", err)
		}

		// Lấy các tham số cơ bản từ header WAV
		// Tần số lấy mẫu: byte 24-27
		sampleRate = int(uint32(header[24]) | uint32(header[25])<<8 | uint32(header[26])<<16 | uint32(header[27])<<24)
		// Số kênh: byte 22-23
		channels = int(uint16(header[22]) | uint16(header[23])<<8)
		if channels < 1 {
			channels = 1
			log.Warnf("Số kênh trong header WAV bằng 0, xử lý như đơn kênh")
		}
		if sampleRate < 1 {
			sampleRate = 24000
			log.Warnf("Tần số lấy mẫu trong header WAV bằng 0, xử lý theo 24000 Hz")
		}
		log.Debugf("Định dạng WAV: %d Hz, %d kênh", sampleRate, channels)
	} else {
		// Đối với dữ liệu PCM gốc, dùng tham số trong format
		sampleRate = int(d.format.SampleRate)
		channels = d.format.NumChannels
		if channels < 1 {
			channels = 1
			log.Warnf("Số kênh PCM bằng 0, xử lý như đơn kênh")
		}
		if sampleRate < 1 {
			sampleRate = 24000
			log.Warnf("Tần số lấy mẫu PCM bằng 0, xử lý theo 24000 Hz")
		}
		log.Debugf("Định dạng PCM gốc: %d Hz, %d kênh", sampleRate, channels)
	}

	// Luôn dùng đầu ra đơn kênh
	outputChannels := 1
	if channels > 1 {
		log.Debugf("Chuyển âm thanh đa kênh thành đầu ra đơn kênh")
	}

	opusSampleRate := int(sampleRate)
	if d.targetSampleRate > 0 {
		opusSampleRate = d.targetSampleRate
	}

	// Quyết định có tạo bộ mã hóa Opus hay không dựa trên định dạng đích
	var enc *opus.Encoder
	var err error
	if d.TargetAudioFormat == "opus" {
		enc, err = opus.NewEncoder(opusSampleRate, outputChannels, opus.AppAudio)
		if err != nil {
			return fmt.Errorf("Tạo bộ mã hóa Opus thất bại: %v", err)
		}
		d.enc = enc
	}

	//Cấu hình và bộ đệm liên quan đến opus
	frameDurationMs := d.perFrameDurationMs               //Thời lượng mỗi khung (ms)
	frameSize := int(sampleRate) * frameDurationMs / 1000 //Số điểm lấy mẫu mỗi khung (dựa trên tần số lấy mẫu gốc)
	pcmBuffer := make([]int16, frameSize*outputChannels)  //Bộ đệm PCM
	opusBuffer := make([]byte, 1000)                      //Bộ đệm đầu ra Opus

	log.Debugf("Bộ giải mã WAV/PCM bắt đầu, tần số lấy mẫu gốc: %d, tần số lấy mẫu đích: %d, kích thước khung: %d, định dạng đích: %s", sampleRate, opusSampleRate, frameSize, d.TargetAudioFormat)

	// Bộ đệm dùng để đọc dữ liệu PCM gốc
	bytesPerPoint := 2 * channels // mẫu 16-bit = 2 byte, đa kênh gộp theo một điểm lấy mẫu
	rawBuffer := make([]byte, frameSize*bytesPerPoint)
	remainderBytes := make([]byte, 0, bytesPerPoint*4) // Lưu lại các byte dư chưa căn chỉnh, tránh làm xáo trộn ranh giới lấy mẫu tiếp theo
	currentFramePos := 0
	var firstFrame bool

	flushLastFrame := func() error {
		if currentFramePos <= 0 {
			return nil
		}

		// Tạo một bộ đệm khung đầy đủ, phần còn lại được điền bằng 0
		paddedFrame := make([]int16, len(pcmBuffer))
		copy(paddedFrame, pcmBuffer[:currentFramePos]) // Sao chép dữ liệu hợp lệ vào đầu, phần còn lại mặc định là 0

		var opusPcmBuffer []int16 = paddedFrame
		if d.targetSampleRate > 0 && d.targetSampleRate != sampleRate {
			pcmBytes := Int16SliceToBytes(opusPcmBuffer)
			pcmFloat32 := PCM16BytesToFloat32(pcmBytes)
			pcmFloat32 = ResampleLinearFloat32(pcmFloat32, sampleRate, d.targetSampleRate)
			opusPcmBuffer = Float32SliceToInt16Slice(pcmFloat32)
		}

		// Xuất dữ liệu dựa trên định dạng đích
		if d.TargetAudioFormat == "opus" {
			// Mã hóa khung cuối cùng
			n, encodeErr := enc.Encode(opusPcmBuffer, opusBuffer)
			if encodeErr != nil {
				log.Errorf("Mã hóa dữ liệu còn lại thất bại: %v", encodeErr)
				return fmt.Errorf("Mã hóa dữ liệu còn lại thất bại: %v", encodeErr)
			}
			frameData := make([]byte, n)
			copy(frameData, opusBuffer[:n])
			select {
			case <-d.ctx.Done():
				log.Debugf("wavDecoder context done, exit")
				return nil
			case d.outputOpusChan <- frameData:
			}
			return nil
		}
		if d.TargetAudioFormat == "pcm" {
			// Xuất trực tiếp dữ liệu PCM
			pcmData := Int16SliceToBytes(opusPcmBuffer)
			select {
			case <-d.ctx.Done():
				log.Debugf("wavDecoder context done, exit")
				return nil
			case d.outputOpusChan <- pcmData:
			}
		}
		return nil
	}

	for {
		select {
		case <-d.ctx.Done():
			log.Debugf("wavDecoder context done, exit")
			return nil
		default:
			// Đọc dữ liệu PCM
			n, readErr := d.pipeReader.Read(rawBuffer)
			if n <= 0 && readErr == nil {
				continue
			}

			var chunk []byte
			if n > 0 {
				chunk = rawBuffer[:n]
				if len(remainderBytes) > 0 {
					combined := make([]byte, 0, len(remainderBytes)+len(chunk))
					combined = append(combined, remainderBytes...)
					combined = append(combined, chunk...)
					chunk = combined
					remainderBytes = remainderBytes[:0]
				}

				alignedBytes := (len(chunk) / bytesPerPoint) * bytesPerPoint
				if alignedBytes < len(chunk) {
					remainderBytes = append(remainderBytes[:0], chunk[alignedBytes:]...)
					chunk = chunk[:alignedBytes]
				}
			}

			// Chuyển dữ liệu byte sang các điểm lấy mẫu int16 (đảm bảo căn chỉnh theo ranh giới điểm lấy mẫu)
			samplesRead := len(chunk) / bytesPerPoint
			for i := 0; i < samplesRead; i++ {
				// Đối với đa kênh, lấy giá trị trung bình
				var sampleSum int32
				for ch := 0; ch < channels; ch++ {
					pos := i*bytesPerPoint + ch*2
					sample := int16(uint16(chunk[pos]) | uint16(chunk[pos+1])<<8)
					sampleSum += int32(sample)
				}

				// Tính giá trị trung bình đa kênh
				avgSample := int16(sampleSum / int32(channels))
				pcmBuffer[currentFramePos] = avgSample
				currentFramePos++

				// Nếu bộ đệm đã đầy, tiến hành mã hóa hoặc xuất ra
				if currentFramePos == len(pcmBuffer) {
					if !firstFrame {
						firstFrame = true
						log.Infof("tts đám mây->thời gian hoàn thành giải mã khung đầu tiên: %d ms", time.Now().UnixMilli()-startTs)
					}

					var opusPcmBuffer []int16 = pcmBuffer
					if d.targetSampleRate > 0 && d.targetSampleRate != sampleRate {
						pcmBytes := Int16SliceToBytes(opusPcmBuffer)
						pcmFloat32 := PCM16BytesToFloat32(pcmBytes)
						pcmFloat32 = ResampleLinearFloat32(pcmFloat32, sampleRate, d.targetSampleRate)
						opusPcmBuffer = Float32SliceToInt16Slice(pcmFloat32)
					}

					if d.TargetAudioFormat == "opus" {
						// Xuất mã hóa Opus
						opusLen, err := enc.Encode(opusPcmBuffer, opusBuffer)
						if err != nil {
							log.Errorf("Mã hóa sau khi giải mã WAV/PCM thất bại: %v", err)
							// Khi mã hóa thất bại, bỏ qua khung này nhưng vẫn tiếp tục xử lý
							currentFramePos = 0 // Đặt lại vị trí khung
							continue
						}

						// Sao chép khung hiện tại vào slice mới
						frameData := make([]byte, opusLen)
						copy(frameData, opusBuffer[:opusLen])
						select {
						case <-d.ctx.Done():
							log.Debugf("wavDecoder context done, exit")
							return nil
						case d.outputOpusChan <- frameData:
						}
					} else if d.TargetAudioFormat == "pcm" {
						// Xuất trực tiếp dữ liệu PCM
						pcmData := Int16SliceToBytes(opusPcmBuffer)
						select {
						case <-d.ctx.Done():
							log.Debugf("wavDecoder context done, exit")
							return nil
						case d.outputOpusChan <- pcmData:
						}
					}
					currentFramePos = 0
				}
			}

			if readErr == io.EOF {
				log.Debugf("Đọc luồng WAV/PCM kết thúc, xử lý dữ liệu còn lại")
				if len(remainderBytes) > 0 {
					log.Warnf("WAV/PCM có byte dư không căn chỉnh, đã loại bỏ: %d", len(remainderBytes))
				}
				return flushLastFrame()
			}
			if readErr != nil {
				return fmt.Errorf("Đọc dữ liệu PCM thất bại: %v", readErr)
			}
		}
	}
}

func (d *AudioDecoder) RunMp3Decoder(startTs int64) error {
	defer func() {
		close(d.outputOpusChan)
		if d.pipeReader != nil {
			d.pipeReader.Close()
		}
	}()

	decoder, format, err := mp3.Decode(d.pipeReader)
	if err != nil {
		return fmt.Errorf("Tạo bộ giải mã MP3 thất bại: %v", err)
	}
	log.Debugf("Định dạng MP3: %d Hz, %d kênh", format.SampleRate, format.NumChannels)
	d.streamer = decoder
	d.format = format

	// Giải mã MP3 theo luồng
	defer func() {
		d.streamer.Close()
	}()

	// Lấy thông tin âm thanh MP3
	sampleRate := format.SampleRate
	channels := format.NumChannels

	// Luôn dùng đầu ra đơn kênh
	outputChannels := 1
	if channels > 1 {
		log.Debugf("Chuyển âm thanh hai kênh thành đầu ra đơn kênh")
	}

	opusSampleRate := int(sampleRate)
	if d.targetSampleRate > 0 {
		opusSampleRate = d.targetSampleRate
	}

	// Quyết định có tạo bộ mã hóa Opus hay không dựa trên định dạng đích
	var enc *opus.Encoder
	if d.TargetAudioFormat == "opus" {
		enc, err = opus.NewEncoder(opusSampleRate, outputChannels, opus.AppAudio)
		if err != nil {
			return fmt.Errorf("Tạo bộ mã hóa Opus thất bại: %v", err)
		}
		d.enc = enc
	}

	//Cấu hình và bộ đệm liên quan đến opus, tạo bộ đệm để nhận mẫu âm thanh
	frameDurationMs := d.perFrameDurationMs               //60ms
	frameSize := int(sampleRate) * frameDurationMs / 1000 // kích thước khung 60ms
	// Lưu trữ PCM tạm thời, chuyển âm thanh sang định dạng PCM
	pcmBuffer := make([]int16, frameSize*outputChannels)

	//Bộ đệm đọc mp3
	mp3Buffer := make([][2]float64, 2048)

	//Bộ đệm đầu ra opus
	opusBuffer := make([]byte, 1000)

	currentFramePos := 0 // Vị trí hiện đang điền vào pcmBuffer
	var firstFrame bool
	frameCount := 0

	log.Debugf("Bộ giải mã MP3 bắt đầu, tần số lấy mẫu gốc: %d, tần số lấy mẫu đích: %d, kích thước khung: %d, định dạng đích: %s", int(sampleRate), opusSampleRate, frameSize, d.TargetAudioFormat)

	for {
		select {
		case <-d.ctx.Done():
			log.Debugf("mp3Decoder context done, exit")
			return nil
		default:
			// Đọc dữ liệu PCM từ MP3
			n, ok := d.streamer.Stream(mp3Buffer)

			if !ok {
				log.Debugf("Đọc luồng MP3 kết thúc, xử lý dữ liệu còn lại")
				// Xử lý phần dữ liệu còn lại chưa đủ một khung
				if currentFramePos > 0 {
					// Tạo một bộ đệm khung đầy đủ, phần còn lại được điền bằng 0
					paddedFrame := make([]int16, len(pcmBuffer))
					copy(paddedFrame, pcmBuffer[:currentFramePos]) // Sao chép dữ liệu hợp lệ vào đầu, phần còn lại mặc định là 0

					var opusPcmBuffer []int16 = paddedFrame
					if d.targetSampleRate > 0 && d.targetSampleRate != int(sampleRate) {
						pcmBytes := Int16SliceToBytes(opusPcmBuffer)
						pcmFloat32 := PCM16BytesToFloat32(pcmBytes)
						pcmFloat32 = ResampleLinearFloat32(pcmFloat32, int(sampleRate), d.targetSampleRate)
						opusPcmBuffer = Float32SliceToInt16Slice(pcmFloat32)
					}

					// Xuất dữ liệu dựa trên định dạng đích
					if d.TargetAudioFormat == "opus" {
						// Mã hóa khung hoàn chỉnh sau khi bù đủ
						n, err := enc.Encode(opusPcmBuffer, opusBuffer)
						if err != nil {
							log.Errorf("Mã hóa dữ liệu còn lại thất bại: %v", err)
							return fmt.Errorf("Mã hóa dữ liệu còn lại thất bại: %v", err)
						} else {
							frameData := make([]byte, n)
							copy(frameData, opusBuffer[:n])

							select {
							case <-d.ctx.Done():
								log.Debugf("mp3Decoder context done, exit")
								return nil
							case d.outputOpusChan <- frameData:
								frameCount++
								log.Debugf("Giải mã MP3 hoàn tất, đã xử lý tổng cộng %d khung", frameCount)
							}
						}
					} else if d.TargetAudioFormat == "pcm" {
						// Xuất trực tiếp dữ liệu PCM
						pcmData := Int16SliceToBytes(opusPcmBuffer)
						select {
						case <-d.ctx.Done():
							log.Debugf("mp3Decoder context done, exit")
							return nil
						case d.outputOpusChan <- pcmData:
							frameCount++
							log.Debugf("Giải mã MP3 hoàn tất, đã xử lý tổng cộng %d khung", frameCount)
						}
					}
				}
				return nil
			}

			if n == 0 {
				continue
			}

			// Chuyển dữ liệu âm thanh số thực sang định dạng PCM (số nguyên 16-bit)
			for i := 0; i < n; i++ {
				// Tính giá trị trung bình ở giai đoạn số thực trước, tránh tràn số khi cộng số nguyên
				monoSampleFloat := (mp3Buffer[i][0] + mp3Buffer[i][1]) * 0.5

				// Giới hạn âm lượng, đảm bảo không vượt phạm vi
				if monoSampleFloat > 1.0 {
					monoSampleFloat = 1.0
				} else if monoSampleFloat < -1.0 {
					monoSampleFloat = -1.0
				}

				// Chuyển giá trị trung bình số thực sang số nguyên 16-bit
				monoSample := int16(monoSampleFloat * 32767.0)
				pcmBuffer[currentFramePos] = monoSample
				currentFramePos++

				// Nếu pcmBuffer đã đầy một khung thì tiến hành mã hóa hoặc xuất ra
				if currentFramePos == len(pcmBuffer) {
					if !firstFrame {
						firstFrame = true
						log.Infof("tts đám mây->thời gian hoàn thành giải mã khung đầu tiên: %d ms", time.Now().UnixMilli()-startTs)
					}

					var opusPcmBuffer []int16 = pcmBuffer
					if d.targetSampleRate > 0 && d.targetSampleRate != int(sampleRate) {
						pcmBytes := Int16SliceToBytes(opusPcmBuffer)
						pcmFloat32 := PCM16BytesToFloat32(pcmBytes)
						pcmFloat32 = ResampleLinearFloat32(pcmFloat32, int(sampleRate), d.targetSampleRate)
						opusPcmBuffer = Float32SliceToInt16Slice(pcmFloat32)
					}

					if d.TargetAudioFormat == "opus" {
						// Xuất mã hóa Opus
						opusLen, err := enc.Encode(opusPcmBuffer, opusBuffer)
						if err != nil {
							log.Errorf("Mã hóa sau khi giải mã MP3 thất bại: %v", err)
							// Khi mã hóa thất bại, bỏ qua khung này nhưng vẫn tiếp tục xử lý
							currentFramePos = 0 // Đặt lại vị trí khung
							continue
						}

						// Sao chép khung hiện tại vào slice mới và thêm vào mảng khung
						frameData := make([]byte, opusLen)
						copy(frameData, opusBuffer[:opusLen])

						select {
						case <-d.ctx.Done():
							log.Debugf("mp3Decoder context done, exit")
							return nil
						case d.outputOpusChan <- frameData:
							frameCount++
							if frameCount%100 == 0 {
								log.Debugf("Giải mã MP3 đã xử lý %d khung", frameCount)
							}
						}
					} else if d.TargetAudioFormat == "pcm" {
						// Xuất trực tiếp dữ liệu PCM
						pcmData := Int16SliceToBytes(opusPcmBuffer)
						select {
						case <-d.ctx.Done():
							log.Debugf("mp3Decoder context done, exit")
							return nil
						case d.outputOpusChan <- pcmData:
							frameCount++
							if frameCount%100 == 0 {
								log.Debugf("Giải mã MP3 đã xử lý %d khung", frameCount)
							}
						}
					}

					currentFramePos = 0 // Đặt lại vị trí khung
				}
			}
		}
	}
}

// GetAudioFormatByMimeType lấy định dạng âm thanh dựa trên loại MIME
func GetAudioFormatByMimeType(mimeType string) string {
	switch mimeType {
	case "audio/mpeg", "audio/mp3", "audio/mpeg3", "audio/x-mpeg-3":
		return "mp3"
	case "audio/wav", "audio/wave", "audio/x-wav":
		return "wav"
	case "audio/pcm", "audio/x-pcm":
		return "pcm"
	case "audio/ogg", "application/ogg":
		return "ogg_opus"
	case "audio/opus":
		return "opus"
	default:
		// Mặc định trả về định dạng mp3
		return "mp3"
	}
}

// writeSeekerBuffer hiện thực interface io.WriteSeeker, bọc bytes.Buffer
type writeSeekerBuffer struct {
	*bytes.Buffer
	pos int64
}

func newWriteSeekerBuffer() *writeSeekerBuffer {
	return &writeSeekerBuffer{
		Buffer: bytes.NewBuffer(nil),
		pos:    0,
	}
}

func (w *writeSeekerBuffer) Write(p []byte) (n int, err error) {
	// Nếu vị trí hiện tại ở cuối bộ đệm, nối thêm trực tiếp
	if w.pos == int64(w.Buffer.Len()) {
		n, err = w.Buffer.Write(p)
		w.pos += int64(n)
		return n, err
	}

	// Nếu vị trí hiện tại nằm giữa bộ đệm, cần ghi tại vị trí đó
	// Lấy bản sao dữ liệu bộ đệm hiện tại (tránh sửa trực tiếp bộ đệm gốc)
	data := make([]byte, w.Buffer.Len())
	copy(data, w.Buffer.Bytes())

	// Nếu ghi vượt quá bộ đệm hiện tại thì cần mở rộng
	endPos := w.pos + int64(len(p))
	if endPos > int64(len(data)) {
		// Mở rộng bộ đệm
		extra := int(endPos - int64(len(data)))
		data = append(data, make([]byte, extra)...)
	}

	// Ghi dữ liệu tại vị trí chỉ định
	copy(data[w.pos:], p)

	// Cập nhật bộ đệm
	w.Buffer.Reset()
	w.Buffer.Write(data)

	n = len(p)
	w.pos += int64(n)
	return n, nil
}

func (w *writeSeekerBuffer) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = w.pos + offset
	case io.SeekEnd:
		newPos = int64(w.Buffer.Len()) + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}

	if newPos < 0 {
		return 0, fmt.Errorf("negative position")
	}

	// Nếu vị trí mới vượt quá độ dài bộ đệm hiện tại thì cần mở rộng
	if newPos > int64(w.Buffer.Len()) {
		// Mở rộng bộ đệm
		extra := int(newPos - int64(w.Buffer.Len()))
		w.Buffer.Write(make([]byte, extra))
	}

	w.pos = newPos
	return w.pos, nil
}

// PCMFloat32BytesToWav chuyển mảng byte PCM float32 sang định dạng WAV
// audioData: mảng byte định dạng PCM float32 (mỗi float32 chiếm 4 byte, little-endian)
// sampleRate: tần số lấy mẫu
// channels: số kênh
// Trả về: mảng byte định dạng WAV
func PCMFloat32BytesToWav(audioData []byte, sampleRate, channels int) ([]byte, error) {
	if len(audioData) == 0 {
		return nil, fmt.Errorf("Dữ liệu âm thanh trống")
	}

	// Chuyển mảng byte sang slice float32 (little-endian, mỗi float32 chiếm 4 byte)
	if len(audioData)%4 != 0 {
		// Nếu không phải bội số của 4, cắt về bội số của 4 gần nhất
		audioData = audioData[:len(audioData)-len(audioData)%4]
	}
	float32Data := make([]float32, len(audioData)/4)
	for i := 0; i < len(float32Data); i++ {
		bits := uint32(audioData[i*4]) | uint32(audioData[i*4+1])<<8 | uint32(audioData[i*4+2])<<16 | uint32(audioData[i*4+3])<<24
		float32Data[i] = math.Float32frombits(bits)
	}

	// Chuyển float32 sang int16
	int16Data := Float32SliceToInt16Slice(float32Data)

	// Tạo bộ mã hóa WAV (dùng writeSeekerBuffer làm đầu ra)
	wavBuffer := newWriteSeekerBuffer()
	wavEncoder := wav.NewEncoder(wavBuffer, sampleRate, 16, channels, 1)

	// Tạo bộ đệm âm thanh
	audioBuf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: channels,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: 16,
		Data:           make([]int, len(int16Data)),
	}

	// Chuyển dữ liệu int16 sang slice int
	for i, sample := range int16Data {
		audioBuf.Data[i] = int(sample)
	}

	// Ghi file WAV
	if err := wavEncoder.Write(audioBuf); err != nil {
		return nil, fmt.Errorf("Ghi dữ liệu WAV thất bại: %v", err)
	}

	if err := wavEncoder.Close(); err != nil {
		return nil, fmt.Errorf("Đóng bộ mã hóa WAV thất bại: %v", err)
	}

	return wavBuffer.Buffer.Bytes(), nil
}

// OpusFramesToWav chuyển mảng khung Opus sang định dạng WAV
// opusFrames: mảng khung âm thanh định dạng Opus (mỗi phần tử là một khung Opus)
// sampleRate: tần số lấy mẫu
// channels: số kênh
// Trả về: mảng byte định dạng WAV
// Tham khảo: cách hiện thực OpusToWav trong test/test_audio/audio_utils.go
func OpusFramesToWav(opusFrames [][]byte, sampleRate, channels int) ([]byte, error) {
	if len(opusFrames) == 0 {
		return nil, fmt.Errorf("Dữ liệu âm thanh trống")
	}

	// Tạo bộ giải mã Opus
	opusDecoder, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("Tạo bộ giải mã Opus thất bại: %v", err)
	}

	// Tạo bộ mã hóa WAV (dùng writeSeekerBuffer làm đầu ra)
	wavBuffer := newWriteSeekerBuffer()
	wavEncoder := wav.NewEncoder(wavBuffer, sampleRate, 16, channels, 1)

	// Tạo bộ đệm âm thanh
	audioBuf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: channels,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: 16,
		Data:           make([]int, 0),
	}

	// Bộ đệm PCM dùng để giải mã (dùng 60ms để ước lượng, đủ lớn để chứa một khung)
	perFrameDuration := 60
	pcmBuffer := make([]int16, channels*sampleRate*perFrameDuration/1000)

	// Duyệt qua tất cả khung Opus và giải mã
	for _, opusFrame := range opusFrames {
		if len(opusFrame) == 0 {
			continue
		}

		// Giải mã khung Opus
		n, err := opusDecoder.Decode(opusFrame, pcmBuffer)
		if err != nil {
			return nil, fmt.Errorf("Giải mã khung Opus thất bại: %v", err)
		}

		// Chuyển dữ liệu PCM sang định dạng int và thêm vào bộ đệm
		for i := 0; i < n; i++ {
			audioBuf.Data = append(audioBuf.Data, int(pcmBuffer[i]))
		}
	}

	// Ghi file WAV
	if len(audioBuf.Data) > 0 {
		if err := wavEncoder.Write(audioBuf); err != nil {
			return nil, fmt.Errorf("Ghi dữ liệu WAV thất bại: %v", err)
		}
	}

	if err := wavEncoder.Close(); err != nil {
		return nil, fmt.Errorf("Đóng bộ mã hóa WAV thất bại: %v", err)
	}

	return wavBuffer.Buffer.Bytes(), nil
}