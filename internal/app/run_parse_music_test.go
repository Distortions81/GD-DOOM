package app

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestMapMusicLumpNameEpisode4Aliases(t *testing.T) {
	cases := []struct {
		mapName string
		want    string
	}{
		{mapName: "E4M1", want: "D_E3M4"},
		{mapName: "E4M2", want: "D_E3M2"},
		{mapName: "E4M3", want: "D_E3M3"},
		{mapName: "E4M4", want: "D_E1M5"},
		{mapName: "E4M5", want: "D_E2M7"},
		{mapName: "E4M6", want: "D_E2M4"},
		{mapName: "E4M7", want: "D_E2M6"},
		{mapName: "E4M8", want: "D_E2M5"},
		{mapName: "E4M9", want: "D_E1M9"},
	}

	for _, tc := range cases {
		got, ok := mapMusicLumpName(mapdata.MapName(tc.mapName))
		if !ok {
			t.Fatalf("%s not mapped", tc.mapName)
		}
		if got != tc.want {
			t.Fatalf("%s mapped to %s want %s", tc.mapName, got, tc.want)
		}
	}
}

func TestMapMusicLumpNamePreservesEpisode1To3AndCommercialMappings(t *testing.T) {
	cases := []struct {
		mapName string
		want    string
	}{
		{mapName: "E1M1", want: "D_E1M1"},
		{mapName: "E3M9", want: "D_E3M9"},
		{mapName: "MAP01", want: "D_RUNNIN"},
		{mapName: "MAP31", want: "D_EVIL"},
		{mapName: "MAP32", want: "D_ULTIMA"},
	}

	for _, tc := range cases {
		got, ok := mapMusicLumpName(mapdata.MapName(tc.mapName))
		if !ok {
			t.Fatalf("%s not mapped", tc.mapName)
		}
		if got != tc.want {
			t.Fatalf("%s mapped to %s want %s", tc.mapName, got, tc.want)
		}
	}
}
