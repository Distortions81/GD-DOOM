package doomruntime

import "gddoom/internal/runtimecfg"

const (
	defaultCLIWindowScale = 5

	sourcePortDefaultWindowW = 1280
	sourcePortDefaultWindowH = 800

	faithfulDefaultWindowW = 1280
	faithfulDefaultWindowH = 960
	faithfulBufferW        = doomLogicalW * 2
	faithfulBufferH        = doomLogicalH * 2
	faithfulAspectLogicalH = 240

	wasmMaxLayoutW = 1280
	wasmMaxLayoutH = 720
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

func clampSourcePortLayoutSizeForPlatform(w, h int, wasm bool) (int, int) {
	if !wasm {
		return w, h
	}
	if w > wasmMaxLayoutW {
		w = wasmMaxLayoutW
	}
	if h > wasmMaxLayoutH {
		h = wasmMaxLayoutH
	}
	return w, h
}
