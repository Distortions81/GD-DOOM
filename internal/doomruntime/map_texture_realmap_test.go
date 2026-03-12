package doomruntime

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
)

func TestRealMap_E1M1_SubsectorTriangulationQuality(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)

	total := len(g.m.SubSectors)
	if total == 0 {
		t.Fatal("no subsectors in E1M1")
	}

	noPoly := 0
	nonSimple := 0
	triFail := 0
	badArea := 0
	diag := realMapDiag{
		Map:        "E1M1",
		SubSectors: total,
	}

	for ss := range g.m.SubSectors {
		verts, stage, ok := mapPolyForSubsectorWithStage(g, ss)
		if !ok || len(verts) < 3 {
			noPoly++
			if noPoly <= 16 {
				sub := g.m.SubSectors[ss]
				t.Logf("noPoly ss=%d segs=%d firstSeg=%d stage=%s", ss, sub.SegCount, sub.FirstSeg, stage)
			}
			diag.Failures = append(diag.Failures, buildSubsectorDiag(g, ss, stage, "no_poly", nil))
			continue
		}
		if !polygonSimple(verts) {
			nonSimple++
		}
		tris, ok := triangulateWorldPolygon(verts)
		if !ok || len(tris) == 0 {
			tris, ok = triangulateByAngleFan(verts)
		}
		if !ok || len(tris) == 0 {
			triFail++
			if triFail <= 16 {
				t.Logf("triFail ss=%d stage=%s verts=%d simple=%t area2=%.3f", ss, stage, len(verts), polygonSimple(verts), polygonArea2(verts))
			}
			diag.Failures = append(diag.Failures, buildSubsectorDiag(g, ss, stage, "triangulation_failed", verts))
			continue
		}
		polyArea := math.Abs(polygonArea2(verts)) * 0.5
		if polyArea <= 1e-6 {
			triFail++
			continue
		}
		triArea := 0.0
		for _, tri := range tris {
			a := verts[tri[0]]
			b := verts[tri[1]]
			c := verts[tri[2]]
			triArea += math.Abs(orient2D(a, b, c)) * 0.5
		}
		// Triangulated area should closely match polygon area.
		if triArea < polyArea*0.98 || triArea > polyArea*1.02 {
			badArea++
		}
	}

	t.Logf("E1M1 subsectors=%d noPoly=%d nonSimple=%d triFail=%d badArea=%d", total, noPoly, nonSimple, triFail, badArea)

	usable := total - noPoly - triFail
	if usable <= 0 {
		t.Fatalf("no usable subsectors from real map: total=%d noPoly=%d triFail=%d", total, noPoly, triFail)
	}
	if badArea != 0 {
		t.Fatalf("triangulated area mismatch on real map: badArea=%d", badArea)
	}
	diag.NoPoly = noPoly
	diag.NonSimple = nonSimple
	diag.TriFail = triFail
	diag.BadArea = badArea
	if os.Getenv("GD_AUTOMAP_WRITE_DIAG") == "1" {
		writeRealMapDiag(t, "e1m1_subsector_diag.json", diag)
	}
}

func TestRealMap_E1M1_SolidVsTexturedEligibility(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)

	total := len(g.m.SubSectors)
	if total == 0 {
		t.Fatal("no subsectors in E1M1")
	}

	solidEligible := 0
	texturedEligible := 0
	missingFlat := 0

	for ss := range g.m.SubSectors {
		sec, ok := g.subSectorSectorIndex(ss)
		if !ok || sec < 0 || sec >= len(g.m.Sectors) {
			continue
		}
		verts, _, ok := mapPolyForSubsectorWithStage(g, ss)
		if !ok || len(verts) < 3 {
			continue
		}
		tris, ok := triangulateWorldPolygon(verts)
		if !ok || len(tris) == 0 {
			tris, ok = triangulateByAngleFan(verts)
		}
		if !ok || len(tris) == 0 {
			continue
		}
		solidEligible++

		if _, ok := g.flatImage(g.m.Sectors[sec].FloorPic); ok {
			texturedEligible++
		} else {
			missingFlat++
		}
	}

	t.Logf("E1M1 eligible solid=%d textured=%d missingFlat=%d", solidEligible, texturedEligible, missingFlat)

	if solidEligible == 0 {
		t.Fatal("no solid-eligible subsectors")
	}
	if texturedEligible != solidEligible {
		t.Fatalf("textured eligibility mismatch: solid=%d textured=%d missingFlat=%d", solidEligible, texturedEligible, missingFlat)
	}
}

