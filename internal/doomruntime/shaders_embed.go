package doomruntime

import _ "embed"

var (
	//go:embed shaders/crt_post.kage
	crtPostShaderSrc []byte

	//go:embed shaders/sky_backdrop.kage
	skyBackdropShaderSrc []byte
)
