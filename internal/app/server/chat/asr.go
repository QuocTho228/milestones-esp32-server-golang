package chat

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
	. "milestones-esp32-server-golang/internal/data/client"
	"milestones-esp32-server-golang/internal/domain/asr"
	asr_types "milestones-esp32-server-golang/internal/domain/asr/types"
	"milestones-esp32-server-golang/internal/domain/audio"
	chathooks "milestones-esp32-server-golang/internal/domain/chat/hooks"
	"milestones-esp32-server-golang/internal/domain/speaker"
	"milestones-esp32-server-golang/internal/domain/vad/inter"
	"milestones-esp32-server-golang/internal/pool"
	log "milestones-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

type ASRManagerOption func(*ASRManager)

const maxFirstSpeechPreAudioMs = 200

// AsrMessageSaveCallback kiểu callback dùng để lưu tin nhắn
type AsrMessageSaveCallback func(userMsg *schema.Message, messageID string, audioData []float32)

type ASRManager struct {
	clientState     *ClientState
	serverTransport *ServerTransport
	session         *ChatSession // dùng để truy cập speakerManager

	// Tài nguyên ASR được quản lý dưới dạng field private
	asrResource *pool.ResourceWrapper[asr.AsrProvider]
	resourceMu  sync.RWMutex // bảo vệ truy cập tài nguyên
}

func NewASRManager(clientState *ClientState, serverTransport *ServerTransport, opts ...ASRManagerOption) *ASRManager {
	asr := &ASRManager{
		clientState:     clientState,
		serverTransport: serverTransport,
		session:         nil, // sẽ được thiết lập sau qua SetSession
	}
	for _, opt := range opts {
		opt(asr)
	}
	return asr
}

func (a *ASRManager) runAudioIdleTimeoutWatchdog(ctx context.Context) {
	state := a.clientState
	if state == nil {
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !state.UsesAudioIdleClock() || !state.AudioIdleStarted() || state.AudioIdlePaused() {
				continue
			}
			if !state.ShouldCountAudioIdleTimeout() || state.Asr.HasReceivedText() {
				continue
			}
			if state.GetClientVoiceStop() || state.AudioIdleTimeoutPending() {
				continue
			}

			elapsed := state.GetAudioIdleElapsed(time.Now())
			threshold := time.Duration(state.GetMaxIdleDuration()) * time.Millisecond
			if elapsed < threshold {
				continue
			}
			if !state.MarkAudioIdleTimeoutPending() {
				continue
			}

			if !state.Asr.HasOpenAudioInput() {
				log.Infof(
					"Timeout nhàn rỗi âm thanh, hiện không có luồng ASR nào đang hoạt động, đóng phiên trực tiếp: device=%s, mode=%s, elapsed=%dms, threshold=%dms",
					state.DeviceID,
					state.ListenMode,
					elapsed.Milliseconds(),
					state.GetMaxIdleDuration(),
				)
				if a.session != nil {
					a.session.CloseWithReason(chatSessionCloseReasonAudioIdleTimeout)
				} else {
					state.ClearAudioIdleTimeoutPending()
				}
				continue
			}

			log.Infof(
				"Timeout nhàn rỗi âm thanh, kích hoạt kết thúc ASR: device=%s, mode=%s, elapsed=%dms, threshold=%dms",
				state.DeviceID,
				state.ListenMode,
				elapsed.Milliseconds(),
				state.GetMaxIdleDuration(),
			)
			state.OnVoiceSilence()
		}
	}
}

