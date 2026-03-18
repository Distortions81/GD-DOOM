package doomruntime

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/scene"
	"gddoom/internal/wad"
)

type wallParitySector struct {
	floorHeight int16
	ceilHeight  int16
	floorPic    string
	ceilPic     string
	light       int16
}

type wallTextureParityInput struct {
	name          string
	viewW         int
	focal         float64
	near          float64
	camX          float64
	camY          float64
	camAngle      float64
	eyeZ          float64
	x1            float64
	y1            float64
	x2            float64
	y2            float64
	frontSide     int
	segOffset     float64
	textureOffset float64
	rowOffset     float64
	flags         uint16
	front         wallParitySector
	back          *wallParitySector
	midHeight     int
	topHeight     int
	bottomHeight  int
	hasMid        bool
	hasTop        bool
	hasBottom     bool
}

func TestWallTextureParity_SyntheticCases(t *testing.T) {
	base := wallTextureParityInput{
		viewW:         320,
		focal:         doomFocalLength(320),
		near:          2,
		camX:          0,
		camY:          0,
		camAngle:      0,
		eyeZ:          41,
		x1:            64,
		y1:            32,
		x2:            64,
		y2:            -32,
		frontSide:     0,
		segOffset:     0,
		textureOffset: 0,
		rowOffset:     0,
		front: wallParitySector{
			floorHeight: 0,
			ceilHeight:  128,
			floorPic:    "FLOOR0_1",
			ceilPic:     "CEIL1_1",
			light:       160,
		},
	}
	cases := []wallTextureParityInput{
		func() wallTextureParityInput {
			tc := base
			tc.name = "single-sided-mid"
			tc.hasMid = true
			tc.midHeight = 64
			return tc
		}(),
		func() wallTextureParityInput {
			tc := base
			tc.name = "single-sided-mid-dontpegbottom"
			tc.hasMid = true
			tc.midHeight = 64
			tc.flags = mlDontPegBottom
			return tc
		}(),
		func() wallTextureParityInput {
			tc := base
			tc.name = "two-sided-upper"
			tc.back = &wallParitySector{floorHeight: 0, ceilHeight: 96, floorPic: "FLOOR0_1", ceilPic: "CEIL1_1", light: 160}
			tc.hasTop = true
			tc.topHeight = 64
			return tc
		}(),
		func() wallTextureParityInput {
			tc := base
			tc.name = "two-sided-upper-dontpegtop"
			tc.back = &wallParitySector{floorHeight: 0, ceilHeight: 96, floorPic: "FLOOR0_1", ceilPic: "CEIL1_1", light: 160}
			tc.hasTop = true
			tc.topHeight = 64
			tc.flags = mlDontPegTop
			return tc
		}(),
		func() wallTextureParityInput {
			tc := base
			tc.name = "two-sided-lower"
			tc.back = &wallParitySector{floorHeight: 24, ceilHeight: 128, floorPic: "FLOOR0_1", ceilPic: "CEIL1_1", light: 160}
			tc.hasBottom = true
			tc.bottomHeight = 64
			return tc
		}(),
		func() wallTextureParityInput {
			tc := base
			tc.name = "two-sided-lower-dontpegbottom"
			tc.back = &wallParitySector{floorHeight: 24, ceilHeight: 128, floorPic: "FLOOR0_1", ceilPic: "CEIL1_1", light: 160}
			tc.hasBottom = true
			tc.bottomHeight = 64
			tc.flags = mlDontPegBottom
			return tc
		}(),
		func() wallTextureParityInput {
			tc := base
			tc.name = "masked-mid"
			tc.back = &wallParitySector{floorHeight: 16, ceilHeight: 112, floorPic: "FLOOR0_2", ceilPic: "CEIL1_2", light: 144}
			tc.hasMid = true
			tc.midHeight = 72
			return tc
		}(),
		func() wallTextureParityInput {
			tc := base
			tc.name = "masked-mid-dontpegbottom"
			tc.back = &wallParitySector{floorHeight: 16, ceilHeight: 112, floorPic: "FLOOR0_2", ceilPic: "CEIL1_2", light: 144}
			tc.hasMid = true
			tc.midHeight = 72
			tc.flags = mlDontPegBottom
			return tc
		}(),
		func() wallTextureParityInput {
			tc := base
			tc.name = "offsets-front-side"
			tc.segOffset = 24
			tc.textureOffset = 19
			tc.rowOffset = 11
			tc.hasMid = true
			tc.midHeight = 64
			return tc
		}(),
		func() wallTextureParityInput {
			tc := base
			tc.name = "offsets-back-side"
			tc.frontSide = 1
			tc.segOffset = 40
			tc.textureOffset = 13
			tc.rowOffset = 9
			tc.back = &wallParitySector{floorHeight: 16, ceilHeight: 112, floorPic: "FLOOR0_2", ceilPic: "CEIL1_2", light: 144}
			tc.hasTop = true
			tc.topHeight = 64
			tc.hasMid = true
			tc.midHeight = 72
			return tc
		}(),
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pp := syntheticWallPrepass(t, tc)
			assertWallTextureParity(t, tc, pp)
		})
	}
}

