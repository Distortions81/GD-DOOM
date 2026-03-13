package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
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

func TestIntermissionParSeconds(t *testing.T) {
	cases := []struct {
		mapName mapdata.MapName
		want    int
	}{
		{mapName: "E1M1", want: 30},
		{mapName: "E2M8", want: 30},
		{mapName: "E3M9", want: 135},
		{mapName: "MAP01", want: 30},
		{mapName: "MAP15", want: 210},
		{mapName: "MAP31", want: 120},
	}
	for _, tc := range cases {
		if got := intermissionParSeconds(tc.mapName); got != tc.want {
			t.Fatalf("intermissionParSeconds(%s)=%d want=%d", tc.mapName, got, tc.want)
		}
	}
}

func TestIntermissionBackgroundName(t *testing.T) {
	if got, ok := intermissionBackgroundName(intermissionState{Episode: 1}); !ok || got != "WIMAP0" {
		t.Fatalf("episode1 background=(%q,%t) want=(WIMAP0,true)", got, ok)
	}
	if got, ok := intermissionBackgroundName(intermissionState{Episode: 4, Retail: true}); !ok || got != "INTERPIC" {
		t.Fatalf("retail e4 background=(%q,%t) want=(INTERPIC,true)", got, ok)
	}
	if got, ok := intermissionBackgroundName(intermissionState{Commercial: true}); !ok || got != "INTERPIC" {
		t.Fatalf("commercial background=(%q,%t) want=(INTERPIC,true)", got, ok)
	}
}

func TestIntermissionLevelPatchName(t *testing.T) {
	if got := intermissionLevelPatchName("E1M1"); got != "WILV00" {
		t.Fatalf("E1M1 patch=%q want WILV00", got)
	}
	if got := intermissionLevelPatchName("E4M9"); got != "WILV38" {
		t.Fatalf("E4M9 patch=%q want WILV38", got)
	}
	if got := intermissionLevelPatchName("MAP01"); got != "CWILV00" {
		t.Fatalf("MAP01 patch=%q want CWILV00", got)
	}
}

func TestTickIntermissionProgressesToShowNextAndCompletion(t *testing.T) {
	sg := &sessionGame{
		intermission: sessionIntermission{
			state: intermissionState{
				Active:     true,
				Screen:     intermissionScreenStats,
				SPState:    1,
				PauseTics:  1,
				Episode:    1,
				Last:       0,
				Next:       1,
				Show:       intermissionStats{MapName: "E1M1", NextMapName: "E1M2", KillsPct: -1, ItemsPct: -1, SecretsPct: -1, TimeSec: -1, ParSec: -1},
				Target:     intermissionStats{MapName: "E1M1", NextMapName: "E1M2", KillsPct: 2, ItemsPct: 2, SecretsPct: 2, TimeSec: 3, ParSec: 3},
				StartMusic: false,
			},
		},
	}
	sawShowNext := false
	done := false
	for i := 0; i < 400; i++ {
		skip := sg.intermission.state.SPState == 10
		if skip {
			sg.intermission.state.Tic = wiSkipInputDelay + 1
		}
		done = sg.tickIntermissionAdvance(skip)
		if sg.intermission.state.Screen == intermissionScreenShowNextLoc {
			sawShowNext = true
		}
		if done {
			break
		}
	}
	if !sawShowNext {
		t.Fatal("intermission did not reach show-next screen")
	}
	if !done {
		t.Fatal("intermission did not complete in expected ticks")
	}
}

func TestTickIntermissionCommercialSkipsShowNext(t *testing.T) {
	sg := &sessionGame{
		intermission: sessionIntermission{
			state: intermissionState{
				Active:     true,
				Screen:     intermissionScreenStats,
				SPState:    1,
				PauseTics:  1,
				Commercial: true,
				Show:       intermissionStats{MapName: "MAP01", NextMapName: "MAP02", KillsPct: -1, ItemsPct: -1, SecretsPct: -1, TimeSec: -1, ParSec: -1},
				Target:     intermissionStats{MapName: "MAP01", NextMapName: "MAP02", KillsPct: 2, ItemsPct: 2, SecretsPct: 2, TimeSec: 3, ParSec: 30},
				StartMusic: false,
			},
		},
	}
	done := false
	sawShowNext := false
	for i := 0; i < 300; i++ {
		skip := sg.intermission.state.SPState == 10
		if skip {
			sg.intermission.state.Tic = wiSkipInputDelay + 1
		}
		done = sg.tickIntermissionAdvance(skip)
		if sg.intermission.state.Screen == intermissionScreenShowNextLoc {
			sawShowNext = true
		}
		if done {
			break
		}
	}
	if sawShowNext {
		t.Fatal("commercial intermission should not use show-next screen")
	}
	if !done {
		t.Fatal("commercial intermission did not complete")
	}
}

