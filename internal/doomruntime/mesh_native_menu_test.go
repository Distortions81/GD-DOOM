package doomruntime

import (
	"math"
	"strings"
	"testing"

	"gddoom/internal/runtimecfg"
	"gddoom/internal/sessionflow"
)

func TestNativeBindingDrawingUsesFilteredActionsAndMatchingScroll(t *testing.T) {
	opts := Options{InputBindings: runtimecfg.DefaultInputBindings(), MessageFontBank: map[rune]WallTexture{}}
	for ch := rune(33); ch <= 95; ch++ {
		pixels := make([]byte, 4*7*4)
		pixels[0] = byte(ch)
		opts.MessageFontBank[ch] = WallTexture{Width: 4, Height: 7, RGBA: pixels}
	}
	var actions []int
	for _, def := range NativeBindingDefinitions(opts.InputBindings) {
		if def.Label != "PUSH TO TALK" {
			actions = append(actions, def.ID)
		}
	}
	r := NewNativeMenuRenderer()
	for _, selected := range []int{10, len(actions) - 1} {
		view := NativeMenuView{State: sessionflow.Frontend{Active: true, MenuActive: true, Mode: NativeMenuBindings}, BindingActions: actions, BindingRow: selected}
		patches := r.Draw(opts, view)
		start, count := NativeBindingMenuWindow(selected, len(actions))
		for row := start; row < min(start+count, len(actions)); row++ {
			var label, primary string
			for _, p := range patches {
				if p.Y != float64(42+16*(row-start)) {
					continue
				}
				if p.X < 188 {
					label += string(rune(p.Texture.RGBA[0]))
				} else if p.X < 258 {
					primary += string(rune(p.Texture.RGBA[0]))
				}
			}
			action := bindingAction(actions[row])
			wantLabel := strings.ReplaceAll(bindingActionLabel(action), " ", "")
			wantValue := bindingSlotLabel(bindingValue(opts.InputBindings, action)[0])
			if row == selected {
				wantValue = "[" + wantValue + "]"
			}
			if label != wantLabel || primary != strings.ReplaceAll(wantValue, " ", "") {
				t.Fatalf("selected=%d row=%d: label=%q value=%q want %q/%q", selected, row, label, primary, wantLabel, wantValue)
			}
		}
	}
}

func TestNativeMouseSensitivityMatchesMainSliderSpacing(t *testing.T) {
	for _, glyphWidth := range []int{4, 8, 12} {
		opts := Options{SourcePortMode: true, MessageFontBank: map[rune]WallTexture{}}
		for ch := rune(33); ch <= 95; ch++ {
			opts.MessageFontBank[ch] = WallTexture{Width: glyphWidth, Height: 1, RGBA: make([]byte, glyphWidth*4)}
		}
		native := NewNativeMenuRenderer()
		for _, speed := range []float64{1.0 / 6, .27, .5, .91, 1.5} {
			for _, dir := range []int{-1, 1} {
				for _, cycle := range []bool{false, true} {
					main := &sessionGame{opts: opts, g: &game{opts: opts}}
					main.g.opts.MouseLookSpeed = speed
					main.rt = main.g
					if cycle {
						main.frontendCycleMouseSensitivity()
					} else {
						main.frontendChangeMouseSensitivity(dir)
					}
					got := native.NextMouseSensitivity(opts, speed, dir, cycle)
					want := main.g.opts.MouseLookSpeed
					if math.Abs(got-want) > 1e-12 {
						t.Fatalf("font=%d speed=%v dir=%d cycle=%v: native=%v main=%v", glyphWidth, speed, dir, cycle, got, want)
					}
				}
			}
		}
	}
}
