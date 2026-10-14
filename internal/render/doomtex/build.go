package doomtex

import (
	"encoding/binary"
	"fmt"
)

func (s *Set) BuildTextureIndexed(name string) ([]byte, int, int, error) {
	if s == nil {
		return nil, 0, 0, parseErrorf("nil texture set")
	}
	t, ok := s.textures[normalizeName(name)]
	if !ok {
		return nil, 0, 0, parseErrorf("texture not found: %s", name)
	}
	pix, _ := s.composeTexture(t)
	return pix, t.Width, t.Height, nil
}

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
	pix, alpha := s.composeTexture(t)
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

// BuildRawPicRGBA decodes a raw indexed picture lump (e.g. TITLEPIC) to RGBA.
// width/height must match the lump's raw pixel dimensions.
func (s *Set) BuildRawPicRGBA(name string, palette int, width, height int) ([]byte, int, int, error) {
	if s == nil {
		return nil, 0, 0, parseErrorf("nil texture set")
	}
	if palette < 0 || palette >= len(s.palettes) {
		return nil, 0, 0, parseErrorf("palette out of range: %d", palette)
	}
	if width <= 0 || height <= 0 {
		return nil, 0, 0, parseErrorf("invalid raw pic size %dx%d", width, height)
	}
	l, ok := s.wad.LumpByName(name)
	if !ok {
		return nil, 0, 0, parseErrorf("raw pic not found: %s", name)
	}
	data, err := s.wad.LumpDataView(l)
	if err != nil {
		return nil, 0, 0, err
	}
	need := width * height
	if len(data) != need {
		return nil, 0, 0, parseErrorf("raw pic %s size mismatch: got=%d want=%d", name, len(data), need)
	}
	pal := s.palettes[palette]
	rgba := make([]byte, need*4)
	for i := 0; i < need; i++ {
		o := i * 4
		idx := data[i]
		rgba[o+0] = pal[idx][0]
		rgba[o+1] = pal[idx][1]
		rgba[o+2] = pal[idx][2]
		rgba[o+3] = 0xFF
	}
	return rgba, width, height, nil
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

func (s *Set) BuildPatchIndexed(name string) ([]byte, []bool, int, int, int, int, error) {
	if s == nil {
		return nil, nil, 0, 0, 0, 0, parseErrorf("nil texture set")
	}
	p, err := s.loadPatch(name)
	if err != nil {
		return nil, nil, 0, 0, 0, 0, err
	}
	index := make([]byte, len(p.index))
	copy(index, p.index)
	opaque := make([]bool, len(p.opaque))
	copy(opaque, p.opaque)
	return index, opaque, p.width, p.height, p.leftOffset, p.topOffset, nil
}

func (s *Set) BuildPatchIndexedView(name string) ([]byte, []bool, int, int, int, int, error) {
	if s == nil {
		return nil, nil, 0, 0, 0, 0, parseErrorf("nil texture set")
	}
	p, err := s.loadPatch(name)
	if err != nil {
		return nil, nil, 0, 0, 0, 0, err
	}
	return p.index, p.opaque, p.width, p.height, p.leftOffset, p.topOffset, nil
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
	data, err := s.wad.LumpDataView(l)
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
		columns:    make([][]patchPost, w),
		raw:        data,
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
			p.columns[x] = append(p.columns[x], patchPost{top: int(top), pixels: data[co : co+length]})
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

// composeTexture mirrors R_GenerateLookup/R_GenerateComposite. The decision to
// composite is per column, not per texture: a single covering patch is read
// directly and its texture-definition origin Y is ignored. Overlapping columns
// apply each patch's origin Y, even when the overlapping post is transparent.
func (s *Set) composeTexture(t TextureDef) ([]byte, []bool) {
	pix := make([]byte, t.Width*t.Height)
	alpha := make([]bool, len(pix))
	patches := make([]*decodedPatch, len(t.Patches))
	coverage := make([]int, t.Width)
	for i, ref := range t.Patches {
		p, err := s.loadPatch(ref.PatchName)
		if err != nil {
			continue
		}
		patches[i] = p
		for x := max(0, ref.OriginX); x < min(t.Width, ref.OriginX+p.width); x++ {
			coverage[x]++
		}
	}
	for i, ref := range t.Patches {
		p := patches[i]
		if p == nil {
			continue
		}
		for x := max(0, ref.OriginX); x < min(t.Width, ref.OriginX+p.width); x++ {
			oy := ref.OriginY
			if coverage[x] == 1 {
				oy = 0
			}
			for _, post := range p.columnPosts(x - ref.OriginX) {
				position, count := oy+post.top, len(post.pixels)
				// R_DrawColumnInCache clips a negative destination by reducing
				// the count without advancing the source pointer. TEKWALL1
				// relies on this original behavior for its first 16 rows.
				if position < 0 {
					count += position
					position = 0
				}
				count = min(count, t.Height-position)
				for i := 0; i < count; i++ {
					di := (position+i)*t.Width + x
					pix[di], alpha[di] = post.pixels[i], true
				}
			}
		}
	}
	return pix, alpha
}

func (p *decodedPatch) columnPosts(x int) []patchPost {
	if len(p.columns) == p.width {
		return p.columns[x]
	}
	// In-memory synthetic patches may contain only decoded pixels.
	var posts []patchPost
	for y := 0; y < p.height; {
		if !p.opaque[y*p.width+x] {
			y++
			continue
		}
		post := patchPost{top: y}
		for y < p.height && p.opaque[y*p.width+x] {
			post.pixels = append(post.pixels, p.index[y*p.width+x])
			y++
		}
		posts = append(posts, post)
	}
	return posts
}
