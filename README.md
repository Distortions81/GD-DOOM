# GD-DOOM

[![Go CI](https://github.com/Distortions81/GD-DOOM/actions/workflows/ci.yml/badge.svg)](https://github.com/Distortions81/GD-DOOM/actions/workflows/ci.yml)
[![Go Vulncheck](https://github.com/Distortions81/GD-DOOM/actions/workflows/govulncheck.yml/badge.svg)](https://github.com/Distortions81/GD-DOOM/actions/workflows/govulncheck.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Distortions81/GD-DOOM)](https://goreportcard.com/report/github.com/Distortions81/GD-DOOM)
[![License](https://img.shields.io/github/license/Distortions81/GD-DOOM)](https://github.com/Distortions81/GD-DOOM/blob/main/LICENSE)

<p align="center">
  "Modern / Source Port" mode depicted below
  <img src="e1m1.png" alt="E1M1 screenshot" width="900">
  <br>
  Play the shareware version: <a href="https://m45sci.xyz/u/dist/GD-DOOM">https://m45sci.xyz/u/dist/GD-DOOM</a>
  
  A "Faithful" mode that strives to be the same as the DOS version is available as well via menu selection.
</p>

GD-DOOM plays original Doom data with two presentation targets: a `Faithful` mode that stays closer to vanilla Doom behavior and a `Source Port` mode that enables additional rendering, interpolation, and music features.

License: GD-DOOM is distributed under GNU GPL v2. It is inspired by, ported from, and derivative of id Software's DOOM source release. See [LICENSE](/home/dist/github/GD-DOOM/LICENSE) and [NOTICE](/home/dist/github/GD-DOOM/NOTICE).

## GD-DOOM Over Vanilla DOOM

### Rendering Features

- 32-bit RGBA output replaces vanilla Doom's indexed framebuffer presentation.
- Internal rendering is not locked to vanilla Doom's low display resolution, so walls, sprites, HUD, and automap lines can be presented sharply on modern displays.
- `Faithful` mode keeps Doom colormap-based shading and gamma-table behavior closer to vanilla. `Source Port` mode keeps Doom-style distance-light math but does not remap through the colormap table.
- Animated light specials such as fire flicker, light flash, strobe, and glow are supported, and in source-port mode those light changes can fade between tics instead of stepping instantly once per tic.
- Floors and ceilings are rendered as textured lit surfaces using the current sector light level, rather than inheriting vanilla Doom's more limited plane presentation.
- Camera position and camera yaw interpolate between simulation tics instead of drawing only the raw 35 Hz simulation step.
- Thing rendering interpolates previous and current X/Y/Z state, so monsters, pickups, decorations, and projectiles move more smoothly on screen.
- Certain monster movement states also use thinker-position blending over the state's tic window, so some monster AI movement is spread across multiple render steps instead of jumping from one thinker result to the next.
- Mouse-driven turning is supported in walk view.
- Animated wall and flat textures can crossfade between frames in source-port mode instead of switching abruptly every 8 tics.
- Many map things use explicit multi-frame sprite sequences, including keys, bonuses, lights, torches, gore decorations, and pickups.
- Weapon psprites can generate blended intermediate patch frames instead of only showing the discrete vanilla frame transitions.
- Source-port mode supports a separate GPU sky path.
- HUD and status-bar presentation can be scaled independently for modern displays.
- Walk view and automap are integrated with pause and front-end overlays.
- Doom-style aspect correction and gamma handling are preserved, with an optional CRT postprocess effect.

### Sound And Music Features

- Two distinct music styles are available: OPL-style synthesis for a sound closer to classic Doom hardware, and SoundFont-backed General MIDI for a fuller, richer, more instrument-like soundtrack.
- Music is rendered as stereo 16-bit PCM with MUS pan handling, so tracks can feel wider and less flat than a simple centered playback path.
- Pan range can be tightened toward center or left fully wide.
- Music and sound effects are mixed separately.
- The runtime supports higher-quality MIDI-style playback through SoundFonts where available instead of limiting the experience to a single classic synth character.
- In-game music controls can switch playback backend and volume.
- A dedicated Music submenu and music player can browse tracks by WAD, episode, and map grouping.
- Browser and desktop builds can use bundled or detected SoundFonts where available.

### Map And Game Data Features

- Original Doom data loads directly, including IWAD plus PWAD combinations.
- When multiple data sources are loaded, GD-DOOM can pick valid maps from that combined content and honor PWAD map replacements.
- If several known base WADs are available, GD-DOOM can present an in-game IWAD picker.
- Save/load support is integrated into the runtime flow.
- Episode, skill, help/read-this, pause, and related menu flows are integrated into the runtime presentation.
- Demo playback and recording are available alongside normal play.
- The automap supports follow mode, heading-up rotation, optional grid overlay, thing legend display, big-map view, and player map marks, while still staying recognizably Doom.

### Performance And Polish

- Detail scaling and optional auto-detail can lower render cost when frame time rises.
- Render-side precaching warms map textures, sprite patch data, monster refs, projectile refs, world-thing animation refs, and weapon blend assets before they are needed on screen.
- Wall rendering uses occlusion, span rejection, span clipping, and slice occlusion to skip covered work.
- Billboard and masked rendering use row and column clipping against wall depth and masked clip buffers, reducing hidden sprite work.

## Quick Start

Requirements:
- Go 1.22+
- A Doom base WAD such as `DOOM.WAD`, `DOOM2.WAD`, `TNT.WAD`, `PLUTONIA.WAD`, or `DOOM1.WAD`

Run:

```bash
go run ./cmd/gddoom -wad DOOM1.WAD
```

PWAD overlays:

```bash
go run ./cmd/gddoom -wad DOOM2.WAD -file mods/nerve.wad,mods/examplepatch.wad
```

If `-wad` is omitted and the working directory contains one known IWAD, GD-DOOM uses it automatically. If several known IWADs are present, it can open an in-game picker.

Tagged GitHub releases now publish desktop bundles for Linux, Windows, macOS Intel, and macOS Apple Silicon. Each release archive includes the platform binary, the tracked `DOOM1.WAD`, and `soundfonts/general-midi.sf2`.

## Music

GD-DOOM supports two music styles:

- `impsynth` for a classic OPL-style Doom feel.
- `meltysynth` for richer SoundFont-based General MIDI playback.

Examples:

```bash
go run ./cmd/gddoom -wad DOOM1.WAD -music-backend=impsynth
go run ./cmd/gddoom -wad DOOM1.WAD -music-backend=meltysynth -soundfont=./soundfonts/general-midi.sf2
```

The frontend options menu has a dedicated Music submenu where you can change music volume, switch between OPL3 and MeltySynth, pick a SoundFont from `./soundfonts`, and open the music player.

## Notes

GD-DOOM is still alpha. It is playable, but full vanilla parity is still in progress and some edge cases remain.
