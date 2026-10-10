package doomruntime

import (
	"context"
	"testing"

	"gddoom/internal/render/levelmesh"
	"gddoom/internal/runtimecfg"
	"github.com/hajimehoshi/ebiten/v2"
)

type menuFontFallbackRecorder struct {
	*game
	text         string
	x, y, sx, sy float64
}

func (r *menuFontFallbackRecorder) sessionDrawHUTextAt(_ *ebiten.Image, text string, x, y, sx, sy float64) {
	r.text, r.x, r.y, r.sx, r.sy = text, x, y, sx, sy
}

func TestFrontendMultiplayerLabelRetainsMinimalHostFallback(t *testing.T) {
	for _, inGame := range []bool{false, true} {
		t.Run(map[bool]string{false: "title", true: "pause"}[inGame], func(t *testing.T) {
			r := &menuFontFallbackRecorder{game: &game{}}
			sg := &sessionGame{rt: r, frontend: frontendState{Active: true, MenuActive: true, InGame: inGame, Mode: frontendModeTitle}}
			sg.opts.AuthorityJoin = func(context.Context, runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
				return runtimecfg.AuthorityJoinResult{}, nil
			}
			sg.drawFrontendContents(nil, 640, 480)
			if r.text != "MULTIPLAYER" || r.x != 194 || r.y != 200 || r.sx != 2 || r.sy != 2 {
				t.Fatalf("missing fallback or wrong logical row: %+v", r)
			}
		})
	}
}

func TestFrontendLargeMenuPatchUsesSharedNativeGeometry(t *testing.T) {
	patch := WallTexture{Width: 150, Height: 15, OffsetX: 2, OffsetY: 1, RGBA: make([]byte, 150*15*4)}
	var commands []levelmesh.Patch
	sg := &sessionGame{opts: Options{MenuPatchBank: map[string]WallTexture{"M_MULTI": patch}}, nativePatches: &commands}
	if !sg.drawMenuPatchAlpha(nil, "M_MULTI", 97, 160, 2, 7, 11, false, .4) || len(commands) != 1 {
		t.Fatal("generated menu artwork did not reach native patch collection")
	}
	p := commands[0]
	if p.X != 197 || p.Y != 329 || p.W != 300 || p.H != 30 || p.Alpha != .4 || &p.Texture.RGBA[0] != &patch.RGBA[0] {
		t.Fatalf("native menu geometry/opacity mismatch: %+v", p)
	}
	if sg.menuPatchImages != nil {
		t.Fatal("native patch collection allocated an Ebiten image")
	}
}
