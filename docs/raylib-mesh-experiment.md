# Raylib GPU mesh experiment

The `experiments` branch has a native Raylib-Go host for the 3D level mesh. It
drives the existing Doom simulation at 35 Hz and draws the repaired directed-side
floor/ceiling triangulation, walls and masked middle textures using OpenGL depth
testing. The window defaults to 1280×800 and renders at the window resolution.

From the repository root on Linux:

```bash
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD

# Build once for subsequent runs.
go build -tags raylib,x11 -o build/raydoom ./cmd/raydoom
./build/raydoom -wad DOOM1.WAD

# Launch a specific level directly.
./build/raydoom -wad DOOM1.WAD -map E1M3

# Load PWADs in order; the last definition of a lump or map wins.
./build/raydoom -wad DOOM1.WAD -file levels.wad,artwork.wad -menu=false
```

This backend requires cgo, a C compiler, OpenGL 3.3 and platform window-system
development libraries. Raylib-Go v0.60.1 is pinned in `go.mod`. On Linux the
`x11` build tag selects X11; see the upstream [build prerequisites and platform
instructions](https://github.com/gen2brain/raylib-go/tree/v0.60.1/raylib).
The mesh ownership code uses the cgo bindings; `CGO_ENABLED=0` is unsupported.
The ordinary application and WASM entry points continue to use Ebiten.

## Controls and scope

- Default controls: WASD moves; up/down also move forward/backward.
- Options → Key Bindings changes primary/alternate keys and mouse buttons.
- Mouse or left/right arrows turn. Always-run defaults on, matching Ebiten;
  either Shift walks, and either Alt strafes with turn keys. Saved preferences
  or `-always-run=false` can select walking by default, with Shift running.
- Caps Lock toggles always-run, F12 toggles automatic weapon switching, and
  backslash toggles mouse turning. The toggles show the shared HUD messages
  and save through the same preferences as the menus.
- Comma/period decrease/increase live game speed by 0.1, slash resets 1x.
  The shared clock clamps speed to 0.1x–8x; demos and watcher streams keep their
  authoritative cadence. Pausing and melt transitions freeze the live clock.
- In 3D view, plus/minus changes the status-bar layout; Ctrl+brackets changes
  HUD size. These shortcuts use the menu's settings and save to preferences.
- Page Up/Page Down, the wheel or mouse buttons 4/5 select the previous/next owned weapon.
- E or Space uses doors and switches. Left mouse or Ctrl fires; 1–7 selects weapons.
- F7 cycles textured, sector colors and wireframe views.
- F8 toggles the main renderer's CRT effect; Shift+F8 cycles nearest, trilinear
  and 8x anisotropic texture filtering.
- Shift+F9 cycles Doom lighting, sector-only lighting and fullbright.
- F2/F3 open save/load slots; F6 quicksaves and F9 quickloads.
- F10 opens the Doom quit prompt; Shift+F10 toggles renderer diagnostics.
- F11 cycles the main renderer's five gamma levels and saves the preference.
- P or Pause pauses/resumes and releases/recaptures the mouse. Losing window focus also
  stops simulation input and playing sound effects.
- R toggles heading-up map rotation in either view. Shift+R reloads the level
  at its normal player start in 3D view. Escape opens/closes the menu.
- F5 cycles full, half, third and quarter scene resolution, then AUTO. HUD and
  menus remain sharp in 3D view; the automap and its overlays scale together.
- Enter restarts after death, retaining campaign history.
- Tab switches between 3D view and the textured automap. Gameplay continues in map view.
- F1 opens the WAD’s Read This pages; F4 opens sound settings. Menu arrows/wheel select
  rows, left/right adjust values, Enter or a mouse click confirms, and Escape
  or Backspace goes back. Quit is available through the menu or window close.

The native host supports playable campaigns. Monsters and
player damage are enabled by default. `-skill` selects 1–5 (default 3), while
`-nomonsters -god` restores the geometry inspection setup. Movement, collision,
AI, hitscan combat, projectiles, doors/lifts, pickups and sector thinkers use
the original engine. The native backend exposes their existing render state
rather than implementing separate gameplay rules.
Monster thinker positions blend between simulation ticks by default, as in the
main source-port host; this changes presentation without changing simulation.

IWAD aliases and case-insensitive filenames use the main launcher's resolver.
`-file` accepts comma-separated PWADs and applies them to the map, texture,
sprite, HUD/font, sound and music loaders together. Direct launches without
`-map` start at the last overlay's first map; overlays containing only artwork
or media do not change that selection. The title loop and New Game retain the
main host's episode/skill flow. Saves list every WAD in load order with its
BLAKE3 hash; relay sessions use SHA1 over the concatenated WAD bytes, so both
hosts identify the same stack and detect missing or reordered overlays.

`-player` selects starts 1–4. `-show-no-skill-items` and `-show-all-items` use the
main engine's item filters. `-cheat-level` selects its startup automap/IDFA/IDKFA
levels (0–3), and `-all-cheats` retains the legacy full-cheat switch. `-invuln`
and `-no-monsters` accept the main flag names alongside `-god` and `-nomonsters`.
`-mouselook-speed`/`-keyboard-turn-speed` also alias the experiment's speed flags;
finite positive command-line values use the main engine's multipliers. `-nofps`
hides the shared counter overlay. These common saved preferences, legacy
horizontal inversion and the demo stop-tic setting are honored, while explicit
flags (including false and zero) override their saved values.

Monsters (including corpses and paired rotation flips), pickups, decorations,
barrels, missiles, impacts, blood/puffs, teleport fog and boss-spawn effects use
camera-facing textured cards in the level's hardware depth buffer. Alpha holes
reveal the geometry behind them. Positions interpolate through the existing
render helpers; pickups disappear when collected. Animated and self-lit sprite
frames use the same selection helpers as the main renderer. Spectres sample the
already drawn scene through the main renderer's coarse 320x200 fuzz posts,
column-first phase progression and palette feedback. Their masks and final
cards use the hardware depth buffer. Visible sprites render from far to near
so a spectre includes farther sprites and spectres in its background snapshot.
Scenes without a visible spectre retain normal texture batching. Snapshots and
fuzz shading stay on the GPU; only the small logical post metadata is uploaded.

The classic HUD shares the main renderer's logical patch layout, numbers,
weapon ownership, keys, ammo capacities and animated face widget. Weapon raising,
lowering, bobbing, attack animations and muzzle-flash composites use existing
psprite state and CPU patch composition. HUD/weapon textures use nearest
sampling, while world sprite textures use the selected world filter. GPU images
remain cached; unchanged HUD patch lists and sprite buffers are reused.
Weapon/HUD uploads have a one-pixel transparent gutter and clamp sampling to
prevent opposite-edge bleed at fractional animation positions with MSAA.
The source rectangle excludes the gutter, preserving artwork size and offsets.

The sky uses the WAD's map-specific SKY texture with a yaw-driven panoramic
shader. Damage/pickup flashes and expiring pickup messages are exposed to the
host. Doom-style lighting remains RGB shading. Invulnerability uses the WAD's
exact inverse colormap for walls, floors, ceilings and world sprites, with the
main renderer's expiration blink. IDDQD retains normal colors. Sky, weapon and
HUD artwork stay unshaded, matching the main source-port presentation. Normal
source-port lighting uses RGB shading in both hosts; faithful palette remapping
is specific to the main renderer's faithful mode.

World surfaces and sprites use the main renderer's exact channel gamma tables
after RGB lighting. The default is level 2; `-gamma-level 0` disables correction,
and an explicit flag overrides the shared `gamma_level` preference. F11 uses the
same cycle and HUD feedback as Ebiten. Gamma persists through save/load,
restart, map transitions and attract demos. Sky and UI artwork retain their
source colors, matching the main source-port path. Changing gamma updates
shader uniforms; geometry and normal texture uploads stay resident. Inverse
colormap and spectre palette images are cached per gamma bank and receive no
second correction in the world shader.

`-crt-effect` and F8 use the main renderer's curvature, animated scanlines, RGB
mask and vignette. The native GLSL shader reads a retained GPU snapshot after
MSAA resolve; it performs no CPU pixel readback, and disabling it allocates no
resources. The 35-Hz world clock freezes the effect during pause. Source-port
3D applies CRT before weapon, HUD, messages and menus, while the automap applies
it to the complete map presentation, matching Ebiten's order. Title artwork and
intermissions remain unprocessed. The shared `crt_effect` preference and save
state survive restart and map changes. Resize replaces only the scene snapshot.

The CRT shader is checked against actual Kage framebuffers at four resolutions
and six world-clock samples, including repeated paused time. All channels must
match within one level except RGB-mask phase changes at floating-point boundary
pixels: those must have equivalent color after adjusting the mask, lie within
0.001 source pixels of a boundary and occupy at most 0.01% of the frame. Built
launcher checks also require every HUD pixel to remain unchanged.

`-no-aspect-correction` and the shared preference are accepted. As in Ebiten's
source-port mode, this faithful-presentation option leaves native projection
unchanged; Doom's 1.2 geometry aspect correction stays active.

Native Raylib sound effects use the original event queue, priorities/budgets,
event selection and optional pitch math. DMX samples are loaded once at the
main game's 11025-Hz base rate, and independent sound aliases permit overlapping
effects. Listener movement updates attenuation/pan; pause/restart releases
voices. `-sound=false` starts SFX muted; `-sfx-volume` accepts 0–1 (default 0.5).
Capture runs remain silent, and an unavailable device produces a warning while
gameplay continues.

Native music uses the main version's ImpSynth/GENMIDI, MeltySynth/SoundFont and
PC-speaker synthesis through stereo Raylib audio streams. Map music, title,
intermission/finale tracks, episode 4 aliases, MUS volume compression and looping
share the existing engine. Two fixed-size buffers bound PCM memory; loop
boundaries join within a buffer. PC-speaker music shares the main tone reducer
and GoBeep86 speaker model; its tone sequence is retained, not a full PCM song.
Music pauses with P/Pause or focus loss and continues through menus.

F4 or Options → Sound changes SFX/music volume, synth and SoundFont. The music
player browses installed WADs, episode/map groups and other tracks using the
same catalog and song names as the main host. Left/right changes the selection;
Enter plays it, and Escape returns to Sound Options. Playing another WAD's music does
not change the current game. New games, map transitions and IDMUS retain their
normal music behavior. Synth changes restart the selected score while retaining
pause state. Invalid songs, SoundFonts or unavailable streams leave the previous
playback intact. Missing downloadable SoundFonts show progress while current
music continues. Local SoundFonts can be placed in `soundfonts/`.

`-music-backend` selects `auto`, `impsynth`, `pcspeaker` or `meltysynth`;
`-soundfont path.sf2` supplies MeltySynth's bank. `-mus-volume-compression` uses
the main launcher's shared note/controller compression (default 3; 1 disables;
effective range 1–8). Synth changes preserve this setting and rebuild from the
original score, so compression never compounds. This launch option is not saved
in preferences, matching the main launcher. `-mus-pan-max` controls MUS stereo
pan. `-pc-speaker-variant` uses `passthrough`, `paper-speaker` or `small-buzzer`.
`-music=false` starts muted; `-music-volume` and `-pc-speaker-volume` accept 0–1
and default to 1, matching Ebiten.
Backend, SoundFont, pan and volumes share the main version's saved settings.
`-pc-speaker` replaces digital effects with the WAD's compact DP tone sequences.
The shared main-version speaker player supplies the effect exclusions, audibility
rules and effect/music arbitration; one retained Raylib stream carries the mixed
speaker output. Effects replace one another on the single speaker instead of
creating digital aliases. Missing DP banks retain digital playback, matching the
main host. Menu navigation, confirm and back use the same sound selections.

