//go:build integration

package doomruntime

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
)

const (
	billboardBBoxDiscardThreshold = 1
	billboardBBoxMaxBoxes         = spriteOpaqueRectMaxCount
)

type billboardBBoxDumpEntry struct {
	Name            string               `json:"name"`
	Category        string               `json:"category"`
	Width           int                  `json:"width"`
	Height          int                  `json:"height"`
	OffsetX         int                  `json:"offset_x"`
	OffsetY         int                  `json:"offset_y"`
	DiscardThresh   int                  `json:"discard_threshold"`
	MaxBoxes        int                  `json:"max_boxes"`
	ExpectedScale   int                  `json:"expected_scale"`
	MinScreenGain   int                  `json:"min_screen_gain"`
	OpaquePixels    int                  `json:"opaque_pixels"`
	CoveredPixels   int                  `json:"covered_pixels"`
	UncoveredPixels int                  `json:"uncovered_pixels"`
	Coverage        float64              `json:"coverage"`
	Boxes           []billboardBBoxEntry `json:"boxes"`
}

type billboardBBox struct {
	minX int
	minY int
	maxX int
	maxY int
	ok   bool
}

type billboardBBoxEntry struct {
	MinX   int `json:"min_x"`
	MinY   int `json:"min_y"`
	MaxX   int `json:"max_x"`
	MaxY   int `json:"max_y"`
	Width  int `json:"width"`
	Height int `json:"height"`
	Area   int `json:"area"`
}

func TestGenerateAlphaPatchBoundingBoxes(t *testing.T) {
	wadPath := strings.TrimSpace(os.Getenv("GD_DUMP_BILLBOARD_WAD"))
	if wadPath == "" {
		wadPath = findDOOM1WAD(t)
	}
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatalf("load textures from %s: %v", wadPath, err)
	}

	patchBank := buildAlphaPatchBankForDump(t, wf, ts)
	if len(patchBank) == 0 {
		t.Fatal("alpha patch bank is empty")
	}

	outDir := strings.TrimSpace(os.Getenv("GD_DUMP_BILLBOARD_OUT_DIR"))
	if outDir == "" {
		outDir = filepath.Join("testdata", "alpha_patch_bbox_dump")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", outDir, err)
	}

	names := sortedWallTextureKeys(patchBank)
	entries := make([]billboardBBoxDumpEntry, 0, len(names))
	for _, name := range names {
		tex := patchBank[name]
		boxes, opaquePixels, coveredPixels := computeOpaqueBillboardBoxes(tex, billboardBBoxMaxBoxes, billboardBBoxDiscardThreshold)
		if len(boxes) == 0 || opaquePixels == 0 {
			continue
		}
		entry := billboardBBoxDumpEntry{
			Name:            name,
			Category:        "patch",
			Width:           tex.Width,
			Height:          tex.Height,
			OffsetX:         tex.OffsetX,
			OffsetY:         tex.OffsetY,
			DiscardThresh:   billboardBBoxDiscardThreshold,
			MaxBoxes:        billboardBBoxMaxBoxes,
			ExpectedScale:   spriteOpaqueRectExpectedScale,
			MinScreenGain:   spriteOpaqueRectMinScreenGain,
			OpaquePixels:    opaquePixels,
			CoveredPixels:   coveredPixels,
			UncoveredPixels: opaquePixels - coveredPixels,
			Coverage:        float64(coveredPixels) / float64(opaquePixels),
			Boxes:           boxes,
		}
		entries = append(entries, entry)

		outPath := filepath.Join(outDir, fmt.Sprintf("%s__%s.png", entry.Category, name))
		if err := writeBillboardBBoxPNG(outPath, tex, boxes); err != nil {
			t.Fatalf("write %s: %v", outPath, err)
		}
	}

	if len(entries) == 0 {
		t.Fatal("no alpha patch bbox entries generated")
	}

	manifestPath := filepath.Join(outDir, "manifest.json")
	manifest, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatalf("write %s: %v", manifestPath, err)
	}
	t.Logf("wrote %d alpha patch bbox PNGs to %s", len(entries), outDir)
}

func buildAlphaPatchBankForDump(t *testing.T, wf *wad.File, ts *doomtex.Set) map[string]WallTexture {
	t.Helper()
	if wf == nil || ts == nil {
		return nil
	}
	names := make([]string, 0, len(wf.Lumps))
	seen := make(map[string]struct{}, len(wf.Lumps))
	for _, lump := range wf.Lumps {
		name := strings.ToUpper(strings.TrimSpace(lump.Name))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)

	out := make(map[string]WallTexture, len(names))
	for _, name := range names {
		rgba, w, h, ox, oy, err := ts.BuildPatchRGBA(name, 0)
		if err != nil || w <= 0 || h <= 0 || len(rgba) != w*h*4 {
			continue
		}
		opaque, transparent := 0, 0
		for i := 3; i < len(rgba); i += 4 {
			if rgba[i] == 0 {
				transparent++
				continue
			}
			opaque++
		}
		if opaque == 0 || transparent == 0 {
			continue
		}
		out[name] = WallTexture{
			RGBA:    rgba,
			Width:   w,
			Height:  h,
			OffsetX: ox,
			OffsetY: oy,
		}
	}
	return out
}

