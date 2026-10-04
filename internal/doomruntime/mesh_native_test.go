package doomruntime

import (
	"math"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/wad"
)

func TestNativeMeshGameUsesDoomSimulation(t *testing.T) {
	fixture := loadMeshExperimentGame(t)
	n := NewNativeMeshGame(fixture.m, fixture.opts)
	// Turn and move through the same fixed-tic simulation used by the main
	// game. The native host never invokes Ebiten input or a drawing loop.
	before := n.Frame(1)
	for range 8 {
		n.Tick(NativeMeshInput{Forward: 1, Turn: 1})
	}
	after := n.Frame(1)
	if after.WorldTic != before.WorldTic+8 || (after.Camera.X == before.Camera.X && after.Camera.Y == before.Camera.Y) || after.Camera.Yaw == before.Camera.Yaw {
		t.Fatalf("native input did not drive Doom: before=%+v after camera=%+v tic=%d", before.Camera, after.Camera, after.WorldTic)
	}
	checksum := n.g.SimChecksum()
	for _, alpha := range []float64{0, 0.5, 1} {
		n.Frame(alpha)
	}
	if n.g.SimChecksum() != checksum {
		t.Fatal("frame extraction changed simulation")
	}
	if err := n.SetPose(math.NaN(), 0, 41, 0); err == nil {
		t.Fatal("NaN camera accepted")
	}
	if err := n.SetPose(40000, 0, 41, 0); err == nil {
		t.Fatal("out-of-range camera accepted")
	}
}

func TestNativeMeshGameDynamicPlaneHeights(t *testing.T) {
	g := loadMeshExperimentMap(t, "E1M3")
	n := NewNativeMeshGame(g.m, g.opts)
	n.Frame(1)
	n.g.sectorFloor[7] += 8 * fracUnit
	frame := n.Frame(1)
	found := false
	for _, tri := range frame.Triangles {
		if tri.Sector == 7 && tri.Kind == levelmesh.Floor {
			found = true
			if tri.Vertices[0].Z != 56 {
				t.Fatal("native snapshot kept stale floor height")
			}
		}
	}
	if !found {
		t.Fatal("native snapshot lost E1M3 exit-room floor")
	}
}

func TestNativeMeshUseOpensDoorAndUpdatesGeometry(t *testing.T) {
	fixture := loadMeshExperimentGame(t)
	n := NewNativeMeshGame(fixture.m, fixture.opts)
	n.Tick(NativeMeshInput{}) // Release Doom's initial reborn use-button latch.
	for idx, line := range fixture.m.Linedefs {
		if line.Special != 1 || line.SideNum[0] < 0 || line.SideNum[1] < 0 {
			continue
		}
		a, b := fixture.m.Vertexes[line.V1], fixture.m.Vertexes[line.V2]
		dx, dy := float64(b.X)-float64(a.X), float64(b.Y)-float64(a.Y)
		length := math.Hypot(dx, dy)
		if length == 0 {
			continue
		}
		front := int(fixture.m.Sidedefs[line.SideNum[0]].Sector)
		door := int(fixture.m.Sidedefs[line.SideNum[1]].Sector)
		x := (float64(a.X)+float64(b.X))/2 + dy/length*32
		y := (float64(a.Y)+float64(b.Y))/2 - dx/length*32
		eye := float64(n.g.sectorFloor[front])/fracUnit + 41
		if err := n.SetPose(x, y, eye, math.Atan2(dx, -dy)); err != nil {
			t.Fatal(err)
		}
		if target, trace := n.g.peekUseTargetLine(); target != idx || trace != useTraceSpecial {
			continue // Find a door with an unobstructed front-side approach.
		}
		before := n.g.sectorCeil[door]
		n.Tick(NativeMeshInput{Use: true})
		for range 7 {
			n.Tick(NativeMeshInput{})
		}
		if n.g.doors[door] == nil || n.g.sectorCeil[door] <= before {
			t.Fatalf("native use/tick did not open door: line=%d front=%d door=%d player-sector=%d before=%d after=%d use=%s", idx, front, door, n.g.playerSector(), before/fracUnit, n.g.sectorCeil[door]/fracUnit, n.g.useText)
		}
		// Alpha zero exposes the exact simulation height; the existing
		// renderer predicts moving doors forward for positive alpha.
		for _, tri := range n.Frame(0).Triangles {
			if tri.Sector == door && tri.Kind == levelmesh.Ceiling {
				if tri.Vertices[0].Z != float64(n.g.sectorCeil[door])/fracUnit {
					t.Fatal("native mesh retained the closed door height")
				}
				t.Logf("opened E1M1 door line %d, sector %d", idx, door)
				return
			}
		}
		t.Fatal("opened door ceiling missing from native mesh")
	}
	t.Fatal("fixture has no reachable manual door")
}

