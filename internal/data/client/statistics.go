package client

import "time"

// Statistic Struct này đã lỗi thời (deprecated), hãy dùng statistic_plugin để lấy thông tin thống kê tại thời điểm MetricTtsStop
type Statistic struct {
	TurnStartTs        int64
	VoiceSilenceTs     int64
	AsrFirstTextTs     int64
	AsrFinalTextTs     int64
	LlmStartTs         int64
	LlmFirstTokenTs    int64
	LlmFirstSentenceTs int64
	LlmEndTs           int64
	TtsStartTs         int64
	TtsFirstFrameTs    int64
	TtsStopTs          int64
}

// MarkTurnStart Ghi lại thời điểm bắt đầu lượt (turn)
func (state *ClientState) MarkTurnStart() {
	state.Statistic.TurnStartTs = time.Now().UnixMilli()
	state.Statistic.VoiceSilenceTs = 0
	state.Statistic.AsrFirstTextTs = 0
	state.Statistic.AsrFinalTextTs = 0
}

// MarkVoiceSilenceAt Ghi lại thời điểm bắt đầu im lặng giọng nói, trả về true nếu đây là lần ghi đầu tiên trong lượt này
func (state *ClientState) MarkVoiceSilenceAt(ts int64) bool {
	if state.Statistic.VoiceSilenceTs != 0 {
		return false
	}
	state.Statistic.VoiceSilenceTs = ts
	return true
}

// MarkVoiceSilence Ghi lại thời điểm bắt đầu im lặng giọng nói, trả về true nếu đây là lần ghi đầu tiên trong lượt này
func (state *ClientState) MarkVoiceSilence() bool {
	return state.MarkVoiceSilenceAt(time.Now().UnixMilli())
}

// MarkAsrFirstText Ghi lại thời điểm ASR trả về văn bản lần đầu
func (state *ClientState) MarkAsrFirstText() {
	if state.Statistic.AsrFirstTextTs == 0 {
		state.Statistic.AsrFirstTextTs = time.Now().UnixMilli()
	}
}

// MarkAsrFinalText Ghi lại thời điểm ASR trả về văn bản cuối cùng
func (state *ClientState) MarkAsrFinalText() {
	state.MarkAsrFinalTextAt(time.Now().UnixMilli())
}

// MarkAsrFinalTextAt Ghi lại thời điểm ASR trả về văn bản cuối cùng, trả về true nếu đây là lần ghi đầu tiên trong lượt này
func (state *ClientState) MarkAsrFinalTextAt(ts int64) bool {
	if state.Statistic.AsrFinalTextTs != 0 {
		return false
	}
	state.Statistic.AsrFinalTextTs = ts
	return true
}

// MarkLlmStart Ghi lại thời điểm LLM bắt đầu
func (state *ClientState) MarkLlmStart() {
	state.Statistic.LlmStartTs = time.Now().UnixMilli()
	state.Statistic.LlmFirstTokenTs = 0
	state.Statistic.LlmFirstSentenceTs = 0
	state.Statistic.LlmEndTs = 0
}

// MarkLlmFirstToken Ghi lại thời điểm LLM trả về token đầu tiên
func (state *ClientState) MarkLlmFirstToken() {
	state.Statistic.LlmFirstTokenTs = time.Now().UnixMilli()
}

// MarkLlmFirstSentenceAt Ghi lại thời điểm LLM xuất câu đầu tiên, trả về true nếu đây là lần ghi đầu tiên trong lượt này
func (state *ClientState) MarkLlmFirstSentenceAt(ts int64) bool {
	if state.Statistic.LlmFirstSentenceTs != 0 {
		return false
	}
	state.Statistic.LlmFirstSentenceTs = ts
	return true
}

// MarkLlmFirstSentence Ghi lại thời điểm LLM xuất câu đầu tiên, trả về true nếu đây là lần ghi đầu tiên trong lượt này
func (state *ClientState) MarkLlmFirstSentence() bool {
	return state.MarkLlmFirstSentenceAt(time.Now().UnixMilli())
}

// MarkLlmEnd Ghi lại thời điểm LLM kết thúc
func (state *ClientState) MarkLlmEnd() {
	state.Statistic.LlmEndTs = time.Now().UnixMilli()
}

// MarkTtsStart Ghi lại thời điểm TTS bắt đầu
func (state *ClientState) MarkTtsStart() {
	state.Statistic.TtsStartTs = time.Now().UnixMilli()
	state.Statistic.TtsFirstFrameTs = 0
	state.Statistic.TtsStopTs = 0
}

// MarkTtsFirstFrame Ghi lại thời điểm khung (frame) TTS đầu tiên
func (state *ClientState) MarkTtsFirstFrame() {
	if state.Statistic.TtsFirstFrameTs == 0 {
		state.Statistic.TtsFirstFrameTs = time.Now().UnixMilli()
	}
}

// MarkTtsStop Ghi lại thời điểm TTS kết thúc
func (state *ClientState) MarkTtsStop() {
	state.Statistic.TtsStopTs = time.Now().UnixMilli()
}

// SetStartAsrTs Thiết lập thời điểm bắt đầu ASR (bí danh, để tương thích ngược)
func (state *ClientState) SetStartAsrTs() { state.MarkVoiceSilence() }

// SetStartLlmTs Thiết lập thời điểm bắt đầu LLM (bí danh, để tương thích ngược)
func (state *ClientState) SetStartLlmTs() { state.MarkLlmStart() }

// SetStartTtsTs Thiết lập thời điểm bắt đầu TTS (bí danh, để tương thích ngược)
func (state *ClientState) SetStartTtsTs() { state.MarkTtsStart() }

// GetAsrDuration Lấy thời gian xử lý ASR (đã lỗi thời, chỉ giữ lại chữ ký phương thức)
func (state *ClientState) GetAsrDuration() int64 {
	return calcStatisticDuration(state.Statistic.VoiceSilenceTs, state.Statistic.AsrFinalTextTs)
}

// GetAsrLlmTtsDuration Lấy tổng thời gian xử lý (đã lỗi thời, chỉ giữ lại chữ ký phương thức)
func (state *ClientState) GetAsrLlmTtsDuration() int64 {
	return calcStatisticDuration(state.Statistic.VoiceSilenceTs, state.Statistic.TtsFirstFrameTs)
}

// GetLlmDuration Lấy thời gian xử lý LLM (đã lỗi thời, chỉ giữ lại chữ ký phương thức)
func (state *ClientState) GetLlmDuration() int64 {
	return calcStatisticDuration(state.Statistic.LlmStartTs, state.Statistic.LlmEndTs)
}

// GetTtsDuration Lấy thời gian xử lý TTS (đã lỗi thời, chỉ giữ lại chữ ký phương thức)
func (state *ClientState) GetTtsDuration() int64 {
	return calcStatisticDuration(state.Statistic.TtsStartTs, state.Statistic.TtsStopTs)
}

func calcStatisticDuration(start, end int64) int64 {
	if start <= 0 || end <= 0 || end < start {
		return 0
	}
	return end - start
}

func (s *Statistic) Reset() {
	if s == nil {
		return
	}
	*s = Statistic{}
}