package doomruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/mapview"
	"gddoom/internal/wad"
)

// Independent point queries on the original directed sides, without ring
// reconstruction or scanline sorting. Both sides of internal lines cancel.
func automapBoundaryWinding(m *mapdata.Map, sector int, x, y float64) (int, int) {
	winding := 0
	total := 0
	for _, ld := range m.Linedefs {
		a, b := m.Vertexes[ld.V1], m.Vertexes[ld.V2]
		for side, sn := range ld.SideNum {
			if sn < 0 || int(m.Sidedefs[sn].Sector) != sector {
				continue
			}
			ax, ay, bx, by := float64(a.X), float64(a.Y), float64(b.X), float64(b.Y)
			if side == 1 {
				ax, ay, bx, by = bx, by, ax, ay
			}
			cross := (bx-ax)*(y-ay) - (by-ay)*(x-ax)
			if ay <= y && by > y {
				total++
			}
			if ay > y && by <= y {
				total--
			}
			if ay <= y && by > y && cross > 0 {
				winding++
			}
			if ay > y && by <= y && cross < 0 {
				winding--
			}
		}
	}
	return winding, total
}

func automapBoundaryContains(m *mapdata.Map, sector int, x, y float64) bool {
	winding, _ := automapBoundaryWinding(m, sector, x, y)
	return winding != 0
}

func TestAutomapE1M3ExitRoomBoundaryCoverage(t *testing.T) {
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "E1M3")
	if err != nil {
		t.Fatal(err)
	}
	flats, err := doomtex.LoadFlatsRGBA(wf, 0)
	if err != nil {
		t.Fatal(err)
	}
	g := newGame(m, Options{Width: 640, Height: 400, SourcePortMode: true, StartInMapMode: true, FlatBank: flats, WallTexBank: map[string]WallTexture{}})
	before, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	g.ensureMapFloorBoundarySetsBuilt()
	g.refreshSectorPlaneCacheTextureRefs()
	g.parity.reveal = revealAllMap
	sets := g.mapFloorBoundarySetsForView()
	const sector = 7
	if m.Sectors[sector].FloorHeight != 48 || m.Sectors[sector].FloorPic != "FLOOR7_1" {
		t.Fatal("unexpected exit-room fixture")
	}
	// The old ring tracer drops the outer boundary but keeps three inner
	// outlines. Prove that this is the failing case, rather than an easy room.
	old := g.buildSectorLoopSets()[sector]
	if pointInRingsEvenOdd(-400, -1600, old.rings) {
		t.Fatal("fixture no longer reproduces the lost outer boundary")
	}
	tex := g.mapFloorTextures()[sector]
	if len(tex) != 64*64*4 {
		t.Fatal("exit-room texture was not resolved")
	}
	for _, angle := range []float64{0, 0.37, math.Pi / 2} {
		t.Run(fmt.Sprintf("angle_%.2f", angle), func(t *testing.T) {
			const w, h = 640, 640
			c, s := math.Cos(angle), math.Sin(angle)
			toWorld := func(px, py float64) (float64, float64) {
				x, y := (px-w/2)*2, (py-h/2)*2
				return -128.125 + c*x - s*y, -1600.25 + s*x + c*y
			}
			toScreen := func(x, y float64) (float64, float64) {
				x, y = x+128.125, y+1600.25
				return (c*x+s*y)/2 + w/2, (-s*x+c*y)/2 + h/2
			}
			pix := make([]byte, w*h*4)
			mapview.RasterizeFloor2D(pix, mapview.FloorRasterInput{
				ViewW: w, ViewH: h, ViewBBox: mapview.WorldBBox{-2000, -3000, 1500, 0},
				LoopSets: []mapview.FloorLoopSet{sets[sector]}, Textures: [][]byte{tex},
				ScreenToWorld: toWorld, WorldToScreen: toScreen,
			})
			checked, filled := 0, 0
			for py := 0; py < h; py += 3 {
				for px := 0; px < w; px += 3 {
					x, y := toWorld(float64(px)+0.5, float64(py)+0.5)
					want := automapBoundaryContains(m, sector, x, y)
					i := (py*w + px) * 4
					if got := pix[i+3] != 0; got != want {
						t.Fatalf("(%g,%g): filled=%t want %t", x, y, got, want)
					}
					if want {
						filled++
						ti := ((int(math.Floor(y))&63)*64 + (int(math.Floor(x)) & 63)) * 4
						if !bytes.Equal(pix[i:i+3], tex[ti:ti+3]) {
							t.Fatalf("texture sampling mismatch at (%g,%g)", x, y)
						}
					}
					checked++
				}
			}
			if filled < 5000 {
				t.Fatalf("only %d room samples filled", filled)
			}
			t.Logf("%d samples: room filled, pillar holes and exterior clear", checked)
			if angle == 0 {
				captureAutomapBoundaryFrame(t, "e1m3-exit-room-fixed", pix, w, h)
				var rings [][]mapview.WorldPt
				for _, ring := range old.rings {
					var pts []mapview.WorldPt
					for _, p := range ring {
						pts = append(pts, mapview.WorldPt{X: p.x, Y: p.y})
					}
					rings = append(rings, pts)
				}
				clear(pix)
				mapview.RasterizeFloor2D(pix, mapview.FloorRasterInput{ViewW: w, ViewH: h, ViewBBox: mapview.WorldBBox{-2000, -3000, 1500, 0}, LoopSets: []mapview.FloorLoopSet{{Rings: rings, BBox: sets[sector].BBox}}, Textures: [][]byte{tex}, ScreenToWorld: toWorld, WorldToScreen: toScreen})
				captureAutomapBoundaryFrame(t, "e1m3-exit-room-before", pix, w, h)
			}
		})
	}
	after, _ := json.Marshal(m)
	if !bytes.Equal(before, after) {
		t.Fatal("automap boundary preparation changed map data")
	}
	// A sector becomes visible only through mapped lines, unless fully revealed.
	g.parity.reveal = 0
	for i := range m.Linedefs {
		m.Linedefs[i].Flags &^= mlMapped
	}
	if len(g.mapFloorBoundarySetsForView()[sector].Edges) != 0 {
		t.Fatal("hidden room revealed")
	}
	m.Linedefs[933].Flags |= mlMapped
	if len(g.mapFloorBoundarySetsForView()[sector].Edges) == 0 {
		t.Fatal("mapped room remained hidden")
	}
}

