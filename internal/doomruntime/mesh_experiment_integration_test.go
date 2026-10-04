//go:build integration

package doomruntime

import (
	"os"
	"testing"

	"gddoom/internal/render/levelmesh"
	"github.com/hajimehoshi/ebiten/v2"
)

type meshPresentationDriver struct {
	done bool
	w, h int
	draw func(*ebiten.Image)
}

func (d *meshPresentationDriver) Update() error {
	if d.done {
		return ebiten.Termination
	}
	return nil
}
func (d *meshPresentationDriver) Layout(int, int) (int, int) {
	if d.w > 0 && d.h > 0 {
		return d.w, d.h
	}
	return 640, 400
}
func (d *meshPresentationDriver) Draw(screen *ebiten.Image) {
	if !d.done {
		d.draw(screen)
		d.done = true
	}
}

func TestMeshExperimentPresentation(t *testing.T) {
	if os.Getenv("GD_MESH_INTEGRATION") == "" {
		t.Skip("set GD_MESH_INTEGRATION=1 for in-game framebuffer checks")
	}
	g := loadMeshExperimentGame(t)
	driver := &meshPresentationDriver{draw: func(screen *ebiten.Image) {
		for _, mode := range []levelmesh.Mode{levelmesh.Textured, levelmesh.Sectors, levelmesh.Wireframe} {
			g.ensureMeshExperiment().mode = mode
			g.Draw(screen)
			pixels := make([]byte, 640*400*4)
			screen.ReadPixels(pixels)
			// The world and status bar must both reach the final framebuffer.
			for _, region := range [][2]int{{40 * 640 * 4, 300 * 640 * 4}, {370 * 640 * 4, 390 * 640 * 4}} {
				changed := false
				for p := region[0]; p < region[1]; p += 4 {
					if pixels[p] != pixels[region[0]] || pixels[p+1] != pixels[region[0]+1] || pixels[p+2] != pixels[region[0]+2] {
						changed = true
						break
					}
				}
				if !changed {
					t.Fatalf("%s: empty framebuffer region", mode)
				}
			}
			captureMeshFrame(t, "e1m1-game-mesh-"+string(mode), pixels, 640, 400)
		}
	}}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}

func TestMeshExperimentE1M3ExitRoomPresentation(t *testing.T) {
	if os.Getenv("GD_MESH_INTEGRATION") == "" {
		t.Skip("set GD_MESH_INTEGRATION=1 for in-game framebuffer checks")
	}
	g := loadMeshExperimentMap(t, "E1M3")
	g.p.x, g.p.y = -600*fracUnit, -1600*fracUnit
	g.p.z, g.playerViewZ, g.p.angle = 48*fracUnit, 89*fracUnit, 0
	g.Layout(1280, 800)
	g.syncRenderState()
	r := g.ensureMeshExperiment()
	fixed := r.planes
	old := make([][]levelmesh.PlaneTriangle, len(fixed))
	for sector, set := range g.buildSectorLoopSets() {
		var rings [][]levelmesh.Point2
		for _, ring := range set.rings {
			var points []levelmesh.Point2
			for _, p := range ring {
				points = append(points, levelmesh.Point2{X: p.x, Y: p.y})
			}
			rings = append(rings, points)
		}
		old[sector], _ = levelmesh.TriangulateRings(rings)
	}
	driver := &meshPresentationDriver{w: 1280, h: 800, draw: func(screen *ebiten.Image) {
		r.planes = old
		g.Draw(screen)
		beforeDepth := append([]float64(nil), r.raster.Depth...)
		pixels := make([]byte, 1280*800*4)
		screen.ReadPixels(pixels)
		captureMeshFrame(t, "e1m3-exit-room-before", pixels, 1280, 800)
		r.planes = fixed
		g.Draw(screen)
		screen.ReadPixels(pixels)
		captureMeshFrame(t, "e1m3-exit-room-fixed", pixels, 1280, 800)
		floor, ceiling := 0, 0
		for i, depth := range r.raster.Depth {
			if depth <= 0 || beforeDepth[i] > 0 {
				continue
			}
			row := i / r.raster.Width
			if row > r.raster.Height*3/5 {
				floor++
			}
			if row < r.raster.Height*2/5 {
				ceiling++
			}
		}
		if floor < 1000 || ceiling < 1000 {
			t.Fatalf("only recovered %d floor and %d ceiling pixels", floor, ceiling)
		}
		t.Logf("1280x800 presentation: recovered %d floor and %d ceiling pixels", floor, ceiling)
	}}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}
