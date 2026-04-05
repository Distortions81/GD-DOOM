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
	out := make([]int16, voicecodec.CaptureFrameSamples)
	for i := range out {
		v := math.Sin(2 * math.Pi * freq * float64(i) / float64(voicecodec.CaptureSampleRate))
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
		_ = agc.ProcessFrame(buf, voicecodec.CaptureSampleRate)
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
		_ = agc.ProcessFrame(buf, voicecodec.CaptureSampleRate)
		frame = buf
	}
	after := rmsInt16(frame)
	if after <= before*2.0 {
		t.Fatalf("quiet voice rms after=%.1f want > %.1f", after, before*2.0)
	}
}

func TestMicAGCDoesNotPumpLowLevelNoiseUpAggressively(t *testing.T) {
	agc := newMicAGC()
	frame := make([]int16, voicecodec.CaptureFrameSamples)
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
		_ = agc.ProcessFrame(buf, voicecodec.CaptureSampleRate)
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
		_ = agc.ProcessFrame(buf, voicecodec.CaptureSampleRate)
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
		_ = agc.ProcessFrame(buf, voicecodec.CaptureSampleRate)
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
	noise := make([]int16, voicecodec.CaptureFrameSamples)
	for i := range noise {
		if i%2 == 0 {
			noise[i] = 40
		} else {
			noise[i] = -40
		}
	}
	for range 80 {
		buf := append([]int16(nil), noise...)
		_ = agc.ProcessFrame(buf, voicecodec.CaptureSampleRate)
	}
	test := append([]int16(nil), noise...)
	before := rmsInt16(test)
	_ = agc.ProcessFrame(test, voicecodec.CaptureSampleRate)
	after := rmsInt16(test)
	if after >= before {
		t.Fatalf("soft knee noise rms after=%.1f want < %.1f", after, before)
	}
}

func TestMicAGCLowLevelNoiseEventuallyBecomesSilence(t *testing.T) {
	agc := newMicAGC()
	noise := make([]int16, voicecodec.CaptureFrameSamples)
	for i := range noise {
		if i%2 == 0 {
			noise[i] = 40
		} else {
			noise[i] = -40
		}
	}
	silence := false
	for range 80 {
		buf := append([]int16(nil), noise...)
		silence = agc.ProcessFrame(buf, voicecodec.CaptureSampleRate)
		if silence {
			for i, sample := range buf {
				if sample != 0 {
					t.Fatalf("buf[%d]=%d want 0 after silence gate", i, sample)
				}
			}
			break
		}
	}
	if !silence {
		t.Fatal("expected low-level noise to eventually be treated as silence")
	}
}

func TestMicAGCMarksFullyGatedFrameAsSilence(t *testing.T) {
	agc := newMicAGC()
	frame := make([]int16, voicecodec.CaptureFrameSamples)
	silence := agc.ProcessFrame(frame, voicecodec.CaptureSampleRate)
	if !silence {
		t.Fatal("ProcessFrame() silence=false want true for fully gated frame")
	}
	for i, sample := range frame {
		if sample != 0 {
			t.Fatalf("frame[%d]=%d want 0", i, sample)
		}
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

func TestMicAGCGateReleaseIsSmoothedAcrossFrames(t *testing.T) {
	agc := newMicAGC()
	agc.gateGain = 1
	rms := 80.0
	noiseAvg := 100.0
	target := gateGainForFrame(false, 0, rms, noiseAvg)
	if target != 0 {
		t.Fatalf("gate target=%0.3f want 0", target)
	}
	agc.gateGain += (target - agc.gateGain) * agcGateRelease
	if agc.gateGain <= 0 || agc.gateGain >= 1 {
		t.Fatalf("smoothed gate=%0.3f want between 0 and 1", agc.gateGain)
	}
}

func TestMicAGCGateAttackIsSmoothedAcrossFrames(t *testing.T) {
	agc := newMicAGC()
	agc.gateGain = 0
	target := gateGainForFrame(true, 0, 300, 100)
	if target != 1 {
		t.Fatalf("gate target=%0.3f want 1", target)
	}
	agc.gateGain += (target - agc.gateGain) * agcGateAttack
	if agc.gateGain <= 0 || agc.gateGain >= 1 {
		t.Fatalf("smoothed gate=%0.3f want between 0 and 1", agc.gateGain)
	}
}

func TestDesiredGateByGroupUsesLookaheadBeforeOnset(t *testing.T) {
	startGate := 0.0
	targetGate := 1.0
	groupRMS := []float64{20, 30, 250, 260, 240}
	got := desiredGateByGroup(startGate, targetGate, true, groupRMS, 100)
	if len(got) != len(groupRMS) {
		t.Fatalf("len(got)=%d want %d", len(got), len(groupRMS))
	}
	if got[0] <= startGate {
		t.Fatalf("got[0]=%0.3f want > %0.3f", got[0], startGate)
	}
	if got[1] <= got[0] {
		t.Fatalf("got[1]=%0.3f want > got[0]=%0.3f", got[1], got[0])
	}
	if got[2] >= targetGate {
		t.Fatalf("got[2]=%0.3f want < %0.3f for smoothed onset", got[2], targetGate)
	}
	if got[3] != targetGate || got[4] != targetGate {
		t.Fatalf("tail gates=%v want fully open after onset", got[3:])
	}
}
