package lobby

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gddoom/internal/netgame"
)

func testPack() Pack {
	return Pack{ID: "doom1", Name: "Doom shareware", WADHashes: []string{strings.Repeat("a", 64), strings.Repeat("b", 64)}, Maps: []string{"E1M1", "E1M2"}}
}

func testRequest() CreateRequest {
	return CreateRequest{RequestID: strings.Repeat("01", 16), Name: "Co-op room", Settings: Settings{PackID: "doom1", Map: "E1M1", Mode: "coop", Skill: 3, PlayerLimit: 4}}
}

func testRoom(t *testing.T) Room {
	t.Helper()
	request := testRequest()
	manifest, err := ValidateSettings(request.Settings, testPack())
	if err != nil {
		t.Fatal(err)
	}
	return Room{ID: "room-1", Name: request.Name, Address: "wss://play.example/rooms/room-1/netplay", State: "ready", Settings: request.Settings, Manifest: manifest, Players: 2, PlayerLimit: 4, ReservedPlayers: 1, Spectators: 1, CreatedAt: time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)}
}

func testState(t *testing.T) State {
	return State{Version: APIVersion, Packs: []Pack{testPack()}, Rooms: []Room{testRoom(t)}, MaxRooms: 4}
}

func TestSettingsBindOrderedContentAndEveryRule(t *testing.T) {
	pack, settings := testPack(), testRequest().Settings
	settings.Mode, settings.Skill, settings.PlayerLimit = "deathmatch", 5, 2
	settings.NoMonsters, settings.FastMonsters, settings.RespawnMonsters, settings.FriendlyFire = true, true, true, true
	settings.FragLimit, settings.TimeLimitSeconds = 25, 120
	manifest, err := ValidateSettings(settings, pack)
	if err != nil {
		t.Fatal(err)
	}
	want := netgame.CompatibilityManifest{Simulation: netgame.SimulationVersion, WADHashes: append([]string(nil), pack.WADHashes...), Map: "E1M1", Mode: "deathmatch", Skill: 5, NoMonsters: true, FastMonsters: true, RespawnMonsters: true, FriendlyFire: true, RespawnDelayTics: 35, FragLimit: 25, TimeLimitTics: 4200}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("manifest got %+v want %+v", manifest, want)
	}
	key, _ := manifest.Key()
	pack.WADHashes[0], pack.WADHashes[1] = pack.WADHashes[1], pack.WADHashes[0]
	if !reflect.DeepEqual(manifest, want) {
		t.Fatal("manifest aliases the caller's catalog")
	}
	reordered, _ := ValidateSettings(settings, pack)
	reorderedKey, _ := reordered.Key()
	if key == reorderedKey {
		t.Fatal("WAD ordering did not affect compatibility")
	}
}

func TestSettingsRejectInvalidSelectionsAndBounds(t *testing.T) {
	for name, change := range map[string]func(*Settings){
		"unknown pack":  func(s *Settings) { s.PackID = "other" },
		"unknown map":   func(s *Settings) { s.Map = "E1M3" },
		"map path":      func(s *Settings) { s.Map = "../E1M1" },
		"invalid mode":  func(s *Settings) { s.Mode = "single" },
		"low skill":     func(s *Settings) { s.Skill = 0 },
		"high skill":    func(s *Settings) { s.Skill = 6 },
		"low capacity":  func(s *Settings) { s.PlayerLimit = 0 },
		"high capacity": func(s *Settings) { s.PlayerLimit = 5 },
		"negative frag": func(s *Settings) { s.FragLimit = -1 },
		"negative time": func(s *Settings) { s.TimeLimitSeconds = -1 },
		"time overflow": func(s *Settings) { s.TimeLimitSeconds = math.MaxUint32/netgame.TickRate + 1 },
	} {
		t.Run(name, func(t *testing.T) {
			settings := testRequest().Settings
			change(&settings)
			if _, err := ValidateSettings(settings, testPack()); err == nil {
				t.Fatal("invalid settings accepted")
			}
		})
	}
}

func TestPackAndCreateRequestBounds(t *testing.T) {
	for name, change := range map[string]func(*Pack){
		"empty id":        func(p *Pack) { p.ID = "" },
		"path id":         func(p *Pack) { p.ID = "../doom" },
		"control name":    func(p *Pack) { p.Name = "bad\nname" },
		"long name":       func(p *Pack) { p.Name = strings.Repeat("é", 65) },
		"no hashes":       func(p *Pack) { p.WADHashes = nil },
		"bad hash":        func(p *Pack) { p.WADHashes[0] = "not-sha256" },
		"uppercase hash":  func(p *Pack) { p.WADHashes[0] = strings.Repeat("A", 64) },
		"too many hashes": func(p *Pack) { p.WADHashes = make([]string, 65) },
		"no maps":         func(p *Pack) { p.Maps = nil },
		"too many maps":   func(p *Pack) { p.Maps = make([]string, MaxMaps+1) },
		"duplicate maps":  func(p *Pack) { p.Maps = []string{"E1M1", "E1M1"} },
	} {
		t.Run(name, func(t *testing.T) {
			pack := testPack()
			change(&pack)
			if err := ValidatePack(pack); err == nil {
				t.Fatal("invalid pack accepted")
			}
		})
	}
	first, err := NewRequestID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRequestID()
	if err != nil || first == second {
		t.Fatalf("request ID generation: %q %q %v", first, second, err)
	}
	request := testRequest()
	request.RequestID, request.Name = first, strings.Repeat("é", 64)
	if err := ValidateCreateRequest(request); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "abc", strings.Repeat("z", 32), strings.Repeat("AB", 16)} {
		request.RequestID = id
		if ValidateCreateRequest(request) == nil {
			t.Fatalf("invalid request ID %q accepted", id)
		}
	}
}

