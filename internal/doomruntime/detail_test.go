package doomruntime

import (
	"testing"
	"time"

	"gddoom/internal/platformcfg"
	"gddoom/internal/render/mapview"
)

func TestDetailPresetIndex(t *testing.T) {
	if got := detailPresetIndex(320, 200); got != 0 {
		t.Fatalf("detailPresetIndex(320,200)=%d want=0", got)
	}
	if got := detailPresetIndex(640, 400); got != 2 {
		t.Fatalf("detailPresetIndex(640,400)=%d want=2", got)
	}
}

func TestDefaultDetailLevelForModeSourcePortHalfDetail(t *testing.T) {
	if got := defaultDetailLevelForMode(1280, 800, true); got != 1 {
		t.Fatalf("defaultDetailLevelForMode(sourceport)=%d want=1", got)
	}
}

func TestDefaultDetailLevelForModeWASMSourcePortStartsAtHalfDetail(t *testing.T) {
	platformcfg.SetForcedWASMMode(true)
	defer platformcfg.SetForcedWASMMode(false)
	if got := defaultDetailLevelForMode(1280, 800, true); got != 1 {
		t.Fatalf("defaultDetailLevelForMode(wasm sourceport)=%d want=1", got)
	}
}

func TestDefaultDetailLevelForModeFaithfulStartsHigh(t *testing.T) {
	if got := defaultDetailLevelForMode(640, 400, false); got != 0 {
		t.Fatalf("defaultDetailLevelForMode(faithful)=%d want=0", got)
	}
}

func TestDetailHUDLabelShowsAuto(t *testing.T) {
	g := &game{autoDetailEnabled: true}
	if got := g.detailHUDLabel(); got != "AUTO" {
		t.Fatalf("detailHUDLabel()=%q want AUTO", got)
	}
}

func TestDetailLevelLabelForSourcePort(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true}}
	if got := g.detailLevelLabelFor(2); got != "1/3x" {
		t.Fatalf("detailLevelLabelFor(2)=%q want 1/3x", got)
	}
}

func TestEstimatedRenderMSForDetailLevelSourcePort(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true}, detailLevel: 2}
	if got := g.estimatedRenderMSForDetailLevel(1, 3.0); got != 6.75 {
		t.Fatalf("estimatedRenderMSForDetailLevel()=%v want 6.75", got)
	}
}

func TestCycleDetailLevelFaithfulTogglesHighLow(t *testing.T) {
	g := &game{
		State: mapview.ViewState{
			FitZoom: 1,
			Zoom:    1,
		},
		viewW:       320,
		viewH:       200,
		detailLevel: 1,
		bounds: bounds{
			minX: 0, minY: 0, maxX: 1024, maxY: 1024,
		},
		autoDetailEnabled:  true,
		hudMessagesEnabled: true,
	}
	g.cycleDetailLevel()
	if g.autoDetailEnabled {
		t.Fatal("first manual detail cycle should disable auto detail")
	}
	if g.viewW != 320 || g.viewH != 200 || g.detailLevel != 0 {
		t.Fatalf("after 1 cycle got %dx%d level=%d", g.viewW, g.viewH, g.detailLevel)
	}
	if g.useText != "Detail: HIGH" {
		t.Fatalf("after 1 cycle useText=%q want Detail: HIGH", g.useText)
	}
	g.cycleDetailLevel()
	if g.viewW != 320 || g.viewH != 200 || g.detailLevel != 1 {
		t.Fatalf("after 2 cycles got %dx%d level=%d", g.viewW, g.viewH, g.detailLevel)
	}
	if g.useText != "Detail: LOW" {
		t.Fatalf("after 2 cycles useText=%q want Detail: LOW", g.useText)
	}
	g.cycleDetailLevel()
	if g.autoDetailEnabled {
		t.Fatal("faithful detail cycle should not return to auto detail")
	}
	if g.viewW != 320 || g.viewH != 200 || g.detailLevel != 0 {
		t.Fatalf("after 3 cycles got %dx%d level=%d", g.viewW, g.viewH, g.detailLevel)
	}
	if g.useText != "Detail: HIGH" {
		t.Fatalf("after 3 cycles useText=%q want Detail: HIGH", g.useText)
	}
}