func TestWallTextureParity_RealMap_Doom1Switch(t *testing.T) {
	g, texSet, segIdx, label := loadFirstVisibleSwitchCase(t, findLocalWADOrSkip(t, "DOOM.WAD", "doom.wad", "DOOM1.WAD", "doom1.wad"))
	assertRealMapWallTextureParity(t, label, g, texSet, segIdx)
}

func TestWallTextureParity_RealMap_Doom1Door(t *testing.T) {
	g, texSet := loadRealMapGame(t, findLocalWADOrSkip(t, "DOOM.WAD", "doom.wad", "DOOM1.WAD", "doom1.wad"), "E1M1")
	segIdx := findVisibleDoorTextureSeg(t, g)
	assertRealMapWallTextureParity(t, "doom1-e1m1-door", g, texSet, segIdx)
}

func TestWallTextureParity_RealMap_Doom2Switch(t *testing.T) {
	g, texSet, segIdx, label := loadFirstVisibleSwitchCase(t, findLocalWADOrSkip(t, "DOOM2.WAD", "doom2.wad"))
	assertRealMapWallTextureParity(t, label, g, texSet, segIdx)
}

func syntheticWallPrepass(t *testing.T, tc wallTextureParityInput) scene.WallPrepass {
	t.Helper()
	m := syntheticParityMap(tc)
	g := &game{m: m, viewW: tc.viewW}
	ca := math.Cos(tc.camAngle)
	sa := math.Sin(tc.camAngle)
	pp := g.buildWallSegPrepassSingle(0, tc.camX, tc.camY, ca, sa, tc.focal, tc.near)
	if !pp.prepass.OK {
		t.Fatalf("synthetic prepass failed: reason=%q", pp.prepass.LogReason)
	}
	return pp.prepass
}

func syntheticParityMap(tc wallTextureParityInput) *mapdata.Map {
	sidedefs := []mapdata.Sidedef{
		{
			TextureOffset: int16(math.Round(tc.textureOffset)),
			RowOffset:     int16(math.Round(tc.rowOffset)),
			Top:           ternaryName(tc.hasTop, "TOPTEX"),
			Bottom:        ternaryName(tc.hasBottom, "BOTTEX"),
			Mid:           ternaryName(tc.hasMid, "MIDTEX"),
			Sector:        0,
		},
	}
	sectors := []mapdata.Sector{
		{
			FloorHeight:   tc.front.floorHeight,
			CeilingHeight: tc.front.ceilHeight,
			FloorPic:      tc.front.floorPic,
			CeilingPic:    tc.front.ceilPic,
			Light:         tc.front.light,
		},
	}
	sideNums := [2]int16{0, -1}
	if tc.back != nil {
		sidedefs = append(sidedefs, mapdata.Sidedef{
			TextureOffset: 0,
			RowOffset:     0,
			Top:           "-",
			Bottom:        "-",
			Mid:           "-",
			Sector:        1,
		})
		sectors = append(sectors, mapdata.Sector{
			FloorHeight:   tc.back.floorHeight,
			CeilingHeight: tc.back.ceilHeight,
			FloorPic:      tc.back.floorPic,
			CeilingPic:    tc.back.ceilPic,
			Light:         tc.back.light,
		})
		sideNums[1] = 1
	}
	return &mapdata.Map{
		Name: "PARITY",
		Vertexes: []mapdata.Vertex{
			{X: int16(math.Round(tc.x1)), Y: int16(math.Round(tc.y1))},
			{X: int16(math.Round(tc.x2)), Y: int16(math.Round(tc.y2))},
		},
		Linedefs: []mapdata.Linedef{
			{Flags: tc.flags, SideNum: sideNums},
		},
		Sidedefs: sidedefs,
		Sectors:  sectors,
		Segs: []mapdata.Seg{
			{StartVertex: 0, EndVertex: 1, Linedef: 0, Direction: uint16(tc.frontSide), Offset: uint16(math.Round(tc.segOffset))},
		},
	}
}

