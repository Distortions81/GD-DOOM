package doomtex

import (
	"encoding/binary"
	"gddoom/internal/media"
)

// BuildTextureMaskedPosts preserves the raw post origins and their bounded
// source bytes for R_DrawMaskedColumn. Composite columns have no original post
// stream; retain the decoded fallback for those nonstandard masked textures.
func (s *Set) BuildTextureMaskedPosts(name string) [][]media.WallPost {
	t, ok := s.textures[normalizeName(name)]
	if !ok {
		return nil
	}
	out := make([][]media.WallPost, t.Width)
	covered := make([]bool, t.Width)
	for _, ref := range t.Patches {
		p, err := s.loadPatch(ref.PatchName)
		if err != nil {
			return nil
		}
		for x := max(0, ref.OriginX); x < min(t.Width, ref.OriginX+p.width); x++ {
			if covered[x] {
				return nil
			}
			covered[x] = true
			for _, post := range p.columnPosts(x - ref.OriginX) {
				bytes := make([]byte, 128)
				copy(bytes, post.pixels[:min(128, cap(post.pixels))])
				out[x] = append(out[x], media.WallPost{Top: post.top, Length: len(post.pixels), Indexed: bytes})
			}
		}
	}
	return out
}

// BuildTextureOpaqueIndexed reconstructs the bounded bytes read by classic's
// R_GetColumn + R_DrawColumn. A short single-patch column can read subsequent
// post headers/pixels within its lump because the row mask is always 127.
// Out-of-lump/allocation reads are left zero, never copied from process memory.
// Masked walls must continue to use the decoded posts from BuildTextureIndexed.
func (s *Set) BuildTextureOpaqueIndexed(name string) ([]byte, error) {
	t, ok := s.textures[normalizeName(name)]
	if !ok {
		return nil, parseErrorf("texture not found: %s", name)
	}
	counts := make([]int, t.Width)
	direct := make([][]byte, t.Width)
	for _, ref := range t.Patches {
		p, err := s.loadPatch(ref.PatchName)
		if err != nil {
			return nil, err
		}
		for x := max(0, ref.OriginX); x < min(t.Width, ref.OriginX+p.width); x++ {
			counts[x]++
			if len(p.raw) >= 8+p.width*4 {
				o := int(binary.LittleEndian.Uint32(p.raw[8+(x-ref.OriginX)*4:])) + 3
				if o >= 0 && o < len(p.raw) {
					direct[x] = p.raw[o:min(o+128, len(p.raw))]
				}
			}
		}
	}
	pix, _ := s.composeTexture(t)
	var composite []byte
	offsets := make([]int, t.Width)
	for x, count := range counts {
		if count > 1 {
			offsets[x] = len(composite)
			for y := 0; y < t.Height; y++ {
				composite = append(composite, pix[y*t.Width+x])
			}
		}
	}
	out := make([]byte, t.Width*128)
	for x, count := range counts {
		source := direct[x]
		if count > 1 {
			source = composite[offsets[x]:min(offsets[x]+128, len(composite))]
		}
		for y, v := range source {
			out[y*t.Width+x] = v
		}
	}
	return out, nil
}

// BuildPatchMaskedPosts keeps adjacent posts distinct. Their individual origins
// matter because classic's sprite column reader masks the post-relative row.
func (s *Set) BuildPatchMaskedPosts(name string) [][]media.WallPost {
	p, err := s.loadPatch(name)
	if err != nil {
		return nil
	}
	out := make([][]media.WallPost, p.width)
	for x := range out {
		for _, post := range p.columnPosts(x) {
			data := make([]byte, 128)
			copy(data, post.pixels[:min(128, cap(post.pixels))])
			out[x] = append(out[x], media.WallPost{Top: post.top, Length: len(post.pixels), Indexed: data})
		}
	}
	return out
}
