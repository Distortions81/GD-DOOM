package audiofx

import (
	"math"
	"testing"
)

func TestPCMMonoU8ToMonoS16IntoReusesBuffer(t *testing.T) {
	dst := make([]int16, 0, 8)
	allocs := testing.AllocsPerRun(1000, func() {
		out := PCMMonoU8ToMonoS16Into(dst[:0], []byte{0, 128, 255})
		if len(out) != 3 {
			t.Fatalf("len=%d want=3", len(out))
		}
		if out[0] != -32768 || out[1] != 0 || out[2] != 32512 {
			t.Fatalf("got=%v", out)
		}
	})
	if allocs != 0 {
		t.Fatalf("allocs=%v want 0", allocs)
	}
}

func TestResampleMonoS16LinearIntoReusesBuffer(t *testing.T) {
	src := []int16{-32768, 0, 32512}
	dst := make([]int16, 0, 16)
	allocs := testing.AllocsPerRun(1000, func() {
		out := resampleMonoS16LinearInto(dst[:0], src, 11025, 44100)
		if len(out) != 12 {
			t.Fatalf("len=%d want=12", len(out))
		}
		if !(out[0] < out[4] && out[4] < out[len(out)-1]) {
			t.Fatalf("expected rising resample, got first=%d mid=%d last=%d", out[0], out[4], out[len(out)-1])
		}
	})
	if allocs != 0 {
		t.Fatalf("allocs=%v want 0", allocs)
	}
}

func TestResampleMonoS16PolyphaseIntoReusesBuffer(t *testing.T) {
	src := []int16{-32768, 0, 32512}
	dst := make([]int16, 0, 16)
	want := resampleMonoS16Polyphase(src, 11025, 44100)
	allocs := testing.AllocsPerRun(1000, func() {
		out := resampleMonoS16PolyphaseInto(dst[:0], src, 11025, 44100)
		if len(out) != len(want) {
			t.Fatalf("len=%d want=%d", len(out), len(want))
		}
		for i := range out {
			if out[i] != want[i] {
				t.Fatalf("out[%d]=%d want=%d", i, out[i], want[i])
			}
		}
	})
	if allocs != 0 {
		t.Fatalf("allocs=%v want 0", allocs)
	}
}

func TestSourcePortSoundDelaySamples(t *testing.T) {
	left, right := sourcePortSoundDelaySamples(11025, SpatialOrigin{
		X:          1000 * fracUnit,
		Y:          0,
		Positioned: true,
	}, 0, 0, 0)
	want := 1003.281
	if math.Abs(left-want) > 0.01 || math.Abs(right-want) > 0.01 {
		t.Fatalf("delaySamples=(%.3f,%.3f) want=(%.3f,%.3f)", left, right, want, want)
	}
}

func TestSourcePortSoundDelaySamples_UsesEarOffsets(t *testing.T) {
	left, right := sourcePortSoundDelaySamples(11025, SpatialOrigin{
		X:          0,
		Y:          100 * fracUnit,
		Positioned: true,
	}, 0, 0, 0)
	if left >= right {
		t.Fatalf("expected left ear closer than right for left-side sound, got left=%.3f right=%.3f", left, right)
	}
}

func TestSourcePortSoundDelaySamples_UnpositionedOrZeroRate(t *testing.T) {
	if left, right := sourcePortSoundDelaySamples(11025, SpatialOrigin{}, 0, 0, 0); left != 0 || right != 0 {
		t.Fatalf("unpositioned delay=(%.3f,%.3f) want=(0,0)", left, right)
	}
	if left, right := sourcePortSoundDelaySamples(0, SpatialOrigin{Positioned: true}, 0, 0, 0); left != 0 || right != 0 {
		t.Fatalf("zero-rate delay=(%.3f,%.3f) want=(0,0)", left, right)
	}
}

func TestPCMMonoS16ToStereoS16LESpatialDelayedInto(t *testing.T) {
	out := PCMMonoS16ToStereoS16LESpatialDelayedInto(nil, []int16{1000, 2000}, 1, 1, 1, 0)
	want := []byte{
		0x00, 0x00, 0xE8, 0x03,
		0xE8, 0x03, 0xD0, 0x07,
		0xD0, 0x07, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	if len(out) != len(want) {
		t.Fatalf("len=%d want=%d", len(out), len(want))
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("out[%d]=%02x want=%02x", i, out[i], want[i])
		}
	}
}

