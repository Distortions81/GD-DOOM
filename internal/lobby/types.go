// Package lobby defines the HTTP control plane for discovering and creating
// isolated authoritative matches. Gameplay continues to use netgame transports.
package lobby

import (
	"time"

	"gddoom/internal/netgame"
)

const (
	APIVersion       = 1
	MaxPacks         = 32
	MaxMaps          = 1000
	MaxRooms         = 32
	MaxNameLength    = 64 // Unicode code points; control characters are forbidden.
	MaxRequestBytes  = 8 << 10
	MaxResponseBytes = 2 << 20
)

type State struct {
	Version        int    `json:"version"`
	Packs          []Pack `json:"packs"`
	Rooms          []Room `json:"rooms"`
	MaxRooms       int    `json:"max_rooms"`
	UploadsEnabled bool   `json:"uploads_enabled"`
}

type Pack struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	WADHashes []string   `json:"wad_hashes"`
	Maps      []string   `json:"maps"`
	Files     []PackFile `json:"files,omitempty"`
}

// PackFile describes one entry in the exact ordered WAD stack. Downloadable
// means the operator has approved redistribution through this lobby; clients
// never interpret Name, License, or Source as a download location.
type PackFile struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	Downloadable bool   `json:"downloadable"`
	License      string `json:"license,omitempty"`
	Source       string `json:"source,omitempty"`
}

type Settings struct {
	PackID           string `json:"pack_id"`
	Map              string `json:"map"`
	Mode             string `json:"mode"`
	Skill            int    `json:"skill"`
	PlayerLimit      int    `json:"player_limit"`
	NoMonsters       bool   `json:"no_monsters"`
	FastMonsters     bool   `json:"fast_monsters"`
	RespawnMonsters  bool   `json:"respawn_monsters"`
	FriendlyFire     bool   `json:"friendly_fire"`
	FragLimit        int    `json:"frag_limit"`
	TimeLimitSeconds int    `json:"time_limit_seconds"`
}

type CreateRequest struct {
	RequestID string   `json:"request_id"`
	Name      string   `json:"name"`
	Settings  Settings `json:"settings"`
}

type Room struct {
	ID              string                        `json:"id"`
	Name            string                        `json:"name"`
	Address         string                        `json:"address"`
	State           string                        `json:"state"`
	Settings        Settings                      `json:"settings"`
	Manifest        netgame.CompatibilityManifest `json:"manifest"`
	Players         int                           `json:"players"`
	PlayerLimit     int                           `json:"player_limit"`
	Spectators      int                           `json:"spectators"`
	ReservedPlayers int                           `json:"reserved_players"`
	CreatedAt       time.Time                     `json:"created_at"`
}
