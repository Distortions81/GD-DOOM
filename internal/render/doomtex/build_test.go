package doomtex

import (
	"encoding/binary"
	"testing"

	"gddoom/internal/wad"
)

func TestDecodePatchAndBlit(t *testing.T) {
	// Build a 2x2 patch: column0=[1,2], column1=[3,4]
	// header: w,h,left,top + columnofs[2]
	b := make([]byte, 16)
	binary.LittleEndian.PutUint16(b[0:2], 2)
	binary.LittleEndian.PutUint16(b[2:4], 2)
	binary.LittleEndian.PutUint32(b[8:12], 16)
	// col0 posts at 16: top=0 len=2 pad=0 data[1,2] pad=0 end=255
	b = append(b, 0, 2, 0, 1, 2, 0, 255)
	co1 := len(b)
	binary.LittleEndian.PutUint32(b[12:16], uint32(co1))
	// col1: top=0 len=2 pad=0 data[3,4] pad=0 end=255
	b = append(b, 0, 2, 0, 3, 4, 0, 255)

	p, err := decodePatch(b)
	if err != nil {
		t.Fatalf("decodePatch: %v", err)
	}
	if p.width != 2 || p.height != 2 {
		t.Fatalf("size=%dx%d", p.width, p.height)
	}
	s := &Set{patchCache: map[string]*decodedPatch{"P": p}, textures: map[string]TextureDef{"T": {Width: 2, Height: 2, Patches: []PatchRef{{PatchName: "P"}}}}}
	legacy, err := s.BuildTextureOpaqueIndexed("T")
	if err != nil {
		t.Fatal(err)
	}
	// The opaque reader sees raw bytes following the short post, including
	// its padding and next column's header; the last row is safely bounded.
	if legacy[0] != 1 || legacy[2] != 2 || legacy[4] != 0 || legacy[6] != 255 || legacy[254] != 0 {
		t.Fatalf("opaque column bytes %v", legacy[:10])
	}
	posts := s.BuildTextureMaskedPosts("T")
	if len(posts) != 2 || posts[0][0].Length != 2 || posts[0][0].Indexed[3] != 255 {
		t.Fatal("lost raw masked post stream")
	}

	dst := make([]byte, 4)
	alpha := make([]bool, 4)
	blitPatch(dst, alpha, 2, 2, p, 0, 0)
	if !alpha[0] || !alpha[1] || !alpha[2] || !alpha[3] {
		t.Fatalf("alpha not fully opaque: %#v", alpha)
	}
	// row-major: [1,3,2,4]
	want := []byte{1, 3, 2, 4}
	for i := range want {
		if dst[i] != want[i] {
			t.Fatalf("dst[%d]=%d want=%d", i, dst[i], want[i])
		}
	}
}

func TestBuildTextureRGBA_CompositesPatches(t *testing.T) {
	p := &decodedPatch{
		width:  2,
		height: 2,
		index:  []uint8{1, 2, 3, 4},
		opaque: []bool{true, true, true, true},
	}
	var pal [256][3]uint8
	pal[1] = [3]uint8{10, 0, 0}
	pal[2] = [3]uint8{0, 20, 0}
	pal[3] = [3]uint8{0, 0, 30}
	pal[4] = [3]uint8{40, 50, 60}

	s := &Set{
		palettes:    [][256][3]uint8{pal},
		textures:    map[string]TextureDef{"TEST": {Name: "TEST", Width: 2, Height: 2, Patches: []PatchRef{{PatchName: "P1"}}}},
		patchCache:  map[string]*decodedPatch{"P1": p},
		patchByName: map[string]wad.Lump{},
	}

	rgba, w, h, err := s.BuildTextureRGBA("test", 0)
	if err != nil {
		t.Fatalf("BuildTextureRGBA: %v", err)
	}
	if w != 2 || h != 2 {
		t.Fatalf("size=%dx%d", w, h)
	}
	if len(rgba) != 16 {
		t.Fatalf("len=%d want=16", len(rgba))
	}
	if rgba[0] != 10 || rgba[1] != 0 || rgba[2] != 0 || rgba[3] != 255 {
		t.Fatalf("first pixel=%v", rgba[:4])
	}
}

