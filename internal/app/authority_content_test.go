package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gddoom/internal/doomruntime"
	"gddoom/internal/gameplay"
	"gddoom/internal/lobby"
	"gddoom/internal/music"
	"gddoom/internal/netgame"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/sound"
	"gddoom/internal/wad"
)

func authorityContentFile(name string, data []byte, downloadable bool) lobby.PackFile {
	return lobby.PackFile{Name: name, Size: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Downloadable: downloadable}
}

func authorityContentPack(id string, files ...lobby.PackFile) lobby.Pack {
	pack := lobby.Pack{ID: id, Name: id, Files: files, Maps: []string{"E1M1", "E1M2", "E1M3"}}
	for _, file := range files {
		pack.WADHashes = append(pack.WADHashes, file.SHA256)
	}
	return pack
}

func authorityContentRoom(t *testing.T, pack lobby.Pack) lobby.Room {
	t.Helper()
	settings := lobby.Settings{PackID: pack.ID, Map: "E1M1", Mode: "coop", Skill: 3, PlayerLimit: 2, NoMonsters: true}
	manifest, err := lobby.ValidateSettings(settings, pack)
	if err != nil {
		t.Fatal(err)
	}
	return lobby.Room{ID: "room", Name: "Content test", State: "ready", Address: "wss://example.test/rooms/room/netplay", Settings: settings, Manifest: manifest, PlayerLimit: 2, CreatedAt: time.Now()}
}

type authorityContentHTTP struct {
	server    *httptest.Server
	state     atomic.Pointer[lobby.State]
	catalogs  atomic.Int32
	downloads atomic.Int32
}