Options → Raylib Options → PC Speaker Options changes effects, physical output volume and
speaker model while playing. These preferences use the common config fields.
Digital music volume stays independent of speaker effects; PC-speaker music and
effects share physical output gain. `-music=false` suppresses music without
muting effects. Pause/focus loss preserves buffered speaker audio and its source
cursors. Clearing effects on menu entry/restart retains music; stopping music or
switching back to a digital synth retains an ongoing speaker effect.
`-pc-speaker-interleave-hz` sets the shared tone-switch rate (10–1000 Hz,
default 280). `-pc-speaker-output linux` selects the shared Linux evdev speaker
backend; the default is `emulated`. Missing or inaccessible hardware falls back
to emulated output with the main launcher's diagnostic. Hardware output shares
the music/effect player, preserves cursors while paused, and needs no Raylib PCM
stream for its tones. The physical device has no software volume or coloration
control, matching the main backend. Capture runs remain silent.

Native saves use the main version's versioned, checksummed `.dsg` format and
`saves/quicksave.dsg` / `saves/dsgN.dsg` files, so either host can load the other's
saves when launched from the same working directory with the same WAD. F2/F3
open scrolling slot lists with map/health/time/date details and a screenshot
preview; existing numbered slots can be overwritten or a new slot added.
F6/F9 save/load the quick slot. Load also works from the title menu. Failed or
corrupt loads leave the active campaign intact; WAD mismatches use the main
version's warning behavior. Loading restores gameplay, automap exploration and
view, skill, inventory, moving sectors and random state, then resumes map music.
Saves are unavailable during intermissions or endings. Preview images are
captured before the menu overlay and fit within the main host's 320x320 limit.
A single replaceable GPU texture displays previews.

