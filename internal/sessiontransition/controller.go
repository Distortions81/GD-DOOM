package sessiontransition

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

func newUnmanagedImage(w, h int) *ebiten.Image {
	return ebiten.NewImageWithOptions(image.Rect(0, 0, w, h), &ebiten.NewImageOptions{
		Unmanaged: true,
	})
}

const MeltVirtualH = 200

type Kind int

const (
	KindNone Kind = iota
	KindBoot
	KindLevel
)

type Controller struct {
	kind        Kind
	pending     bool
	initialized bool
	holdTics    int
	width       int
	height      int
	y           []int
	fromPix     []byte
	toPix       []byte
	workPix     []byte
	from        *ebiten.Image
	to          *ebiten.Image
	work        *ebiten.Image
	lastFrame   *ebiten.Image
}

func (c *Controller) Queue(kind Kind, holdTics int) {
	if kind == KindNone {
		c.Clear()
		return
	}
	c.kind = kind
	c.pending = true
	c.initialized = false
	if holdTics < 0 {
		holdTics = 0
	}
	c.holdTics = holdTics
	c.y = nil
}

func (c *Controller) Clear() {
	*c = Controller{}
}

func (c *Controller) Active() bool {
	return c != nil && c.kind != KindNone
}

func (c *Controller) Kind() Kind {
	if c == nil {
		return KindNone
	}
	return c.kind
}

func (c *Controller) HoldTics() int {
	if c == nil {
		return 0
	}
	return c.holdTics
}

func (c *Controller) SkipHold() {
	if c != nil && c.holdTics > 0 {
		c.holdTics = 0
	}
}

func (c *Controller) Initialized() bool {
	return c != nil && c.initialized
}

func (c *Controller) NeedsResize(w, h int) bool {
	return c != nil && c.initialized && (c.width != w || c.height != h)
}

func (c *Controller) Invalidate() {
	if c == nil {
		return
	}
	c.initialized = false
	c.pending = true
	c.y = nil
}

func (c *Controller) LastFrame() *ebiten.Image {
	if c == nil {
		return nil
	}
	return c.lastFrame
}

func (c *Controller) CaptureLastFrame(src *ebiten.Image) {
	if c == nil || src == nil {
		return
	}
	w := src.Bounds().Dx()
	h := src.Bounds().Dy()
	if w <= 0 || h <= 0 {
		return
	}
	if c.lastFrame == nil || c.lastFrame.Bounds().Dx() != w || c.lastFrame.Bounds().Dy() != h {
		c.lastFrame = newUnmanagedImage(w, h)
	}
	c.lastFrame.Clear()
	c.lastFrame.DrawImage(src, nil)
}

func (c *Controller) EnsureReady(width, height int, sourcePort bool, initCols, moveCols int, drawFrom func(*ebiten.Image), drawTo func(*ebiten.Image)) {
	if c == nil || c.kind == KindNone || c.initialized || !c.pending {
		return
	}
	if width <= 0 || height <= 0 {
		return
	}
	if c.from == nil || c.from.Bounds().Dx() != width || c.from.Bounds().Dy() != height {
		c.from = newUnmanagedImage(width, height)
	}
	if c.to == nil || c.to.Bounds().Dx() != width || c.to.Bounds().Dy() != height {
		c.to = newUnmanagedImage(width, height)
	}
	if c.work == nil || c.work.Bounds().Dx() != width || c.work.Bounds().Dy() != height {
		c.work = newUnmanagedImage(width, height)
	}
	if drawFrom != nil {
		drawFrom(c.from)
	}
	if drawTo != nil {
		drawTo(c.to)
	}
	need := width * height * 4
	if len(c.fromPix) != need {
		c.fromPix = make([]byte, need)
	}
	if len(c.toPix) != need {
		c.toPix = make([]byte, need)
	}
	if len(c.workPix) != need {
		c.workPix = make([]byte, need)
	}
	c.from.ReadPixels(c.fromPix)
	c.to.ReadPixels(c.toPix)
	copy(c.workPix, c.fromPix)
	c.work.WritePixels(c.workPix)
	c.width = width
	c.height = height
	c.initialized = true
	c.pending = false
	if c.holdTics <= 0 {
		if sourcePort {
			c.y = initMeltColumnsScaled(initCols, sourcePortMeltRNGScale(c.height))
		} else {
			c.y = initMeltColumns(width)
		}
	}
}

func (c *Controller) Tick(sourcePort bool, initCols, moveCols int) {
	if c == nil || c.kind == KindNone || !c.initialized {
		return
	}
	if c.holdTics > 0 {
		c.holdTics--
		if c.holdTics == 0 {
			if sourcePort {
				c.y = initMeltColumnsScaled(initCols, sourcePortMeltRNGScale(c.height))
			} else {
				c.y = initMeltColumns(c.width)
			}
		}
		return
	}
	if len(c.y) == 0 {
		if sourcePort {
			c.y = initMeltColumnsScaled(initCols, sourcePortMeltRNGScale(c.height))
		} else {
			c.y = initMeltColumns(c.width)
		}
	}
	done := false
	if sourcePort {
		done = stepMeltSlicesVirtual(c.y, MeltVirtualH, c.width, c.height, c.fromPix, c.toPix, c.workPix, 1, moveCols)
	} else {
		done = stepMeltColumns(c.y, c.width, c.height, c.fromPix, c.toPix, c.workPix, 1)
	}
	if done {
		c.work.WritePixels(c.toPix)
		c.CaptureLastFrame(c.to)
		c.Clear()
		return
	}
	c.work.WritePixels(c.workPix)
}

func (c *Controller) DrawFrame(screen *ebiten.Image, sw, sh int) {
	if c == nil || c.work == nil {
		screen.Fill(color.Black)
		return
	}
	tw := max(c.width, 1)
	th := max(c.height, 1)
	if tw == sw && th == sh {
		screen.DrawImage(c.work, nil)
		return
	}
	screen.Fill(color.Black)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(sw)/float64(tw), float64(sh)/float64(th))
	screen.DrawImage(c.work, op)
}

func sourcePortMeltRNGScale(height int) int {
	scale := height / MeltVirtualH
	if scale < 1 {
		return 1
	}
	return scale
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
