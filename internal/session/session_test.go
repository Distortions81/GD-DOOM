package session

import (
	"errors"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

type cadenceRuntime struct {
	samples, updates int
}

func (r *cadenceRuntime) SampleInput()               { r.samples++ }
func (r *cadenceRuntime) Update() error              { r.updates++; return nil }
func (r *cadenceRuntime) Draw(*ebiten.Image)         {}
func (r *cadenceRuntime) Layout(w, h int) (int, int) { return w, h }

type pumpedCadenceRuntime struct {
	cadenceRuntime
	pumps int
	err   error
}

func (r *pumpedCadenceRuntime) UpdateHostFrame() error {
	r.pumps++
	if r.samples != r.pumps+r.updates {
		return errors.New("network pump ran without sampling its host frame")
	}
	return r.err
}

func TestHostFramePumpPreservesFixedSessionCadence(t *testing.T) {
	plain, network := &cadenceRuntime{}, &pumpedCadenceRuntime{}
	for _, runtime := range []Runtime{plain, network} {
		g := New(runtime)
		for range 140 {
			if err := g.Update(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if plain.samples != 140 || plain.updates != 35 || network.samples != 140 || network.updates != 35 || network.pumps != 105 {
		t.Fatalf("singleplayer=%+v multiplayer=%+v; want 140 samples,35 full updates,105 network-only pumps", plain, network)
	}
}

func TestHostFramePumpPropagatesFailure(t *testing.T) {
	runtime := &pumpedCadenceRuntime{err: errors.New("pump failed")}
	g := New(runtime)
	if err := g.Update(); err != nil {
		t.Fatal(err)
	}
	if err := g.Update(); !errors.Is(err, runtime.err) {
		t.Fatalf("pump error = %v", err)
	}
}
