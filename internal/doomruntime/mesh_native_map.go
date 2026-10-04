package doomruntime

import (
	"fmt"
	"image/color"
	"math"

	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/mapview"
	"gddoom/internal/render/mapview/presenter"
)

type NativeMapInput struct {
	mapview.InputState
	ToggleGrid, ToggleRotate, ToggleReveal, CycleIDDT, CycleThings, ToggleLegend bool
}

type NativeMapLabel struct {
	X, Y float64
	Text string
}

// NativeMapFrame borrows buffers until the next MapFrame call. Drawing does not
// discover sectors or change simulation; mapped flags come from gameplay.
type NativeMapFrame struct {
	Width, Height               int
	Pixels                      []byte
	Lines                       []mapview.CachedLine
	Segments                    []mapview.Segment
	Patches                     []levelmesh.Patch
	Labels                      []NativeMapLabel
	GridSegments, ActorSegments int
	Legend                      bool
	ThingMode                   string
	LegendColors                presenter.LegendColors
}

func (n *NativeMeshGame) SetMapActive(active bool) {
	if active {
		n.g.mode = viewMap
	} else {
		n.g.mode = viewWalk
	}
	if n.g.State.FollowMode {
		n.g.State.SetCamera(float64(n.g.p.x)/fracUnit, float64(n.g.p.y)/fracUnit)
	}
	n.g.State.SyncRender()
}

// MapViewport preserves the relative zoom through window resizes.
func (n *NativeMeshGame) MapViewport(width, height int) {
	g := n.g
	width, height = max(1, width), max(1, height)
	if g.viewW == width && g.viewH == height {
		return
	}
	g.viewW, g.viewH = width, height
	_, _, ww, wh := boundsViewMetrics(g.bounds)
	g.State.Refit(ww, wh, width, height, doomInitialZoomMul)
	g.State.SyncRender()
}

func (n *NativeMeshGame) updateMap(in NativeMapInput) mapview.UpdateResult {
	g := n.g
	view := g.State.Snapshot()
	result := mapview.Update(in.InputState, mapview.UpdateState{EdgeInputPass: true, IsSourcePort: true, FollowMode: view.FollowEnabled(), Zoom: view.ZoomLevel(), FitZoom: view.FitZoomLevel()})
	g.State.SetZoom(result.Zoom)
	if result.ToggleFollow {
		g.State.ToggleFollowMode()
		n.Notify(fmt.Sprintf("Follow %t", g.State.FollowMode))
	}
	if result.ToggleBigMap {
		g.toggleBigMap()
	}
	if result.ResetView {
		g.resetView()
	}
	if result.AddMark {
		g.addMark()
	}
	if result.ClearMarks {
		g.clearMarks()
	}
	n.ApplyMapPresentation(in)
	// Big-map/reset actions may have changed follow state after Update computed it.
	result.SyncCameraToPlayer = g.State.FollowMode
	if result.SyncCameraToPlayer {
		result.PanDX, result.PanDY = 0, 0
	}
	return result
}

// These presentation controls work in both the map and the 3D view. They do
// not move the map camera or advance simulation when used outside map view.
func (n *NativeMeshGame) ApplyMapPresentation(in NativeMapInput) {
	g := n.g
	onOff := func(label string, enabled bool) {
		if enabled {
			n.Notify(label + " ON")
		} else {
			n.Notify(label + " OFF")
		}
	}
	if in.ToggleGrid {
		g.showGrid = !g.showGrid
		onOff("Grid", g.showGrid)
	}
	if in.ToggleRotate {
		g.rotateView = !g.rotateView
		onOff("Heading-Up", g.rotateView)
	}
	if in.ToggleReveal {
		if g.parity.reveal == revealNormal {
			g.parity.reveal = revealAllMap
		} else {
			g.parity.reveal = revealNormal
		}
		onOff("Allmap", g.parity.reveal == revealAllMap)
	}
	if in.CycleIDDT {
		g.parity.iddt = (g.parity.iddt + 1) % 3
		n.Notify(fmt.Sprintf("IDDT %d", g.parity.iddt))
	}
	if in.CycleThings {
		g.opts.SourcePortThingRenderMode = cycleSourcePortThingRenderMode(g.opts.SourcePortThingRenderMode)
		n.Notify("Thing Render: " + sourcePortThingRenderModeLabel(g.opts.SourcePortThingRenderMode))
	}
	if in.ToggleLegend {
		g.showLegend = !g.showLegend
		onOff("Thing Legend", g.showLegend)
	}
}

