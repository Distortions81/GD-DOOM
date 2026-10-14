// Package menufont composes reusable Doom menu lettering into ordinary patch
// textures. Composition happens when assets load, not in the render loop.
package menufont

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"sync"

	"gddoom/internal/media"
)

// The atlas contains UZDoom's community-completed BIGUPPER face, assembled by
// Eevee. It is game artwork, not covered by the engine's source-code license.
// See assets/NOTICE.md and assets/UPSTREAM-LICENSE.md.
//
//go:embed assets/bigupper.png
var atlasPNG []byte

//go:embed assets/glyphs.json
var glyphJSON []byte

const (
	lineHeight   = 17
	spaceWidth   = 9
	kerning      = -1
	maxDimension = 4096
	maxPixels    = 4 << 20
)

type glyph struct {
	texture media.WallTexture
	x, y    int
}

type sourceFont struct {
	image  image.Image
	glyphs map[rune]sourceGlyph
}

type sourceGlyph struct {
	rect image.Rectangle
	x, y int
}

var source = sync.OnceValue(loadSource)

func loadSource() *sourceFont {
	img, err := png.Decode(bytes.NewReader(atlasPNG))
	if err != nil {
		panic(fmt.Errorf("menu font atlas: %w", err))
	}
	var metrics map[string]string
	if err := json.Unmarshal(glyphJSON, &metrics); err != nil {
		panic(fmt.Errorf("menu font metrics: %w", err))
	}
	out := &sourceFont{image: img, glyphs: make(map[rune]sourceGlyph, len(metrics))}
	for text, metric := range metrics {
		chars := []rune(text)
		geometry, offset, _ := strings.Cut(metric, "@")
		var w, h, x, y, dx, dy int
		if n, err := fmt.Sscanf(geometry, "%dx%d+%d+%d", &w, &h, &x, &y); err != nil || n != 4 || len(chars) != 1 || w <= 0 || h <= 0 {
			panic("invalid menu font glyph metric: " + metric)
		}
		if offset != "" {
			parts := strings.Split(offset, ",")
			if len(parts) != 2 {
				panic("invalid menu font glyph offset: " + metric)
			}
			var err error
			dx, err = strconv.Atoi(parts[0])
			if err != nil {
				panic(err)
			}
			dy, err = strconv.Atoi(parts[1])
			if err != nil {
				panic(err)
			}
		}
		rect := image.Rect(x, y, x+w, y+h)
		if !rect.In(img.Bounds()) {
			panic("menu font glyph outside atlas: " + metric)
		}
		out.glyphs[chars[0]] = sourceGlyph{rect: rect, x: dx, y: dy}
	}
	return out
}

// Font preserves the original face's 15-pixel capitals, 12-pixel small capitals
// (lowercase input), proportions, baseline offsets, and one-pixel kerning.
// It owns its pixel data and does not retain or modify the supplied patch bank.
type Font struct{ glyphs map[rune]glyph }

// New colors the face using matching N/M artwork from the loaded Doom WAD.
// A missing or differently shaped menu uses the standard Doom red ramp. Mods
// can supply an authored M_MULTI patch instead of using this generated face.
func New(patches map[string]media.WallTexture) *Font {
	src := source()
	ramp := makeRamp(src, patches)
	f := &Font{glyphs: make(map[rune]glyph, len(src.glyphs))}
	for ch, s := range src.glyphs {
		w, h := s.rect.Dx(), s.rect.Dy()
		pixels := make([]byte, w*h*4)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				p := color.NRGBAModel.Convert(src.image.At(s.rect.Min.X+x, s.rect.Min.Y+y)).(color.NRGBA)
				if p.A == 0 {
					continue
				}
				c := ramp[p.R]
				i := (y*w + x) * 4
				pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = c.R, c.G, c.B, p.A
			}
		}
		f.glyphs[ch] = glyph{texture: media.WallTexture{RGBA: pixels, Width: w, Height: h}, x: s.x, y: s.y}
	}
	return f
}

