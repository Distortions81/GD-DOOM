package runtimecfg

import "gddoom/internal/demo"

func PrepareDemoPlaybackOptions(opts Options, script *demo.Script) Options {
	if script == nil {
		return opts
	}

	opts.SkillLevel = int(script.Header.Skill) + 1
	opts.FastMonsters = script.Header.Fast
	opts.RespawnMonsters = script.Header.Respawn
	opts.NoMonsters = script.Header.NoMonsters
	opts.GameMode = demoPlaybackGameMode(script)
	opts.PlayerSlot = demoPlaybackPlayerSlot(script)

	// Vanilla demo playback ignores launcher-side gameplay mutators.
	opts.ShowNoSkillItems = false
	opts.ShowAllItems = false
	opts.CheatLevel = 0
	opts.Invulnerable = false
	opts.AllCheats = false

	return opts
}

func demoPlaybackGameMode(script *demo.Script) string {
	if script == nil {
		return "single"
	}
	if script.Header.Deathmatch {
		return "deathmatch"
	}
	activePlayers := 0
	for _, on := range script.Header.PlayerInGame {
		if on {
			activePlayers++
		}
	}
	if activePlayers > 1 {
		return "coop"
	}
	return "single"
}

func demoPlaybackPlayerSlot(script *demo.Script) int {
	if script == nil {
		return 1
	}
	playerSlot := int(script.Header.ConsolePlayer) + 1
	if playerSlot < 1 || playerSlot > 4 || !script.Header.PlayerInGame[playerSlot-1] {
		playerSlot = 1
	}
	return playerSlot
}
