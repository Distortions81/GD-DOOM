package doomruntime

import (
	"fmt"

	"gddoom/internal/render/levelmesh"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type experimentalMeshRenderer struct {
	mode      levelmesh.Mode
	planes    [][]levelmesh.PlaneTriangle
	heights   []levelmesh.Heights
	triangles []levelmesh.Triangle
	raster    levelmesh.Rasterizer
	image     *ebiten.Image
	missing   int
	fallback  int
	// Materials are resolved once per frame, shared by both geometry and raster.
	materials map[meshMaterialKey]levelmesh.Texture
}

type meshMaterialKey struct {
	name string
	side int
	kind levelmesh.Kind
}

func (g *game) ensureMeshExperiment() *experimentalMeshRenderer {
	if g.opts.MeshRenderer == "" {
		return nil
	}
	if g.meshExperiment != nil {
		return g.meshExperiment
	}
	r := &experimentalMeshRenderer{mode: levelmesh.Mode(g.opts.MeshRenderer), planes: make([][]levelmesh.PlaneTriangle, len(g.m.Sectors)), heights: make([]levelmesh.Heights, len(g.m.Sectors)), materials: make(map[meshMaterialKey]levelmesh.Texture)}
	// Use the same original directed sides as the minimap. Ring extraction can
	// silently discard an outer room while retaining its pillars (E1M3 sector
	// 7), so successful ring triangulation does not prove complete geometry.
	g.ensureMapFloorBoundarySetsBuilt()
	for sector, set := range g.mapFloorBoundarySets {
		edges := make([]levelmesh.PlaneEdge, 0, len(set.Edges))
		for _, e := range set.Edges {
			edges = append(edges, levelmesh.PlaneEdge{A: levelmesh.Point2{X: e.A.X, Y: e.A.Y}, B: levelmesh.Point2{X: e.B.X, Y: e.B.Y}})
		}
		tris, err := levelmesh.TriangulateEdges(edges)
		if err == nil {
			r.planes[sector] = tris
			continue
		}
		r.fallback++
		if sector < len(g.sectorPlaneTris) {
			for _, t := range g.sectorPlaneTris[sector] {
				r.planes[sector] = append(r.planes[sector], levelmesh.PlaneTriangle{{X: t.a.x, Y: t.a.y}, {X: t.b.x, Y: t.b.y}, {X: t.c.x, Y: t.c.y}})
			}
		}
	}
	for ss := range g.m.SubSectors {
		if len(g.subSectorPoly[ss]) < 3 || len(g.subSectorTris[ss]) == 0 {
			r.missing++
		}
	}
	g.meshExperiment = r
	return r
}

func (g *game) updateMeshExperiment() {
	if g.opts.MeshRenderer == "" || g.chatComposeOpen || !g.keyJustPressed(ebiten.KeyF7) {
		return
	}
	r := g.ensureMeshExperiment()
	switch r.mode {
	case levelmesh.Textured:
		r.mode = levelmesh.Sectors
	case levelmesh.Sectors:
		r.mode = levelmesh.Wireframe
	case levelmesh.Wireframe:
		r.mode = ""
	default:
		r.mode = levelmesh.Textured
	}
}

func meshTextureSlot(kind levelmesh.Kind) switchTextureSlot {
	switch kind {
	case levelmesh.Upper:
		return switchTextureSlotTop
	case levelmesh.Lower:
		return switchTextureSlotBottom
	default:
		return switchTextureSlotMid
	}
}

func (g *game) meshMaterial(r *experimentalMeshRenderer, t levelmesh.Triangle) levelmesh.Texture {
	key := meshMaterialKey{t.Texture, t.Sidedef, t.Kind}
	if tex, ok := r.materials[key]; ok {
		return tex
	}
	tex := levelmesh.Texture{}
	if t.Kind == levelmesh.Floor || t.Kind == levelmesh.Ceiling {
		if sample, ok := g.flatTextureBlend(t.Texture); ok {
			tex = levelmesh.Texture{RGBA: sample.fromRGBA, Width: 64, Height: 64, Indexed: sample.fromIndexed}
			tex.BlendRGBA, tex.BlendIndexed, tex.BlendAlpha = sample.toRGBA, sample.toIndexed, sample.alpha
		}
	} else if sample, ok := g.wallTextureBlend(t.Texture, t.Sidedef, meshTextureSlot(t.Kind)); ok && sample.from != nil {
		tex = nativeTexture(sample.from)
		if sample.to != nil {
			tex.BlendRGBA, tex.BlendIndexed, tex.BlendAlpha = sample.to.RGBA, sample.to.Indexed, sample.alpha
			if blend := g.switchTextureBlendFor(t.Sidedef, meshTextureSlot(t.Kind), t.Texture); blend.fromKey != "" {
				tex.BlendInstance = t.Sidedef*3 + int(meshTextureSlot(t.Kind)) + 1
			}
		}
	}
	r.materials[key] = tex
	return tex
}

func (g *game) drawWorld3D(screen *ebiten.Image) {
	r := g.ensureMeshExperiment()
	if r == nil || r.mode == "" {
		g.drawDoomBasic3D(screen)
		if r != nil {
			ebitenutil.DebugPrintAt(screen, "Mesh experiment: classic | F7 cycle", 8, 8)
		}
		return
	}
	g.renderMeshExperiment(r)
	if r.image == nil || r.image.Bounds().Dx() != g.viewW || r.image.Bounds().Dy() != g.viewH {
		if r.image != nil {
			r.image.Deallocate()
		}
		r.image = ebiten.NewImage(g.viewW, g.viewH)
	}
	r.image.WritePixels(r.raster.Pixels)
	screen.DrawImage(r.image, nil)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Mesh: %s | F7 cycle | %d tris (%d submitted)\nPlane fallbacks: %d | geometry only", r.mode, len(r.triangles), r.raster.Drawn, r.fallback), 8, 8)
}

func (g *game) buildMeshExperimentGeometry(r *experimentalMeshRenderer) {
	clear(r.materials)
	for sector, s := range g.m.Sectors {
		h := levelmesh.Heights{Floor: float64(s.FloorHeight), Ceiling: float64(s.CeilingHeight)}
		if f, c, ok := g.sectorHeightRenderSnapshot(sector); ok {
			h.Floor, h.Ceiling = float64(f)/fracUnit, float64(c)/fracUnit
		}
		r.heights[sector] = h
	}
	r.triangles = levelmesh.Build(r.triangles, g.m, r.planes, r.heights, func(name string, side int, kind levelmesh.Kind) float64 {
		return float64(g.meshMaterial(r, levelmesh.Triangle{Texture: name, Sidedef: side, Kind: kind}).Height)
	}, func(special uint16) float64 { return wallSpecialScrollXOffset(special, g.worldTic) })
}

func (g *game) renderMeshExperiment(r *experimentalMeshRenderer) {
	g.buildMeshExperimentGeometry(r)
	r.raster.Render(r.triangles, g.viewW, g.viewH, levelmesh.Camera{X: g.renderPX, Y: g.renderPY, Z: g.playerEyeZ(), Yaw: angleToRadians(g.renderAngle), Focal: doomFocalLength(g.viewW), FocalY: g.verticalFocalLength(doomFocalLength(g.viewW))}, r.mode, func(t levelmesh.Triangle) levelmesh.Texture { return g.meshMaterial(r, t) }, func(sector int) float64 {
		return float64(sectorLightMul(g.sectorLightForRender(sector, &g.m.Sectors[sector]))) / 256
	})
}
