# Future Plans

This document is for concrete future work that already has a basis in the current GD-DOOM codebase.

It is not a vague wishlist. Each item below is tied to something that already exists:

- implemented flags that are only partially surfaced
- standalone tools that could become better product features
- protocol/runtime code that points to the next stage
- active technical work already documented elsewhere in the repo

## How To Use This Doc

- Add items only when they are specific enough to act on.
- Prefer "build X" over "improve X".
- If an item depends on another system, say so directly.
- Move finished items into release notes or a changelog, not back into this file.

## Current Direction

The repo already has five strong pillars:

1. Vanilla-compatible gameplay and demo behavior.
2. A smoother source-port presentation path.
3. Browser and mobile play.
4. Relay-backed watch, chat, and voice sessions.
5. Power-user tooling around WADs, maps, demos, traces, and audio.

The best future work is the work that sharpens those pillars instead of adding random side features.

## Near-Term Backlog

### 1. Demo Parity And Desync Reduction

- Finish the remaining `DOOM2 demo3` desync work tracked in [desync-work.md](/home/dist/github/GD-DOOM/desync-work.md).
- Turn the trace-compare flow into a standard regression check built around [scripts/demo_trace_compare.sh](/home/dist/github/GD-DOOM/scripts/demo_trace_compare.sh).
- Add an easier summary mode for trace diffs so the first mismatch is readable without digging through full JSONL.
- Expand parity coverage beyond the built-in demos to selected recorded demos already stored under [demos](/home/dist/github/GD-DOOM/demos).

Why this is concrete:
- The harness already exists.
- The repo already documents a specific active mismatch.
- Demo compatibility is one of the project's strongest differentiators.

### 2. Promote Existing Debug Tools Into Real Workflows

- Expand [cmd/wadtool/main.go](/home/dist/github/GD-DOOM/cmd/wadtool/main.go) beyond `extract-lump` into a general WAD inspection utility.
- Add structured output modes to [cmd/mapprobe/main.go](/home/dist/github/GD-DOOM/cmd/mapprobe/main.go) so it is easier to use in scripts.
- Convert [cmd/mapaudit/main.go](/home/dist/github/GD-DOOM/cmd/mapaudit/main.go) from a fixed local report generator into a reusable auditing tool with explicit inputs and output formats.
- Decide whether [cmd/tmpinspect/main.go](/home/dist/github/GD-DOOM/cmd/tmpinspect/main.go) should become a supported tool or be folded into `mapprobe`.

Why this is concrete:
- These tools already exist.
- They solve real investigation problems.
- They currently feel more like internal utilities than a coherent toolkit.

### 3. Make Advanced Runtime Features Easier To Discover

- Group the many existing startup flags in [internal/app/run_parse.go](/home/dist/github/GD-DOOM/internal/app/run_parse.go) into clearer user-facing feature sets.
- Expose more implemented audio, rendering, and control options in-game instead of requiring startup flags.
- Decide which debugging and profiling flags should stay developer-only and which deserve better documentation.
- Create a short "advanced usage" doc for features that are implemented but easy to miss.

Concrete examples already in code:
- `-trace-demo-state`
- `-dump-music`
- `-texture-anim-crossfade-frames`
- `-mic-device`
- `-pc-speaker-variant`
- `-pc-speaker-output`

### 4. Browser And Mobile Usability Pass

- Tighten the browser IWAD picker and local WAD loading flow.
- Improve browser save/load resilience around `localStorage`.
- Refine touch control defaults and make mobile setup less trial-and-error.
- Separate "browser limitations" from ordinary gameplay settings so the web build feels intentional rather than merely constrained.

Why this is concrete:
- The browser build already exists.
- The README already describes touch controls, save previews, and browser-specific behavior.
- Browser support is a core product surface, not a side experiment.

### 5. Watch / Chat / Voice Polish

- Improve relay session UX around session discovery, metadata, and join flow.
- Make watcher-facing information clearer during connection, waiting, and failure states.
- Tighten microphone configuration and diagnostics around device selection, codec choice, gate, and AGC behavior.
- Decide which relay features should stay "watch a broadcaster" and which should move toward true multi-player session behavior.

Why this is concrete:
- Broadcast/watch/chat/voice are already implemented.
- [netplay-protocol.md](/home/dist/github/GD-DOOM/netplay-protocol.md) already describes a larger next protocol stage.

## Mid-Term Concepts With A Real Code Basis

### 6. Peer Multiplayer From Existing Lockstep Foundations

- Build on [internal/netplay/lockstep.go](/home/dist/github/GD-DOOM/internal/netplay/lockstep.go) and the peer protocol sections in [netplay-protocol.md](/home/dist/github/GD-DOOM/netplay-protocol.md).
- Define the minimum viable feature set for a real peer session:
  player join
  roster updates
  tic exchange
  checkpoints
  desync recovery expectations
- Keep the scope narrow enough that it does not derail demo parity or single-player stability.

This belongs here because:
- The code already contains a lockstep coordinator.
- The protocol doc already names peer-only frames.
- The repo is past the "pure idea" stage for this work.

### 7. Replay Review Mode

- Reuse existing demo playback and trace export support to build a more deliberate replay-analysis workflow.
- Focus on features that help investigation:
  tic stepping
  event summaries
  map/thing inspection overlays
  trace mismatch context
- Keep it useful for both demo parity work and player-facing replay viewing.

This belongs here because:
- Demo playback and per-tic trace export already exist.
- Tooling like `mapprobe` suggests the project already values detailed inspection.

### 8. Better Music / PC Speaker Comparison Tooling

- Unify the music export path in [cmd/musicwav/main.go](/home/dist/github/GD-DOOM/cmd/musicwav/main.go) with the PC speaker capture utilities in [cmd/pcspeaker/main.go](/home/dist/github/GD-DOOM/cmd/pcspeaker/main.go).
- Make it easier to compare `impsynth`, `meltysynth`, and PC speaker render paths for the same song.
- Decide whether these should stay developer tools or become a documented audio workflow.

This belongs here because:
- The codebase already contains the pieces.
- Audio is clearly a distinctive part of the project.

## Concrete Cleanup Candidates

- Create a single docs index or `docs/` directory once root-level notes become harder to scan.
- Separate "active investigations" from "long-term roadmap" so desync work and future product work do not mix.
- Add a supported-tools section to the README covering `mapprobe`, `wadtool`, `mapaudit`, `musicwav`, `pcspeaker`, and `demotracecmp`.
- Decide which experimental flags should be renamed, hidden, or promoted.

## Not Worth Prioritizing Right Now

- Cosmetic UI churn without a usability reason.
- Large feature ideas with no code path started.
- More niche flags unless they simplify an existing workflow.
- New systems that compete with the current pillars instead of strengthening them.