func TestNativeMeshInspectionPoseUsesDestinationSector(t *testing.T) {
	g := loadMeshExperimentMap(t, "E1M3")
	n := NewNativeMeshGame(g.m, g.opts)
	if err := n.SetPose(-600, -1600, 89, 0); err != nil {
		t.Fatal(err)
	}
	if n.g.playerSector() != 7 || n.g.p.floorz != 48*fracUnit {
		t.Fatal("inspection pose retained spawn-sector physics")
	}
	for range 20 {
		n.Tick(NativeMeshInput{})
	}
	c := n.Frame(1).Camera
	if c.X != -600 || c.Y != -1600 || math.Abs(c.Z-89) > 0.01 {
		t.Fatalf("stationary inspection camera moved after physics ticks: %+v", c)
	}
}

func TestNativeMeshAnimatedLightsAndPowerupSnapshot(t *testing.T) {
	fixture := loadMeshExperimentMap(t, "E1M3")
	// The fixture's game has already consumed the map's light specials.
	// Load an original map, as the native launcher does, before starting it.
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "E1M3")
	if err != nil {
		t.Fatal(err)
	}
	opts := fixture.opts
	opts.SourcePortSectorLighting = true
	// An intentionally non-linear colormap proves that the native host passes
	// the WAD's brightness curve rather than an assumed linear falloff.
	opts.DoomPaletteRGBA = make([]byte, 256*4)
	opts.DoomColorMapRows = 32
	opts.DoomColorMap = make([]byte, 32*256)
	for i := range 256 {
		opts.DoomPaletteRGBA[i*4], opts.DoomPaletteRGBA[i*4+1], opts.DoomPaletteRGBA[i*4+2], opts.DoomPaletteRGBA[i*4+3] = byte(i), byte(i), byte(i), 255
		for row := range 32 {
			if row != 3 {
				opts.DoomColorMap[row*256+i] = byte(i)
			}
		}
	}
	n := NewNativeMeshGame(m, opts)
	if ramp := n.LightRamp(); ramp[0] != 1 || ramp[3] != 0 || ramp[4] != 1 {
		t.Fatal("native host did not preserve the WAD's non-linear light ramp")
	}
	changed := false
	for range 64 {
		before := make([]int16, len(n.g.m.Sectors))
		for sector := range before {
			before[sector] = n.g.m.Sectors[sector].Light
		}
		n.Tick(NativeMeshInput{})
		n.Frame(1)
		for sector, old := range before {
			current := n.g.m.Sectors[sector].Light
			if current != old {
				changed = true
				if n.Light(sector)*256 != float64(current) {
					t.Fatal("native light snapshot retained an old flicker/glow state")
				}
			}
		}
	}
	if !changed {
		t.Fatal("fixture did not animate its sector lights")
	}
	for _, power := range []struct {
		tics   int
		bright bool
	}{{160, true}, {7, false}, {8, true}, {0, false}} {
		n.g.inventory.LightAmpTics = power.tics
		if n.Frame(1).Fullbright != power.bright {
			t.Fatal("native frame lost the light-amplification blink")
		}
	}
}
