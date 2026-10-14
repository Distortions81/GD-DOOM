package doomruntime

import (
	"encoding/csv"
	"os"
	"strconv"
	"testing"
)

func TestClassicCameraOracle(t *testing.T) {
	path := os.Getenv("GD_CLASSIC_CAMERA")
	if path == "" {
		path = "testdata/classic_camera.csv"
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 577 {
		t.Fatalf("camera oracle rows=%d, want 577", len(rows))
	}
	for i, r := range rows[1:] {
		var v [12]int64
		for j, s := range r {
			v[j], err = strconv.ParseInt(s, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
		}
		g := &game{isDead: v[0] != 0, worldTic: int(v[1]), p: player{z: v[2], floorz: v[3], ceilz: v[4], momx: v[5], momy: v[6], viewHeight: v[7], deltaViewHeight: v[8]}}
		g.tickPlayerViewHeight()
		if g.playerViewZ != v[9] || g.p.viewHeight != v[10] || g.p.deltaViewHeight != v[11] {
			t.Fatalf("case %d: got view,height,delta=%d,%d,%d; classic=%d,%d,%d", i, g.playerViewZ, g.p.viewHeight, g.p.deltaViewHeight, v[9], v[10], v[11])
		}
		if g.playerBaseEyeZ() != float64(v[9])/fracUnit {
			t.Fatalf("case %d: rendered eye=%g, classic=%g", i, g.playerBaseEyeZ(), float64(v[9])/fracUnit)
		}
	}
}
