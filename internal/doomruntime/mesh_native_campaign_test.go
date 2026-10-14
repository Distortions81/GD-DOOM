package doomruntime

import (
	"gddoom/internal/mapdata"
	"gddoom/internal/sessionflow"
	"gddoom/internal/wad"
	"os"
	"reflect"
	"testing"
)

func nativeCampaignFixture(t *testing.T, name mapdata.MapName) *NativeCampaign {
	t.Helper()
	fixture := loadMeshExperimentMap(t, name)
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, name)
	if err != nil {
		t.Fatal(err)
	}
	opts := fixture.opts
	return NewNativeCampaign(NewNativeMeshGame(m, opts), opts, func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error) {
		name, err := mapdata.NextMapName(wf, current, secret)
		if err != nil {
			return nil, "", err
		}
		m, err := mapdata.LoadMap(wf, name)
		return m, name, err
	})
}
func finishNativeIntermission(t *testing.T, c *NativeCampaign) {
	t.Helper()
	for i := 0; i < 1500 && c.Phase() != NativeCampaignPlaying; i++ {
		if err := c.Tick(NativeMeshInput{}, i%15 == 0); err != nil {
			t.Fatal(err)
		}
	}
	if c.Phase() != NativeCampaignPlaying {
		t.Fatal("campaign did not finish intermission")
	}
}

