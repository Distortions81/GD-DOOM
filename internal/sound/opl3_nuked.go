//go:build cgo

package sound

/*
#cgo CFLAGS: -I${SRCDIR}/third_party/nuked_opl3
#include <stdlib.h>
#include "opl3.h"
#include "opl3.c"
*/
import "C"
import (
	"runtime"
	"unsafe"
)

// NukedOPL3 wraps the Nuked OPL3 core.
type NukedOPL3 struct {
	chip       *C.opl3_chip
	sampleRate int
}

// NewNukedOPL3 creates a Nuked OPL3 instance at the given output sample rate.
func NewNukedOPL3(sampleRate int) *NukedOPL3 {
	if sampleRate <= 0 {
		sampleRate = opl3DefaultSampleRate
	}
	chip := (*C.opl3_chip)(C.malloc(C.size_t(C.sizeof_opl3_chip)))
	o := &NukedOPL3{
		chip:       chip,
		sampleRate: sampleRate,
	}
	runtime.SetFinalizer(o, (*NukedOPL3).free)
	if o.chip != nil {
		C.OPL3_Reset(o.chip, C.uint32_t(sampleRate))
	}
	return o
}

// Reset resets all OPL state and registers.
func (o *NukedOPL3) Reset() {
	if o == nil || o.chip == nil {
		return
	}
	C.OPL3_Reset(o.chip, C.uint32_t(o.sampleRate))
}

// WriteReg writes a register in the OPL address space.
func (o *NukedOPL3) WriteReg(addr uint16, value uint8) {
	if o == nil || o.chip == nil {
		return
	}
	// Match Chocolate Doom's Nuked path: queue register writes through
	// the buffered entry point to preserve chip write timing behavior.
	C.OPL3_WriteRegBuffered(o.chip, C.uint16_t(addr), C.uint8_t(value))
}

// GenerateStereoS16 produces interleaved stereo signed-16 PCM.
func (o *NukedOPL3) GenerateStereoS16(frames int) []int16 {
	if o == nil || o.chip == nil || frames <= 0 {
		return nil
	}
	out := make([]int16, frames*2)
	C.OPL3_GenerateStream(
		o.chip,
		(*C.int16_t)(unsafe.Pointer(unsafe.SliceData(out))),
		C.uint32_t(frames),
	)
	return out
}

// GenerateMonoU8 produces unsigned 8-bit mono PCM from mixed stereo output.
func (o *NukedOPL3) GenerateMonoU8(frames int) []byte {
	st := o.GenerateStereoS16(frames)
	if len(st) == 0 {
		return nil
	}
	out := make([]byte, frames)
	for i := 0; i < frames; i++ {
		l := int(st[i*2])
		r := int(st[i*2+1])
		m := (l + r) / 2
		u := (m >> 8) + 128
		if u < 0 {
			u = 0
		} else if u > 255 {
			u = 255
		}
		out[i] = byte(u)
	}
	return out
}

func (o *NukedOPL3) free() {
	if o == nil || o.chip == nil {
		return
	}
	C.free(unsafe.Pointer(o.chip))
	o.chip = nil
}
