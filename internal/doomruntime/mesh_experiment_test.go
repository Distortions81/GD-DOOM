package doomruntime

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestMeshExperimentRealE1M1(t *testing.T) {
	g := loadMeshExperimentGame(t)
	r := g.ensureMeshExperiment()
	if r.missing != 0 {
		t.Fatalf("E1M1 has %d unmeshed leaves", r.missing)
	}
	if r.fallback != 0 {
		t.Fatalf("E1M1 has %d fallback sector planes", r.fallback)
	}
	before := g.SimChecksum()
	for _, mode := range []levelmesh.Mode{levelmesh.Textured, levelmesh.Sectors, levelmesh.Wireframe} {
		r.mode = mode
		g.renderMeshExperiment(r)
		covered := 0
		for _, depth := range r.raster.Depth {
			if math.IsNaN(depth) || math.IsInf(depth, 0) {
				t.Fatal("nonfinite depth")
			}
			if depth > 0 {
				covered++
			}
		}
		if covered != g.viewW*g.viewH {
			t.Fatalf("mode %s has %d uncovered pixels in the enclosed starting room", mode, g.viewW*g.viewH-covered)
		}
		t.Logf("%s: %d triangles, %d visible, %d covered pixels", mode, len(r.triangles), r.raster.Drawn, covered)
		captureMeshFrame(t, "e1m1-mesh-"+string(mode), r.raster.Pixels, g.viewW, g.viewH)
	}
	if g.SimChecksum() != before {
		t.Fatal("mesh render changed simulation")
	}
	// A rendering height change must reach the mesh on its next frame.
	sector := 0
	g.sectorFloor[sector] += fracUnit * 8
	g.renderMeshExperiment(r)
	for _, tri := range r.triangles {
		if tri.Sector == sector && tri.Kind == levelmesh.Floor && tri.Vertices[0].Z != float64(g.sectorFloor[sector])/fracUnit {
			t.Fatal("floor mesh kept stale height")
		}
	}
}

func loadMeshExperimentGame(t testing.TB) *game {
	return loadMeshExperimentMap(t, "E1M1")
}

func loadMeshExperimentMap(t testing.TB, name mapdata.MapName) *game {
	t.Helper()
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, name)
	if err != nil {
		t.Fatal(err)
	}
	flats, err := doomtex.LoadFlatsRGBA(wf, 0)
	if err != nil {
		t.Fatal(err)
	}
	walls := map[string]WallTexture{}
	for _, name := range set.TextureNames() {
		rgba, w, h, err := set.BuildTextureRGBA(name, 0)
		if err != nil {
			t.Fatal(err)
		}
		walls[name] = WallTexture{RGBA: rgba, Width: w, Height: h}
	}
	status, sprites := map[string]WallTexture{}, map[string]WallTexture{}
	for _, lump := range wf.Lumps {
		if !strings.HasPrefix(lump.Name, "ST") && !strings.HasPrefix(lump.Name, "PISG") && !strings.HasPrefix(lump.Name, "PISF") {
			continue
		}
		rgba, w, h, ox, oy, err := set.BuildPatchRGBA(lump.Name, 0)
		if err != nil {
			continue
		}
		tex := WallTexture{RGBA: rgba, Width: w, Height: h, OffsetX: ox, OffsetY: oy}
		if strings.HasPrefix(lump.Name, "ST") {
			status[lump.Name] = tex
		} else {
			sprites[lump.Name] = tex
		}
	}
	g := newGame(m, Options{Width: 640, Height: 400, SourcePortMode: true, MeshRenderer: "textured", NoMonsters: true, NoFPS: true, FlatBank: flats, WallTexBank: walls, StatusPatchBank: status, SpritePatchBank: sprites})
	g.renderPX, g.renderPY = float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit
	g.renderAngle = g.p.angle
	g.renderAlpha = 1
	g.syncRenderState()
	return g
}

