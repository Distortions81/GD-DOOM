# Map Audit

Generated from local IWADs with `doom-source` as the behavior reference.

## In Plain Terms

These are the map-data cases found in the IWADs that do not work in original Doom:

- Some non-player things in the IWADs have no skill bits set. In vanilla Doom those things do not spawn at all.
- There is one malformed linedef special value `65535` in `doom.wad` (`E2M7`). That is not a real Doom special.

## doom.wad

Maps scanned: 36

### Summary

| Category | Count | Why it matters | Examples |
| --- | ---: | --- | --- |
| Unknown linedef specials | 1 | Likely unsupported gameplay triggers or malformed data. | E2M7#1125 |
| Linedef special 48 | 147 | Known wall-scroll special tracked separately from gameplay triggers. | E1M1#348 (scroll wall), E1M1#349 (scroll wall), E1M1#350 (scroll wall), E1M1#351 (scroll wall), E1M1#352 (scroll wall), E1M1#353 (scroll wall), E1M1#354 (scroll wall), E1M1#355 (scroll wall), E1M7#925 (scroll wall), E1M7#926 (scroll wall), E1M7#927 (scroll wall), E1M7#928 (scroll wall) |
| Unknown sector specials | 0 | Sector behavior not recognized by current runtime. | none |
| Things with no skill bits | 63 | Vanilla Doom will not spawn these non-player things. | E1M4#16 (type=2008), E1M4#48 (type=2008), E1M7#112 (type=2035), E1M7#113 (type=2035), E1M7#114 (type=2035), E1M7#187 (type=2035), E2M2#174 (type=2035), E2M2#251 (type=62), E2M2#252 (type=62), E2M2#253 (type=62), E2M2#254 (type=62), E2M4#202 (type=62) |
| Things with ambush flag | 2113 | Parsed, but currently ineffective without Doom-style sound wake behavior. | E1M1#8 (type=3001), E1M1#9 (type=3001), E1M1#10 (type=3004), E1M1#11 (type=3004), E1M1#12 (type=3004), E1M1#14 (type=3004), E1M1#15 (type=3004), E1M1#16 (type=3001), E1M1#17 (type=3004), E1M1#18 (type=3004), E1M1#19 (type=2012), E1M1#20 (type=2007) |
| Things with not-single flag | 294 | Current runtime handles this flag. | E1M1#22 (type=2008), E1M1#74 (type=2003), E1M1#75 (type=2046), E1M1#76 (type=2046), E1M1#77 (type=2046), E1M1#78 (type=2002), E1M1#79 (type=2048), E1M1#80 (type=2048), E1M1#81 (type=2048), E1M1#82 (type=2048), E1M1#83 (type=2048), E1M1#84 (type=2049) |
| Things with not-deathmatch flag | 0 | Current runtime handles this flag. | none |
| Things with not-coop flag | 0 | Current runtime handles this flag. | none |
| Things with unknown flag bits | 0 | Flag bits outside the current Doom thing mask. | none |
| Lines with block-monsters flag | 360 | Currently ignored; can affect monster routing and wake behavior. | E1M1#178, E1M1#179, E1M1#180, E1M1#181, E1M1#182, E1M1#183, E1M1#192, E1M1#193, E1M1#226, E1M1#259, E1M1#458, E1M1#459 |
| Lines with don't-peg-top | 3305 | Currently ignored; affects wall texture alignment. | E1M1#25, E1M1#28, E1M1#29, E1M1#30, E1M1#37, E1M1#38, E1M1#39, E1M1#46, E1M1#47, E1M1#48, E1M1#49, E1M1#50 |
| Lines with don't-peg-bottom | 5321 | Currently ignored; affects wall texture alignment. | E1M1#25, E1M1#28, E1M1#29, E1M1#30, E1M1#46, E1M1#62, E1M1#63, E1M1#70, E1M1#71, E1M1#72, E1M1#73, E1M1#74 |
| Lines with sound-block | 506 | Currently ignored; affects monster hearing propagation. | E1M2#121, E1M2#572, E1M2#579, E1M2#885, E1M2#886, E1M3#184, E1M3#216, E1M3#496, E1M3#497, E1M3#523, E1M3#528, E1M4#58 |

