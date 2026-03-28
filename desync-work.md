# Demo Desync Status

Current stock-demo status after fixing all stock `DOOM1` demos.

## DOOM1

### demo1

- Status: clean
- Result: `traces match lines=5026`
- Fixed in: `79ea1c5`
- Notes: fixed a same-tic lost-target reacquire / `JUSTATTACKED` chase parity issue on `E1M5`.

### demo2

- Status: clean
- Result: `traces match lines=3836`
- Fixed in: `3ab5506`, `9570e96`
- Notes: fixed the dropped-clip support parity issue first, then the remaining drop/chase parity causing the late target mismatch.

### demo3

- Status: clean
- Result: `traces match lines=2134`
- Fixed in: working tree
- Notes: fixed resumed just-attacked lost-target chase parity for non-zombieman monsters, which removed the live `demo3` shotgun guy desync on `E1M7`.

## DOOM2

### demo1

- Status: clean
- Result: `traces match lines=1205`

### demo2

- Status: desync
- Current first mismatch: `line=1088`
- Path: `root.mobj_count`
- Reference: `187`
- GD-DOOM: `186`

### demo3

- Status: desync
- Current first mismatch: `line=253`
- Path: `root.mobjs[266].z`
- Reference: `-16777216`
- GD-DOOM: `-17301504`
