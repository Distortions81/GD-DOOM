# Demo Desync Status

Current stock-demo status after fixing `DOOM1 demo1` and `DOOM1 demo2`.

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

- Status: desync
- Current first mismatch: `line=402`
- Path: `root.mobjs[266].floorz`
- Reference: `3145728`
- GD-DOOM: `3670016`

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
