# GD-DOOM

<p align="center">
  "Modern / Source Port" mode depicted below
  <img src="e1m1.png" alt="E1M1 screenshot" width="900">
  <br>
  Play the shareware version: <a href="https://m45sci.xyz/u/dist/GD-DOOM">https://m45sci.xyz/u/dist/GD-DOOM</a>
  
  A "Faithful" mode that strives to be the same as the DOS version is available as well via menu selection.
</p>

GD-DOOM is a Go-based Doom runtime focused on loading original game data, preserving classic behavior where it matters, and providing a modern execution environment for experimentation, profiling, and feature work. What began as a small prototype is now a substantially larger codebase with gameplay, rendering, audio, frontend, and demo tooling.

The project sits between a faithful runtime and a source-port-style sandbox. It supports original WAD content, layered PWAD setups, in-session menus and frontend screens, walk and automap views, demo playback and recording, and a growing set of Doom gameplay systems. It is intended both as a playable runtime and as an engineering vehicle for understanding and extending Doom’s data formats and mechanics.

License: GD-DOOM is distributed under GNU GPL v2. It is inspired by, ported from, and derivative of id Software's DOOM source release. See [LICENSE](/home/dist/github/GD-DOOM/LICENSE) and [NOTICE](/home/dist/github/GD-DOOM/NOTICE).

## Status

GD-DOOM is still alpha. Core runtime systems are in place, but full parity remains in progress, especially around weapon completeness, monster edge cases, and deterministic demo compatibility.

## Purpose And Scope

GD-DOOM is not just a renderer or file parser. The project aims to cover the full path from WAD ingestion to interactive play:

- WAD loading for IWAD and PWAD stacks, including overlay behavior and strict map validation.
- Runtime gameplay systems such as pickups, doors, hazardous sectors, combat foundations, key progression, save/load, and level transitions.
- Multiple presentation modes, including a more faithful Doom-style profile and a more permissive source-port-oriented mode.
- Audio, music, frontend screens, automap, and session flow needed for a usable end-to-end runtime.
- Demo playback, live recording, tracing, and profiling workflows useful for regression work and performance analysis.

## Highlights

- Loads original Doom data directly, including stacked IWAD plus PWAD configurations.
- Runs as a desktop Ebiten application with walk view, automap, menus, help screens, and pause/quit flows.
- Supports faithful defaults alongside optional source-port conveniences such as mouselook and expanded automap behavior.
- Includes Doom v1.10 demo playback, demo recording, and JSONL state tracing for investigation and benchmarking.
- Provides profiling helpers and benchmark-oriented workflows for runtime and rendering work.

## Quick Start

Requirements:
- Go 1.22+
- A Doom base WAD (examples the runtime auto-detects in the repo root: `DOOM.WAD`, `DOOM2.WAD`, `TNT.WAD`, `PLUTONIA.WAD`, `DOOM1.WAD`)

Run:

```bash
go run ./cmd/gddoom -wad DOOM1.WAD
```

PWAD overlays:

```bash
go run ./cmd/gddoom -wad DOOM2.WAD -file mods/nerve.wad,mods/examplepatch.wad
```

By default it starts in walk mode (`-start-in-map=false`), and `TAB` toggles walk/map.
If `-map` is omitted, GD-DOOM starts on the first valid map it finds, preferring the last PWAD overlay in the stack when overlays are present.
If `-wad` is omitted and the working directory contains one local known IWAD, GD-DOOM uses it automatically; if multiple known IWADs are present during render startup, it opens an in-game picker.

For the current command-line interface, run:

```bash
go run ./cmd/gddoom -h
```

## Current Feature Coverage

- Item pickup is active for core Doom pickup classes, including keys, health, armor, ammo, backpack, and weapons.
- Hazardous floors, radiation suits, player death state, restart flow, and damage or pickup flashes are implemented.
- Combat foundations are present, including hitscan attacks, ammo consumption, monster damage and death, and a growing set of Doom-like monster behaviors.
- Doors, locked progression checks, level exits, secret-exit routing, and in-session map transitions are implemented.
- Automap and walk-view rendering include Doom-style lighting work, masked mid textures, player stats, and mode-specific presentation behavior.
- Save/load, in-session frontend screens, demo playback, live recording, and demo-state tracing are available.

## Tooling And Profiling

The repository includes helper scripts for extracting bundled demo data, running demo-driven profiles, and generating profile graphs:

```bash
scripts/extract_wad_demo.py
scripts/demo_profile.sh --mem
scripts/pprof_graphs.sh
```

The extractor saves `DEMO1` from `DOOM1.WAD` to `demos/DOOM1-DEMO1.lmp`, which is the default input for the profiling script. The graph helper renders SVG call graphs from the newest CPU and memory profiles in `./profiles`.
