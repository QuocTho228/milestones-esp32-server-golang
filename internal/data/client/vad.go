package client

import (
	"fmt"
	"milestones-esp32-server-golang/internal/domain/vad"
	vad_inter "milestones-esp32-server-golang/internal/domain/vad/inter"
	"sync"
	"sync/atomic"
)

type Vad struct {
	lock sync.RWMutex
	// Provider VAD
	VadProvider vad_inter.VAD

	IdleDuration           int64 // Thời gian rảnh (idle), đơn vị: ms
	VoiceDuration          int64 // Thời lượng phát hiện giọng nói tích lũy, đơn vị: ms
	VoiceDurationInSession int64 // Thời lượng phát hiện giọng nói tích lũy trong một phiên xử lý, đơn vị: ms
}

func (v *Vad) AddIdleDuration(idleDuration int64) int64 {
	return atomic.AddInt64(&v.IdleDuration, idleDuration)
}

func (v *Vad) GetIdleDuration() int64 {
	return atomic.LoadInt64(&v.IdleDuration)
}

func (v *Vad) ResetIdleDuration() {
	atomic.StoreInt64(&v.IdleDuration, 0)
}

func (v *Vad) AddVoiceDuration(voiceDuration int64) int64 {
	atomic.AddInt64(&v.VoiceDurationInSession, voiceDuration)
	return atomic.AddInt64(&v.VoiceDuration, voiceDuration)
}

func (v *Vad) GetVoiceDuration() int64 {
	return atomic.LoadInt64(&v.VoiceDuration)
}

func (v *Vad) ResetVoiceDuration() {
	atomic.StoreInt64(&v.VoiceDuration, 0)
	atomic.StoreInt64(&v.VoiceDurationInSession, 0)
}

// ResetVoiceContinuousDuration reset thời lượng giọng nói liên tục
func (v *Vad) ResetVoiceContinuousDuration() {
	atomic.StoreInt64(&v.VoiceDuration, 0)
}

func (v *Vad) GetVoiceContinuousDuration() int64 {
	return atomic.LoadInt64(&v.VoiceDuration)
}

func (v *Vad) GetVoiceDurationInSession() int64 {
	return atomic.LoadInt64(&v.VoiceDurationInSession)
}

func (v *Vad) Init(provider string, config map[string]interface{}) error {
	v.lock.Lock()
	defer v.lock.Unlock()
	vadProvider, err := vad.AcquireVAD(provider, config)
	if err != nil {
		return fmt.Errorf("Tạo provider VAD thất bại: %v", err)
	}

	vadProvider.Reset()
	v.VadProvider = vadProvider
	return nil
}

func (v *Vad) ResetVad() error {
	v.lock.Lock()
	defer v.lock.Unlock()
	if v.VadProvider != nil {
		v.VadProvider.Reset()
		return nil
	}
	return fmt.Errorf("vad provider is nil")
}

func (v *Vad) IsVADExt(pcmData []float32, sampleRate int, frameSize int) (bool, error) {
	v.lock.Lock()
	defer v.lock.Unlock()
	if v.VadProvider != nil {
		return v.VadProvider.IsVADExt(pcmData, sampleRate, frameSize)
	}
	return false, nil
}

func (v *Vad) Reset() error {
	v.lock.Lock()
	defer v.lock.Unlock()
	if v.VadProvider != nil {
		vad.ReleaseVAD(v.VadProvider) //Giải phóng tài nguyên instance vad
		v.VadProvider = nil           //Đặt về nil
	}
	v.ResetIdleDuration()
	v.ResetVoiceDuration()
	return nil
}