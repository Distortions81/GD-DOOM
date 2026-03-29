# Demo Desync Status

Current stock-demo status after moving zombieman, shotgun guy, and chaingunner onto an exact Doom-style state-machine path modeled on `P_MobjThinker` / `P_SetMobjState` in `../doom-source/linuxdoom-1.10/p_mobj.c` and the corresponding `info.c` state tables and `p_enemy.c` actions.

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
- Fixed in: working tree
- Notes: the earlier zombie chase/state mismatch was resolved by the exact-state-machine refactor for hitscanners.

## DOOM2

### demo1

- Status: clean
- Result: `traces match lines=1205`

### demo2

- Status: clean
- Result: `traces match lines=1949`
- Fixed in: working tree
- Notes: still clean after replacing the old helper-driven hitscanner flow with exact Doom state transitions.

### demo3

- Status: desync
- Current first mismatch: `line=1022`
- Path: `root.mobjs[268].ceilingz`
- Reference: `-3670016`
- GD-DOOM: `-8388608`
- Notes: this first mismatch is transient FX (`type 37`). With transient FX ignored, the next real mismatch is `line=1260`, `root.mobjs[8].momz`, where a lost soul has `0` in reference vs `-415061` in GD-DOOM.

## Next Issue

Highest-signal next runtime issue to investigate:

- extend the exact Doom state-machine treatment beyond hitscanners
- next likely family: projectile attackers, starting with imp / projectile-driven `A_Chase` and missile-state transitions
- reason: the hitscanner slice now matches again, and further demo work should follow `../doom-source` state execution order instead of adding more helper-level parity patches

Secondary issue after that:

- `DOOM2-DEMO3` non-FX first gameplay mismatch at `line=1260`, `root.mobjs[8].momz`
- actor: lost soul
- reference `momz=0`
- GD-DOOM `momz=-415061`
- implication: likely lost soul / skull-fly vertical momentum parity in `p_mobj.c` / `A_SkullAttack` related paths, separate from the hitscanner state-machine work.
