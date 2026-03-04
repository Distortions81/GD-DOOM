package automap

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
)

func TestMeltCompareRender(t *testing.T) {
	const (
		w = 320
		h = 240
	)

	fromPix, toPix := buildMeltCompareBuffers(t, w, h)
	doomrand.Clear()
	faithfulFrames := simulateFaithfulMeltFrames(w, h, fromPix, toPix)
	doomrand.Clear()
	sourceFrames := simulateSourceportMeltFrames(w, h, fromPix, toPix)
	if len(faithfulFrames) == 0 || len(sourceFrames) == 0 {
		t.Fatalf("empty melt frame sequences: faithful=%d source=%d", len(faithfulFrames), len(sourceFrames))
	}
	if !bytes.Equal(faithfulFrames[len(faithfulFrames)-1], toPix) {
		t.Fatal("faithful melt final frame did not converge to target image")
	}
	if !bytes.Equal(sourceFrames[len(sourceFrames)-1], toPix) {
		t.Fatal("sourceport melt final frame did not converge to target image")
	}

	sampleFracs := []float64{0.0, 0.25, 0.5, 0.75, 1.0}
	totalDiff := uint64(0)
	for _, frac := range sampleFracs {
		fi := sampleFrameIndex(len(faithfulFrames), frac)
		si := sampleFrameIndex(len(sourceFrames), frac)
		totalDiff += frameAbsDiff(faithfulFrames[fi], sourceFrames[si])
	}
	avgDiff := float64(totalDiff) / float64(len(sampleFracs)*w*h*4)
	t.Logf("melt-compare faithful_frames=%d source_frames=%d sampled_avg_abs_diff=%.3f", len(faithfulFrames), len(sourceFrames), avgDiff)

	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	outPath := filepath.Join("testdata", "melt_compare.png")
	if err := writeMeltComparePNG(outPath, w, h, sampleFracs, faithfulFrames, sourceFrames); err != nil {
		t.Fatalf("write compare png: %v", err)
	}
	t.Logf("wrote %s", outPath)
}

func buildMeltCompareBuffers(t *testing.T, w, h int) ([]byte, []byte) {
	if from, to, ok := loadMeltCompareBuffersFromWAD(t, w, h); ok {
		return from, to
	}
	t.Log("melt-compare using synthetic buffers (WAD image lumps unavailable)")
	return buildSyntheticMeltCompareBuffers(w, h)
}

func loadMeltCompareBuffersFromWAD(t *testing.T, w, h int) ([]byte, []byte, bool) {
	t.Helper()
	wadPaths := []string{
		"DOOM1.WAD",
		filepath.Join("..", "DOOM1.WAD"),
		filepath.Join("..", "..", "DOOM1.WAD"),
		filepath.Join("..", "..", "..", "DOOM1.WAD"),
	}
	var (
		wf  *wad.File
		err error
	)
	for _, p := range wadPaths {
		wf, err = wad.Open(p)
		if err == nil {
			t.Logf("melt-compare wad path=%s", p)
			break
		}
	}
	if wf == nil {
		t.Logf("melt-compare wad open failed: %v", err)
		return nil, nil, false
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Logf("melt-compare texture load failed: %v", err)
		return nil, nil, false
	}
	from, ok := loadRawPic(ts, "TITLEPIC", w, h)
	if !ok {
		t.Log("melt-compare missing TITLEPIC raw image")
		return nil, nil, false
	}
	choices := []string{"INTERPIC", "HELP1", "CREDIT"}
	for _, name := range choices {
		to, ok := loadRawPic(ts, name, w, h)
		if ok {
			t.Logf("melt-compare using WAD images: from=TITLEPIC to=%s", name)
			return from, to, true
		}
	}
	t.Log("melt-compare missing INTERPIC/HELP1/CREDIT raw image")
	return nil, nil, false
}

func loadRawPic(ts *doomtex.Set, name string, w, h int) ([]byte, bool) {
	if ts == nil {
		return nil, false
	}
	rgba, rw, rh, err := ts.BuildRawPicRGBA(name, 0, w, h)
	if err == nil && rw == w && rh == h && len(rgba) == w*h*4 {
		return rgba, true
	}
	// Common Doom screen lumps are 320x200; upscale if our compare buffer differs.
	const srcW, srcH = 320, 200
	rgba, rw, rh, err = ts.BuildRawPicRGBA(name, 0, srcW, srcH)
	if err != nil || rw != srcW || rh != srcH || len(rgba) != srcW*srcH*4 {
		// Some WADs store these images as patches.
		prgba, pw, ph, _, _, perr := ts.BuildPatchRGBA(name, 0)
		if perr != nil || pw <= 0 || ph <= 0 || len(prgba) != pw*ph*4 {
			return nil, false
		}
		if pw == w && ph == h {
			return prgba, true
		}
		return scaleRGBA(prgba, pw, ph, w, h), true
	}
	return scaleRGBA(rgba, srcW, srcH, w, h), true
}