func newAuthorityContentHTTP(t *testing.T, packs []lobby.Pack, content map[string][]byte) *authorityContentHTTP {
	t.Helper()
	f := &authorityContentHTTP{}
	f.state.Store(&lobby.State{Version: lobby.APIVersion, MaxRooms: 4, Packs: packs})
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/lobby":
			f.catalogs.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(f.state.Load())
		case strings.HasPrefix(r.URL.Path, "/api/v1/content/"):
			f.downloads.Add(1)
			data, ok := content[strings.TrimPrefix(r.URL.Path, "/api/v1/content/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func TestResolveAuthorityContentCombinesPrivateBaseAndDownloadedOverlay(t *testing.T) {
	base := buildAppTestWAD("IWAD", []appTestLump{{name: "TEST", data: []byte{1, 2, 3}}})
	overlay := buildAppTestWAD("PWAD", []appTestLump{{name: "TEST", data: []byte{7, 8, 9}}})
	baseFile, overlayFile := authorityContentFile("private.wad", base, false), authorityContentFile("free.wad", overlay, true)
	pack := authorityContentPack("mixed", baseFile, overlayFile)
	f := newAuthorityContentHTTP(t, []lobby.Pack{pack}, map[string][]byte{overlayFile.SHA256: overlay})
	path := filepath.Join(t.TempDir(), "private.wad")
	if err := os.WriteFile(path, base, 0600); err != nil {
		t.Fatal(err)
	}
	var progress []runtimecfg.AuthorityContentProgress
	data, err := resolveAuthorityContent(context.Background(), f.server.URL, pack, []string{path}, []string{baseFile.SHA256}, func(p runtimecfg.AuthorityContentProgress) { progress = append(progress, p) })
	if err != nil || len(data) != 2 || !bytes.Equal(data[0], base) || !bytes.Equal(data[1], overlay) || f.downloads.Load() != 1 {
		t.Fatalf("ordered private/downloaded stack failed: %v, downloads=%d", err, f.downloads.Load())
	}
	if len(progress) != 2 || progress[0].Name != overlayFile.Name || progress[0].Total != overlayFile.Size || progress[1].Stage != "VERIFIED" || progress[1].Received != overlayFile.Size {
		t.Fatalf("progress includes private bytes or lacks verification: %+v", progress)
	}
	left, _ := wad.OpenData("base", data[0])
	right, _ := wad.OpenData("overlay", data[1])
	merged := wad.Merge(left, right)
	lump, _ := merged.LumpByName("TEST")
	got, _ := merged.LumpDataView(lump)
	if !bytes.Equal(got, []byte{7, 8, 9}) {
		t.Fatal("target WAD order lost overlay priority")
	}
}

func TestResolveAuthorityContentPreflightAndChangedLocalFile(t *testing.T) {
	base := buildAppTestWAD("IWAD", []appTestLump{{name: "TEST", data: []byte{1}}})
	patch := buildAppTestWAD("PWAD", []appTestLump{{name: "TEST", data: []byte{2}}})
	baseFile, patchFile := authorityContentFile("free-base.wad", base, true), authorityContentFile("private-patch.wad", patch, false)
	pack := authorityContentPack("mixed", baseFile, patchFile)
	f := newAuthorityContentHTTP(t, []lobby.Pack{pack}, map[string][]byte{baseFile.SHA256: base})
	if data, err := resolveAuthorityContent(context.Background(), f.server.URL, pack, nil, nil, nil); err == nil || data != nil || f.downloads.Load() != 0 {
		t.Fatal("download started before detecting a later missing private file")
	}
	path := filepath.Join(t.TempDir(), "private.wad")
	if err := os.WriteFile(path, base, 0600); err != nil {
		t.Fatal(err)
	}
	// The frozen hash refers to another same-sized WAD. Never silently use
	// changed local bytes, even when the advertised filename still exists.
	changed := authorityContentPack("changed", patchFile, baseFile)
	if data, err := resolveAuthorityContent(context.Background(), f.server.URL, changed, []string{path}, []string{patchFile.SHA256}, nil); err == nil || data != nil || !strings.Contains(err.Error(), "local WAD changed") || f.downloads.Load() != 0 {
		t.Fatalf("changed local identity accepted or downloaded first: %v", err)
	}
}

func TestPrepareAuthorityContentRefetchesCatalogAndChecksRules(t *testing.T) {
	data := buildAppTestWAD("IWAD", []appTestLump{{name: "TEST", data: []byte{1}}})
	file := authorityContentFile("free.wad", data, true)
	pack := authorityContentPack("free", file)
	room := authorityContentRoom(t, pack)
	f := newAuthorityContentHTTP(t, []lobby.Pack{pack}, map[string][]byte{file.SHA256: data})
	for _, change := range []func(*lobby.Room){
		func(r *lobby.Room) { r.Manifest.Skill = 4 },
		func(r *lobby.Room) { r.Manifest.WADHashes = []string{strings.Repeat("a", 64)} },
		func(r *lobby.Room) { r.Settings.PackID = "removed" },
	} {
		stale := room
		change(&stale)
		prepared, err := prepareAuthorityContent(context.Background(), f.server.URL, stale, nil, nil, nil, nil)
		if err == nil || prepared.Load != nil || prepared.Cancel != nil {
			t.Fatal("stale catalog/settings produced a preparation")
		}
	}
	if f.catalogs.Load() != 3 || f.downloads.Load() != 0 {
		t.Fatal("preparation did not refetch or downloaded before validation")
	}
	ctx, cancel := context.WithCancel(context.Background())
	prepared, err := prepareAuthorityContent(ctx, f.server.URL, room, nil, nil, func(runtimecfg.AuthorityContentProgress) { cancel() }, nil)
	if !errors.Is(err, context.Canceled) || prepared.Load != nil || prepared.Cancel != nil || f.downloads.Load() != 0 {
		t.Fatalf("canceled download preparation escaped: %v", err)
	}
	prior := f.catalogs.Load()
	if _, err := prepareAuthorityContent(ctx, f.server.URL, room, nil, nil, nil, nil); !errors.Is(err, context.Canceled) || f.catalogs.Load() != prior {
		t.Fatal("pre-canceled preparation made a request")
	}
	// A live room may rotate while its immutable creation settings retain
	// the first map; prepare the currently advertised map in that same pack.
	room.Manifest.Map = "E1M2"
	prepared, err = prepareAuthorityContent(context.Background(), f.server.URL, room, nil, nil, nil, nil)
	if err != nil {
		t.Fatal("rotated room content refused:", err)
	}
	prepared.Cancel()
	prepared.Cancel()
}

func TestPrepareAuthorityContentRebuildsAssetsAndCallbacksAcrossReloads(t *testing.T) {
	baseBytes, err := os.ReadFile(findLocalWADOrSkip(t, "DOOM1.WAD"))
	if err != nil {
		t.Fatal(err)
	}
	base, err := wad.OpenData("DOOM1.WAD", baseBytes)
	if err != nil {
		t.Fatal(err)
	}
	readLump := func(name string) []byte {
		t.Helper()
		lump, ok := base.LumpByName(name)
		if !ok {
			t.Fatal("fixture missing", name)
		}
		data, err := base.LumpData(lump)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	basePath := filepath.Join(t.TempDir(), "DOOM1.WAD")
	if err := os.WriteFile(basePath, baseBytes, 0600); err != nil {
		t.Fatal(err)
	}
	baseFile := authorityContentFile("DOOM1.WAD", baseBytes, false)
	var packs []lobby.Pack
	var patches, musicBytes [][]byte
	content := make(map[string][]byte)
	for i, menu := range []string{"M_OPTION", "M_NGAME"} {
		palette := readLump("PLAYPAL")
		palette[0], palette[1], palette[2] = byte(17+i*30), 31, 47
		mus := buildAppTestMUS([]byte{0x1f, byte(35 + i), 0x60})
		patch := buildAppTestWAD("PWAD", []appTestLump{
			{name: "PLAYPAL", data: palette}, {name: "M_MULTI", data: readLump(menu)},
			{name: "DSPISTOL", data: readLump("DSSHOTGN")}, {name: "D_E1M1", data: mus},
		})
		file := authorityContentFile(fmt.Sprintf("overlay%d.wad", i), patch, true)
		packs = append(packs, authorityContentPack(fmt.Sprintf("pack%d", i), baseFile, file))
		patches, musicBytes = append(patches, patch), append(musicBytes, mus)
		content[file.SHA256] = patch
	}
	f := newAuthorityContentHTTP(t, packs, content)
	current := runtimecfg.Options{
		Width: 640, Height: 400, StartZoom: 1, GameMode: "single", SkillLevel: 3, PlayerSlot: 1,
		SourcePortMode: true, InitialDetailLevel: 1, AutoDetail: true, InitialGammaLevel: 2,
		MouseLook: true, MouseInvert: true, MouseLookSpeed: 1.75, KeyboardTurnSpeed: 1.25,
		MusicBackend: music.BackendImpSynth, MusicVolume: .3, SFXVolume: .7, MUSVolumeCompression: 1,
		AlwaysRun: true, AutoWeaponSwitch: true, NoVsync: true, SmoothCameraYaw: true,
		VoiceCodec: "g726", VoiceG726BitsPerSample: 4, VoiceGateEnabled: true, VoiceGateThreshold: .125,
		InputBindings:         runtimecfg.DefaultInputBindings(),
		AuthorityJoinDefaults: runtimecfg.AuthorityJoinRequest{Name: "Doomer"},
		AuthorityServers:      []runtimecfg.AuthorityServerEntry{{Address: "favorite:6666", Label: "Favorite"}},
	}
	current.InputBindings.Fire = runtimecfg.KeyBinding{"F", "MB1"}
	var settingsCalls, bindingsCalls, serversCalls, voiceCalls, retiredRuntimeCalls int
	current.OnRuntimeSettingsChanged = func(gameplay.RuntimeSettings) { settingsCalls++ }
	current.OnInputBindingsChanged = func(runtimecfg.InputBindings) { bindingsCalls++ }
	current.OnAuthorityServersChanged = func([]runtimecfg.AuthorityServerEntry) error { serversCalls++; return nil }
	current.OnVoiceSettingsChanged = func(runtimecfg.VoiceSettings) error { voiceCalls++; return nil }
	if err := configureAuthorityLobby(&current, []string{basePath}, f.server.URL); err != nil {
		t.Fatal(err)
	}
	var previousCleanup func()
	var previousMemoryPath string
	for i, pack := range packs {
		room := authorityContentRoom(t, pack)
		if i == 1 {
			room.Manifest.Map = "E1M2"
		}
		prepared, err := current.AuthorityPrepareRoom(context.Background(), f.server.URL, room, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(prepared.Cancel)
		if previousCleanup != nil {
			previousCleanup()
			if _, ok := wad.EmbeddedDataForPath(previousMemoryPath); ok {
				t.Fatal("retired bundle retained its memory registration")
			}
		}
		// Simulate runtime wrappers: preparation must restore the original
		// host persistence callbacks, not capture a retired session closure.
		current.OnRuntimeSettingsChanged = func(gameplay.RuntimeSettings) { retiredRuntimeCalls++ }
		current.OnInputBindingsChanged = func(runtimecfg.InputBindings) { retiredRuntimeCalls++ }
		current.OnAuthorityServersChanged = func([]runtimecfg.AuthorityServerEntry) error { retiredRuntimeCalls++; return nil }
		current.OnVoiceSettingsChanged = func(runtimecfg.VoiceSettings) error { retiredRuntimeCalls++; return nil }
		bundle, err := prepared.Load(current)
		if err != nil {
			t.Fatal(err)
		}
		opts := bundle.Options
		if opts.AuthorityContentCleanup == nil || opts.AuthorityJoin == nil || opts.AuthorityPrepareRoom == nil || !slices.Equal(opts.AuthorityWADHashes, pack.WADHashes) || string(bundle.Map.Name) != room.Manifest.Map {
			t.Fatal("replacement content lacks ownership, identity, callbacks, or current room map")
		}
		if opts.InputBindings != current.InputBindings || opts.MouseLookSpeed != current.MouseLookSpeed || opts.MouseInvert != current.MouseInvert || opts.KeyboardTurnSpeed != current.KeyboardTurnSpeed || opts.InitialDetailLevel != current.InitialDetailLevel || opts.AutoDetail != current.AutoDetail || opts.MusicVolume != current.MusicVolume || opts.SFXVolume != current.SFXVolume || !opts.AlwaysRun || !opts.SmoothCameraYaw || opts.VoiceGateThreshold != current.VoiceGateThreshold || !reflect.DeepEqual(opts.AuthorityServers, current.AuthorityServers) {
			t.Fatal("replacement reset live controls, display, audio, voice, or saved servers")
		}
		opts.OnRuntimeSettingsChanged(gameplay.RuntimeSettings{})
		opts.OnInputBindingsChanged(runtimecfg.InputBindings{})
		_ = opts.OnAuthorityServersChanged(nil)
		_ = opts.OnVoiceSettingsChanged(runtimecfg.VoiceSettings{})
		if settingsCalls != i+1 || bindingsCalls != i+1 || serversCalls != i+1 || voiceCalls != i+1 || retiredRuntimeCalls != 0 {
			t.Fatal("host callbacks did not survive content replacement cleanly")
		}
		patch, _ := wad.OpenData("overlay", patches[i])
		merged := wad.Merge(base, patch)
		set, err := doomtex.LoadFromWAD(merged)
		if err != nil {
			t.Fatal(err)
		}
		wantMenu, w, h, _, _, err := set.BuildPatchRGBA("M_MULTI", 0)
		if err != nil {
			t.Fatal(err)
		}
		gotMenu := opts.MenuPatchBank["M_MULTI"]
		if len(opts.DoomPaletteRGBA) < 4 || !bytes.Equal(opts.DoomPaletteRGBA[:4], []byte{byte(17 + i*30), 31, 47, 255}) || gotMenu.Width != w || gotMenu.Height != h || !bytes.Equal(gotMenu.RGBA, wantMenu) {
			t.Fatal("replacement palette/authored menu artwork did not reach imported banks")
		}
		wantSound := buildAutomapSoundBank(sound.ImportDigitalSounds(merged), true).ShootPistol
		if !bytes.Equal(opts.SoundBank.ShootPistol.Data, wantSound.Data) || len(wantSound.Data) == 0 {
			t.Fatal("replacement pistol sound was not imported")
		}
		gotMusic, err := opts.MapMusicLoader("E1M1")
		wantMusic, musicErr := music.ParseMUSData(musicBytes[i])
		wantMusic = music.ApplyMUSVolumeCompression(wantMusic, current.MUSVolumeCompression)
		if err != nil || musicErr != nil || !reflect.DeepEqual(gotMusic, wantMusic) {
			t.Fatal("map music loader retained old content")
		}
		if next, err := opts.NewGameLoader("E1M3"); err != nil || next.Name != "E1M3" {
			t.Fatal("new-game map loader lost replacement WAD")
		}
		if next, name, err := bundle.NextMap("E1M1", false); err != nil || name != "E1M2" || next.Name != name {
			t.Fatal("replacement campaign loader missing")
		}
		if len(opts.MusicPlayerCatalog) == 0 || !strings.HasPrefix(opts.MusicPlayerCatalog[0].Key, "memory-wad/") {
			t.Fatal("replacement did not register portable content paths")
		}
		previousMemoryPath = opts.MusicPlayerCatalog[0].Key
		if raw, ok := wad.EmbeddedDataForPath(previousMemoryPath); !ok || !bytes.Equal(raw, baseBytes) {
			t.Fatal("replacement memory base identity changed")
		}
		if i == 0 {
			// The second preparation must use the first bundle's owned registry,
			// not a stale closure over the original local filesystem path.
			if err := os.Remove(basePath); err != nil {
				t.Fatal(err)
			}
		}
		if i == 1 {
			testAuthorityContentJoin(t, bundle, room.Manifest)
		}
		previousCleanup, current = opts.AuthorityContentCleanup, opts
	}
	previousCleanup()
	previousCleanup()
	if _, ok := wad.EmbeddedDataForPath(previousMemoryPath); ok {
		t.Fatal("final bundle cleanup retained registered bytes")
	}
	if f.downloads.Load() != 2 || f.catalogs.Load() != 2 {
		t.Fatalf("unexpected content traffic: catalogs=%d downloads=%d", f.catalogs.Load(), f.downloads.Load())
	}
}

func testAuthorityContentJoin(t *testing.T, bundle runtimecfg.AuthorityContentBundle, manifest netgame.CompatibilityManifest) {
	t.Helper()
	key, err := manifest.Key()
	if err != nil {
		t.Fatal(err)
	}
	contentKey, _ := manifest.ContentKey()
	authority, err := doomruntime.NewAuthority(bundle.Map, doomruntime.Options{GameMode: manifest.Mode, SkillLevel: manifest.Skill, NoMonsters: manifest.NoMonsters, WADHash: contentKey})
	if err != nil {
		t.Fatal(err)
	}
	world := &resumeAuditWorld{Authority: authority}
	match, err := netgame.NewMatch(world, world, netgame.MatchConfig{Epoch: 27, Compatibility: key, Manifest: &manifest, PlayerLimit: 2, InputLead: 3, FutureTicks: 35, HoldTicks: 2, DisconnectTicks: 350, SnapshotInterval: 2})
	if err != nil {
		t.Fatal(err)
	}
	server, err := netgame.NewServer(match)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	joined, err := bundle.Options.AuthorityJoin(ctx, runtimecfg.AuthorityJoinRequest{Address: listener.Addr().String(), Name: "Reloaded"})
	if err != nil {
		t.Fatal("replacement join callback rejected its new content:", err)
	}
	defer joinedClientLeave(t, joined.Client)()
	if !reflect.DeepEqual(joined.Manifest, manifest) || joined.Map.Name != bundle.Map.Name {
		t.Fatal("join callback captured another content manifest")
	}
	next := manifest
	next.Map = "E1M3"
	nextKey, _ := next.Key()
	if nextMap, err := joined.MapLoader(netgame.MapChange{Map: next.Map, Compatibility: nextKey}); err != nil || string(nextMap.Name) != next.Map {
		t.Fatal("replacement join map loader failed:", err)
	}
}
