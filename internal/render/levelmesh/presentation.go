package levelmesh

// Sprite is a view-facing Doom patch at a world-space origin (Z up).
// OffsetX/OffsetY locate the patch's top-left relative to that origin.
type Sprite struct {
	Texture                  Texture
	X, Y, Z                  float64
	OffsetX, OffsetY, ScaleY float64
	Light                    float64
	Flip, Fullbright, Shadow bool
	Name                     string
}

// Patch is an unlit overlay in logical Doom coordinates, with offsets baked in.
type Patch struct {
	Texture    Texture
	X, Y, W, H float64
	Alpha      float64 // Optional opacity; zero retains the default opaque tint.
}

// FuzzSpan is an opaque spectre post on Doom's logical 320x200 grid.
type FuzzSpan struct{ X, Y0, Y1, Phase int }

type FuzzColors struct {
	Palette, Remap, Lookup Texture
	Offsets                [50]float32
	Probes                 int
	RemapEnabled           bool
	Shade                  float32
}