func mapPolyForSubsectorWithStage(g *game, ss int) ([]worldPt, string, bool) {
	if ss >= 0 && ss < len(g.subSectorPoly) && len(g.subSectorPoly[ss]) >= 3 {
		stage := "cache"
		if ss < len(g.subSectorPolySrc) {
			switch g.subSectorPolySrc[ss] {
			case subPolySrcPrebuiltLoop:
				stage = "prebuilt_loop"
			case subPolySrcNodes:
				stage = "nodes"
			case subPolySrcWorld:
				stage = "world"
			case subPolySrcConvex:
				stage = "convex"
			case subPolySrcSegList:
				stage = "seglist"
			}
		}
		return g.subSectorPoly[ss], stage, true
	}
	verts, _, _, ok := g.subSectorWorldVertices(ss)
	if !ok || len(verts) < 3 {
		stage := "world"
		verts, _, _, ok = g.subSectorConvexVertices(ss)
		if ok && len(verts) >= 3 {
			return verts, "convex", true
		}
		verts, _, _, ok = g.subSectorVerticesFromSegList(ss)
		if ok && len(verts) >= 3 {
			return verts, "seglist", true
		}
		return nil, stage, false
	}
	return verts, "world", true
}

func mustLoadE1M1GameForMapTextureTests(t *testing.T) *game {
	t.Helper()

	wadPath := findDOOM1WAD(t)
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatalf("load E1M1: %v", err)
	}
	flats, err := doomtex.LoadFlatsRGBA(wf, 0)
	if err != nil {
		t.Fatalf("load flats: %v", err)
	}

	g := newGame(m, Options{
		Width:          1067,
		Height:         960,
		SourcePortMode: true,
		StartInMapMode: true,
		FlatBank:       flats,
		WallTexBank:    map[string]WallTexture{},
	})
	g.syncRenderState()
	g.prepareRenderState()
	return g
}

