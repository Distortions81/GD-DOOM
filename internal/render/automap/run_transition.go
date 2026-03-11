package automap

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

func (sg *sessionGame) drawGamePresented(dst *ebiten.Image, g *game) {
	if dst == nil || g == nil {
		return
	}
	if !sg.opts.SourcePortMode {
		vw := max(g.viewW, 1)
		vh := max(g.viewH, 1)
		if sg.faithfulSurface == nil || sg.faithfulSurface.Bounds().Dx() != vw || sg.faithfulSurface.Bounds().Dy() != vh {
			sg.faithfulSurface = ebiten.NewImage(vw, vh)
		}
		g.Draw(sg.faithfulSurface)
		src := sg.faithfulSurface
		if sg.palettePostEnabled() {
			src = sg.applyFaithfulPalettePost(sg.faithfulSurface)
		}
		sg.drawFaithfulPresented(dst, src)
		sg.captureLastFrame(src)
		return
	}
	if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != g.viewW || sg.presentSurface.Bounds().Dy() != g.viewH {
		sg.presentSurface = ebiten.NewImage(max(g.viewW, 1), max(g.viewH, 1))
	}
	g.Draw(sg.presentSurface)
	src := sg.presentSurface
	if sg.palettePostEnabled() {
		src = sg.applyFaithfulPalettePost(sg.presentSurface)
	}
	sg.drawSourcePortPresented(dst, src, max(dst.Bounds().Dx(), 1), max(dst.Bounds().Dy(), 1))
}

func (sg *sessionGame) drawSourcePortPresented(dst, src *ebiten.Image, sw, sh int) {
	if dst == nil || src == nil {
		return
	}
	vw := max(src.Bounds().Dx(), 1)
	vh := max(src.Bounds().Dy(), 1)
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(sw)/float64(vw), float64(sh)/float64(vh))
	dst.DrawImage(src, op)
}

func (sg *sessionGame) drawFaithfulPresented(dst, src *ebiten.Image) {
	if dst == nil || src == nil {
		return
	}
	sw := max(dst.Bounds().Dx(), 1)
	sh := max(dst.Bounds().Dy(), 1)
	vw := max(src.Bounds().Dx(), 1)
	vh := max(src.Bounds().Dy(), 1)
	targetH := faithfulAspectLogicalH
	if sg.opts.DisableAspectCorrection {
		targetH = doomLogicalH
	}
	scale := sw / doomLogicalW
	scaleY := sh / targetH
	if scaleY < scale {
		scale = scaleY
	}
	if scale < 1 {
		scale = 1
	}
	nearestW := doomLogicalW * scale
	nearestH := doomLogicalH * scale
	if sg.faithfulNearest == nil || sg.faithfulNearest.Bounds().Dx() != nearestW || sg.faithfulNearest.Bounds().Dy() != nearestH {
		sg.faithfulNearest = ebiten.NewImage(nearestW, nearestH)
	}
	sg.faithfulNearest.Clear()
	nearestOp := &ebiten.DrawImageOptions{}
	nearestOp.Filter = ebiten.FilterNearest
	nearestOp.GeoM.Scale(float64(nearestW)/float64(vw), float64(nearestH)/float64(vh))
	sg.faithfulNearest.DrawImage(src, nearestOp)

	dst.Clear()
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterLinear
	op.GeoM.Scale(float64(sw)/float64(nearestW), float64(sh)/float64(nearestH))
	dst.DrawImage(sg.faithfulNearest, op)
}

func (sg *sessionGame) drawBootSplashPresented(dst *ebiten.Image) {
	if dst == nil {
		return
	}
	if sg.bootSplashImage == nil && sg.opts.BootSplash.Width > 0 && sg.opts.BootSplash.Height > 0 &&
		len(sg.opts.BootSplash.RGBA) == sg.opts.BootSplash.Width*sg.opts.BootSplash.Height*4 {
		sg.bootSplashImage = ebiten.NewImage(sg.opts.BootSplash.Width, sg.opts.BootSplash.Height)
		sg.bootSplashImage.WritePixels(sg.opts.BootSplash.RGBA)
	}
	if sg.bootSplashImage == nil {
		dst.Fill(color.Black)
		return
	}
	if !sg.opts.SourcePortMode {
		sg.drawFaithfulPresented(dst, sg.bootSplashImage)
		return
	}
	sw := max(dst.Bounds().Dx(), 1)
	sh := max(dst.Bounds().Dy(), 1)
	bw := max(sg.bootSplashImage.Bounds().Dx(), 1)
	bh := max(sg.bootSplashImage.Bounds().Dy(), 1)
	dst.Fill(color.Black)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(sw)/float64(bw), float64(sh)/float64(bh))
	dst.DrawImage(sg.bootSplashImage, op)
}

