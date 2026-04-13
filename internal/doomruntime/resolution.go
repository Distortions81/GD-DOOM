package doomruntime

import "gddoom/internal/runtimecfg"

const (
	defaultCLIWindowScale = 5

	sourcePortDefaultWindowW = 1280
	sourcePortDefaultWindowH = 800

	sourcePortMaxWindowW = 1280
	sourcePortMaxWindowH = 720

	faithfulDefaultWindowW = 1280
	faithfulDefaultWindowH = 960
	faithfulBufferW        = doomLogicalW * 2
	faithfulBufferH        = doomLogicalH * 2
	faithfulAspectLogicalH = 240
)

// DefaultCLIWindowSize returns the CLI/config default window size.
func DefaultCLIWindowSize() (int, int) {
	return runtimecfg.DefaultCLIWindowSize()
}

func ensurePositiveRenderSize(opts *Options) {
	if opts == nil {
		return
	}
	defW := doomLogicalW
	defH := doomLogicalH
	if opts.SourcePortMode {
		defW = sourcePortDefaultWindowW
		defH = sourcePortDefaultWindowH
	}
	if opts.Width <= 0 {
		opts.Width = defW
	}
	if opts.Height <= 0 {
		opts.Height = defH
	}
}

func normalizeRunDimensions(opts Options) (Options, int, int) {
	return runtimecfg.NormalizeRunDimensions(opts)
}

func clampSourcePortGameSizeForPlatform(w, h int, wasm bool) (int, int) {
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