func findDOOM1WAD(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Prefer full IWADs first. DOOM1.WAD can be the shareware/demo IWAD.
	candidates := []string{
		"DOOM.WAD",
		"doom.wad",
		"DOOM2.WAD",
		"doom2.wad",
		"DOOM1.WAD",
		"doom1.wad",
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
	t.Fatalf("DOOM.WAD/DOOM2.WAD/DOOM1.WAD not found from %s", wd)
	return ""
}

type realMapDiag struct {
	Map        string          `json:"map"`
	SubSectors int             `json:"subsectors"`
	NoPoly     int             `json:"no_poly"`
	NonSimple  int             `json:"non_simple"`
	TriFail    int             `json:"tri_fail"`
	BadArea    int             `json:"bad_area"`
	Failures   []subsectorDiag `json:"failures"`
}

type mapTexLoopDump struct {
	Map        string              `json:"map"`
	Stats      mapTexLoopDumpStats `json:"stats"`
	SubSectors []mapTexLoopSubRec  `json:"subsectors"`
}

type mapTexLoopDumpStats struct {
	OK              int `json:"ok"`
	SegShort        int `json:"seg_short"`
	NoPoly          int `json:"no_poly"`
	NonSimple       int `json:"non_simple"`
	TriFail         int `json:"tri_fail"`
	Orphan          int `json:"orphan"`
	LoopMultiNext   int `json:"loop_multi_next"`
	LoopDeadEnd     int `json:"loop_dead_end"`
	LoopEarlyClose  int `json:"loop_early_close"`
	LoopNoClose     int `json:"loop_no_close"`
	NonConvex       int `json:"non_convex"`
	DegenerateArea  int `json:"degenerate_area"`
	TriAreaMismatch int `json:"tri_area_mismatch"`
}

type mapTexLoopSubRec struct {
	Index             int     `json:"index"`
	FirstSeg          int     `json:"firstseg"`
	NumSegs           int     `json:"numsegs"`
	Sector            int     `json:"sector"`
	PolySource        string  `json:"poly_source"`
	DiagCode          uint8   `json:"diag_code"`
	Diag              string  `json:"diag"`
	LoopDiag          string  `json:"loop_diag"`
	HasPoly           bool    `json:"has_poly"`
	PolyVerts         int     `json:"poly_verts"`
	PolyArea2         float64 `json:"poly_area2"`
	TriCount          int     `json:"tri_count"`
	LoopVerts         int     `json:"loop_verts"`
	LoopHasConnectors bool    `json:"loop_has_connectors"`
	Orphan            bool    `json:"orphan"`
	GeomDiag          string  `json:"geom_diag"`
}

type sectorPlaneTriDiag struct {
	Map             string                 `json:"map"`
	Sectors         int                    `json:"sectors"`
	SubSectors      int                    `json:"subsectors"`
	SectorsWithSubs int                    `json:"sectors_with_subsectors"`
	MissingSectors  []sectorPlaneTriSector `json:"missing_sectors"`
}

type sectorPlaneTriSector struct {
	Sector     int   `json:"sector"`
	TriCount   int   `json:"tri_count"`
	SubSectors []int `json:"subsectors"`
}

func TestRealMap_E1M1_SectorPlaneTriCacheCoverage(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	if len(g.sectorPlaneTris) != len(g.m.Sectors) {
		t.Fatalf("sector plane cache size mismatch: sectors=%d cache=%d", len(g.m.Sectors), len(g.sectorPlaneTris))
	}

	sectorSubs := make([][]int, len(g.m.Sectors))
	for ss := range g.m.SubSectors {
		sec := -1
		if ss < len(g.subSectorSec) {
			sec = g.subSectorSec[ss]
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			if s, ok := g.subSectorSectorIndex(ss); ok {
				sec = s
			}
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			continue
		}
		sectorSubs[sec] = append(sectorSubs[sec], ss)
	}

	diag := sectorPlaneTriDiag{
		Map:        "E1M1",
		Sectors:    len(g.m.Sectors),
		SubSectors: len(g.m.SubSectors),
	}
	missing := make([]sectorPlaneTriSector, 0, 16)
	withSubs := 0
	for sec := range g.m.Sectors {
		if len(sectorSubs[sec]) == 0 {
			continue
		}
		withSubs++
		tc := 0
		if sec < len(g.sectorPlaneTris) {
			tc = len(g.sectorPlaneTris[sec])
		}
		if tc == 0 {
			missing = append(missing, sectorPlaneTriSector{
				Sector:     sec,
				TriCount:   tc,
				SubSectors: append([]int(nil), sectorSubs[sec]...),
			})
			if len(missing) <= 16 {
				t.Logf("missing sector tris: sector=%d subsectors=%v", sec, sectorSubs[sec])
			}
		}
	}
	diag.SectorsWithSubs = withSubs
	diag.MissingSectors = missing
	t.Logf("E1M1 sector plane cache: sectors=%d withSubs=%d missing=%d", len(g.m.Sectors), withSubs, len(missing))

	if os.Getenv("GD_AUTOMAP_WRITE_DIAG") == "1" {
		writeSectorPlaneTriDiag(t, "e1m1_sector_plane_tri_diag.json", diag)
	}
	if len(missing) != 0 {
		t.Fatalf("sector plane cache has missing sectors: %d", len(missing))
	}
}

func TestRealMap_E1M1_MapTextureLoopDiagnosticsDump(t *testing.T) {
	if os.Getenv("GD_AUTOMAP_WRITE_DIAG") != "1" && os.Getenv("GD_AUTOMAP_WRITE_LOOP_DIAG") != "1" {
		t.Skip("set GD_AUTOMAP_WRITE_DIAG=1 (or GD_AUTOMAP_WRITE_LOOP_DIAG=1) to write loop diagnostic JSON")
	}
	g := mustLoadE1M1GameForMapTextureTests(t)
	g.updateMapTextureDiagCache()

	out := mapTexLoopDump{
		Map: "E1M1",
		Stats: mapTexLoopDumpStats{
			OK:              g.mapTexDiagStats.ok,
			SegShort:        g.mapTexDiagStats.segShort,
			NoPoly:          g.mapTexDiagStats.noPoly,
			NonSimple:       g.mapTexDiagStats.nonSimple,
			TriFail:         g.mapTexDiagStats.triFail,
			Orphan:          g.mapTexDiagStats.orphan,
			LoopMultiNext:   g.mapTexDiagStats.loopMultiNext,
			LoopDeadEnd:     g.mapTexDiagStats.loopDeadEnd,
			LoopEarlyClose:  g.mapTexDiagStats.loopEarlyClose,
			LoopNoClose:     g.mapTexDiagStats.loopNoClose,
			NonConvex:       g.mapTexDiagStats.nonConvex,
			DegenerateArea:  g.mapTexDiagStats.degenerateArea,
			TriAreaMismatch: g.mapTexDiagStats.triAreaMismatch,
		},
		SubSectors: make([]mapTexLoopSubRec, 0, len(g.m.SubSectors)),
	}
	for ss := range g.m.SubSectors {
		sub := g.m.SubSectors[ss]
		sec := -1
		if ss < len(g.subSectorSec) {
			sec = g.subSectorSec[ss]
		}
		diagCode := uint8(0)
		if ss < len(g.subSectorDiagCode) {
			diagCode = g.subSectorDiagCode[ss]
		}
		loopDiag := loopDiagOK
		if ss < len(g.subSectorLoopDiag) {
			loopDiag = g.subSectorLoopDiag[ss]
		}
		polyVerts := 0
		polyArea2 := 0.0
		if ss < len(g.subSectorPoly) && len(g.subSectorPoly[ss]) >= 3 {
			polyVerts = len(g.subSectorPoly[ss])
			polyArea2 = polygonArea2(g.subSectorPoly[ss])
		}
		triCount := 0
		if ss < len(g.subSectorTris) {
			triCount = len(g.subSectorTris[ss])
		}
		loopVerts := 0
		if ss < len(g.subSectorLoopVerts) {
			loopVerts = len(g.subSectorLoopVerts[ss])
		}
		polySource := "none"
		if ss < len(g.subSectorPolySrc) {
			polySource = subPolySourceLabel(g.subSectorPolySrc[ss])
		}
		out.SubSectors = append(out.SubSectors, mapTexLoopSubRec{
			Index:             ss,
			FirstSeg:          int(sub.FirstSeg),
			NumSegs:           int(sub.SegCount),
			Sector:            sec,
			PolySource:        polySource,
			DiagCode:          diagCode,
			Diag:              subDiagCodeLabel(diagCode),
			LoopDiag:          loopDiagLabel(loopDiag),
			HasPoly:           polyVerts >= 3,
			PolyVerts:         polyVerts,
			PolyArea2:         polyArea2,
			TriCount:          triCount,
			LoopVerts:         loopVerts,
			LoopHasConnectors: loopVerts > int(sub.SegCount),
			Orphan:            ss < len(g.orphanSubSector) && g.orphanSubSector[ss],
			GeomDiag:          subDiagCodeLabel(g.subSectorLoopGeomDiag(ss)),
		})
	}
	writeMapTexLoopDiag(t, "e1m1_maptex_loop_diag.json", out)
}

func TestRealMap_E1M1_SectorPlaneTriCoverageVsSectorLoops(t *testing.T) {
	if os.Getenv("GD_AUTOMAP_STRICT_SECTOR_COVERAGE") != "1" {
		t.Skip("diagnostic coverage test; set GD_AUTOMAP_STRICT_SECTOR_COVERAGE=1 to enable")
	}
	g := mustLoadE1M1GameForMapTextureTests(t)
	loops := g.buildSectorLoopSets()
	if len(loops) != len(g.m.Sectors) {
		t.Fatalf("sector loop set size mismatch: sectors=%d loops=%d", len(g.m.Sectors), len(loops))
	}
	if len(g.sectorPlaneTris) != len(g.m.Sectors) {
		t.Fatalf("sector plane cache size mismatch: sectors=%d cache=%d", len(g.m.Sectors), len(g.sectorPlaneTris))
	}

	const sampleStep = 8.0
	const minCoverage = 0.995
	const maxLeak = 0.005

	type miss struct {
		sec      int
		inside   int
		covered  int
		outside  int
		leaked   int
		coverage float64
		leak     float64
	}
	bad := make([]miss, 0, 16)

	for sec := range g.m.Sectors {
		set := loops[sec]
		if len(set.rings) == 0 {
			continue
		}
		tris := g.sectorPlaneTris[sec]
		if len(tris) == 0 {
			bad = append(bad, miss{sec: sec})
			continue
		}

		inside := 0
		covered := 0
		outside := 0
		leaked := 0
		for y := set.bbox.minY + sampleStep*0.5; y <= set.bbox.maxY; y += sampleStep {
			for x := set.bbox.minX + sampleStep*0.5; x <= set.bbox.maxX; x += sampleStep {
				p := worldPt{x: x, y: y}
				inLoop := pointInRingsEvenOdd(x, y, set.rings)
				inTri := pointInAnyWorldTri(p, tris)
				if inLoop {
					inside++
					if inTri {
						covered++
					}
					continue
				}
				outside++
				if inTri {
					leaked++
				}
			}
		}
		if inside == 0 {
			continue
		}
		coverage := float64(covered) / float64(inside)
		leak := 0.0
		if outside > 0 {
			leak = float64(leaked) / float64(outside)
		}
		if coverage < minCoverage || leak > maxLeak {
			bad = append(bad, miss{
				sec:      sec,
				inside:   inside,
				covered:  covered,
				outside:  outside,
				leaked:   leaked,
				coverage: coverage,
				leak:     leak,
			})
			if len(bad) <= 20 {
				t.Logf("sector=%d coverage=%.3f leak=%.3f inside=%d covered=%d outside=%d leaked=%d tris=%d", sec, coverage, leak, inside, covered, outside, leaked, len(tris))
			}
		}
	}

	if len(bad) > 0 {
		t.Fatalf("sector plane cache coverage mismatch on %d sectors", len(bad))
	}
}

func TestRealMap_E1M1_SubsectorWorldVsSegListAgreement(t *testing.T) {
	if os.Getenv("GD_AUTOMAP_STRICT_SUBSECTOR_LOOP_AGREEMENT") != "1" {
		t.Skip("diagnostic loop-agreement test; set GD_AUTOMAP_STRICT_SUBSECTOR_LOOP_AGREEMENT=1 to enable")
	}
	g := mustLoadE1M1GameForMapTextureTests(t)
	mismatch := 0
	for ss := range g.m.SubSectors {
		sub := g.m.SubSectors[ss]
		if sub.SegCount < 3 {
			continue
		}
		wv, _, _, wok := g.subSectorWorldVertices(ss)
		sv, _, _, sok := g.subSectorVerticesFromSegList(ss)
		switch {
		case wok != sok:
			mismatch++
			if mismatch <= 24 {
				t.Logf("ss=%d segs=%d world_ok=%t seglist_ok=%t", ss, sub.SegCount, wok, sok)
			}
			continue
		case !wok || !sok:
			continue
		}

		aw := math.Abs(polygonArea2(wv)) * 0.5
		as := math.Abs(polygonArea2(sv)) * 0.5
		if aw <= 1e-6 || as <= 1e-6 {
			mismatch++
			if mismatch <= 24 {
				t.Logf("ss=%d segs=%d degenerate area world=%.6f seglist=%.6f", ss, sub.SegCount, aw, as)
			}
			continue
		}
		r := aw / as
		if r < 0.98 || r > 1.02 {
			mismatch++
			if mismatch <= 24 {
				t.Logf("ss=%d segs=%d area mismatch world=%.3f seglist=%.3f ratio=%.3f", ss, sub.SegCount, aw, as, r)
			}
		}
	}
	t.Logf("subsector world-vs-seglist mismatches=%d total=%d", mismatch, len(g.m.SubSectors))
	if mismatch > 0 {
		t.Fatalf("world subsector reconstruction diverges from seg-order on %d subsectors", mismatch)
	}
}

func pointInAnyWorldTri(p worldPt, tris []worldTri) bool {
	const eps = 1e-6
	for _, tri := range tris {
		area := orient2D(tri.a, tri.b, tri.c)
		if math.Abs(area) <= eps {
			continue
		}
		if pointInWorldTri(p, tri.a, tri.b, tri.c, eps) {
			return true
		}
	}
	return false
}

func pointInWorldTri(p, a, b, c worldPt, eps float64) bool {
	o1 := orient2D(a, b, p)
	o2 := orient2D(b, c, p)
	o3 := orient2D(c, a, p)
	hasNeg := o1 < -eps || o2 < -eps || o3 < -eps
	hasPos := o1 > eps || o2 > eps || o3 > eps
	return !(hasNeg && hasPos)
}

type subsectorDiag struct {
	Index    int          `json:"index"`
	Reason   string       `json:"reason"`
	Stage    string       `json:"stage"`
	SegCount int          `json:"seg_count"`
	FirstSeg int          `json:"first_seg"`
	Segs     [][2]uint16  `json:"segs"`
	Verts    [][2]float64 `json:"verts,omitempty"`
}

func buildSubsectorDiag(g *game, ss int, stage, reason string, verts []worldPt) subsectorDiag {
	d := subsectorDiag{
		Index:  ss,
		Reason: reason,
		Stage:  stage,
	}
	if ss < 0 || ss >= len(g.m.SubSectors) {
		return d
	}
	sub := g.m.SubSectors[ss]
	d.SegCount = int(sub.SegCount)
	d.FirstSeg = int(sub.FirstSeg)
	d.Segs = make([][2]uint16, 0, d.SegCount)
	for i := 0; i < d.SegCount; i++ {
		si := d.FirstSeg + i
		if si < 0 || si >= len(g.m.Segs) {
			continue
		}
		sg := g.m.Segs[si]
		d.Segs = append(d.Segs, [2]uint16{sg.StartVertex, sg.EndVertex})
	}
	if len(verts) > 0 {
		d.Verts = make([][2]float64, 0, len(verts))
		for _, v := range verts {
			d.Verts = append(d.Verts, [2]float64{v.x, v.y})
		}
	}
	return d
}

func writeRealMapDiag(t *testing.T, name string, diag realMapDiag) {
	t.Helper()
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	b, err := json.MarshalIndent(diag, "", "  ")
	if err != nil {
		t.Fatalf("marshal diag: %v", err)
	}
	p := filepath.Join("testdata", name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("write diag: %v", err)
	}
	t.Logf("wrote %s", p)
}

func writeSectorPlaneTriDiag(t *testing.T, name string, diag sectorPlaneTriDiag) {
	t.Helper()
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	path := filepath.Join("testdata", name)
	b, err := json.MarshalIndent(diag, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	t.Logf("wrote %s", path)
}

func writeMapTexLoopDiag(t *testing.T, name string, diag mapTexLoopDump) {
	t.Helper()
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	path := filepath.Join("testdata", name)
	b, err := json.MarshalIndent(diag, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	t.Logf("wrote %s", path)
}

func subDiagCodeLabel(code uint8) string {
	switch code {
	case subDiagOK:
		return "ok"
	case subDiagSegShort:
		return "seg_short"
	case subDiagNoPoly:
		return "no_poly"
	case subDiagNonSimple:
		return "non_simple"
	case subDiagTriFail:
		return "tri_fail"
	case subDiagLoopMultiNext:
		return "loop_multi_next"
	case subDiagLoopDeadEnd:
		return "loop_dead_end"
	case subDiagLoopEarlyClose:
		return "loop_early_close"
	case subDiagLoopNoClose:
		return "loop_no_close"
	case subDiagNonConvex:
		return "non_convex"
	case subDiagDegenerateArea:
		return "degenerate_area"
	case subDiagTriAreaMismatch:
		return "tri_area_mismatch"
	default:
		return "unknown"
	}
}

func loopDiagLabel(code loopBuildDiag) string {
	switch code {
	case loopDiagOK:
		return "ok"
	case loopDiagMultipleCandidates:
		return "multiple_candidates"
	case loopDiagDeadEnd:
		return "dead_end"
	case loopDiagEarlyClose:
		return "early_close"
	case loopDiagNoClose:
		return "no_close"
	default:
		return "unknown"
	}
}

func subPolySourceLabel(src uint8) string {
	switch src {
	case subPolySrcNone:
		return "none"
	case subPolySrcPrebuiltLoop:
		return "prebuilt_loop"
	case subPolySrcWorld:
		return "world"
	case subPolySrcConvex:
		return "convex"
	case subPolySrcSegList:
		return "seglist"
	case subPolySrcNodes:
		return "nodes"
	default:
		return "unknown"
	}
}
