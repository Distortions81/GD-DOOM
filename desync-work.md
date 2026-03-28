# Demo Desync Status

Current stock-demo status after fixing `DOOM1 demo1`.

## DOOM1

### demo1

- Status: clean
- Result: `traces match lines=5026`
- Fixed in: `79ea1c5`
- Notes: fixed a same-tic lost-target reacquire / `JUSTATTACKED` chase parity issue on `E1M5`.

### demo2

- Status: desync
- Current first mismatch: `line=1726`
- Path: `root.mobjs[294].ceilingz`
- Reference: `15204352`
- GD-DOOM: `11534336`

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
