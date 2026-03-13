# Useful Launch Params

Main user-facing flags for running the game. For the exact current CLI surface, use:

```bash
go run ./cmd/gddoom -h
```

This file intentionally focuses on gameplay and presentation flags rather than every debug/development option.

## Basic

- `-wad <path>`: choose the IWAD file.
- `-map <name>`: start on a specific map such as `E1M1` or `MAP01`.
- `-config <path>`: use a specific `config.toml`.
- `-render=false`: parse and validate only; do not launch the Ebiten runtime.
- `-details`: include extra parsed map details in CLI output.
- `-width <n>` / `-height <n>`: set window size.

Example:

```bash
go run ./cmd/gddoom -wad DOOM1.WAD -map E1M1 -width 1280 -height 720
```

## Gameplay

- `-player <1..4>`: choose the local player start slot.
- `-skill <1..5>`: set Doom skill level.
- `-game-mode <single|coop|deathmatch>`: apply thing spawn filtering for the selected mode.
- `-fastmonsters`: enable fast-monsters behavior.
- `-always-run`: start with always-run enabled.
- `-auto-weapon-switch`: start with auto weapon switch enabled.
- `-start-in-map`: start with automap open.
- `-cheat-level <0..3>`: set startup automap/inventory cheats.
- `-invuln`: start invulnerable.
- `-all-cheats`: shortcut for full startup cheats.
- `-show-no-skill-items`: show pickup items that have no skill bits set.
- `-show-all-items`: show pickup items regardless of normal pickup spawn filters.

These item-visibility overrides affect pickup items only. They do not force monsters to spawn.

## View and Control

- `-zoom <n>`: override startup automap zoom.
- `-detail-level <n>`: set startup detail level (`-1` keeps the mode default).
- `-gamma-level <n>`: set startup gamma level (`-1` keeps the mode default).
- `-mouselook=true|false`: enable or disable mouse turning.
- `-mouselook-speed <n>`: adjust mouse turn speed.
- `-keyboard-turn-speed <n>`: adjust keyboard turn speed.
- `-line-color-mode <parity|doom>`: choose automap line coloring.
- `-sourceport-mode`: enable source-port conveniences and alternate defaults.
- `-sourceport-thing-render-mode <glyphs|items|sprites>`: choose sourceport automap thing rendering.
- `-sourceport-thing-blend-frames`: allow blended sub-tic thing frames on the automap.

Example:

```bash
go run ./cmd/gddoom -wad DOOM1.WAD -map E1M1 -sourceport-mode
```

## Audio

- `-music-volume <0..1>`: set music volume.
- `-sfx-volume <0..1>`: set sound-effect volume.
- `-mus-pan-max <0..1>`: limit stereo panning for OPL3 music playback.
- `-opl-volume <0..4>`: adjust OPL synth output gain.
- `-opl3-backend <auto|impsynth|nuked>`: choose the OPL3 backend implementation.
- `-opl-bank <path>`: override the WAD `GENMIDI` bank with an external OP2/GENMIDI bank file.

## Rendering and Presentation

- `-doom-lighting=true|false`: toggle Doom lighting math and `COLORMAP` shading.
- `-sourceport-sector-lighting=true|false`: toggle classic sector lighting while in sourceport mode.
- `-gpu-sky=true|false`: enable or disable the experimental GPU sky path used by sourceport mode.
- `-sky-upscale <nearest|sharp>`: choose GPU sky upscale mode.
- `-kage-shader=true|false`: enable or disable Kage postprocessing support.
- `-crt-effect=true|false`: toggle the CRT effect.
- `-texture-anim-crossfade-frames <n>`: sourceport texture animation blend window.
- `-import-textures=true|false`: build Doom texture tables for the software renderer.
- `-import-pcspeaker=true|false`: import PC-speaker startup sounds.
- `-no-aspect-correction`: disable Doom-style 4:3 aspect correction.
- `-no-vsync`: disable vsync.
- `-nofps`: hide the FPS/MS overlay.

Sourceport note:
- If `-sourceport-mode` is enabled and you do not explicitly set `-gpu-sky` or `-sky-upscale`, GD-DOOM currently defaults to GPU sky with `-sky-upscale=sharp`.
- Sourceport mode also normalizes automap thing rendering to `items` unless you override it with `-sourceport-thing-render-mode`.

## Demo and Trace

- `-demo <path>`: run a Doom v1.10 `.lmp` demo and exit when it completes.
- `-record-demo <path>`: record live input to a Doom v1.10 `.lmp` demo.
- `-trace-demo-state <path>`: write per-tic JSONL demo state while `-demo` is playing back.
- `-cpuprofile <path>`: write a Go CPU profile.

## Advanced Render Tuning

- `-wall-occlusion=true|false`: enable or disable coarse wall-span occlusion.
- `-wall-span-reject=true|false`: enable or disable early solid-span rejection.
- `-wall-span-clip=true|false`: enable or disable x-range clipping against coarse wall spans.
- `-wall-slice-occlusion=true|false`: enable or disable wall-slice triangle/bbox occlusion checks.
- `-billboard-clipping=true|false`: enable or disable sprite/thing/projectile clipping.
- `-no-cull-clipping`: disable wall occlusion and billboard clipping together.

## Config

- `config.toml` is auto-read by default when present.
- Command-line flags override config values.
- A subset of runtime settings save back to the active config path, including detail/gamma, audio volumes, mouse look, line colors, and a few gameplay/presentation toggles.