func (sg *sessionGame) drawGameTransitionSurface(dst *ebiten.Image, g *game) {
	if dst == nil || g == nil {
		return
	}
	if sg.opts.SourcePortMode {
		if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != g.viewW || sg.presentSurface.Bounds().Dy() != g.viewH {
			sg.presentSurface = ebiten.NewImage(max(g.viewW, 1), max(g.viewH, 1))
		}
		g.Draw(sg.presentSurface)
		src := sg.presentSurface
		if sg.palettePostEnabled() {
			src = sg.applyFaithfulPalettePost(sg.presentSurface)
		}
		dw := max(dst.Bounds().Dx(), 1)
		dh := max(dst.Bounds().Dy(), 1)
		dst.Fill(color.Black)
		sg.drawSourcePortPresented(dst, src, dw, dh)
		return
	}
	vw := max(g.viewW, 1)
	vh := max(g.viewH, 1)
	if sg.faithfulSurface == nil || sg.faithfulSurface.Bounds().Dx() != vw || sg.faithfulSurface.Bounds().Dy() != vh {
		sg.faithfulSurface = ebiten.NewImage(vw, vh)
	}
	g.Draw(sg.faithfulSurface)
	src := sg.faithfulSurface
	if sg.palettePostEnabled() {
		src = sg.applyFaithfulPalettePost(sg.faithfulSurface)
	}
	dw := max(dst.Bounds().Dx(), 1)
	dh := max(dst.Bounds().Dy(), 1)
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(dw)/float64(vw), float64(dh)/float64(vh))
	dst.Fill(color.Black)
	dst.DrawImage(src, op)
}

func (sg *sessionGame) drawBootSplashTransitionSurface(dst *ebiten.Image) {
	if dst == nil {
		return
	}
	if sg.bootSplashImage == nil && sg.opts.BootSplash.Width > 0 && sg.opts.BootSplash.Height > 0 &&
		len(sg.opts.BootSplash.RGBA) == sg.opts.BootSplash.Width*sg.opts.BootSplash.Height*4 {
		sg.bootSplashImage = ebiten.NewImage(sg.opts.BootSplash.Width, sg.opts.BootSplash.Height)
		sg.bootSplashImage.WritePixels(sg.opts.BootSplash.RGBA)
	}
	if sg.bootSplashImage == nil {
		dst.Fill(color.Black)
		return
	}
	dw := max(dst.Bounds().Dx(), 1)
	dh := max(dst.Bounds().Dy(), 1)
	bw := max(sg.bootSplashImage.Bounds().Dx(), 1)
	bh := max(sg.bootSplashImage.Bounds().Dy(), 1)
	dst.Fill(color.Black)
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(dw)/float64(bw), float64(dh)/float64(bh))
	dst.DrawImage(sg.bootSplashImage, op)
}

func (sg *sessionGame) queueTransition(kind transitionKind, holdTics int) {
	if kind == transitionNone {
		sg.clearTransition()
		return
	}
	sg.transition.kind = kind
	sg.transition.pending = true
	sg.transition.initialized = false
	if holdTics < 0 {
		holdTics = 0
	}
	sg.transition.holdTics = holdTics
	sg.transition.y = nil
}

func (sg *sessionGame) shouldShowBootSplash() bool {
	if sg.opts.DemoScript != nil {
		return false
	}
	if sg.shouldStartInFrontend() {
		return false
	}
	return sg.opts.BootSplash.Width > 0 &&
		sg.opts.BootSplash.Height > 0 &&
		len(sg.opts.BootSplash.RGBA) == sg.opts.BootSplash.Width*sg.opts.BootSplash.Height*4
}

func (sg *sessionGame) transitionActive() bool {
	return sg.transition.kind != transitionNone
}

func (sg *sessionGame) transitionSurfaceSize(screenW, screenH int) (int, int) {
	if sg.opts.SourcePortMode {
		return max(screenW, 1), max(screenH, 1)
	}
	w := sg.opts.Width
	h := sg.opts.Height
	if w <= 0 {
		w = doomLogicalW
	}
	if h <= 0 {
		h = doomLogicalH
	}
	return w, h
}

