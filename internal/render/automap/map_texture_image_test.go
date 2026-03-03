package automap

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestTriangulateWorldPolygonWritesDebugImage(t *testing.T) {
	verts := []worldPt{
		{x: 20, y: 20},
		{x: 236, y: 20},
		{x: 236, y: 92},
		{x: 148, y: 72},
		{x: 92, y: 164},
		{x: 20, y: 164},
	}
	tris, ok := triangulateWorldPolygon(verts)
	if !ok || len(tris) == 0 {
		t.Fatal("expected triangulation for debug polygon")
	}

	img := image.NewRGBA(image.Rect(0, 0, 256, 192))
	fillRect(img, img.Bounds(), color.RGBA{R: 8, G: 8, B: 12, A: 255})

	for _, tri := range tris {
		a := verts[tri[0]]
		b := verts[tri[1]]
		c := verts[tri[2]]
		fillTriangle(img, a, b, c, color.RGBA{R: 70, G: 140, B: 220, A: 255})
	}
	for i := 0; i < len(verts); i++ {
		j := (i + 1) % len(verts)
		drawLine(img, verts[i], verts[j], color.RGBA{R: 255, G: 80, B: 80, A: 255})
	}

	outPath := filepath.Join("testdata", "triangulation_debug.png")
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	rawPath := filepath.Join("testdata", "triangulation_debug.rgb")
	raw, err := os.Create(rawPath)
	if err != nil {
		t.Fatalf("create rgb: %v", err)
	}
	defer raw.Close()
	if _, err := raw.Write(extractRGB(img)); err != nil {
		t.Fatalf("write rgb: %v", err)
	}
	metaPath := filepath.Join("testdata", "triangulation_debug.rgb.txt")
	meta := []byte("width=256\nheight=192\nformat=RGB24\nrow_major=true\n")
	if err := os.WriteFile(metaPath, meta, 0o644); err != nil {
		t.Fatalf("write rgb meta: %v", err)
	}
	t.Logf("wrote %s", outPath)
}

func extractRGB(img *image.RGBA) []byte {
	b := make([]byte, 0, img.Bounds().Dx()*img.Bounds().Dy()*3)
	r := img.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := img.RGBAAt(x, y)
			b = append(b, c.R, c.G, c.B)
		}
	}
	return b
}

func fillRect(img *image.RGBA, r image.Rectangle, clr color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, clr)
		}
	}
}

func fillTriangle(img *image.RGBA, a, b, c worldPt, clr color.RGBA) {
	minX := int(math.Floor(min3(a.x, b.x, c.x)))
	maxX := int(math.Ceil(max3(a.x, b.x, c.x)))
	minY := int(math.Floor(min3(a.y, b.y, c.y)))
	maxY := int(math.Ceil(max3(a.y, b.y, c.y)))
	if minX < img.Bounds().Min.X {
		minX = img.Bounds().Min.X
	}
	if minY < img.Bounds().Min.Y {
		minY = img.Bounds().Min.Y
	}
	if maxX >= img.Bounds().Max.X {
		maxX = img.Bounds().Max.X - 1
	}
	if maxY >= img.Bounds().Max.Y {
		maxY = img.Bounds().Max.Y - 1
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			p := worldPt{x: float64(x) + 0.5, y: float64(y) + 0.5}
			if pointInTri(p, a, b, c) {
				img.SetRGBA(x, y, clr)
			}
		}
	}
}

func drawLine(img *image.RGBA, a, b worldPt, clr color.RGBA) {
	x0 := int(math.Round(a.x))
	y0 := int(math.Round(a.y))
	x1 := int(math.Round(b.x))
	y1 := int(math.Round(b.y))
	dx := int(math.Abs(float64(x1 - x0)))
	dy := -int(math.Abs(float64(y1 - y0)))
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		if image.Pt(x0, y0).In(img.Bounds()) {
			img.SetRGBA(x0, y0, clr)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func min3(a, b, c float64) float64 { return math.Min(a, math.Min(b, c)) }
func max3(a, b, c float64) float64 { return math.Max(a, math.Max(b, c)) }
