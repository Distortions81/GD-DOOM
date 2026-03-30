package doomruntime

import "testing"

func TestMouseLookTurnRawWithWidthIgnoresResolution(t *testing.T) {
	base := mouseLookTurnRawWithWidth(10, 1.0, doomLogicalW)
	if base >= 0 {
		t.Fatalf("base turn=%d want negative for +dx", base)
	}
	doubleW := mouseLookTurnRawWithWidth(10, 1.0, doomLogicalW*2)
	if doubleW >= 0 {
		t.Fatalf("double-width turn=%d want negative for +dx", doubleW)
	}
	halfW := mouseLookTurnRawWithWidth(10, 1.0, doomLogicalW/2)
	if halfW >= 0 {
		t.Fatalf("half-width turn=%d want negative for +dx", halfW)
	}
	if doubleW != base {
		t.Fatalf("double-width turn=%d want=%d", doubleW, base)
	}
	if halfW != base {
		t.Fatalf("half-width turn=%d want=%d", halfW, base)
	}
}

func TestMouseLookTurnRawWithWidthPreservesDirectionAndMinimumStep(t *testing.T) {
	if got := mouseLookTurnRawWithWidth(0, 1.0, doomLogicalW); got != 0 {
		t.Fatalf("dx=0 got=%d want=0", got)
	}
	if got := mouseLookTurnRawWithWidth(1, 0.0000001, doomLogicalW); got != -1 {
		t.Fatalf("+tiny dx got=%d want=-1", got)
	}
	if got := mouseLookTurnRawWithWidth(-1, 0.0000001, doomLogicalW); got != 1 {
		t.Fatalf("-tiny dx got=%d want=+1", got)
	}
}

func TestLiveMouseLookRenderActiveIgnoresDemoPlayback(t *testing.T) {
	g := &game{
		mode: viewWalk,
		opts: Options{
			MouseLook:  true,
			DemoScript: &DemoScript{},
		},
		mouseLookSet: true,
	}
	if g.liveMouseLookRenderActive() {
		t.Fatal("demo playback should not use live render-time mouse yaw")
	}
}

func TestLiveMouseLookRenderActiveRequiresUnsuppressedWalkMouselook(t *testing.T) {
	g := &game{
		mode: viewWalk,
		opts: Options{
			MouseLook: true,
		},
		mouseLookSet: true,
	}
	if !g.liveMouseLookRenderActive() {
		t.Fatal("expected live gameplay mouse yaw override to be active")
	}
	g.mouseLookSuppressTicks = 1
	if g.liveMouseLookRenderActive() {
		t.Fatal("suppressed mouselook should not affect render yaw")
	}
	g.mouseLookSuppressTicks = 0
	g.mode = viewMap
	if g.liveMouseLookRenderActive() {
		t.Fatal("non-walk view should not use live render-time mouse yaw")
	}
}

func TestLiveMouseLookRenderActiveStopsForMenus(t *testing.T) {
	g := &game{
		mode: viewWalk,
		opts: Options{
			MouseLook: true,
		},
		mouseLookSet: true,
	}
	if !g.liveMouseLookRenderActive() {
		t.Fatal("expected live gameplay mouse yaw override to be active")
	}
	g.pauseMenuActive = true
	if g.liveMouseLookRenderActive() {
		t.Fatal("pause menu should block live render-time mouse yaw")
	}
	g.pauseMenuActive = false
	g.quitPromptActive = true
	if g.liveMouseLookRenderActive() {
		t.Fatal("quit prompt should block live render-time mouse yaw")
	}
	g.quitPromptActive = false
	g.frontendActive = true
	if g.liveMouseLookRenderActive() {
		t.Fatal("frontend overlay should block live render-time mouse yaw")
	}
}

func TestRenderAngleWithCursorAppliesCurrentMouseDelta(t *testing.T) {
	g := &game{
		mode:       viewWalk,
		lastMouseX: 100,
		opts: Options{
			MouseLook:      true,
			MouseLookSpeed: 1.0,
		},
		mouseLookSet: true,
	}
	base := uint32(123 << 16)
	got := g.renderAngleWithCursor(base, 112)
	want := uint32(int64(base) + g.mouseLookTurnRaw(12))
	if got != want {
		t.Fatalf("render angle=%d want=%d", got, want)
	}
}

func TestBaseRenderCameraAngleUsesCommittedAngleForLiveMouseLook(t *testing.T) {
	g := &game{
		mode: viewWalk,
		p: player{
			angle: 300,
		},
		prevPrevAngle: 100,
		prevAngle:     200,
		opts: Options{
			MouseLook:       true,
			SmoothCameraYaw: true,
		},
		mouseLookSet: true,
	}
	if got := g.baseRenderCameraAngle(0.5); got != g.p.angle {
		t.Fatalf("base render angle=%d want committed angle=%d", got, g.p.angle)
	}
}
