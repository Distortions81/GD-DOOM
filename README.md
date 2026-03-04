# GD-DOOM

Minimal Doom map/parser + automap project in Go.

## What It Does

- Loads an IWAD (`IWAD` only) and parses Doom map lumps.
- Validates map data with strict checks.
- Launches an Ebiten desktop automap renderer.
- Supports Doom-style defaults plus optional source-port controls.

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
- `-mouselook`: enable mouse-based turning in walk mode (default `true`)
- `-mouselook-speed <float>`: mouse turn speed multiplier (`>0`, default `1.0`)
- `-keyboard-turn-speed <float>`: keyboard turn speed multiplier (`>0`, default `1.0`)
- `-always-run`: start with always-run enabled (holding `Shift` temporarily inverts it)
- `-auto-weapon-switch`: auto-switch to newly picked weapons (default `true`)
- `-cheat-level <0-3>`: startup cheats (`0=off`, `1=automap reveal`, `2=IDFA-like`, `3=IDKFA + invuln`)
- `-invuln`: start with invulnerability (`IDDQD`-like)
- `-sourceport-mode`: enable source-port style automap extras at startup
- `-kage-shader`: enable postprocess shader chain (LUT/gamma/CRT)
- `-gpu-sky`: enable experimental GPU sky path in sourceport mode (default `false`; CPU sky is default)
- `-crt-effect`: enable CRT pass (applied last in shader chain)
- `-depth-buffer-view`: show grayscale depth view instead of 3D scene
- `-texture-anim-crossfade-frames`: sourceport animation blend frames (`0` disables, max effective `7` at Doom's 8-tic cadence)
- `-all-cheats`: legacy alias for full cheats (`-cheat-level=3 -invuln=true`)
- `-start-in-map`: start with automap open (default `false`)
- `-line-color-mode <parity|doom>`: automap line coloring mode
- `-import-pcspeaker`: import startup sound lumps (`DP*` and `DS*`) and print decode status
- `-import-textures`: parse `PLAYPAL`/`PNAMES`/`TEXTURE1/2` and build texture tables for Ebiten use
- `-map-floor-tex-2d`: sourceport map mode: draw floor flat textures in 2D automap (defaults to `true` when `-sourceport-mode` is enabled)
- `-demo <path>`: run a scripted `gddoom-demo-v1` input stream for benchmarking and exit when done
- `-record-demo <path>`: record live input each game tic to `gddoom-demo-v1` and write on exit
- `-cpuprofile <path>`: write Go CPU profile
- `-no-vsync`: disable vsync and uncap draw FPS
- `-nofps`: hide FPS/MS overlay

Demo format (`gddoom-demo-v1`):

```text
gddoom-demo-v1
# forward side turn turn_raw run use fire
25 0 0 0 0 0 0
25 0 -1 0 0 0 0
0 0 0 0 0 1 0
0 0 0 0 0 0 1
```

- One tic per line after the header.
- `run/use/fire` are bits (`0` or `1`).
- At completion, the app prints `demo-bench ...` timing stats and exits.
  It includes `wad` (WAD SHA-1 hash), `map`, and `rng_start` (`M_Random`/`P_Random` indices at demo start).
- `-demo` and `-record-demo` are mutually exclusive.

Level progression:
- Exit linedefs now transition to the next map in-sequence.
- Secret exits follow Doom targets when present (with WAD-order fallback).
- Level changes are handled in-session (no GLFW/Ebiten full reboot between maps).

## Controls (Default Doom Profile)

- `TAB`: toggle walk/map mode
- `WASD`: move
- `Q/E`: turn (map mode)
- `Shift`: run
- `CapsLock`: toggle always-run
- `E` / `Space`: use
- `Ctrl` / left mouse: fire (hitscan prototype)
- `1..7`: weapon slot select
- `[` / `]` or `PgUp` / `PgDn`: previous/next weapon (walk mode)
- mouse wheel (walk mode): cycle weapons
- `F12`: toggle auto weapon-switch
- `Arrow keys`: pan map (follow off)
- `F`: toggle follow
- `G`: toggle grid
- `0`: big map toggle
- `M`: add mark
- `C`: clear marks
- `+` / `-` / mouse wheel: zoom
- `F1`: help overlay
- `F2`: save (menu WIP)
- `F3`: load (menu WIP)
- `F5`: cycle detail level (faithful) / clean upscale ratio (sourceport)
- `F6`: quicksave (WIP)
- `F7`: end game flow (WIP)
- `F8`: toggle HUD messages
- `F9`: quickload (WIP)
- `F10`: quit
- `F11`: gamma (faithful mode; requires `-kage-shader`)
- `Esc`: quit

Cheat controls are currently startup-config driven (`-cheat-level`, `-invuln`, `-all-cheats`).

Source-port extras are enabled only with `-sourceport-mode`.
In sourceport mode, press `\` to toggle mouselook at runtime.
In sourceport mode, thing legend overlay is enabled by default; press `V` to toggle it.
In sourceport mode, the current `use` target line is highlighted on automap.
In sourceport mode, use-trigger button/switch lines are drawn in a distinct overlay color.
In sourceport mode, legend panel includes map line-color meanings.
In sourceport mode, walk view defaults to `doom-basic` textured wall rendering; press `P` to toggle pseudo-3D.
In sourceport mode, map floor flats are drawn in 2D automap by default; press `J` to toggle at runtime.

Config notes:
- `config.toml` is auto-read by default when present.
- CLI flags always override config values.
- Runtime setting changes are auto-saved to the active config path (`-config`, default `config.toml`) for persisted keys like detail/gamma and key gameplay toggles.

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
- Kage postprocess is opt-in (`-kage-shader`); default faithful startup path runs without post shaders.
- Sourceport GPU sky is currently experimental and opt-in (`-gpu-sky`); default path uses CPU sky rendering.

## Project Docs

- Implemented features: `docs/IMPLEMENTED.md`
- Active plan/todo: `docs/plans/PLAN-TODO.md`
- Render mode policy: `docs/render-modes.md`
- Automap parity checklist: `docs/automap-parity-notes.md`
- Historical milestone specs (archive):
  - `docs/archive/m1-parser-spec.md`
  - `docs/archive/m2-automap-spec.md`
