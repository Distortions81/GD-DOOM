package automap

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
	dir := wd
	for i := 0; i < 8; i++ {
		cand := filepath.Join(dir, "DOOM1.WAD")
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	t.Fatalf("DOOM1.WAD not found from %s", wd)
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
