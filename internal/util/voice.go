package util

import (
	"bytes"
	"encoding/binary"
	"math"
)

// PCM16BytesToFloat32 chuyển đổi luồng byte PCM 16-bit little-endian thành slice float32 (khoảng giá trị -1.0~1.0)
func PCM16BytesToFloat32(pcm []byte) []float32 {
	n := len(pcm) / 2
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		// Lấy hai byte, chuyển thành int16 theo little-endian
		sample := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		out[i] = float32(sample) / float32(math.MaxInt16)
	}
	return out
}

// Float32ToPCMBytes chuyển đổi mảng float32 thành mảng byte PCM 16-bit
func Float32ToPCMBytes(samples []float32, pcmBytes []byte) {
	for i, sample := range samples {
		// Chuyển float32 (-1.0 đến 1.0) thành int16 (-32768 đến 32767)
		intSample := float32ToInt16(sample)

		// Ghi vào mảng byte theo little-endian
		binary.LittleEndian.PutUint16(pcmBytes[i*2:], uint16(intSample))
	}

	return
}

// float32ToInt16 chuyển đổi giá trị float32 thành giá trị int16 (khoảng -1.0~1.0 chuyển thành -32768~32767)
func float32ToInt16(sample float32) int16 {
	if sample > 1.0 {
		return 32767
	} else if sample < -1.0 {
		return -32768
	} else {
		return int16(sample * 32767)
	}
}

// Float32SliceToInt16Slice chuyển đổi slice float32 thành slice int16
func Float32SliceToInt16Slice(samples []float32) []int16 {
	result := make([]int16, len(samples))
	for i, sample := range samples {
		result[i] = float32ToInt16(sample)
	}
	return result
}

// Int16SliceToBytes chuyển đổi slice int16 thành []byte (little-endian)
func Int16SliceToBytes(samples []int16) []byte {
	buf := new(bytes.Buffer)
	for _, s := range samples {
		buf.WriteByte(byte(s))
		buf.WriteByte(byte(s >> 8))
	}
	return buf.Bytes()
}

func ResampleLinearFloat32(input []float32, inRate, outRate int) []float32 {
	ratio := float64(outRate) / float64(inRate)
	outLen := int(float64(len(input)) * ratio)
	output := make([]float32, outLen)

	for i := 0; i < outLen; i++ {
		pos := float64(i) / ratio
		index := int(pos)
		if index >= len(input)-1 {
			output[i] = input[len(input)-1]
		} else {
			frac := float32(pos - float64(index))
			output[i] = input[index]*(1-frac) + input[index+1]*frac
		}
	}
	return output
}

// Float32SliceToBytes chuyển đổi mảng float32 thành mảng byte (little-endian, mỗi float32 chiếm 4 byte)
func Float32SliceToBytes(data []float32) []byte {
	if len(data) == 0 {
		return nil
	}
	bytes := make([]byte, len(data)*4)
	for i, v := range data {
		binary.LittleEndian.PutUint32(bytes[i*4:i*4+4], math.Float32bits(v))
	}
	return bytes
}