//go:build raylib && cgo && integration && !js

package doomruntime

import (
	"fmt"
	"image"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"gddoom/internal/render/raymesh"
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/hajimehoshi/ebiten/v2"
)

type nativeCRTSample struct{ width, height, tic int }

func nativeCRTSamples() []nativeCRTSample {
	var samples []nativeCRTSample
	for _, size := range [][2]int{{320, 200}, {640, 400}, {960, 540}, {511, 383}} {
		for _, tic := range []int{0, 1, 18, 35, 123, 123} {
			samples = append(samples, nativeCRTSample{size[0], size[1], tic})
		}
	}
	return samples
}

func nativeCRTSource(s nativeCRTSample) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, s.width, s.height))
	for y := range s.height {
		for x := range s.width {
			i := (y*s.width + x) * 4
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = byte(20+x*180/s.width), byte(40+y*140/s.height), byte(30+(x+y)*170/(s.width+s.height)), 255
		}
	}
	return img
}

// Each renderer owns GLFW; keep the actual Kage reference in a child process.
func TestNativeCRTEbitenReference(t *testing.T) {
	dir := os.Getenv("GD_RAYLIB_CRT_REFERENCE_DIR")
	if dir == "" {
		t.Skip("reference child for CRT comparison")
	}
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		shader, err := ebiten.NewShader(crtPostShaderSrc)
		if err != nil {
			t.Fatal(err)
		}
		defer shader.Deallocate()
		for i, s := range nativeCRTSamples() {
			src := newUnmanagedImage(s.width, s.height)
			src.WritePixels(nativeCRTSource(s).Pix)
			dst := newUnmanagedImage(s.width, s.height)
			op := &ebiten.DrawRectShaderOptions{Uniforms: map[string]any{"Time": float32(s.tic) / 35}}
			op.Images[0] = src
			dst.DrawRectShader(s.width, s.height, shader, op)
			pixels := make([]byte, s.width*s.height*4)
			dst.ReadPixels(pixels)
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.rgba", i)), pixels, 0600); err != nil {
				t.Fatal(err)
			}
			src.Deallocate()
			dst.Deallocate()
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}

func TestNativeCRTFramebuffersMatchMain(t *testing.T) {
	if os.Getenv("GD_RAYLIB_CRT_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_CRT_INTEGRATION=1")
	}
	dir := t.TempDir()
	child := exec.Command(os.Args[0], "-test.run=^TestNativeCRTEbitenReference$", "-test.v")
	child.Env = append(os.Environ(), "GD_RAYLIB_CRT_REFERENCE_DIR="+dir)
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("Kage reference: %v\n%s", err, out)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(320, 200, "CRT equivalence")
	defer rl.CloseWindow()
	crt := &raymesh.CRT{}
	defer crt.Close()
	for i, s := range nativeCRTSamples() {
		if int(rl.GetRenderWidth()) != s.width || int(rl.GetRenderHeight()) != s.height {
			rl.SetWindowSize(s.width, s.height)
			for range 2 {
				rl.BeginDrawing()
				rl.ClearBackground(rl.Black)
				rl.EndDrawing()
			}
		}
		img := rl.NewImageFromImage(nativeCRTSource(s))
		texture := rl.LoadTextureFromImage(img)
		rl.UnloadImage(img)
		rl.SetTextureFilter(texture, rl.FilterPoint)
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		rl.DrawTexture(texture, 0, 0, rl.White)
		if err := crt.Apply(s.tic); err != nil {
			t.Fatal(err)
		}
		frame, err := raymesh.CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		colors := rl.LoadImageColors(frame)
		want, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("%d.rgba", i)))
		if err != nil || len(want) != len(colors)*4 {
			t.Fatalf("invalid reference: %v", err)
		}
		boundaryPixels := 0
		for pixel, got := range colors {
			mismatch := false
			for channel, value := range []byte{got.R, got.G, got.B, got.A} {
				if delta := int(value) - int(want[pixel*4+channel]); delta < -1 || delta > 1 {
					mismatch = true
					break
				}
			}
			if mismatch {
				if !nativeCRTMaskBoundaryEquivalent(s, pixel, got, want[pixel*4:pixel*4+4]) {
					t.Fatalf("%dx%d tic=%d pixel=(%d,%d) got=%v main=%v", s.width, s.height, s.tic, pixel%s.width, pixel/s.width, got, want[pixel*4:pixel*4+4])
				}
				boundaryPixels++
			}
		}
		if boundaryPixels > len(colors)/10000 {
			t.Fatalf("too many mask boundary differences: %d/%d", boundaryPixels, len(colors))
		}
		t.Logf("%dx%d tic=%d: %d mask-boundary pixels; all others match within one channel level", s.width, s.height, s.tic, boundaryPixels)
		rl.UnloadImageColors(colors)
		rl.UnloadImage(frame)
		rl.EndDrawing()
		rl.UnloadTexture(texture)
	}
	t.Logf("%d complete CRT framebuffer comparisons passed", len(nativeCRTSamples()))
}

// Source-coordinate interpolation and floating-point contraction can select
// opposite sides of floor() at an RGB-mask boundary in the two GPU backends.
// Permit only that specific case: warped X within 0.001 pixels of an integer,
// alpha unchanged, and RGB equal after changing the mask phase. The caller also
// limits these differences to 0.01% of the complete framebuffer.
func nativeCRTMaskBoundaryEquivalent(s nativeCRTSample, pixel int, got rl.Color, want []byte) bool {
	x, y := float64(pixel%s.width)+.5, float64(pixel/s.width)+.5
	px, py := x/float64(s.width)*2-1, y/float64(s.height)*2-1
	warpedX := (px*(1+.04*(px*px+py*py)) + 1) * .5 * float64(s.width)
	if math.Abs(warpedX-math.Round(warpedX)) > .001 || got.A != want[3] {
		return false
	}
	mask := [3][3]float64{{1, .88, .88}, {.88, 1, .88}, {.88, .88, 1}}
	values := [3]byte{got.R, got.G, got.B}
	for from := range 3 {
		for to := range 3 {
			if from == to {
				continue
			}
			matches := true
			for channel := range 3 {
				adjusted := float64(values[channel]) * mask[to][channel] / mask[from][channel]
				matches = matches && math.Abs(adjusted-float64(want[channel])) <= 2
			}
			if matches {
				return true
			}
		}
	}
	return false
}