func TestNativeCampaignIntermissionMatchesMainAndCarriesInventory(t *testing.T) {
	c := nativeCampaignFixture(t, "E1M1")
	g := c.Game.g
	g.stats.Health, g.stats.Armor, g.stats.Shells = 73, 40, 19
	g.inventory.Weapons[2001] = true
	g.inventory.ReadyWeapon = weaponShotgun
	g.inventory.BlueKey = true
	g.inventory.InvulnTics = 175
	g.playerKillCount, g.playerItemCount, g.secretsFound, g.worldTic = 5, 3, 1, 700
	g.requestLevelExit(false, "exit")
	if err := c.Tick(NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	if c.Phase() != NativeCampaignIntermission || c.TakeMusicRequest() != "D_INTER" {
		t.Fatal("exit did not enter intermission with Doom music")
	}
	expected := c.session.intermission.state
	for tic := 0; tic < 80; tic++ {
		skip := tic == 20
		next, _, done := tickVanillaIntermission(expected, skip)
		if done {
			t.Fatal("reference finished too soon")
		}
		expected = next
		if err := c.Tick(NativeMeshInput{}, skip); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(c.session.intermission.state, expected) {
			t.Fatalf("native statistics animation diverged on tic %d", tic)
		}
		if c.Game.g.worldTic != 700 {
			t.Fatal("combat simulation advanced during intermission")
		}
	}
	finishNativeIntermission(t, c)
	if c.Map().Name != "E1M2" || c.TakeMusicRequest() != "D_E1M2" {
		t.Fatal("normal exit did not load E1M2 and its track")
	}
	n := c.Game.g
	if n.stats.Health != 73 || n.stats.Armor != 40 || n.stats.Shells != 19 || !n.inventory.Weapons[2001] || n.inventory.ReadyWeapon != weaponShotgun {
		t.Fatal("level transition lost player inventory")
	}
	if n.inventory.BlueKey || n.inventory.InvulnTics != 0 || n.levelExitRequested || n.worldTic != 0 {
		t.Fatal("new level retained keys, powerups or old level state")
	}
}

func TestNativeCampaignSecretRouteAndEpisodeFinale(t *testing.T) {
	c := nativeCampaignFixture(t, "E1M3")
	c.Game.g.requestLevelExit(true, "secret exit")
	if err := c.Tick(NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	if c.session.intermission.state.Target.NextMapName != "E1M9" || !c.session.intermission.state.DidSecret {
		t.Fatal("secret exit lost E1M9 route")
	}
	finishNativeIntermission(t, c)
	c.Game.g.requestLevelExit(false, "return")
	if err := c.Tick(NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	finishNativeIntermission(t, c)
	if c.Map().Name != "E1M4" || !c.session.secretVisited {
		t.Fatal("secret map did not rejoin campaign with secret visit remembered")
	}
	finale := nativeCampaignFixture(t, "E1M8")
	finale.Game.g.requestLevelExit(false, "episode complete")
	if err := finale.Tick(NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	if finale.Phase() != NativeCampaignFinale || finale.TakeMusicRequest() != "D_VICTOR" || finale.session.finale.Screen != "CREDIT" {
		t.Fatal("episode exit did not start shared ending")
	}
	for i := 0; i < 1000 && finale.Phase() != NativeCampaignComplete; i++ {
		if err := finale.Tick(NativeMeshInput{}, i%15 == 0); err != nil {
			t.Fatal(err)
		}
	}
	if finale.Phase() != NativeCampaignComplete {
		t.Fatal("episode ending never returned to frontend")
	}
}

func TestNativeCampaignPatchCollectionDoesNotInitializeEbitenImages(t *testing.T) {
	c := nativeCampaignFixture(t, "E1M1")
	tex := WallTexture{RGBA: []byte{255, 0, 0, 255}, Width: 1, Height: 1, OffsetX: 2, OffsetY: 3}
	c.session.opts.IntermissionPatchBank = map[string]WallTexture{"WIMAP0": tex, "WILV00": tex, "WIF": tex, "WINUM0": tex, "WIOSTK": tex, "WIOSTI": tex, "WISCRT2": tex, "WITIME": tex, "WIPAR": tex}
	c.Game.g.requestLevelExit(false, "exit")
	if err := c.Tick(NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	checksum := c.Game.g.SimChecksum()
	patches := c.Patches()
	if len(patches) < 5 || patches[0].X != -2 || patches[0].Y != -3 {
		t.Fatal("intermission collection lost artwork or WAD offsets")
	}
	if c.session.intermissionImages != nil || c.Game.g.messageFontImg != nil {
		t.Fatal("native intermission created Ebiten image cache")
	}
	if c.Game.g.SimChecksum() != checksum {
		t.Fatal("drawing intermission changed simulation")
	}
}

func TestNativeCampaignDoomIIStoryAndSecretReturn(t *testing.T) {
	wf, err := wad.Open("../../wads/DOOM2.WAD")
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("Doom II WAD unavailable")
		}
		t.Fatal(err)
	}
	opts := loadMeshExperimentGame(t).opts
	newCampaign := func(name mapdata.MapName) *NativeCampaign {
		m, err := mapdata.LoadMap(wf, name)
		if err != nil {
			t.Fatal(err)
		}
		return NewNativeCampaign(NewNativeMeshGame(m, opts), opts, func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error) {
			name, err := mapdata.NextMapName(wf, current, secret)
			if err != nil {
				return nil, "", err
			}
			m, err := mapdata.LoadMap(wf, name)
			return m, name, err
		})
	}
	for _, tc := range []struct {
		from, to mapdata.MapName
		secret   bool
	}{{"MAP06", "MAP07", false}, {"MAP11", "MAP12", false}, {"MAP20", "MAP21", false}, {"MAP15", "MAP31", true}, {"MAP31", "MAP32", true}} {
		t.Run(string(tc.from), func(t *testing.T) {
			c := newCampaign(tc.from)
			c.Game.g.requestLevelExit(tc.secret, "exit")
			if err := c.Tick(NativeMeshInput{}, false); err != nil {
				t.Fatal(err)
			}
			if c.TakeMusicRequest() != "D_DM2INT" {
				t.Fatal("commercial intermission used wrong music")
			}
			for i := 0; i < 1500 && c.Phase() == NativeCampaignIntermission; i++ {
				if err := c.Tick(NativeMeshInput{}, i%15 == 0); err != nil {
					t.Fatal(err)
				}
			}
			if c.Phase() != NativeCampaignFinale || !c.session.finale.Commercial || c.TakeMusicRequest() != "D_READ_M" {
				t.Fatal("commercial story screen missing")
			}
			// Commercial text ignores buttons until its vanilla 50-tic input delay.
			for range 50 {
				if err := c.Tick(NativeMeshInput{}, true); err != nil {
					t.Fatal(err)
				}
			}
			if c.Phase() != NativeCampaignFinale {
				t.Fatal("commercial story skipped before input delay")
			}
			for i := 0; i < 10 && c.Phase() != NativeCampaignPlaying; i++ {
				if err := c.Tick(NativeMeshInput{}, true); err != nil {
					t.Fatal(err)
				}
			}
			if c.Map().Name != tc.to || c.Phase() != NativeCampaignPlaying {
				t.Fatalf("story transition map=%s want=%s", c.Map().Name, tc.to)
			}
		})
	}
	c := newCampaign("MAP32")
	c.Game.g.requestLevelExit(false, "return")
	if err := c.Tick(NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	finishNativeIntermission(t, c)
	if c.Map().Name != "MAP16" {
		t.Fatal("super-secret level did not return to MAP16")
	}
	terminal := newCampaign("MAP30")
	terminal.Game.g.requestLevelExit(false, "end")
	if err := terminal.Tick(NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1500 && terminal.Phase() == NativeCampaignIntermission; i++ {
		if err := terminal.Tick(NativeMeshInput{}, i%15 == 0); err != nil {
			t.Fatal(err)
		}
	}
	for range 60 {
		if err := terminal.Tick(NativeMeshInput{}, true); err != nil {
			t.Fatal(err)
		}
	}
	if terminal.session.finale.Stage != sessionflow.FinaleStageCast {
		t.Fatal("Doom II ending did not reach shared cast state")
	}
}

func TestNativeCampaignRestartKeepsSecretHistoryAndResetsPlayer(t *testing.T) {
	c := nativeCampaignFixture(t, "E1M4")
	c.session.secretVisited = true
	c.Game.g.stats.Health = 0
	c.Game.g.isDead = true
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := mapdata.LoadMap(wf, "E1M4")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Restart(fresh); err != nil {
		t.Fatal(err)
	}
	if c.Game.IsDead() || c.Game.g.stats.Health != 100 || c.Game.g.stats.Bullets != 50 || !c.session.secretVisited {
		t.Fatal("restart lost campaign history or retained dead player")
	}
	if c.Phase() != NativeCampaignPlaying || c.TakeMusicRequest() != "D_E1M4" {
		t.Fatal("restart did not restore gameplay/music")
	}
	if err := c.Restart(nil); err == nil {
		t.Fatal("restart accepted missing map")
	}
}