// ProcessVadAudio khởi động xử lý âm thanh VAD
func (a *ASRManager) ProcessVadAudio(ctx context.Context) {
	state := a.clientState
	go func() {
		hasTriggeredCancel := true // cờ đánh dấu đã kích hoạt thao tác hủy hay chưa (khi voiceDuration > 120)
		hasLoggedFirstTextExtendedWait := false
		speakerInterruptTriggered := atomic.Bool{}
		speakerPeekInFlight := atomic.Bool{}
		lastSpeakerPeekDoneAt := atomic.Int64{}
		var speakerPeekAudioMs int64
		var speakerPeekRequestSeq uint64
		const speakerPeekInterval = 200 * time.Millisecond
		const firstSpeakerPeekAudioThresholdMs int64 = 400
		audioFormat := state.InputAudioFormat
		// Dùng một buffer đủ lớn để giải mã (giả định thời lượng khung tối đa là 120ms)
		maxFrameSize := audioFormat.SampleRate * audioFormat.Channels * 120 / 1000
		audioProcesser, err := audio.GetAudioProcesser(audioFormat.SampleRate, audioFormat.Channels, 20) // truyền một giá trị mặc định để tạo bộ giải mã
		if err != nil {
			log.Errorf("Lấy bộ giải mã thất bại: %v", err)
			return
		}

		// Lấy kích thước khung và thời lượng khung từ dữ liệu khung thực tế đầu tiên
		var frameSize int
		var frameDurationMs int
		var vadNeedGetCount int // số khung mà VAD cần, sẽ được tính sau khung đầu tiên

		// Tài nguyên VAD chuyển sang lazy-load + giải phóng khi rảnh, tránh chiếm dụng instance resource pool lâu dài.
		var vadWrapper *pool.ResourceWrapper[inter.VAD]
		var vadProvider inter.VAD
		var vadLastUseAt time.Time
		const vadIdleReleaseTimeout = 2 * time.Second
		vadIdleTicker := time.NewTicker(time.Second)
		defer vadIdleTicker.Stop()
		needVad := !(state.Asr.AutoEnd || state.ListenMode == "manual")
		vadProviderName := state.DeviceConfig.Vad.Provider
		vadProviderConfig := state.DeviceConfig.Vad.Config
		effectiveVadProviderName := vadProviderName
		if configProvider, ok := vadProviderConfig["provider"].(string); ok && configProvider != "" {
			effectiveVadProviderName = configProvider
		}
		isSileroVAD := effectiveVadProviderName == "silero_vad"
		releaseVad := func(reason string) {
			if vadWrapper == nil {
				return
			}
			pool.Release(vadWrapper)
			vadWrapper = nil
			vadProvider = nil
			vadLastUseAt = time.Time{}
			log.Debugf("Giải phóng tài nguyên VAD: device=%s, reason=%s", state.DeviceID, reason)
		}
		defer releaseVad("process_exit")
		ensureVad := func() bool {
			if !needVad {
				return false
			}
			if vadProvider != nil {
				return true
			}

			// Kiểm tra provider có rỗng không, nếu rỗng thì ghi log cảnh báo
			if vadProviderName == "" {
				log.Warnf("VAD provider rỗng, thử lấy từ config")
			} else {
				log.Debugf("Lấy tài nguyên VAD: provider=%s", vadProviderName)
			}

			wrapper, err := pool.Acquire[inter.VAD](
				"vad",
				vadProviderName,
				vadProviderConfig,
			)
			if err != nil {
				log.Errorf("Lấy tài nguyên VAD thất bại: provider=%s, config=%+v, error=%v", vadProviderName, vadProviderConfig, err)
				return false
			}
			vadWrapper = wrapper
			vadProvider = wrapper.GetProvider()
			vadLastUseAt = time.Now()
			return true
		}
		for {
			// Dùng kích thước khung tối đa làm buffer, sau khi giải mã sẽ có kích thước khung thực tế
			pcmFrame := make([]float32, maxFrameSize)

			select {
			case <-vadIdleTicker.C:
				if vadWrapper != nil && !vadLastUseAt.IsZero() && time.Since(vadLastUseAt) >= vadIdleReleaseTimeout {
					releaseVad("idle_timeout")
				}
				continue
			case opusFrame, ok := <-state.OpusAudioBuffer:
				//log.Debugf("processAsrAudio nhận được dữ liệu âm thanh, len: %d", len(opusFrame))
				if !ok {
					log.Debugf("processAsrAudio channel âm thanh đã đóng")
					return
				}

				var skipVad bool
				var haveVoice bool
				clientHaveVoice := state.GetClientHaveVoice()
				if state.ListenMode == "manual" {
					skipVad = true         //bỏ qua vad
					clientHaveVoice = true //trước đó có tiếng
					haveVoice = true       //lần này có tiếng
				} else if state.Asr.AutoEnd {
					skipVad = true   // vẫn để provider tự kiểm soát stop, nhưng không thay đổi ngữ nghĩa idle
					haveVoice = true // âm thanh lần này đi thẳng vào ASR
				}

				if state.GetClientVoiceStop() { //đã dừng nói thì không nhận dữ liệu âm thanh nữa
					//log.Infof("Client đã dừng nói, bỏ qua dữ liệu âm thanh")
					continue
				}

				//log.Debugf("clientVoiceStop: %+v, asrDataSize: %d, listenMode: %s, isSkipVad: %v\n", state.GetClientVoiceStop(), state.AsrAudioBuffer.GetAsrDataSize(), state.ListenMode, skipVad)

				n, err := audioProcesser.DecoderFloat32(opusFrame, pcmFrame)
				if err != nil {
					log.Errorf("Giải mã thất bại: %v", err)
					continue
				}

				// Tính động kích thước khung và thời lượng khung từ dữ liệu đã giải mã thực tế
				if frameSize == 0 {
					// Khung đầu tiên: tính thông tin khung từ dữ liệu giải mã thực tế
					frameSize = n
					samplesPerChannel := n / audioFormat.Channels
					frameDurationMs = samplesPerChannel * 1000 / audioFormat.SampleRate
					audioFormat.FrameDuration = frameDurationMs

					// Tính số khung mà VAD cần
					vadNeedGetCount = 1
					log.Debugf("Tính thông tin khung từ dữ liệu âm thanh thực tế: frameSize=%d, frameDurationMs=%d, vadNeedGetCount=%d", frameSize, frameDurationMs, vadNeedGetCount)
				}

				var vadPcmData []float32
				pcmData := pcmFrame[:n]
				speakerPcmData := pcmFrame[:n]

				// Kiểm tra kích thước khung có nhất quán không (bình thường sẽ nhất quán, nhưng nếu không thì dùng giá trị thực tế)
				if n != frameSize {
					log.Debugf("Kích thước khung không nhất quán: kỳ vọng=%d, thực tế=%d, dùng giá trị thực tế", frameSize, n)
					// Tính lại thời lượng của khung này
					samplesPerChannel := n / audioFormat.Channels
					currentFrameDurationMs := samplesPerChannel * 1000 / audioFormat.SampleRate
					frameSize = n
					frameDurationMs = currentFrameDurationMs
					audioFormat.FrameDuration = frameDurationMs
				}

				if !skipVad && needVad {
					if !ensureVad() {
						continue
					}
					//decode opus to pcm
					state.AsrAudioBuffer.AddAsrAudioData(pcmData)

					// Tính lượng dữ liệu tối thiểu mà VAD cần
					vadNeedMinSize := frameSize

					if state.AsrAudioBuffer.GetAsrDataSize() >= vadNeedMinSize {
						if isSileroVAD {
							vadPcmData = pcmData
						} else {
							vadPcmData = state.AsrAudioBuffer.GetAsrData(vadNeedGetCount, frameSize)
						}

						//nếu đã phát hiện giọng nói, thì không thực hiện kiểm tra vad nữa, truyền pcmData trực tiếp cho asr
						// Dùng tài nguyên VAD lấy ở ngoài vòng lặp để kiểm tra
						if !isSileroVAD {
							vadLastUseAt = time.Now()
							if err := vadProvider.Reset(); err != nil {
								log.Errorf("Reset vad thất bại: %v", err)
								continue
							}
						}

						// Thực hiện kiểm tra VAD
						vadLastUseAt = time.Now()
						haveVoice, err = vadProvider.IsVADExt(vadPcmData, audioFormat.SampleRate, frameSize)
						if err != nil {
							log.Errorf("processAsrAudio kiểm tra VAD thất bại: %v", err)
							continue
						}

						//khi lần đầu phát hiện giọng nói, để đảm bảo tính toàn vẹn dữ liệu âm thanh, gán vadPcmData cho pcmData, dữ liệu âm thanh sau đó sẽ đi hết vào asr
						if haveVoice && !clientHaveVoice {
							//lần đầu phát hiện giọng nói, chỉ giữ lại tối đa 200ms dữ liệu tĩnh lặng phía trước
							currentFrameSamples := len(pcmData)
							allData := state.AsrAudioBuffer.GetAndClearAllData()
							pcmData = trimFirstSpeechAudio(allData, currentFrameSamples, audioFormat.SampleRate, audioFormat.Channels)
						}
					}
					//log.Debugf("isVad, pcmData len: %d, vadPcmData len: %d, haveVoice: %v", len(pcmData), len(vadPcmData), haveVoice)
				}

				if haveVoice {
					hasLoggedFirstTextExtendedWait = false
					//log.Infof("Phát hiện giọng nói, len: %d", len(pcmData))
					state.SetClientHaveVoice(true)
					state.SetClientHaveVoiceLastTime(time.Now().UnixMilli())
					state.Vad.ResetIdleDuration()
					// Tích lũy thời lượng phát hiện giọng nói (đồng thời cập nhật thời lượng trong quá trình)
					state.Vad.AddVoiceDuration(int64(frameDurationMs))

					continuousVoiceDuration := state.Vad.GetVoiceContinuousDuration()
					if state.IsRealTime() && viper.GetInt("chat.realtime_mode") == 1 && continuousVoiceDuration > 360 {
						// Chỉ thực hiện khi chưa từng kích hoạt, đảm bảo chỉ thực hiện một lần
						if !hasTriggeredCancel {
							if a.session != nil && a.session.isRealtimeMcpAudioGateActive() {
								log.Debugf("Thiết bị %s cổng phát media realtime đang kích hoạt, bỏ qua ngắt VAD", state.DeviceID)
								hasTriggeredCancel = true
							} else {
								//ở chế độ realtime, nếu đang có llm và tts đang chạy thì hủy chúng
								log.Debugf("Ngắt VAD ở chế độ realtime && thời lượng giọng nói vượt quá %d ms, nếu đang có llm và tts đang chạy thì hủy chúng", continuousVoiceDuration)
								if a.session != nil {
									a.session.StopAssistantOutputAfterAsrWithReason(true, "ASRManager.ProcessVadAudio realtime_mode=1 VAD interrupt")
								} else {
									state.AfterAsrSessionCtx.CancelWithReason("ASRManager.ProcessVadAudio: realtime_mode=1 VAD interrupt")
								}
								hasTriggeredCancel = true // đánh dấu đã kích hoạt
							}
						}
					}
				} else {
					state.Vad.AddIdleDuration(int64(frameDurationMs))
					state.Vad.ResetVoiceContinuousDuration()

					// Khi không có tiếng, nếu trước đó cũng không có tiếng, thì reset thời lượng tiếng nói đã tích lũy
					// Nếu trước đó có tiếng nhưng lần này không có, giữ lại giá trị thời lượng, để logic sau quyết định có nên reset hay không
					if !clientHaveVoice {
						speakerInterruptTriggered.Store(false)
						lastSpeakerPeekDoneAt.Store(0)
						speakerPeekAudioMs = 0
						//giữ lại khoảng 10 khung gần nhất
						/*
							if state.AsrAudioBuffer.GetFrameCount(frameSize) > vadNeedGetCount*3 {
								state.AsrAudioBuffer.RemoveAsrAudioData(1, frameSize)
							}*/
						continue
					}
				}

				if clientHaveVoice || haveVoice {
					// Khi lần đầu bắt được giọng nói cũng cần forward ngay khung dữ liệu hiện đang cache, tránh giọng nói cực ngắn không được gửi hết vào ASR.

					//vad nhận dạng thành công, gửi dữ liệu vào channel âm thanh của asr
					//log.Infof("vad nhận dạng thành công, gửi dữ liệu vào channel âm thanh của asr, len: %d", len(pcmData))
					state.Asr.AddAudioData(pcmData)

					// Vân giọng nói (speaker) chỉ nhận các khung được xác định là có tiếng ở hiện tại, tránh việc gửi đoạn tĩnh lặng đầu/cuối vào luồng nhận dạng.
					if haveVoice &&
						state.IsSpeakerEnabled() && state.HasSpeakerGroups() &&
						a.session != nil && a.session.speakerManager != nil {
						// Lần đầu phát hiện giọng nói, khởi động luồng nhận dạng streaming
						if !a.session.speakerManager.IsActive() {
							sampleRate := audioFormat.SampleRate
							agentId := a.session.clientState.AgentID
							if err := a.session.speakerManager.StartStreaming(ctx, sampleRate, agentId); err != nil {
								log.Warnf("Khởi động luồng nhận dạng vân giọng nói thất bại: %v", err)
							} else {
								speakerInterruptTriggered.Store(false)
								lastSpeakerPeekDoneAt.Store(0)
								speakerPeekAudioMs = 0
							}
						}

						// Gửi khối âm thanh
						if err := a.session.speakerManager.SendAudioChunk(ctx, speakerPcmData); err != nil {
							log.Warnf("Gửi khối âm thanh đến dịch vụ nhận dạng vân giọng nói thất bại: %v", err)
						} else if a.session.speakerManager.IsActive() {
							if audioFormat.Channels > 0 && audioFormat.SampleRate > 0 {
								speakerPeekAudioMs += int64(len(speakerPcmData)/audioFormat.Channels) * 1000 / int64(audioFormat.SampleRate)
							}

							if state.IsRealTime() &&
								viper.GetInt("chat.realtime_mode") == 3 &&
								!speakerInterruptTriggered.Load() &&
								speakerPeekAudioMs >= firstSpeakerPeekAudioThresholdMs {
								now := time.Now()
								lastDoneAt := lastSpeakerPeekDoneAt.Load()
								if (lastDoneAt <= 0 || now.Sub(time.Unix(0, lastDoneAt)) >= speakerPeekInterval) &&
									speakerPeekInFlight.CompareAndSwap(false, true) {
									reqSeq := atomic.AddUint64(&speakerPeekRequestSeq, 1)
									requestID := fmt.Sprintf("peek_%d_%d", now.UnixMilli(), reqSeq)

									go func(reqID string) {
										defer func() {
											lastSpeakerPeekDoneAt.Store(time.Now().UnixNano())
											speakerPeekInFlight.Store(false)
										}()

										if a.session == nil || a.session.speakerManager == nil || !a.session.speakerManager.IsActive() {
											return
										}

										peekCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
										defer cancel()

										peekResult, throttled, err := a.session.speakerManager.PeekAndIdentify(peekCtx, reqID)
										if err != nil {
											if ctx.Err() == nil {
												log.Debugf("Peek vân giọng nói thất bại: device=%s, request_id=%s, err=%v", state.DeviceID, reqID, err)
											}
											return
										}
										if throttled {
											return
										}
										if peekResult == nil || !peekResult.Identified {
											return
										}
										if !speakerInterruptTriggered.CompareAndSwap(false, true) {
											return
										}

										log.Infof(
											"Peek vân giọng nói ở chế độ realtime khớp, ngắt ngay lập tức: device=%s, speaker=%s, confidence=%.4f, threshold=%.4f",
											state.DeviceID,
											peekResult.SpeakerName,
											peekResult.Confidence,
											peekResult.Threshold,
										)
										if a.session != nil && a.session.isRealtimeMcpAudioGateActive() {
											log.Debugf("Thiết bị %s cổng phát media realtime đang kích hoạt, bỏ qua ngắt speaker peek", state.DeviceID)
											return
										}
										a.session.MarkTurnSpeakerInterrupted()
										if a.session != nil {
											a.session.StopAssistantOutputAfterAsrWithReason(true, "ASRManager.ProcessVadAudio realtime_mode=3 speaker peek interrupt")
										} else {
											state.AfterAsrSessionCtx.CancelWithReason("ASRManager.ProcessVadAudio: realtime_mode=3 speaker peek interrupt")
										}
									}(requestID)
								}
							}
						}
					}
				}

				//đã có giọng nói rồi, nhưng lần này không phát hiện được giọng nói, cần xác định xem đã dừng nói hay chưa
				lastHaveVoiceTime := state.GetClientHaveVoiceLastTime()

				if clientHaveVoice && lastHaveVoiceTime > 0 && !haveVoice {
					// Kiểm tra thời lượng giọng nói có âm thanh, nếu nhỏ hơn 300ms thì reset clientHaveVoice, tránh nhận định sai do giọng nói quá ngắn
					voiceDurationInSession := state.Vad.GetVoiceDurationInSession()
					if voiceDurationInSession < 100 {
						log.Debugf("Thời lượng giọng nói quá ngắn (%dms < 300ms), reset clientHaveVoice", voiceDurationInSession)
						state.SetClientHaveVoice(false)
						state.Vad.ResetVoiceDuration()
						speakerInterruptTriggered.Store(false)
						lastSpeakerPeekDoneAt.Store(0)
						speakerPeekAudioMs = 0
						continue
					}

					idleDuration := state.Vad.GetIdleDuration()
					if state.IsRealTime() && !state.Asr.HasReceivedText() {
						preTextSilenceDuration := state.GetPreAsrTextSilenceDuration()
						if idleDuration <= preTextSilenceDuration {
							log.Debugf(
								"Chế độ realtime chưa nhận được văn bản ASR đầu tiên, trì hoãn kết thúc theo ngưỡng tĩnh lặng: status=%s, idle=%dms, pre_text_timeout=%dms, voice_duration=%dms, voice_duration_in_session=%dms, history_audio_samples=%d",
								state.Status,
								idleDuration,
								preTextSilenceDuration,
								state.Vad.GetVoiceDuration(),
								voiceDurationInSession,
								state.Asr.GetHistoryAudioLen(),
							)
							continue
						}

						if !hasLoggedFirstTextExtendedWait {
							log.Debugf(
								"Chế độ realtime đã timeout tĩnh lặng nhưng vẫn chưa nhận được văn bản ASR, tiếp tục giữ luồng ASR hiện tại và forward âm thanh: status=%s, idle=%dms, pre_text_timeout=%dms, voice_duration=%dms, voice_duration_in_session=%dms, history_audio_samples=%d",
								state.Status,
								idleDuration,
								preTextSilenceDuration,
								state.Vad.GetVoiceDuration(),
								voiceDurationInSession,
								state.Asr.GetHistoryAudioLen(),
							)
							hasLoggedFirstTextExtendedWait = true
						}
						continue
					}

					if state.IsSilence(idleDuration) { //xác định từ có tiếng chuyển sang im lặng
						log.Debugf(
							"Xác định giọng nói đã kết thúc, chuẩn bị dừng ASR: status=%s, idle=%dms, voice_duration=%dms, voice_duration_in_session=%dms, history_audio_samples=%d, pending_restart=%v",
							state.Status,
							idleDuration,
							state.Vad.GetVoiceDuration(),
							state.Vad.GetVoiceDurationInSession(),
							state.Asr.GetHistoryAudioLen(),
							state.AudioIdleTimeoutPending(),
						)
						// Reset cờ trước khi gọi OnVoiceSilence, để lần sau có thể kích hoạt lại
						hasTriggeredCancel = false
						speakerInterruptTriggered.Store(false)
						lastSpeakerPeekDoneAt.Store(0)
						speakerPeekAudioMs = 0
						state.OnVoiceSilence()
						//state.VoiceStatus.Reset()
						continue
					}
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}

// releaseResource giải phóng tài nguyên ASR (phương thức nội bộ)
func (a *ASRManager) releaseResource() {
	a.resourceMu.Lock()
	defer a.resourceMu.Unlock()
	if a.asrResource != nil {
		pool.Release(a.asrResource)
		a.asrResource = nil
		log.Debugf("Tài nguyên ASR đã được trả lại")
	}
}

// Cleanup dọn dẹp tài nguyên ASR (dùng cho bên ngoài gọi)
func (a *ASRManager) Cleanup() {
	a.releaseResource()
}

// restartAsrRecognition khởi động lại nhận dạng ASR
func (a *ASRManager) RestartAsrRecognition(ctx context.Context) error {
	state := a.clientState
	log.Debugf("Bắt đầu khởi động lại nhận dạng ASR")
	if a.session != nil {
		a.session.ResetTurnSpeakerInterrupted()
	}

	// Hủy context ASR hiện tại
	state.Asr.CancelWithReason("ASRManager.RestartAsrRecognition: cancel previous ASR context before restart")

	state.Asr.ResetReceivedText()
	state.VoiceStatus.Reset()
	state.AsrAudioBuffer.ClearAsrAudioData()
	state.Asr.ClearHistoryAudio() // Xóa cache âm thanh lịch sử

	// Kiểm tra xem đã có tài nguyên chưa, nếu chưa thì lấy mới
	a.resourceMu.Lock()
	var asrProvider asr.AsrProvider
	if a.asrResource == nil {
		// Cần lấy tài nguyên mới
		a.resourceMu.Unlock()

		asrWrapper, err := pool.Acquire[asr.AsrProvider](
			"asr",
			state.DeviceConfig.Asr.Provider,
			state.DeviceConfig.Asr.Config,
		)
		if err != nil {
			log.Errorf("Lấy tài nguyên ASR thất bại: %v", err)
			return fmt.Errorf("lấy tài nguyên ASR thất bại: %w", err)
		}

		// Lưu tham chiếu tài nguyên vào field private
		a.resourceMu.Lock()
		a.asrResource = asrWrapper
		asrProvider = asrWrapper.GetProvider()
		a.resourceMu.Unlock()
		log.Debugf("Đã lấy tài nguyên ASR mới")
	} else {
		// Tái sử dụng tài nguyên hiện có
		asrProvider = a.asrResource.GetProvider()
		a.resourceMu.Unlock()
		log.Debugf("Tái sử dụng tài nguyên ASR hiện có")
	}

	// Tạo lại context và channel ASR
	state.Asr.Ctx, state.Asr.Cancel = context.WithCancel(ctx)
	state.Asr.AsrAudioChannel = make(chan []float32, 100)

	// Khởi động lại nhận dạng streaming
	asrResultChannel, err := asrProvider.StreamingRecognize(state.Asr.Ctx, state.Asr.AsrAudioChannel)
	if err != nil {
		// Nhận dạng thất bại, trả lại tài nguyên (vì tài nguyên có thể đã hỏng)
		a.releaseResource()
		log.Errorf("Khởi động lại nhận dạng streaming ASR thất bại: %v", err)
		return fmt.Errorf("khởi động lại nhận dạng streaming ASR thất bại: %w", err)
	}

	state.AsrResultChannel = asrResultChannel
	// Reset thời gian thống kê, dùng để tính tổng thời gian của lượt hội thoại này
	state.MarkTurnStart()
	if a.session != nil {
		a.session.TraceTurnStart(state.Asr.Ctx, state.Statistic.TurnStartTs)
	}
	log.Debugf("Khởi động lại nhận dạng ASR thành công")
	return nil
}

// StartAsrRecognitionLoop khởi động vòng lặp xử lý kết quả nhận dạng ASR
// onMessageSave: hàm callback lưu tin nhắn
// onError: hàm callback xử lý lỗi (ví dụ đóng phiên)
func (a *ASRManager) StartAsrRecognitionLoop(
	ctx context.Context,
	onMessageSave AsrMessageSaveCallback,
	onError func(error),
) {
	state := a.clientState

	// Khởi động một goroutine xử lý kết quả asr
	go func() {
		// Dùng defer để đảm bảo giải phóng tài nguyên ASR khi goroutine thoát
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("goroutine xử lý kết quả asr panic: %v, stack: %s", r, string(debug.Stack()))
			}
			// Dù thoát bình thường hay panic, đều giải phóng tài nguyên
			a.releaseResource()
		}()

		//tối đa nhàn rỗi 60s
		var startIdleTime, maxIdleTime int64
		startIdleTime = time.Now().Unix()
		maxIdleTime = 60

		// Đếm số lần chờ khi trạng thái không cho phép restart (tránh vòng lặp vô hạn)
		var invalidStatusWaitCount int64
		maxInvalidStatusWaitCount := int64(10) // chờ tối đa 10 lần (khoảng 1 giây)

		// Bảo vệ ngắn hạn với kết quả rỗng: tránh dịch vụ ASR bất thường liên tục trả về chuỗi rỗng khiến luồng chính bị vòng lặp chết
		const emptyResultProtectWindow = 3 * time.Second
		const maxEmptyResultInWindow = 3
		emptyResultWindowStart := time.Now()
		emptyResultCount := 0

		// Bảo vệ ngắn hạn với lỗi có thể khôi phục: tránh upstream liên tục trả về lỗi instance mất hiệu lực khiến việc kết nối lại vô hạn
		const recoverableErrorProtectWindow = 10 * time.Second
		const maxRecoverableErrorInWindow = 3
		recoverableErrorWindowStart := time.Now()
		recoverableErrorCount := 0

		isAllowedToRestart := func() bool {
			allowed := state.Status == ClientStatusListening || state.Status == ClientStatusListenStop
			if state.IsRealTime() {
				allowed = state.Status != ClientStatusInit
			}
			return allowed
		}
		resumeAudioIdle := func() {
			state.ResumeAudioIdleWindow(time.Now())
		}
		startAudioIdle := func() {
			state.StartAudioIdleWindow(time.Now())
		}
		closeAudioIdleTimeout := func(reason string) {
			if !state.AudioIdleTimeoutPending() {
				return
			}

			state.ClearAudioIdleTimeoutPending()
			log.Infof("Hoàn tất kết thúc do timeout nhàn rỗi âm thanh: device=%s, reason=%s", state.DeviceID, reason)
			if a.session != nil {
				a.session.CloseWithReason(chatSessionCloseReasonAudioIdleTimeout)
				return
			}
			if onError != nil {
				onError(fmt.Errorf("audio idle timeout: %s", reason))
			}
		}

		for {
			select {
			case <-ctx.Done():
				log.Debugf("asr ctx done")
				return
			default:
			}

			result, isRetry, err := state.RetireAsrResult(ctx)
			if err != nil {
				if ctx.Err() != nil || errors.Is(err, context.Canceled) {
					log.Debugf("Xử lý kết quả asr thất bại, ASR đã bị hủy: %v", err)
				} else {
					log.Errorf("Xử lý kết quả asr thất bại: %v", err)
				}
				if onError != nil {
					onError(err)
				}
				return
			}
			if !isRetry {
				log.Debugf("asrResult is not retry, return")
				return
			}
			text := result.Text

			if result.RetryReason != "" {
				if state.AudioIdleTimeoutPending() {
					closeAudioIdleTimeout(result.RetryReason)
					return
				}

				now := time.Now()
				if now.Sub(recoverableErrorWindowStart) > recoverableErrorProtectWindow {
					recoverableErrorWindowStart = now
					recoverableErrorCount = 0
				}
				recoverableErrorCount++
				log.Warnf(
					"Lỗi có thể khôi phục của ASR: reason=%s, count=%d/%d, status=%s",
					result.RetryReason,
					recoverableErrorCount,
					maxRecoverableErrorInWindow,
					state.Status,
				)

				if recoverableErrorCount >= maxRecoverableErrorInWindow {
					err := fmt.Errorf("ASR liên tục gặp lỗi có thể khôi phục trong thời gian ngắn (%d lần/%s), dừng retry và ngắt kết nối", recoverableErrorCount, recoverableErrorProtectWindow)
					log.Errorf("%v", err)
					if onError != nil {
						onError(err)
					}
					return
				}

				switch result.RetryReason {
				case asr_types.RetryReasonDoubaoResponseCode45000081, asr_types.RetryReasonXunfeiServiceInstanceInvalid, asr_types.RetryReasonAliyunQwen3ConnectionClosed:
					a.releaseResource()
					if isAllowedToRestart() {
						invalidStatusWaitCount = 0
						if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
							log.Errorf("Khởi động lại nhận dạng sau lỗi có thể khôi phục của ASR thất bại: reason=%s, err=%v", result.RetryReason, restartErr)
							if onError != nil {
								onError(restartErr)
							}
							return
						}
						resumeAudioIdle()
						continue
					}

					log.Warnf("Trạng thái hiện tại không cho phép restart ngay khi xảy ra lỗi có thể khôi phục của ASR: reason=%s, status=%s, realtime=%v", result.RetryReason, state.Status, state.IsRealTime())
					state.Asr.CancelWithReason("ASRManager.StartAsrRecognitionLoop: recoverable error but restart not allowed yet")
					resumeAudioIdle()
					continue
				case asr_types.RetryReasonDoubaoWaitingNextPacketTimeout:
					log.Warnf("Phiên ASR doubao timeout nhàn rỗi, tạm ngưng luồng hiện tại và chờ lần nói tiếp theo để tái tạo")
					state.Asr.CancelWithReason("ASRManager.StartAsrRecognitionLoop: doubao waiting next packet timeout")
					resumeAudioIdle()
					continue
				}
			}

			if text != "" {
				asrFinalTs := time.Now().UnixMilli()
				state.MarkAsrFinalTextAt(asrFinalTs)
				if a.session != nil {
					a.session.TraceAsrFinalText(ctx, asrFinalTs)
				}
				log.Debugf("Xử lý kết quả asr: %s, thời gian xử lý: %d ms", text, state.GetAsrDuration())

				state.ClearAudioIdleTimeoutPending()
				// Sau khi nhận dạng thành công, reset bộ đếm kết quả rỗng
				emptyResultWindowStart = time.Now()
				emptyResultCount = 0
				recoverableErrorWindowStart = time.Now()
				recoverableErrorCount = 0

				//nếu ở chế độ realtime, cần dừng llm và tts hiện tại
				if state.IsRealTime() && viper.GetInt("chat.realtime_mode") == 2 {
					shouldInterrupt := true
					if a.session != nil && a.session.isRealtimeMcpAudioGateActive() {
						shouldInterrupt = false
						log.Debugf("Thiết bị %s cổng phát media realtime đang kích hoạt, trì hoãn đến khi có phán định final ASR, bỏ qua việc ngắt bởi kết quả ASR", state.DeviceID)
					}
					if shouldInterrupt {
						log.Debugf("OnListenStart ở chế độ realtime, dừng llm và tts hiện tại")
						if a.session != nil {
							a.session.StopAssistantOutputAfterAsrWithReason(true, "ASRManager.StartAsrRecognitionLoop realtime_mode=2 ASR result interrupt")
						} else {
							state.AfterAsrSessionCtx.CancelWithReason("ASRManager.StartAsrRecognitionLoop: realtime_mode=2 ASR result interrupt")
						}
					}
				}

				// Reset bộ đếm retry
				startIdleTime = time.Now().Unix()

				//khi lấy được kết quả asr, kết thúc đầu vào giọng nói (OnVoiceSilence sẽ lấy kết quả vân giọng nói bất đồng bộ)
				state.OnVoiceSilence()

				// Lấy kết quả vân giọng nói tạm lưu (có timeout)
				speakerResult := a.getSpeakerResult()
				speakerInterrupted := false
				if a.session != nil {
					speakerInterrupted = a.session.ConsumeTurnSpeakerInterrupted()
				}

				if a.session != nil {
					payload, stop, hookErr := a.session.hookHub.EmitASROutput(a.session.hookContext(ctx), chathooks.ASROutputData{Text: text, SpeakerResult: speakerResult})
					if hookErr != nil {
						log.Warnf("Thực thi hook ASR_OUTPUT thất bại: %v", hookErr)
					}
					text = payload.Text
					speakerResult = payload.SpeakerResult
					if stop {
						log.Infof("Hook ASR_OUTPUT yêu cầu dừng luồng xử lý hiện tại")
						state.Asr.ClearHistoryAudio()
						if state.UsesAudioIdleClock() {
							startAudioIdle()
						} else {
							state.ResetAudioIdleWindow()
						}
						continue
					}
				}

				if a.session != nil {
					allowChat, denyReason := a.session.ShouldAllowSpeakerChat(speakerResult, speakerInterrupted)
					if !allowChat {
						log.Infof(
							"Bỏ kết quả ASR và bỏ qua STT/LLM: device=%s, reason=%s, speaker_interrupted=%v, speaker_result=%+v, text=%q",
							state.DeviceID,
							denyReason,
							speakerInterrupted,
							speakerResult,
							text,
						)
						state.Asr.ClearHistoryAudio()

						if !state.IsRealTime() {
							startAudioIdle()
							return
						}
						if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
							log.Errorf("Khởi động lại nhận dạng sau khi bỏ kết quả ASR thất bại: %v", restartErr)
							if onError != nil {
								onError(restartErr)
							}
							return
						}
						startAudioIdle()
						continue
					}
				}

				// Tạo tin nhắn người dùng, dùng văn bản đã được hook viết lại để đi vào chuỗi hiệu ứng phụ tiếp theo
				userMsg := &schema.Message{
					Role:    schema.User,
					Content: text,
				}

				// Sinh MessageID (dùng hash MD5 để rút ngắn độ dài, tránh vượt quá giới hạn varchar(64) của database)
				// Định dạng gốc: {SessionID}-{Role}-{Timestamp}
				rawMessageID := fmt.Sprintf("%s-%s-%d",
					state.SessionID,
					userMsg.Role,
					time.Now().UnixMilli())
				// Dùng hash MD5 để sinh chuỗi hex cố định 32 ký tự
				hash := md5.Sum([]byte(rawMessageID))
				messageID := hex.EncodeToString(hash[:])

				// Đồng bộ thêm vào bộ nhớ (dùng cho context LLM)
				state.AddMessage(userMsg)

				// Lấy dữ liệu âm thanh (âm thanh lịch sử ASR)
				audioData := state.Asr.GetHistoryAudio()
				state.Asr.ClearHistoryAudio()

				// Lưu tin nhắn qua callback
				if onMessageSave != nil {
					onMessageSave(userMsg, messageID, audioData)
				}

				// Kết quả ASR gửi cho client cũng dùng văn bản đã được hook viết lại
				err = a.serverTransport.SendAsrResult(text)
				if err != nil {
					log.Errorf("Gửi tin nhắn asr thất bại: %v", err)
					if onError != nil {
						onError(err)
					}
					return
				}

				if a.session != nil {
					handledByRealtimeGate, gateErr := a.session.tryHandleRealtimeMcpAudioASR(ctx, text)
					if gateErr != nil {
						log.Warnf("Điều khiển nhanh phát media realtime thất bại: device=%s text=%q err=%v", state.DeviceID, text, gateErr)
					}
					if handledByRealtimeGate {
						if !state.IsRealTime() {
							return
						}
						if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
							log.Errorf("Khởi động lại nhận dạng ASR sau điều khiển media realtime thất bại: %v", restartErr)
							if onError != nil {
								onError(restartErr)
							}
							return
						}
						startAudioIdle()
						continue
					}
				}

				// Thêm vào hàng đợi (đã chuyển sang xử lý trong ASRManager)
				if err := a.addAsrResultToQueue(text, speakerResult); err != nil {
					log.Errorf("Bắt đầu hội thoại thất bại: %v", err)
					if onError != nil {
						onError(err)
					}
					return
				}

				// Ở chế độ không realtime, nhận dạng ASR hoàn tất, trả lại tài nguyên
				// Ở chế độ realtime, tài nguyên sẽ được tự động quản lý trong RestartAsrRecognition (trả tài nguyên cũ trước rồi mới lấy tài nguyên mới)
				if !state.IsRealTime() {
					return
				}

				// Ở chế độ realtime, khởi động lại nhận dạng ASR (RestartAsrRecognition sẽ trả tài nguyên cũ trước rồi mới lấy tài nguyên mới)
				if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
					log.Errorf("Khởi động lại nhận dạng ASR thất bại: %v", restartErr)
					if onError != nil {
						onError(restartErr)
					}
					return
				}
				// Ở chế độ realtime, tiếp tục vòng lặp xử lý kết quả ASR tiếp theo
				continue
			} else {
				log.Debugf(
					"Chi tiết kết quả rỗng của ASR: status=%s, emptyReason=%s, client_voice_stop=%v, history_audio_samples=%d, voice_duration=%dms, voice_duration_in_session=%dms, idle_duration=%dms, realtime=%v",
					state.Status,
					result.EmptyReason,
					state.GetClientVoiceStop(),
					state.Asr.GetHistoryAudioLen(),
					state.Vad.GetVoiceDuration(),
					state.Vad.GetVoiceDurationInSession(),
					state.Vad.GetIdleDuration(),
					state.IsRealTime(),
				)
				if state.AudioIdleTimeoutPending() {
					closeAudioIdleTimeout(result.EmptyReason)
					return
				}
				if result.EmptyReason != "" {
					log.Debugf("Kết quả rỗng của ASR đã được phân loại: reason=%s, status=%s", result.EmptyReason, state.Status)
					emptyResultWindowStart = time.Now()
					emptyResultCount = 0

					if result.EmptyReason == asr_types.EmptyReasonNoServerResponse ||
						result.EmptyReason == asr_types.EmptyReasonProviderEmptyFinal {
						state.Asr.CancelWithReason("ASRManager.StartAsrRecognitionLoop: empty final result from provider")
						resumeAudioIdle()
						continue
					}
				}

				now := time.Now()
				if now.Sub(emptyResultWindowStart) > emptyResultProtectWindow {
					emptyResultWindowStart = now
					emptyResultCount = 0
				}
				emptyResultCount++
				if emptyResultCount >= maxEmptyResultInWindow {
					err := fmt.Errorf("ASR liên tục trả về kết quả rỗng trong thời gian ngắn (%d lần/%s), kích hoạt bảo vệ và ngắt kết nối", emptyResultCount, emptyResultProtectWindow)
					log.Errorf("%v", err)
					if onError != nil {
						onError(err)
					}
					return
				}

				// Trường hợp text rỗng
				select {
				case <-ctx.Done():
					log.Debugf("asr ctx done")
					return
				default:
				}

				log.Debugf("ready Restart Asr, state.Status: %s", state.Status)
				// Ở chế độ realtime, ngay cả khi trạng thái là LLMStart hoặc TTSStart, vẫn nên tiếp tục lắng nghe (cho phép restart ASR)
				// Ở chế độ không realtime, chỉ trạng thái Listening hoặc ListenStop mới cho phép restart ASR
				if isAllowedToRestart() {
					// Trạng thái cho phép restart, reset bộ đếm chờ
					invalidStatusWaitCount = 0
					// text rỗng, kiểm tra xem có cần khởi động lại ASR hay không
					diffTs := time.Now().Unix() - startIdleTime
					if startIdleTime > 0 && diffTs <= maxIdleTime {
						log.Warnf("Kết quả nhận dạng ASR rỗng, thử khởi động lại nhận dạng ASR, diff ts: %d", diffTs)
						if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
							log.Errorf("Khởi động lại nhận dạng ASR thất bại: %v", restartErr)
							if onError != nil {
								onError(restartErr)
							}
							return
						}
						resumeAudioIdle()
						continue
					} else {
						log.Warnf("Kết quả nhận dạng ASR rỗng, đã đạt thời gian nhàn rỗi tối đa: %d", maxIdleTime)
						if onError != nil {
							onError(fmt.Errorf("kết quả nhận dạng ASR rỗng, đã đạt thời gian nhàn rỗi tối đa: %d", maxIdleTime))
						}
						return
					}
				} else {
					// Trường hợp trạng thái không cho phép restart, chờ ngắn rồi tiếp tục vòng lặp, tạo cơ hội cho trạng thái phục hồi
					invalidStatusWaitCount++
					if invalidStatusWaitCount >= maxInvalidStatusWaitCount {
						// Chờ quá thời gian, thoát khỏi vòng lặp
						log.Debugf("Trạng thái là %s, realtime: %v, đã chờ %d lần vẫn không đổi, thoát khỏi vòng lặp nhận dạng ASR", state.Status, state.IsRealTime(), maxInvalidStatusWaitCount)
						return
					}
					// Chờ ngắn rồi tiếp tục vòng lặp, chờ trạng thái phục hồi
					log.Debugf("Trạng thái là %s, realtime: %v, không cho phép restart, chờ trạng thái phục hồi (số lần chờ: %d/%d)", state.Status, state.IsRealTime(), invalidStatusWaitCount, maxInvalidStatusWaitCount)
					time.Sleep(200 * time.Millisecond) // chờ 100ms
					continue
				}
			}
		}
	}()
}

