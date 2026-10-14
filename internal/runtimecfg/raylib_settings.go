package runtimecfg

// RaylibSettings is preserved by the main host when it writes common settings.
// Pointer fields distinguish an omitted preference from an explicit zero/false.
type RaylibSettings struct {
	Messages      *bool   `toml:"messages"`
	ScreenBlocks  *int    `toml:"screen_blocks"`
	HUDScale      *int    `toml:"hud_scale"`
	Width         *int    `toml:"width"`
	Height        *int    `toml:"height"`
	FPS           *int    `toml:"fps"`
	Fullscreen    *bool   `toml:"fullscreen"`
	Debug         *bool   `toml:"debug"`
	MSAA          *bool   `toml:"msaa"`
	TextureScale  *int    `toml:"texture_scale"`
	TextureFilter *string `toml:"texture_filter"`
	Lighting      *string `toml:"lighting"`
	MeshMode      *string `toml:"mesh_mode"`
}
