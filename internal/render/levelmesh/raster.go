package levelmesh

import "math"

type Mode string

const (
	Textured  Mode = "textured"
	Sectors   Mode = "sectors"
	Wireframe Mode = "wireframe"
)

type Texture struct {
	RGBA                                    []byte
	Width, Height                           int
	Indexed                                 []byte // Optional original WAD indices, preserving duplicate palette colors.
	FixedRGBA                               []byte // Optional fixed-colormap variant; RGBA remains the stable texture identity.
	BlendRGBA, BlendIndexed, BlendFixedRGBA []byte // Optional second frame with matching dimensions.
	BlendAlpha                              uint8  // Shared animation/switch weight, 0 disables blending.
	BlendInstance                           int    // Independent switch timeline; 0 shares the global animation phase.
}

func (t Texture) HasBlend() bool {
	return t.BlendAlpha != 0 && t.Width > 0 && t.Height > 0 && len(t.BlendRGBA) == t.Width*t.Height*4
}

func (t Texture) BlendTexture() Texture {
	return Texture{RGBA: t.BlendRGBA, Indexed: t.BlendIndexed, FixedRGBA: t.BlendFixedRGBA, Width: t.Width, Height: t.Height}
}

type Camera struct{ X, Y, Z, Yaw, Focal, FocalY float64 }
type Rasterizer struct {
	Pixels        []byte
	Depth         []float64 // Reciprocal depth: zero means untouched.
	Width, Height int
	Drawn         int
}

type cameraVertex struct{ x, y, z, u, v float64 }

// Render performs near/frustum clipping, backface culling, perspective-correct
// sampling and alpha testing. Texture lookup happens once per source triangle.
// It has no dependency on game state or Ebiten, making depth behavior testable.
func (r *Rasterizer) Render(tris []Triangle, w, h int, c Camera, mode Mode, texture func(Triangle) Texture, light func(int) float64) {
	if w <= 0 || h <= 0 {
		return
	}
	if r.Width != w || r.Height != h {
		r.Width, r.Height = w, h
		r.Pixels = make([]byte, w*h*4)
		r.Depth = make([]float64, w*h)
	}
	clear(r.Depth)
	for i := 0; i < len(r.Pixels); i += 4 {
		r.Pixels[i], r.Pixels[i+1], r.Pixels[i+2], r.Pixels[i+3] = 42, 49, 65, 255
	}
	r.Drawn = 0
	ca, sa := math.Cos(c.Yaw), math.Sin(c.Yaw)
	if c.Focal <= 0 {
		c.Focal = float64(w) / 2
	}
	if c.FocalY <= 0 {
		c.FocalY = c.Focal
	}
	// Five half-spaces bound every projected triangle before rasterization.
	planes := [5][4]float64{{0, 0, 1, -2}, {1, 0, float64(w) / 2 / c.Focal, 0}, {-1, 0, float64(w) / 2 / c.Focal, 0}, {0, 1, float64(h) / 2 / c.FocalY, 0}, {0, -1, float64(h) / 2 / c.FocalY, 0}}
	for _, tri := range tris {
		if tri.Sky {
			continue
		}
		a, b, d := tri.Vertices[0], tri.Vertices[1], tri.Vertices[2]
		bx, by, bz := b.X-a.X, b.Y-a.Y, b.Z-a.Z
		dx, dy, dz := d.X-a.X, d.Y-a.Y, d.Z-a.Z
		if (by*dz-bz*dy)*(c.X-a.X)+(bz*dx-bx*dz)*(c.Y-a.Y)+(bx*dy-by*dx)*(c.Z-a.Z) <= 0 {
			continue
		}
		var bufA, bufB [12]cameraVertex
		poly := bufA[:3]
		work := bufB[:0]
		for i, v := range tri.Vertices {
			x, y := v.X-c.X, v.Y-c.Y
			poly[i] = cameraVertex{x*sa - y*ca, c.Z - v.Z, x*ca + y*sa, v.U, v.V}
		}
		for _, p := range planes {
			work = work[:0]
			prev := poly[len(poly)-1]
			pd := p[0]*prev.x + p[1]*prev.y + p[2]*prev.z + p[3]
			for _, cur := range poly {
				cd := p[0]*cur.x + p[1]*cur.y + p[2]*cur.z + p[3]
				if (pd >= 0) != (cd >= 0) {
					t := pd / (pd - cd)
					work = append(work, cameraVertex{prev.x + (cur.x-prev.x)*t, prev.y + (cur.y-prev.y)*t, prev.z + (cur.z-prev.z)*t, prev.u + (cur.u-prev.u)*t, prev.v + (cur.v-prev.v)*t})
				}
				if cd >= 0 {
					work = append(work, cur)
				}
				prev, pd = cur, cd
			}
			poly, work = work, poly[:0]
			if len(poly) < 3 {
				break
			}
		}
		if len(poly) < 3 {
			continue
		}
		tex := Texture{}
		if texture != nil {
			tex = texture(tri)
		}
		shade := 1.0
		if light != nil {
			shade = math.Max(0, math.Min(1, light(tri.Sector)))
		}
		for i := 1; i+1 < len(poly); i++ {
			r.triangle([3]cameraVertex{poly[0], poly[i], poly[i+1]}, c, tri, tex, shade, mode)
		}
		r.Drawn++
	}
}

