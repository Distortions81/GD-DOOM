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
- Current first mismatch: `line=3756`
- Path: `root.mobjs[238].angle`
- Reference: `536870912`
- GD-DOOM: `1073741824`
- Notes: fixed the earlier dropped-clip support/ceiling mismatch at `line=1726`.

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
