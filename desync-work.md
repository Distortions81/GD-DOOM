# Demo Desync Status

Current stock-demo status after the recent `doom-source` parity work on monster walk-special handling and lost-soul death behavior.

## DOOM1

### demo1

- Status: clean
- Result: `traces match lines=4973`
- Notes: fixed the sector-71 normal-door thinker lifetime mismatch at `gametic 306`; the close-complete path now defers thinker retirement by one prune cycle, while open-only door completions still remove immediately.

### demo2

- Status: clean
- Result: `traces match lines=3778`
- Fixed in: `3ab5506`, `9570e96`
- Notes: still clean after the current chase/harness changes.

### demo3

- Status: clean
- Result: `traces match lines=2079`
- Fixed in: `2adb2c6`
- Notes: still clean after the later `DOOM2-DEMO3` fixes.

## DOOM2

### demo1

- Status: clean
- Result: `traces match lines=1205`

### demo2

- Status: clean
- Result: `traces match lines=1949`
- Fixed in: `2adb2c6`
- Notes: still clean after the current monster walk-special and lost-soul death fixes.

### demo3

- Status: desync
- Current first mismatch: `line=2802` (`gametic 2801`)
- Path: `root.mobj_count`
- Reference: `245`
- GD-DOOM: `244`
- Notes:
- Earlier blockers now fixed in the working tree:
- false Cacodemon teleport on MAP26 line `555` caused by monster walk-special full-scan fallback instead of Doom `spechit` candidates
- dead lost soul gravity mismatch caused by clearing effective `MF_NOGRAVITY` on `MT_SKULL` deaths
- dead lost soul linger mismatch caused by removing non-corpse deaths one tic late instead of on final-frame expiry
- Mancubus chase / post-attack reacquire divergence at `gametic≈1810` is now fixed:
- dead-target cleanup for removed non-corpse thinkers now clears or reassigns monster explicit targets
- resumed-from-attack `A_Chase` reacquire now takes the `MF_JUSTATTACKED` chase-dir branch on the same tic when Doom does
- Mancubus demo-trace state numbers now match Doom's actual `S_FATT_*` enum values (`RUN1=364`, `ATK1=376`, `PAIN=386`, `DIE1=388`)
- lost-soul skull-fly zero-XY handling now falls through to the same-tic normal Z/support path, and `resetLostSoulCharge` now clears `thingInFloat`
- the later sector-74 blaze-close door linger was a stale full-trace artifact after cleanup; a clean rerun now matches the reference through that window
- Current remaining lead is a single missing decorative mobj:
- reference keeps green torch `type=45` at `x=33554432`, `y=46137344`, `z=-4718592` through `gametic 2805`
- GD-DOOM still has it at `gametic 2800` and drops it at `gametic 2801`
- trace debug maps that torch to map thing `idx=64` in sector `65`
- Fresh replay against the archived reference logs still looks gameplay-clean by eye; no new stock-demo desyncs were observed during the rerun.

## Next Issue

Highest-signal next runtime issue to investigate:

- identify what path marks decorative thing `idx=64` as removed one tic early during the sector-65 movement around `gametic 2801`
- likely code area: generic thing support / height-clipping or any path that flips `thingCollected[idx]` for non-shootable decorations during moving-sector updates
- reason: the first remaining mismatch is no longer monster AI or door timing; it is a single object-lifecycle divergence on a static map thing

Secondary lead:

- confirm whether Doom keeps the torch as a normal decorative thinker while the local runtime is incorrectly treating it as removed, collected, or clipped out of the trace
