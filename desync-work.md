# Demo Desync Status

Current stock-demo status after the recent `doom-source` parity work on monster walk-special handling and lost-soul death behavior.

## DOOM1

### demo1

- Status: playback terminates cleanly
- Result: direct playback exits at `tics=4973` with `player_dead=true`
- Notes: the full JSON trace compare for this demo is still expensive enough to be awkward for iteration, but the runtime now completes headless demo playback cleanly with `-demo-exit-on-death`.

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
- Current first mismatch: `line=1811`
- Path: `root.mobjs[84].movecount`
- Reference: `2`
- GD-DOOM: `0`
- Notes:
- Earlier blockers now fixed in the working tree:
- false Cacodemon teleport on MAP26 line `555` caused by monster walk-special full-scan fallback instead of Doom `spechit` candidates
- dead lost soul gravity mismatch caused by clearing effective `MF_NOGRAVITY` on `MT_SKULL` deaths
- dead lost soul linger mismatch caused by removing non-corpse deaths one tic late instead of on final-frame expiry
- Current remaining lead is later and narrower: a monster chase / `movecount` divergence, not teleport or lost-soul death cleanup.

## Next Issue

Highest-signal next runtime issue to investigate:

- identify `root.mobjs[84]` at `DOOM2-DEMO3` `line=1811` / `gametic≈1810` and compare its `movecount` / chase progression directly against `../doom-source`
- likely code area: ordinary monster chase movement ordering in `monsters.go`, especially the exact point where `movecount` is decremented/reset around successful `P_Move`-style movement
- reason: the current first mismatch is now a pure chase-state scalar mismatch after removing the earlier teleport and lost-soul death parity bugs

Secondary lead:

- once the `movecount` divergence is identified, rerun `DOOM2-DEMO3` to see whether the next issue stays in chase logic or shifts to another actor family
