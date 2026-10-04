// Package raymesh prepares the experimental Doom mesh for resident GPU buffers.
// The Raylib backend is selected with the raylib build tag and requires cgo.
package raymesh

import (
	"math"

	"gddoom/internal/render/levelmesh"
)

const WorldScale = float32(1.0 / 64)

type TextureKey struct {
	Pixels        *byte
	Width, Height int
}

type BatchKey struct {
	Texture TextureKey
	Masked  bool
}

type Batch struct {
	Key            BatchKey
	Texture        levelmesh.Texture
	Positions, UVs []float32
	Colors         []byte
	generation     uint64
}

type Builder struct {
	batches    map[BatchKey]*Batch
	active     []*Batch
	generation uint64
}

var missingTexture = func() levelmesh.Texture {
	p := make([]byte, 64*64*4)
	for y := range 64 {
		for x := range 64 {
			i := (y*64 + x) * 4
			p[i], p[i+1], p[i+2], p[i+3] = 210, 45, 190, 255
			if (x/16+y/16)&1 == 0 {
				p[i], p[i+1], p[i+2] = 45, 10, 45
			}
		}
	}
	return levelmesh.Texture{RGBA: p, Width: 64, Height: 64}
}()

// Build reuses CPU staging buffers; equal batches need no GPU updates. Texture
// identity includes the resolved animated/switch frame, shared across sidedefs.
func (b *Builder) Build(tris []levelmesh.Triangle, texture func(levelmesh.Triangle) levelmesh.Texture, light func(int) float64, mode levelmesh.Mode) []*Batch {
	if b.batches == nil {
		b.batches = make(map[BatchKey]*Batch)
	}
	b.generation++
	b.active = b.active[:0]
	for _, tri := range tris {
		if tri.Sky {
			continue
		}
		tex := texture(tri)
		if tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
			tex = missingTexture
		}
		key := BatchKey{Texture: TextureKey{&tex.RGBA[0], tex.Width, tex.Height}, Masked: tri.Masked}
		batch := b.batches[key]
		if batch == nil {
			batch = &Batch{Key: key, Texture: tex}
			b.batches[key] = batch
		}
		if batch.generation != b.generation {
			batch.Positions, batch.UVs, batch.Colors = batch.Positions[:0], batch.UVs[:0], batch.Colors[:0]
			batch.generation = b.generation
			b.active = append(b.active, batch)
		}
		shade := math.Max(0, math.Min(1, light(tri.Sector)))
		red, green, blue := byte(255), byte(255), byte(255)
		if mode != levelmesh.Textured {
			hash := uint32(tri.Sector+1) * 2654435761
			red, green, blue = 80+byte(hash&127), 80+byte((hash>>8)&127), 80+byte((hash>>16)&127)
		}
		if mode == levelmesh.Wireframe {
			shade = 1
		}
		red, green, blue = byte(float64(red)*shade), byte(float64(green)*shade), byte(float64(blue)*shade)
		for _, v := range tri.Vertices {
			// (X,Y,Z-up) -> (X,Y-up,Z) with positive determinant: preserve
			// inward-facing winding while matching the CPU camera's handedness.
			batch.Positions = append(batch.Positions, float32(v.X)*WorldScale, float32(v.Z)*WorldScale, -float32(v.Y)*WorldScale)
			batch.UVs = append(batch.UVs, float32(v.U)/float32(tex.Width), float32(v.V)/float32(tex.Height))
			batch.Colors = append(batch.Colors, red, green, blue, 255)
		}
	}
	return b.active
}
