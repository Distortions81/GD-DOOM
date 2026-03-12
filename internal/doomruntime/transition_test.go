package doomruntime

import (
	"testing"

	"gddoom/internal/media"
)

func TestTransitionSurfaceSizeFaithfulUsesLogicalRenderSize(t *testing.T) {
	sg := &sessionGame{
		opts: Options{
			SourcePortMode: false,
			Width:          doomLogicalW,
			Height:         doomLogicalH,
		},
	}
	w, h := sg.transitionSurfaceSize(1280, 800)
	if w != doomLogicalW || h != doomLogicalH {
		t.Fatalf("faithful transition size=%dx%d want %dx%d", w, h, doomLogicalW, doomLogicalH)
	}
}

func TestTransitionSurfaceSizeFaithfulFallback(t *testing.T) {
	sg := &sessionGame{
		opts: Options{
			SourcePortMode: false,
			Width:          0,
			Height:         0,
		},
	}
	w, h := sg.transitionSurfaceSize(1920, 1200)
	if w != doomLogicalW || h != doomLogicalH {
		t.Fatalf("faithful fallback transition size=%dx%d want %dx%d", w, h, doomLogicalW, doomLogicalH)
	}
}

func TestTransitionSurfaceSizeSourcePortUsesScreenSize(t *testing.T) {
	sg := &sessionGame{
		opts: Options{
			SourcePortMode: true,
			Width:          320,
			Height:         200,
		},
	}
	w, h := sg.transitionSurfaceSize(1366, 768)
	if w != 1366 || h != 768 {
		t.Fatalf("sourceport transition size=%dx%d want 1366x768", w, h)
	}
}

func TestShouldShowBootSplashAllowsFrontendMenuStartup(t *testing.T) {
	sg := &sessionGame{
		opts: Options{
			OpenMenuOnFrontendStart: true,
			BootSplash: media.WallTexture{
				Width:  1,
				Height: 1,
				RGBA:   []byte{0, 0, 0, 255},
			},
		},
	}
	if !sg.shouldShowBootSplash() {
		t.Fatal("shouldShowBootSplash() = false, want true for frontend menu startup")
	}
}
