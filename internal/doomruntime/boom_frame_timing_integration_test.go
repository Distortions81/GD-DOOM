//go:build integration

package doomruntime

import (
	"encoding/json"
	"fmt"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

// Portable to the previous commit for an apples-to-apples world-rendering A/B.
// ReadPixels is benchmark-only synchronization, not part of normal rendering.
func TestBoomFrameTiming(t *testing.T) {
	out := os.Getenv("GD_FRAME_TIMING_OUT")
	if out == "" {
		t.Skip("set GD_FRAME_TIMING_OUT")
	}
	type result struct {
		Case, Backend          string
		Width, Height, Samples int
		P50MS, P95MS, P99MS    float64
	}
	var results []result
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		for _, size := range [][2]int{{320, 200}, {1280, 800}} {
			for _, gpu := range []bool{false, true} {
				for _, label := range []string{"vanilla-map01", "opaque-panels", "hidden-translucency", "two-translucent-layers"} {
					if only := os.Getenv("GD_FRAME_TIMING_ONLY"); only != "" && label != only {
						continue
					}
					path := os.Getenv("GD_FRAME_TIMING_IWAD")
					if label != "vanilla-map01" {
						path = os.Getenv("GD_FRAME_TIMING_FIXTURE")
					}
					w, err := wad.Open(path)
					if err != nil {
						t.Fatal(err)
					}
					m, err := mapdata.LoadMap(w, "MAP01")
					if err != nil {
						t.Fatal(err)
					}
					if label == "opaque-panels" {
						for i := range m.Linedefs {
							if m.Linedefs[i].Special == 260 {
								m.Linedefs[i].Special = 0
							}
						}
					}
					ts, err := doomtex.LoadFromWAD(w)
					if err != nil {
						t.Fatal(err)
					}
					palette, err := ts.PaletteRGBA(0)
					if err != nil {
						t.Fatal(err)
					}
					walls := make(map[string]WallTexture)
					for _, name := range ts.TextureNames() {
						rgba, x, y, e := ts.BuildTextureRGBA(name, 0)
						if e != nil {
							t.Fatal(e)
						}
						indexed, _, _, e := ts.BuildTextureIndexed(name)
						if e != nil {
							t.Fatal(e)
						}
						tex := WallTexture{Width: x, Height: y, RGBA: rgba, Indexed: indexed}
						if data, e := ts.BuildTextureOpaqueIndexed(name); e == nil {
							tex.SetClassicOpaque(data)
							tex.ClassicMasked = ts.BuildTextureMaskedPosts(name)
						}
						walls[name] = tex
					}
					sprites := make(map[string]WallTexture)
					inside := false
					for _, l := range w.Lumps {
						switch l.Name {
						case "S_START", "SS_START":
							inside = true
							continue
						case "S_END", "SS_END":
							inside = false
							continue
						}
						if !inside || l.Size == 0 {
							continue
						}
						rgba, x, y, ox, oy, e := ts.BuildPatchRGBA(l.Name, 0)
						if e != nil {
							continue
						}
						indexed, _, _, _, _, _, e := ts.BuildPatchIndexed(l.Name)
						if e != nil {
							continue
						}
						tex := WallTexture{Width: x, Height: y, OffsetX: ox, OffsetY: oy, RGBA: rgba, Indexed: indexed}
						tex.EnsureOpaqueMask()
						sprites[l.Name] = tex
					}
					flats, err := doomtex.LoadFlatsRGBA(w, 0)
					if err != nil {
						t.Fatal(err)
					}
					indices, err := doomtex.LoadFlatsIndexed(w)
					if err != nil {
						t.Fatal(err)
					}
					l, ok := w.LumpByName("COLORMAP")
					if !ok {
						t.Fatal("COLORMAP missing")
					}
					cm, err := w.LumpData(l)
					if err != nil {
						t.Fatal(err)
					}
					opts := Options{Width: size[0], Height: size[1], PlayerSlot: 1, SkillLevel: 3, NoMonsters: true, GPURenderer: gpu, DoomPaletteRGBA: palette, DoomColorMap: cm, DoomColorMapRows: len(cm) / 256, WallTexBank: walls, SpritePatchBank: sprites, FlatBank: flats, FlatBankIndexed: indices}
					// Reflection is only for running this same benchmark source before the
					// option was added. Gameplay and frame loops never use reflection.
					field := reflect.ValueOf(&opts).Elem().FieldByName("BoomTranMaps")
					if field.IsValid() {
						maps := map[string][]byte{}
						for _, name := range []string{"TRANMAP", "CUSTMAP", "REVMAP"} {
							if lump, ok := w.LumpByName(name); ok && lump.Size == 65536 {
								maps[name], _ = w.LumpDataView(lump)
							}
						}
						field.Set(reflect.ValueOf(maps))
					}
					g := newGame(m, opts)
					g.setGammaLevel(0)
					g.syncRenderState()
					g.prepareRenderState()
					g.viewW, g.viewH = size[0], size[1]
					g.geometryAspectY = 1
					if label != "vanilla-map01" {
						g.renderPX, g.renderPY, g.renderAngle = -192, 0, 0
						g.playerViewZ = 41 * fracUnit
						g.p.z = 0
						if label == "hidden-translucency" {
							g.renderAngle = doomAng180
						}
					}
					dst := ebiten.NewImage(size[0], size[1])
					pixels := make([]byte, size[0]*size[1]*4)
					draw := func() {
						g.ensureWallLayer()
						clear(g.wallPix)
						dst.Clear()
						g.drawDoomBasic3D(dst)
						dst.ReadPixels(pixels)
					}
					for i := 0; i < 20; i++ {
						draw()
					}
					if gpu && (g.gpu == nil || g.gpu.failed) {
						t.Fatal("GPU fallback")
					}
					samples := make([]float64, 120)
					for i := range samples {
						start := time.Now()
						draw()
						samples[i] = float64(time.Since(start)) / float64(time.Millisecond)
					}
					sort.Float64s(samples)
					backend := "cpu"
					if gpu {
						backend = "gpu"
					}
					v := result{label, backend, size[0], size[1], len(samples), samples[len(samples)*50/100], samples[len(samples)*95/100], samples[len(samples)*99/100]}
					results = append(results, v)
					fmt.Printf("TIMING %s %s %dx%d p50=%.3f p95=%.3f p99=%.3f ms\n", label, backend, size[0], size[1], v.P50MS, v.P95MS, v.P99MS)
					dst.Deallocate()
				}
			}
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, append(raw, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}
