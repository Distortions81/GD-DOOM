package audiofx

import (
	"io"
	"testing"

	"gddoom/internal/sound"
)

func TestPCSpeakerVariantsProducePCM(t *testing.T) {
	t.Parallel()

	seq := make([]sound.PCSpeakerTone, 140)
	for i := range seq {
		seq[i] = sound.PCSpeakerTone{Active: true, ToneValue: 96}
	}

	check := func(t *testing.T, variant PCSpeakerVariant, minPeak int) {
		t.Helper()
		src := &pcSpeakerSource{variant: variant, model: modelForVariant(variant), reverb: newCaseReverb()}
		src.load(seq, 44100)

		buf := make([]byte, 4096)
		maxAbs := 0
		for {
			n, err := src.Read(buf)
			for i := 0; i+3 < n; i += 4 {
				v := int(int16(buf[i]) | int16(buf[i+1])<<8)
				if v < 0 {
					v = -v
				}
				if v > maxAbs {
					maxAbs = v
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("read failed for %s: %v", variant.String(), err)
			}
		}
		if maxAbs < minPeak {
			t.Fatalf("%s peak too low: got %d want >= %d", variant.String(), maxAbs, minPeak)
		}
	}

	check(t, PCSpeakerVariantClean, 1000)
	check(t, PCSpeakerVariantSmallSpeaker, 1000)
	check(t, PCSpeakerVariantPiezo, 1000)
}
