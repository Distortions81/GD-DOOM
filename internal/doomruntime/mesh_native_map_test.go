package doomruntime

import (
	"gddoom/internal/render/mapview"
	"math"
	"testing"
)

func TestNativeMapExitRoomFloorsAndExploration(t *testing.T) {
	fixture := loadMeshExperimentMap(t, "E1M3")
	n := NewNativeMeshGame(fixture.m, fixture.opts)
	n.SetPose(-600, -1600, 89, 0)
	n.SetMapActive(true)
	n.MapViewport(640, 640)
	g := n.g
	// A fixed north-up view reproduces the formerly missing exit-room floor.
	g.State.SetFollowMode(false)
	g.State.SetCamera(-128, -1600)
	g.State.Zoom = .8
	g.State.SyncRender()
	g.rotateView = false
	g.parity.reveal = revealAllMap
	checksum := g.SimChecksum()
	for _, rotate := range []bool{false, true} {
		g.rotateView = rotate
		g.renderAngle = uint32(uint64(35) * 4294967296 / 360)
		frame := n.MapFrame(640, 640)
		for _, p := range [][2]float64{{-400, -1600}, {-80, -1480}} {
			x, y := g.worldToScreen(p[0], p[1])
			i := (int(y)*640 + int(x)) * 4
			if frame.Pixels[i+3] != 255 {
				t.Fatalf("exit-room floor missing at %v, rotated=%t", p, rotate)
			}
		}
		if len(frame.Lines) == 0 || len(frame.Segments) == 0 {
			t.Fatal("map omitted wall and player vectors")
		}
	}
	if g.SimChecksum() != checksum {
		t.Fatal("map extraction changed gameplay state")
	}
	g.parity.reveal = revealNormal
	for i := range g.m.Linedefs {
		g.m.Linedefs[i].Flags &^= mlMapped
	}
	g.mapLines.Touch()
	frame := n.MapFrame(640, 640)
	for i := 3; i < len(frame.Pixels); i += 4 {
		if frame.Pixels[i] != 0 {
			t.Fatal("undiscovered floor revealed")
		}
	}
	if len(frame.Lines) != 0 || len(frame.Patches) != 0 {
		t.Fatal("unexplored geometry or things leaked through map")
	}
	n.TypeCheats([]rune("iddt"))
	frame = n.MapFrame(640, 640)
	if len(frame.Lines) == 0 {
		t.Fatal("IDDT did not reveal native map lines")
	}
}

func TestNativeMapControlsAndGameplayParity(t *testing.T) {
	n := loadNativeCombatGame(t)
	reference := loadNativeCombatGame(t)
	n.SetMapActive(true)
	n.MapViewport(640, 400)
	for i := 0; i < 12; i++ {
		in := NativeMeshInput{Forward: 1, Turn: 1, Fire: true}
		reference.Tick(in)
		in.Map = &NativeMapInput{InputState: mapview.InputState{ZoomInHeld: true}}
		n.Tick(in)
		if n.g.SimChecksum() != reference.g.SimChecksum() {
			t.Fatalf("map mode diverged from shared combat on tic %d", i)
		}
	}
	g := n.g
	n.Tick(NativeMeshInput{Map: &NativeMapInput{InputState: mapview.InputState{ToggleFollowPressed: true, PanRightHeld: true, AddMarkPressed: true}, ToggleGrid: true}})
	x, y := g.State.Camera()
	if g.State.FollowMode || !g.showGrid || g.marks.Count() != 1 {
		t.Fatal("follow/grid/mark controls missing")
	}
	n.Tick(NativeMeshInput{Map: &NativeMapInput{InputState: mapview.InputState{PanRightHeld: true}}})
	if g.State.CamX <= x || g.State.CamY != y {
		t.Fatal("arrow panning did not move map camera")
	}
	savedX, savedY, savedZoom := g.State.CamX, g.State.CamY, g.State.Zoom
	n.Tick(NativeMeshInput{Map: &NativeMapInput{InputState: mapview.InputState{ToggleBigMapPressed: true}}})
	if math.Abs(g.State.Zoom-g.State.FitZoom) > 1e-9 {
		t.Fatal("whole-map view did not fit level")
	}
	n.Tick(NativeMeshInput{Map: &NativeMapInput{InputState: mapview.InputState{ToggleBigMapPressed: true, ClearMarksPressed: true}}})
	if g.State.CamX != savedX || g.State.CamY != savedY || g.State.Zoom != savedZoom || g.marks.Count() != 0 {
		t.Fatal("whole-map view did not restore saved view")
	}
	relativeZoom := g.State.Zoom / g.State.FitZoom
	n.MapViewport(1280, 800)
	if math.Abs(g.State.Zoom/g.State.FitZoom-relativeZoom) > 1e-9 {
		t.Fatal("resize lost relative map zoom")
	}
	n.Frame(1)
	f := n.MapFrame(1280, 800)
	if len(f.Pixels) != 1280*800*4 || f.GridSegments == 0 {
		t.Fatal("resized floor/grid snapshot invalid")
	}
}

func TestNativeMapInspectionCentersOnPlayer(t *testing.T) {
	n := loadNativeCombatGame(t)
	n.SetPose(0, 0, 41, 0)
	n.SetMapActive(true)
	n.Frame(1)
	n.MapFrame(640, 400)
	x, y := n.g.worldToScreen(n.g.renderPX, n.g.renderPY)
	if math.Abs(x-320) > .001 || math.Abs(y-200) > .001 {
		t.Fatalf("inspection player off center: %g,%g", x, y)
	}
}
