package doomtex

import (
	"gddoom/internal/wad"
	"path/filepath"
	"testing"
)

func TestLoadBoomColormaps(t *testing.T) {
	w, err := wad.Open(filepath.Join("..", "..", "..", "testdata", "boom", "water-actors.wad"))
	if err != nil {
		t.Fatal(err)
	}
	maps := LoadBoomColormaps(w)
	if len(maps) != 3 || len(maps["BWLOW"]) != 34*256 || maps["BWLOW"][0] == maps["BWMID"][0] {
		t.Fatalf("unexpected maps: %v", len(maps))
	}
	// Same named entries outside the namespace must not replace valid tables.
	w.Lumps = append(w.Lumps, wad.Lump{Name: "BWLOW", Size: 0})
	if len(LoadBoomColormaps(w)) != 3 {
		t.Fatal("global namespace shadowed custom colormap")
	}
	// An incomplete table inside the namespace cannot safely supply row 32.
	w.Lumps = append(w.Lumps, wad.Lump{Name: "C_START"}, wad.Lump{Name: "BWLOW", Size: 256}, wad.Lump{Name: "C_END"})
	if _, ok := LoadBoomColormaps(w)["BWLOW"]; ok {
		t.Fatal("truncated custom light table accepted")
	}
}
