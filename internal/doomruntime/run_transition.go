package doomruntime

import (
	"image"
	"image/color"
	"time"

	"gddoom/internal/sessiontransition"

	"github.com/hajimehoshi/ebiten/v2"
)

func newUnmanagedImage(w, h int) *ebiten.Image {
	return newDebugImageWithOptions("transition:unmanaged", image.Rect(0, 0, w, h), &ebiten.NewImageOptions{
		Unmanaged: true,
	})
}

func (sg *sessionGame) ensureGameplaySurface(width, height int) *ebiten.Image {
	if sg == nil {
		return nil
	}
	width = max(width, 1)
	height = max(height, 1)
	if sg.gameplaySurface == nil || sg.gameplaySurface.Bounds().Dx() != width || sg.gameplaySurface.Bounds().Dy() != height {
		sg.gameplaySurface = newUnmanagedImage(width, height)
	}
	return sg.gameplaySurface
}

func (sg *sessionGame) ensureFrontendSurface(width, height int) *ebiten.Image {
	if sg == nil {
		return nil
	}
	width = max(width, 1)
	height = max(height, 1)
	if sg.frontendSurface == nil || sg.frontendSurface.Bounds().Dx() != width || sg.frontendSurface.Bounds().Dy() != height {
		sg.frontendSurface = newUnmanagedImage(width, height)
	}
	return sg.frontendSurface
}

func (sg *sessionGame) ensureTransitionCaptureSurface(width, height int) *ebiten.Image {
	if sg == nil {
		return nil
	}
	width = max(width, 1)
	height = max(height, 1)
	if sg.transitionCaptureSurface == nil || sg.transitionCaptureSurface.Bounds().Dx() != width || sg.transitionCaptureSurface.Bounds().Dy() != height {
		sg.transitionCaptureSurface = newUnmanagedImage(width, height)
	}
	return sg.transitionCaptureSurface
}

func (sg *sessionGame) drawGamePresented(dst *ebiten.Image, g *game) {
	if dst == nil || g == nil {
		return
	}
	dw := max(dst.Bounds().Dx(), 1)
	dh := max(dst.Bounds().Dy(), 1)
	var drawStart time.Time
	if g.mode != viewMap {
		drawStart = time.Now()
		g.renderStamp = drawStart
		if g.opts.DemoScript != nil {
			g.demoBenchDraws++
		}
		g.frameUpload = 0
		g.perfInDraw = true
		defer func() { g.perfInDraw = false }()
		defer g.finishPerfCounter(drawStart)
	}
	if !sg.opts.SourcePortMode {
		vw := max(g.viewW, 1)
		vh := max(g.viewH, 1)
		if sg.faithfulSurface == nil || sg.faithfulSurface.Bounds().Dx() != vw || sg.faithfulSurface.Bounds().Dy() != vh {
			sg.faithfulSurface = newUnmanagedImage(vw, vh)
		}
		sg.faithfulSurface.Fill(color.Black)
		if g.mode != viewMap {
			g.drawWalk3D(sg.faithfulSurface)
			g.drawWalkOverlays(sg.faithfulSurface)
		} else {
			g.Draw(sg.faithfulSurface)
		}
		src := sg.faithfulSurface
		if sg.palettePostEnabled() {
			src = sg.applyFaithfulPalettePost(sg.faithfulSurface)
		}
		sg.drawFaithfulPresented(dst, src)
		if sg.transitionActive() {
			sg.transition.CaptureLastFrame(src)
		}
		return
	}
	if sg.canDrawSourcePortDirect(dst, g) {
		dst.Fill(color.Black)
		if g.mode != viewMap {
			g.drawWalk3D(dst)
		} else {
			g.Draw(dst)
		}
		if sg.transitionActive() {
			sg.transition.CaptureLastFrame(dst)
		}
		if g.mode != viewMap {
			g.drawWalkOverlays(dst)
		}
		return
	}
	present := sg.ensureGameplaySurface(g.viewW, g.viewH)
	present.Fill(color.Black)
	if g.mode != viewMap {
		g.drawWalk3D(present)
	} else {
		g.Draw(present)
	}
	src := present
	if sg.palettePostEnabled() {
		src = sg.applyFaithfulPalettePost(present)
	}
	sg.drawSourcePortPresented(dst, src, dw, dh)
	if sg.transitionActive() {
		capture := sg.ensureTransitionCaptureSurface(dw, dh)
		capture.Clear()
		sg.drawSourcePortPresented(capture, src, dw, dh)
		sg.transition.SetLastFrame(capture)
	}
	if g.mode != viewMap {
		prevW, prevH := g.viewW, g.viewH
		g.viewW = dw
		g.viewH = dh
		g.drawWalkOverlays(dst)
		g.viewW = prevW
		g.viewH = prevH
	}
}

