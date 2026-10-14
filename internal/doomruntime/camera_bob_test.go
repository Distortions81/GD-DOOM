package doomruntime

import (
	"math"
	"testing"
	"time"

	"gddoom/internal/doomrand"
)

func bobTestGame(tic int, momentum int64) *game {
	g := &game{opts: Options{SourcePortMode: true}, worldTic: tic, p: player{
		viewHeight: playerViewHeight, ceilz: 128 * fracUnit, momx: momentum,
	}}
	g.tickPlayerViewHeight()
	g.capturePrevState()
	g.worldTic++
	g.tickPlayerViewHeight()
	return g
}

func TestCameraBobContinuousPhaseAndFrameRates(t *testing.T) {
	// Constant momentum produces MAXBOB/2 = 8 units. Include the sine
	// maximum (where lerping samples flattens it), phase wrap, and large tics.
	for _, tic := range []int{4, 19, 20, 8191, 1000000000} {
		g := bobTestGame(tic, 8*fracUnit)
		stamp := time.Unix(100, 0)
		g.markSimUpdate(stamp)
		body, eye, world := g.p, g.playerViewZ, g.worldTic
		rnd, prnd := doomrand.State()
		for _, fps := range []int{30, 60, 144, 240} {
			for frame := 0; frame <= fps; frame++ {
				// Sample the fraction of a simulation tic reached at each display time.
				fraction := math.Mod(float64(frame)*35/float64(fps), 1)
				elapsed := time.Duration(fraction * float64(time.Second) / 35)
				g.prepareRenderStateAt(stamp.Add(elapsed))
				a := elapsed.Seconds() * 35
				phase := (float64((tic*409)&8191) + 409*a + 0.5) * 2 * math.Pi / 8192
				want := 41 + 8*math.Sin(phase)
				if got := g.playerEyeZ(); math.Abs(got-want) > 1e-12 {
					t.Fatalf("tic=%d fps=%d frame=%d eye=%.15f want=%.15f", tic, fps, frame, got, want)
				}
			}
		}
		if g.p != body || g.playerViewZ != eye || g.worldTic != world {
			t.Fatal("rendering mutated simulation")
		}
		if r, p := doomrand.State(); r != rnd || p != prnd {
			t.Fatal("rendering consumed RNG")
		}
	}
	// The continuous sine must curve above the chord near its maximum.
	g := bobTestGame(4, 8*fracUnit)
	g.renderAlpha = 0
	start := g.playerEyeZ()
	g.renderAlpha = 1
	end := g.playerEyeZ()
	g.renderAlpha = .5
	if g.playerEyeZ() <= (start+end)/2+.05 {
		t.Fatal("bob is linearly interpolating sampled heights")
	}
}

func TestCameraBobInterpolatesAmplitudeAndBaseBeforeClipping(t *testing.T) {
	g := bobTestGame(3, 4*fracUnit)
	g.capturePrevState()
	g.worldTic++
	g.p.z, g.p.floorz = 10*fracUnit, 10*fracUnit
	g.p.momx = 8 * fracUnit
	g.tickPlayerViewHeight()
	g.renderAlpha = .5
	// Half amplitudes of momx^2/4: 2 -> 8; base 41 -> 51.
	phase := (409*4 + 409*.5 + .5) * 2 * math.Pi / 8192
	want := 46 + 5*math.Sin(phase)
	if got := g.playerEyeZ(); math.Abs(got-want) > 1e-12 {
		t.Fatalf("eye=%v want=%v", got, want)
	}
	// Momentum has friction applied after P_CalcHeight. Rendering must retain
	// the amplitude sampled before that thinker update.
	g.p.momx = 0
	if got := g.playerEyeZ(); math.Abs(got-want) > 1e-12 {
		t.Fatalf("post-thinker momentum changed bob: %v", got)
	}
	g.cameraBob.ceiling, g.prevCameraBob.ceiling = 47, 45
	if got := g.playerEyeZ(); got != 46 {
		t.Fatalf("ceiling clipping=%v want=46", got)
	}
}

