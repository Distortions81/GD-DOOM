package doomruntime

import (
	"encoding/csv"
	"math"
	"os"
	"strconv"
	"testing"
)

// The fixture is emitted by sampling_probe.inc calling the original Linux Doom
// R_MapPlane/R_ScaleFromGlobalAngle and reading its xtoviewangle table. The
// visual harness regenerates and checks it on every run, independent of Go math.
func TestClassicSamplingOracle(t *testing.T) {
	path := os.Getenv("GD_CLASSIC_SAMPLING")
	if path == "" {
		path = "testdata/classic_sampling.csv"
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
	if len(rows) != 385 {
		t.Fatalf("oracle has %d rows, want 385", len(rows))
	}
	width, height := classicOracleDimensions(t)
	focal, cy := float64(width)/2, float64(height)/2
	g := &game{viewW: width, viewH: height}
	for _, r := range rows[1:] {
		v := make([]int64, 7)
		for i := range v {
			v[i], err = strconv.ParseInt(r[i+1], 10, 64)
			if err != nil {
				t.Fatal(err)
			}
		}
		angle, x, y := uint32(v[0]), int(v[1]), int(v[2])
		switch r[0] {
		case "sky":
			cols, _ := g.buildSkyLookupParallel(width, height, focal, angleToRadians(angle), 256, 128)
			if got := cols[x]; got != int(v[3])&255 {
				t.Fatalf("sky %v got %d", r, got)
			}
		case "scale":
			s := classicWallSampler{projection: classicProjectionForWidth(width), view: angle, normal: angle, distance: 64 * fracUnit, focal: focal}
			if got := s.scaleAt(x); got != v[3] {
				t.Fatalf("scale %v got %d", r, got)
			}
		case "plane":
			planeHeight := 0.0
			if y < height/2 {
				planeHeight = 128
			}
			a := angleToRadians(angle)
			s, ok := g.planeRowRenderState(y, plane3DKey{height: planeHeight}, 41, 13, -27, math.Cos(a), math.Sin(a), focal, focal, focal, cy)
			if !ok {
				t.Fatalf("plane missing %v", r)
			}
			u, vv := s.textureStart(x)
			if u != v[3] || vv != v[4] || s.stepWXFixed != v[5] || s.stepWYFixed != v[6] {
				t.Errorf("plane %v got %d,%d,%d,%d", r, u, vv, s.stepWXFixed, s.stepWYFixed)
			}
		default:
			t.Fatalf("unknown oracle row %v", r)
		}
	}
}

func TestClassicColumnIntegerRowAndReciprocalScale(t *testing.T) {
	g := &game{viewH: 200}
	// In classic, even scale 1 has iscale 0xffffffff/65536 = 65535.
	for _, tc := range []struct {
		y           int
		mid, depth  float64
		start, step int64
	}{
		{100, 41, 160, 41 * 65536, 65535},
		{99, -7, 160, -7*65536 - 65535, 65535},
		{101, 13, 64, 13*65536 + 26214, 26214},
	} {
		a, b := g.wallColumnTextureStep(tc.y, tc.mid, tc.depth, 160)
		if a != tc.start || b != tc.step {
			t.Fatalf("%+v got %d,%d", tc, a, b)
		}
	}
}

// The harness supplies dimensions for its freshly generated C oracle; the
// committed vanilla fixture remains 320x200.
func classicOracleDimensions(t *testing.T) (int, int) {
	t.Helper()
	dimensions := []int{320, 200}
	for i, key := range []string{"GD_VISUAL_WIDTH", "GD_VISUAL_HEIGHT"} {
		if value := os.Getenv(key); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 64 || n%2 != 0 {
				t.Fatalf("invalid %s: %q", key, value)
			}
			dimensions[i] = n
		}
	}
	return dimensions[0], dimensions[1]
}
