package doomruntime

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"gddoom/internal/render/levelmesh"
)

func meshPlaneHits(x, y float64, tris []levelmesh.PlaneTriangle) int {
	hits := 0
	for _, tri := range tris {
		inside := true
		for i, a := range tri {
			b := tri[(i+1)%3]
			if (b.X-a.X)*(y-a.Y)-(b.Y-a.Y)*(x-a.X) < -1e-7 {
				inside = false
				break
			}
		}
		if inside {
			hits++
		}
	}
	return hits
}

func TestMeshExperimentE1M3ExitRoomBoundaries(t *testing.T) {
	g := loadMeshExperimentMap(t, "E1M3")
	beforeMap, _ := json.Marshal(g.m)
	beforeCache := make([][]worldTri, len(g.sectorPlaneTris))
	for i, tris := range g.sectorPlaneTris {
		if tris != nil {
			beforeCache[i] = append(make([]worldTri, 0, len(tris)), tris...)
		}
	}
	beforeSim := g.SimChecksum()
	r := g.ensureMeshExperiment()
	const sector = 7
	if r.fallback != 0 {
		t.Fatalf("%d plane fallbacks", r.fallback)
	}
	if pointInRingsEvenOdd(-400, -1600, g.buildSectorLoopSets()[sector].rings) {
		t.Fatal("fixture no longer reproduces the missing exit-room boundary")
	}
	filled, checked := 0, 0
	for y := -2000.25; y < -1200; y += 3.7 {
		for x := -600.125; x < 400; x += 4.1 {
			want := automapBoundaryContains(g.m, sector, x, y)
			hits := meshPlaneHits(x, y, r.planes[sector])
			if (hits > 0) != want || hits > 1 {
				t.Fatalf("exit room (%g,%g): hits=%d want %t", x, y, hits, want)
			}
			if want {
				filled++
			}
			checked++
		}
	}
	if filled < 10000 {
		t.Fatalf("only %d exit-room samples filled", filled)
	}
	// This point is in the old BSP leaf but outside the diagonal wall.
	if meshPlaneHits(-509.125, -1915.25, r.planes[sector]) != 0 {
		t.Fatal("mesh spills outside exit-room wall")
	}
	g.renderMeshExperiment(r)
	counts := map[levelmesh.Kind]int{}
	for _, tri := range r.triangles {
		if tri.Sector != sector || (tri.Kind != levelmesh.Floor && tri.Kind != levelmesh.Ceiling) {
			continue
		}
		counts[tri.Kind]++
		wantZ, wantTex := float64(g.m.Sectors[sector].FloorHeight), g.m.Sectors[sector].FloorPic
		if tri.Kind == levelmesh.Ceiling {
			wantZ, wantTex = float64(g.m.Sectors[sector].CeilingHeight), g.m.Sectors[sector].CeilingPic
		}
		for _, v := range tri.Vertices {
			if v.Z != wantZ || v.U != v.X || v.V != v.Y || tri.Texture != wantTex {
				t.Fatalf("incorrect plane height, texture or UV: %+v", tri)
			}
		}
	}
	if counts[levelmesh.Floor] != len(r.planes[sector]) || counts[levelmesh.Ceiling] != len(r.planes[sector]) {
		t.Fatalf("floor/ceiling did not use repaired planes: %v", counts)
	}
	afterMap, _ := json.Marshal(g.m)
	if !bytes.Equal(beforeMap, afterMap) || !reflect.DeepEqual(beforeCache, g.sectorPlaneTris) || beforeSim != g.SimChecksum() {
		t.Fatal("mesh boundary construction or rendering changed gameplay data")
	}
	t.Logf("%d samples verified; %d floor and ceiling triangles", checked, len(r.planes[sector]))
}

// Cast independent pixel-center rays at the repaired planes. Both horizontal
// surfaces must rasterize with the right depth and world-space texture UVs at
// normal source-port resolution, rather than merely existing in the cache.
func TestMeshExperimentE1M3ExitRoomPlaneRaster(t *testing.T) {
	g := loadMeshExperimentMap(t, "E1M3")
	r := g.ensureMeshExperiment()
	g.renderMeshExperiment(r)
	const w, h = 1280, 800
	camera := levelmesh.Camera{X: -128, Y: -1800, Z: 89, Yaw: math.Pi / 2, Focal: 640, FocalY: 640}
	for _, kind := range []levelmesh.Kind{levelmesh.Floor, levelmesh.Ceiling} {
		var faces []levelmesh.Triangle
		for _, tri := range r.triangles {
			if tri.Sector == 7 && tri.Kind == kind {
				faces = append(faces, tri)
			}
		}
		var raster levelmesh.Rasterizer
		raster.Render(faces, w, h, camera, levelmesh.Textured, func(tri levelmesh.Triangle) levelmesh.Texture { return g.meshMaterial(r, tri) }, nil)
		z := faces[0].Vertices[0].Z
		tex := g.meshMaterial(r, faces[0])
		checked := 0
		for py := 1; py < h; py += 3 {
			depth := (camera.Z - z) * camera.FocalY / (float64(py) + 0.5 - h/2)
			if depth < 2 {
				continue
			}
			for px := 1; px < w; px += 3 {
				x, y := camera.X+(float64(px)+0.5-w/2)*depth/camera.Focal, camera.Y+depth
				want := automapBoundaryContains(g.m, 7, x, y)
				i := py*w + px
				if (raster.Depth[i] > 0) != want {
					t.Fatalf("kind %d pixel (%d,%d) at (%g,%g): missing or exterior plane", kind, px, py, x, y)
				}
				if !want {
					continue
				}
				if math.Abs(raster.Depth[i]-1/depth) > 1e-9 {
					t.Fatalf("kind %d incorrect depth at (%d,%d)", kind, px, py)
				}
				// Skip texel boundaries where roundoff can select either neighbor.
				if math.Abs(x-math.Round(x)) > 1e-7 && math.Abs(y-math.Round(y)) > 1e-7 {
					ti := ((int(math.Floor(y))&63)*64 + (int(math.Floor(x)) & 63)) * 4
					if !bytes.Equal(raster.Pixels[i*4:i*4+3], tex.RGBA[ti:ti+3]) {
						t.Fatalf("kind %d incorrect UV at (%d,%d)", kind, px, py)
					}
				}
				checked++
			}
		}
		if checked < 1000 {
			t.Fatalf("kind %d only checked %d filled pixels", kind, checked)
		}
		t.Logf("kind %d: %d plane pixels verified at 1280x800", kind, checked)
	}
}
