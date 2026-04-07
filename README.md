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

GD-DOOM is a Go-based Doom runtime and source port project for original Doom data. It supports native desktop play and a WebAssembly build, loads IWAD and PWAD content, and can play or record Doom v1.10 demos.

It currently exposes two presentation styles:

- `Faithful` mode stays closer to classic DOS Doom behavior and presentation.
- `Source Port` mode enables smoother camera motion, higher-fidelity rendering and more.

GD-DOOM is distributed under GNU GPL v2. It is inspired by, ported from, and derivative of id Software's DOOM source release. See [LICENSE](/home/dist/github/GD-DOOM/LICENSE) and [NOTICE](/home/dist/github/GD-DOOM/NOTICE).

## Highlights

### Rendering

- `Faithful` and `Source Port` runtime modes.
- 32-bit RGBA rendering instead of a vanilla indexed framebuffer presentation.
- Higher-resolution walls, sprites, HUD, and automap output.
- Interpolated camera movement, yaw, and thing motion between 35 Hz simulation tics.
- Textured lit floors and ceilings, optional GPU sky path, and optional CRT postprocess effect.
- Texture animation crossfades, smoother weapon transitions, and broader multi-frame sprite presentation.
- Integrated automap with follow mode, rotate mode, big-map view, grid, and map marks.

### Audio

- `impsynth` backend for an OPL-style classic Doom feel.
- `meltysynth` backend for SoundFont-backed General MIDI playback.
- Separate music and SFX volume controls.
- Stereo MUS playback with adjustable pan width.
- In-game music menu and browser/player music flow.
- Experimental Linux PulseAudio capture scaffold for upcoming relay voice/audio input work.

### Runtime

- Direct IWAD loading with optional PWAD overlays.
- Automatic IWAD selection when one known base WAD is present.
- In-game IWAD picker when multiple known IWADs are available.
- Save/load support integrated into runtime flow.
- Demo playback, demo recording, and per-tic demo trace export.
- Relay-backed gameplay broadcast/watch sessions with optional microphone voice streaming.
- Native config-file support through `config.toml`.

## Requirements

- Go `1.26.1` or newer
- A Doom IWAD such as `DOOM.WAD`, `DOOM1.WAD`, `DOOM2.WAD`, `TNT.WAD`, or `PLUTONIA.WAD`

On Linux, native builds also need the usual Ebiten desktop dependencies for X11, OpenGL, and audio.

## Quick Start

Run from the repository root:

```bash
go run . -wad DOOM1.WAD
```

The dedicated desktop entrypoint is equivalent:

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

If `-wad` is omitted and the working directory contains one known IWAD, GD-DOOM uses it automatically. If multiple known IWADs are present, the runtime can open an in-game picker.

## Common Options

Print all flags:

```bash
go run . -help
```

Frequently used options:

- `-sourceport-mode` enables Source Port defaults.
- `-music-backend=auto|impsynth|meltysynth` selects the music backend.
- `-soundfont=PATH` selects an external `.sf2` file for `meltysynth`.
- `-map=E1M1` or `-map=MAP01` starts on a specific map.
- `-record-demo=out.lmp` records a Doom v1.10 demo from live play.
- `-demo=path/to/demo.lmp` plays back a Doom v1.10 demo and exits when playback ends.
- `-trace-demo-state=path.jsonl` exports per-tic demo state during `-demo` playback.
- `-broadcast[=ADDR]` publishes gameplay to a GDSF relay, defaulting to `127.0.0.1:6670`.
- `-watch[=ADDR] -watch-session=N` joins a relay session as a viewer.
- `-low-latency` disables streamer-side tic batching for relay broadcast.
- `-mic` publishes microphone audio on the relay audio stream when broadcasting.
- `-mic-codec=silk|g726|pcm` selects the microphone wire codec.
- `-config=config.toml` reads and persists native runtime settings.
- `-dump-music` renders WAV exports for detected IWAD music.

Examples:

```bash
go run . -wad DOOM1.WAD -sourceport-mode
go run . -wad DOOM1.WAD -music-backend=impsynth
go run . -wad DOOM1.WAD -music-backend=meltysynth -soundfont=./soundfonts/general-midi.sf2
go run . -wad DOOM2.WAD -map=MAP01 -record-demo=output.lmp
go run . -wad DOOM1.WAD -demo=demos/DOOM1-DEMO1.lmp
go run . -wad DOOM1.WAD -dump-music
go run . -wad DOOM1.WAD -broadcast
go run . -wad DOOM1.WAD -broadcast -mic -mic-codec=silk
go run . -wad DOOM1.WAD -watch -watch-session=1
go run . -wad DOOM1.WAD -cheat-level=3
go run . -wad DOOM1.WAD -all-cheats
```

## Relay Watch / Voice

Run the relay server:

```bash
go run ./cmd/gdsfrelay
```

Broadcast a session to the default local relay:

