package doomruntime

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestMultiplayerLobbyNameCommitsPendingText(t *testing.T) {
	for _, tc := range []struct {
		name, earlier, final, want string
		key                        ebiten.Key
	}{
		{name: "replace and confirm", final: "Deathmatch Test", want: "Deathmatch Test", key: ebiten.KeyEnter},
		{name: "append and confirm", earlier: "Deathmatch ", final: "Test", want: "Deathmatch Test", key: ebiten.KeyKPEnter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sg := lobbyMenuTestSession(t)
			sg.openAuthorityCreate()
			m := &sg.multiplayer.lobby
			m.request.RequestID = "retry-id"
			lobbyMenuKey(t, sg, ebiten.KeyEnter)
			if tc.earlier != "" {
				sg.input = sessionInputSnapshot{inputChars: []rune(tc.earlier)}
				if err := sg.tickFrontendMultiplayer(); err != nil {
					t.Fatal(err)
				}
			}
			// Both events were sampled during host frames before one menu tic.
			sg.input = sessionInputSnapshot{
				inputChars:      []rune(tc.final),
				justPressedKeys: map[ebiten.Key]int{tc.key: 1},
			}
			if err := sg.tickFrontendMultiplayer(); err != nil {
				t.Fatal(err)
			}
			if m.request.Name != tc.want || m.request.RequestID != "" || sg.multiplayer.editing || m.editRoomName {
				t.Fatalf("pending text was not committed: name=%q retry=%q editing=%v", m.request.Name, m.request.RequestID, sg.multiplayer.editing)
			}
			if len(sg.input.inputChars) != 0 || sg.multiplayer.request.Name != "Doomer" {
				t.Fatal("commit retained text or changed the independent player name")
			}
		})
	}
}