func TestCycleSourcePortDetailLevelIncludesAuto(t *testing.T) {
	g := &game{
		opts:               Options{SourcePortMode: true},
		detailLevel:        2,
		autoDetailEnabled:  true,
		hudMessagesEnabled: true,
	}
	g.cycleSourcePortDetailLevel()
	if g.autoDetailEnabled {
		t.Fatal("first source-port cycle should disable auto detail")
	}
	if g.useText != "Detail: 1/3x" {
		t.Fatalf("after 1 cycle useText=%q want Detail: 1/3x", g.useText)
	}
	g.cycleSourcePortDetailLevel()
	if g.detailLevel != 3 {
		t.Fatalf("after 2 cycles detail=%d want 3", g.detailLevel)
	}
	if g.useText != "Detail: 1/4x" {
		t.Fatalf("after 2 cycles useText=%q want Detail: 1/4x", g.useText)
	}
	g.cycleSourcePortDetailLevel()
	if g.detailLevel != 0 {
		t.Fatalf("after 3 cycles detail=%d want 0", g.detailLevel)
	}
	if g.autoDetailEnabled {
		t.Fatal("third source-port cycle should stay manual at 1x")
	}
	if g.useText != "Detail: 1x" {
		t.Fatalf("after 3 cycles useText=%q want Detail: 1x", g.useText)
	}
	g.cycleSourcePortDetailLevel()
	if !g.autoDetailEnabled {
		t.Fatal("fourth source-port cycle should return to auto detail")
	}
	if g.useText != "Detail: AUTO" {
		t.Fatalf("after 4 cycles useText=%q want Detail: AUTO", g.useText)
	}
}

func TestApplyAutoDetailSampleDropsDetailAfterSustainedLowFPS(t *testing.T) {
	const (
		samples  = 4
		lowFPS   = 54.0
		renderMS = 17.5
		wantNext = 2
	)
	g := &game{
		opts:               Options{SourcePortMode: true},
		mode:               viewWalk,
		detailLevel:        1,
		autoDetailEnabled:  true,
		hudMessagesEnabled: true,
	}
	g.applyAutoDetailSample(lowFPS, renderMS)
	if g.detailLevel != 1 {
		t.Fatalf("detail after first low sample=%d want 1", g.detailLevel)
	}
	g.applyAutoDetailSample(lowFPS, renderMS)
	g.applyAutoDetailSample(lowFPS, renderMS)
	if g.detailLevel != 1 {
		t.Fatalf("detail before fourth low sample=%d want 1", g.detailLevel)
	}
	g.applyAutoDetailSample(lowFPS, renderMS)
	if g.detailLevel != wantNext {
		t.Fatalf("detail after %dth low sample=%d want %d", samples, g.detailLevel, wantNext)
	}
	if g.autoDetailCooldown == 0 {
		t.Fatal("expected auto detail cooldown after change")
	}
	wantText := "Detail: AUTO DOWN -> " + g.detailLevelLabelFor(wantNext)
	if g.useText != wantText {
		t.Fatalf("useText=%q want %q", g.useText, wantText)
	}
}

func TestApplyAutoDetailSampleRaisesDetailAfterHeadroom(t *testing.T) {
	const (
		samples     = 4
		headroomFPS = 70.0
		renderMS    = 3.0
		wantNext    = 1
	)
	g := &game{
		opts:               Options{SourcePortMode: true},
		mode:               viewWalk,
		detailLevel:        2,
		autoDetailEnabled:  true,
		hudMessagesEnabled: true,
	}
	for i := 0; i < 4; i++ {
		g.applyAutoDetailSample(headroomFPS, renderMS)
	}
	if g.detailLevel != wantNext {
		t.Fatalf("detail after %d high-FPS samples=%d want %d", samples, g.detailLevel, wantNext)
	}
	wantText := "Detail: AUTO UP -> " + g.detailLevelLabelFor(wantNext)
	if g.useText != wantText {
		t.Fatalf("useText=%q want %q", g.useText, wantText)
	}
}

func TestApplyAutoDetailSampleRaisesDetailAtVsyncCappedFPS(t *testing.T) {
	const (
		samples  = 4
		vsyncFPS = 60.0
		renderMS = 3.0
		wantNext = 1
	)
	g := &game{
		opts:               Options{SourcePortMode: true},
		mode:               viewWalk,
		detailLevel:        2,
		autoDetailEnabled:  true,
		hudMessagesEnabled: true,
	}
	for i := 0; i < 4; i++ {
		g.applyAutoDetailSample(vsyncFPS, renderMS)
	}
	if g.detailLevel != wantNext {
		t.Fatalf("detail after %d vsync-capped samples=%d want %d", samples, g.detailLevel, wantNext)
	}
	wantText := "Detail: AUTO UP -> " + g.detailLevelLabelFor(wantNext)
	if g.useText != wantText {
		t.Fatalf("useText=%q want %q", g.useText, wantText)
	}
}

func TestApplyAutoDetailSampleDropsDetailAt50FPSEvenWithLowRenderMS(t *testing.T) {
	const (
		samples  = 4
		fps      = 50.0
		renderMS = 2.6
		wantNext = 3
	)
	g := &game{
		opts:               Options{SourcePortMode: true},
		mode:               viewWalk,
		detailLevel:        2,
		autoDetailEnabled:  true,
		hudMessagesEnabled: true,
	}
	for i := 0; i < 4; i++ {
		g.applyAutoDetailSample(fps, renderMS)
	}
	if g.detailLevel != wantNext {
		t.Fatalf("detail after %d low-FPS samples=%d want %d", samples, g.detailLevel, wantNext)
	}
	wantText := "Detail: AUTO DOWN -> " + g.detailLevelLabelFor(wantNext)
	if g.useText != wantText {
		t.Fatalf("useText=%q want %q", g.useText, wantText)
	}
}

