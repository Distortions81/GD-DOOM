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

	wasmMaxGameW = 1280
	wasmMaxGameH = 720
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
	if w <= wasmMaxGameW && h <= wasmMaxGameH {
		return w, h
	}
	if w <= 0 || h <= 0 {
		return max(1, min(w, wasmMaxGameW)), max(1, min(h, wasmMaxGameH))
	}
	scaleW := float64(wasmMaxGameW) / float64(w)
	scaleH := float64(wasmMaxGameH) / float64(h)
	scale := min(scaleW, scaleH)
	if scale >= 1 {
		return w, h
	}
	w = max(1, int(float64(w)*scale))
	h = max(1, int(float64(h)*scale))
	return w, h
}
