package doomtex

import (
	"gddoom/internal/wad"
	"path/filepath"
	"testing"
)

func TestBoomTranMapResourceSelection(t *testing.T) {
	w, err := wad.Open(filepath.Join("..", "..", "..", "testdata", "boom", "translucency-override.wad"))
	if err != nil {
		t.Fatal(err)
	}
	maps := LoadBoomTranMaps(w)
	if len(maps["TRANMAP"]) != 65536 || len(maps["CUSTMAP"]) != 65536 || maps["CUSTMAP"][40*256+200] != 140 || maps["TRANMAP"][40*256+200] != 100 {
		t.Fatal("global/named TRANMAP selection incorrect")
	}
}
func TestGeneratedBoomTranMapWeightAndTieBreak(t *testing.T) {
	p := make([]byte, 1024)
	for i := 0; i < 256; i++ {
		p[i*4], p[i*4+1], p[i*4+2], p[i*4+3] = byte(i), byte(i), byte(i), 255
	}
	table := GenerateBoomTranMap(p)
	if len(table) != 65536 || table[40*256+200] != 146 || table[200*256+40] != 94 {
		t.Fatal("default 66% source weighting incorrect")
	}
	if &GenerateBoomTranMap(p)[0] != &table[0] {
		t.Fatal("same palette regenerated its table")
	}
	clear(p)
	if got := GenerateBoomTranMap(p)[0]; got != 255 {
		t.Fatalf("equal-color tie selected %d instead of 255", got)
	}
	if GenerateBoomTranMap(nil) != nil {
		t.Fatal("generated table without a palette")
	}
}
