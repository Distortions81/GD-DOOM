# GD-DOOM

<p align="center">
  <img src="e1m1.png" alt="E1M1 screenshot" width="900">
</p>

Minimal Doom runtime, map parser, and renderer in Go.

## What It Does

- Loads an IWAD (`IWAD` only) and parses Doom map lumps.
- Validates map data with strict checks.
- Launches an Ebiten desktop Doom runtime with walk view, automap, and in-session frontend screens.
- Supports faithful Doom defaults plus optional source-port conveniences.
- Can play, record, and trace Doom v1.10 demos.

## Quick Start

Requirements:
- Go 1.22+
- A Doom IWAD (example in this repo: `DOOM1.WAD`)

Run:

```bash
go run ./cmd/gddoom -wad DOOM1.WAD
```

By default it starts in walk mode (`-start-in-map=false`), and `TAB` toggles walk/map.

## Key Flags

For the exact current CLI, run:

```bash
go run ./cmd/gddoom -h
```

- `-wad <path>`: IWAD path
- `-config <path>`: TOML config file path (defaults to `config.toml` if present)
- `-map <E#M#|MAP##>`: map to load
- `-render=false`: parse/validate only (CLI summary output)
- `-details`: include extra parsed details in CLI output
- `-width <px>` / `-height <px>`: window size
- `-zoom <float>`: startup zoom override (`>0`); default uses Doom-style auto zoom
- `-detail-level <int>`: startup detail level (`-1` keeps mode default)
- `-gamma-level <int>`: startup gamma level (`-1` keeps mode default)
- `-player <1-4>`: choose local player start slot
- `-skill <1-5>`: Doom skill level (`1=ITYTD`, `2=HNTR`, `3=HMP`, `4=UV`, `5=NM`)
- `-game-mode <single|coop|deathmatch>`: apply thing spawn filtering for the selected mode
- `-fastmonsters`: enable fast-monsters behavior
- `-show-no-skill-items`: show pickup items that have no skill bits set
- `-show-all-items`: show pickup items regardless of normal pickup spawn filters
- `-mouselook`: enable mouse-based turning in walk mode (default `true`)
- `-mouselook-speed <float>`: mouse turn speed multiplier (`>0`, default `0.5`)
- `-keyboard-turn-speed <float>`: keyboard turn speed multiplier (`>0`, default `1.0`)
- `-music-volume <float>`: music output volume (`0..1`, default `1.0`)
- `-mus-pan-max <float>`: maximum MUS pan amount (`0..1`, default `0.8`; lower pulls pan toward center)
- `-opl-volume <float>`: OPL synth output gain (`0..4`, default `2.25`)
- `-opl3-backend <auto|impsynth|nuked>`: choose the OPL3 backend
- `-opl-bank <path>`: override the WAD `GENMIDI` bank with an external OP2/GENMIDI file
- `-sfx-volume <float>`: sound-effect output volume (`0..1`, default `0.5`)
- `-always-run`: start with always-run enabled (holding `Shift` temporarily inverts it)
- `-auto-weapon-switch`: auto-switch to newly picked weapons (default `true`)
- `-cheat-level <0-3>`: startup cheats (`0=off`, `1=automap reveal`, `2=IDFA-like`, `3=IDKFA + invuln`)
- `-invuln`: start with invulnerability (`IDDQD`-like)
- `-all-cheats`: legacy alias for full cheats (`-cheat-level=3 -invuln=true`)
- `-line-color-mode <parity|doom>`: automap line coloring mode
- `-sourceport-mode`: enable source-port defaults and runtime conveniences
- `-sourceport-sector-lighting`: show classic sector lighting while in sourceport mode (default `true`)
- `-sourceport-thing-render-mode <glyphs|items|sprites>`: sourceport automap thing rendering mode
- `-sourceport-thing-blend-frames`: allow blended sub-tic thing sprite frames on the automap
- `-doom-lighting`: enable Doom lighting math and `COLORMAP` shading
- `-gpu-sky`: enable the experimental GPU sky path for sourceport mode
- `-sky-upscale <nearest|sharp>`: GPU sky upscale mode
- `-kage-shader`: enable Kage postprocess shaders
- `-crt-effect`: enable CRT pass
- `-texture-anim-crossfade-frames`: sourceport animation blend frames (`0` disables, max effective `7` at Doom's 8-tic cadence)
- `-start-in-map`: start with automap open (default `false`)
- `-import-pcspeaker`: import startup sound lumps (`DP*` and `DS*`) and print decode status
- `-import-textures`: parse Doom texture data and build tables for the software renderer
- `-demo <path>`: run a Doom v1.10 `.lmp` demo for benchmarking and exit when it completes
- `-record-demo <path>`: record live input to a Doom v1.10 `.lmp` demo
- `-trace-demo-state <path>`: write per-tic GD-DOOM demo state JSONL during `-demo` playback
- `-cpuprofile <path>`: write Go CPU profile
- `-memprofile <path>`: write Go heap profile on exit
- `-memstats`: log Go runtime memory stats at startup and exit
- `-no-vsync`: disable vsync and uncap draw FPS
- `-nofps`: hide FPS/MS overlay
- `-no-aspect-correction`: disable Doom-style 4:3 aspect correction

- `-demo` and `-record-demo` use Doom v1.10 `.lmp` files.
- At completion, `-demo` prints `demo-bench ...` timing stats and exits.
  It includes `wad` (WAD SHA-1 hash), `map`, and `rng_start` (`M_Random`/`P_Random` indices at demo start).
- `-demo` and `-record-demo` are mutually exclusive.
- `-trace-demo-state` requires `-demo`.

Level progression:
- Exit linedefs now transition to the next map in-sequence.
- Secret exits follow Doom targets when present (with WAD-order fallback).
- Level changes are handled in-session (no GLFW/Ebiten full reboot between maps).

## Controls (Default Doom Profile)

- `TAB`: toggle walk/map mode
- `Esc`: open/close the in-game pause menu
- `F4` / `F10`: open the quit prompt
- `F1`: open the Read This/help screen
- `WASD` or arrow keys: move/turn in walk mode (`Alt` + left/right arrow strafes)
- `Q/E`: turn in map mode
- `Shift`: run
- `CapsLock`: toggle always-run
- `E` / `Space`: use
- `Ctrl` / left mouse: fire (hitscan prototype)
- `1..7`: weapon slot select
- `[` / `]` or `PgUp` / `PgDn`: previous/next weapon (walk mode)
- `Ctrl` + `[` / `]`: HUD size down/up
- mouse wheel or side mouse buttons (walk mode): cycle weapons
- `F12`: toggle auto weapon-switch
- `Arrow keys`: pan map (follow off)
- `F`: toggle follow
- `0`: big map toggle
- `Home`: reset map view
- `M`: add mark
- `C`: clear marks
- `G`: toggle grid
- `+` / `-`: zoom
- `,` / `.` / `/`: game speed down/up/reset
- `F2`: save menu placeholder
- `F3`: load menu placeholder
- `F5`: cycle detail level
- `F6`: quicksave placeholder in faithful mode
- `F7`: end-game placeholder in faithful mode
- `F8`: toggle HUD messages in faithful mode
- `F9`: quickload placeholder
- `F11`: cycle Doom-style gamma correction
- `Enter`: restart the current level after death

Cheat controls are currently startup-config driven (`-cheat-level`, `-invuln`, `-all-cheats`).

Source-port extras are enabled only with `-sourceport-mode`.
In sourceport mode, press `\` to toggle mouselook at runtime.
In sourceport mode, press `R` to toggle heading-up automap while follow mode is enabled.
In sourceport mode, press `B` for big-map, `O` to toggle allmap, and `I` to cycle `IDDT` state.
In sourceport mode, press `L` to toggle line-color mode, `V` to toggle the thing legend, and `T` to cycle automap thing rendering.
In sourceport mode, the legend is enabled by default and thing rendering normalizes to `items` unless overridden with `-sourceport-thing-render-mode`.
In sourceport mode, if you do not explicitly set `-gpu-sky` or `-sky-upscale`, GD-DOOM defaults to GPU sky with `-sky-upscale=sharp`.

Config notes:
- `config.toml` is auto-read by default when present.
- CLI flags always override config values.
- Runtime setting changes are auto-saved to the active config path (`-config`, default `config.toml`) for persisted keys like detail/gamma, gameplay toggles, and audio volumes.

## Runtime Gameplay State (Current)

- Item pickup is active for core Doom pickup classes (keys, health, armor, ammo, backpack, weapons).
- Hazardous floor sectors now apply periodic damage (Doom-style timed ticks); radiation suit pickup is supported.
- Player death state is tracked (`YOU DIED` overlay) when health reaches `0`.
- On death, press `Enter` to restart the current level in-session.
- Damage/pickup screen flashes are active (red for damage, amber for pickups).
- Basic combat foundation is active: pistol-like hitscan, ammo consumption, monster HP/death removal.
- Monsters now have more Doom-like wake/chase/attack behavior (type-specific melee/ranged styles, randomized cooldown/chance).
- Collected pickups are removed from automap thing rendering.
- Locked doors now check collected key inventory.
- Doors now visibly slide open/closed in walk view (rendered door-ceiling motion tracks gameplay state).
- Source-port info line shows tracked player stats (`hp`, `armor`, ammo pools, keyring).
- 3D lighting now uses Doom `COLORMAP` behavior with fullbright sprite support.
- Two-sided masked mid textures render in a deferred masked pass (portal/grate style walls).
- In-session pause/options screens, Read This, and quit prompt frontend are wired.
- Demo playback, live demo recording, and JSONL demo-state tracing are available.
- Kage postprocess is opt-in (`-kage-shader`); CRT is the active runtime postprocess effect.
- Sourceport GPU sky remains experimental, but sourceport mode now defaults it on unless you explicitly disable or override sky settings.

## Project Docs

- Implemented feature snapshot: `docs/IMPLEMENTED.md`
- Launch flags and runtime defaults: `docs/launch-params.md`
- Action list: `docs/ACTIONS.md`
- Render mode policy: `docs/render-modes.md`
- Demo trace harness notes: `docs/doom-trace-harness-checklist.md`
- IWAD map-data audit: `docs/map-audit.md`
