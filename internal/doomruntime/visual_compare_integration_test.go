//go:build integration

package doomruntime

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

type visualProbeScene struct {
	BodyZ, ViewHeight, FloorZ, CeilingZ float64
	Name                                string `json:"name"`
	Map                                 string `json:"map"`
	X, Y, Z                             float64
	Angle                               uint32 `json:"angle"`
	Tic                                 int    `json:"tic"`
	Category                            string `json:"category"`
	DeathTics                           int    `json:"death_tics,omitempty"`
	LevelTime                           int    `json:"level_time"`
	Dead                                bool   `json:"dead"`
}

// TestVisualRendererCapture is driven by scripts/visual_compare.py. It uses
// real world draws, optional map actors and optional world-tic simulation.
// Fixed cameras have no HUD or interpolation.
func TestVisualRendererCapture(t *testing.T) {
	out := os.Getenv("GD_VISUAL_OUT")
	if out == "" {
		t.Skip("run scripts/visual_compare.py")
	}
	width, err := strconv.Atoi(os.Getenv("GD_VISUAL_WIDTH"))
	if err != nil || width < 64 {
		t.Fatal("invalid GD_VISUAL_WIDTH")
	}
	height, err := strconv.Atoi(os.Getenv("GD_VISUAL_HEIGHT"))
	if err != nil || height < 64 {
		t.Fatal("invalid GD_VISUAL_HEIGHT")
	}
	wf, err := wad.Open(os.Getenv("GD_VISUAL_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatal(err)
	}
	palette, err := ts.PaletteRGBA(0)
	if err != nil {
		t.Fatal(err)
	}
	walls := buildWallTextureBankForTest(t, ts)
	var sprites map[string]WallTexture
	if os.Getenv("GD_VISUAL_ACTORS") == "1" {
		sprites = buildBillboardSpritePatchBankForTest(t, ts)
		for name, tex := range sprites {
			index, _, _, _, _, _, err := ts.BuildPatchIndexed(name)
			if err != nil {
				t.Fatal(err)
			}
			tex.Indexed = index
			tex.EnsureOpaqueMask()
			sprites[name] = tex
		}
	}
	flats, err := doomtex.LoadFlatsRGBA(wf, 0)
	if err != nil {
		t.Fatal(err)
	}
	indices, err := doomtex.LoadFlatsIndexed(wf)
	if err != nil {
		t.Fatal(err)
	}
	lump, ok := wf.LumpByName("COLORMAP")
	if !ok {
		t.Fatal("missing COLORMAP")
	}
	colormap, err := wf.LumpData(lump)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("GD_VISUAL_TRANMAP") == "1" {
		table := doomtex.LoadBoomTranMaps(wf)["TRANMAP"]
		if len(table) != 65536 {
			table = doomtex.GenerateBoomTranMap(palette)
		}
		if err := os.WriteFile(filepath.Join(out, "gd-tranmap.bin"), table, 0644); err != nil {
			t.Fatal(err)
		}
	}
	fullbright := os.Getenv("GD_VISUAL_LIGHTING") != "1"
	if fullbright {
		for i := 0; i < min(len(colormap), 32*256); i++ {
			colormap[i] = byte(i)
		}
	}
	var demo *DemoScript
	if path := os.Getenv("GD_VISUAL_DEMO"); path != "" {
		demo, err = LoadDemoScript(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	var scenes []visualProbeScene
	if input := os.Getenv("GD_VISUAL_SCENES"); input != "" {
		data, err := os.ReadFile(input)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &scenes); err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		validName := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
		for _, c := range scenes {
			if !validName.MatchString(c.Name) || len(c.Name) > 127 || seen[c.Name] || c.Tic < 0 || c.Tic > 100000 ||
				math.Abs(c.X) > 32767 || math.Abs(c.Y) > 32767 || math.Abs(c.Z) > 32767 ||
				!strings.Contains(","+os.Getenv("GD_VISUAL_MAPS")+",", ","+c.Map+",") {
				t.Fatalf("invalid or duplicate scene: %+v", c)
			}
			seen[c.Name] = true
		}
	}
	manifest, err := os.Create(filepath.Join(out, "scenes.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer manifest.Close()
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		for _, name := range strings.Split(os.Getenv("GD_VISUAL_MAPS"), ",") {
			m, err := mapdata.LoadMap(wf, mapdata.MapName(name))
			if err != nil {
				t.Fatal(err)
			}
			opts := Options{BoomTranMaps: doomtex.LoadBoomTranMaps(wf), BoomColorMaps: doomtex.LoadBoomColormaps(wf), SpritePatchBank: sprites, Width: width, Height: height, PlayerSlot: 1, SkillLevel: 3, DoomPaletteRGBA: palette, DoomColorMap: colormap, DoomColorMapRows: len(colormap) / 256, WallTexBank: walls, FlatBank: flats, FlatBankIndexed: indices}
			opts.DisableDoomLighting = os.Getenv("GD_VISUAL_DISABLE_LIGHTING") == "1"
			opts.SourcePortMode = os.Getenv("GD_VISUAL_SOURCE_PORT") == "1"
			opts.SourcePortSectorLighting = opts.SourcePortMode
			opts.WallTextureAnimSequences = doomtex.LoadWallTextureAnimSequences(ts, doomtex.DoomWallAnimDefs)
			opts.FlatTextureAnimSequences = doomtex.LoadFlatAnimSequences(wf, doomtex.DoomFlatAnimDefs)
			g := newGame(m, opts)
			g.setGammaLevel(0)
			g.syncRenderState()
			g.prepareRenderState()
			g.viewW, g.viewH = width, height
			g.geometryAspectY = 1
			if os.Getenv("GD_VISUAL_SCENES") == "" {
				if demo != nil {
					for _, raw := range strings.Split(os.Getenv("GD_VISUAL_DEMO_TICS"), ",") {
						tic, e := strconv.Atoi(raw)
						if e != nil || tic < 1 || tic > len(demo.Tics) {
							t.Fatalf("invalid demo tic %s", raw)
						}
						for _, rawDeath := range strings.Split(os.Getenv("GD_VISUAL_DEATH_TICS"), ",") {
							death, e := strconv.Atoi(rawDeath)
							if e != nil || death < 0 || death > 140 {
								t.Fatalf("invalid death tic %s", rawDeath)
							}
							scenes = append(scenes, visualProbeScene{Name: fmt.Sprintf("%s-demo-%d-death-%d", name, tic, death), Map: name, Tic: tic, DeathTics: death, Category: "demo"})
						}
					}
				} else {
					scenes = append(scenes, visualProbeScenes(g)...)
				}
			}
			cpu, gpu := ebiten.NewImage(width, height), ebiten.NewImage(width, height)
			for sceneIndex := range scenes {
				c := scenes[sceneIndex]
				if c.Map != name {
					continue
				}
				if os.Getenv("GD_VISUAL_SIMULATE") == "1" {
					fresh, e := mapdata.LoadMap(wf, mapdata.MapName(name))
					if e != nil {
						t.Fatal(e)
					}
					g = newGame(fresh, opts)
					g.setGammaLevel(0)
					g.viewW, g.viewH = width, height
					g.geometryAspectY = 1
					for i := 0; i < c.Tic; i++ {
						g.runGameplayTic(moveCmd{}, false, false)
					}
					writeVisualSimulationState(t, g, filepath.Join(out, c.Name+"-gd-state.csv"))
				}
				if demo != nil {
					fresh, e := mapdata.LoadMap(wf, mapdata.MapName(name))
					if e != nil {
						t.Fatal(e)
					}
					opts.DemoScript = demo
					g = newGame(fresh, opts)
					g.setGammaLevel(0)
					g.viewW, g.viewH = width, height
					g.geometryAspectY = 1
					for i := 0; i < c.Tic; i++ {
						if e := g.advanceDemoTic(); e != nil {
							t.Fatalf("demo tic %d: %v", i, e)
						}
					}
					if g.demoIntermissionActive || g.demoFinaleActive {
						t.Fatal("demo snapshot exceeds current level")
					}
					if c.DeathTics > 0 {
						g.inventory.InvulnTics = 0
						g.invulnerable = false
						g.damagePlayer(10000, "visual death probe")
						g.statusHasAttacker = false
						for i := 0; i < c.DeathTics; i++ {
							g.runGameplayTic(moveCmd{}, false, false)
						}
					}
					g.inventory.InvulnTics = 0
					g.inventory.LightAmpTics = 0
					g.syncRenderState()
					g.prepareRenderState()
					c.X, c.Y, c.Z = float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit, g.playerEyeZ()
					c.Angle, c.LevelTime, c.Dead = g.p.angle, g.worldTic, g.isDead
					c.BodyZ, c.ViewHeight, c.FloorZ, c.CeilingZ = float64(g.p.z)/fracUnit, float64(g.p.viewHeight)/fracUnit, float64(g.p.floorz)/fracUnit, float64(g.p.ceilz)/fracUnit
					scenes[sceneIndex] = c
				}
				g.renderPX, g.renderPY, g.renderAngle = c.X, c.Y, c.Angle
				g.p.x, g.p.y = int64(c.X*fracUnit), int64(c.Y*fracUnit)
				g.playerViewZ = int64(c.Z * fracUnit)
				// Fixed cameras have no body simulation; anchor their body below
				// the eye. Demo captures retain the independently simulated body.
				if demo == nil {
					g.p.z = int64((c.Z - 41) * fracUnit)
				}
				if g.playerEyeZ() != c.Z {
					t.Fatalf("camera eye Z=%g, want %g", g.playerEyeZ(), c.Z)
				}
				if demo == nil {
					g.worldTic = c.Tic
				}
				g.renderAlpha = 0
				for _, backend := range []struct {
					name    string
					enabled bool
					dst     *ebiten.Image
				}{{"cpu", false, cpu}, {"gpu", true, gpu}} {
					g.ensureWallLayer()
					clear(g.wallPix)
					backend.dst.Fill(color.Black)
					g.opts.GPURenderer = backend.enabled
					var wallTrace *os.File
					if !backend.enabled && os.Getenv("GD_VISUAL_TRACE") == "1" {
						wallTrace, err = os.Create(filepath.Join(out, c.Name+"-cpu-walls.csv"))
						if err != nil {
							t.Fatal(err)
						}
						fmt.Fprintln(wallTrace, "seg,x1,x2,scale,step,distance,offset")
						g.wallRangeTrace = func(seg, x1, x2 int, s classicWallSampler) {
							fmt.Fprintf(wallTrace, "%d,%d,%d,%d,%d,%d,%d\n", seg, x1, x2, s.scale, s.scaleStep, s.distance, s.offset)
						}
					}
					g.drawDoomBasic3D(backend.dst)
					g.wallRangeTrace = nil
					if wallTrace != nil {
						if err := wallTrace.Close(); err != nil {
							t.Fatal(err)
						}
					}
					if !backend.enabled && os.Getenv("GD_VISUAL_TRACE") == "1" {
						writeVisualSpanTrace(t, g, c, filepath.Join(out, c.Name+"-cpu-spans.csv"))
					}
					if backend.enabled && (g.gpu == nil || g.gpu.failed) {
						t.Fatal("GPU renderer fell back")
					}
					pixels := make([]byte, width*height*4)
					backend.dst.ReadPixels(pixels)
					f, err := os.Create(filepath.Join(out, c.Name+"-"+backend.name+".png"))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(f, &image.RGBA{Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height)})
					closeErr := f.Close()
					if err != nil {
						t.Fatal(err)
					}
					if closeErr != nil {
						t.Fatal(closeErr)
					}
				}
				episode, mapnum := 1, 1
				if strings.HasPrefix(name, "MAP") {
					fmt.Sscanf(name, "MAP%d", &mapnum)
				} else {
					fmt.Sscanf(name, "E%dM%d", &episode, &mapnum)
				}
				bright := 0
				if fullbright {
					bright = 1
				}
				fmt.Fprintf(manifest, "%s %s %d %d %.17g %.17g %.17g %d %d %d %d\n", c.Name, name, episode, mapnum, c.X, c.Y, c.Z, c.Angle, c.Tic, bright, c.DeathTics)
			}
			cpu.Deallocate()
			gpu.Deallocate()
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(scenes, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "scenes.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("captured %d fixed-camera scenes", len(scenes))
}

func visualProbeScenes(g *game) []visualProbeScene {
	var scenes []visualProbeScene
	name := string(g.m.Name)
	add := func(label, category string, x, y, z float64, angle uint32, tics []int) {
		for _, tic := range tics {
			scenes = append(scenes, visualProbeScene{Name: fmt.Sprintf("%s-%s-t%03d", name, label, tic), Map: name, X: x, Y: y, Z: z, Angle: angle, Tic: tic, Category: category})
		}
	}
	for i := 0; i < 4; i++ {
		add(fmt.Sprintf("spawn-%d", i), "spawn", g.renderPX, g.renderPY, g.playerEyeZ(), g.renderAngle+uint32(i)*0x40000000, []int{0})
	}
	counts := map[string]int{}
	for li, ld := range g.m.Linedefs {
		a, b := g.m.Vertexes[ld.V1], g.m.Vertexes[ld.V2]
		dx, dy := float64(b.X)-float64(a.X), float64(b.Y)-float64(a.Y)
		length := math.Hypot(dx, dy)
		if length < 32 {
			continue
		}
		for side, idx := range ld.SideNum {
			if idx < 0 {
				continue
			}
			sd := g.m.Sidedefs[idx]
			sec := g.m.Sectors[sd.Sector]
			if sec.CeilingHeight-sec.FloorHeight < 48 {
				continue
			}
			category := ""
			if ld.Special == 48 {
				category = "scroll"
			} else if len(g.flatTextureAnimRefs[sec.FloorPic].frames) > 1 || len(g.wallTextureAnimRefs[sd.Mid].frames) > 1 || len(g.wallTextureAnimRefs[sd.Top].frames) > 1 || len(g.wallTextureAnimRefs[sd.Bottom].frames) > 1 {
				category = "animated"
			} else if texturePresent(sd.Mid) && ld.SideNum[1-side] >= 0 {
				category = "masked"
			} else if sd.TextureOffset != 0 || sd.RowOffset != 0 {
				category = "offset"
			} else if ld.Flags&(mlDontPegTop|mlDontPegBottom) != 0 {
				category = "pegged"
			} else if sec.CeilingPic == "F_SKY1" {
				category = "sky"
			} else if texturePresent(sd.Top) || texturePresent(sd.Bottom) {
				category = "tiers"
			}
			if category == "" || counts[category] >= 2 {
				continue
			}
			sign := 1.0
			if side == 1 {
				sign = -1
			}
			mx, my := (float64(a.X)+float64(b.X))/2, (float64(a.Y)+float64(b.Y))/2
			for _, distance := range []float64{64, 32, 16} {
				x, y := mx+sign*dy/length*distance, my-sign*dx/length*distance
				// Quantize once to Doom's fixed point, then send identical coordinates.
				x, y = math.Round(x*fracUnit)/fracUnit, math.Round(y*fracUnit)/fracUnit
				if g.sectorAt(int64(x*fracUnit), int64(y*fracUnit)) != int(sd.Sector) {
					continue
				}
				angle := uint32(int64(math.Atan2(my-y, mx-x) / (2 * math.Pi) * 4294967296))
				tics := []int{0}
				if category == "scroll" || category == "animated" {
					tics = []int{0, 1, 7, 8, 9, 35, 64}
				}
				eyeHeight := 41.0
				if category == "animated" {
					// Keep animated floors visible even in narrow passages.
					eyeHeight = 16
				}
				add(fmt.Sprintf("%s-line%d-side%d", category, li, side), category, x, y, float64(sec.FloorHeight)+eyeHeight, angle, tics)
				counts[category]++
				break
			}
		}
	}
	return scenes
}

// Reconstruct span starts from the actual completed frame's visplanes. This
// test-only trace shares the sampler with the draw and adds no runtime hooks.
func writeVisualSpanTrace(t *testing.T, g *game, c visualProbeScene, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fmt.Fprintln(f, "plane,height,flat,y,x1,x2,u,v,du,dv")
	a := angleToRadians(c.Angle)
	for id, pl := range g.plane3DPool[:g.plane3DPoolUsed] {
		if pl.key.sky {
			continue
		}
		for _, sp := range makePlane3DSpansWithScratch(pl, g.viewH, nil, nil) {
			s, ok := g.planeRowRenderState(sp.y, pl.key, c.Z, c.X, c.Y, math.Cos(a), math.Sin(a), float64(g.viewW)/2, float64(g.viewW)/2, float64(g.viewW)/2, float64(g.viewH)/2)
			if !ok {
				continue
			}
			u, v := s.textureStart(sp.x1)
			fmt.Fprintf(f, "%d,%d,%s,%d,%d,%d,%d,%d,%d,%d\n", id, floorFixed(pl.key.height), g.resolveAnimatedFlatName(g.flatNameByID(pl.key.flatID)), sp.y, sp.x1, sp.x2, u, v, s.stepWXFixed, s.stepWYFixed)
		}
	}
	if err := f.Sync(); err != nil {
		t.Fatal(err)
	}
}

func writeVisualSimulationState(t *testing.T, g *game, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fmt.Fprintf(f, "player,%d,%d,%d,%d,%d\n", g.p.x, g.p.y, g.p.z, g.p.momx, g.p.momy)
	for i := range g.m.Sectors {
		floor, ceil, _ := g.sectorHeightSnapshot(i)
		a, b := g.boomPlaneOffset(i, true), g.boomPlaneOffset(i, false)
		fmt.Fprintf(f, "sector,%d,%d,%d,%d,%d,%d,%d\n", i, floor, ceil, a.x, a.y, b.x, b.y)
	}
	for i, side := range g.m.Sidedefs {
		off := g.boomWallOffsets(i)
		for _, line := range g.m.Linedefs {
			if int(line.SideNum[0]) == i {
				off.x += floorFixed(wallSpecialScrollXOffset(line.Special, g.worldTic))
			}
		}
		fmt.Fprintf(f, "side,%d,%d,%d\n", i, boomFixed(int64(side.TextureOffset)*fracUnit+off.x), boomFixed(int64(side.RowOffset)*fracUnit+off.y))
	}
}