func TestTickIntermissionRetailEpisode4UsesNoNodeScreen(t *testing.T) {
	sg := &sessionGame{
		intermission: sessionIntermission{
			state: intermissionState{
				Active:     true,
				Screen:     intermissionScreenStats,
				SPState:    10,
				PauseTics:  1,
				Tic:        wiSkipInputDelay + 1,
				Episode:    4,
				Retail:     true,
				Last:       0,
				Next:       1,
				Show:       intermissionStats{MapName: "E4M1", NextMapName: "E4M2", KillsPct: 10, ItemsPct: 20, SecretsPct: 30, TimeSec: 40, ParSec: 0},
				Target:     intermissionStats{MapName: "E4M1", NextMapName: "E4M2", KillsPct: 10, ItemsPct: 20, SecretsPct: 30, TimeSec: 40, ParSec: 0},
				StartMusic: false,
			},
		},
	}
	_ = sg.tickIntermissionAdvance(true)
	if sg.intermission.state.Screen != intermissionScreenShowNextLoc {
		t.Fatalf("screen=%v want show-next", sg.intermission.state.Screen)
	}
	if got, ok := intermissionBackgroundName(sg.intermission.state); !ok || got != "INTERPIC" {
		t.Fatalf("retail e4 background=(%q,%t) want=(INTERPIC,true)", got, ok)
	}
	if nodes := intermissionEpisodeNodePos(sg.intermission.state.Episode); nodes != nil {
		t.Fatalf("episode4 nodes=%v want nil", nodes)
	}
}

func TestTickIntermissionSkipFastForwardsStatsThenExits(t *testing.T) {
	sg := &sessionGame{
		intermission: sessionIntermission{
			state: intermissionState{
				Active:     true,
				Screen:     intermissionScreenStats,
				SPState:    2,
				PauseTics:  1,
				Episode:    1,
				Last:       0,
				Next:       1,
				Tic:        wiSkipInputDelay + 1,
				Show:       intermissionStats{MapName: "E1M1", NextMapName: "E1M2", KillsPct: 0, ItemsPct: 0, SecretsPct: 0, TimeSec: 0, ParSec: 0},
				Target:     intermissionStats{MapName: "E1M1", NextMapName: "E1M2", KillsPct: 10, ItemsPct: 20, SecretsPct: 30, TimeSec: 40, ParSec: 50},
				StartMusic: false,
			},
		},
	}
	_ = sg.tickIntermissionAdvance(true)
	if sg.intermission.state.SPState != 10 {
		t.Fatalf("spState=%d want=10", sg.intermission.state.SPState)
	}
	if sg.intermission.state.Show.KillsPct != 10 || sg.intermission.state.Show.TimeSec != 40 {
		t.Fatalf("fast-forward show=%+v want final counters", sg.intermission.state.Show)
	}
	_ = sg.tickIntermissionAdvance(true)
	if sg.intermission.state.Screen != intermissionScreenShowNextLoc {
		t.Fatalf("screen=%v want show-next after second skip", sg.intermission.state.Screen)
	}
}

func TestFinishIntermissionTransitionsAfterStateClears(t *testing.T) {
	next := &mapdata.Map{Name: "E1M2"}
	sg := &sessionGame{
		intermission: sessionIntermission{
			state:   intermissionState{},
			nextMap: next,
		},
		opts: Options{
			MapMusicInfo: func(mapName string) (string, string) {
				return mapName, "TEST MUSIC"
			},
		},
	}

	sg.finishIntermission()

	if sg.current != "E1M2" {
		t.Fatalf("current=%q want E1M2", sg.current)
	}
	if sg.intermission.nextMap != nil || sg.intermission.state.Active {
		t.Fatalf("intermission not cleared: %+v", sg.intermission)
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