func scaleRGBA(src []byte, sw, sh, dw, dh int) []byte {
	if sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0 || len(src) < sw*sh*4 {
		return nil
	}
	out := make([]byte, dw*dh*4)
	for y := 0; y < dh; y++ {
		sy := (y * sh) / dh
		for x := 0; x < dw; x++ {
			sx := (x * sw) / dw
			si := (sy*sw + sx) * 4
			di := (y*dw + x) * 4
			copy(out[di:di+4], src[si:si+4])
		}
	}
	return out
}

func buildSyntheticMeltCompareBuffers(w, h int) ([]byte, []byte) {
	from := make([]byte, w*h*4)
	to := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			// Source image: dark checker + horizontal ramp.
			c0 := uint8((x * 255) / max(w-1, 1))
			if ((x/8)+(y/8))&1 == 0 {
				c0 /= 2
			}
			from[i+0] = c0
			from[i+1] = uint8((y * 180) / max(h-1, 1))
			from[i+2] = uint8((x + y) & 0xFF)
			from[i+3] = 0xFF

			// Target image: bright vertical bars + inverse ramp.
			bar := uint8(0)
			if (x/10)%2 == 0 {
				bar = 80
			}
			to[i+0] = uint8(255 - c0)
			to[i+1] = uint8((255-y*255/max(h-1, 1))/2) + bar
			to[i+2] = uint8((x * y) & 0xFF)
			to[i+3] = 0xFF
		}
	}
	return from, to
}

func simulateFaithfulMeltFrames(w, h int, fromPix, toPix []byte) [][]byte {
	work := append([]byte(nil), fromPix...)
	y := initMeltColumns(w)
	frames := make([][]byte, 0, 128)
	frames = append(frames, append([]byte(nil), work...))
	for i := 0; i < 4096; i++ {
		done := stepMeltColumns(y, w, h, fromPix, toPix, work, 1)
		frames = append(frames, append([]byte(nil), work...))
		if done {
			break
		}
	}
	return frames
}

func simulateSourceportMeltFrames(w, h int, fromPix, toPix []byte) [][]byte {
	work := append([]byte(nil), fromPix...)
	mult := sourcePortMeltRNGScale(h)
	y := initMeltColumnsScaled(sourcePortMeltInitColumns(), mult)
	meltTicks := sourcePortMeltRNGScale(h)
	frames := make([][]byte, 0, 128)
	frames = append(frames, append([]byte(nil), work...))
	for i := 0; i < 4096; i++ {
		done := stepMeltSlicesVirtual(y, meltVirtualH, w, h, fromPix, toPix, work, meltTicks, sourcePortMeltMoveColumns())
		frames = append(frames, append([]byte(nil), work...))
		if done {
			break
		}
	}
	return frames
}

func sampleFrameIndex(n int, frac float64) int {
	if n <= 1 {
		return 0
	}
	if frac <= 0 {
		return 0
	}
	if frac >= 1 {
		return n - 1
	}
	return int(frac * float64(n-1))
}

func frameAbsDiff(a, b []byte) uint64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var total uint64
	for i := 0; i < n; i++ {
		da := int(a[i]) - int(b[i])
		if da < 0 {
			da = -da
		}
		total += uint64(da)
	}
	return total
}

func writeMeltComparePNG(path string, w, h int, fracs []float64, faithfulFrames, sourceFrames [][]byte) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("invalid output size %dx%d", w, h)
	}
	out := image.NewRGBA(image.Rect(0, 0, w*3, h*len(fracs)))
	for i, frac := range fracs {
		fi := sampleFrameIndex(len(faithfulFrames), frac)
		si := sampleFrameIndex(len(sourceFrames), frac)
		yOff := i * h
		blitRGBA(out, image.Rect(0, yOff, w, yOff+h), faithfulFrames[fi], w, h)
		blitRGBA(out, image.Rect(w, yOff, 2*w, yOff+h), sourceFrames[si], w, h)
		diff := buildDiffFrame(faithfulFrames[fi], sourceFrames[si], w, h)
		blitRGBA(out, image.Rect(2*w, yOff, 3*w, yOff+h), diff, w, h)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, out)
}

func blitRGBA(dst *image.RGBA, rect image.Rectangle, src []byte, w, h int) {
	if dst == nil || len(src) < w*h*4 {
		return
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			dst.SetRGBA(rect.Min.X+x, rect.Min.Y+y, color.RGBA{
				R: src[i+0],
				G: src[i+1],
				B: src[i+2],
				A: src[i+3],
			})
		}
	}
}

func buildDiffFrame(a, b []byte, w, h int) []byte {
	n := w * h * 4
	out := make([]byte, n)
	for i := 0; i < n; i += 4 {
		dr := absInt(int(a[i+0]) - int(b[i+0]))
		dg := absInt(int(a[i+1]) - int(b[i+1]))
		db := absInt(int(a[i+2]) - int(b[i+2]))
		v := uint8(max(dr, max(dg, db)))
		out[i+0] = v
		out[i+1] = v
		out[i+2] = v
		out[i+3] = 0xFF
	}
	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