func TestBuildTextureIndexed_CompositesPatches(t *testing.T) {
	p := &decodedPatch{
		width:  2,
		height: 2,
		index:  []uint8{1, 2, 3, 4},
		opaque: []bool{true, true, true, true},
	}
	s := &Set{
		textures:    map[string]TextureDef{"TEST": {Name: "TEST", Width: 2, Height: 2, Patches: []PatchRef{{PatchName: "P1"}}}},
		patchCache:  map[string]*decodedPatch{"P1": p},
		patchByName: map[string]wad.Lump{},
	}

	indexed, w, h, err := s.BuildTextureIndexed("test")
	if err != nil {
		t.Fatalf("BuildTextureIndexed: %v", err)
	}
	if w != 2 || h != 2 {
		t.Fatalf("size=%dx%d", w, h)
	}
	want := []byte{1, 2, 3, 4}
	for i := range want {
		if indexed[i] != want[i] {
			t.Fatalf("indexed[%d]=%d want=%d", i, indexed[i], want[i])
		}
	}
}

func TestBuildPatchIndexedViewSharesDecodedPatchSlices(t *testing.T) {
	p := &decodedPatch{
		width:  2,
		height: 2,
		index:  []uint8{1, 2, 3, 4},
		opaque: []bool{true, true, true, true},
	}
	s := &Set{
		patchCache:  map[string]*decodedPatch{"P1": p},
		patchByName: map[string]wad.Lump{},
	}

	indexed, opaque, w, h, _, _, err := s.BuildPatchIndexedView("P1")
	if err != nil {
		t.Fatalf("BuildPatchIndexedView: %v", err)
	}
	if w != 2 || h != 2 {
		t.Fatalf("size=%dx%d", w, h)
	}
	indexed[0] = 9
	opaque[1] = false
	if p.index[0] != 9 {
		t.Fatalf("index view did not share backing slice")
	}
	if p.opaque[1] {
		t.Fatalf("opaque view did not share backing slice")
	}
}

func TestTexturePatchOriginIsChosenPerColumn(t *testing.T) {
	// Column 0 has one patch (ignore Y=-1), column 1 overlaps two patches
	// (apply both origins), and column 2 is uncovered. Transparent posts still
	// count toward R_GenerateLookup's patch coverage.
	a := &decodedPatch{width: 3, height: 4, index: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, opaque: []bool{true, true, true, true, true, true, true, true, true, true, true, true}}
	b := &decodedPatch{width: 1, height: 2, index: []byte{0, 99}, opaque: []bool{false, true}}
	var pal [256][3]uint8
	for i := range pal {
		pal[i] = [3]uint8{uint8(i), uint8(i), uint8(i)}
	}
	s := &Set{palettes: [][256][3]uint8{pal}, patchCache: map[string]*decodedPatch{"A": a, "B": b}, textures: map[string]TextureDef{"TEST": {Width: 3, Height: 4, Patches: []PatchRef{{PatchName: "A", OriginX: -1, OriginY: -1}, {PatchName: "B", OriginX: 1, OriginY: 1}}}}}
	want := []byte{2, 3, 0, 5, 6, 0, 8, 99, 0, 11, 0, 0}
	pixels, _, _, err := s.BuildTextureIndexed("TEST")
	if err != nil {
		t.Fatal(err)
	}
	rgba, _, _, err := s.BuildTextureRGBA("TEST", 0)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range want {
		alpha := byte(255)
		if v == 0 {
			alpha = 0
		}
		if pixels[i] != v || rgba[i*4] != v || rgba[i*4+3] != alpha {
			t.Fatalf("pixel %d: index=%d rgba=%v want=%d alpha=%d", i, pixels[i], rgba[i*4:i*4+4], v, alpha)
		}
	}
}
