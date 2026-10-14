package doomruntime

import (
	"bytes"
	"testing"
)

func TestNativeGammaUsesSharedTablesAndKeepsSimulation(t *testing.T) {
	c := nativeSaveFixture(t)
	n := c.Game
	checksum := n.g.SimChecksum()
	for level := range doomGammaLevels {
		c.SetGammaLevel(level)
		if n.GammaTable() != doomGammaTables[level] || activeGammaLevel != level || n.GammaLevel() != level {
			t.Fatal("native gamma diverged from main table")
		}
		before := n.SpectreFuzzColors()
		n.CycleGammaLevel()
		want := (level + 1) % doomGammaLevels
		if n.GammaLevel() != want || n.Frame(1).Message != gammaMessage(want) {
			t.Fatal("F11 did not cycle gamma with shared HUD message")
		}
		c.SetGammaLevel(level)
		after := n.SpectreFuzzColors()
		if before.RemapEnabled {
			if &before.Palette.RGBA[0] != &after.Palette.RGBA[0] || &before.Lookup.RGBA[0] != &after.Lookup.RGBA[0] {
				t.Fatal("returning to gamma rebuilt cached fuzz colors")
			}
			if !bytes.Equal(before.Palette.RGBA, after.Palette.RGBA) {
				t.Fatal("cached palette changed")
			}
		}
	}
	if n.g.SimChecksum() != checksum {
		t.Fatal("gamma changed simulation state")
	}
}

func TestNativeGammaSurvivesSaveRestartAndMapChange(t *testing.T) {
	for _, level := range []int{0, 4} {
		c := nativeSaveFixture(t)
		c.SetGammaLevel(level)
		data, err := c.SaveData("gamma")
		if err != nil {
			t.Fatal(err)
		}
		c.SetGammaLevel(2)
		if err := c.LoadData(data); err != nil {
			t.Fatal(err)
		}
		if c.Game.GammaLevel() != level || activeGammaLevel != level {
			t.Fatal("load did not activate saved gamma bank")
		}
		fresh, err := c.session.opts.NewGameLoader("E1M1")
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Restart(fresh); err != nil {
			t.Fatal(err)
		}
		if c.Game.GammaLevel() != level {
			t.Fatal("restart lost gamma")
		}
		c.Game.g.requestLevelExit(false, "gamma test")
		if err := c.Tick(NativeMeshInput{}, false); err != nil {
			t.Fatal(err)
		}
		finishNativeIntermission(t, c)
		if c.Map().Name != "E1M2" || c.Game.GammaLevel() != level || activeGammaLevel != level {
			t.Fatal("map change lost gamma")
		}
	}
}
