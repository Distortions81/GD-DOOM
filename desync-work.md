# Demo Desync Status

Current stock-demo status as of the latest sweep and `DOOM1 demo1` follow-up.

## DOOM1

### demo1

- Status: desync
- Current first mismatch: `line=2525`
- Path: `root.mobjs[23].angle`
- Reference: `3758096384`
- GD-DOOM: `3221225472`
- Notes: the earlier dropped-shotgun support-state mismatch at `line=2440` is fixed. Current investigation is on a zombieman chase/retarget divergence on `E1M5`.

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