func makeRamp(src *sourceFont, patches map[string]media.WallTexture) [256]color.NRGBA {
	// Original Doom menu shades sampled from its red palette; these also cover
	// partially replaced menus where neither reference glyph remains intact.
	points := map[int]color.NRGBA{}
	for _, pair := range [][2]int{{0, 0}, {67, 67}, {71, 71}, {79, 79}, {91, 91}, {107, 103}, {119, 115}, {127, 127}, {139, 139}, {159, 155}, {167, 167}, {179, 179}, {191, 191}, {203, 203}, {219, 215}, {231, 227}, {239, 239}, {255, 255}} {
		points[pair[0]] = color.NRGBA{R: byte(pair[1]), A: 255}
	}
	sampled := map[int]color.NRGBA{}
	for _, ref := range []struct {
		name             string
		width, height, x int
		ch               rune
		overlappedEdge   bool
	}{
		{"M_NGAME", 110, 15, 0, 'N', false}, {"M_NEWG", 121, 15, 0, 'N', false},
		{"M_NEWG", 121, 15, 90, 'M', false}, {"M_NEWG", 121, 15, 30, 'W', true},
	} {
		p, ok := patches[ref.name]
		if !ok || p.Width != ref.width || p.Height != ref.height || len(p.RGBA) != p.Width*p.Height*4 {
			continue
		}
		s := src.glyphs[ref.ch]
		candidate := map[int]color.NRGBA{}
		matches := true
		for y := 0; y < s.rect.Dy() && matches; y++ {
			for x := 0; x < s.rect.Dx(); x++ {
				original := color.NRGBAModel.Convert(src.image.At(s.rect.Min.X+x, s.rect.Min.Y+y)).(color.NRGBA)
				i := (y*p.Width + ref.x + x) * 4
				// The preceding E overlaps W's first column at two pixels.
				if ref.overlappedEdge && x == 0 && original.A == 0 {
					continue
				}
				if (original.A == 0) != (p.RGBA[i+3] == 0) {
					matches = false
					break
				}
				if original.A == 0 {
					continue
				}
				value := color.NRGBA{R: p.RGBA[i], G: p.RGBA[i+1], B: p.RGBA[i+2], A: 255}
				if prev, ok := candidate[int(original.R)]; ok && prev != value {
					matches = false
					break
				}
				candidate[int(original.R)] = value
			}
		}
		// A replaced title may use a different palette from the main menu.
		// Keep the first matching palette instead of mixing unrelated colors.
		for shade, c := range candidate {
			if previous, ok := sampled[shade]; ok && previous != c {
				matches = false
				break
			}
		}
		if matches {
			for shade, c := range candidate {
				sampled[shade] = c
			}
		}
	}
	if len(sampled) > 1 {
		// Do not mix the default red samples into a recolored WAD's ramp.
		points = sampled
	}
	var ramp [256]color.NRGBA
	for shade := range ramp {
		lo, hi := -1, 256
		for s := range points {
			if s <= shade && s > lo {
				lo = s
			}
			if s >= shade && s < hi {
				hi = s
			}
		}
		if lo < 0 {
			ramp[shade] = points[hi]
			continue
		}
		if hi > 255 || lo == hi {
			ramp[shade] = points[lo]
			continue
		}
		a, b := points[lo], points[hi]
		lerp := func(x, y byte) byte { return byte((int(x)*(hi-shade) + int(y)*(shade-lo) + (hi-lo)/2) / (hi - lo)) }
		ramp[shade] = color.NRGBA{R: lerp(a.R, b.R), G: lerp(a.G, b.G), B: lerp(a.B, b.B), A: 255}
	}
	return ramp
}

// Measure returns the composed patch dimensions, including accents/descenders.
// Empty or excessively large labels return zero. Unsupported characters use ?.
func (f *Font) Measure(text string) (width, height int) {
	bounds, ok := f.layout(text, nil)
	if !ok {
		return 0, 0
	}
	return bounds.Dx(), bounds.Dy()
}

// Compose makes a transparent, nearest-neighbor-ready patch. It supports
// multiple lines and returns false for empty or excessively large labels.
// Preserve the returned patch offsets when drawing accented text.
func (f *Font) Compose(text string) (media.WallTexture, bool) {
	bounds, ok := f.layout(text, nil)
	if !ok {
		return media.WallTexture{}, false
	}
	out := media.WallTexture{Width: bounds.Dx(), Height: bounds.Dy(), OffsetX: -bounds.Min.X, OffsetY: -bounds.Min.Y}
	out.RGBA = make([]byte, out.Width*out.Height*4)
	f.layout(text, func(g glyph, x, y int) {
		x -= bounds.Min.X
		y -= bounds.Min.Y
		for gy := 0; gy < g.texture.Height; gy++ {
			for gx := 0; gx < g.texture.Width; gx++ {
				si := (gy*g.texture.Width + gx) * 4
				if g.texture.RGBA[si+3] == 0 {
					continue
				}
				di := ((y+gy)*out.Width + x + gx) * 4
				copy(out.RGBA[di:di+4], g.texture.RGBA[si:si+4])
			}
		}
	})
	out.EnsureOpaqueMask()
	// The optional renderer run tables encode coordinates in eight bits.
	// Longer generated labels can still be drawn directly from RGBA/mask.
	if out.Width <= 256 && out.Height <= 256 {
		out.EnsureOpaqueColumnBounds()
	}
	return out, true
}

func (f *Font) layout(text string, draw func(glyph, int, int)) (image.Rectangle, bool) {
	if f == nil || len(f.glyphs) == 0 || text == "" || len(text) > 8192 {
		return image.Rectangle{}, false
	}
	x, y := 0, 0
	bounds := image.Rect(0, 0, 0, 15)
	hasInk := false
	for _, ch := range text {
		switch ch {
		case '\n':
			x = 0
			y += lineHeight
		case ' ':
			x += spaceWidth + kerning
		default:
			g, ok := f.glyphs[ch]
			if !ok {
				g = f.glyphs['?']
			}
			rect := image.Rect(x+g.x, y+g.y, x+g.x+g.texture.Width, y+g.y+g.texture.Height)
			// image.Rectangle.Union drops an empty initial rectangle. Keep the cap
			// origin so a lowercase-only label still sits on the same baseline.
			bounds.Min.X = min(bounds.Min.X, rect.Min.X)
			bounds.Min.Y = min(bounds.Min.Y, rect.Min.Y)
			bounds.Max.X = max(bounds.Max.X, rect.Max.X)
			bounds.Max.Y = max(bounds.Max.Y, rect.Max.Y)
			if draw != nil {
				draw(g, rect.Min.X, rect.Min.Y)
			}
			x += g.texture.Width + kerning
			hasInk = true
		}
		if x > maxDimension || y > maxDimension || bounds.Dx() > maxDimension || bounds.Dy() > maxDimension || bounds.Dx()*bounds.Dy() > maxPixels {
			return image.Rectangle{}, false
		}
	}
	return bounds, hasInk
}
