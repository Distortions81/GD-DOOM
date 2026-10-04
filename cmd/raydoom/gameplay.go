//go:build raylib && cgo && !js

package main

import (
	"flag"
	"fmt"

	"gddoom/internal/doomruntime"
)

type nativeGameplayFlags struct {
	player, cheatLevel                                            *int
	textureCrossfade                                              *int
	showNoSkillItems, showAllItems, allCheats, noFPS, debugEvents *bool
}

func registerNativeGameplayFlags(fs *flag.FlagSet) nativeGameplayFlags {
	return nativeGameplayFlags{
		player:           fs.Int("player", 1, "player start slot (1 through 4)"),
		textureCrossfade: fs.Int("texture-anim-crossfade-frames", 7, "source-port texture and switch crossfade frames (0 disables)"),
		cheatLevel:       fs.Int("cheat-level", 0, "startup cheats: 0 off, 1 automap, 2 IDFA, 3 IDKFA and invulnerability"),
		showNoSkillItems: fs.Bool("show-no-skill-items", false, "show pickup items with no skill bits set"),
		showAllItems:     fs.Bool("show-all-items", false, "show pickup items regardless of skill/game-mode spawn filters"),
		allCheats:        fs.Bool("all-cheats", false, "legacy alias for startup full cheats and invulnerability"),
		noFPS:            fs.Bool("nofps", false, "hide the FPS/MS and tic counter overlay"),
		debugEvents:      fs.Bool("debug-events", false, "log gameplay events such as teleports and restarts"),
	}
}

// Keep the experiment's existing flag names and accept the main launcher's
// names too. Both names bind the same value, including explicit false/zero.
func registerNativeFlagAliases(fs *flag.FlagSet) {
	for alias, original := range nativeFlagAliases {
		if f := fs.Lookup(original); f != nil {
			fs.Var(f.Value, alias, f.Usage)
		}
	}
}

var nativeFlagAliases = map[string]string{
	"no-monsters": "nomonsters", "invuln": "god",
	"mouselook-speed": "mouse-speed", "keyboard-turn-speed": "keyboard-speed",
}

func (f nativeGameplayFlags) Apply(opts *doomruntime.Options) error {
	if *f.player < 1 || *f.player > 4 {
		return fmt.Errorf("-player must be between 1 and 4")
	}
	if *f.cheatLevel < 0 || *f.cheatLevel > 3 {
		return fmt.Errorf("-cheat-level must be between 0 and 3")
	}
	opts.PlayerSlot, opts.CheatLevel = *f.player, *f.cheatLevel
	opts.TextureAnimCrossfadeFrames = *f.textureCrossfade
	opts.ShowNoSkillItems, opts.ShowAllItems = *f.showNoSkillItems, *f.showAllItems
	opts.AllCheats, opts.NoFPS, opts.DebugEvents = *f.allCheats, *f.noFPS, *f.debugEvents
	opts.GameMode, opts.SourcePortMode = "single", true
	return nil
}
