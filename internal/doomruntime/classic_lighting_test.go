package doomruntime

import (
	"encoding/csv"
	"os"
	"strconv"
	"testing"
)

func TestClassicLightingOracle(t *testing.T) {
	resetLightingMathState(t)
	prev := doomColormapRows
	doomColormapRows = 32
	t.Cleanup(func() { doomColormapRows = prev })
	path := os.Getenv("GD_CLASSIC_LIGHTING")
	if path == "" {
		path = "testdata/classic_lighting.csv"
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
	if len(rows) != 2817 {
		t.Fatalf("lighting oracle rows=%d, want 2817", len(rows))
	}
	width, _ := classicOracleDimensions(t)
	focal := float64(width) / 2
	for _, r := range rows[1:] {
		v := make([]int, 3)
		for i := range v {
			v[i], err = strconv.Atoi(r[i+1])
			if err != nil {
				t.Fatal(err)
			}
		}
		// Exercise every source table entry and interior values in its bucket.
		for _, fraction := range []float64{0, .5, .999} {
			light := int16(v[0] + 7) // sector light must truncate before lookup
			got := 0
			switch r[0] {
			case "wall":
				scale := float64(v[1])*4096 + fraction*4096
				if scale == 0 {
					scale = 1
				}
				got = doomWallLightRow(light, 0, focal*fracUnit/scale, focal)
			case "plane":
				got = doomPlaneLightRow(light, float64(v[1])*16+fraction*16)
			default:
				t.Fatalf("unknown row %v", r)
			}
			if got != v[2] {
				t.Fatalf("%v fraction=%g got=%d", r, fraction, got)
			}
		}
	}
}