func TestNormalizeAddress(t *testing.T) {
	for input, want := range map[string]string{" https://play.example/ ": "https://play.example", "http://127.0.0.1:1234/lobby/": "http://127.0.0.1:1234/lobby", "http://[::1]:1234": "http://[::1]:1234"} {
		got, err := NormalizeAddress(input)
		if err != nil || got != want {
			t.Fatalf("%q: got %q err=%v", input, got, err)
		}
	}
	for _, input := range []string{"", "play.example", "wss://play.example", "file:///tmp/private", "https://user:pass@play.example", "https://play.example?x=1", "https://play.example?", "https://play.example#", "https://play.example/#frag", "https://play.example/a/../b", "https://play.example/%2fpath", "https://play.example:0", "https://play.example:65536", "https://play.example:", "https://play.example/a\\b", "https://play.example/bad path"} {
		if got, err := NormalizeAddress(input); err == nil {
			t.Fatalf("invalid address %q normalized as %q", input, got)
		}
	}
}

func TestRoomJSONKeepsSnakeCaseWithoutChangingGameManifest(t *testing.T) {
	room := testRoom(t)
	key, _ := room.Manifest.Key()
	data, err := json.Marshal(room)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"WADHashes"`)) || !bytes.Contains(data, []byte(`"wad_hashes"`)) || !bytes.Contains(data, []byte(`"respawn_delay_tics"`)) {
		t.Fatalf("unexpected lobby JSON: %s", data)
	}
	var decoded Room
	if err := decodeStrict(data, &decoded); err != nil || !reflect.DeepEqual(decoded, room) {
		t.Fatalf("round trip: decoded=%+v err=%v", decoded, err)
	}
	decodedKey, _ := decoded.Manifest.Key()
	gameJSON, _ := json.Marshal(decoded.Manifest)
	if decodedKey != key || !bytes.Contains(gameJSON, []byte(`"WADHashes"`)) {
		t.Fatal("lobby encoding changed game compatibility")
	}
}

func TestStateAcceptsProgressionAndTerminalHistory(t *testing.T) {
	state := testState(t)
	state.MaxRooms = 1
	state.Rooms[0].Manifest.Map = "E1M2"
	state.Rooms[0].Manifest.Simulation = "another-engine-version"
	ended := state.Rooms[0]
	ended.ID, ended.State, ended.Address = "older-room", "ended", ""
	ended.Players, ended.ReservedPlayers, ended.Spectators = 0, 0, 0
	state.Rooms = append(state.Rooms, ended)
	if err := validateState(state); err != nil {
		t.Fatal(err)
	}
}

func TestStateRejectsInconsistentDiscovery(t *testing.T) {
	for name, change := range map[string]func(*State){
		"version":             func(s *State) { s.Version++ },
		"capacity":            func(s *State) { s.MaxRooms = MaxRooms + 1 },
		"too many rooms":      func(s *State) { s.Rooms = make([]Room, MaxRooms+1) },
		"too many packs":      func(s *State) { s.Packs = make([]Pack, MaxPacks+1) },
		"duplicate packs":     func(s *State) { s.Packs = append(s.Packs, s.Packs[0]) },
		"duplicate rooms":     func(s *State) { s.Rooms = append(s.Rooms, s.Rooms[0]) },
		"unknown pack":        func(s *State) { s.Rooms[0].Settings.PackID = "other" },
		"wrong hashes":        func(s *State) { s.Rooms[0].Manifest.WADHashes[0] = strings.Repeat("c", 64) },
		"wrong rules":         func(s *State) { s.Rooms[0].Manifest.FriendlyFire = true },
		"wrong capacity":      func(s *State) { s.Rooms[0].PlayerLimit = 2 },
		"unknown map":         func(s *State) { s.Rooms[0].Manifest.Map = "E1M9" },
		"unknown state":       func(s *State) { s.Rooms[0].State = "surprise" },
		"negative players":    func(s *State) { s.Rooms[0].Players = -1 },
		"too many players":    func(s *State) { s.Rooms[0].Players = 5 },
		"too many reserved":   func(s *State) { s.Rooms[0].ReservedPlayers = 3 },
		"too many spectators": func(s *State) { s.Rooms[0].Spectators = netgame.MaxSpectators + 1 },
		"no creation time":    func(s *State) { s.Rooms[0].CreatedAt = time.Time{} },
		"no ready address":    func(s *State) { s.Rooms[0].Address = "" },
		"credentials":         func(s *State) { s.Rooms[0].Address = "wss://secret@play.example/netplay" },
		"not websocket":       func(s *State) { s.Rooms[0].Address = "http://play.example/netplay" },
	} {
		t.Run(name, func(t *testing.T) {
			state := testState(t)
			change(&state)
			if err := validateState(state); err == nil {
				t.Fatal("invalid discovery accepted")
			}
		})
	}
}

func TestHTTPFetchAndCreateUseValidatedContract(t *testing.T) {
	state, room, request := testState(t), testRoom(t), testRequest()
	var gets, posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Header.Get("Accept") != "application/json" {
			t.Error("missing JSON accept")
		}
		switch r.URL.Path {
		case "/prefix/api/v1/lobby":
			gets.Add(1)
			if r.Method != http.MethodGet {
				t.Errorf("fetch method %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(state)
		case "/prefix/api/v1/rooms":
			posts.Add(1)
			var got CreateRequest
			if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" || json.NewDecoder(r.Body).Decode(&got) != nil || got != request {
				t.Error("creation request did not preserve identity/settings")
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(room)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	got, err := Fetch(context.Background(), server.URL+"/prefix/")
	if err != nil || !reflect.DeepEqual(got, state) {
		t.Fatalf("fetch: %+v, %v", got, err)
	}
	created, err := Create(context.Background(), server.URL+"/prefix", request)
	if err != nil || !reflect.DeepEqual(created, room) || gets.Load() != 1 || posts.Load() != 1 {
		t.Fatalf("create: %+v, %v gets=%d posts=%d", created, err, gets.Load(), posts.Load())
	}
}

func TestHTTPRejectsMalformedOrUnboundedResponses(t *testing.T) {
	valid, _ := json.Marshal(testState(t))
	for _, test := range []struct {
		name, body, contentType string
		status                  int
	}{
		{"unknown top field", strings.TrimSuffix(string(valid), "}") + `,"command":"run"}`, "application/json", 200},
		{"unknown nested field", strings.Replace(string(valid), `"skill":3`, `"skill":3,"args":["-x"]`, 1), "application/json", 200},
		{"unknown manifest field", strings.Replace(string(valid), `"simulation":`, `"new_rule":true,"simulation":`, 1), "application/json", 200},
		{"duplicate field", strings.Replace(string(valid), `"version":1`, `"version":2,"version":1`, 1), "application/json", 200},
		{"case duplicate", strings.Replace(string(valid), `"version":1`, `"VERSION":2,"version":1`, 1), "application/json", 200},
		{"trailing data", string(valid) + `{}`, "application/json", 200},
		{"wrong content type", string(valid), "text/html", 200},
		{"unexpected success", string(valid), "application/json", 202},
		{"large body", strings.Repeat(" ", MaxResponseBytes+1), "application/json", 200},
		{"invalid utf8", strings.Replace(string(valid), "Doom shareware", "bad\xffname", 1), "application/json", 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			if _, err := Fetch(context.Background(), server.URL); err == nil {
				t.Fatal("malformed HTTP response accepted")
			}
		})
	}
}

func TestHTTPCreateRefusalMismatchAndNoRetry(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusConflict, http.StatusTooManyRequests} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			room := testRoom(t)
			room.Settings.Skill = 4
			room.Manifest.Skill = 4
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if status >= 400 {
					_, _ = io.WriteString(w, `{"error":"room capacity exhausted"}`)
				} else {
					_ = json.NewEncoder(w).Encode(room)
				}
			}))
			defer server.Close()
			_, err := Create(context.Background(), server.URL, testRequest())
			if err == nil || calls.Load() != 1 {
				t.Fatalf("invalid create response accepted or request retried: %v calls=%d", err, calls.Load())
			}
			var httpErr *HTTPError
			if status >= 400 && (!errors.As(err, &httpErr) || httpErr.StatusCode != status || httpErr.Message != "room capacity exhausted") {
				t.Fatalf("lost structured refusal: %v", err)
			}
		})
	}
}

func TestHTTPDoesNotFollowRedirectOrSendInvalidRequest(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	if _, err := Fetch(context.Background(), server.URL); err == nil || hits.Load() != 1 {
		t.Fatalf("redirect followed: err=%v calls=%d", err, hits.Load())
	}
	request := testRequest()
	request.RequestID = "invalid"
	if _, err := Create(context.Background(), server.URL, request); err == nil || hits.Load() != 1 {
		t.Fatal("invalid create was sent")
	}
}

func TestHTTPCancellationClosesBlockedRequests(t *testing.T) {
	for _, create := range []bool{false, true} {
		t.Run(fmt.Sprint(create), func(t *testing.T) {
			started, stopped := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(started)
				<-r.Context().Done()
				close(stopped)
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				if create {
					_, err := Create(ctx, server.URL, testRequest())
					done <- err
				} else {
					_, err := Fetch(ctx, server.URL)
					done <- err
				}
			}()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("request did not start")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation returned %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("request ignored cancellation")
			}
			select {
			case <-stopped:
			case <-time.After(2 * time.Second):
				t.Fatal("canceled request left server connection alive")
			}
		})
	}
}