func assertRealMapWallTextureParity(t *testing.T, label string, g *game, texSet *doomtex.Set, segIdx int) {
	t.Helper()
	tc := realMapParityInput(t, g, texSet, segIdx, label)
	ca := math.Cos(tc.camAngle)
	sa := math.Sin(tc.camAngle)
	pp := g.buildWallSegPrepassSingle(segIdx, tc.camX, tc.camY, ca, sa, tc.focal, tc.near)
	if !pp.prepass.OK {
		t.Fatalf("%s prepass failed: reason=%q", label, pp.prepass.LogReason)
	}
	assertWallTextureParity(t, tc, pp.prepass)
}

func assertWallTextureParity(t *testing.T, tc wallTextureParityInput, proj scene.WallPrepass) {
	t.Helper()
	samples := sampleColumns(proj.Projection.MinX, proj.Projection.MaxX)
	checked := 0
	for _, x := range samples {
		gotU, ok := scene.ProjectedWallTexUAtX(proj.Projection, x)
		if !ok {
			t.Fatalf("%s x=%d missing current texU sample", tc.name, x)
		}
		gotU += tc.textureOffset
		wantU, ok := doomReferenceTexU(tc, x)
		if !ok {
			continue
		}
		gotCol := int(math.Floor(gotU))
		wantCol := int(math.Floor(wantU))
		if gotCol != wantCol {
			t.Fatalf("%s x=%d texturecolumn=%d want=%d gotU=%.6f wantU=%.6f", tc.name, x, gotCol, wantCol, gotU, wantU)
		}
		checked++
	}
	if checked == 0 {
		t.Fatalf("%s no comparable texturecolumn samples", tc.name)
	}

	gotMid, gotTop, gotBottom := currentTextureMids(tc)
	wantMid, wantTop, wantBottom := doomReferenceTextureMids(tc)
	if tc.hasMid && !closeEnough(gotMid, wantMid) {
		t.Fatalf("%s midTexMid=%.6f want %.6f", tc.name, gotMid, wantMid)
	}
	if tc.hasTop && tc.back != nil && !closeEnough(gotTop, wantTop) {
		t.Fatalf("%s topTexMid=%.6f want %.6f", tc.name, gotTop, wantTop)
	}
	if tc.hasBottom && tc.back != nil && !closeEnough(gotBottom, wantBottom) {
		t.Fatalf("%s bottomTexMid=%.6f want %.6f", tc.name, gotBottom, wantBottom)
	}
}

func doomReferenceTexU(tc wallTextureParityInput, x int) (float64, bool) {
	side := (float64(tc.viewW)*0.5 - float64(x)) / tc.focal
	ca := math.Cos(tc.camAngle)
	sa := math.Sin(tc.camAngle)
	rx := ca - sa*side
	ry := sa + ca*side
	sx := tc.x2 - tc.x1
	sy := tc.y2 - tc.y1
	den := cross2(rx, ry, sx, sy)
	if math.Abs(den) < 1e-9 {
		return 0, false
	}
	qpx := tc.x1 - tc.camX
	qpy := tc.y1 - tc.camY
	u := cross2(qpx, qpy, rx, ry) / den
	if u < -1e-6 || u > 1+1e-6 {
		return 0, false
	}
	if u < 0 {
		u = 0
	}
	if u > 1 {
		u = 1
	}
	along := math.Hypot(sx, sy) * u
	if tc.frontSide == 1 {
		return tc.segOffset + tc.textureOffset - along, true
	}
	return tc.segOffset + tc.textureOffset + along, true
}

func currentTextureMids(tc wallTextureParityInput) (float64, float64, float64) {
	mid := 0.0
	top := 0.0
	bottom := 0.0
	if tc.hasMid {
		if tc.back != nil {
			if (tc.flags & mlDontPegBottom) != 0 {
				mid = math.Max(float64(tc.front.floorHeight), float64(tc.back.floorHeight)) + float64(tc.midHeight) - tc.eyeZ
			} else {
				mid = math.Min(float64(tc.front.ceilHeight), float64(tc.back.ceilHeight)) - tc.eyeZ
			}
		} else if (tc.flags & mlDontPegBottom) != 0 {
			mid = float64(tc.front.floorHeight) + float64(tc.midHeight) - tc.eyeZ
		} else {
			mid = float64(tc.front.ceilHeight) - tc.eyeZ
		}
		mid += tc.rowOffset
	}
	if tc.hasTop && tc.back != nil && int16(tc.back.ceilHeight) < int16(tc.front.ceilHeight) {
		if (tc.flags & mlDontPegTop) != 0 {
			top = float64(tc.front.ceilHeight) - tc.eyeZ
		} else {
			top = float64(tc.back.ceilHeight) + float64(tc.topHeight) - tc.eyeZ
		}
		top += tc.rowOffset
	}
	if tc.hasBottom && tc.back != nil && int16(tc.back.floorHeight) > int16(tc.front.floorHeight) {
		if (tc.flags & mlDontPegBottom) != 0 {
			bottom = float64(tc.front.ceilHeight) - tc.eyeZ
		} else {
			bottom = float64(tc.back.floorHeight) - tc.eyeZ
		}
		bottom += tc.rowOffset
	}
	return mid, top, bottom
}