func TestCameraBobCutsFaithfulAndAuthoritativeViews(t *testing.T) {
	for _, cut := range []string{"teleport", "sync", "external-eye", "faithful", "prediction"} {
		t.Run(cut, func(t *testing.T) {
			g := bobTestGame(4, 8*fracUnit)
			switch cut {
			case "teleport":
				g.p.teleportedThisTic = true
				g.playerViewZ = 200 * fracUnit
			case "sync":
				g.playerViewZ = 200 * fracUnit
				g.syncRenderState()
			case "external-eye":
				g.playerViewZ = 0 // Zero is a legitimate camera height.
			case "faithful":
				g.opts.SourcePortMode = false
			case "prediction":
				g.clientPrediction = &ClientPrediction{g: g}
			}
			for _, a := range []float64{0, .25, .5, .75, 1} {
				g.renderAlpha = a
				if got := g.playerEyeZ(); got != g.playerBaseEyeZ() {
					t.Fatalf("alpha=%v eye=%v want canonical %v", a, got, g.playerBaseEyeZ())
				}
			}
		})
	}
}

func TestCameraBobAirborneDeathAndIdle(t *testing.T) {
	g := bobTestGame(0, 0)
	g.renderAlpha = .5
	if g.playerEyeZ() != 41 {
		t.Fatal("stationary camera bobs")
	}
	// Airborne eyes remain above a low ceiling, matching P_CalcHeight's
	// early return, with no bob even while horizontal momentum is nonzero.
	g.p.z, g.p.ceilz, g.p.momx = 64*fracUnit, 70*fracUnit, 8*fracUnit
	g.tickPlayerViewHeight()
	g.capturePrevState()
	g.worldTic++
	g.tickPlayerViewHeight()
	if got := g.playerEyeZ(); got != 105 {
		t.Fatalf("airborne eye=%v want=105", got)
	}
	// A moving corpse keeps the same bob formula while its eye height falls.
	g.p.z, g.p.floorz, g.p.ceilz = 0, 0, 128*fracUnit
	g.isDead = true
	g.tickPlayerViewHeight()
	g.capturePrevState()
	g.worldTic++
	g.tickPlayerViewHeight()
	phase := (float64(g.prevCameraBob.tic*409) + 409*.5 + .5) * 2 * math.Pi / 8192
	if got, want := g.playerEyeZ(), 39.5+8*math.Sin(phase); math.Abs(got-want) > 1e-12 {
		t.Fatalf("death eye=%v want=%v", got, want)
	}
}

func TestCameraBobContinuousFormulaMatchesClassicSamplesWithinQuantization(t *testing.T) {
	for tic := 0; tic < 100; tic++ {
		g := bobTestGame(tic, 8*fracUnit)
		g.renderAlpha = 1
		// Classic truncates the sine table and the fixed-point multiply. Modern
		// retains the continuous value; only that sub-texel quantization differs.
		if delta := math.Abs(g.playerEyeZ() - g.playerBaseEyeZ()); delta > 9.0/fracUnit {
			t.Fatalf("tic=%d analytic/classic delta=%v exceeds fixed-point quantization", tic+1, delta)
		}
	}
}

func TestCameraBobNativeRendererUsesContinuousEyeWithoutChangingSimulation(t *testing.T) {
	c := nativeSaveFixture(t)
	g := c.Game.g
	g.worldTic = 4
	g.p.momx, g.p.momy = 8*fracUnit, 0
	g.tickPlayerViewHeight()
	g.capturePrevState()
	g.worldTic++
	g.tickPlayerViewHeight()
	checksum := g.SimChecksum()
	for _, alpha := range []float64{0, .25, .5, .75, 1} {
		want := g.cameraBob.base + 8*math.Sin((409*(4+alpha)+.5)*2*math.Pi/8192)
		if got := c.Game.Frame(alpha).Camera.Z; math.Abs(got-want) > 1e-12 {
			t.Fatalf("native alpha=%v eye=%v want=%v", alpha, got, want)
		}
	}
	if g.SimChecksum() != checksum {
		t.Fatal("native rendering changed simulation checksum")
	}
}