func TestMeshExperimentE1M3MidwallBottom(t *testing.T) {
	g := loadMeshExperimentMap(t, "E1M3")
	r := g.ensureMeshExperiment()
	g.renderMeshExperiment(r)
	// Line 5 has BRNSMALC on both sides of a 72-unit opening. Its 64-unit
	// texture must wrap through the bottom eight units, as our classic
	// renderer does. Previously the mesh cut the card off at Z=72.
	line := g.m.Linedefs[5]
	side := int(line.SideNum[0])
	if g.m.Sidedefs[side].Mid != "BRNSMALC" {
		t.Fatal("unexpected E1M3 fixture")
	}
	var faces []levelmesh.Triangle
	bottom := math.Inf(1)
	for _, tri := range r.triangles {
		if tri.Sidedef == side && tri.Kind == levelmesh.Middle {
			faces = append(faces, tri)
			for _, v := range tri.Vertices {
				bottom = math.Min(bottom, v.Z)
			}
		}
	}
	if bottom != 64 {
		t.Fatalf("midwall ends at Z=%g instead of portal floor 64", bottom)
	}
	var raster levelmesh.Rasterizer
	raster.Render(faces, 320, 200, levelmesh.Camera{X: -1312, Y: -2944, Z: 100, Yaw: -math.Pi / 2}, levelmesh.Textured, func(tri levelmesh.Triangle) levelmesh.Texture { return g.meshMaterial(r, tri) }, nil)
	// Pixel center (160.5,179.5) hits Z=68.2, inside the former bottom gap.
	p := (179*320 + 160) * 4
	tex := g.meshMaterial(r, faces[0])
	want := tex.RGBA[(3*tex.Width+32)*4 : (3*tex.Width+32)*4+3]
	if raster.Depth[179*320+160] == 0 || !bytes.Equal(raster.Pixels[p:p+3], want) {
		t.Fatalf("bottom gap pixel=%v want repeated texel %v", raster.Pixels[p:p+3], want)
	}
	captureMeshFrame(t, "e1m3-midwall-bottom", raster.Pixels, 320, 200)
}

func BenchmarkMeshExperimentE1M1(b *testing.B) {
	for _, width := range []int{320, 640, 1280} {
		b.Run(fmt.Sprint(width), func(b *testing.B) {
			g := loadMeshExperimentGame(b)
			g.viewW, g.viewH = width, width*5/8
			r := g.ensureMeshExperiment()
			g.renderMeshExperiment(r)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				g.renderMeshExperiment(r)
			}
		})
	}
}

func captureMeshFrame(t *testing.T, name string, pixels []byte, w, h int) {
	t.Helper()
	if dir := os.Getenv("GD_MESH_CAPTURE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, &image.RGBA{Pix: pixels, Stride: w * 4, Rect: image.Rect(0, 0, w, h)})
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
}

func TestMeshExperimentOptInAndCycle(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	if g.ensureMeshExperiment() != nil {
		t.Fatal("experiment enabled by default")
	}
	g.opts.MeshRenderer = "textured"
	g.input.justPressedKeys = map[ebiten.Key]struct{}{ebiten.KeyF7: {}}
	for _, want := range []levelmesh.Mode{levelmesh.Sectors, levelmesh.Wireframe, "", levelmesh.Textured} {
		g.updateMeshExperiment()
		if got := g.meshExperiment.mode; got != want {
			t.Fatalf("cycle %q want %q", got, want)
		}
	}
}

// Audit all supplied maps without treating unsupported sector rings as proof
// that their subsector fallback is watertight. The HUD exposes these fallbacks.
func TestMeshExperimentMaps(t *testing.T) {
	paths := os.Getenv("GD_GEOMETRY_WADS")
	if paths == "" {
		t.Skip("set GD_GEOMETRY_WADS to comma-separated IWAD paths")
	}
	marker := regexp.MustCompile(`^(E[1-9]M[1-9]|MAP[0-9][0-9])$`)
	for _, path := range strings.Split(paths, ",") {
		wf, err := wad.Open(strings.TrimSpace(path))
		if err != nil {
			t.Fatal(err)
		}
		maps, fallbacks := 0, 0
		for _, lump := range wf.Lumps {
			if !marker.MatchString(lump.Name) {
				continue
			}
			m, err := mapdata.LoadMap(wf, mapdata.MapName(lump.Name))
			if err != nil {
				t.Fatal(err)
			}
			g := &game{m: m, bounds: mapBounds(m), opts: Options{MeshRenderer: "textured"}}
			g.initSubSectorSectorCache()
			r := g.ensureMeshExperiment()
			for _, tris := range r.planes {
				for _, tri := range tris {
					for _, v := range tri {
						if math.IsNaN(v.X) || math.IsNaN(v.Y) || math.IsInf(v.X, 0) || math.IsInf(v.Y, 0) {
							t.Fatalf("%s has nonfinite vertex", m.Name)
						}
					}
				}
			}
			if r.fallback > 0 {
				t.Logf("%s %s: %d fallback sector planes", filepath.Base(path), m.Name, r.fallback)
			}
			maps++
			fallbacks += r.fallback
		}
		if maps == 0 {
			t.Fatalf("no supported maps in %s", path)
		}
		t.Logf("%s: %d maps, %d fallback sector planes", filepath.Base(path), maps, fallbacks)
	}
}
