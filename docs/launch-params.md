# Useful Launch Params

Simple end-user flags for running the game. This list intentionally skips debug and internal development options.

## Basic

- `-wad <path>`
  - Choose the IWAD file.
- `-map <name>`
  - Start on a specific map such as `E1M1` or `MAP01`.
- `-width <n>`
  - Set window width.
- `-height <n>`
  - Set window height.
- `-config <path>`
  - Use a specific `config.toml`.

Example:

```bash
go run ./cmd/gddoom -wad DOOM1.WAD -map E1M1 -width 1280 -height 720
```

## Gameplay

- `-player <1..4>`
  - Choose the player start slot.
- `-skill <1..5>`
  - Set Doom skill level.
- `-game-mode single|coop|deathmatch`
  - Apply thing spawn filtering for the chosen mode.
- `-fastmonsters`
  - Enable fast-monsters behavior.
- `-always-run`
  - Start with always-run enabled.
- `-auto-weapon-switch`
  - Start with auto weapon switch enabled.

Example:

```bash
go run ./cmd/gddoom -wad DOOM2.WAD -map MAP01 -skill 4 -fastmonsters
```

## Item Visibility Overrides

- `-show-no-skill-items`
  - Show pickup items that have no skill bits set.
- `-show-all-items`
  - Show pickup items regardless of skill and game-mode spawn filters.

These only affect pickup items. They do not force monsters to spawn.

## View and Control

- `-sourceport-mode`
  - Start with the source-port profile and convenience behavior enabled.
- `-mouselook=true|false`
  - Enable or disable mouse turning.
- `-mouselook-speed <n>`
  - Adjust mouse turn speed.
- `-keyboard-turn-speed <n>`
  - Adjust keyboard turn speed.
- `-start-in-map`
  - Start with the automap open.
- `-zoom <n>`
  - Override startup map zoom.

Example:

```bash
go run ./cmd/gddoom -wad DOOM1.WAD -sourceport-mode
```

## Audio

- `-music-volume <0..1>`
  - Set music volume.
- `-sfx-volume <0..1>`
  - Set sound-effect volume.
- `-mus-pan-max <0..1>`
  - Limit stereo panning for OPL3 music playback.
- `-opl-volume <0..4>`
  - Adjust OPL synth output gain.

## Visual Options

- `-detail-level <n>`
  - Set startup detail level.
- `-gamma-level <n>`
  - Set startup gamma level.
- `-sourceport-sector-lighting=true|false`
  - Toggle classic sector lighting while in source-port mode.
- `-doom-lighting=true|false`
  - Toggle Doom lighting math/colormap shading.
- `-crt-effect`
  - Enable CRT post-processing.
- `-gpu-sky`
  - Enable the experimental GPU sky path.
- `-no-aspect-correction`
  - Disable DOS aspect ratio correction.
- `-no-vsync`
  - Disable vsync.
- `-nofps`
  - Hide the FPS/MS overlay.

## Cheats and Convenience

- `-cheat-level <0..3>`
  - Start with automap/inventory cheat levels.
- `-invuln`
  - Start invulnerable.
- `-all-cheats`
  - Shortcut for full startup cheats.

## Config

Most of these flags also have matching `config.toml` keys. Command-line flags override config values.