func captureAutomapBoundaryFrame(t *testing.T, name string, pix []byte, w, h int) {
	t.Helper()
	dir := os.Getenv("GD_AUTOMAP_CAPTURE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	img := &image.RGBA{Pix: append([]byte(nil), pix...), Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAutomapBoundaryMaps(t *testing.T) {
	paths := os.Getenv("GD_GEOMETRY_WADS")
	if paths == "" {
		t.Skip("set GD_GEOMETRY_WADS to comma-separated IWAD paths")
	}
	marker := regexp.MustCompile(`^(E[1-9]M[1-9]|MAP[0-9][0-9])$`)
	for _, path := range strings.Split(paths, ",") {
		wf, err := wad.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, lump := range wf.Lumps {
			if !marker.MatchString(lump.Name) {
				continue
			}
			m, err := mapdata.LoadMap(wf, mapdata.MapName(lump.Name))
			if err != nil {
				t.Fatal(err)
			}
			g := &game{m: m}
			g.ensureMapFloorBoundarySetsBuilt()
			covered, open := 0, 0
			for sec, set := range g.mapFloorBoundarySets {
				if len(set.Edges) == 0 {
					continue
				}
				const w, h = 40, 40
				// Offset sample centers away from integral map boundaries.
				sx, sy := (set.BBox.MaxX-set.BBox.MinX+2.17)/w, (set.BBox.MaxY-set.BBox.MinY+1.79)/h
				toWorld := func(x, y float64) (float64, float64) { return set.BBox.MinX - 1.11 + x*sx, set.BBox.MinY - 0.93 + y*sy }
				toScreen := func(x, y float64) (float64, float64) {
					return (x - set.BBox.MinX + 1.11) / sx, (y - set.BBox.MinY + 0.93) / sy
				}
				pix := make([]byte, w*h*4)
				mapview.RasterizeFloor2D(pix, mapview.FloorRasterInput{ViewW: w, ViewH: h, ViewBBox: mapview.WorldBBox{set.BBox.MinX - 2, set.BBox.MinY - 2, set.BBox.MaxX + 2, set.BBox.MaxY + 2}, LoopSets: []mapview.FloorLoopSet{set}, ScreenToWorld: toWorld, WorldToScreen: toScreen})
				for py := 0; py < h; py += 7 {
					for px := 0; px < w; px += 7 {
						x, y := toWorld(float64(px)+0.5, float64(py)+0.5)
						winding, total := automapBoundaryWinding(m, sec, x, y)
						// An isolated sidedef can have nonzero winding all the way
						// to infinity. It does not define a bounded floor interior.
						// Audit such rows separately rather than asserting that
						// their unbounded half-plane should be textured.
						if total != 0 {
							open++
							continue
						}
						want := x >= set.BBox.MinX && x < set.BBox.MaxX && y >= set.BBox.MinY && y < set.BBox.MaxY && winding != 0
						if got := pix[(py*w+px)*4+3] != 0; got != want {
							t.Fatalf("%s %s sector %d (%g,%g): filled=%t want %t", path, m.Name, sec, x, y, got, want)
						}
						covered++
					}
				}
			}
			t.Logf("%s %s: %d bounded samples verified, %d open-boundary samples", filepath.Base(path), m.Name, covered, open)
		}
	}
}