The native Doom menus use WAD artwork, the Doom font and animated skull cursor.
Core menus collect drawing commands from the same frontend used by Ebiten and
use its navigation state machine. The six main rows, episode/skill selection,
Read This pages, options, sound, binding editor, music player and save/load lists
share artwork, font placement, scrolling and cursor timing. Menus use square
pixels on a centered 320×200 layout; the world retains Doom’s 1.2 pixel aspect.
Options → Raylib Options holds filtering, lighting, the frame limit, fullscreen,
diagnostics, additional controls and PC-speaker settings. Quit uses the shared
Doom prompt and sound sequence: Y confirms, N/Space/Escape cancels and restores
the previous menu. Confirmation waits 53 Doom tics, independently of frame rate.
Settings feedback expires on the shared 35-Hz menu clock. Voice Options currently reports
that support is unavailable. Gameplay and accumulated input pause while a menu is
open; dismissing it does not fire or use a door with the same mouse click.
Normal launches start the shared title/credit/demo loop. Any key or a click opens
the main menu; Escape returns to the loop. Page timers and attract demos continue
behind its menus, as in the main host. New Game defaults to E1M1 or MAP01,
depending on the WAD; attract demos retain their own map/skill without changing
that selection. `-menu` opens the menu immediately. An explicit `-map E1M3`
launches directly into that level; add `-menu` to open its pause menu instead.
`-menu=false` skips the startup loop and launches
the first campaign level. Common sound/control preferences and `[keybinds]` use the main version's
`config.toml`; renderer preferences and window dimensions live in `[raylib]`.
Changes save atomically. The main host preserves `[raylib]` when saving its own
preferences. `-config PATH` selects a file; `-config=` disables loading/saving.
Explicit command-line options override saved defaults. The native launcher
continues to use the title loop unless `-map` is explicitly supplied, even if
`map` is present in the shared config. Batch capture runs read preferences but
do not write them.

Options → Raylib Options → Controls exposes mouse aiming/inversion, mouse and keyboard turn
speed, always-run, automatic weapon switching and camera smoothing. Mouse
sensitivity defaults to 0.5 and camera smoothing defaults on, matching Ebiten.
`-smooth-camera-yaw=false` disables smoothing, and the toggle shares the main
host's `smooth_camera_yaw` preference. Key Bindings scrolls through
the implemented actions: arrows select an action and primary/alternate slot,
Enter waits for a key or mouse button, Escape cancels capture, Backspace clears
a slot and F5 restores defaults. Duplicate assignments display the shared
conflict warning. Mouse clicks select either slot. P/R inspection shortcuts
cede to custom gameplay bindings. Mouse turn sensitivity uses the main host's
angular units per pixel. Alt+Enter also toggles fullscreen. Window size changes
save after resizing settles. Chat is remappable on this page; voice is excluded
from the native parity scope.

Normal and secret exits use the main version's WAD map routing. Intermissions
share its statistics counters, animation timing, par times, route markers,
artwork layout and skip delay. Raylib draws collected patch commands from that
layout without creating Ebiten images. Weapons, ammo, health and armor carry
into the next level; keys and temporary powerups clear through the shared
carryover rules. Restarting resets the player and current map while retaining
secret-level history. Starting New Game or warping begins a fresh campaign.

Episode endings use the shared finale text, tiled backdrop and ending picture,
then return to the title loop. Doom II story breaks retain the pending next map
and continue when the shared input delay permits a press. Map, intermission,
victory and commercial story music change with the session. Any key or a click
advances intermissions/endings; menus and pause stop their command tics.
New Game, restart, IDCLEV and entry into the next map use the main version's
35-Hz Doom melt: 160 virtual slices, matching delays/acceleration and cosmetic
RNG consumption. Two retained GPU snapshots compose the wipe through batched
texture slices; no framebuffer readback or pixel uploads occur during it.
Gameplay and command input wait for the melt to finish, and next-map music is
deferred until then. Focus loss freezes transition tics. Resizing reinitializes
the incoming snapshot while scaling the previous frame, matching the main
host's resize behavior. Saves load immediately. Entry into statistics/finale
screens remains immediate, as in the main host. Doom II cast presentation remains unimplemented in both hosts.

The textured automap shares the main renderer's exploration flags, line colors,
directed-side floor rasterizer, animated textures, sector lighting, thing glyphs
and sprite selection. Raylib uploads its reusable floor buffer into one texture;
subsequent frames update that texture, and window resizes replace its storage.
Map drawing does not discover new sectors or alter gameplay state.

