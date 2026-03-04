package doomtex

import (
	"encoding/binary"
	"fmt"
)

func (s *Set) BuildTextureRGBA(name string, palette int) ([]byte, int, int, error) {
	if s == nil {
		return nil, 0, 0, parseErrorf("nil texture set")
	}
	if palette < 0 || palette >= len(s.palettes) {
		return nil, 0, 0, parseErrorf("palette out of range: %d", palette)
	}
	t, ok := s.textures[normalizeName(name)]
	if !ok {
		return nil, 0, 0, parseErrorf("texture not found: %s", name)
	}
	pix := make([]byte, t.Width*t.Height)
	alpha := make([]bool, t.Width*t.Height)
	for _, ref := range t.Patches {
		p, err := s.loadPatch(ref.PatchName)
		if err != nil {
			// Keep going; Doom tolerates missing patches by leaving holes.
			continue
		}
		blitPatch(pix, alpha, t.Width, t.Height, p, ref.OriginX, ref.OriginY)
	}
	pal := s.palettes[palette]
	rgba := make([]byte, t.Width*t.Height*4)
	for i := 0; i < len(pix); i++ {
		o := i * 4
		if !alpha[i] {
			rgba[o+0] = 0
			rgba[o+1] = 0
			rgba[o+2] = 0
			rgba[o+3] = 0
			continue
		}
		idx := pix[i]
		rgba[o+0] = pal[idx][0]
		rgba[o+1] = pal[idx][1]
		rgba[o+2] = pal[idx][2]
		rgba[o+3] = 0xFF
	}
	return rgba, t.Width, t.Height, nil
}

// BuildPatchRGBA decodes a raw Doom patch lump (e.g. STBAR, STTNUM0) to RGBA.
// Returned offsets are Doom patch left/top offsets from the lump header.
func (s *Set) BuildPatchRGBA(name string, palette int) ([]byte, int, int, int, int, error) {
	if s == nil {
		return nil, 0, 0, 0, 0, parseErrorf("nil texture set")
	}
	if palette < 0 || palette >= len(s.palettes) {
		return nil, 0, 0, 0, 0, parseErrorf("palette out of range: %d", palette)
	}
	p, err := s.loadPatch(name)
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	pal := s.palettes[palette]
	rgba := make([]byte, p.width*p.height*4)
	for i := 0; i < len(p.index); i++ {
		o := i * 4
		if !p.opaque[i] {
			rgba[o+0] = 0
			rgba[o+1] = 0
			rgba[o+2] = 0
			rgba[o+3] = 0
			continue
		}
		idx := p.index[i]
		rgba[o+0] = pal[idx][0]
		rgba[o+1] = pal[idx][1]
		rgba[o+2] = pal[idx][2]
		rgba[o+3] = 0xFF
	}
	return rgba, p.width, p.height, p.leftOffset, p.topOffset, nil
}

func (s *Set) loadPatch(name string) (*decodedPatch, error) {
	key := normalizeName(name)
	if p, ok := s.patchCache[key]; ok {
		return p, nil
	}
	l, ok := s.patchByName[key]
	if !ok {
		return nil, parseErrorf("patch not found: %s", key)
	}
	data, err := s.wad.LumpData(l)
	if err != nil {
		return nil, err
	}
	p, err := decodePatch(data)
	if err != nil {
		return nil, fmt.Errorf("patch %s: %w", key, err)
	}
	s.patchCache[key] = p
	return p, nil
}

func decodePatch(data []byte) (*decodedPatch, error) {
	if len(data) < 8 {
		return nil, parseErrorf("patch too short")
	}
	w := int(int16(binary.LittleEndian.Uint16(data[0:2])))
	h := int(int16(binary.LittleEndian.Uint16(data[2:4])))
	leftOffset := int(int16(binary.LittleEndian.Uint16(data[4:6])))
	topOffset := int(int16(binary.LittleEndian.Uint16(data[6:8])))
	if w <= 0 || h <= 0 {
		return nil, parseErrorf("patch invalid size %dx%d", w, h)
	}
	header := 8 + w*4
	if len(data) < header {
		return nil, parseErrorf("patch column table truncated")
	}
	p := &decodedPatch{
		width:      w,
		height:     h,
		leftOffset: leftOffset,
		topOffset:  topOffset,
		index:      make([]uint8, w*h),
		opaque:     make([]bool, w*h),
	}
	for x := 0; x < w; x++ {
		co := int(binary.LittleEndian.Uint32(data[8+x*4 : 12+x*4]))
		if co < 0 || co >= len(data) {
			return nil, parseErrorf("patch column %d offset out of range", x)
		}
		for {
			if co >= len(data) {
				return nil, parseErrorf("patch column %d truncated", x)
			}
			top := data[co]
			co++
			if top == 0xFF {
				break
			}
			if co+2 > len(data) {
				return nil, parseErrorf("patch post header truncated")
			}
			length := int(data[co])
			co++
			co++ // unused byte
			if co+length+1 > len(data) {
				return nil, parseErrorf("patch post pixels truncated")
			}
			for i := 0; i < length; i++ {
				y := int(top) + i
				if y < 0 || y >= h {
					continue
				}
				idx := y*w + x
				p.index[idx] = data[co+i]
				p.opaque[idx] = true
			}
			co += length
			co++ // trailing unused byte
		}
	}
	return p, nil
}

func blitPatch(dst []byte, alpha []bool, dw, dh int, p *decodedPatch, ox, oy int) {
	for py := 0; py < p.height; py++ {
		dy := oy + py
		if dy < 0 || dy >= dh {
			continue
		}
		for px := 0; px < p.width; px++ {
			dx := ox + px
			if dx < 0 || dx >= dw {
				continue
			}
			si := py*p.width + px
			if !p.opaque[si] {
				continue
			}
			di := dy*dw + dx
			dst[di] = p.index[si]
			alpha[di] = true
		}
	}
}
