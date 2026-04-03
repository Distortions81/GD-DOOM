# GD-DOOM

[![Go CI](https://github.com/Distortions81/GD-DOOM/actions/workflows/ci.yml/badge.svg)](https://github.com/Distortions81/GD-DOOM/actions/workflows/ci.yml)
[![Go Vulncheck](https://github.com/Distortions81/GD-DOOM/actions/workflows/govulncheck.yml/badge.svg)](https://github.com/Distortions81/GD-DOOM/actions/workflows/govulncheck.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Distortions81/GD-DOOM)](https://goreportcard.com/report/github.com/Distortions81/GD-DOOM)
[![License](https://img.shields.io/github/license/Distortions81/GD-DOOM)](https://github.com/Distortions81/GD-DOOM/blob/main/LICENSE)

<p align="center">
  <img src="e1m1.png" alt="E1M1 screenshot" width="900">
  <br>
  Source Port mode shown above.
  <br>
  Browser build: <a href="https://m45sci.xyz/u/dist/GD-DOOM">https://m45sci.xyz/u/dist/GD-DOOM</a>
</p>

GD-DOOM is a Go-based Doom runtime that plays original Doom data with two presentation modes:

- `Faithful` mode stays closer to DOS Doom behavior and presentation.
- `Source Port` mode enables higher-fidelity rendering, interpolation, smoother animation, and expanded music playback.

It loads original IWAD data, supports PWAD overlays, can play or record Doom v1.10 demos, and ships with native and browser build paths.

License: GD-DOOM is distributed under GNU GPL v2. It is inspired by, ported from, and derivative of id Software's DOOM source release. See [LICENSE](/home/dist/github/GD-DOOM/LICENSE) and [NOTICE](/home/dist/github/GD-DOOM/NOTICE).

## Highlights

### Rendering

- Two runtime modes: `Faithful` and `Source Port`.
- 32-bit RGBA output instead of vanilla Doom's indexed framebuffer presentation.
- Higher-resolution presentation for walls, sprites, HUD, and automap.
- Interpolated camera movement, yaw, and thing motion between 35 Hz simulation tics.
- Textured lit floors and ceilings, optional GPU sky path, and optional CRT postprocess effect.
- Texture animation crossfades, blended weapon psprite transitions, and broader multi-frame sprite presentation for pickups and decorations.
- Integrated automap with follow mode, rotate mode, big-map view, grid, and map marks.

### Audio

- `impsynth` backend for an OPL-style classic Doom feel.
- `meltysynth` backend for SoundFont-backed General MIDI playback.
- Separate music and SFX volume controls.
- Stereo MUS playback with adjustable pan width.
- In-game music menu and browser/player flow for WAD, episode, and map-organized tracks.

### Data And Runtime

- Direct IWAD loading with optional PWAD overlays.
- Automatic IWAD selection when one known base WAD is present.
- In-game IWAD picker when multiple known IWADs are available.
- Save/load support integrated into the runtime flow.
- Demo playback, demo recording, and demo trace export support.
- Config-file support on native builds through `config.toml`.

## Requirements

- Go `1.26.1` or newer
- A Doom IWAD such as `DOOM.WAD`, `DOOM2.WAD`, `TNT.WAD`, `PLUTONIA.WAD`, or `DOOM1.WAD`

On Linux, native builds also need the usual Ebiten desktop dependencies for X11/OpenGL/audio. Tagged GitHub releases avoid that setup by shipping prebuilt desktop bundles.

## Quick Start

Run from the repository root:

```bash
go run . -wad DOOM1.WAD
```

The dedicated desktop command works too:

```bash
go run ./cmd/gddoom -wad DOOM1.WAD
```

You can also pass the IWAD as the first positional argument:

```bash
go run . DOOM1.WAD
```

PWAD overlays are comma-separated:

```bash
go run . -wad DOOM2.WAD -file mods/nerve.wad,mods/examplepatch.wad
```

If `-wad` is omitted and the working directory contains one known IWAD, GD-DOOM uses it automatically. If several known IWADs are present, the runtime can open an in-game picker.

## Common Options

```bash
go run . -help
```

Useful flags:

- `-sourceport-mode` enables Source Port defaults.
- `-music-backend=impsynth|meltysynth|auto` selects the music synth backend.
- `-soundfont=PATH` selects an external `.sf2` file for `meltysynth`.
- `-map=E1M1` or `-map=MAP01` starts on a specific map.
- `-record-demo=out.lmp` records a Doom v1.10 demo from live play.
- `-demo=path/to/demo.lmp` plays back a Doom v1.10 demo.
- `-config=config.toml` reads and persists native runtime settings.

Examples:

```bash
go run . -wad DOOM1.WAD -sourceport-mode
go run . -wad DOOM1.WAD -music-backend=impsynth
go run . -wad DOOM1.WAD -music-backend=meltysynth -soundfont=./soundfonts/general-midi.sf2
go run . -wad DOOM2.WAD -map=MAP01 -record-demo=output.lmp
go run . -wad DOOM1.WAD -demo=demos/DOOM1-DEMO1.lmp
```

## Releases

Tagged releases publish desktop bundles for:

- Linux x86_64
- Windows x86_64
- macOS Intel
- macOS Apple Silicon

Each release archive includes the platform binary, `DOOM1.WAD`, `soundfonts/general-midi.sf2`, the README, and license files.

## WebAssembly Build

Build the browser version:

```bash
./scripts/build_wasm.sh
```

That script writes the app to `build/wasm`. It expects `DOOM1.WAD` at the repository root and uses `wasm-opt` automatically when available.

Serve the generated app:

```bash
go run ./cmd/wasmserve
```

Or from inside `build/wasm`:

```bash
cd build/wasm
go run ./server.go
```

The web UI can also load browser-selected `.wad` files through its local IWAD picker flow.

## Development

Run tests:

```bash
go test ./...
```

Additional utilities live under `cmd/` and `scripts/`, including helpers for WAD inspection, map analysis, WASM serving, demo tracing, and music export.

## Status

GD-DOOM is still alpha. It is playable and already covers a broad set of Doom runtime features, but vanilla parity work and edge-case cleanup are still in progress.
