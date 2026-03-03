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

By default it starts in automap mode (`-start-in-map=true`), and `TAB` toggles walk/map.

## Key Flags

- `-wad <path>`: IWAD path
- `-config <path>`: TOML config file path (defaults to `config.toml` if present)
- `-map <E#M#|MAP##>`: map to load
- `-render=false`: parse/validate only (CLI summary output)
- `-details`: include extra parsed details in CLI output
- `-width <px>` / `-height <px>`: window size
- `-zoom <float>`: startup zoom override (`>0`); default uses Doom-style auto zoom
- `-player <1-4>`: choose local player start slot
- `-sourceport-mode`: enable source-port style automap extras at startup
- `-all-cheats`: start with automap cheats enabled (`allmap` + `IDDT2`)
- `-start-in-map`: start with automap open (default `true`)
- `-line-color-mode <parity|doom>`: automap line coloring mode
- `-import-pcspeaker`: import startup sound lumps (`DP*` and `DS*`) and print decode status

Level progression:
- Exit linedefs now transition to the next map in-sequence.
- Secret exits follow Doom targets when present (with WAD-order fallback).
- Level changes are handled in-session (no GLFW/Ebiten full reboot between maps).

## Controls (Default Doom Profile)

- `TAB`: toggle walk/map mode
- `WASD`: move
- `Q/E`: turn (map mode)
- `Shift`: run
- `E` / `Space`: use
- `Arrow keys`: pan map (follow off)
- `F`: toggle follow
- `G`: toggle grid
- `0`: big map toggle
- `M`: add mark
- `C`: clear marks
- `+` / `-` / mouse wheel: zoom
- `F1`: help overlay
- `Esc`: quit

Source-port extras are enabled only with `-sourceport-mode`.
In sourceport mode, thing legend overlay is enabled by default; press `V` to toggle it.
In sourceport mode, the current `use` target line is highlighted on automap.
In sourceport mode, use-trigger button/switch lines are drawn in a distinct overlay color.

Config notes:
- `config.toml` is auto-read by default when present.
- CLI flags always override config values.

## Runtime Gameplay State (Current)

- Item pickup is active for core Doom pickup classes (keys, health, armor, ammo, backpack, weapons).
- Hazardous floor sectors now apply periodic damage (Doom-style timed ticks); radiation suit pickup is supported.
- Player death state is tracked (`YOU DIED` overlay) when health reaches `0`.
- Damage/pickup screen flashes are active (red for damage, amber for pickups).
- Collected pickups are removed from automap thing rendering.
- Locked doors now check collected key inventory.
- Source-port info line shows tracked player stats (`hp`, `armor`, ammo pools, keyring).

## Project Docs

- Implemented features: `docs/IMPLEMENTED.md`
- Active plan/todo: `docs/PLAN-TODO.md`
- Automap parity checklist: `docs/automap-parity-notes.md`
- Historical milestone specs (archive):
  - `docs/archive/m1-parser-spec.md`
  - `docs/archive/m2-automap-spec.md`
