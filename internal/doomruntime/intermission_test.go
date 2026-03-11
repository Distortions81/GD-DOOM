package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/sessionflow"
)

func TestEpisodeMapSlot(t *testing.T) {
	ep, slot, ok := episodeMapSlot("E2M7")
	if !ok || ep != 2 || slot != 7 {
		t.Fatalf("episodeMapSlot(E2M7)=(%d,%d,%t) want=(2,7,true)", ep, slot, ok)
	}
	if _, _, ok := episodeMapSlot("MAP01"); ok {
		t.Fatal("MAP01 should not be treated as episode map")
	}
}

func TestShouldShowYouAreHere(t *testing.T) {
	if !shouldShowYouAreHere("E1M1", "E1M2") {
		t.Fatal("same episode should show YOU ARE HERE")
	}
	if shouldShowYouAreHere("E1M9", "E2M1") {
		t.Fatal("different episodes should not show YOU ARE HERE")
	}
	if shouldShowYouAreHere("MAP01", "MAP02") {
		t.Fatal("commercial maps should not show YOU ARE HERE")
	}
}

func TestShouldShowEnteringScreen(t *testing.T) {
	if !shouldShowEnteringScreen("E1M1", "E1M2") {
		t.Fatal("episode progression should show ENTERING screen")
	}
	if shouldShowEnteringScreen("MAP01", "MAP02") {
		t.Fatal("commercial maps should not use episode ENTERING screen")
	}
	if shouldShowEnteringScreen("E1M1", "MAP02") {
		t.Fatal("mixed map formats should not use episode ENTERING screen")
	}
}

func TestEpisodeFinaleScreen(t *testing.T) {
	if got, ok := episodeFinaleScreen("E1M8", false); !ok || got != "CREDIT" {
		t.Fatalf("episodeFinaleScreen(E1M8,false)=(%q,%t) want=(CREDIT,true)", got, ok)
	}
	if got, ok := episodeFinaleScreen("E2M8", false); !ok || got != "VICTORY2" {
		t.Fatalf("episodeFinaleScreen(E2M8,false)=(%q,%t) want=(VICTORY2,true)", got, ok)
	}
	if got, ok := episodeFinaleScreen("E3M8", false); !ok || got != "ENDPIC" {
		t.Fatalf("episodeFinaleScreen(E3M8,false)=(%q,%t) want=(ENDPIC,true)", got, ok)
	}
	if _, ok := episodeFinaleScreen("E1M8", true); ok {
		t.Fatal("secret exits should not trigger episode finale screen")
	}
	if _, ok := episodeFinaleScreen("E1M7", false); ok {
		t.Fatal("non-episode-end map should not trigger episode finale screen")
	}
}

func TestTickIntermissionProgressesToCompletion(t *testing.T) {
	sg := &sessionGame{
		intermission: sessionIntermission{
			state: sessionflow.Intermission{
				Active: true,
				Phase:  sessionflow.PhaseKills,
				Show: intermissionStats{
					MapName:     mapdata.MapName("E1M1"),
					NextMapName: mapdata.MapName("E1M2"),
				},
				Target: intermissionStats{
					MapName:     mapdata.MapName("E1M1"),
					NextMapName: mapdata.MapName("E1M2"),
					KillsPct:    4,
					ItemsPct:    4,
					SecretsPct:  4,
					TimeSec:     6,
				},
			},
		},
	}
	sawYouAreHere := false
	done := false
	for i := 0; i < 600; i++ {
		done = sg.tickIntermission()
		if sg.intermission.state.Phase == sessionflow.PhaseYouAreHere {
			sawYouAreHere = true
		}
		if done {
			break
		}
	}
	if !sawYouAreHere {
		t.Fatal("intermission did not reach YOU ARE HERE phase")
	}
	if !done {
		t.Fatal("intermission did not complete in expected ticks")
	}
}

func TestTickIntermissionCommercialSkipsEnteringPhases(t *testing.T) {
	sg := &sessionGame{
		intermission: sessionIntermission{
			state: sessionflow.Intermission{
				Active:         true,
				Phase:          sessionflow.PhaseKills,
				ShowEntering:   false,
				ShowYouAreHere: false,
				EnteringWait:   0,
				YouAreHereWait: 1,
				Show: intermissionStats{
					MapName:     mapdata.MapName("MAP01"),
					NextMapName: mapdata.MapName("MAP02"),
				},
				Target: intermissionStats{
					MapName:     mapdata.MapName("MAP01"),
					NextMapName: mapdata.MapName("MAP02"),
					KillsPct:    2,
					ItemsPct:    2,
					SecretsPct:  2,
					TimeSec:     3,
				},
			},
		},
	}
	done := false
	sawEntering := false
	for i := 0; i < 300; i++ {
		done = sg.tickIntermission()
		if sg.intermission.state.Phase == sessionflow.PhaseEntering {
			sawEntering = true
		}
		if done {
			break
		}
	}
	if sawEntering {
		t.Fatal("commercial intermission should not enter episode map phase")
	}
	if !done {
		t.Fatal("commercial intermission did not complete in expected ticks")
	}
}

func TestTickIntermissionSkipDoesNotResetFinalHold(t *testing.T) {
	sg := &sessionGame{
		intermission: sessionIntermission{
			state: sessionflow.Intermission{
				Active:  true,
				Phase:   sessionflow.PhaseYouAreHere,
				Tic:     sessionflow.IntermissionSkipInputDelayTics + 1,
				WaitTic: 5,
				Show: intermissionStats{
					MapName:     mapdata.MapName("E1M1"),
					NextMapName: mapdata.MapName("E1M2"),
				},
				Target: intermissionStats{
					MapName:     mapdata.MapName("E1M1"),
					NextMapName: mapdata.MapName("E1M2"),
				},
			},
		},
	}
	if done := sg.tickIntermissionAdvance(true); done {
		t.Fatal("final intermission hold should not complete immediately")
	}
	if got := sg.intermission.state.WaitTic; got != 4 {
		t.Fatalf("waitTic=%d want=4", got)
	}
}

func TestCollectIntermissionStats_UsesInitialSecretTotalAfterDiscovery(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{Special: 0},
				{Special: 0},
			},
		},
		secretsTotal: 2,
		secretsFound: 1,
	}
	got := collectIntermissionStats(g, "E1M1", "E1M2")
	if got.SecretsTotal != 2 {
		t.Fatalf("secretsTotal=%d want=2", got.SecretsTotal)
	}
	if got.SecretsFound != 1 {
		t.Fatalf("secretsFound=%d want=1", got.SecretsFound)
	}
	if got.SecretsPct != 50 {
		t.Fatalf("secretsPct=%d want=50", got.SecretsPct)
	}
}
