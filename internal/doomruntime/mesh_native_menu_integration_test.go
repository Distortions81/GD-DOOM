//go:build integration

package doomruntime

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/sessionflow"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

// Compare collected native commands with real Ebiten frontend draws, including
// font offsets/scaling, faded SoundFont entries, list scrolling and skull blink.
func TestNativeMenuRenderingMatchesEbiten(t *testing.T) {
	if os.Getenv("GD_MENU_INTEGRATION") == "" {
		t.Skip("set GD_MENU_INTEGRATION=1 for shared-menu pixel comparison")
	}
	// Ebiten cannot create images after RunGame returns. Keep its reference
	// window in a child, so subsequent native fixtures can initialize normally.
	if os.Getenv("GD_MENU_REFERENCE_CHILD") == "" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestNativeMenuRenderingMatchesEbiten$", "-test.count=1")
		cmd.Env = append(os.Environ(), "GD_MENU_REFERENCE_CHILD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("menu reference: %v\n%s", err, output)
		}
		return
	}
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		wf, err := wad.Open(findDOOM1WAD(t))
		if err != nil {
			t.Fatal(err)
		}
		set, err := doomtex.LoadFromWAD(wf)
		if err != nil {
			t.Fatal(err)
		}
		opts := Options{Episodes: []int{1, 2, 3}, InputBindings: runtimecfg.DefaultInputBindings(), MenuPatchBank: map[string]WallTexture{}, IntermissionPatchBank: map[string]WallTexture{}, MessageFontBank: map[rune]WallTexture{}}
		for _, lump := range wf.Lumps {
			if !strings.HasPrefix(lump.Name, "M_") && !strings.HasPrefix(lump.Name, "STCFN") && !strings.HasPrefix(lump.Name, "HELP") {
				continue
			}
			rgba, w, h, x, y, err := set.BuildPatchRGBA(lump.Name, 0)
			if err != nil {
				continue
			}
			tex := WallTexture{RGBA: rgba, Width: w, Height: h, OffsetX: x, OffsetY: y}
			if strings.HasPrefix(lump.Name, "M_") {
				opts.MenuPatchBank[lump.Name] = tex
			} else if strings.HasPrefix(lump.Name, "HELP") {
				opts.IntermissionPatchBank[lump.Name] = tex
			} else {
				for ch := rune(33); ch <= 95; ch++ {
					if lump.Name == fmt.Sprintf("STCFN%03d", ch) {
						opts.MessageFontBank[ch] = tex
					}
				}
			}
		}
		opts.MusicBackend = "impsynth"
		opts.SFXVolume, opts.MusicVolume, opts.MouseLookSpeed = .7, .5, 1
		opts.MusicSoundFontPath = "soundfonts/a-very-long-custom-soundfont-name.sf2"
		opts.MusicPlayerCatalog = []runtimecfg.MusicPlayerWAD{{Key: "doom", Label: "THE ULTIMATE DOOM", Episodes: []runtimecfg.MusicPlayerEpisode{{Label: "EPISODE 1", Tracks: []runtimecfg.MusicPlayerTrack{{Label: "E1M4 - COMMAND CONTROL", MusicName: "KITCHEN ACE (AND TAKING NAMES)"}}}}}}
		modes := []sessionflow.FrontendMode{frontendModeTitle, frontendModeEpisode, frontendModeSkill, frontendModeOptions, frontendModeSound, frontendModeKeybinds, frontendModeMusicPlayer, frontendModeReadThis, frontendModeSaveLoad, frontendModeNone}
		for _, mode := range modes {
			for _, tic := range []int{0, menuSkullBlinkTics} {
				view := NativeMenuView{State: sessionflow.Frontend{Active: true, MenuActive: true, Mode: mode, ItemOn: 3, EpisodeOn: 1, SkillOn: 2, OptionsOn: 4, SoundOn: 3, SaveLoadOn: 11, SaveLoadSaving: true}, Frame: tic, BindingRow: 19, BindingSlot: 1, BindingCapture: tic > 0, MusicRow: 2, NowPlaying: "E1M4 - COMMAND CONTROL\nSONG: KITCHEN ACE (AND TAKING NAMES)", Messages: true, ShowFPS: true, HUDScale: 3}
				if mode == frontendModeNone {
					prompt, _ := NativeStartQuitPrompt(tic)
					view.QuitLines = prompt.Lines
				}
				for i := 0; i < 15; i++ {
					view.Slots = append(view.Slots, NativeSaveSlot{Slot: i})
				}
				r := NewNativeMenuRenderer()
				patches := slices.Clone(r.Draw(opts, view))
				if r.session.menuPatchImages != nil || r.session.intermissionImages != nil {
					t.Fatal("native extraction allocated Ebiten artwork")
				}
				expectedSG := r.session
				expectedSG.nativePatches = nil
				expectedSG.nativeSaveSlots = view.Slots
				const width, height = 1280, 800
				want := ebiten.NewImage(width, height)
				want.Fill(color.Black)
				expectedSG.drawFrontend(want)
				if mode == frontendModeNone {
					expectedSG.drawQuitPrompt(want)
				}
				got := ebiten.NewImage(width, height)
				got.Fill(color.Black)
				for _, p := range patches {
					tex := menuComparisonImage(&expectedSG, p.Texture)
					op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
					op.GeoM.Scale(4*p.W/float64(p.Texture.Width), 4*p.H/float64(p.Texture.Height))
					op.GeoM.Translate(4*p.X, 4*p.Y)
					if p.Alpha > 0 && p.Alpha < 1 {
						op.ColorScale.ScaleAlpha(float32(p.Alpha))
					}
					got.DrawImage(tex, op)
				}
				actual, expected := make([]byte, width*height*4), make([]byte, width*height*4)
				got.ReadPixels(actual)
				want.ReadPixels(expected)
				differences := 0
				for i := 0; i < len(actual); i += 4 {
					if slices.Equal(actual[i:i+4], expected[i:i+4]) {
						continue
					}
					differences++
					if !menuPixelNear(actual, expected, i, width, height) || !menuPixelNear(expected, actual, i, width, height) {
						t.Fatalf("mode=%d tic=%d differs beyond one raster pixel at %d,%d: native=%v Ebiten=%v", mode, tic, (i/4)%width, (i/4)/width, actual[i:i+4], expected[i:i+4])
					}
				}
				if differences > width*height/100 {
					t.Fatalf("mode=%d tic=%d changed more than 1%% of pixels: %d", mode, tic, differences)
				}

				t.Logf("mode=%d tic=%d: matching layout, %d subpixel edge differences, %d patch commands", mode, tic, differences, len(patches))
			}
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}

