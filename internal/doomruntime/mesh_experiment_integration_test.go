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
	draw func(*ebiten.Image)
}

func (d *meshPresentationDriver) Update() error {
	if d.done {
		return ebiten.Termination
	}
	return nil
}
func (d *meshPresentationDriver) Layout(int, int) (int, int) { return 640, 400 }
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
