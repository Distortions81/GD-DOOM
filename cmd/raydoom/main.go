//go:build raylib && cgo && !js

package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	wadPath := flag.String("wad", "DOOM1.WAD", "IWAD path")
	mapName := flag.String("map", "E1M3", "level name")
	width := flag.Int("width", 1280, "window width")
	height := flag.Int("height", 800, "window height")
	fps := flag.Int("fps", 144, "frame limit (0 = uncapped)")
	frames := flag.Int("frames", 0, "exit after this many frames (0 = interactive)")
	capture := flag.String("capture", "", "PNG screenshot path, taken on final frame")
	pose := flag.String("camera", "", "inspection pose x,y,eye-z,yaw-degrees")
	modeFlag := flag.String("mode", "textured", "textured, sectors, or wireframe")
	textureScale := flag.Int("texture-scale", 2, "texture upload scale (1 or 2; nearest-neighbor enlargement)")
	textureFilter := flag.String("texture-filter", "anisotropic", "nearest, trilinear, or anisotropic (8x)")
	lightingFlag := flag.String("lighting", "doom", "doom, sector, or fullbright")
	msaa := flag.Bool("msaa", true, "request 4x multisample antialiasing for geometry edges and seams")
	flag.Parse()
	lightingMode := raymesh.LightingMode(*lightingFlag)
	if err := lightingMode.Validate(); err != nil {
		return err
	}
	textureOptions := raymesh.TextureOptions{Scale: *textureScale, Filter: raymesh.TextureFilter(*textureFilter)}
	if err := textureOptions.Validate(); err != nil {
		return err
	}
	if *width < 320 || *height < 200 || *fps < 0 || *frames < 0 {
		return fmt.Errorf("invalid window size, frame limit, or frame count")
	}
	mode := levelmesh.Mode(*modeFlag)
	if mode != levelmesh.Textured && mode != levelmesh.Sectors && mode != levelmesh.Wireframe {
		return fmt.Errorf("invalid mesh mode %q", mode)
	}
	if *capture != "" && *frames == 0 {
		return fmt.Errorf("-capture requires -frames")
	}
	wf, err := wad.Open(*wadPath)
	if err != nil {
		return err
	}
	m, err := mapdata.LoadMap(wf, mapdata.MapName(strings.ToUpper(*mapName)))
	if err != nil {
		return err
	}
	opts, err := loadAssets(wf)
	if err != nil {
		return err
	}
	opts.Width, opts.Height = *width, *height
	game := doomruntime.NewNativeMeshGame(m, opts)
	if *pose != "" {
		var x, y, z, yaw float64
		if n, err := fmt.Sscanf(*pose, "%f,%f,%f,%f", &x, &y, &z, &yaw); err != nil || n != 4 {
			return fmt.Errorf("-camera needs x,y,eye-z,yaw-degrees")
		}
		if err := game.SetPose(x, y, z, yaw*math.Pi/180); err != nil {
			return err
		}
	}
	rl.SetTraceLogLevel(rl.LogWarning)
	windowFlags := uint32(rl.FlagWindowResizable)
	aaLabel := "off"
	if *msaa {
		// Raylib's MSAA hint must precede InitWindow. Draw directly to its
		// multisampled framebuffer; ordinary render textures are single-sample.
		windowFlags |= rl.FlagMsaa4xHint
		aaLabel = "4x MSAA requested"
	}
	rl.SetConfigFlags(windowFlags)
	rl.InitWindow(int32(*width), int32(*height), "GD-DOOM | Raylib GPU mesh experiment")
	if !rl.IsWindowReady() {
		return fmt.Errorf("Raylib window could not initialize")
	}
	defer rl.CloseWindow()
	rl.SetTargetFPS(int32(*fps))
	if *frames == 0 {
		rl.DisableCursor()
	}
	renderer, err := raymesh.NewRendererWithTextureOptions(textureOptions)
	if err != nil {
		return err
	}
	defer renderer.Close()
	if err := renderer.SetLightingMode(lightingMode); err != nil {
		return err
	}
	renderer.SetLightRamp(game.LightRamp())
	const tic = 1.0 / 35
	accumulator := 0.0
	last := time.Now()
	use, fire := false, false
	weapon := 0
	yawPending := 0.0
	for frame := 0; !rl.WindowShouldClose(); frame++ {
		now := time.Now()
		accumulator += math.Min(now.Sub(last).Seconds(), 0.25)
		last = now
		if rl.IsKeyPressed(rl.KeyF7) {
			switch mode {
			case levelmesh.Textured:
				mode = levelmesh.Sectors
			case levelmesh.Sectors:
				mode = levelmesh.Wireframe
			default:
				mode = levelmesh.Textured
			}
		}
		if rl.IsKeyPressed(rl.KeyF8) {
			switch textureOptions.Filter {
			case raymesh.Nearest:
				textureOptions.Filter = raymesh.Trilinear
			case raymesh.Trilinear:
				textureOptions.Filter = raymesh.Anisotropic
			default:
				textureOptions.Filter = raymesh.Nearest
			}
			if err := renderer.SetTextureFilter(textureOptions.Filter); err != nil {
				return err
			}
		}
		if rl.IsKeyPressed(rl.KeyF9) {
			switch lightingMode {
			case raymesh.DoomLighting:
				lightingMode = raymesh.SectorLighting
			case raymesh.SectorLighting:
				lightingMode = raymesh.FullbrightLighting
			default:
				lightingMode = raymesh.DoomLighting
			}
			if err := renderer.SetLightingMode(lightingMode); err != nil {
				return err
			}
		}
		if rl.IsKeyPressed(rl.KeyR) {
			fresh, err := mapdata.LoadMap(wf, m.Name)
			if err != nil {
				return err
			}
			game = doomruntime.NewNativeMeshGame(fresh, opts)
			accumulator, yawPending, weapon, use, fire = 0, 0, 0, false, false
		}
		use = use || rl.IsKeyDown(rl.KeySpace) || rl.IsKeyDown(rl.KeyE)
		fire = rl.IsMouseButtonDown(rl.MouseButtonLeft) || rl.IsKeyDown(rl.KeyLeftControl)
		for slot := 1; slot <= 7; slot++ {
			if rl.IsKeyPressed(int32(rl.KeyOne + slot - 1)) {
				weapon = slot
			}
		}
		if *frames == 0 {
			yawPending -= float64(rl.GetMouseDelta().X) * 0.0025
		}
		for accumulator >= tic {
			in := doomruntime.NativeMeshInput{Forward: axis(rl.KeyW, rl.KeyS) + axis(rl.KeyUp, rl.KeyDown), Side: axis(rl.KeyD, rl.KeyA), Turn: axis(rl.KeyLeft, rl.KeyRight), Run: rl.IsKeyDown(rl.KeyLeftShift), Use: use, Fire: fire, WeaponSlot: weapon, YawDelta: yawPending}
			game.Tick(in)
			weapon, yawPending = 0, 0
			use = rl.IsKeyDown(rl.KeySpace) || rl.IsKeyDown(rl.KeyE)
			accumulator -= tic
		}
		alpha := accumulator / tic
		if *frames > 0 {
			alpha = 1
		} // Deterministic still camera for capture checks.
		snapshot := game.Frame(alpha)
		renderer.Sync(snapshot.Triangles, game.Texture, game.Light, mode)
		renderer.SetFullbright(snapshot.Fullbright)
		w, h := rl.GetScreenWidth(), rl.GetScreenHeight()
		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(42, 49, 65, 255))
		renderer.Draw(snapshot.Camera, w, h, mode)
		stats := renderer.Stats()
		rl.DrawText(fmt.Sprintf("Raylib GPU | %s | %d tris | %d batches | F7 cycle", mode, stats.Triangles, stats.DrawCalls), 8, 8, 18, rl.White)
		rl.DrawText(fmt.Sprintf("Resident meshes: %d | uploads: %d | buffer updates: %d", stats.ResidentMeshes, stats.MeshUploads, stats.BufferUpdates), 8, 30, 16, rl.White)
		rl.DrawText(fmt.Sprintf("Textures: %dx | %s | F8 filter", textureOptions.Scale, textureOptions.Filter), 8, 50, 16, rl.White)
		rl.DrawText(fmt.Sprintf("Lighting: %s | F9 cycle | AA: %s", lightingMode, aaLabel), 8, 70, 16, rl.White)
		rl.DrawRectangle(0, int32(h-48), int32(w), 48, rl.NewColor(12, 16, 24, 255))
		rl.DrawText(fmt.Sprintf("HEALTH %d  ARMOR %d  %s %d  | WASD move | mouse/arrows turn | E/Space use | ESC quit", snapshot.Health, snapshot.Armor, strings.ToUpper(snapshot.Weapon), snapshot.Ammo), 12, int32(h-34), 18, rl.White)
		if snapshot.Exited {
			rl.DrawText("LEVEL COMPLETE", int32(w/2-120), int32(h/2), 28, rl.White)
		}
		var captureErr error
		if *frames > 0 && frame+1 >= *frames && *capture != "" {
			captureErr = captureScreen(*capture)
		}
		rl.EndDrawing()
		if captureErr != nil {
			return captureErr
		}
		if *frames > 0 && frame+1 >= *frames {
			fmt.Printf("raylib-mesh map=%s frames=%d triangles=%d batches=%d resident=%d uploads=%d updates=%d texture-scale=%d texture-filter=%s lighting=%s msaa=%t\n", m.Name, frame+1, stats.Triangles, stats.DrawCalls, stats.ResidentMeshes, stats.MeshUploads, stats.BufferUpdates, textureOptions.Scale, textureOptions.Filter, lightingMode, *msaa)
			break
		}
	}
	return nil
}