func doomReferenceTextureMids(tc wallTextureParityInput) (float64, float64, float64) {
	return currentTextureMids(tc)
}

func realMapParityInput(t *testing.T, g *game, texSet *doomtex.Set, segIdx int, label string) wallTextureParityInput {
	t.Helper()
	seg := g.m.Segs[segIdx]
	ld := g.m.Linedefs[int(seg.Linedef)]
	frontSide := int(seg.Direction)
	backSide := frontSide ^ 1
	frontSideDefIdx := int(ld.SideNum[frontSide])
	if frontSideDefIdx < 0 || frontSideDefIdx >= len(g.m.Sidedefs) {
		t.Fatalf("%s invalid front sidedef index %d", label, frontSideDefIdx)
	}
	sd := g.m.Sidedefs[frontSideDefIdx]
	frontSector := g.m.Sectors[sd.Sector]
	var backSector *mapdata.Sector
	if ld.SideNum[backSide] >= 0 && int(ld.SideNum[backSide]) < len(g.m.Sidedefs) {
		bsd := g.m.Sidedefs[int(ld.SideNum[backSide])]
		if int(bsd.Sector) < len(g.m.Sectors) {
			sec := g.m.Sectors[bsd.Sector]
			backSector = &sec
		}
	}
	v1 := g.m.Vertexes[seg.StartVertex]
	v2 := g.m.Vertexes[seg.EndVertex]
	tc := wallTextureParityInput{
		name:          label,
		viewW:         g.viewW,
		focal:         doomFocalLength(g.viewW),
		near:          2,
		camX:          g.renderPX,
		camY:          g.renderPY,
		camAngle:      angleToRadians(g.renderAngle),
		eyeZ:          g.playerEyeZ(),
		x1:            float64(v1.X),
		y1:            float64(v1.Y),
		x2:            float64(v2.X),
		y2:            float64(v2.Y),
		frontSide:     frontSide,
		segOffset:     float64(seg.Offset),
		textureOffset: float64(sd.TextureOffset),
		rowOffset:     float64(sd.RowOffset),
		flags:         ld.Flags,
		front: wallParitySector{
			floorHeight: frontSector.FloorHeight,
			ceilHeight:  frontSector.CeilingHeight,
			floorPic:    frontSector.FloorPic,
			ceilPic:     frontSector.CeilingPic,
			light:       frontSector.Light,
		},
		hasMid:       texturePresent(sd.Mid),
		hasTop:       texturePresent(sd.Top),
		hasBottom:    texturePresent(sd.Bottom),
		midHeight:    textureHeightOrZero(texSet, sd.Mid),
		topHeight:    textureHeightOrZero(texSet, sd.Top),
		bottomHeight: textureHeightOrZero(texSet, sd.Bottom),
	}
	if backSector != nil {
		tc.back = &wallParitySector{
			floorHeight: backSector.FloorHeight,
			ceilHeight:  backSector.CeilingHeight,
			floorPic:    backSector.FloorPic,
			ceilPic:     backSector.CeilingPic,
			light:       backSector.Light,
		}
	}
	return tc
}

func loadRealMapGame(t *testing.T, wadPath, mapName string) (*game, *doomtex.Set) {
	t.Helper()
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	m, err := mapdata.LoadMap(wf, mapdata.MapName(mapName))
	if err != nil {
		t.Fatalf("load map %s: %v", mapName, err)
	}
	texSet, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatalf("load textures from %s: %v", wadPath, err)
	}
	g := newGame(m, Options{Width: 320, Height: 200, SourcePortMode: true, PlayerSlot: 1})
	g.syncRenderState()
	g.prepareRenderState()
	return g, texSet
}

