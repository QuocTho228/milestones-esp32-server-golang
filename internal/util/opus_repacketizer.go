package util

/*
#cgo pkg-config: opus
#include <opus.h>
*/
import "C"

import (
	"fmt"
	"unsafe"

	"gopkg.in/hraban/opus.v2"
)

type opusRepacketizer struct {
	ptr *C.OpusRepacketizer
}

func newOpusRepacketizer() (*opusRepacketizer, error) {
	ptr := C.opus_repacketizer_create()
	if ptr == nil {
		return nil, fmt.Errorf("tạo Opus repacketizer thất bại")
	}
	return &opusRepacketizer{ptr: ptr}, nil
}

func (r *opusRepacketizer) close() {
	if r == nil || r.ptr == nil {
		return
	}
	C.opus_repacketizer_destroy(r.ptr)
	r.ptr = nil
}

func (r *opusRepacketizer) reset() {
	if r == nil || r.ptr == nil {
		return
	}
	C.opus_repacketizer_init(r.ptr)
}

func (r *opusRepacketizer) nbFrames() int {
	if r == nil || r.ptr == nil {
		return 0
	}
	return int(C.opus_repacketizer_get_nb_frames(r.ptr))
}

func (r *opusRepacketizer) cat(packet []byte) error {
	if r == nil || r.ptr == nil {
		return fmt.Errorf("Opus repacketizer chưa được khởi tạo")
	}
	if len(packet) == 0 {
		return fmt.Errorf("packet Opus không được để trống")
	}
	code := C.opus_repacketizer_cat(
		r.ptr,
		(*C.uchar)(unsafe.Pointer(&packet[0])),
		C.opus_int32(len(packet)),
	)
	if code != C.OPUS_OK {
		return opus.Error(code)
	}
	return nil
}

func (r *opusRepacketizer) out() ([]byte, error) {
	return r.outRange(0, r.nbFrames())
}

func (r *opusRepacketizer) outRange(begin int, end int) ([]byte, error) {
	if r == nil || r.ptr == nil {
		return nil, fmt.Errorf("Opus repacketizer chưa được khởi tạo")
	}
	if begin < 0 || end < begin {
		return nil, fmt.Errorf("range repacketizer không hợp lệ: begin=%d end=%d", begin, end)
	}

	maxFrames := end - begin
	if maxFrames <= 0 {
		return nil, fmt.Errorf("range repacketizer không được để trống")
	}

	// 1277 là độ dài tối đa lý thuyết của một khung đơn, với giới hạn 120ms thì buffer ở đây đủ để bao phủ đầu ra.
	buf := make([]byte, 1277*maxFrames)
	outLen := C.opus_repacketizer_out_range(
		r.ptr,
		C.int(begin),
		C.int(end),
		(*C.uchar)(unsafe.Pointer(&buf[0])),
		C.opus_int32(len(buf)),
	)
	if outLen < 0 {
		return nil, opus.Error(outLen)
	}
	packet := make([]byte, int(outLen))
	copy(packet, buf[:int(outLen)])
	return packet, nil
}