package doomruntime

import (
	"testing"

	"gddoom/internal/media"
	"gddoom/internal/sessionmusic"
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

func TestPlayMusicForMapDefersUntilLevelTransitionCompletes(t *testing.T) {
	sg := &sessionGame{
		musicCtl: &sessionmusic.Playback{},
	}
	sg.queueTransition(transitionLevel, 0)
	sg.playMusicForMap("MAP01")
	if got := sg.transitionMusicPending.kind; got != musicPlaybackSourceMap {
		t.Fatalf("transitionMusicPending.kind=%d want map", got)
	}
	if got := sg.currentMusicSource.kind; got != musicPlaybackSourceNone {
		t.Fatalf("currentMusicSource.kind=%d want none while level transition is active", got)
	}
	sg.transition.Clear()
	sg.releaseTransitionMusicIfReady()
	if got := sg.transitionMusicPending.kind; got != musicPlaybackSourceNone {
		t.Fatalf("transitionMusicPending.kind=%d want none after transition", got)
	}
	if got := sg.currentMusicSource.kind; got != musicPlaybackSourceMap {
		t.Fatalf("currentMusicSource.kind=%d want map after transition", got)
	}
}
