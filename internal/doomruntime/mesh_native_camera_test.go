package doomruntime

import (
	"math"
	"testing"
)

func TestNativeCameraSmoothingAffectsRenderingAndSurvivesRestart(t *testing.T) {
	c := nativeSaveFixture(t)
	c.SetCameraSmoothing(true)
	g := c.Game.g
	// Accelerating the turn distinguishes the shared smooth curve from linear yaw.
	g.prevPrevAngle, g.prevAngle, g.p.angle = 0, 0, 1<<29
	checksum := g.SimChecksum()
	smooth := c.Game.Frame(.5).Camera.Yaw
	if smooth != angleToRadians(interpolateCameraAngle(0, 0, 1<<29, .5)) {
		t.Fatal("native camera did not use the main smooth curve")
	}
	c.SetCameraSmoothing(false)
	linear := c.Game.Frame(.5).Camera.Yaw
	if math.Abs(linear-math.Pi/8) > 1e-8 || smooth == linear {
		t.Fatalf("smoothing toggle did not change displayed yaw: smooth=%v linear=%v", smooth, linear)
	}
	if g.SimChecksum() != checksum {
		t.Fatal("camera preference changed simulation state")
	}
	fresh, err := c.session.opts.NewGameLoader("E1M1")
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{true, false} {
		c.SetCameraSmoothing(enabled)
		if err := c.Restart(fresh); err != nil {
			t.Fatal(err)
		}
		if c.session.opts.SmoothCameraYaw != enabled || c.Game.g.opts.SmoothCameraYaw != enabled {
			t.Fatal("restart lost camera smoothing preference")
		}
	}
}
