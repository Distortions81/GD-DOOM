package doomruntime

import (
	"bytes"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/levelmesh"
)

func TestNativeMeshMaterialsKeepSharedAnimationAndSwitchSamples(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true, TextureAnimCrossfadeFrames: 7,
		FlatBank: map[string][]byte{}, FlatBankIndexed: map[string][]byte{}, WallTexBank: map[string]WallTexture{}},
		m: &mapdata.Map{Sidedefs: []mapdata.Sidedef{{Mid: "SW2TEST"}, {Mid: "SW2TEST"}}}}
	for i, name := range []string{"FRAMEA", "FRAMEB"} {
		pixels, indexed := make([]byte, 64*64*4), make([]byte, 64*64)
		for p := range indexed {
			indexed[p] = byte(i + 1)
			copy(pixels[p*4:p*4+4], []byte{byte(50 + i*150), 60, 90, 255})
		}
		g.opts.FlatBank[name], g.opts.FlatBankIndexed[name] = pixels, indexed
		g.opts.WallTexBank[name] = WallTexture{Width: 64, Height: 64, RGBA: pixels, Indexed: indexed}
	}
	g.opts.WallTexBank["SW1TEST"], g.opts.WallTexBank["SW2TEST"] = g.opts.WallTexBank["FRAMEA"], g.opts.WallTexBank["FRAMEB"]
	refs := buildTextureAnimRefsFromSequences(map[string][]string{"FRAMEA": {"FRAMEA", "FRAMEB"}})
	g.flatTextureAnimRefs, g.wallTextureAnimRefs = refs, refs
	n := &NativeMeshGame{g: g}
	r := &experimentalMeshRenderer{materials: map[meshMaterialKey]levelmesh.Texture{}}
	for tic := 0; tic < 32; tic++ {
		for _, alpha := range []float64{0, .25, .75, 1} {
			g.worldTic, g.renderAlpha = tic, alpha
			clear(r.materials)
			flat, _ := g.flatTextureBlend("FRAMEA")
			wall, _ := g.wallTextureBlend("FRAMEA", -1, switchTextureSlotMid)
			for _, kind := range []levelmesh.Kind{levelmesh.Floor, levelmesh.Ceiling, levelmesh.Middle, levelmesh.Upper, levelmesh.Lower} {
				got := g.meshMaterial(r, levelmesh.Triangle{Texture: "FRAMEA", Kind: kind, Sidedef: -1})
				if kind == levelmesh.Floor || kind == levelmesh.Ceiling {
					if !bytes.Equal(got.RGBA, flat.fromRGBA) || !bytes.Equal(got.BlendRGBA, flat.toRGBA) || got.BlendAlpha != flat.alpha || !bytes.Equal(got.BlendIndexed, flat.toIndexed) {
						t.Fatal("plane lost shared fractional animation sample")
					}
				} else if !bytes.Equal(got.RGBA, wall.from.RGBA) || got.BlendAlpha != wall.alpha || (wall.to != nil && !bytes.Equal(got.BlendRGBA, wall.to.RGBA)) {
					t.Fatal("wall lost shared fractional animation sample")
				}
			}
		}
	}
	g.worldTic, g.renderAlpha = 13, .5
	g.switchTextureBlends = []switchTextureBlend{{sidedef: 0, slot: switchTextureSlotMid, from: "SW1TEST", to: "SW2TEST", startTic: 10}, {sidedef: 1, slot: switchTextureSlotMid, from: "SW1TEST", to: "SW2TEST", startTic: 11}}
	clear(r.materials)
	a := n.fixedWorldTexture(g.meshMaterial(r, levelmesh.Triangle{Texture: "SW2TEST", Kind: levelmesh.Middle, Sidedef: 0}))
	b := n.fixedWorldTexture(g.meshMaterial(r, levelmesh.Triangle{Texture: "SW2TEST", Kind: levelmesh.Middle, Sidedef: 1}))
	if !a.HasBlend() || !b.HasBlend() || a.BlendAlpha == b.BlendAlpha || a.BlendInstance == b.BlendInstance {
		t.Fatal("independent switch phases were merged")
	}
	g.opts.TextureAnimCrossfadeFrames = 0
	clear(r.materials)
	if g.meshMaterial(r, levelmesh.Triangle{Texture: "FRAMEA", Kind: levelmesh.Floor}).HasBlend() || g.meshMaterial(r, levelmesh.Triangle{Texture: "SW2TEST", Kind: levelmesh.Middle, Sidedef: 0}).HasBlend() {
		t.Fatal("disabled crossfade still has a second frame")
	}
}