func axis(positive, negative int32) int {
	v := 0
	if rl.IsKeyDown(positive) {
		v++
	}
	if rl.IsKeyDown(negative) {
		v--
	}
	return v
}

func loadAssets(wf *wad.File) (doomruntime.Options, error) {
	var colorMap []byte
	if lump, ok := wf.LumpByName("COLORMAP"); ok {
		var err error
		colorMap, err = wf.LumpData(lump)
		if err != nil {
			return doomruntime.Options{}, err
		}
	}
	set, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		return doomruntime.Options{}, err
	}
	flats, err := doomtex.LoadFlatsRGBA(wf, 0)
	if err != nil {
		return doomruntime.Options{}, err
	}
	palette, err := doomtex.LoadPaletteRGBA(wf, 0)
	if err != nil {
		return doomruntime.Options{}, err
	}
	walls := make(map[string]doomruntime.WallTexture)
	for _, name := range set.TextureNames() {
		pixels, w, h, err := set.BuildTextureRGBA(name, 0)
		if err != nil {
			return doomruntime.Options{}, err
		}
		walls[name] = doomruntime.WallTexture{RGBA: pixels, Width: w, Height: h}
	}
	return doomruntime.Options{NoMonsters: true, Invulnerable: true, SourcePortSectorLighting: true, MouseLookSpeed: 1, KeyboardTurnSpeed: 1, AutoWeaponSwitch: true, FlatBank: flats, WallTexBank: walls, DoomPaletteRGBA: palette, DoomColorMap: colorMap, DoomColorMapRows: len(colorMap) / 256, WallTextureAnimSequences: doomtex.LoadWallTextureAnimSequences(set, doomtex.DoomWallAnimDefs), FlatTextureAnimSequences: doomtex.LoadFlatAnimSequences(wf, doomtex.DoomFlatAnimDefs)}, nil
}

func captureScreen(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	img, err := raymesh.CaptureWindow()
	if err != nil {
		return err
	}
	defer rl.UnloadImage(img)
	if !rl.ExportImage(*img, path) {
		return fmt.Errorf("could not save screenshot %q", path)
	}
	return nil
}