// Converting scaled glyph sizes back into a transform can shift the sampled
// texel at a fractional-scale boundary through floating-point rounding. Require
// matching colors within a single raster pixel, in both directions; large
// offsets, missing glyphs, opacity mistakes and wrong menu artwork still fail.
func menuPixelNear(a, b []byte, i, width, height int) bool {
	x, y := (i/4)%width, (i/4)/width
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if x+dx < 0 || x+dx >= width || y+dy < 0 || y+dy >= height {
				continue
			}
			j := ((y+dy)*width + x + dx) * 4
			matches := true
			for ch := 0; ch < 4; ch++ {
				d := int(a[i+ch]) - int(b[j+ch])
				if d < -1 || d > 1 {
					matches = false
					break
				}
			}
			if matches {
				return true
			}
		}
	}
	return false
}

// Reuse the same GPU source image for equal source artwork to isolate the
// native command conversion from atlas-dependent nearest-sampling precision.
func menuComparisonImage(sg *sessionGame, texture levelmesh.Texture) *ebiten.Image {
	for ch, p := range sg.g.opts.MessageFontBank {
		if &p.RGBA[0] == &texture.RGBA[0] {
			if img := sg.g.messageFontImg[ch]; img != nil {
				return img
			}
		}
	}
	for name, p := range sg.opts.MenuPatchBank {
		if &p.RGBA[0] == &texture.RGBA[0] {
			if img := sg.menuPatchImages[name]; img != nil {
				return img
			}
		}
	}
	for name, p := range sg.opts.IntermissionPatchBank {
		if &p.RGBA[0] == &texture.RGBA[0] {
			if img := sg.intermissionImages[name]; img != nil {
				return img
			}
		}
	}
	img := ebiten.NewImage(texture.Width, texture.Height)
	img.WritePixels(texture.RGBA)
	return img
}