func (r *Rasterizer) triangle(v [3]cameraVertex, c Camera, tri Triangle, tex Texture, shade float64, mode Mode) {
	var x, y, q, u, t [3]float64
	for i, p := range v {
		q[i] = 1 / p.z
		x[i] = float64(r.Width)/2 + p.x*c.Focal*q[i]
		y[i] = float64(r.Height)/2 + p.y*c.FocalY*q[i]
		u[i] = p.u * q[i]
		t[i] = p.v * q[i]
	}
	area := (x[1]-x[0])*(y[2]-y[0]) - (y[1]-y[0])*(x[2]-x[0])
	if math.Abs(area) < 1e-8 {
		return
	}
	x0 := max(0, int(math.Ceil(min(x[0], x[1], x[2])-0.5)))
	x1 := min(r.Width-1, int(math.Floor(max(x[0], x[1], x[2])-0.5)))
	y0 := max(0, int(math.Ceil(min(y[0], y[1], y[2])-0.5)))
	y1 := min(r.Height-1, int(math.Floor(max(y[0], y[1], y[2])-0.5)))
	dw0x, dw0y := (y[1]-y[2])/area, (x[2]-x[1])/area
	dw1x, dw1y := (y[2]-y[0])/area, (x[0]-x[2])/area
	edge0 := math.Hypot(dw0x, dw0y)
	edge1 := math.Hypot(dw1x, dw1y)
	edge2 := math.Hypot(dw0x+dw1x, dw0y+dw1y)
	blend := tex.HasBlend()
	blendAlpha := uint32(tex.BlendAlpha)
	validTex := tex.Width > 0 && tex.Height > 0 && len(tex.RGBA) == tex.Width*tex.Height*4
	for py := y0; py <= y1; py++ {
		px0 := float64(x0) + 0.5
		fy := float64(py) + 0.5
		w0 := ((x[1]-px0)*(y[2]-fy) - (y[1]-fy)*(x[2]-px0)) / area
		w1 := ((x[2]-px0)*(y[0]-fy) - (y[2]-fy)*(x[0]-px0)) / area
		for px := x0; px <= x1; px++ {
			w2 := 1 - w0 - w1
			if w0 >= -1e-9 && w1 >= -1e-9 && w2 >= -1e-9 {
				depth := w0*q[0] + w1*q[1] + w2*q[2]
				idx := py*r.Width + px
				if depth > r.Depth[idx]+1e-12 {
					uv := (w0*u[0] + w1*u[1] + w2*u[2]) / depth
					vv := (w0*t[0] + w1*t[1] + w2*t[2]) / depth
					red, green, blue := byte(210), byte(45), byte(190)
					visible := true
					if validTex {
						tu := wrap(int(math.Floor(uv)), tex.Width)
						tv := wrap(int(math.Floor(vv)), tex.Height)
						ti := (tv*tex.Width + tu) * 4
						if tri.Masked && tex.RGBA[ti+3] < 128 {
							visible = false
						}
						red, green, blue = tex.RGBA[ti], tex.RGBA[ti+1], tex.RGBA[ti+2]
						if blend {
							visible = !tri.Masked || tex.RGBA[ti+3] >= 128 || tex.BlendRGBA[ti+3] >= 128
							a := blendAlpha
							red = byte((uint32(red)*(255-a) + uint32(tex.BlendRGBA[ti])*a + 127) / 255)
							green = byte((uint32(green)*(255-a) + uint32(tex.BlendRGBA[ti+1])*a + 127) / 255)
							blue = byte((uint32(blue)*(255-a) + uint32(tex.BlendRGBA[ti+2])*a + 127) / 255)
						}
					} else if (int(math.Floor(uv/16))+int(math.Floor(vv/16)))&1 == 0 {
						red, green, blue = 45, 10, 45
					}
					if visible {
						if mode == Sectors {
							hash := uint32(tri.Sector+1) * 2654435761
							red, green, blue = 80+byte(hash&127), 80+byte((hash>>8)&127), 80+byte((hash>>16)&127)
						}
						if mode == Wireframe {
							red, green, blue = 12, 17, 24
							if w0 <= edge0*0.8 || w1 <= edge1*0.8 || w2 <= edge2*0.8 {
								red, green, blue = 90, 230, 200
							}
							shade = 1
						}
						r.Depth[idx] = depth
						p := idx * 4
						r.Pixels[p], r.Pixels[p+1], r.Pixels[p+2] = byte(float64(red)*shade), byte(float64(green)*shade), byte(float64(blue)*shade)
					}
				}
			}
			w0 += dw0x
			w1 += dw1x
		}
	}
}

func wrap(x, n int) int {
	x %= n
	if x < 0 {
		x += n
	}
	return x
}
