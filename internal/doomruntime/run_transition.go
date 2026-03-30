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

func (sg *sessionGame) drawGamePresented(dst *ebiten.Image, g *game) {
	if dst == nil || g == nil {
		return
	}
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
		sg.transition.SetLastFrame(src)
		return
	}
	if sg.canDrawSourcePortDirect(dst, g) {
		dst.Fill(color.Black)
		if g.mode != viewMap {
			g.drawWalk3D(dst)
		} else {
			g.Draw(dst)
		}
		if g.mode != viewMap {
			g.drawWalkOverlays(dst)
		}
		sg.transition.SetLastFrame(dst)
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
	ow := max(dst.Bounds().Dx(), 1)
	oh := max(dst.Bounds().Dy(), 1)
	sg.drawSourcePortPresented(dst, src, ow, oh)
	if g.mode != viewMap {
		prevW, prevH := g.viewW, g.viewH
		g.viewW = ow
		g.viewH = oh
		g.drawWalkOverlays(dst)
		g.viewW = prevW
		g.viewH = prevH
	}
	sg.transition.SetLastFrame(src)
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
	op.GeoM.Scale(float64(sw)/float64(vw), float64(sh)/float64(vh))
	dst.DrawImage(src, op)
}

func (sg *sessionGame) drawBootSplashPresented(dst *ebiten.Image) {
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
		sg.bootSplashImage = newUnmanagedImage(sg.opts.BootSplash.Width, sg.opts.BootSplash.Height)
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
		t.EnsureReady(width, height, sg.opts.SourcePortMode, sourcePortMeltInitColumns(), sourcePortMeltMoveColumns(), func(dst *ebiten.Image) {
			if last := t.LastFrame(); last != nil {
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
	sg.transition.DrawFrame(screen, sw, sh)
}
