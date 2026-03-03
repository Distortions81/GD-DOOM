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

This starts the app in walk mode. Press `TAB` to open automap.

## Key Flags

- `-wad <path>`: IWAD path
- `-map <E#M#|MAP##>`: map to load
- `-render=false`: parse/validate only (CLI summary output)
- `-details`: include extra parsed details in CLI output
- `-width <px>` / `-height <px>`: window size
- `-zoom <float>`: startup zoom override (`>0`); default uses Doom-style auto zoom
- `-sourceport-mode`: enable source-port style automap extras at startup
- `-line-color-mode <parity|doom>`: automap line coloring mode

## Controls (Default Doom Profile)

- `TAB`: toggle walk/map mode
- `WASD`: move
- `Q/E`: turn (map mode)
- `Shift`: run
- `Space`: use
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

## Project Docs

- Implemented features: `docs/IMPLEMENTED.md`
- Active plan/todo: `docs/PLAN-TODO.md`
- Automap parity checklist: `docs/automap-parity-notes.md`
- Historical milestone specs (archive):
  - `docs/archive/m1-parser-spec.md`
  - `docs/archive/m2-automap-spec.md`