func sortedWallTextureKeys(m map[string]WallTexture) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func collectBillboardSpriteNamesForDump(g *game) map[string]map[string]struct{} {
	out := map[string]map[string]struct{}{
		"monster":        {},
		"projectile":     {},
		"projectile_fx":  {},
		"world_thing":    {},
		"hitscan_effect": {},
	}
	add := func(category, name string) {
		name = strings.ToUpper(strings.TrimSpace(name))
		if name == "" {
			return
		}
		if _, ok := out[category]; !ok {
			out[category] = make(map[string]struct{}, 16)
		}
		out[category][name] = struct{}{}
	}

	for typ := int16(0); typ < 4096; typ++ {
		for tic := 0; tic < 96; tic++ {
			add("world_thing", g.worldThingSpriteName(typ, tic))
		}
	}

	monsterTypes := []int16{3004, 9, 3001, 3002, 58, 3006, 3005, 3003, 16, 7}
	views := []struct {
		x float64
		y float64
	}{
		{100, 0},
		{100, 100},
		{0, 100},
		{-100, 100},
		{-100, 0},
		{-100, -100},
		{0, -100},
		{100, -100},
	}
	for _, typ := range monsterTypes {
		th := mapdata.Thing{Type: typ, X: 0, Y: 0, Angle: 0}
		g.thingAttackTics[0] = 0
		g.thingPainTics[0] = 0
		g.thingDeathTics[0] = 0
		g.thingDead[0] = false
		for tic := 0; tic < 32; tic += 4 {
			for _, view := range views {
				name, _ := g.monsterSpriteNameForView(0, th, tic, view.x, view.y)
				add("monster", name)
			}
		}

		g.thingAttackTics[0] = 16
		for _, view := range views {
			name, _ := g.monsterSpriteNameForView(0, th, 0, view.x, view.y)
			add("monster", name)
		}
		g.thingAttackTics[0] = 6
		for _, view := range views {
			name, _ := g.monsterSpriteNameForView(0, th, 0, view.x, view.y)
			add("monster", name)
		}

		g.thingAttackTics[0] = 0
		g.thingPainTics[0] = 6
		for _, view := range views {
			name, _ := g.monsterSpriteNameForView(0, th, 0, view.x, view.y)
			add("monster", name)
		}

		g.thingPainTics[0] = 0
		g.thingDead[0] = true
		g.thingDeathTics[0] = monsterDeathAnimTotalTics(typ)
		for _, view := range views {
			name, _ := g.monsterSpriteNameForView(0, th, 0, view.x, view.y)
			add("monster", name)
		}
		g.thingDeathTics[0] = 0
		for _, view := range views {
			name, _ := g.monsterSpriteNameForView(0, th, 0, view.x, view.y)
			add("monster", name)
		}
		g.thingDead[0] = false
	}

	projectileKinds := []projectileKind{
		projectileFireball,
		projectilePlasmaBall,
		projectileBaronBall,
		projectileRocket,
	}
	for _, kind := range projectileKinds {
		for tic := 0; tic < 20; tic++ {
			add("projectile", g.projectileSpriteName(kind, tic))
		}
		for elapsed := 0; elapsed < 20; elapsed++ {
			add("projectile_fx", g.projectileImpactSpriteName(kind, elapsed))
		}
	}

	for _, name := range []string{"PUFFA0", "PUFFB0", "PUFFC0", "PUFFD0", "BLUDA0", "BLUDB0", "BLUDC0"} {
		add("hitscan_effect", name)
	}

	return out
}