func TestPCMMonoS16ToStereoS16LESpatialDelayedInto_FractionalDelaySplitsSamples(t *testing.T) {
	out := PCMMonoS16ToStereoS16LESpatialDelayedInto(nil, []int16{1000}, 1, 0, 0.5, 0)
	want := []byte{
		0xF4, 0x01, 0x00, 0x00,
		0xF4, 0x01, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	if len(out) != len(want) {
		t.Fatalf("len=%d want=%d", len(out), len(want))
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("out[%d]=%02x want=%02x", i, out[i], want[i])
		}
	}
}

func TestSourcePortStereoMix_NormalizesCombinedGain(t *testing.T) {
	p := &SpatialPlayer{volume: 1, sourcePort: true}
	left, right, _, _, ok := p.eventStereoMix(SpatialOrigin{
		X:          1000 * fracUnit,
		Y:          0,
		Positioned: true,
	}, 0, 0, 0, false)
	if !ok {
		t.Fatal("expected valid stereo mix")
	}
	rms := math.Sqrt((left*left + right*right) * 0.5)
	want := sourcePortDistanceGain(1000)
	if math.Abs(rms-want) > 0.01 {
		t.Fatalf("rms gain=%.4f want %.4f", rms, want)
	}
}

func TestSourcePortStereoMix_RearSourcesAreQuieter(t *testing.T) {
	p := &SpatialPlayer{volume: 1, sourcePort: true}
	frontL, frontR, _, _, ok := p.eventStereoMix(SpatialOrigin{
		X:          1000 * fracUnit,
		Y:          0,
		Positioned: true,
	}, 0, 0, 0, false)
	if !ok {
		t.Fatal("expected valid front stereo mix")
	}
	backL, backR, _, _, ok := p.eventStereoMix(SpatialOrigin{
		X:          -1000 * fracUnit,
		Y:          0,
		Positioned: true,
	}, 0, 0, 0, false)
	if !ok {
		t.Fatal("expected valid rear stereo mix")
	}
	frontRMS := math.Sqrt((frontL*frontL + frontR*frontR) * 0.5)
	backRMS := math.Sqrt((backL*backL + backR*backR) * 0.5)
	if backRMS >= frontRMS {
		t.Fatalf("rear rms=%.4f want less than front rms=%.4f", backRMS, frontRMS)
	}
	want := frontRMS * sourcePortRearGainMin
	if math.Abs(backRMS-want) > 0.01 {
		t.Fatalf("rear rms=%.4f want %.4f", backRMS, want)
	}
}

func TestApplySourcePortLowPassInto_NoStrengthCopiesSignal(t *testing.T) {
	src := []int16{1000, -1000, 2000, -2000}
	got := applySourcePortLowPassInto(nil, src, 44100, 0, 0)
	if len(got) != len(src) {
		t.Fatalf("len=%d want=%d", len(got), len(src))
	}
	for i := range src {
		if got[i] != src[i] {
			t.Fatalf("got[%d]=%d want=%d", i, got[i], src[i])
		}
	}
}

func TestApplySourcePortLowPassInto_StrongFilterReducesHighFrequencySwing(t *testing.T) {
	src := []int16{2000, -2000, 2000, -2000, 2000, -2000}
	got := applySourcePortLowPassInto(nil, src, 44100, 1, 0)
	if len(got) != len(src) {
		t.Fatalf("len=%d want=%d", len(got), len(src))
	}
	if absInt(int(got[1])) >= absInt(int(src[1])) {
		t.Fatalf("filtered sample=%d want reduced swing from %d", got[1], src[1])
	}
	if absInt(int(got[len(got)-1])) >= absInt(int(src[len(src)-1])) {
		t.Fatalf("tail sample=%d want reduced swing from %d", got[len(got)-1], src[len(src)-1])
	}
}

func TestSourcePortFilterParams_BehindAndFarIncreaseFilterDrive(t *testing.T) {
	farFront, behindFront := sourcePortFilterParams(SpatialOrigin{
		X:          1000 * fracUnit,
		Y:          0,
		Positioned: true,
	}, 0, 0, 0)
	farBack, behindBack := sourcePortFilterParams(SpatialOrigin{
		X:          -1000 * fracUnit,
		Y:          0,
		Positioned: true,
	}, 0, 0, 0)
	if farFront <= 0 {
		t.Fatalf("farFront=%f want > 0", farFront)
	}
	if behindFront != 0 {
		t.Fatalf("behindFront=%f want 0", behindFront)
	}
	if behindBack <= behindFront {
		t.Fatalf("behindBack=%f want greater than %f", behindBack, behindFront)
	}
	if math.Abs(farBack-farFront) > 0.001 {
		t.Fatalf("farBack=%f want same as farFront=%f at equal distance", farBack, farFront)
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