In map view, WASD moves the player, Q/E turns, and Space uses doors/switches.
F toggles follow; arrows pan when follow is off. B or 0 toggles the fitted whole
map and restores the previous view on a second press. Plus/minus or the wheel
zooms; Home resets the view. M adds a numbered mark, C clears marks, G toggles
the grid, R changes heading-up rotation, O toggles allmap, I cycles IDDT, T cycles thing
glyph/item/sprite modes, and V shows the shared thing/line legend. `-automap`
starts a directly selected map in this view for inspection and captures.
G/R/O/I/T/V also change map presentation while the 3D view is active, so the
selected settings are ready when Tab opens the map. F5 controls detail in both views.

Type the classic cheats during gameplay: `IDDQD`, `IDKFA`, `IDFA`, `IDCLIP` or
`IDSPISPOPD`, `IDBEHOLD` plus V/S/I/R/A/L, `IDCHOPPERS`, and `IDMYPOS`.
`IDCLEV##` validates and loads a WAD map at the current skill. `IDMUS##` selects
a track through the shared main-version mapping; Doom II additionally supports
33–35 and 00 to stop playback. Invalid targets leave the current level/track
alone. P/R/number and map-letter shortcuts are suppressed during cheat typing.
`IDDT` cycles the original reveal state and updates the native automap.

Native demo playback accepts a Doom v1.9/v1.10 LMP file or a built-in `DEMO1`
through `DEMO4` lump with `-demo`. The demo header selects its map, skill and
gameplay rules, and playback exits at the end marker. Commands, pause/rebirth,
intermissions and map transitions use the shared ticker. `-demo-stop-after-tics`
and `-demo-exit-on-death` limit playback; `-trace-demo-state` writes the main
host's per-tic JSONL format, and completion prints its benchmark summary.

`-record-demo PATH` records live commands to a compatible v1.10 LMP. Both hosts
simulate the exact encoded command, including keyboard turns and weapon changes,
so recordings replay without angle rounding drift. Native recordings checkpoint
atomically every ten seconds and flush on quit. Level exits, restart, New Game,
warps and successful save loads freeze the recording; failed loads keep recording.
The header retains the starting map and gameplay options. Playback and recording
are mutually exclusive. Interactive command timing is 35 Hz; batch `-frames`
runs consume one demo/recording/title tic per frame for deterministic captures.

```bash
./build/raydoom -demo DEMO1
./build/raydoom -record-demo /tmp/playthrough.lmp
./build/raydoom -demo /tmp/playthrough.lmp -trace-demo-state /tmp/replay.jsonl
```

## Broadcast, watch and chat

The native launcher uses the main version's GDSF relay protocol and keyframe
format. Start the existing relay, publish a session, and use its printed session
ID to watch from either host:

```bash
go run ./cmd/gdsfrelay -listen 127.0.0.1:6670
./build/raydoom -map E1M3 -broadcast
./build/raydoom -watch -watch-session SESSION_ID
```

`-broadcast ADDRESS` and `-watch ADDRESS` select another relay; a hostname
without a port uses 6670. `-low-latency` flushes each broadcast command immediately.
Network sessions exclude demo playback, recording and tracing. The watcher
adopts the host's map, skill and simulation options and checks the same SHA1 WAD
fingerprint as the main launcher. Its local movement, cheats, New Game and
save/load controls are disabled; automap, presentation settings and chat remain
available.

Broadcasts include periodic snapshots every five seconds of gameplay. Late joins
load the newest snapshot and replay subsequent commands. Save loads, restarts,
new games and completed cheats publish mandatory snapshots for connected watchers. Intermission
advances follow the broadcaster's input. Network state stays in the shared
simulation; Raylib only supplies the window and drawing backend.

T opens chat by default. Enter sends, Escape cancels, and Backspace removes one
Unicode character. The main compose handler supplies normalization, its 160-rune
limit and duplicate/burst rejection. History is delivered by relay echo and uses
the same WAD font, wrapping, physical-pixel placement and expiration as Ebiten.
Chat polling continues while menus pause local gameplay. Voice is omitted.
The performance overlay also uses the main HUD font and layout. Frame and tic
rates come from the shared counters, with render time measured before Raylib's
frame limiter; the built-in green Raylib counter no longer overlaps chat.

Pickup, cheat and save messages and death prompts use the shared HUD layout
and the WAD's message font. Text measurement reads patch metadata without
allocating Ebiten images. Damage and pickup flashes use the same status counters
as the main host, with berserk priority and radiation-suit blinking. The native
overlay uses premultiplied alpha and covers the HUD as well as the world; messages
draw after the flash in 3D, matching the main overlay order.
The recording circle and REC label use the shared marker layout and disappear
when recording freezes on a level exit. The native standalone P/Pause control
uses M_PAUSE artwork, with a WAD-font fallback if the patch is absent.

## Parity stages

| Stage | Work | Status |
| --- | --- | --- |
| 1 | Visible combat, world objects/effects, weapons, classic HUD, sky, SFX, pause | Implemented and tested |
| 2 | Automap/minimap, menus/settings, keybindings, save/load, campaign progression and intermissions | Implemented and tested |
| 3 | Music/synth choices, demo playback/recording, network sessions and remaining presentation effects | All three music synths, SoundFonts, MUS compression, shared-catalog music player, emulated/Linux PC-speaker effects/music arbitration, demo playback/recording, title loop, broadcast/watch, chat and CRT implemented |

Main and browser entry points retain their existing backend. Shared cast
presentation remains unimplemented in both hosts. Voice is excluded from the
requested parity scope. The native host covers the main desktop source-port
gameplay and presentation features listed below; its 3D rasterization and texture
filtering intentionally follow this experiment's GPU settings.
Rendering-worker, CPU fast-path and faithful-resolution
settings belong to the Ebiten backend; the native host has its GPU-specific
filtering, antialiasing and frame-limit controls.

### Desktop source-port parity audit