func trimFirstSpeechAudio(allData []float32, currentFrameSamples, sampleRate, channels int) []float32 {
	if len(allData) == 0 {
		return nil
	}
	if currentFrameSamples <= 0 || currentFrameSamples > len(allData) || sampleRate <= 0 || channels <= 0 {
		return allData
	}

	maxPreSpeechSamples := sampleRate * channels * maxFirstSpeechPreAudioMs / 1000
	keepSamples := currentFrameSamples + maxPreSpeechSamples
	if keepSamples >= len(allData) {
		return allData
	}

	audio := make([]float32, keepSamples)
	copy(audio, allData[len(allData)-keepSamples:])
	return audio
}

// getSpeakerResult lấy kết quả vân giọng nói tạm lưu (có timeout)
func (a *ASRManager) getSpeakerResult() *speaker.IdentifyResult {
	if a.session == nil || a.session.speakerManager == nil {
		return nil
	}

	log.Debugf("speakerManager: %+v, IsActive: %+v", a.session.speakerManager, a.session.speakerManager.IsActive())

	timeout := time.NewTimer(200 * time.Millisecond)
	defer timeout.Stop()

	var speakerResult *speaker.IdentifyResult
	select {
	case <-a.session.speakerResultReady:
		a.session.speakerResultMu.RLock()
		speakerResult = a.session.pendingSpeakerResult
		a.session.speakerResultMu.RUnlock()
	case <-timeout.C:
		// Sau khi timeout, đọc kết quả hiện tại (có thể là nil)
		a.session.speakerResultMu.RLock()
		speakerResult = a.session.pendingSpeakerResult
		a.session.speakerResultMu.RUnlock()
		log.Debugf("Lấy kết quả nhận dạng vân giọng nói timeout, dùng kết quả hiện tại")
	}
	log.Debugf("Lấy kết quả nhận dạng vân giọng nói: %+v", speakerResult)
	return speakerResult
}

// addAsrResultToQueue thêm kết quả ASR vào hàng đợi (đã chuyển sang xử lý trong ASRManager)
func (a *ASRManager) addAsrResultToQueue(text string, speakerResult *speaker.IdentifyResult) error {
	if a.session == nil {
		return fmt.Errorf("session is nil")
	}
	return a.session.AddAsrResultToQueue(text, speakerResult)
}