### Unknown Linedef Specials

| Special | Count | Examples |
| --- | ---: | --- |
| `65535` | 1 | E2M7#1125 |


## doom2.wad

Maps scanned: 32

### Summary

| Category | Count | Why it matters | Examples |
| --- | ---: | --- | --- |
| Unknown linedef specials | 0 | Likely unsupported gameplay triggers or malformed data. | none |
| Linedef special 48 | 103 | Known wall-scroll special tracked separately from gameplay triggers. | MAP08#170 (scroll wall), MAP08#171 (scroll wall), MAP08#172 (scroll wall), MAP08#173 (scroll wall), MAP08#174 (scroll wall), MAP08#175 (scroll wall), MAP08#176 (scroll wall), MAP08#177 (scroll wall), MAP11#336 (scroll wall), MAP11#337 (scroll wall), MAP11#338 (scroll wall), MAP11#339 (scroll wall) |
| Unknown sector specials | 0 | Sector behavior not recognized by current runtime. | none |
| Things with no skill bits | 20 | Vanilla Doom will not spawn these non-player things. | MAP04#65 (type=3002), MAP06#238 (type=11), MAP06#239 (type=11), MAP22#76 (type=11), MAP22#77 (type=11), MAP22#78 (type=11), MAP22#79 (type=11), MAP22#82 (type=11), MAP25#63 (type=2003), MAP25#113 (type=2049), MAP25#121 (type=80), MAP25#183 (type=2012) |
| Things with ambush flag | 2331 | Parsed, but currently ineffective without Doom-style sound wake behavior. | MAP01#46 (type=3001), MAP01#47 (type=3001), MAP01#53 (type=3001), MAP01#54 (type=3001), MAP02#11 (type=3004), MAP02#12 (type=3004), MAP02#13 (type=9), MAP02#14 (type=9), MAP02#15 (type=9), MAP02#16 (type=9), MAP02#17 (type=9), MAP02#18 (type=9) |
| Things with not-single flag | 184 | Current runtime handles this flag. | MAP01#55 (type=82), MAP01#56 (type=2004), MAP01#57 (type=2006), MAP01#58 (type=2002), MAP01#64 (type=2046), MAP01#65 (type=2003), MAP01#66 (type=2006), MAP01#67 (type=82), MAP02#111 (type=2003), MAP02#112 (type=2002), MAP02#113 (type=2001), MAP02#114 (type=2049) |
| Things with not-deathmatch flag | 0 | Current runtime handles this flag. | none |
| Things with not-coop flag | 0 | Current runtime handles this flag. | none |
| Things with unknown flag bits | 0 | Flag bits outside the current Doom thing mask. | none |
| Lines with block-monsters flag | 170 | Currently ignored; can affect monster routing and wake behavior. | MAP01#34, MAP01#35, MAP02#327, MAP02#328, MAP02#338, MAP02#339, MAP02#378, MAP02#379, MAP02#380, MAP02#384, MAP02#385, MAP02#386 |
| Lines with don't-peg-top | 3118 | Currently ignored; affects wall texture alignment. | MAP01#8, MAP01#21, MAP01#69, MAP01#94, MAP01#95, MAP01#97, MAP01#98, MAP01#99, MAP01#100, MAP01#102, MAP01#103, MAP01#127 |
| Lines with don't-peg-bottom | 5004 | Currently ignored; affects wall texture alignment. | MAP01#8, MAP01#55, MAP01#67, MAP01#114, MAP01#115, MAP01#122, MAP01#123, MAP01#131, MAP01#133, MAP01#162, MAP01#171, MAP01#172 |
| Lines with sound-block | 547 | Currently ignored; affects monster hearing propagation. | MAP01#21, MAP01#151, MAP01#184, MAP01#189, MAP01#190, MAP01#199, MAP03#86, MAP03#87, MAP06#183, MAP06#220, MAP06#221, MAP06#222 |