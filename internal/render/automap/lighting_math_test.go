package automap

import (
	"sync"
	"testing"
)

func TestSectorLightMulCoversFullRange(t *testing.T) {
	sectorLightLUTOnce = sync.Once{}
	if got := sectorLightMul(0); got != 0 {
		t.Fatalf("sectorLightMul(0)=%d want=0", got)
	}
	if got := sectorLightMul(255); got != 255 {
		t.Fatalf("sectorLightMul(255)=%d want=255", got)
	}
	if got := sectorLightMul(-10); got != 0 {
		t.Fatalf("sectorLightMul(-10)=%d want=0", got)
	}
	if got := sectorLightMul(300); got != 255 {
		t.Fatalf("sectorLightMul(300)=%d want=255", got)
	}
}

func TestCombineShadeMulIsMultiplicative(t *testing.T) {
	if got := combineShadeMul(256, 256); got != 256 {
		t.Fatalf("combine(256,256)=%d want=256", got)
	}
	if got := combineShadeMul(128, 128); got != 64 {
		t.Fatalf("combine(128,128)=%d want=64", got)
	}
	if got := combineShadeMul(0, 200); got != 0 {
		t.Fatalf("combine(0,200)=%d want=0", got)
	}
}
