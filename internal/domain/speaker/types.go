package speaker

// IdentifyResult kết quả nhận diện vân giọng nói
type IdentifyResult struct {
	Identified  bool    `json:"identified"`
	SpeakerID   string  `json:"speaker_id"`
	SpeakerName string  `json:"speaker_name"`
	Confidence  float32 `json:"confidence"`
	Threshold   float32 `json:"threshold"`
}