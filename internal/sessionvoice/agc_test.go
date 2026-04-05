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

func TestMicAGCBoostsQuietVoiceLikeFrame(t *testing.T) {
	agc := newMicAGC()
	frame := sineFrame(220, 500)
	before := rmsInt16(frame)
	for range 30 {
		buf := append([]int16(nil), frame...)
		agc.ProcessFrame(buf, voicecodec.SampleRate)
		frame = buf
	}
	after := rmsInt16(frame)
	if after <= before*2.0 {
		t.Fatalf("quiet voice rms after=%.1f want > %.1f", after, before*2.0)
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

func TestMicAGCAllowsHigherGainForQuietSpeech(t *testing.T) {
	agc := newMicAGC()
	frame := sineFrame(220, 450)
	for range 40 {
		buf := append([]int16(nil), frame...)
		agc.ProcessFrame(buf, voicecodec.SampleRate)
	}
	if agc.gain <= 6.0 {
		t.Fatalf("gain after quiet speech=%.2f want > 6.0", agc.gain)
	}
	if agc.gain > agcMaxGain {
		t.Fatalf("gain after quiet speech=%.2f want <= %.2f", agc.gain, agcMaxGain)
	}
}

func TestMicAGCSoftNoiseKneeReducesNearFloorNoise(t *testing.T) {
	agc := newMicAGC()
	noise := make([]int16, voicecodec.FrameSamples)
	for i := range noise {
		if i%2 == 0 {
			noise[i] = 40
		} else {
			noise[i] = -40
		}
	}
	for range 80 {
		buf := append([]int16(nil), noise...)
		agc.ProcessFrame(buf, voicecodec.SampleRate)
	}
	test := append([]int16(nil), noise...)
	before := rmsInt16(test)
	agc.ProcessFrame(test, voicecodec.SampleRate)
	after := rmsInt16(test)
	if after >= before {
		t.Fatalf("soft knee noise rms after=%.1f want < %.1f", after, before)
	}
}

func TestGateGainForFrameStartsReducingDuringVoiceHold(t *testing.T) {
	noiseAvg := 100.0
	rms := 80.0
	knee := softGateGain(rms, noiseAvg)
	got := gateGainForFrame(false, agcVoiceHoldFrames/2, rms, noiseAvg)
	if got >= 1 {
		t.Fatalf("gateGainForFrame()=%0.3f want < 1", got)
	}
	if got <= knee {
		t.Fatalf("gateGainForFrame()=%0.3f want > knee %0.3f", got, knee)
	}
}
