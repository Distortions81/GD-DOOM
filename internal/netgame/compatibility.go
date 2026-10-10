package netgame

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"unicode/utf8"
)

// SimulationVersion changes whenever authority physics, rules, or baseline
// interpretation changes incompatibly. Transport framing has its own version.
const SimulationVersion = "gd-doom-authority-dev-3"

// CompatibilityManifest names content by digest, never by local filesystem path.
// WADHashes is ordered: changing add-on load order changes the match identity.
type CompatibilityManifest struct {
	Simulation       string
	WADHashes        []string
	Map              string
	Mode             string
	Skill            int
	NoMonsters       bool
	FastMonsters     bool
	RespawnMonsters  bool
	FriendlyFire     bool
	RespawnDelayTics uint32
	FragLimit        int
	TimeLimitTics    uint32
}

func (m CompatibilityManifest) Key() (string, error) {
	if len(m.Simulation) == 0 || len(m.Simulation) > 128 || !utf8.ValidString(m.Simulation) || !validManifestMapName(m.Map) ||
		(m.Mode != "coop" && m.Mode != "deathmatch") || m.Skill < 1 || m.Skill > 5 ||
		len(m.WADHashes) == 0 || len(m.WADHashes) > 64 || m.FragLimit < 0 || m.FragLimit > math.MaxInt32 {
		return "", fmt.Errorf("invalid multiplayer compatibility manifest")
	}
	for _, h := range m.WADHashes {
		b, err := hex.DecodeString(h)
		if err != nil || len(b) != sha256.Size {
			return "", fmt.Errorf("invalid content SHA-256 digest")
		}
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func validManifestMapName(name string) bool {
	if len(name) == 0 || len(name) > 32 {
		return false
	}
	for _, c := range name {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

// ContentKey identifies the simulation and ordered resources independently of
// the current map. Runtime snapshots also contain their own map digest, while
// the session's full Key verifies map and rules during each epoch handshake.
func (m CompatibilityManifest) ContentKey() (string, error) {
	if _, err := m.Key(); err != nil {
		return "", err
	}
	data, err := json.Marshal(struct {
		Simulation string
		WADHashes  []string
	}{m.Simulation, m.WADHashes})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