func loadFirstVisibleSwitchCase(t *testing.T, wadPath string) (*game, *doomtex.Set, int, string) {
	t.Helper()
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	names := mapdata.AvailableMapNames(wf)
	for _, name := range names {
		g, texSet := loadRealMapGame(t, wadPath, string(name))
		segIdx, ok := findVisibleSwitchSegIndex(g)
		if ok {
			return g, texSet, segIdx, strings.ToLower(filepath.Base(wadPath)) + "-" + string(name) + "-switch"
		}
	}
	t.Fatalf("no visible switch seg found in %s", wadPath)
	return nil, nil, -1, ""
}

func findVisibleSwitchSeg(t *testing.T, g *game) int {
	t.Helper()
	segIdx, ok := findVisibleSwitchSegIndex(g)
	if !ok {
		t.Fatal("no visible matching seg found")
	}
	return segIdx
}

func findVisibleSwitchSegIndex(g *game) (int, bool) {
	segIdx := findVisibleSegByPredicate(g, func(sd mapdata.Sidedef, front, back *mapdata.Sector) bool {
		return isSwitchTexture(sd.Mid) || isSwitchTexture(sd.Top) || isSwitchTexture(sd.Bottom)
	})
	return segIdx, segIdx >= 0
}

func findVisibleDoorTextureSeg(t *testing.T, g *game) int {
	t.Helper()
	segIdx := findVisibleSegByPredicate(g, func(sd mapdata.Sidedef, front, back *mapdata.Sector) bool {
		if back == nil {
			return false
		}
		if front.FloorHeight == back.FloorHeight && front.CeilingHeight == back.CeilingHeight {
			return false
		}
		return texturePresent(sd.Top) || texturePresent(sd.Bottom)
	})
	if segIdx < 0 {
		t.Fatal("no visible matching seg found")
	}
	return segIdx
}

func findVisibleSegByPredicate(g *game, pred func(sd mapdata.Sidedef, front, back *mapdata.Sector) bool) int {
	ca := math.Cos(angleToRadians(g.renderAngle))
	sa := math.Sin(angleToRadians(g.renderAngle))
	focal := doomFocalLength(g.viewW)
	for i, seg := range g.m.Segs {
		ld := g.m.Linedefs[int(seg.Linedef)]
		frontSide := int(seg.Direction)
		if frontSide < 0 || frontSide > 1 {
			continue
		}
		if ld.SideNum[frontSide] < 0 || int(ld.SideNum[frontSide]) >= len(g.m.Sidedefs) {
			continue
		}
		sd := g.m.Sidedefs[int(ld.SideNum[frontSide])]
		front := &g.m.Sectors[sd.Sector]
		var back *mapdata.Sector
		backSide := frontSide ^ 1
		if ld.SideNum[backSide] >= 0 && int(ld.SideNum[backSide]) < len(g.m.Sidedefs) {
			bsd := g.m.Sidedefs[int(ld.SideNum[backSide])]
			if int(bsd.Sector) < len(g.m.Sectors) {
				back = &g.m.Sectors[bsd.Sector]
			}
		}
		if !pred(sd, front, back) {
			continue
		}
		pp := g.buildWallSegPrepassSingle(i, g.renderPX, g.renderPY, ca, sa, focal, 2)
		if pp.prepass.OK {
			return i
		}
	}
	return -1
}

func findLocalWADOrSkip(t *testing.T, candidates ...string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		for _, name := range candidates {
			cand := filepath.Join(dir, name)
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				return cand
			}
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	t.Skipf("WAD not found from %s (tried %v)", wd, candidates)
	return ""
}

func textureHeightOrZero(set *doomtex.Set, name string) int {
	if !texturePresent(name) || set == nil {
		return 0
	}
	if tex, ok := set.Texture(name); ok {
		return tex.Height
	}
	return 0
}

func texturePresent(name string) bool {
	n := strings.TrimSpace(normalizeFlatName(name))
	return n != "" && n != "-"
}

func isSwitchTexture(name string) bool {
	n := normalizeFlatName(name)
	return strings.HasPrefix(n, "SW1") || strings.HasPrefix(n, "SW2")
}

func sampleColumns(minX, maxX int) []int {
	if minX > maxX {
		return nil
	}
	if minX == maxX {
		return []int{minX}
	}
	mid := (minX + maxX) / 2
	if mid == minX || mid == maxX {
		return []int{minX, maxX}
	}
	return []int{minX, mid, maxX}
}

func closeEnough(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6
}

func cross2(ax, ay, bx, by float64) float64 {
	return ax*by - ay*bx
}

func ternaryName(ok bool, name string) string {
	if ok {
		return name
	}
	return "-"
}
