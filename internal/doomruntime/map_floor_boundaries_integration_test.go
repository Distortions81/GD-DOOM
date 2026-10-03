//go:build integration

package doomruntime

import (
	"os"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/mapview"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

type automapBoundaryDriver struct {
	done bool
	draw func(*ebiten.Image)
}

func (d *automapBoundaryDriver) Update() error {
	if d.done {
		return ebiten.Termination
	}
	return nil
}
func (d *automapBoundaryDriver) Layout(int, int) (int, int) { return 1280, 800 }
func (d *automapBoundaryDriver) Draw(screen *ebiten.Image) {
	if !d.done {
		d.draw(screen)
		d.done = true
	}
}

func TestAutomapE1M3Presentation(t *testing.T) {
	if os.Getenv("GD_AUTOMAP_INTEGRATION") == "" {
		t.Skip("set GD_AUTOMAP_INTEGRATION=1 for framebuffer checks")
	}
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
	g := newGame(m, Options{Width: 1280, Height: 800, SourcePortMode: true, SourcePortSectorLighting: true, StartInMapMode: true, NoFPS: true, FlatBank: flats, WallTexBank: map[string]WallTexture{}})
	g.mode = viewMap
	g.State.SetFollowMode(false)
	g.State.SetCamera(-128, -1600)
	g.State.Zoom = 0.8
	g.State.SyncRender()
	g.parity.reveal = revealAllMap
	driver := &automapBoundaryDriver{draw: func(screen *ebiten.Image) {
		g.Draw(screen)
		pixels := make([]byte, 1280*800*4)
		screen.ReadPixels(pixels)
		for _, p := range [][2]float64{{-400, -1600}, {-80, -1480}} {
			x, y := g.worldToScreen(p[0], p[1])
			i := (int(y)*1280 + int(x)) * 4
			if g.mapFloorPix[i+3] != 255 {
				t.Fatalf("exit-room floor is missing at %v in production minimap draw", p)
			}
			if g.mapFloorPix[i] == 0 && g.mapFloorPix[i+1] == 0 && g.mapFloorPix[i+2] == 0 {
				t.Fatalf("exit-room texture is black at %v", p)
			}
			if pixels[i] != g.mapFloorPix[i] || pixels[i+1] != g.mapFloorPix[i+1] || pixels[i+2] != g.mapFloorPix[i+2] {
				t.Fatalf("floor did not reach final framebuffer at %v", p)
			}
		}
		captureAutomapBoundaryFrame(t, "e1m3-minimap-fixed", pixels, 1280, 800)
		// Capture the same view through the old ring input for comparison.
		old := g.buildSectorLoopSets()
		for sec, set := range old {
			var rings [][]mapview.WorldPt
			for _, ring := range set.rings {
				var pts []mapview.WorldPt
				for _, p := range ring {
					pts = append(pts, mapview.WorldPt{X: p.x, Y: p.y})
				}
				rings = append(rings, pts)
			}
			g.mapFloorBoundarySets[sec] = mapview.FloorLoopSet{Rings: rings, BBox: mapview.WorldBBox{MinX: set.bbox.minX, MinY: set.bbox.minY, MaxX: set.bbox.maxX, MaxY: set.bbox.maxY}}
		}
		g.Draw(screen)
		screen.ReadPixels(pixels)
		captureAutomapBoundaryFrame(t, "e1m3-minimap-before", pixels, 1280, 800)
	}}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}
