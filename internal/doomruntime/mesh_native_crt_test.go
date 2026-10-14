package doomruntime

import "testing"

func TestNativeCRTToggleAndSavedState(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		c := nativeSaveFixture(t)
		checksum := c.Game.g.SimChecksum()
		c.SetCRTEnabled(enabled)
		c.Game.ToggleCRT()
		want := "CRT ON"
		if enabled {
			want = "CRT OFF"
		}
		if c.Game.CRTEnabled() == enabled || c.Game.Frame(1).Message != want {
			t.Fatal("CRT toggle diverged from main")
		}
		c.SetCRTEnabled(enabled)
		if c.Game.g.SimChecksum() != checksum {
			t.Fatal("CRT changed simulation")
		}
		data, err := c.SaveData("CRT setting")
		if err != nil {
			t.Fatal(err)
		}
		c.SetCRTEnabled(!enabled)
		if err := c.LoadData(data); err != nil {
			t.Fatal(err)
		}
		if c.Game.CRTEnabled() != enabled {
			t.Fatal("save load lost CRT")
		}
		c.SetCRTEnabled(c.Game.CRTEnabled())
		fresh, err := c.session.opts.NewGameLoader("E1M1")
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Restart(fresh); err != nil {
			t.Fatal(err)
		}
		if c.Game.CRTEnabled() != enabled {
			t.Fatal("restart lost CRT")
		}
		c.Game.g.requestLevelExit(false, "CRT test")
		if err := c.Tick(NativeMeshInput{}, false); err != nil {
			t.Fatal(err)
		}
		finishNativeIntermission(t, c)
		if c.Game.CRTEnabled() != enabled {
			t.Fatal("next map lost CRT")
		}
	}
}