func computeOpaqueBillboardBoxes(tex WallTexture, maxBoxes, discardThreshold int) ([]billboardBBoxEntry, int, int) {
	if tex.Width <= 0 || tex.Height <= 0 {
		return nil, 0, 0
	}
	tex.EnsureOpaqueMask()
	if len(tex.OpaqueMask) != tex.Width*tex.Height {
		return nil, 0, 0
	}
	opaquePixels := 0
	for _, opaque := range tex.OpaqueMask {
		if opaque != 0 {
			opaquePixels++
		}
	}
	if opaquePixels == 0 {
		return nil, 0, 0
	}
	rects := buildSpriteOpaqueRects(tex.OpaqueMask, tex.Width, tex.Height)
	if len(rects) > maxBoxes {
		rects = rects[:maxBoxes]
	}
	boxes := make([]billboardBBoxEntry, 0, len(rects))
	coveredPixels := 0
	for _, rect := range rects {
		area := spriteOpaqueRectArea(rect)
		if area < discardThreshold {
			break
		}
		boxes = append(boxes, billboardBBoxEntry{
			MinX:   rect.minX(),
			MinY:   rect.minY(),
			MaxX:   rect.maxX(),
			MaxY:   rect.maxY(),
			Width:  rect.maxX() - rect.minX() + 1,
			Height: rect.maxY() - rect.minY() + 1,
			Area:   area,
		})
		coveredPixels += area
	}
	return boxes, opaquePixels, coveredPixels
}

func writeBillboardBBoxPNG(path string, tex WallTexture, boxes []billboardBBoxEntry) error {
	const (
		scale = 2
		pad   = 4
	)
	dst := image.NewNRGBA(image.Rect(0, 0, (tex.Width+pad*2)*scale, (tex.Height+pad*2)*scale))
	drawChecker(dst)
	spriteRect := image.Rect(pad*scale, pad*scale, (pad+tex.Width)*scale, (pad+tex.Height)*scale)
	draw.Draw(dst, spriteRect, image.NewUniform(color.NRGBA{0, 0, 0, 0}), image.Point{}, draw.Src)
	for y := 0; y < tex.Height; y++ {
		for x := 0; x < tex.Width; x++ {
			i := (y*tex.Width + x) * 4
			c := color.NRGBA{
				R: tex.RGBA[i+0],
				G: tex.RGBA[i+1],
				B: tex.RGBA[i+2],
				A: tex.RGBA[i+3],
			}
			baseX := (pad + x) * scale
			baseY := (pad + y) * scale
			for sy := 0; sy < scale; sy++ {
				for sx := 0; sx < scale; sx++ {
					dst.SetNRGBA(baseX+sx, baseY+sy, c)
				}
			}
		}
	}
	colors := []color.NRGBA{
		{R: 255, G: 80, B: 80, A: 255},
		{R: 80, G: 220, B: 255, A: 255},
		{R: 255, G: 210, B: 0, A: 255},
		{R: 120, G: 255, B: 120, A: 255},
	}
	for i, box := range boxes {
		drawBBoxOutline(dst, billboardBBox{
			minX: box.MinX,
			minY: box.MinY,
			maxX: box.MaxX,
			maxY: box.MaxY,
			ok:   true,
		}, pad, scale, colors[i%len(colors)])
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, dst)
}

func drawChecker(dst *image.NRGBA) {
	c0 := color.NRGBA{R: 26, G: 24, B: 24, A: 255}
	c1 := color.NRGBA{R: 40, G: 38, B: 38, A: 255}
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			if ((x / 8) + (y/8)&1) == 0 {
				dst.SetNRGBA(x, y, c0)
				continue
			}
			dst.SetNRGBA(x, y, c1)
		}
	}
}

func drawBBoxOutline(dst *image.NRGBA, box billboardBBox, pad, scale int, c color.NRGBA) {
	if !box.ok {
		return
	}
	minX := (box.minX + pad) * scale
	minY := (box.minY + pad) * scale
	maxX := (box.maxX+pad+1)*scale - 1
	maxY := (box.maxY+pad+1)*scale - 1
	for x := minX; x <= maxX; x++ {
		dst.SetNRGBA(x, minY, c)
		dst.SetNRGBA(x, maxY, c)
	}
	for y := minY; y <= maxY; y++ {
		dst.SetNRGBA(minX, y, c)
		dst.SetNRGBA(maxX, y, c)
	}
}

