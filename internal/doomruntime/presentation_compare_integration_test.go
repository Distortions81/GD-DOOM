//go:build integration

package doomruntime

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestPresentationCapture(t *testing.T) {
	out := os.Getenv("GD_PRESENTATION_OUT")
	if out == "" {
		t.Skip("run scripts/presentation_compare.py")
	}
	wf, err := wad.Open(os.Getenv("GD_PRESENTATION_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatal(err)
	}
	palette, _ := ts.PaletteRGBA(0)
	palettes := make([][]byte, ts.PaletteCount())
	for i := range palettes {
		palettes[i], _ = ts.PaletteRGBA(i)
	}
	lump, _ := wf.LumpByName("COLORMAP")
	cm, _ := wf.LumpData(lump)
	bank := map[string]WallTexture{}
	load := func(name string) {
		rgba, w, h, ox, oy, err := ts.BuildPatchRGBA(name, 0)
		if err != nil {
			return
		}
		indexed, opaque, _, _, _, _, _ := ts.BuildPatchIndexedView(name)
		mask := make([]byte, len(opaque))
		for i, v := range opaque {
			if v {
				mask[i] = 1
			}
		}
		bank[name] = WallTexture{ClassicMasked: ts.BuildPatchMaskedPosts(name), RGBA: rgba, Width: w, Height: h, OffsetX: ox, OffsetY: oy, Indexed: indexed, OpaqueMask: mask}
	}
	for _, def := range weaponPspriteDefs {
		load(def.sprite)
	}
	for _, name := range []string{"STBAR", "STARMS", "STTPRCNT", "STTMINUS", "STFGOD0", "STFDEAD0", "STFB0", "STFB1", "STFB2", "STFB3"} {
		load(name)
	}
	for i := 0; i < 10; i++ {
		for _, prefix := range []string{"STTNUM", "STYSNUM", "STGNUM", "STKEYS"} {
			load(fmt.Sprintf("%s%d", prefix, i))
		}
	}
	for i := 0; i < 5; i++ {
		for j := 0; j < 3; j++ {
			load(fmt.Sprintf("STFST%d%d", i, j))
		}
		for _, prefix := range []string{"STFTR", "STFTL"} {
			load(fmt.Sprintf("%s%d0", prefix, i))
		}
		for _, prefix := range []string{"STFOUCH", "STFEVL", "STFKILL"} {
			load(fmt.Sprintf("%s%d", prefix, i))
		}
	}
	rows := func(name string) [][]string {
		f, e := os.Open(filepath.Join(out, name))
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		r, e := csv.NewReader(f).ReadAll()
		if e != nil {
			t.Fatal(e)
		}
		return r[1:]
	}
	num := func(s string) int64 {
		v, e := strconv.ParseInt(s, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for _, r := range rows("bob.csv") {
		g := &game{worldTic: int(num(r[0]))}
		g.p.momx, g.p.momy = num(r[1]), num(r[2])
		x, y := g.weaponBobFixed()
		if x+fracUnit != num(r[3]) || y+weaponTopY*fracUnit != num(r[4]) {
			t.Fatalf("bob mismatch: %v got %d,%d", r, x+fracUnit, y+weaponTopY*fracUnit)
		}
	}
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		save := func(name string, img *ebiten.Image) {
			rgba := image.NewRGBA(img.Bounds())
			img.ReadPixels(rgba.Pix)
			f, e := os.Create(filepath.Join(out, name+"-gd.png"))
			if e != nil {
				t.Fatal(e)
			}
			if e = png.Encode(f, rgba); e != nil {
				t.Fatal(e)
			}
			f.Close()
		}
		g := &game{opts: Options{DoomPaletteRGBA: palette, DoomPalettesRGBA: palettes, DoomColorMap: cm, DoomColorMapRows: len(cm) / 256, SpritePatchBank: bank, StatusPatchBank: bank}, m: &mapdata.Map{Sectors: []mapdata.Sector{{Light: 128}}}, viewW: 320, viewH: 200, mode: viewWalk, screenBlocks: 11, renderAlpha: 1}
		target := ebiten.NewImage(320, 200)
		for _, r := range rows("weapons.csv") {
			var state weaponPspriteState
			found := false
			for s, def := range weaponPspriteDefs {
				if def.sprite == r[1] {
					state = s
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("missing weapon frame %s", r[1])
			}
			g.weaponState, g.weaponFlashState = state, weaponStateNone
			g.weaponPSpriteY = int(num(r[3]) >> fracBits)
			g.weaponPose = weaponPresentationPose{X: num(r[2]), FractionY: num(r[3]) & (fracUnit - 1), Valid: true}
			g.m.Sectors[0].Light = int16(num(r[4]))
			target.Fill(color.Black)
			g.drawWeaponOverlay(target)
			save(r[0], target)
		}
		g.inventory = playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}}
		g.stats.Bullets = 50
		g.weaponState = weaponStateNone
		g.weaponFlashState = weaponStateNone
		g.weaponPose = weaponPresentationPose{}
		g.m.Sectors[0].Light = 128
		g.bringUpWeapon()
		for _, r := range rows("sequence.csv") {
			tick := int(num(r[0]))
			g.worldTic = tick
			g.p.momx = int64(tick%10) * fracUnit
			g.p.momy = fracUnit / 2
			if tick == 30 {
				g.setWeaponPSpriteState(weaponStatePistolAtk1, false)
			}
			if tick == 50 {
				g.isDead = true
				g.stats.Health = 0
				g.setWeaponPSpriteState(weaponStatePistolDown, false)
			}
			g.tickWeaponOverlay()
			x, y := g.weaponPresentationPosition()
			if int64(x*fracUnit) != num(r[1]) || int64(y*fracUnit) != num(r[2]) {
				t.Fatalf("sequence tic %d pose=%v,%v classic=%v", tick, x, y, r)
			}
			target.Fill(color.Black)
			g.drawWeaponOverlay(target)
			save(fmt.Sprintf("sequence-%03d", tick), target)
		}
		target.Dispose()
		target = ebiten.NewImage(320, 32)
		health := []int{100, 0, 1, 9, 10, 99, 101, 200, -5}
		weaponTypes := []int16{0, 0, 2001, 2002, 2003, 2004, 2006, 2005, 82}
		for fixture := 0; fixture < 136; fixture++ {
			n := fixture
			if fixture >= 72 {
				n -= 72
			}
			g.inventory = playerInventory{SkullKeys: [3]bool{n&8 != 0, n&16 != 0, n&32 != 0}, ReadyWeapon: weaponPistol, Backpack: n%2 == 1, Weapons: map[int16]bool{}, BlueKey: n&9 != 0, YellowKey: n&18 != 0, RedKey: n&36 != 0}
			if n%8 == 0 {
				g.inventory.ReadyWeapon = weaponFist
			}
			for i, typ := range weaponTypes {
				if typ != 0 {
					g.inventory.Weapons[typ] = (n>>(i%6))&1 != 0
				}
			}
			g.m.Things = []mapdata.Thing{{Type: 40}, {Type: 39}, {Type: 38}}
			g.thingCollected = []bool{n&8 != 0, n&16 != 0, n&32 != 0}
			g.stats.Health, g.stats.Armor = health[n%9], (n*13)%201
			g.stats.Bullets, g.stats.Shells, g.stats.Cells, g.stats.Rockets = n*17%201, (n*17+23)%201, (n*17+46)%201, (n*17+69)%201
			g.statusFaceIndex = n % 42
			target.Fill(color.Black)
			g.opts.GameMode = gameModeSingle
			if fixture >= 72 {
				g.localSlot = n%4 + 1
				g.opts.GameMode = gameModeCoop
				if n >= 32 {
					g.opts.GameMode = gameModeDeathmatch
				}
				g.authorityRules = &authorityRulesState{}
				g.authorityRules.Scores[g.localSlot].Frags = []int{0, 1, -1, 9, 10, 99, 100, -100}[(n/4)%8]
			}
			g.drawStatusBarLogicalBar(target, g.statusBarState())
			if fixture < 72 {
				save(fmt.Sprintf("hud-%02d", n), target)
			} else {
				save(fmt.Sprintf("net-hud-%02d", n), target)
			}
		}
		target.Dispose()
		target = ebiten.NewImage(320, 200)
		pattern := make([]byte, 320*200*4)
		for i := 0; i < 320*200; i++ {
			idx := ((i%320)/2 + (i/320%2)*128) % 256
			copy(pattern[i*4:i*4+4], palette[idx*4:idx*4+4])
		}
		g.isDead = false
		g.opts.GameMode = gameModeSingle
		g.authorityRules = nil
		for _, r := range rows("effects.csv") {
			g.statusDamageCount = int(num(r[1]))
			g.statusBonusCount = int(num(r[2]))
			g.inventory = playerInventory{StrengthCount: int(num(r[3])), RadSuitTics: int(num(r[4]))}
			target.WritePixels(pattern)
			g.drawFlashOverlay(target)
			save(r[0], target)
		}
		g.opts.KageShader = true
		for _, r := range rows("effects.csv") {
			g.statusDamageCount = int(num(r[1]))
			g.statusBonusCount = int(num(r[2]))
			g.inventory = playerInventory{StrengthCount: int(num(r[3])), RadSuitTics: int(num(r[4]))}
			target.WritePixels(pattern)
			g.drawFlashOverlay(target)
			save("shader-"+r[0], target)
		}
		g.opts.KageShader = false
		g.m.Sectors[0].Light = 64
		for _, r := range rows("powers.csv") {
			g.inventory = playerInventory{InvulnTics: int(num(r[1])), LightAmpTics: int(num(r[2])), InvisTics: int(num(r[3]))}
			g.weaponState = weaponStatePistolReady
			g.weaponFlashState = weaponStateNone
			if num(r[4]) != 0 {
				g.weaponFlashState = weaponStatePistolFlash
			}
			g.weaponPSpriteY = 32
			g.weaponPose = weaponPresentationPose{X: fracUnit, Valid: true}
			g.spectreFuzzPos = 0
			target.WritePixels(pattern)
			g.drawWeaponOverlay(target)
			save(r[0], target)
		}
		target.Dispose()
		// Modern fuzz keeps its grain size at higher/noninteger resolutions;
		// exercise both host shader settings and resize caches in this loop.
		g.opts.SourcePortMode = true
		g.opts.SourcePortSectorLighting = true
		g.weaponFlashState = weaponStateNone
		g.prevWeaponState = g.weaponState
		g.prevWeaponPose = g.weaponPose
		g.prevWeaponPSpriteY = g.weaponPSpriteY
		for _, size := range [][2]int{{320, 200}, {640, 400}, {960, 540}} {
			g.viewW, g.viewH = size[0], size[1]
			target = ebiten.NewImage(g.viewW, g.viewH)
			for _, shader := range []bool{false, true} {
				g.opts.KageShader = shader
				g.inventory = playerInventory{}
				target.Fill(color.RGBA{90, 60, 30, 255})
				g.drawWeaponOverlay(target)
				normal := make([]byte, g.viewW*g.viewH*4)
				target.ReadPixels(normal)
				g.inventory.InvisTics = 128
				target.Fill(color.RGBA{90, 60, 30, 255})
				g.drawWeaponOverlay(target)
				blink := make([]byte, len(normal))
				target.ReadPixels(blink)
				if !bytes.Equal(normal, blink) {
					t.Fatal("modern invisibility did not blink off")
				}
				g.inventory.InvisTics = 129
				target.Fill(color.RGBA{90, 60, 30, 255})
				g.drawWeaponOverlay(target)
				target.ReadPixels(blink)
				if bytes.Equal(normal, blink) {
					t.Fatal("modern weapon did not become fuzzy")
				}
				if !bytes.Equal(blink[:4], []byte{90, 60, 30, 255}) {
					t.Fatal("fuzz changed background outside weapon")
				}
			}
			target.Dispose()
		}
	}
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}
