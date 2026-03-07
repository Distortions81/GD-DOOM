# Map Audit

Generated from local IWADs with `doom-source` as the behavior reference.

This report only lists map-data oddities that are either malformed, not meaningful to original Doom, or risky for parity work.

Notes:
- In vanilla Doom, a non-player thing with no skill bits set does not spawn. This is not harmless data.
- Linedef special `48` is tracked separately because it is a wall-scroll/render effect, not a gameplay trigger.
- Unknown thing flag bits means map data outside the normal Doom thing-option mask.

## doom.wad

Maps scanned: 36

### Summary

| Category | Count | Why it matters | Examples |
| --- | ---: | --- | --- |
| Unknown linedef specials | 1 | Likely unsupported gameplay triggers or malformed data. | E2M7#1125 |
| Linedef special 48 | 147 | Known wall-scroll special tracked separately from gameplay triggers. | E1M1#348 (scroll wall), E1M1#349 (scroll wall), E1M1#350 (scroll wall), E1M1#351 (scroll wall), E1M1#352 (scroll wall), E1M1#353 (scroll wall), E1M1#354 (scroll wall), E1M1#355 (scroll wall), E1M7#925 (scroll wall), E1M7#926 (scroll wall), E1M7#927 (scroll wall), E1M7#928 (scroll wall) |
| Unknown sector specials | 0 | Sector behavior not recognized by current runtime. | none |
| Things with no skill bits | 63 | Vanilla Doom will not spawn these non-player things. | E1M4#16 (type=2008), E1M4#48 (type=2008), E1M7#112 (type=2035), E1M7#113 (type=2035), E1M7#114 (type=2035), E1M7#187 (type=2035), E2M2#174 (type=2035), E2M2#251 (type=62), E2M2#252 (type=62), E2M2#253 (type=62), E2M2#254 (type=62), E2M4#202 (type=62) |
| Things with unknown flag bits | 0 | Flag bits outside the current Doom thing mask. | none |

### Unknown Linedef Specials

| Special | Count | Examples |
| --- | ---: | --- |
| `65535` | 1 | E2M7#1125 |

### Unknown Sector Specials

None.

## doom2.wad

Maps scanned: 32

### Summary

| Category | Count | Why it matters | Examples |
| --- | ---: | --- | --- |
| Unknown linedef specials | 0 | Likely unsupported gameplay triggers or malformed data. | none |
| Linedef special 48 | 103 | Known wall-scroll special tracked separately from gameplay triggers. | MAP08#170 (scroll wall), MAP08#171 (scroll wall), MAP08#172 (scroll wall), MAP08#173 (scroll wall), MAP08#174 (scroll wall), MAP08#175 (scroll wall), MAP08#176 (scroll wall), MAP08#177 (scroll wall), MAP11#336 (scroll wall), MAP11#337 (scroll wall), MAP11#338 (scroll wall), MAP11#339 (scroll wall) |
| Unknown sector specials | 0 | Sector behavior not recognized by current runtime. | none |
| Things with no skill bits | 20 | Vanilla Doom will not spawn these non-player things. | MAP04#65 (type=3002), MAP06#238 (type=11), MAP06#239 (type=11), MAP22#76 (type=11), MAP22#77 (type=11), MAP22#78 (type=11), MAP22#79 (type=11), MAP22#82 (type=11), MAP25#63 (type=2003), MAP25#113 (type=2049), MAP25#121 (type=80), MAP25#183 (type=2012) |
| Things with unknown flag bits | 0 | Flag bits outside the current Doom thing mask. | none |

### Unknown Linedef Specials

None.

### Unknown Sector Specials

None.