| Feature group | Verification |
| --- | --- |
| Gameplay, controls, collision, combat, doors/lifts, pickups, thinkers, cheats | Shared simulation and random-sequence comparisons; live movement/fire and real door tests; keyboard/cheat regressions |
| Geometry, map exploration, sprites, weapons, HUD, sky | Directed-sidedef geometry tests; exit-room floor/map capture; real combat and spectre captures; sprite depth and HUD framebuffer tests |
| Lighting, powerups, gamma, fuzz, animated surfaces, CRT, antialiasing | Main-byte/pixel comparisons, light/powerup timers, depth and seam checks; all detail levels retain sharp 3D overlays |
| Title/credit/demo loop, menus, controls/bindings, music player, pause/quit | Shared frontend state/layout tests; main menu pixel comparison in an isolated child process; mouse-hit and keyboard tests |
| Save/load, previews, restarts, progression, intermissions, finales, melt | Cross-host saves and exact continuation; normal/secret route and Doom II story tests; GPU previews, transitions and main melt comparison |
| Digital SFX, music, synths, SoundFonts, PC speaker | Shared PCM and event policy; all three synths; native audio streams with pause/resume, reconfiguration and fallback tests |
| Demo playback/recording, broadcast/watch, chat | Main demo traces and replay checksums; actual local-relay launcher, late join, mandatory keyframes and chat lifecycle |
| IWAD/PWAD overlays, startup options, preferences, timing, HUD/detail shortcuts | Last-wins asset/geometry tests; launch/config precedence; shared speed and AUTO timing; persistent settings and real X11 keyboard checks |

Browser touch/pointer-lock handling and WASM deployment continue through the
existing Ebiten entry point. Main-only map inspection, asset export and Go
profiling commands remain available through `cmd/gddoom`; they are developer
tools rather than features of the native renderer. Native Raylib execution
requires cgo and a desktop graphics driver. Physical Linux PC-speaker hardware
has not been tested here; its routing/fallback is covered with a fake device.

Use `-width`, `-height`, `-fps` (0 means uncapped) and `-mode` to adjust the view.
VSync is enabled by default, as in the main host. `-no-vsync` disables VSync
and removes the default draw cap; an explicitly chosen or saved native `-fps`
limit still applies. The shared `no_vsync` preference is read and preserved.
The camera uses the main renderer's 90-degree horizontal FOV and 1.2 pixel
aspect. The projection uses the world viewport after reserving space for the
HUD, so a status bar or window resize does not narrow the horizontal view.

Normal resolution remains the native default. `-detail-level=0..3` chooses the
shared integer scene divisors 1, 2, 3 and 4; `-auto-detail=true` enables the main
host's 60-FPS policy, five-second worst-case sampling, sustained-load thresholds
and cooldowns. Both settings use the shared source-port preferences and survive
save loads, restarts and map transitions. AUTO never changes detail in map view.
The GPU renders the smaller scene into the window's MSAA framebuffer, resolves
that region into a retained texture and nearest-scales it before 3D overlays.
The full-resolution path allocates no detail texture; resident geometry stays
uploaded when detail changes. CRT and spectre snapshots use the scene dimensions.

## Seam antialiasing

The native window requests **4x MSAA** by default. Multisample coverage smooths
geometry silhouettes and boundaries between adjacent materials or sector
lights, while leaving texture interiors sharp. It works with the existing
depth buffer and resident meshes; no blur shader, extra draw pass or mesh
upload is needed. The window draws directly to its multisampled framebuffer.
Texture UVs use centroid interpolation to avoid sampling outside covered
triangles at partially covered pixels. Screenshot captures explicitly resolve
the window samples into a single image before reading them back.

Use `-msaa=false` for a comparison or to reduce framebuffer cost. The setting
is applied before window creation, so changing it requires restarting. Actual
sample availability depends on the graphics driver; this is Raylib's 4x MSAA
hint. Multisampling adds framebuffer storage and sample/depth work, with no
change to geometry or texture memory. Alpha-tested texture cutouts still use
the existing threshold; this pass smooths geometry coverage.

## Doom-style lighting

The default `-lighting doom` adds view-distance shading to the existing animated
sector lights. Walls and planes use their respective Doom light-table curves,
normalized to the original 320-wide basis so resolution changes do not change
the falloff. Horizontal walls have Doom's darker axis bias, vertical walls
its brighter bias, and diagonal walls no bias. Both sides of a wall keep the
same axis classification while retaining their own sector light.

The shader selects a continuous shade row per fragment using view-forward
depth and the main source-port renderer's row count and RGB brightness ramp.
WADs with exactly the active row count use their PLAYPAL/COLORMAP brightness;
WADs with extra special rows use the shared integer linear ramp fallback.
Interpolated multipliers round to 0..256, shaded channels truncate to bytes,
and the shared gamma table runs afterward. Filtering remains selectable.
Masked walls use sector shading without wall axis bias or distance shading;
sprites use the shared cutout distance factor multiplied by sector brightness.
Self-lit sprite frames stay fullbright. Masked alpha holes stay transparent.

```bash
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3 -lighting doom

# Comparisons; Shift+F9 also switches between these views while playing.
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3 -lighting sector
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3 -lighting fullbright
```

Flicker, strobe, glow and switch-driven changes use the original simulation
and interpolated sector-light cache. Sector light is carried in vertex colors;
static surface metadata uses the second UV attribute. Camera motion and Shift+F9
comparisons only change uniforms, without rebuilding GPU meshes or increasing
the texture batches. An active light-amplification pickup overrides distance
shading and retains its original expiration blink. Invulnerability remaps original
WAD indices through the main renderer's fixed COLORMAP row before texture
filtering. The inverse images are uploaded on first use and cached; blinking
only selects a resident texture and updates lighting uniforms. Geometry remains
resident. Duplicate palette colors retain their distinct original indices,
and masked alpha holes remain transparent. Arbitrary fixed-colormap selection
remains future work. The main engine's muzzle-flash light actions are currently
empty, so neither frontend adds world illumination from those actions.

