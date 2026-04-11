package sound

import (
	"bytes"
	"strings"
	"testing"
)

func TestPCSpeakerCaptureRoundTrip(t *testing.T) {
	in := PCSpeakerCapture{
		TickRate: 140,
		Tones: []PCSpeakerTone{
			{Active: false},
			{Active: true, ToneValue: 23},
			{Active: true, Divisor: 912},
		},
	}
	var buf bytes.Buffer
	if err := WritePCSpeakerCapture(&buf, in); err != nil {
		t.Fatalf("WritePCSpeakerCapture() error: %v", err)
	}
	got, err := ReadPCSpeakerCapture(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadPCSpeakerCapture() error: %v", err)
	}
	if got.TickRate != in.TickRate {
		t.Fatalf("TickRate=%d want=%d", got.TickRate, in.TickRate)
	}
	if len(got.Tones) != len(in.Tones) {
		t.Fatalf("len(Tones)=%d want=%d", len(got.Tones), len(in.Tones))
	}
	for i := range in.Tones {
		if got.Tones[i] != in.Tones[i] {
			t.Fatalf("tone[%d]=%+v want=%+v", i, got.Tones[i], in.Tones[i])
		}
	}
}

func TestPCSpeakerCaptureRejectsBadMagic(t *testing.T) {
	_, err := ReadPCSpeakerCapture(strings.NewReader("not-a-capture"))
	if err == nil || !strings.Contains(err.Error(), "invalid pc speaker capture header") {
		t.Fatalf("ReadPCSpeakerCapture() error=%v want invalid header", err)
	}
}
