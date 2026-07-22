package client

type VoiceStatus struct {
	HaveVoice            bool  // Có giọng nói ở lần kiểm tra trước.
	HaveVoiceLastTime    int64 // Thời điểm nói lần cuối.
	VoiceStop            bool  // Đã dừng nói.
	SilenceThresholdTime int64 // Ngưỡng thời gian im lặng.
}

func (v *VoiceStatus) Reset() {
	v.HaveVoice = false
	v.HaveVoiceLastTime = 0
	v.VoiceStop = false
}

func (v *VoiceStatus) IsSilence(diffMilli int64) bool {
	return diffMilli > v.SilenceThresholdTime
}

func (v *VoiceStatus) GetClientHaveVoice() bool {
	return v.HaveVoice
}

func (v *VoiceStatus) SetClientHaveVoice(haveVoice bool) {
	v.HaveVoice = haveVoice
}

func (v *VoiceStatus) GetClientHaveVoiceLastTime() int64 {
	return v.HaveVoiceLastTime
}

func (v *VoiceStatus) SetClientHaveVoiceLastTime(lastTime int64) {
	v.HaveVoiceLastTime = lastTime
}

func (v *VoiceStatus) GetClientVoiceStop() bool {
	return v.VoiceStop
}

func (v *VoiceStatus) SetClientVoiceStop(voiceStop bool) {
	v.VoiceStop = voiceStop
}