func (sg *sessionGame) ensureTransitionReady(width, height int) {
	t := &sg.transition
	if t.kind == transitionNone || t.initialized || !t.pending {
		return
	}
	tw := width
	th := height
	if tw <= 0 || th <= 0 {
		return
	}
	if t.from == nil || t.from.Bounds().Dx() != tw || t.from.Bounds().Dy() != th {
		t.from = ebiten.NewImage(tw, th)
	}
	if t.to == nil || t.to.Bounds().Dx() != tw || t.to.Bounds().Dy() != th {
		t.to = ebiten.NewImage(tw, th)
	}
	if t.work == nil || t.work.Bounds().Dx() != tw || t.work.Bounds().Dy() != th {
		t.work = ebiten.NewImage(tw, th)
	}
	switch t.kind {
	case transitionBoot:
		sg.drawBootSplashTransitionSurface(t.from)
		if sg.frontend.active {
			sg.drawFrontendTransitionSurface(t.to)
		} else {
			sg.drawGameTransitionSurface(t.to, sg.g)
		}
	case transitionLevel:
		if sg.lastFrame != nil {
			t.from.Clear()
			op := &ebiten.DrawImageOptions{}
			lw := max(sg.lastFrame.Bounds().Dx(), 1)
			lh := max(sg.lastFrame.Bounds().Dy(), 1)
			op.Filter = ebiten.FilterNearest
			op.GeoM.Scale(float64(tw)/float64(lw), float64(th)/float64(lh))
			t.from.DrawImage(sg.lastFrame, op)
		} else {
			sg.drawGameTransitionSurface(t.from, sg.g)
		}
		sg.drawGameTransitionSurface(t.to, sg.g)
	default:
		sg.clearTransition()
		return
	}
	need := tw * th * 4
	if len(t.fromPix) != need {
		t.fromPix = make([]byte, need)
	}
	if len(t.toPix) != need {
		t.toPix = make([]byte, need)
	}
	if len(t.workPix) != need {
		t.workPix = make([]byte, need)
	}
	t.from.ReadPixels(t.fromPix)
	t.to.ReadPixels(t.toPix)
	copy(t.workPix, t.fromPix)
	t.work.WritePixels(t.workPix)
	t.width = tw
	t.height = th
	t.initialized = true
	t.pending = false
	if t.holdTics <= 0 {
		if sg.opts.SourcePortMode {
			t.y = initMeltColumnsScaled(sourcePortMeltInitColumns(), sourcePortMeltRNGScale(t.height))
		} else {
			t.y = initMeltColumns(tw)
		}
	}
}

func (sg *sessionGame) tickTransition() {
	t := &sg.transition
	if t.kind == transitionNone || !t.initialized {
		return
	}
	if t.holdTics > 0 {
		t.holdTics--
		if t.holdTics == 0 {
			if sg.opts.SourcePortMode {
				t.y = initMeltColumnsScaled(sourcePortMeltInitColumns(), sourcePortMeltRNGScale(t.height))
			} else {
				t.y = initMeltColumns(t.width)
			}
		}
		return
	}
	if len(t.y) == 0 {
		if sg.opts.SourcePortMode {
			t.y = initMeltColumnsScaled(sourcePortMeltInitColumns(), sourcePortMeltRNGScale(t.height))
		} else {
			t.y = initMeltColumns(t.width)
		}
	}
	meltTicks := 1
	done := false
	if sg.opts.SourcePortMode {
		done = stepMeltSlicesVirtual(t.y, meltVirtualH, t.width, t.height, t.fromPix, t.toPix, t.workPix, meltTicks, sourcePortMeltMoveColumns())
	} else {
		done = stepMeltColumns(t.y, t.width, t.height, t.fromPix, t.toPix, t.workPix, meltTicks)
	}
	if done {
		t.work.WritePixels(t.toPix)
		sg.captureLastFrame(t.to)
		sg.clearTransition()
		return
	}
	t.work.WritePixels(t.workPix)
}

func sourcePortMeltRNGScale(height int) int {
	scale := height / meltVirtualH
	if scale < 1 {
		return 1
	}
	return scale
}

func sourcePortMeltInitColumns() int {
	return sourcePortMeltInitCols
}

func sourcePortMeltMoveColumns() int {
	return sourcePortMeltMoveCols
}

func (sg *sessionGame) drawTransitionFrame(screen *ebiten.Image, sw, sh int) {
	t := &sg.transition
	if t.work == nil {
		screen.Fill(color.Black)
		return
	}
	tw := max(t.width, 1)
	th := max(t.height, 1)
	if tw == sw && th == sh {
		screen.DrawImage(t.work, nil)
		return
	}
	screen.Fill(color.Black)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(sw)/float64(tw), float64(sh)/float64(th))
	screen.DrawImage(t.work, op)
}

func (sg *sessionGame) captureLastFrame(src *ebiten.Image) {
	if src == nil {
		return
	}
	w := src.Bounds().Dx()
	h := src.Bounds().Dy()
	if w <= 0 || h <= 0 {
		return
	}
	if sg.lastFrame == nil || sg.lastFrame.Bounds().Dx() != w || sg.lastFrame.Bounds().Dy() != h {
		sg.lastFrame = ebiten.NewImage(w, h)
	}
	sg.lastFrame.Clear()
	sg.lastFrame.DrawImage(src, nil)
}

func (sg *sessionGame) clearTransition() {
	sg.transition = sessionTransition{}
}