func TestApplyAutoDetailSampleDropsFromHalfDetailAt50FPS(t *testing.T) {
	const (
		samples  = 4
		fps      = 50.0
		renderMS = 2.6
		wantNext = 2
	)
	g := &game{
		opts:               Options{SourcePortMode: true},
		mode:               viewWalk,
		detailLevel:        1,
		autoDetailEnabled:  true,
		hudMessagesEnabled: true,
	}
	for i := 0; i < 4; i++ {
		g.applyAutoDetailSample(fps, renderMS)
	}
	if g.detailLevel != wantNext {
		t.Fatalf("detail after %d low-FPS samples=%d want %d", samples, g.detailLevel, wantNext)
	}
	wantText := "Detail: AUTO DOWN -> " + g.detailLevelLabelFor(wantNext)
	if g.useText != wantText {
		t.Fatalf("useText=%q want %q", g.useText, wantText)
	}
}

func TestApplyAutoDetailSampleDoesNotRaiseWhenProjectedRenderExceedsBudget(t *testing.T) {
	const (
		samples  = 4
		fps      = 60.0
		renderMS = 6.5
	)
	g := &game{
		opts:               Options{SourcePortMode: true},
		mode:               viewWalk,
		detailLevel:        2,
		autoDetailEnabled:  true,
		hudMessagesEnabled: true,
	}
	for i := 0; i < 4; i++ {
		g.applyAutoDetailSample(fps, renderMS)
	}
	if g.detailLevel != 2 {
		t.Fatalf("detail after %d over-budget samples=%d want 2", samples, g.detailLevel)
	}
	if g.useText != "" {
		t.Fatalf("useText=%q want empty when no raise occurs", g.useText)
	}
}

func TestApplyAutoDetailSampleIgnoresMapView(t *testing.T) {
	g := &game{
		opts:               Options{SourcePortMode: true},
		mode:               viewMap,
		detailLevel:        1,
		autoDetailEnabled:  true,
		hudMessagesEnabled: true,
	}
	g.applyAutoDetailSample(54, 17.5)
	g.applyAutoDetailSample(54, 17.5)
	if g.detailLevel != 1 {
		t.Fatalf("detail changed in map view=%d want 1", g.detailLevel)
	}
	if g.useText != "" {
		t.Fatalf("useText=%q want empty in map view", g.useText)
	}
}

func TestRecordAutoDetailPerfSampleWaitsForFiveSecondWindow(t *testing.T) {
	g := &game{
		opts:               Options{SourcePortMode: true},
		mode:               viewWalk,
		detailLevel:        1,
		autoDetailEnabled:  true,
		hudMessagesEnabled: true,
	}
	start := time.Unix(0, 0)
	for i := 0; i < 5; i++ {
		g.recordAutoDetailPerfSample(start.Add(time.Duration(i)*time.Second), 54, 17.5)
	}
	if g.detailLevel != 1 {
		t.Fatalf("detail before five-second window elapsed=%d want 1", g.detailLevel)
	}
	if g.autoDetailLowSamples != 0 {
		t.Fatalf("autoDetailLowSamples before window=%d want 0", g.autoDetailLowSamples)
	}
}

func TestRecordAutoDetailPerfSampleUsesWorstValuesPerFiveSecondWindow(t *testing.T) {
	g := &game{
		opts:               Options{SourcePortMode: true},
		mode:               viewWalk,
		detailLevel:        1,
		autoDetailEnabled:  true,
		hudMessagesEnabled: true,
	}
	start := time.Unix(0, 0)
	for window := 0; window < 4; window++ {
		base := start.Add(time.Duration(window*6) * time.Second)
		g.recordAutoDetailPerfSample(base, 60, 8.0)
		g.recordAutoDetailPerfSample(base.Add(1*time.Second), 59, 8.5)
		g.recordAutoDetailPerfSample(base.Add(2*time.Second), 54, 17.5)
		g.recordAutoDetailPerfSample(base.Add(3*time.Second), 60, 8.0)
		g.recordAutoDetailPerfSample(base.Add(4*time.Second), 60, 8.0)
		g.recordAutoDetailPerfSample(base.Add(5*time.Second), 60, 8.0)
	}
	if g.detailLevel != 2 {
		t.Fatalf("detail after four five-second worst-case windows=%d want 2", g.detailLevel)
	}
	if g.useText != "Detail: AUTO DOWN -> 1/3x" {
		t.Fatalf("useText=%q want auto down message", g.useText)
	}
}
