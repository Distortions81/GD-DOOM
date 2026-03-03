package automap

import (
	"fmt"
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

	solid := image.NewRGBA(image.Rect(0, 0, 256, 192))
	fillRect(solid, solid.Bounds(), color.RGBA{R: 8, G: 8, B: 12, A: 255})
	textured := image.NewRGBA(image.Rect(0, 0, 256, 192))
	fillRect(textured, textured.Bounds(), color.RGBA{R: 8, G: 8, B: 12, A: 255})

	for _, tri := range tris {
		a := verts[tri[0]]
		b := verts[tri[1]]
		c := verts[tri[2]]
		fillTriangleSolid(solid, a, b, c, color.RGBA{R: 70, G: 140, B: 220, A: 255})
		fillTriangleTextured(textured, a, b, c)
	}
	for i := 0; i < len(verts); i++ {
		j := (i + 1) % len(verts)
		drawLine(solid, verts[i], verts[j], color.RGBA{R: 255, G: 80, B: 80, A: 255})
		drawLine(textured, verts[i], verts[j], color.RGBA{R: 255, G: 80, B: 80, A: 255})
	}

	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	writeDebugImageSet(t, "triangulation_debug", solid)
	writeDebugImageSet(t, "triangulation_textured_debug", textured)
	t.Logf("wrote testdata/triangulation_debug.* and testdata/triangulation_textured_debug.*")
}

func TestTriangulateWorldPolygonWritesComplexLevelImage(t *testing.T) {
	solid := image.NewRGBA(image.Rect(0, 0, 512, 320))
	textured := image.NewRGBA(image.Rect(0, 0, 512, 320))
	fillRect(solid, solid.Bounds(), color.RGBA{R: 8, G: 8, B: 12, A: 255})
	fillRect(textured, textured.Bounds(), color.RGBA{R: 8, G: 8, B: 12, A: 255})

	floorRegions := [][]worldPt{
		rectPoly(20, 20, 492, 120),   // top strip
		rectPoly(20, 120, 180, 180),  // mid-left
		rectPoly(240, 120, 300, 180), // mid-center
		rectPoly(360, 120, 492, 180), // mid-right
		rectPoly(20, 180, 492, 300),  // bottom strip
	}
	for _, poly := range floorRegions {
		tris, ok := triangulateWorldPolygon(poly)
		if !ok {
			t.Fatal("expected floor region triangulation")
		}
		for _, tri := range tris {
			a := poly[tri[0]]
			b := poly[tri[1]]
			c := poly[tri[2]]
			fillTriangleSolid(solid, a, b, c, color.RGBA{R: 70, G: 140, B: 220, A: 255})
			fillTriangleTextured(textured, a, b, c)
		}
	}

	outer := rectPoly(20, 20, 492, 300)
	for i := 0; i < len(outer); i++ {
		j := (i + 1) % len(outer)
		drawLine(solid, outer[i], outer[j], color.RGBA{R: 255, G: 80, B: 80, A: 255})
		drawLine(textured, outer[i], outer[j], color.RGBA{R: 255, G: 80, B: 80, A: 255})
	}

	pillars := [][]worldPt{
		rectPoly(180, 120, 240, 180),
		rectPoly(300, 120, 360, 180),
		rectPoly(120, 220, 160, 260),
		rectPoly(340, 220, 380, 260),
	}
	for _, p := range pillars {
		// Pillars are blockers: dark interior + wall outline.
		pr, ok := triangulateWorldPolygon(p)
		if !ok {
			t.Fatal("expected pillar triangulation")
		}
		for _, tri := range pr {
			a := p[tri[0]]
			b := p[tri[1]]
			c := p[tri[2]]
			fillTriangleSolid(solid, a, b, c, color.RGBA{R: 5, G: 5, B: 7, A: 255})
			fillTriangleSolid(textured, a, b, c, color.RGBA{R: 5, G: 5, B: 7, A: 255})
		}
		for i := 0; i < len(p); i++ {
			j := (i + 1) % len(p)
			drawLine(solid, p[i], p[j], color.RGBA{R: 255, G: 80, B: 80, A: 255})
			drawLine(textured, p[i], p[j], color.RGBA{R: 255, G: 80, B: 80, A: 255})
		}
	}

	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	writeDebugImageSet(t, "triangulation_complex_debug", solid)
	writeDebugImageSet(t, "triangulation_complex_textured_debug", textured)
	t.Logf("wrote testdata/triangulation_complex_debug.* and testdata/triangulation_complex_textured_debug.*")
}

