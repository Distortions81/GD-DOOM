package scene

import "testing"

func TestEncodeDepthBiasQ_RoundsUp(t *testing.T) {
	if got := EncodeDepthBiasQ(0.01); got == 0 {
		t.Fatal("expected small positive bias to round up")
	}
}

func TestPackDepthStamped_RoundTrip(t *testing.T) {
	packed := PackDepthStamped(123, 456)
	if got := UnpackDepthQ(packed); got != 123 {
		t.Fatalf("depth=%d want 123", got)
	}
	if got := UnpackDepthStamp(packed); got != 456 {
		t.Fatalf("stamp=%d want 456", got)
	}
}

func TestSpriteOccludedDepthQAt(t *testing.T) {
	stamp := uint16(7)
	depthPix := []uint32{PackDepthStamped(100, stamp)}
	if !SpriteOccludedDepthQAt(depthPix, nil, stamp, 101, 0, 0) {
		t.Fatal("expected wall/sprite depth to occlude deeper sample")
	}
	planePix := []uint32{PackDepthStamped(100, stamp)}
	if !SpriteOccludedDepthQAt([]uint32{0}, planePix, stamp, 105, 4, 0) {
		t.Fatal("expected plane depth with bias to occlude deeper sample")
	}
	if SpriteOccludedDepthQAt([]uint32{0}, planePix, stamp, 104, 4, 0) {
		t.Fatal("expected sample at threshold to remain visible")
	}
}
