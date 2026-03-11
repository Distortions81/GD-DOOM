package doomruntime

import _ "embed"

var (
	//go:embed shaders/faithful_palette.kage
	faithfulPaletteShaderSrc []byte

	//go:embed shaders/faithful_palette_nogamma.kage
	faithfulPaletteNoGammaShaderSrc []byte

	//go:embed shaders/crt_post.kage
	crtPostShaderSrc []byte

	//go:embed shaders/sky_backdrop.kage
	skyBackdropShaderSrc []byte
)
