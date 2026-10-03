# Rendering experiments

An idea board for the `experiments` branch. These are proposed effects, not
implemented features. Effort is relative: **small** uses the existing frame or
shader inputs; **medium** adds masks, history, or draw metadata; **large** needs
a new geometry or visibility output.

## Quick visual wins

| Experiment | What it looks like | First implementation | Effort |
| --- | --- | --- | --- |
| **Comic-book carnage** | Heavy black ink, a handful of color bands, and halftone shadows. Doom as a moving comic panel. | Quantize colors and find image edges in a presentation shader; add a screen-space dot pattern. | Small |
| **Pocket Doom** | A tiny green handheld display, four shades, chunky pixels, and an optional red monochrome variant. | Downsample before palette mapping; make the logical resolution and palette selectable. | Small |
| **Dither cathedral** | Blue-noise or ordered stipple replaces smooth lighting, with selectable EGA, CGA, and Doom palettes. | Start with ordered dithering at a fixed logical resolution. Compare screen-anchored and texture-anchored patterns. | Small |
| **VHS from hell** | Tape wobble, chroma bleed, head-switch noise, and an occasional tracking slip. | A presentation shader with a repeatable time-based distortion; build on the existing CRT pass. | Small |
| **Infrared imp hunt** | Cold blue rooms, bright orange creatures, and white-hot projectiles. | First try a luminance-based false-color shader. A later version adds sprite/material masks so brightness does not stand in for heat. | Small to medium |

## Effects with a little more machinery

| Experiment | What it looks like | First implementation | Effort |
| --- | --- | --- | --- |
| **Phosphor afterlife** | Green or amber light trails linger behind rockets and moving silhouettes. | Retain a previous-frame image and blend with controlled decay. Begin with bright pixels; add object masks for selective trails. | Medium |
| **Glass spectres** | Invisible monsters bend the scene like moving glass, with a faint shimmering rim. | Replace fuzz sampling with bounded background offsets inside the existing spectre mask and ordered background snapshots. | Medium |
| **Lava breath** | Hot floors shimmer and distort the lower part of the room. | Prototype distortion over visible damaging-floor pixels. Broader heat haze needs a separate mask and background composition. | Medium |
| **Rain on Mars** | Rain streaks pass outside windows, lightning flashes through courtyards, and wet surfaces gleam. | Start with sky-masked rain and cosmetic lightning. Treat wet-floor reflections as a separate, larger experiment. | Medium |
| **Paper-cut hell** | Sprites and walls become layers of printed card, with ink outlines and offset paper shadows. | Start with sprite masks and a small displaced silhouette shadow; add paper grain in texture coordinates. | Medium |
| **Bad console port, beautifully** | Low color precision, coarse texture coordinates, and deliberate projection jitter. | Quantize projected boundaries and texture sampling in an optional draw-command path. Keep the distortion adjustable. | Medium |
| **Acid automap** | The textured automap pulses with neon boundaries, projectile streaks, and fading paths. | Reuse map rendering, add a history layer for trails, and apply glow. Pulse from presentation time first; music-driven pulses need an audio signal. | Medium |

## Bigger, stranger experiments

| Experiment | What it looks like | First implementation | Effort |
| --- | --- | --- | --- |
| **Oscilloscope Doom** | Bright vector edges and sparse world-aligned hatching, with phosphor bloom and persistence. | Export accepted wall fragments and plane ownership from Doom visibility. Follow the existing vector visibility notes. | Large |
| **ASCII dungeon** | The scene becomes animated text: `@` monsters, dense glyph shadows, and colored punctuation fireballs. | Begin with a screen-space glyph atlas selected by cell luminance and edge direction. A semantic version needs object IDs. | Medium to large |
| **Sonar nightmare** | A scanning pulse reveals wire outlines that fade back into darkness. | A screen-space sweep is the teaser. A world-distance pulse needs visible-surface position or depth data. | Large |
| **The renderer builds the world** | Watch each frame emerge: BSP visits, wall columns, floor spans, sprites, then light. | Capture actual visibility and draw events, freeze a frame, and reveal the recorded stages with a scrubber. | Large |
| **Miniature massacre** | Rooms feel like a tiny model, with a narrow focus band and softened distant scenery. | Begin with a screen-space tilt-shift approximation. Correct scene-depth focus needs a composed depth image, including floors and sprites. | Medium to large |

## First round

1. **Comic-book carnage:** a strong visual change with one presentation shader.
2. **Phosphor afterlife:** introduces reusable frame history for several later effects.
3. **Glass spectres:** makes use of the engine's existing ordered spectre snapshots.

After those, **oscilloscope Doom** is the most distinctive larger project.

## Trying an experiment

Expose each effect as an opt-in preset with a strength control and an immediate
before/after toggle. Start by applying effects to the world view, with a separate
option for the weapon and HUD. Keep gameplay state and gameplay RNG untouched;
drive animation from a separate presentation clock or deterministic demo time.

Use the same demo and camera for comparisons. Collect stills and short moving
clips: E1M1 outdoors, a dark interior, a close-up spectre, overlapping masked
textures, and a busy Doom II fight. Check 320x200, 1080p, and browser output;
measure whole-frame time as well as individual shader cost. Clear history on
map changes, resolution changes, and preset switches.

The existing renderer has indexed atlases and COLORMAP lookup, CPU visibility
decisions, ordered sprite/spectre drawing, and a CRT presentation shader. A
complete GPU depth, normal, or material buffer would be new work; proposals
that require one should budget for it explicitly.

## Starting points in the code

- `internal/doomruntime/shaders/crt_post.kage`: existing presentation effect.
- `internal/doomruntime/shaders/world_indexed.kage`: indexed shading and spectre sampling.
- `internal/doomruntime/gpu_renderer.go`: draw commands, atlases, and background snapshots.
- `internal/doomruntime/game.go`: world visibility and frame composition.
- `internal/render/mapview/render2d.go`: automap rendering.
- [GPU renderer checks](gpu-renderer-performance.md): comparisons and performance measurements.
- [Vector visibility notes](vector-rendering-visibility.md): clipping and plane ownership for vector experiments.