func (n *NativeMeshGame) MapFrame(width, height int) NativeMapFrame {
	g := n.g
	n.MapViewport(width, height)
	g.State.PrepareRender(g.renderAlpha)
	size := g.viewW * g.viewH * 4
	if len(n.mapPixels) != size {
		n.mapPixels = make([]byte, size)
	} else {
		clear(n.mapPixels)
	}
	if len(g.opts.FlatBank) > 0 {
		mapview.RasterizeFloor2D(n.mapPixels, g.mapFloorRasterInput())
	}
	key := g.mapLineStateKey()
	if g.mapLines.NeedsRebuild(key) {
		g.rebuildMapLineCache(key)
	}
	n.mapSegments = n.mapSegments[:0]
	n.mapPatches = n.mapPatches[:0]
	n.mapLabels = n.mapLabels[:0]
	line := func(ax, ay, bx, by float64, width float32, c color.Color) {
		n.mapSegments = append(n.mapSegments, mapview.Segment{X1: ax, Y1: ay, X2: bx, Y2: by, Width: width, Color: c})
	}
	worldLine := func(ax, ay, bx, by float64, width float32, c color.Color) {
		ax, ay = g.worldToScreen(ax, ay)
		bx, by = g.worldToScreen(bx, by)
		line(ax, ay, bx, by, width, c)
	}
	if g.showGrid {
		b := g.screenWorldBBox()
		c := color.RGBA{40, 50, 60, 255}
		for x := math.Floor(b.minX/128) * 128; x <= b.maxX; x += 128 {
			worldLine(x, b.minY, x, b.maxY, 1, c)
		}
		for y := math.Floor(b.minY/128) * 128; y <= b.maxY; y += 128 {
			worldLine(b.minX, y, b.maxX, y, 1, c)
		}
	}
	gridSegments := len(n.mapSegments)
	for _, li := range g.visibleLineIndices() {
		if li >= len(g.lineSpecial) || !buttonHighlightEligible(g.lineSpecial[li]) || !g.linedefDecision(g.m.Linedefs[li]).Visible {
			continue
		}
		pi := g.physForLine[li]
		if pi < 0 || pi >= len(g.lines) {
			continue
		}
		p := g.lines[pi]
		worldLine(float64(p.x1)/fracUnit, float64(p.y1)/fracUnit, float64(p.x2)/fracUnit, float64(p.y2)/fracUnit, 2.4, wallUseSpecial)
	}
	if li, tr := g.peekUseTargetLine(); tr == useTraceSpecial && li >= 0 && li < len(g.physForLine) {
		pi := g.physForLine[li]
		if pi >= 0 && pi < len(g.lines) {
			p := g.lines[pi]
			worldLine(float64(p.x1)/fracUnit, float64(p.y1)/fracUnit, float64(p.x2)/fracUnit, float64(p.y2)/fracUnit, 3, useTargetColor)
		}
	}
	zoom := g.State.Zoom
	for i, th := range g.m.Things {
		if g.thingCollected[i] || !g.automapThingRevealed(i, th) {
			continue
		}
		x, y := g.thingPosFixed(i, th)
		sx, sy := g.worldToScreen(float64(x)/fracUnit, float64(y)/fracUnit)
		if g.shouldDrawMapThingSprite(th) {
			if tex, ok := g.monsterSpriteTexture(g.mapThingSpriteName(i, th)); ok && tex.Width > 0 && tex.Height > 0 {
				target := math.Max(6, presenter.ThingGlyphSize(zoom)*2.4)
				scale := math.Min(target/float64(tex.Width), target/float64(tex.Height))
				w, h := float64(tex.Width)*scale, float64(tex.Height)*scale
				n.mapPatches = append(n.mapPatches, levelmesh.Patch{Texture: nativeTexture(tex), X: sx - w/2, Y: sy - h/2, W: w, H: h})
				continue
			}
		}
		ang := presenter.WorldAngleToGlyphAngle(g.thingWorldAngle(i, th))
		if g.mapRotationActive() {
			ang = presenter.RelativeWorldAngle(g.thingWorldAngle(i, th), g.renderAngle)
		}
		style := presenter.StyleForThingType(th.Type, isPlayerStart(th.Type), isMonster(th.Type))
		n.mapSegments = presenter.AppendThingGlyph(n.mapSegments, style, sx, sy, ang, presenter.ThingGlyphSize(zoom))
	}
	actorSegments := len(n.mapSegments)
	for _, mark := range g.marks.Items() {
		x, y := g.worldToScreen(mark.X, mark.Y)
		line(x-5, y-5, x+5, y+5, 1.3, color.RGBA{120, 210, 255, 255})
		line(x-5, y+5, x+5, y-5, 1.3, color.RGBA{120, 210, 255, 255})
		n.mapLabels = append(n.mapLabels, NativeMapLabel{X: x + 6, Y: y - 6, Text: fmt.Sprint(mark.ID)})
	}
	arrow := func(x, y, angle float64, c color.Color) {
		ca, sa := math.Cos(angle), math.Sin(angle)
		for _, s := range doomPlayerArrow {
			worldLine(x+s[0]*ca-s[1]*sa, y+s[0]*sa+s[1]*ca, x+s[2]*ca-s[3]*sa, y+s[2]*sa+s[3]*ca, 2, c)
		}
	}
	arrow(g.renderPX, g.renderPY, angleToRadians(g.renderAngle), playerColor)
	for _, p := range g.peerStarts {
		arrow(float64(p.x)/fracUnit, float64(p.y)/fracUnit, angleToRadians(p.angle), otherPlayerColor)
	}
	return NativeMapFrame{Width: g.viewW, Height: g.viewH, Pixels: n.mapPixels, Lines: g.mapLines.Items(), Segments: n.mapSegments, Patches: n.mapPatches, Labels: n.mapLabels, ActorSegments: actorSegments, GridSegments: gridSegments, Legend: g.showLegend, ThingMode: sourcePortThingRenderModeLabel(g.opts.SourcePortThingRenderMode), LegendColors: presenter.LegendColors{ThingPlayer: presenter.ThingPlayerColor, ThingMonster: presenter.ThingMonsterColor, ThingItem: presenter.ThingItemColor, ThingKey: presenter.ThingKeyBlue, ThingMisc: presenter.ThingMiscColor, WallOneSided: wallOneSided, WallFloor: wallFloorChange, WallCeil: wallCeilChange, WallTeleport: wallTeleporter, WallUse: wallUseSpecial, WallHidden: wallUnrevealed}}
}
