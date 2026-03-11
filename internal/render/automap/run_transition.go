package automap

import (
	"image/color"

	"gddoom/internal/sessiontransition"

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
		sg.transition.CaptureLastFrame(src)
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
	sg.transition.Queue(kind, holdTics)
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