func (sg *sessionGame) canDrawSourcePortDirect(dst *ebiten.Image, g *game) bool {
	if sg == nil || dst == nil || g == nil {
		return false
	}
	if !sg.opts.SourcePortMode {
		return false
	}
	if sg.palettePostEnabled() {
		return false
	}
	return max(dst.Bounds().Dx(), 1) == max(g.viewW, 1) &&
		max(dst.Bounds().Dy(), 1) == max(g.viewH, 1)
}

func (sg *sessionGame) drawSourcePortPresented(dst, src *ebiten.Image, sw, sh int) {
	if dst == nil || src == nil {
		return
	}
	if dst == src {
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
	dst.Clear()
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	rw, rh, ox, oy := faithfulPresentationRect(sw, sh, sg != nil && sg.opts.DisableAspectCorrection)
	op.GeoM.Scale(float64(rw)/float64(vw), float64(rh)/float64(vh))
	op.GeoM.Translate(float64(ox), float64(oy))
	dst.DrawImage(src, op)
}

func faithfulPresentationRect(sw, sh int, disableAspectCorrection bool) (rw, rh, ox, oy int) {
	aspectH := faithfulAspectLogicalH
	if disableAspectCorrection {
		aspectH = doomLogicalH
	}
	return fitRect(sw, sh, doomLogicalW, aspectH)
}

func (sg *sessionGame) drawBootSplashPresented(dst *ebiten.Image, capture bool) {
	if dst == nil {
		return
	}
	if sg.bootSplashImage == nil && sg.opts.BootSplash.Width > 0 && sg.opts.BootSplash.Height > 0 &&
		len(sg.opts.BootSplash.RGBA) == sg.opts.BootSplash.Width*sg.opts.BootSplash.Height*4 {
		sg.bootSplashImage = newUnmanagedImage(sg.opts.BootSplash.Width, sg.opts.BootSplash.Height)
		sg.bootSplashImage.WritePixels(sg.opts.BootSplash.RGBA)
	}
	if sg.bootSplashImage == nil {
		dst.Fill(color.Black)
		return
	}
	if !sg.opts.SourcePortMode {
		if capture {
			dst.Fill(color.Black)
			bw := max(sg.bootSplashImage.Bounds().Dx(), 1)
			bh := max(sg.bootSplashImage.Bounds().Dy(), 1)
			op := &ebiten.DrawImageOptions{}
			op.Filter = ebiten.FilterNearest
			op.GeoM.Scale(float64(max(dst.Bounds().Dx(), 1))/float64(bw), float64(max(dst.Bounds().Dy(), 1))/float64(bh))
			dst.DrawImage(sg.bootSplashImage, op)
			return
		}
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
		present := sg.ensureGameplaySurface(g.viewW, g.viewH)
		g.Draw(present)
		src := present
		if sg.palettePostEnabled() {
			src = sg.applyFaithfulPalettePost(present)
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
		sg.faithfulSurface = newUnmanagedImage(vw, vh)
	}
	sg.faithfulSurface.Fill(color.Black)
	if g.mode != viewMap {
		g.drawWalk3D(sg.faithfulSurface)
		g.drawWalkOverlays(sg.faithfulSurface)
	} else {
		g.Draw(sg.faithfulSurface)
	}
	src := sg.faithfulSurface
	if sg.palettePostEnabled() {
		src = sg.applyFaithfulPalettePost(sg.faithfulSurface)
	}
	dw := max(dst.Bounds().Dx(), 1)
	dh := max(dst.Bounds().Dy(), 1)
	sw := max(src.Bounds().Dx(), 1)
	sh := max(src.Bounds().Dy(), 1)
	dst.Clear()
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(dw)/float64(sw), float64(dh)/float64(sh))
	dst.DrawImage(src, op)
}

func (sg *sessionGame) drawBootSplashTransitionSurface(dst *ebiten.Image) {
	if dst == nil {
		return
	}
	if sg.bootSplashImage == nil && sg.opts.BootSplash.Width > 0 && sg.opts.BootSplash.Height > 0 &&
		len(sg.opts.BootSplash.RGBA) == sg.opts.BootSplash.Width*sg.opts.BootSplash.Height*4 {
		sg.bootSplashImage = newUnmanagedImage(sg.opts.BootSplash.Width, sg.opts.BootSplash.Height)
		sg.bootSplashImage.WritePixels(sg.opts.BootSplash.RGBA)
	}
	if sg.bootSplashImage == nil {
		dst.Fill(color.Black)
		return
	}
	if !sg.opts.SourcePortMode {
		dst.Fill(color.Black)
		bw := max(sg.bootSplashImage.Bounds().Dx(), 1)
		bh := max(sg.bootSplashImage.Bounds().Dy(), 1)
		op := &ebiten.DrawImageOptions{}
		op.Filter = ebiten.FilterNearest
		op.GeoM.Scale(float64(max(dst.Bounds().Dx(), 1))/float64(bw), float64(max(dst.Bounds().Dy(), 1))/float64(bh))
		dst.DrawImage(sg.bootSplashImage, op)
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
	if sg == nil {
		return
	}
	if sg.opts.DemoScript != nil || (sg.g != nil && sg.g.sessionSignals().DemoActive) {
		sg.transition.Clear()
		return
	}
	sg.transition.Queue(kind, holdTics)
}

func (sg *sessionGame) shouldShowBootSplash() bool {
	if sg.opts.DemoScript != nil {
		return false
	}
	if sg.opts.LiveTicSource != nil || sg.opts.LiveTicSink != nil {
		return false
	}
	if sg.shouldStartInFrontend() && !(sg.opts.OpenMenuOnFrontendStart && len(sg.opts.AttractDemos) == 0) {
		return false
	}
	return sg.opts.BootSplash.Width > 0 &&
		sg.opts.BootSplash.Height > 0 &&
		len(sg.opts.BootSplash.RGBA) == sg.opts.BootSplash.Width*sg.opts.BootSplash.Height*4
}

func (sg *sessionGame) transitionActive() bool {
	if sg == nil {
		return false
	}
	if sg.opts.DemoScript != nil || (sg.g != nil && sg.g.sessionSignals().DemoActive) {
		return false
	}
	return sg.transition.Active()
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
	switch t.Kind() {
	case sessiontransition.KindBoot:
		t.EnsureReady(width, height, sg.opts.SourcePortMode, sourcePortMeltInitColumns(), sourcePortMeltMoveColumns(), func(dst *ebiten.Image) {
			sg.drawBootSplashTransitionSurface(dst)
		}, func(dst *ebiten.Image) {
			if sg.frontend.Active {
				sg.drawFrontendTransitionSurface(dst)
			} else {
				sg.drawGameTransitionSurface(dst, sg.g)
			}
		})
	case sessiontransition.KindLevel:
		last := t.LastFrame()
		var fromSrc *ebiten.Image
		if last != nil && last.Bounds().Dx() == width && last.Bounds().Dy() == height {
			fromSrc = last
		}
		t.EnsureReadyWithFrames(width, height, sg.opts.SourcePortMode, sourcePortMeltInitColumns(), sourcePortMeltMoveColumns(), fromSrc, nil, func(dst *ebiten.Image) {
			if last != nil {
				dst.Clear()
				op := &ebiten.DrawImageOptions{}
				lw := max(last.Bounds().Dx(), 1)
				lh := max(last.Bounds().Dy(), 1)
				op.Filter = ebiten.FilterNearest
				op.GeoM.Scale(float64(width)/float64(lw), float64(height)/float64(lh))
				dst.DrawImage(last, op)
			} else {
				sg.drawGameTransitionSurface(dst, sg.g)
			}
		}, func(dst *ebiten.Image) {
			sg.drawGameTransitionSurface(dst, sg.g)
		})
	default:
		sg.transition.Clear()
	}
}

func (sg *sessionGame) tickTransition() {
	sg.transition.Tick(sg.opts.SourcePortMode, sourcePortMeltInitColumns(), sourcePortMeltMoveColumns())
}

func sourcePortMeltInitColumns() int {
	return sourcePortMeltInitCols
}

func sourcePortMeltMoveColumns() int {
	return sourcePortMeltMoveCols
}

func (sg *sessionGame) drawTransitionFrame(screen *ebiten.Image, sw, sh int) {
	if screen == nil {
		return
	}
	if sg == nil {
		sg.transition.DrawFrame(screen, sw, sh)
		return
	}
	work := sg.transition.WorkFrame()
	if work == nil {
		sg.transition.DrawFrame(screen, sw, sh)
		return
	}
	screen.Fill(color.Black)
	if sg.opts.SourcePortMode {
		sg.drawSourcePortPresented(screen, work, sw, sh)
		return
	}
	sg.drawFaithfulPresented(screen, work)
}