```bash
go run . -wad DOOM1.WAD -broadcast
```

The broadcaster prints the assigned session id on startup. View from another instance with the same IWAD/PWAD stack:

```bash
go run . -wad DOOM1.WAD -watch -watch-session=1
```

Optional voice broadcast is available on native Linux builds through PulseAudio capture:

```bash
go run . -wad DOOM1.WAD -broadcast -mic
go run . -wad DOOM1.WAD -broadcast -mic -mic-codec=silk
```

Notes:

- `-broadcast` and `-watch` are mutually exclusive.
- `-watch` also connects to the paired relay audio stream automatically.
- `-low-latency` flushes each gameplay tic immediately instead of batching.
- Current microphone codecs are `silk`, `g726`, and `pcm`.
- The wire format is documented in [`netplay-protocol.md`](/home/dist/github/GD-DOOM/netplay-protocol.md).

## Cheats

Startup cheats:

- `-cheat-level=1` enables full automap reveal with `IDDT 2`.
- `-cheat-level=2` applies the above plus `IDFA`.
- `-cheat-level=3` applies the above plus `IDKFA` and invulnerability.
- `-invuln` starts with invulnerability enabled.
- `-all-cheats` is the alias for full startup cheats.

Typed in-game cheats:

- `iddqd` toggles invulnerability.
- `idfa` grants weapons, ammo, and armor.
- `idkfa` grants weapons, ammo, armor, and keys.
- `iddt` cycles automap reveal and thing display states.
- `idclip` toggles no-clip.
- `idspispopd` also toggles no-clip.
- `idmypos` prints the current player angle and coordinates.
- `idchoppers` grants chainsaw + invulnerability tick behavior matching classic Doom.
- `idclev##` warps to a map such as `idclev11` or `idclev23`.
- `idmus##` changes music when the current WAD supports that track selection.
- `idbehold` shows the power-up cheat prompt.
- `idbeholdv`, `idbeholds`, `idbeholdi`, `idbeholdr`, `idbeholda`, and `idbeholdl` toggle the matching power-up effect.

## Controls

Default desktop controls are:

- Menus: `Arrow Keys` + `Enter`, `Esc` to go back.
- Game: `WASD` or arrow keys to move, mouse to turn.
- Fire: `Ctrl` or left mouse button.
- Use / open: `E` or `Space`.
- Run modifier: `Shift`.
- Strafe modifier: `Alt`.
- Automap: `Tab`.
- Help: `F1`.

There are additional runtime shortcuts for features like detail level, gamma, screenshots, and automap behavior.

## WebAssembly Build

Build the browser version:

```bash
./scripts/build_wasm.sh
```

The script writes output to `build/wasm`, copies the web assets, and produces `gddoom.wasm.gz`. It requires:

- `DOOM1.WAD` at the repository root
- `wasm_exec.js` from your local Go toolchain
- optional `wasm-opt` on `PATH` for automatic optimization

Serve the generated app:

```bash
go run ./cmd/wasmserve
```

By default `cmd/wasmserve` serves the current directory if it already contains the built app; otherwise it falls back to `build/wasm` and listens on `:8000`.

You can also serve a specific output directory:

```bash
go run ./cmd/wasmserve -dir build/wasm -addr :8000
```

The browser UI can load user-selected `.wad` files through its local IWAD picker flow.

## Development

Run the test suite:

```bash
go test ./...
```

## Environment Variables

All current GD-DOOM environment toggles use the `GD_DOOM_` prefix. Any non-empty value enables the behavior.

- `GD_DOOM_NET_BANDWIDTH_OVERLAY` shows the in-game network bandwidth overlay.
- `GD_DOOM_VOICE_SYNC_OVERLAY` adds the voice sync offset to the bandwidth overlay when voice sync data is available.
- `GD_DOOM_VOICE_AGC_LOG` prints rate-limited voice AGC diagnostics to stdout while broadcasting voice.

Examples:

```bash
GD_DOOM_NET_BANDWIDTH_OVERLAY=1 go run . -wad DOOM1.WAD
GD_DOOM_NET_BANDWIDTH_OVERLAY=1 GD_DOOM_VOICE_SYNC_OVERLAY=1 go run . -wad DOOM1.WAD
GD_DOOM_VOICE_AGC_LOG=1 go run . -wad DOOM1.WAD
```

Voice runtime notes:

- Viewer skip-ahead recovery logs remain on by default as `voice-skip ...` lines so audio catch-up is visible without extra flags.

Useful helper commands and tools live under [`cmd/`](/home/dist/github/GD-DOOM/cmd) and [`scripts/`](/home/dist/github/GD-DOOM/scripts), including utilities for WAD inspection, map analysis, demo tracing, music export, and WASM serving.

## Status

GD-DOOM is still alpha. It is already playable and covers a broad set of Doom runtime features, but vanilla parity work and edge-case cleanup are still in progress.
