package automap

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

func TestTickIntermissionProgressesToCompletion(t *testing.T) {
	sg := &sessionGame{
		intermission: sessionIntermission{
			active: true,
			phase:  intermissionPhaseKills,
			show: intermissionStats{
				mapName:     mapdata.MapName("E1M1"),
				nextMapName: mapdata.MapName("E1M2"),
			},
			target: intermissionStats{
				mapName:     mapdata.MapName("E1M1"),
				nextMapName: mapdata.MapName("E1M2"),
				killsPct:    4,
				itemsPct:    4,
				secretsPct:  4,
				timeSec:     6,
			},
		},
	}
	sawYouAreHere := false
	done := false
	for i := 0; i < 600; i++ {
		done = sg.tickIntermission()
		if sg.intermission.phase == intermissionPhaseYouAreHere {
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
