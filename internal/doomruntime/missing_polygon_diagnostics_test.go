package doomruntime

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func TestRealMap_E1M1_ShortLeafPolygonCoverage(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	checked := 0
	for ss, sub := range g.m.SubSectors {
		if sub.SegCount == 0 || sub.SegCount >= 3 {
			continue
		}
		checked++
		poly := g.subSectorPoly[ss]
		if len(poly) < 3 || len(g.subSectorTris[ss]) == 0 {
			t.Errorf("subsector %d with %d SEGs has no polygon or triangles", ss, sub.SegCount)
			continue
		}
		if !polygonSimple(poly) || math.Abs(polygonArea2(poly)) < 1e-6 {
			t.Errorf("subsector %d has invalid polygon", ss)
		}
		c, ok := worldPolygonCentroid(poly)
		if !ok || g.subSectorAtFixed(worldToFixed(c.x), worldToFixed(c.y)) != ss {
			t.Errorf("subsector %d polygon centroid belongs to another BSP leaf", ss)
		}
	}
	t.Logf("validated %d short BSP leaves", checked)
}

// Audit every map in locally supplied WADs; missing polygons are reported rather
// than universally rejected because malformed and unsupported leaves can exist.
func TestRealMaps_SubsectorPolygonCoverageDiagnostics(t *testing.T) {
	paths := os.Getenv("GD_GEOMETRY_WADS")
	if paths == "" {
		t.Skip("set GD_GEOMETRY_WADS to comma-separated IWAD paths")
	}
	marker := regexp.MustCompile(`^(E[1-9]M[1-9]|MAP[0-9][0-9])$`)
	for _, path := range strings.Split(paths, ",") {
		path = strings.TrimSpace(path)
		wf, err := wad.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		maps := 0
		for _, lump := range wf.Lumps {
			if !marker.MatchString(lump.Name) {
				continue
			}
			m, err := mapdata.LoadMap(wf, mapdata.MapName(lump.Name))
			if err != nil {
				t.Fatal(err)
			}
			g := &game{m: m, bounds: mapBounds(m)}
			g.initSubSectorSectorCache()
			missing, shortMissing, shortTotal := 0, 0, 0
			for ss, sub := range m.SubSectors {
				short := sub.SegCount > 0 && sub.SegCount < 3
				if short {
					shortTotal++
				}
				if len(g.subSectorPoly[ss]) < 3 || len(g.subSectorTris[ss]) == 0 {
					missing++
					if short {
						shortMissing++
					}
				}
			}
			t.Logf("%s %s: subsectors=%d short=%d missing=%d shortMissing=%d", filepath.Base(path), m.Name, len(m.SubSectors), shortTotal, missing, shortMissing)
			maps++
		}
		if maps == 0 {
			t.Errorf("%s has no supported maps", path)
		}
	}
}
