package launchcatalog

import (
	"gddoom/internal/mapdata"
	"gddoom/internal/netplay"
	"gddoom/internal/runtimecfg"
)

func BroadcastSessionConfig(name mapdata.MapName, opts runtimecfg.Options) netplay.SessionConfig {
	return netplay.SessionConfig{WADHash: opts.WADHash, MapName: string(name), PlayerSlot: opts.PlayerSlot, SkillLevel: opts.SkillLevel, GameMode: opts.GameMode, ShowNoSkillItems: opts.ShowNoSkillItems, ShowAllItems: opts.ShowAllItems, FastMonsters: opts.FastMonsters, RespawnMonsters: opts.RespawnMonsters, NoMonsters: opts.NoMonsters, AutoWeaponSwitch: opts.AutoWeaponSwitch, CheatLevel: opts.CheatLevel, Invulnerable: opts.Invulnerable, SourcePortMode: opts.SourcePortMode}
}

func ApplyWatchSession(opts *runtimecfg.Options, s netplay.SessionConfig) {
	opts.PlayerSlot, opts.SkillLevel, opts.GameMode = s.PlayerSlot, s.SkillLevel, s.GameMode
	opts.ShowNoSkillItems, opts.ShowAllItems = s.ShowNoSkillItems, s.ShowAllItems
	opts.FastMonsters, opts.RespawnMonsters, opts.NoMonsters = s.FastMonsters, s.RespawnMonsters, s.NoMonsters
	opts.AutoWeaponSwitch, opts.CheatLevel, opts.Invulnerable = s.AutoWeaponSwitch, s.CheatLevel, s.Invulnerable
}
