package sessionvoice

import (
	"math"
	"testing"

	"gddoom/internal/voicecodec"
)

func rmsInt16(pcm []int16) float64 {
	if len(pcm) == 0 {
		return 0
	}
	var sum float64
	for _, s := range pcm {
		v := float64(s)
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(pcm)))
}

func sineFrame(freq float64, amp int16) []int16 {
	out := make([]int16, voicecodec.FrameSamples)
	for i := range out {
		v := math.Sin(2 * math.Pi * freq * float64(i) / float64(voicecodec.SampleRate))
		out[i] = int16(v * float64(amp))
	}
	return out
}

func TestMicAGCBoostsVoiceLikeFrame(t *testing.T) {
	agc := newMicAGC()
	frame := sineFrame(220, 1200)
	before := rmsInt16(frame)
	for range 20 {
		buf := append([]int16(nil), frame...)
		agc.ProcessFrame(buf, voicecodec.SampleRate)
		frame = buf
	}
	after := rmsInt16(frame)
	if after <= before*1.5 {
		t.Fatalf("voice rms after=%.1f want > %.1f", after, before*1.5)
	}
}

func TestMicAGCDoesNotPumpLowLevelNoiseUpAggressively(t *testing.T) {
	agc := newMicAGC()
	frame := make([]int16, voicecodec.FrameSamples)
	for i := range frame {
		if i%2 == 0 {
			frame[i] = 18
		} else {
			frame[i] = -18
		}
	}
	base := rmsInt16(frame)
	for range 60 {
		buf := append([]int16(nil), frame...)
		agc.ProcessFrame(buf, voicecodec.SampleRate)
		frame = buf
	}
	after := rmsInt16(frame)
	if after > base*2.0 {
		t.Fatalf("noise rms after=%.1f want <= %.1f", after, base*2.0)
	}
}

func TestMicAGCBoundsPeakLevelForHotInput(t *testing.T) {
	agc := newMicAGC()
	frame := sineFrame(220, 30000)
	for range 8 {
		buf := append([]int16(nil), frame...)
		agc.ProcessFrame(buf, voicecodec.SampleRate)
		frame = buf
	}
	var peak int16
	for _, s := range frame {
		if abs := int16(math.Abs(float64(s))); abs > peak {
			peak = abs
		}
	}
	if peak > int16(agcPeakLimit)+512 {
		t.Fatalf("peak after=%d want <= %d", peak, int16(agcPeakLimit)+512)
	}
}