The original references are id Software's [wall lighting and axis
bias](https://github.com/id-Software/DOOM/blob/master/linuxdoom-1.10/r_segs.c),
[plane lighting](https://github.com/id-Software/DOOM/blob/master/linuxdoom-1.10/r_plane.c)
and [light-table setup](https://github.com/id-Software/DOOM/blob/master/linuxdoom-1.10/r_main.c).

## Texture enlargement and filtering

The native launcher defaults to **2x texture dimensions with 8x anisotropic
filtering over a trilinear mip chain**. Enlargement duplicates each source pixel
into a 2x2 block before upload, so original colors and sharp nearby detail are
preserved. It adds no new artwork or detail. GPU UVs still use the original
dimensions: texture repeats, offsets and wall pegging remain the same size in
the world.

```bash
# Current default; anisotropic sampling retains detail at oblique angles.
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3 \
  -texture-scale 2 -texture-filter anisotropic

# Compare mipmapped trilinear sampling.
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3 \
  -texture-scale 2 -texture-filter trilinear

# Original unfiltered textures, for comparison.
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3 \
  -texture-scale 1 -texture-filter nearest
```

Scale accepts 1 or 2. Shift+F8 changes the filter without reuploading meshes or texture
images; mipmaps are generated once on the first filtered use. Nearest samples
only the base level, even after toggling from a mipmapped filter. Anisotropic
filtering depends on driver support; trilinear remains the base filter if the
requested anisotropy is unavailable. Raylib reports unsupported hardware or
limits in the terminal.

Filtered middle-wall cutouts use edge colors under transparent texels, with
wrap-aware propagation across repeat seams. Their original alpha remains
unchanged, so black transparent RGB cannot darken the visible edges. Masked and
opaque uses have separate GPU images when sharing a source texture. Mipmapped
alpha still uses the existing 0.5 cutoff; very thin cutouts can lose coverage
at a distance. Alpha-coverage-preserving mipmaps would be a further experiment.

Doubling both dimensions costs 4x the base texture storage; a full mip chain
adds roughly another third (about 5.3x the original base image for square
textures). Draw-call and geometry counts are unchanged.

## Animated textures and switches

The native host accepts `-texture-anim-crossfade-frames` and the shared
`texture_anim_crossfade_frames` preference. Its default is 7, matching the main
launcher; 0 disables blending. The shared animation resolver supplies the exact
eight-tic cadence, fractional render phase and switch shutter window. Floors,
ceilings and walls retain both resolved frames rather than dropping the next
frame from the mesh material.

Both frames are cached on the GPU. The shader changes an eight-bit blend weight;
changing that weight never uploads an image or rewrites a mesh buffer. Independent
switch timelines stay in separate batches, while sharing texture uploads.
Frame-pair changes can select another cached batch. Middle-wall animations retain
the union of both frames' cutout coverage, including holes common to both frames.
Invulnerability remaps each frame's original WAD indices before blending; gamma
correction applies once. Both frames use the selected texture filter and mipmaps.
The CPU mesh experiment also consumes the shared second-frame metadata.

## GPU storage

Triangles are grouped by resolved texture and masked/opaque status. Sector
lighting lives in vertex colors, so sectors sharing a texture share a draw
call. Mesh and texture buffers stay resident after upload. Each frame reuses
CPU staging arrays and compares positions, UVs and colors; only changed buffers
are sent to the GPU. Changing topology recreates the affected mesh. Animated
texture frames retain their own cached batches. Hardware depth and alpha
discard make transparent texture holes independent of draw order.

The inspected E1M3 exit-room view has 4,468 non-sky triangles in 73 batches.
Moving the camera alone requires no geometry upload. Sector-light animation
can update color buffers, and doors/lifts update positions. Mesh construction
and batch comparison still run on the CPU; this is not a measured hardware
speedup claim. Frustum/BSP visibility culling, indexed vertices and retaining
unchanged CPU geometry are useful follow-up optimizations.

Horizontal mesh topology uses the [same directed original linedef-side fill
as the CPU experiment](mesh-renderer-experiment.md). Its documented limitations
for open boundaries and rendering tricks also apply here.

## Validation and captures

```bash
go test ./internal/render/raymesh
go test ./internal/doomruntime -run '^TestNative'

# Actual main Update versus scaled native tics at eight simulation speeds.
go test ./internal/doomruntime -run '^TestNative(ScaledLiveSimulation|SimulationSpeed)' -v

# Source-port lighting bytes, gamma, sprites/masked mids and PWAD row counts.
GD_RAYLIB_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration \
  ./internal/doomruntime -run '^TestNativeDoomLightingMatchesMainSourcePortBytes$' -v

# Actual OpenGL depth, alpha discard and resident-buffer update regression.
GD_RAYLIB_INTEGRATION=1 go test -tags raylib,x11,integration \
  ./internal/render/raymesh -run '^TestRaylibResidentMeshDepthAndMask$' -v

# GPU lighting versus the existing independent wall/plane lighting functions.
GD_RAYLIB_INTEGRATION=1 go test -tags raylib,x11,integration \
  ./internal/doomruntime -run '^TestNativeMeshLightingMatchesDoom$' -v

# Multisampled shared seams, silhouettes and unchanged material interiors.
GD_RAYLIB_INTEGRATION=1 go test -tags raylib,x11,integration \
  ./internal/render/raymesh -run '^TestRaylibMultisampledSeams$' -v

# World depth and transparent holes for sprite cards in a separate draw pass.
GD_RAYLIB_INTEGRATION=1 go test -tags raylib,x11,integration \
  ./internal/render/raymesh -run '^TestRaylibSpritesShareWorldDepth$' -v

# Every blend weight reuses resident meshes; actual GPU checks cover cutout
# union, all filters, gamma and both fixed-palette frames.
GD_RAYLIB_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration \
  ./internal/render/raymesh -run '^TestRaylibTextureCrossfade' -v

# 128 GPU draws of a changing real E1M3 NUKAGE texel versus the main sampler.
GD_RAYLIB_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration \
  ./internal/doomruntime -run '^TestNativeAnimatedWADFlatMatchesMainBlendPixels$' -v

# Exact fuzz pixels across resolution, gamma, filtering, flip and inverse palette;
# retained resizing, ordered backgrounds, depth clipping and HUD reservation.
GD_RAYLIB_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration \
  ./internal/doomruntime -run '^TestNativeSpectreFuzz' -v
GD_RAYLIB_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration \
  ./internal/render/raymesh -run '^TestRaylibSpectreFuzz' -v

# Real E1M5 WAD masks, anisotropic scenery and MSAA; saves a framebuffer capture.
GD_RAYLIB_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration \
  ./cmd/raydoom -run '^TestNativeSpectreFuzzWithDoomAssets$' -v

# All channel gamma tables after lighting, plus fixed-palette bypass.
GD_RAYLIB_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration \
  ./internal/doomruntime -run '^TestNativeGammaGPU' -v

# Actual Kage/GLSL CRT pixels, retained snapshots, resize and sharp overlays.
GD_RAYLIB_CRT_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration \
  ./internal/doomruntime ./internal/render/raymesh \
  -run 'TestNativeCRTFramebuffers|TestRaylibCRT' -v
# Build first; every HUD pixel and source-port aspect must remain unchanged.
GD_RAYLIB_CRT_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration \
  ./cmd/raydoom -run '^TestNativeCRTLauncher' -v

# Real Ebiten and Raylib overlays: 48 framebuffer comparisons at three sizes.
# Each channel must match within one level, including the HUD area and glyphs.
GD_RAYLIB_OVERLAY_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration \
  ./internal/doomruntime -run '^TestNativeHUDOverlayFramebuffersMatchMain$' -v

# Built native launcher: saved three-tic demo, recording circle and WAD REC pixels.
GD_RAYLIB_OVERLAY_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration \
  ./cmd/raydoom -run '^TestNativeRecordingMarkerLauncher$' -v

# Requires an audio device (an ALSA null PCM also works for headless validation).
GD_RAYLIB_AUDIO_INTEGRATION=1 go test -tags raylib,x11,integration \
  ./cmd/raydoom -run '^TestNativeAudioLoadsDMXAndPlaysAliases$' -v

# Shared PC-speaker PCM, mixed effects/music, pause and synth switching.
GD_RAYLIB_AUDIO_INTEGRATION=1 xvfb-run -a go test \
  -tags raylib,x11,integration ./cmd/raydoom -run TestNativeSharedPCSpeaker

# Compression changes rebuild original scores; physical-output routing uses a
# simulated speaker, while evdev pause/close tests use files, never real hardware.
GD_RAYLIB_AUDIO_INTEGRATION=1 xvfb-run -a go test \
  -tags raylib,x11,integration ./cmd/raydoom \
  -run 'TestNativeMusic(ReconfiguresCompression|SharesPhysical)' -v
xvfb-run -a go test -race ./internal/audiofx -run '^TestLinuxPCSpeaker' -v

# Real relay late joins, shared snapshots/chat, exit and intermission replay.
xvfb-run -a go test ./internal/doomruntime \
  -run '^TestNative(Chat|Relay|Mandatory|WatchReplays)' -v

# Build first; exercise both native launcher roles and capture the watched map.
GD_RAYLIB_NETWORK_INTEGRATION=1 xvfb-run -a go test \
  -tags raylib,x11,integration ./cmd/raydoom -run TestNativeNetworkLauncher -v

# Ordered PWAD overrides, asset banks, player/cheat options and save metadata.
xvfb-run -a go test -tags raylib,x11 ./internal/launchcatalog ./cmd/raydoom \
  -run 'TestSharedWAD|TestNative(WADStack|Gameplay)' -v
GD_RAYLIB_WAD_INTEGRATION=1 xvfb-run -a go test \
  -tags raylib,x11,integration ./cmd/raydoom -run TestNativePWADLauncher -v
GD_RAYLIB_AUDIO_INTEGRATION=1 xvfb-run -a go test \
  -tags raylib,x11,integration ./cmd/raydoom -run TestNativePWADSoundAndMusic -v

# GPU melt frames versus the main compositor, resizing and a real map exit.
GD_RAYLIB_WIPE_INTEGRATION=1 xvfb-run -a go test \
  -tags raylib,x11,integration ./cmd/raydoom -run 'TestNativeWipe|TestNativeCampaignWipe'

# Controls and keybinding pages with real WAD artwork.
GD_RAYLIB_CONTROLS_INTEGRATION=1 xvfb-run -a go test \
  -tags raylib,x11,integration ./cmd/raydoom -run TestNativeControlsAndBindingMenusGPU

# Full built-in demo traces, recording round trips and title-loop parity.
xvfb-run -a go test -tags raylib,x11 ./internal/doomruntime \
  -run 'TestNativeDemo|TestNativeRecorded|TestNativeRecording|TestMainRecordedWalk|TestNativeAttract|TestNativeFrontend'

# Capture 120 demo tics after the 170-tic shareware title page.
./build/raydoom -config= -frames 290 \
  -capture build/raylib-captures/native-attract-demo1.png

# Save previews and restored gameplay through the actual GPU framebuffer.
GD_RAYLIB_SAVE_INTEGRATION=1 xvfb-run -a go test \
  -tags raylib,x11,integration ./cmd/raydoom -run TestNativeSavePreviewGPU

# Textured automap framebuffer, updates and resizing in the E1M3 exit room.
GD_RAYLIB_MAP_INTEGRATION=1 go test -tags raylib,x11,integration \
  ./cmd/raydoom -run '^TestNativeAutomapGPUFloorUpdatesAndResize$' -v

# Activate a real exit switch, render intermissions and enter the next level.
GD_RAYLIB_CAMPAIGN_INTEGRATION=1 go test -tags raylib,x11,integration \
  ./cmd/raydoom -run '^TestNativeCampaignIntermissionGPUAndNextLevel$' -v

# Reproduce the formerly missing E1M3 exit-room floor and ceiling.
./build/raydoom -wad DOOM1.WAD -map E1M3 \
  -camera=-600,-1600,89,0 -frames 3 \
  -capture build/raylib-captures/e1m3-exit-room.png
```

On headless Linux, prepend `xvfb-run -a` to windowed tests or captures. The
OpenGL regression checks a foreground cutout against a background wall,
reverses triangle submission order, verifies that unchanged geometry causes
zero uploads, then moves the foreground behind the wall and checks the changed
framebuffer. Native host tests cover fixed-tic movement, render-only simulation
checksums, destination-sector camera physics, opening a real E1M1 door and
changing floor heights.

Texture tests check exact 2x pixel/alpha duplication, unmodified source images
and repeating cutout edge colors. The OpenGL regression checks doubled GPU
image dimensions and mipmaps, exercises all three filters without mesh
uploads, verifies that cutouts retain foreground/background pixels without
dark fringes, and returns to the original nearest-filtered framebuffer.
A minified checkerboard framebuffer check verifies that trilinear and
anisotropic sampling use averaged mip levels, while nearest samples the
original black/white texels.

Lighting checks compare actual framebuffer pixels at three wall distances and
three wall orientations against the existing Doom functions. Pixel-center rays
verify the lighting of a real floor at three distances. They also cover
sector/fullbright comparisons, the powerup override, non-linear uploaded row
ramps and zero geometry uploads when switching lighting. Native host tests
check the WAD-derived ramp, animated E1M3 sector lights and the light-amp blink.
The source-port byte comparison renders all 256 palette samples across six
surface types, seven distances, six sector brightnesses, five gamma levels
and three COLORMAP row counts (3,780 GPU draws). It includes masked mids,
ordinary sprites and self-lit effects, checked against the shared main functions.
Timing tests compare 640 live updates with the actual main Update path at eight
speeds, including exact world tics, gameplay checksums and random sequences.
They check speed limits, shared HUD feedback, slow interpolation and exclusion
of demo/watch/intermission clocks. An isolated X11 keyboard smoke check also
confirmed persisted control/HUD toggles and a reduced world-tic count in the
actual native executable (175 normal versus 83 slowed tics over 300 frames).
Detail checks compare native scene dimensions with the actual main session
layout, exercise the shared AUTO sampler through render-frame counters and
preserve settings through save loads, restarts and level exits. GPU checks verify
cached MSAA resolves, exact nearest scaling of opaque and translucent pixels,
CRT ordering and sharp overlays. Launcher checks cover all four 3D detail levels,
the automap's enlarged HUD/CRT and real E1M5 spectres at each divisor. An isolated
keyboard check confirms R does not restart a live level, F5 works in map view and
the AUTO cycle persists to the shared configuration.

```bash
GD_RAYLIB_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration ./cmd/raydoom -run 'TestNative(Detail|SpectreFuzz)'
GD_RAYLIB_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration ./internal/render/raymesh -run TestRaylibReducedDetail
```

The MSAA framebuffer check draws two coplanar materials sharing a diagonal
edge. It verifies intermediate coverage at the seam and silhouette, no
background samples leaking through the shared edge, unchanged colors inside
both materials, and identical output after reversing submission order.

Native combat tests use real IWAD patches and cover monster/item extraction,
render-only checksum stability, collected-object removal, weapon visibility,
ammunition use and firing events, overlay timer decay, stereo orientation and
sound clipping. A movement-and-firing sequence produces the same simulation
checksum as the main engine's recorded-tic path. Sprite GPU tests check the
projected horizontal size as well as world depth across separate draw passes,
cutout holes, UV flips without mesh uploads and self-lit frames in dark sectors.
The audio check loads actual DMX samples into Raylib, starts overlapping aliases
and releases them on pause/restart; it validates the mixer path, not subjective
sound quality.
Invulnerability checks compare mapped pixels with the main renderer's COLORMAP
row, preserve duplicate indices and alpha, and verify expiration blinking and
IDDQD behavior. GPU checks exercise cached variants with all three texture
filters and confirm zero geometry uploads/updates during palette changes.
A real E1M1 capture verifies the inverse world and exact restoration after
IDBEHOLDV is toggled off.
An MSAA patch regression uses contrasting colors on all four edges at fractional
positions to check that no edge wraps onto its opposite. It also checks original
interior pixels, cached uploads and separate sampling state for repeating skies.

Native menu tests cover row navigation, mouse coordinates, bounded volume/filter
settings, quit confirmation, and map/skill selection. Cheat tests exercise shared
god/inventory/noclip/powerup actions, music callbacks, validated map warps and
conflicting shortcut suppression. Music checks compare PCM chunks directly with
the shared synthesizer, test loop boundaries and reject zero-duration scores.
The audio integration test exercises actual native streams, pause/resume, IDMUS
selection, mute and stop using an audio device or an ALSA null PCM.

Campaign tests compare intermission state against the main tick function,
exercise E1M3/E1M9 secret routing, verify inventory carryover and death restart,
and check episode endings plus Doom II story delays and secret-level returns.
The campaign GPU check activates E1M1's actual exit switch, captures statistics
and route artwork, loads E1M2 and verifies geometry, HUD and the raised weapon.

Music checks compare successive stereo PCM buffers with the main synths for
ImpSynth, MeltySynth and PC-speaker synthesis. Native audio integration exercises
switching, pause/resume, stop and invalid-font/track rollback. Menu framebuffer
checks capture sound settings and the shared-catalog track browser.

## Menu regression checks

The shared command comparison renders the same menu states through the Ebiten
frontend and native patch collector at 1280×800. It checks both skull frames,
binding capture, scrolling and faded text; nearest-sampling differences are
bounded to one raster pixel. Native GPU checks also cover renderer options,
Read This, pause and quit pages. Mouse-hit tests cover wide, tall and standard
window sizes.

Menu behavior regressions compare mouse sensitivity directly with Ebiten’s
WAD-dependent slider spacing, check singleton music selectors, preserve arbitrary
configured volume precision and verify quit-sound aliases with missing sounds.
Quit-delay tests keep rendering and repeated input independent of the tic clock.

```bash
GD_MENU_INTEGRATION=1 xvfb-run -a go test -tags integration ./internal/doomruntime -run TestNativeMenuRenderingMatchesEbiten
GD_RAYLIB_CONTROLS_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration ./cmd/raydoom -run TestNativeControlsAndBindingMenusGPU
GD_RAYLIB_INTEGRATION=1 xvfb-run -a go test -tags raylib,x11,integration ./cmd/raydoom -run TestNativeInvulnerabilityGPU
```
