package doomruntime

import (
	"testing"
	"time"
)

func TestNativeDetailCycleMatchesMainLayoutAndSurvivesCampaignChanges(t *testing.T) {
	c := nativeSaveFixture(t)
	checksum := c.Game.g.SimChecksum()
	for _, want := range []struct {
		level int
		auto  bool
		label string
	}{
		{1, false, "1/2x"}, {2, false, "1/3x"}, {3, false, "1/4x"}, {3, true, "AUTO"}, {0, false, "1x"},
	} {
		c.CycleDetail()
		level, auto := c.DetailSettings()
		if level != want.level || auto != want.auto || c.Game.g.useText != "Detail: "+want.label {
			t.Fatalf("cycle: level=%d auto=%t message=%q", level, auto, c.Game.g.useText)
		}
		main := &sessionGame{opts: c.Game.g.opts, g: c.Game.g, rt: c.Game.g}
		for _, size := range [][2]int{{1280, 800}, {641, 401}, {1, 1}} {
			main.Layout(size[0], size[1])
			w, h := c.SceneSize(size[0], size[1])
			if w != c.Game.g.viewW || h != c.Game.g.viewH {
				t.Fatalf("layout %v: native %dx%d main %dx%d", size, w, h, c.Game.g.viewW, c.Game.g.viewH)
			}
		}
		if c.Game.g.SimChecksum() != checksum {
			t.Fatal("detail cycle changed simulation")
		}
	}
	// A save does not capture resolution. Loading retains the current runtime
	// preference, including AUTO, just as the main session does.
	data, err := c.SaveData("detail")
	if err != nil {
		t.Fatal(err)
	}
	for range 4 {
		c.CycleDetail()
	}
	assertSettings := func() {
		t.Helper()
		level, auto := c.DetailSettings()
		if level != 3 || !auto {
			t.Fatalf("lost detail: %d/%t", level, auto)
		}
	}
	if err := c.LoadData(data); err != nil {
		t.Fatal(err)
	}
	assertSettings()
	fresh, err := c.session.opts.NewGameLoader("E1M1")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Restart(fresh); err != nil {
		t.Fatal(err)
	}
	assertSettings()
	c.Game.g.requestLevelExit(false, "detail")
	if err := c.Tick(NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	finishNativeIntermission(t, c)
	assertSettings()
}

func TestNativeRenderFramesDriveSharedAutoDetailPolicy(t *testing.T) {
	c := nativeCampaignFixture(t, "E1M1")
	for range 4 {
		c.CycleDetail()
	}
	g := c.Game.g
	g.setDetailLevel(0)
	g.autoDetailPeriodSeen = true
	checksum := g.SimChecksum()
	// Each frame closes a five-second worst-case period after its one-second
	// FPS counter. Four sustained low periods must change detail exactly once.
	for period := range 4 {
		now := time.Now()
		g.fpsStamp = now.Add(-time.Second)
		g.fpsFrames = 49
		g.autoDetailPeriodStart = now.Add(-6 * time.Second)
		g.autoDetailPeriodWorstFPS = 50
		g.autoDetailPeriodWorstRenderMS = 20
		c.RecordRenderFrame(20 * time.Millisecond)
		want := 0
		if period == 3 {
			want = 1
		}
		level, auto := c.DetailSettings()
		if level != want || !auto {
			t.Fatalf("period %d detail %d/%t", period, level, auto)
		}
	}
	if c.session.opts.InitialDetailLevel != 1 || !c.session.opts.AutoDetail {
		t.Fatal("AUTO preference did not reach campaign options")
	}
	if g.SimChecksum() != checksum {
		t.Fatal("AUTO changed simulation")
	}
	c.Game.SetMapActive(true)
	g.autoDetailCooldown = 0
	for range 8 {
		g.applyAutoDetailSample(40, 25)
	}
	if level, _ := c.DetailSettings(); level != 1 {
		t.Fatal("AUTO changed detail in map view")
	}
}

func TestNativeMapPresentationShortcutsInWalkView(t *testing.T) {
	n := loadNativeCombatGame(t)
	g := n.g
	checksum := g.SimChecksum()
	x, y, zoom := g.State.CamX, g.State.CamY, g.State.Zoom
	grid, rotate, legend := g.showGrid, g.rotateView, g.showLegend
	for _, step := range []struct {
		input   NativeMapInput
		message string
	}{
		{NativeMapInput{ToggleGrid: true}, "Grid "},
		{NativeMapInput{ToggleRotate: true}, "Heading-Up "},
		{NativeMapInput{ToggleReveal: true}, "Allmap ON"},
		{NativeMapInput{CycleIDDT: true}, "IDDT 1"},
		{NativeMapInput{CycleThings: true}, "Thing Render: "},
		{NativeMapInput{ToggleLegend: true}, "Thing Legend "},
	} {
		n.ApplyMapPresentation(step.input)
		if len(g.useText) < len(step.message) || g.useText[:len(step.message)] != step.message {
			t.Fatalf("message=%q want prefix %q", g.useText, step.message)
		}
	}
	if g.mode != viewWalk || g.State.CamX != x || g.State.CamY != y || g.State.Zoom != zoom || g.SimChecksum() != checksum {
		t.Fatal("walk shortcuts moved camera or changed simulation")
	}
	if g.showGrid == grid || g.rotateView == rotate || g.showLegend == legend || g.parity.reveal != revealAllMap || g.parity.iddt != 1 {
		t.Fatal("walk shortcuts lost map settings")
	}
	n.SetMapActive(true)
	if g.showGrid == grid || g.rotateView == rotate || g.showLegend == legend {
		t.Fatal("opening map lost walk shortcuts")
	}
}
