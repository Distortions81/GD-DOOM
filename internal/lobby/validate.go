package lobby

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"gddoom/internal/netgame"
)

func NewRequestID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

// NormalizeAddress accepts a lobby base URL, optionally below a path prefix.
// Room gameplay URLs are separate and are never interpreted as lobby URLs.
func NormalizeAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	u, err := checkedURL(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("lobby address must be an HTTP or HTTPS URL without credentials, query, or fragment")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path != "" && path.Clean(u.Path) != u.Path {
		return "", errors.New("lobby address must have a clean path")
	}
	return u.String(), nil
}

func checkedURL(address string) (*url.URL, error) {
	if len(address) == 0 || len(address) > 512 || !utf8.ValidString(address) || strings.IndexFunc(address, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0 || strings.ContainsAny(address, "#\\") {
		return nil, errors.New("invalid URL")
	}
	u, err := url.Parse(address)
	if err != nil || u.Hostname() == "" || strings.HasSuffix(u.Host, ":") || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" {
		return nil, errors.New("invalid URL")
	}
	if p := u.Port(); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil || port < 1 || port > 65535 {
			return nil, errors.New("invalid URL port")
		}
	}
	return u, nil
}

func validID(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, c := range value {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func validName(value string) bool {
	return utf8.ValidString(value) && strings.TrimSpace(value) == value && value != "" && utf8.RuneCountInString(value) <= MaxNameLength && strings.IndexFunc(value, unicode.IsControl) < 0
}

func validMap(value string) bool {
	if len(value) == 0 || len(value) > 32 {
		return false
	}
	for _, c := range value {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func ValidatePack(pack Pack) error {
	if !validID(pack.ID) || !validName(pack.Name) || len(pack.WADHashes) == 0 || len(pack.WADHashes) > 64 || len(pack.Maps) == 0 || len(pack.Maps) > MaxMaps {
		return errors.New("invalid lobby content pack")
	}
	for _, hash := range pack.WADHashes {
		data, err := hex.DecodeString(hash)
		if err != nil || len(data) != 32 || hash != strings.ToLower(hash) {
			return errors.New("content pack requires ordered lowercase SHA-256 hashes")
		}
	}
	if len(pack.Files) != 0 {
		if len(pack.Files) != len(pack.WADHashes) {
			return errors.New("content file metadata must describe the complete ordered WAD stack")
		}
		for i, file := range pack.Files {
			if err := validatePackFile(file); err != nil {
				return err
			}
			if file.SHA256 != pack.WADHashes[i] {
				return errors.New("content file metadata does not match the ordered WAD hashes")
			}
		}
	}
	seen := make(map[string]bool, len(pack.Maps))
	for _, name := range pack.Maps {
		if !validMap(name) || seen[name] {
			return errors.New("content pack contains an invalid or duplicate map")
		}
		seen[name] = true
	}
	return nil
}

func validateSettingsShape(settings Settings) error {
	if !validID(settings.PackID) || !validMap(settings.Map) || (settings.Mode != "coop" && settings.Mode != "deathmatch") || settings.Skill < 1 || settings.Skill > 5 || settings.PlayerLimit < 1 || settings.PlayerLimit > netgame.MaxPlayers || settings.FragLimit < 0 || uint64(settings.FragLimit) > math.MaxInt32 || settings.TimeLimitSeconds < 0 || uint64(settings.TimeLimitSeconds) > math.MaxUint32/netgame.TickRate {
		return errors.New("invalid lobby match settings")
	}
	return nil
}

// ValidateSettings binds an allowed map and immutable ordered content identity
// to the same manifest that gdserver must advertise before a room is ready.
func ValidateSettings(settings Settings, pack Pack) (netgame.CompatibilityManifest, error) {
	if err := ValidatePack(pack); err != nil {
		return netgame.CompatibilityManifest{}, err
	}
	if err := validateSettingsShape(settings); err != nil {
		return netgame.CompatibilityManifest{}, err
	}
	if settings.PackID != pack.ID || !slices.Contains(pack.Maps, settings.Map) {
		return netgame.CompatibilityManifest{}, errors.New("match requires a map from the selected content pack")
	}
	manifest := netgame.CompatibilityManifest{
		Simulation: netgame.SimulationVersion, WADHashes: slices.Clone(pack.WADHashes), Map: settings.Map,
		Mode: settings.Mode, Skill: settings.Skill, NoMonsters: settings.NoMonsters,
		FastMonsters: settings.FastMonsters, RespawnMonsters: settings.RespawnMonsters, FriendlyFire: settings.FriendlyFire,
		RespawnDelayTics: netgame.TickRate, FragLimit: settings.FragLimit, TimeLimitTics: uint32(settings.TimeLimitSeconds) * netgame.TickRate,
	}
	if _, err := manifest.Key(); err != nil {
		return netgame.CompatibilityManifest{}, err
	}
	return manifest, nil
}

func ValidateCreateRequest(request CreateRequest) error {
	id, err := hex.DecodeString(request.RequestID)
	if err != nil || len(id) != 16 || request.RequestID != strings.ToLower(request.RequestID) || !validName(request.Name) {
		return errors.New("creation requires a 32-character lowercase hexadecimal request ID and a name of 1–64 characters")
	}
	return validateSettingsShape(request.Settings)
}

func validateRoom(room Room, pack *Pack) error {
	if !validID(room.ID) || !validName(room.Name) || room.CreatedAt.IsZero() || room.CreatedAt.Year() < 1 || room.CreatedAt.Year() > 9999 || room.PlayerLimit != room.Settings.PlayerLimit || room.Players < 0 || room.Players > room.PlayerLimit || room.ReservedPlayers < 0 || room.ReservedPlayers > room.Players || room.Spectators < 0 || room.Spectators > netgame.MaxSpectators {
		return errors.New("invalid lobby room")
	}
	switch room.State {
	case "starting", "ready", "stopping", "ended", "failed":
	default:
		return errors.New("invalid lobby room state")
	}
	if room.Address == "" {
		if room.State == "ready" {
			return errors.New("ready lobby room has no gameplay address")
		}
	} else {
		u, err := checkedURL(room.Address)
		if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Path == "" || path.Clean(u.Path) != u.Path {
			return errors.New("invalid lobby room WebSocket address")
		}
	}
	if pack == nil {
		// A create response has no catalog alongside it. Still verify that its
		// advertised manifest agrees with all settings and has valid identities.
		pack = &Pack{ID: room.Settings.PackID, Name: "Room content", WADHashes: room.Manifest.WADHashes, Maps: []string{room.Settings.Map}}
		if room.Manifest.Map != room.Settings.Map {
			pack.Maps = append(pack.Maps, room.Manifest.Map)
		}
	}
	expected, err := ValidateSettings(room.Settings, *pack)
	if err != nil {
		return err
	}
	if !slices.Contains(pack.Maps, room.Manifest.Map) {
		return errors.New("lobby room is running an unknown content pack map")
	}
	// Settings retain the creation request; normal map progression changes only
	// the live manifest's map, not the room's initial configuration.
	expected.Map = room.Manifest.Map
	// Other engine versions remain visible as incompatible rooms. The gameplay
	// join path separately checks the local engine and loaded WAD hashes.
	expected.Simulation = room.Manifest.Simulation
	want, err := expected.Key()
	got, gotErr := room.Manifest.Key()
	if err != nil || gotErr != nil || want != got {
		return errors.New("lobby room manifest does not match its settings and content pack")
	}
	return nil
}

func validateState(state State) error {
	if state.Version != APIVersion || state.MaxRooms < 1 || state.MaxRooms > MaxRooms || len(state.Packs) > MaxPacks || len(state.Rooms) > MaxRooms {
		return errors.New("invalid or unsupported lobby state")
	}
	packs := make(map[string]Pack, len(state.Packs))
	for _, pack := range state.Packs {
		if err := ValidatePack(pack); err != nil {
			return err
		}
		if _, exists := packs[pack.ID]; exists {
			return errors.New("duplicate lobby content pack ID")
		}
		packs[pack.ID] = pack
	}
	rooms := make(map[string]bool, len(state.Rooms))
	for _, room := range state.Rooms {
		pack, exists := packs[room.Settings.PackID]
		if !exists || rooms[room.ID] {
			return errors.New("unknown room content pack or duplicate room ID")
		}
		if err := validateRoom(room, &pack); err != nil {
			return fmt.Errorf("room %s: %w", room.ID, err)
		}
		rooms[room.ID] = true
	}
	return nil
}
