package automap

const (
	defaultCLIWindowScale = 5

	sourcePortDefaultWindowW = 1280
	sourcePortDefaultWindowH = 800

	faithfulDefaultWindowW = 1280
	faithfulDefaultWindowH = 960
)

// DefaultCLIWindowSize returns the CLI/config default window size.
func DefaultCLIWindowSize() (int, int) {
	return doomLogicalW * defaultCLIWindowScale, doomLogicalH * defaultCLIWindowScale
}

func defaultRenderSizeForMode(sourcePort bool) (int, int) {
	if sourcePort {
		return sourcePortDefaultWindowW, sourcePortDefaultWindowH
	}
	return doomLogicalW, doomLogicalH
}

func ensurePositiveRenderSize(opts *Options) {
	if opts == nil {
		return
	}
	defW, defH := defaultRenderSizeForMode(opts.SourcePortMode)
	if opts.Width <= 0 {
		opts.Width = defW
	}
	if opts.Height <= 0 {
		opts.Height = defH
	}
}

// normalizeRunDimensions centralizes render/window sizing policy for runtime.
func normalizeRunDimensions(opts Options) (Options, int, int) {
	windowW := opts.Width
	windowH := opts.Height
	if opts.SourcePortMode {
		ensurePositiveRenderSize(&opts)
		return opts, opts.Width, opts.Height
	}

	// Faithful mode keeps a fixed Doom logical internal render size.
	opts.Width = doomLogicalW
	opts.Height = doomLogicalH

	if windowW <= 0 {
		windowW = faithfulDefaultWindowW
	}
	if windowH <= 0 {
		windowH = faithfulDefaultWindowH
	}

	scaleX := windowW / doomLogicalW
	scaleY := windowH / doomLogicalH
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	if scale < 1 {
		scale = 1
	}
	windowW = doomLogicalW * scale
	windowH = doomLogicalH * scale
	return opts, windowW, windowH
}
