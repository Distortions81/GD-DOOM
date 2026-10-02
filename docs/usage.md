# Usage and development reference

Detailed options, controls, cheats, relay setup, configuration, browser builds,
and development tools. Start with the [README](../README.md) for an overview.
Run the commands below from the repository root.

## Common Options

Print all flags:

```bash
go run . -help
```

Frequently used options:

- `-gpu-renderer=false` selects the CPU software renderer. GPU shaders are enabled by default in both Faithful and Source Port modes; see [Rendering](#rendering).
- `-sourceport-mode` starts in the smoother, higher-fidelity Source Port profile.
- `-pc-speaker` switches sound effects to the PC speaker emulation path.
- `-pc-speaker-output=linux` (Linux only) routes PC speaker output to a real hardware buzzer. This uses the `pcspkr` evdev node and requires write permission to it.
- `-pc-speaker-variant=paper-speaker|small-buzzer|passthrough` selects the physical speaker model or direct beep output.
- `-pc-speaker-interleave-hz=N` tunes how quickly the speaker switches between SFX and music when both are active (range 10–1000 Hz).
- `-music-backend=auto|impsynth|pcspeaker|meltysynth` selects the music style/engine.
- `-soundfont=PATH` selects an external `.sf2` file for `meltysynth`.
- `-detail-level=N` sets starting image detail and `-auto-detail` tries to keep the game near 60 FPS automatically.
- `-no-monsters` disables monster spawns.
- `-crt-effect` and `-texture-anim-crossfade-frames=N` enable extra visual polish in Source Port mode.
- `-map=E1M1` or `-map=MAP01` starts on a specific map.
- `-record-demo=out.lmp` records a Doom v1.10 demo from live play.
- `-demo=path/to/demo.lmp` plays back a Doom v1.10 demo and exits when playback ends.
- `-trace-demo-state=path.jsonl` writes a detailed tick-by-tick state log during demo playback.
- `-broadcast[=ADDR]` starts a live session for watchers, defaulting to `127.0.0.1:6670`.
- `-watch[=ADDR] -watch-session=N` joins a relay session as a viewer.
- `-low-latency` trades some efficiency for faster live delivery.
- `-mic` sends microphone audio while broadcasting.
- `-mic-codec=silk|g726|pcm` selects the voice codec used for microphone streaming.
- `-config=config.toml` reads and persists native runtime settings.
- `-dump-music` saves the game's music tracks as WAV files.
  Note: the main app dump path currently exports the built-in OPL/SoundFont renderers, while `cmd/musicwav` and `scripts/dump_music.sh` support direct PC speaker WAV export modes.

There are more flags than the short list above. Use `go run . -help` for the full set if you want every tweak and debug option.

## Rendering

**GPU shaders keep Doom's software-rendered appearance** on desktop and in the
browser. The CPU retains the software renderer's visibility, projection,
clipping, and lighting decisions. Shaders sample indexed textures to draw the
same walls, floors, ceilings, masked textures, sprites, and sky with crisp texel
sampling. Repeated columns and rows use compact commands, and horizontal sprite
visibility spans are reused between clipping boundaries to reduce close-up work.

Faithful mode resolves texture indices through the WAD's 256-entry COLORMAP rows
and selected gamma palette. Its sky uses the exact CPU column/row coordinate
lookups, fullbright effects use the active palette, and low detail duplicates
even framebuffer columns. Source Port mode keeps its smoother motion,
high-resolution output, and full-color lighting with the same GPU draw path.

GPU drawing is on by default. Unsupported texture banks automatically use the
CPU renderer; `-gpu-renderer=false` also selects it explicitly. The renderer flag
is not saved to config. Minor texel-boundary rounding differences are possible.
Spectre fuzz uses a 320×200 logical mask and grain at every resolution, classic
column sequencing, and the WAD's COLORMAP row six in both modes. The GPU resolves
short neighbor-feedback chains on that logical grid, then scales the result up
with full-resolution wall clipping. Sprites, spectres, and masked textures draw
in depth order when fuzz is present. Each spectre snapshots the scene behind it,
including farther spectres; nearer enemies draw afterward and stay sharp.
Feedback interrupted by wall clipping still approximates the software framebuffer.

VSync is enabled by default on desktop and WASM. Browser rendering follows the
display refresh without the former 75 FPS sleep throttle; `-no-vsync` remains an
explicit override. Doom simulation continues at 35 tics per second.

```bash
# Faithful look with GPU drawing.
go run . -wad DOOM1.WAD -sourceport-mode=false

# Faithful look with software drawing, for comparison.
go run . -wad DOOM1.WAD -sourceport-mode=false -gpu-renderer=false

# Uncapped rendering for performance comparisons.
go run . -wad DOOM1.WAD -sourceport-mode -no-vsync
```

See [GPU renderer checks and benchmarks](gpu-renderer-performance.md) for visual
comparisons, palette checks, and measured CPU preparation costs. Those synthetic
timings are not whole-game FPS; hardware and browser performance varies.

Aspect correction note:
In faithful mode, GD-DOOM applies Doom's classic 4:3 correction as a whole-screen stretch after rendering. In Source Port mode, it applies that correction during rendering. A small set of sprites that are meant to read as circular, such as pickups and fireballs, are kept round instead of being stretched.

Examples:

```bash
go run . -wad DOOM1.WAD -sourceport-mode
go run . -wad DOOM1.WAD -pc-speaker
go run . -wad DOOM1.WAD -music-backend=impsynth
go run . -wad DOOM1.WAD -music-backend=meltysynth -soundfont=./soundfonts/general-midi.sf2
go run . -wad DOOM1.WAD -detail-level=2 -auto-detail
go run . -wad DOOM2.WAD -map=MAP01 -record-demo=output.lmp
go run . -wad DOOM1.WAD -demo=demos/DOOM1-DEMO1.lmp
go run . -wad DOOM1.WAD -dump-music
go run ./cmd/musicwav -doom2 DOOM2.WAD -song D_RUNNIN -mode pcspeaker-clean -out ./out/music-pcspeaker-clean
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

The broadcaster prints the assigned session id on startup. View from another instance using the same base game and mod files:

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
- Watchers can also participate in session chat.
- `-low-latency` favors quicker delivery over more batching.
- Current microphone codecs are `silk`, `g726`, and `pcm`.
- The wire format is documented in [`netplay-protocol.md`](../netplay-protocol.md).

This is live spectating, not traditional co-op. One machine plays, the others watch the run as it happens, with chat and optional voice alongside the stream.

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
- Chat: `T`.
- Push to talk: `Caps Lock`.
- Weapon next / previous: `Page Down` / `Page Up` or mouse buttons `MB5` / `MB4`.
- Help: `F1`.

Bindings can be changed in the frontend and pause-menu keybind screens and saved in `config.toml`. There are also extra runtime shortcuts for detail level, gamma, screenshots, and automap behavior.

## Menus And Config

The frontend and pause menus expose most settings people actually want to change while playing:

- Sound options for SFX/music volume.
- Voice options for codec, sample rate, automatic gain control, gate strength, device selection, and push-to-talk.
- Key binding menus with primary/alternate bindings and reset-to-default support.
- Browser/touch-friendly frontend flow, including touch prompts on the title screen, touch controls in frontend submenus such as the music player, and touch-safe menu-close debounce.
- Persisted native settings through `config.toml`, including runtime options and the `keybinds` table.

`config.toml` is the desktop settings file. GD-DOOM reads it at startup and writes changes back when you update settings or bindings in-game. You can ignore it and use the menus, or edit it by hand.

A representative config can include entries such as:

```toml
detail_level_faithful = 0
detail_level_sourceport = 0
auto_detail = false
gamma_level = 2
mouselook = true
music_backend = "meltysynth"
soundfont = "soundfonts/general-midi.sf2"

[keybinds]
move_forward = ["W", "UP"]
chat = ["T", ""]
voice = ["CAPSLOCK", ""]
use = ["SPACE", "E"]
```

## Browser Build

GD-DOOM also has a browser version. To build it locally:

```bash
./scripts/build_wasm.sh
```

The script writes fresh browser assets, including `gddoom.wasm.gz`. It requires:

- `DOOM1.WAD` at the repository root
- `wasm_exec.js` from your local Go toolchain
- optional `wasm-opt` on `PATH` for automatic optimization (`-O4` by default, override with `WASM_OPT_LEVEL`)

Serve the generated app:

```bash
go run ./cmd/wasmserve
```

By default `cmd/wasmserve` serves the current directory if it already contains the built app; otherwise it falls back to `build/wasm` and listens on `:8000`.

You can also serve a specific output directory:

```bash
go run ./cmd/wasmserve -dir build/wasm -addr :8000
```

The browser UI can load WAD files locally from your machine, cache SoundFonts for `meltysynth`, and keep saves in browser storage. It shares most of the same runtime code as desktop builds, though some features remain platform-specific, especially microphone capture.

On browsers with strict autoplay policies, a click is required before audio starts. On touch devices, the browser build uses a dual-pad layout for movement, turning, fire, use, and menu access.

## Development

Run the test suite:

```bash
go test ./...
```

Run the slower export and real-asset integration checks explicitly:

```bash
scripts/test_integration.sh ./internal/app
```

This integration lane also includes generator-style tests that write artifacts, such as the billboard bbox dump in `internal/doomruntime`.

If you are working on the engine itself, extra utilities are included under [`cmd/`](../cmd):

- [`cmd/gdsfrelay`](../cmd/gdsfrelay) runs the live session relay used by `-broadcast` and `-watch`.
- [`cmd/wasmserve`](../cmd/wasmserve) serves the browser build locally.
- [`cmd/demotracecmp`](../cmd/demotracecmp) compares two demo state logs to help find mismatches or desyncs.
- [`cmd/musicwav`](../cmd/musicwav) exports in-game music tracks to WAV files, including `impsynth`, `pcspeaker`, `pcspeaker-clean`, and `pcspeaker-piezo` modes with optional single-song selection via `-song`.
- [`cmd/pcspeaker`](../cmd/pcspeaker) captures live PC speaker output, interleaves music and SFX streams, and can drive the Linux hardware buzzer directly for testing.
- [`cmd/mapprobe`](../cmd/mapprobe) inspects map data such as sectors, lines, tags, and things.
- [`cmd/mapaudit`](../cmd/mapaudit) generates a report about oddities in local Doom map data.
- [`cmd/wadtool`](../cmd/wadtool) extracts individual files from WADs.

These tools are for development, testing, and troubleshooting rather than normal play.

The lessons learned while adapting Doom's BSP, wall clipping, and visplanes to
an external vector-display renderer are documented in
[`docs/vector-rendering-visibility.md`](../docs/vector-rendering-visibility.md).

## Advanced Diagnostics

These optional environment variables are mainly useful when troubleshooting voice or live-session behavior. Any non-empty value enables the feature.

- `GD_DOOM_NET_BANDWIDTH_OVERLAY` shows the in-game network bandwidth overlay.
- `GD_DOOM_VOICE_SYNC_OVERLAY` adds the voice sync offset to the bandwidth overlay when voice sync data is available.
- `GD_DOOM_VOICE_AGC_LOG` prints occasional automatic gain control diagnostics while broadcasting voice.

Examples:

```bash
GD_DOOM_NET_BANDWIDTH_OVERLAY=1 go run . -wad DOOM1.WAD
GD_DOOM_NET_BANDWIDTH_OVERLAY=1 GD_DOOM_VOICE_SYNC_OVERLAY=1 go run . -wad DOOM1.WAD
GD_DOOM_VOICE_AGC_LOG=1 go run . -wad DOOM1.WAD
```

Voice runtime notes:

- If the viewer has to skip ahead to catch live audio back up, you will see `voice-skip ...` messages in the console.

Supported commercial Doom-family game/add-on fingerprints tracked by the runtime are documented in [`commercial-wads.md`](../commercial-wads.md).

That file is for recognition and compatibility lookup. It is not a promise that GD-DOOM fully supports every non-Doom title listed there.
