package lobby

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"gddoom/internal/netgame"
)

// The game manifest's JSON representation participates in its compatibility
// hash. Keep the lobby's snake_case wire representation separate from it.
type manifestJSON struct {
	Simulation       string   `json:"simulation"`
	WADHashes        []string `json:"wad_hashes"`
	Map              string   `json:"map"`
	Mode             string   `json:"mode"`
	Skill            int      `json:"skill"`
	NoMonsters       bool     `json:"no_monsters"`
	FastMonsters     bool     `json:"fast_monsters"`
	RespawnMonsters  bool     `json:"respawn_monsters"`
	FriendlyFire     bool     `json:"friendly_fire"`
	RespawnDelayTics uint32   `json:"respawn_delay_tics"`
	FragLimit        int      `json:"frag_limit"`
	TimeLimitTics    uint32   `json:"time_limit_tics"`
}

type roomJSON Room

func (room Room) MarshalJSON() ([]byte, error) {
	m := room.Manifest
	return json.Marshal(struct {
		roomJSON
		Manifest manifestJSON `json:"manifest"`
	}{roomJSON(room), manifestJSON{
		Simulation: m.Simulation, WADHashes: m.WADHashes, Map: m.Map, Mode: m.Mode, Skill: m.Skill,
		NoMonsters: m.NoMonsters, FastMonsters: m.FastMonsters, RespawnMonsters: m.RespawnMonsters,
		FriendlyFire: m.FriendlyFire, RespawnDelayTics: m.RespawnDelayTics, FragLimit: m.FragLimit, TimeLimitTics: m.TimeLimitTics,
	}})
}

func (room *Room) UnmarshalJSON(data []byte) error {
	var wire struct {
		roomJSON
		Manifest manifestJSON `json:"manifest"`
	}
	if err := decodeStrict(data, &wire); err != nil {
		return err
	}
	m := wire.Manifest
	result := Room(wire.roomJSON)
	result.Manifest = netgame.CompatibilityManifest{
		Simulation: m.Simulation, WADHashes: m.WADHashes, Map: m.Map, Mode: m.Mode, Skill: m.Skill,
		NoMonsters: m.NoMonsters, FastMonsters: m.FastMonsters, RespawnMonsters: m.RespawnMonsters,
		FriendlyFire: m.FriendlyFire, RespawnDelayTics: m.RespawnDelayTics, FragLimit: m.FragLimit, TimeLimitTics: m.TimeLimitTics,
	}
	*room = result
	return nil
}

func decodeStrict(data []byte, target any) error {
	if !utf8.Valid(data) {
		return errors.New("lobby JSON must be valid UTF-8")
	}
	if err := uniqueJSONKeys(json.NewDecoder(bytes.NewReader(data)), 0); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("lobby JSON must contain exactly one value")
	}
	return nil
}

// Reject ambiguous duplicate keys and excessive nesting before decoding a
// typed response. Unknown fields are rejected separately by DisallowUnknownFields.
func uniqueJSONKeys(decoder *json.Decoder, depth int) error {
	if depth > 32 {
		return errors.New("lobby JSON nesting exceeds limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]bool)
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			key = strings.ToLower(key)
			if !ok || seen[key] {
				return errors.New("lobby JSON contains a duplicate object field")
			}
			seen[key] = true
			if err := uniqueJSONKeys(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := uniqueJSONKeys(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid lobby JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}