func buildBillboardSpritePatchBankForTest(t *testing.T, ts *doomtex.Set) map[string]WallTexture {
	t.Helper()
	if ts == nil {
		return nil
	}
	names := make([]string, 0, 512)
	seen := make(map[string]struct{}, 512)
	add := func(name string) {
		name = strings.ToUpper(strings.TrimSpace(name))
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	addExpandedSeed := func(seed string) {
		if len(seed) < 6 {
			return
		}
		pfx := seed[:4]
		for fr := byte('A'); fr <= byte('Z'); fr++ {
			add(fmt.Sprintf("%s%c0", pfx, fr))
			add(fmt.Sprintf("%s%c1", pfx, fr))
		}
	}

	for _, pfx := range []string{"POSS", "SPOS", "TROO", "SARG", "SKUL", "HEAD", "BOSS", "CYBR", "SPID"} {
		for fr := byte('A'); fr <= byte('Z'); fr++ {
			add(fmt.Sprintf("%s%c0", pfx, fr))
			add(fmt.Sprintf("%s%c1", pfx, fr))
			add(fmt.Sprintf("%s%c1%c5", pfx, fr, fr))
			add(fmt.Sprintf("%s%c5%c1", pfx, fr, fr))
			add(fmt.Sprintf("%s%c2%c8", pfx, fr, fr))
			add(fmt.Sprintf("%s%c8%c2", pfx, fr, fr))
			add(fmt.Sprintf("%s%c3%c7", pfx, fr, fr))
			add(fmt.Sprintf("%s%c7%c3", pfx, fr, fr))
			add(fmt.Sprintf("%s%c4%c6", pfx, fr, fr))
			add(fmt.Sprintf("%s%c6%c4", pfx, fr, fr))
			add(fmt.Sprintf("%s%c5", pfx, fr))
		}
	}

	for _, pfx := range []string{"MISL", "BAL1", "BAL2", "BAL7", "PLSS", "PLSE", "BFS1", "BFE1", "FATB", "MANF", "FBXP", "BOSF", "FIRE"} {
		for fr := byte('A'); fr <= byte('E'); fr++ {
			add(fmt.Sprintf("%s%c0", pfx, fr))
			add(fmt.Sprintf("%s%c1", pfx, fr))
			add(fmt.Sprintf("%s%c1%c5", pfx, fr, fr))
			add(fmt.Sprintf("%s%c5%c1", pfx, fr, fr))
		}
	}
	for fr := byte('F'); fr <= byte('H'); fr++ {
		add(fmt.Sprintf("FIRE%c0", fr))
		add(fmt.Sprintf("FIRE%c1", fr))
	}
	for _, pfx := range []string{"APBX", "APLS", "IFOG", "PINS", "PINV", "SOUL"} {
		for fr := byte('A'); fr <= byte('E'); fr++ {
			add(fmt.Sprintf("%s%c0", pfx, fr))
		}
	}
	for _, name := range []string{"PLSSA0", "PLSSB0", "BFS1A0", "BFS1B0"} {
		add(name)
	}

	for _, name := range []string{
		"PLAYN0", "POSSL0", "SPOSL0", "TROOL0", "SARGN0", "HEADL0", "SKULF0", "BBRNA0", "BBRNB0",
		"POL1A0", "POL2A0", "POL3A0", "POL4A0", "POL5A0", "POL6A0",
		"COL1A0", "COL2A0", "COL3A0", "COL4A0", "COL5A0", "TRE1A0", "TRE2A0",
		"CANDA0", "CBRAA0", "CEYEA0", "FSKUA0", "FCANA0", "ELECA0",
		"GOR1A0", "GOR2A0", "GOR3A0", "GOR4A0", "GOR5A0",
		"SMITA0", "KEENA0",
		"BKEYA0", "YKEYA0", "RKEYA0", "BSKUA0", "YSKUA0", "RSKUA0",
		"STIMA0", "MEDIA0", "SOULA0", "BON1A0", "BON2A0",
		"ARM1A0", "ARM2A0", "PINVA0", "PSTRA0", "PINSA0", "SUITA0", "PMAPA0", "PVISA0", "MEGAA0",
		"APBXA0", "APLSA0", "IFOGA0", "PLSSA0", "PLSEA0", "BFS1A0",
		"CLIPA0", "AMMOA0", "SHELA0", "SBOXA0", "ROCKA0", "BROKA0", "CELLA0", "CELPA0", "BPAKA0",
		"SHOTA0", "MGUNA0", "LAUNA0", "PLASA0", "CSAWA0", "BFUGA0",
		"BAR1A0", "BEXPA0",
		"TBLUA0", "TGRNA0", "TREDA0", "SMRTA0", "SMGTA0", "SMBTA0", "TLMPA0", "TLP2A0",
		"PUFFA0", "BLUDA0",
		"TFOGA0", "TFOGB0", "TFOGC0", "TFOGD0", "TFOGE0", "TFOGF0", "TFOGG0", "TFOGH0", "TFOGI0", "TFOGJ0",
	} {
		add(name)
		addExpandedSeed(name)
	}

	out := make(map[string]WallTexture, len(names))
	for _, name := range names {
		rgba, w, h, ox, oy, err := ts.BuildPatchRGBA(name, 0)
		if err != nil || w <= 0 || h <= 0 || len(rgba) != w*h*4 {
			continue
		}
		out[name] = WallTexture{
			RGBA:    rgba,
			Width:   w,
			Height:  h,
			OffsetX: ox,
			OffsetY: oy,
		}
	}
	return out
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
