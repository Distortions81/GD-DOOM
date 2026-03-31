# Demo Desync Status

Current stock-demo status after the recent `doom-source` parity work on monster walk-special handling and lost-soul death behavior.

Fresh no-render playback rerun on `2026-03-30`:

- `DOOM1 demo1`: completed, `tics=5026`, `map=E1M5`, `player_dead=true`
- `DOOM1 demo2`: completed, `tics=3836`, `map=E1M3`, `player_dead=true`
- `DOOM1 demo3`: completed, `tics=2134`, `map=E1M7`, `player_dead=true`
- `DOOM2 demo1`: completed, `tics=1205`, `map=MAP11`, `player_dead=true`
- `DOOM2 demo2`: completed, `tics=2001`, `map=MAP05`, `player_dead=true`
- `DOOM2 demo3`: completed, `tics=4471`, `map=MAP26`, `player_dead=true`

Those fresh runs used GD-DOOM's `-render=false` demo path only. The trace-compare sections below still reflect the latest full `doom-source` comparison runs, not a same-turn full retrace of all six demos.

## DOOM1

### demo1

- Status: clean
- Result: `traces match lines=4973`
- Fresh no-render replay: `completed tics=5026 map=E1M5`
- Notes: fixed the sector-71 normal-door thinker lifetime mismatch at `gametic 306`; the close-complete path now defers thinker retirement by one prune cycle, while open-only door completions still remove immediately.

### demo2

- Status: clean
- Result: `traces match lines=3778`
- Fresh no-render replay: `completed tics=3836 map=E1M3`
- Fixed in: `3ab5506`, `9570e96`
- Notes: still clean after the current chase/harness changes.

### demo3

- Status: clean
- Result: `traces match lines=2079`
- Fresh no-render replay: `completed tics=2134 map=E1M7`
- Fixed in: `2adb2c6`
- Notes: still clean after the later `DOOM2-DEMO3` fixes.

## DOOM2

### demo1

- Status: clean
- Result: `traces match lines=1063`
- Fresh no-render replay: `completed tics=1205 map=MAP11`
- Notes: fixed a real hitscan parity bug at `gametic 791`. A shotgun pellet was incorrectly seeing an imp through a blocking line because `doomPointOnDivlineSide` did not match Doom's shifted fixed-point side test in `PIT_AddThingIntercepts`. The fresh rerun is now fully trace-clean.

### demo2

- Status: clean
- Result: `traces match lines=1949`
- Fresh no-render replay: `completed tics=2001 map=MAP05`
- Fixed in: `2adb2c6`
- Notes: still clean after the current monster walk-special and lost-soul death fixes.

### demo3

- Status: desync
- Fresh no-render replay: `completed tics=4471 map=MAP26`
- Earlier note about a missing sector-65 green torch at `gametic 2801` was stale.
- Fresh 2026-03-30 trace debugging on the current tree found a different earlier issue first.
- Notes:
- Earlier blockers now fixed in the working tree:
- `sector 74` blaze-close door linger is fixed; the remaining mismatch is no longer a door thinker lifetime issue
- false Cacodemon teleport on MAP26 line `555` caused by monster walk-special full-scan fallback instead of Doom `spechit` candidates
- dead lost soul gravity mismatch caused by clearing effective `MF_NOGRAVITY` on `MT_SKULL` deaths
- dead lost soul linger mismatch caused by removing non-corpse deaths one tic late instead of on final-frame expiry
- Mancubus chase / post-attack reacquire divergence at `gametic≈1810` is now fixed:
- dead-target cleanup for removed non-corpse thinkers now clears or reassigns monster explicit targets
- resumed-from-attack `A_Chase` reacquire now takes the `MF_JUSTATTACKED` chase-dir branch on the same tic when Doom does
- Mancubus demo-trace state numbers now match Doom's actual `S_FATT_*` enum values (`RUN1=364`, `ATK1=376`, `PAIN=386`, `DIE1=388`)
- lost-soul skull-fly zero-XY handling now falls through to the same-tic normal Z/support path, and `resetLostSoulCharge` now clears `thingInFloat`
- Fresh current lead and fix:
- fresh short compare/debug run found a false teleport for Cacodemon `actor_idx=151` on MAP26 line `555` at `tic=1539` (`gametic 1538`)
- the actor's successful move probe touched no special lines on the second step, but GD-DOOM passed `nil` candidates and `checkWalkSpecialLinesForActorWithCandidates` fell back to a full line scan
- that full-scan path incorrectly crossed repeat teleport line `555`, spawning two `MT_TFOG` mobjs and moving the Cacodemon into sector `78`; Doom keeps the actor in sector `79` there
- fixed locally by making monster move probes and skull-fly probes return an explicit empty candidate slice instead of `nil`, preserving the "no fallback to full scan" contract already covered by unit tests
- targeted rebuilt debug rerun confirms the bogus line-555 teleport no longer fires in the `1538-1540` window
- full fresh end-to-end `doom-source` compare for the entire demo is still pending after this fix; `DOOM2 demo1` and `DOOM2 demo2` remain clean in the latest recorded full compares

## Next Issue

Highest-signal next runtime issue to investigate:

- rerun the full reference compare for `DOOM2 demo3` after the line-555 teleport candidate fix to locate the next real first mismatch
- reason: the previously documented sector-65 decoration mismatch was stale, and the fresh current first actionable issue was an earlier monster teleport false positive that is now fixed locally

Secondary lead:

- if another MAP26 desync remains after the fresh full compare, re-check whether it is still in the teleporter room cluster or whether the first mismatch has moved elsewhere in the run
