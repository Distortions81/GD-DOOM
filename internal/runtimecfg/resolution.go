package runtimecfg

const (
	doomLogicalW = 320
	doomLogicalH = 200

	defaultCLIWindowScale = 5

	sourcePortDefaultWindowW = 1280
	sourcePortDefaultWindowH = 800

	faithfulDefaultWindowW = 1280
	faithfulDefaultWindowH = 960
	faithfulAspectLogicalH = 240

	sourcePortMaxWindowW = 1280
	sourcePortMaxWindowH = 720
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

func clampSourcePortWindowSizeForPlatform(w, h int, wasm bool) (int, int) {
	if !wasm {
		return w, h
	}
	if w <= sourcePortMaxWindowW && h <= sourcePortMaxWindowH {
		return w, h
	}
	if w <= 0 {
		w = sourcePortMaxWindowW
	}
	if h <= 0 {
		h = sourcePortMaxWindowH
	}
	// Keep aspect ratio while fitting into the 1280x720 ceiling.
	if w*sourcePortMaxWindowH >= h*sourcePortMaxWindowW {
		clampedH := (h * sourcePortMaxWindowW) / w
		if clampedH < 1 {
			clampedH = 1
		}
		return sourcePortMaxWindowW, clampedH
	}
	clampedW := (w * sourcePortMaxWindowH) / h
	if clampedW < 1 {
		clampedW = 1
	}
	return clampedW, sourcePortMaxWindowH
}

// NormalizeRunDimensions centralizes render/window sizing policy for runtime.
func NormalizeRunDimensions(opts Options) (Options, int, int) {
	windowW := opts.Width
	windowH := opts.Height
	aspectH := faithfulAspectLogicalH
	if opts.DisableAspectCorrection {
		aspectH = doomLogicalH
	}
	if opts.SourcePortMode {
		ensurePositiveRenderSize(&opts)
		opts.Width, opts.Height = clampSourcePortWindowSizeForPlatform(opts.Width, opts.Height, isWASMBuild())
		return opts, opts.Width, opts.Height
	}

	opts.Width = doomLogicalW
	opts.Height = doomLogicalH

	if windowW <= 0 {
		windowW = faithfulDefaultWindowW
	}
	if windowH <= 0 {
		windowH = faithfulDefaultWindowH
	}

	if windowW*aspectH <= windowH*doomLogicalW {
		windowH = (windowW*aspectH + doomLogicalW - 1) / doomLogicalW
	} else {
		windowW = (windowH * doomLogicalW) / aspectH
	}
	if windowW < doomLogicalW {
		windowW = doomLogicalW
	}
	if windowH < aspectH {
		windowH = aspectH
	}
	return opts, windowW, windowH
}
