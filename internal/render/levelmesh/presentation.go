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
}