func TestTriangulateWorldPolygonWritesMultiTextureLevelImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 512, 320))
	fillRect(img, img.Bounds(), color.RGBA{R: 8, G: 8, B: 12, A: 255})

	type region struct {
		poly []worldPt
		tex  int
	}
	regions := []region{
		{poly: rectPoly(20, 20, 260, 120), tex: 0},
		{poly: rectPoly(260, 20, 492, 120), tex: 1},
		{poly: rectPoly(20, 120, 180, 220), tex: 2},
		{poly: rectPoly(180, 120, 340, 220), tex: 3},
		{poly: rectPoly(340, 120, 492, 220), tex: 1},
		{poly: rectPoly(20, 220, 250, 300), tex: 2},
		{poly: rectPoly(250, 220, 492, 300), tex: 0},
	}

	for _, r := range regions {
		tris, ok := triangulateWorldPolygon(r.poly)
		if !ok {
			t.Fatal("expected region triangulation")
		}
		for _, tri := range tris {
			a := r.poly[tri[0]]
			b := r.poly[tri[1]]
			c := r.poly[tri[2]]
			fillTriangleTexturedVariant(img, a, b, c, r.tex)
		}
		for i := 0; i < len(r.poly); i++ {
			j := (i + 1) % len(r.poly)
			drawLine(img, r.poly[i], r.poly[j], color.RGBA{R: 255, G: 80, B: 80, A: 255})
		}
	}

	// Pillar blockers (dark fill + red walls).
	pillars := [][]worldPt{
		rectPoly(210, 145, 240, 175),
		rectPoly(285, 155, 315, 185),
		rectPoly(110, 245, 145, 280),
		rectPoly(400, 245, 435, 280),
	}
	for _, p := range pillars {
		pr, ok := triangulateWorldPolygon(p)
		if !ok {
			t.Fatal("expected pillar triangulation")
		}
		for _, tri := range pr {
			a := p[tri[0]]
			b := p[tri[1]]
			c := p[tri[2]]
			fillTriangleSolid(img, a, b, c, color.RGBA{R: 6, G: 6, B: 8, A: 255})
		}
		for i := 0; i < len(p); i++ {
			j := (i + 1) % len(p)
			drawLine(img, p[i], p[j], color.RGBA{R: 255, G: 80, B: 80, A: 255})
		}
	}

	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	writeDebugImageSet(t, "triangulation_multitex_debug", img)
	t.Logf("wrote testdata/triangulation_multitex_debug.*")
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

func fillTriangleSolid(img *image.RGBA, a, b, c worldPt, clr color.RGBA) {
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

func fillTriangleTextured(img *image.RGBA, a, b, c worldPt) {
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
	den := orient2D(a, b, c)
	if math.Abs(den) < 1e-9 {
		return
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			p := worldPt{x: float64(x) + 0.5, y: float64(y) + 0.5}
			if !pointInTri(p, a, b, c) {
				continue
			}
			w0 := orient2D(b, c, p) / den
			w1 := orient2D(c, a, p) / den
			w2 := orient2D(a, b, p) / den
			u := w0*a.x + w1*b.x + w2*c.x
			v := w0*a.y + w1*b.y + w2*c.y
			img.SetRGBA(x, y, checker64(u, v))
		}
	}
}

func checker64(u, v float64) color.RGBA {
	iu := int(math.Floor(u)) & 63
	iv := int(math.Floor(v)) & 63
	cell := ((iu >> 3) ^ (iv >> 3)) & 1
	if cell == 0 {
		return color.RGBA{R: 64, G: 170, B: 220, A: 255}
	}
	return color.RGBA{R: 215, G: 180, B: 72, A: 255}
}

func fillTriangleTexturedVariant(img *image.RGBA, a, b, c worldPt, texID int) {
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
	den := orient2D(a, b, c)
	if math.Abs(den) < 1e-9 {
		return
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			p := worldPt{x: float64(x) + 0.5, y: float64(y) + 0.5}
			if !pointInTri(p, a, b, c) {
				continue
			}
			w0 := orient2D(b, c, p) / den
			w1 := orient2D(c, a, p) / den
			w2 := orient2D(a, b, p) / den
			u := w0*a.x + w1*b.x + w2*c.x
			v := w0*a.y + w1*b.y + w2*c.y
			img.SetRGBA(x, y, samplePattern(texID, u, v))
		}
	}
}

func samplePattern(texID int, u, v float64) color.RGBA {
	switch texID % 4 {
	case 1:
		iu := int(math.Floor(u)) & 63
		iv := int(math.Floor(v)) & 63
		cell := ((iu >> 2) ^ (iv >> 2)) & 1
		if cell == 0 {
			return color.RGBA{R: 96, G: 180, B: 96, A: 255}
		}
		return color.RGBA{R: 46, G: 86, B: 46, A: 255}
	case 2:
		iu := int(math.Floor(u))
		iv := int(math.Floor(v))
		band := (iu + iv) & 7
		if band < 4 {
			return color.RGBA{R: 170, G: 120, B: 72, A: 255}
		}
		return color.RGBA{R: 120, G: 80, B: 48, A: 255}
	case 3:
		du := frac01(u / 64)
		dv := frac01(v / 64)
		return color.RGBA{
			R: uint8(40 + 120*du),
			G: uint8(60 + 120*dv),
			B: 180,
			A: 255,
		}
	default:
		return checker64(u, v)
	}
}

func writeDebugImageSet(t *testing.T, base string, img *image.RGBA) {
	t.Helper()
	outPath := filepath.Join("testdata", base+".png")
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	rawPath := filepath.Join("testdata", base+".rgb")
	raw, err := os.Create(rawPath)
	if err != nil {
		t.Fatalf("create rgb: %v", err)
	}
	defer raw.Close()
	if _, err := raw.Write(extractRGB(img)); err != nil {
		t.Fatalf("write rgb: %v", err)
	}
	metaPath := filepath.Join("testdata", base+".rgb.txt")
	meta := []byte(
		"width=" + itoa(img.Bounds().Dx()) + "\n" +
			"height=" + itoa(img.Bounds().Dy()) + "\n" +
			"format=RGB24\nrow_major=true\n",
	)
	if err := os.WriteFile(metaPath, meta, 0o644); err != nil {
		t.Fatalf("write rgb meta: %v", err)
	}
}

func rectPoly(x0, y0, x1, y1 float64) []worldPt {
	return []worldPt{
		{x: x0, y: y0},
		{x: x1, y: y0},
		{x: x1, y: y1},
		{x: x0, y: y1},
	}
}

func itoa(v int) string {
	return fmt.Sprintf("%d", v)
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
