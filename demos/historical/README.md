# Recovered regression demos

These five distinct recordings were recovered from all available branch history
on 2026-09-30. The original bytes are preserved; the names identify the IWAD/map
and distinguish successive recordings that originally shared `output.lmp`.
They are automatically included by `scripts/demo_trace_compare_all.py`.

| File | Original path | Git blob |
| --- | --- | --- |
| `DOOM1-E1M1-OUTPUT-2198.lmp` | `output.lmp` on `render-visualize` | `820fe52b5b544a21e05f564c6efc2cd971625469` |
| `DOOM1-E1M1-OUTPUT-2110.lmp` | `output.lmp` | `93b465df3c630c9374e3ba929057e1df942eb182` |
| `DOOM1-E1M1-OUTPUT-1966.lmp` | `output.lmp` | `850d32c20b25bb3df783fe6679f6c636b7914aec` |
| `DOOM1-E1M1-SKY-TEST.lmp` | `test.lmp` | `a1e413ca0a9e8f50bb87669e45f7b649cc1eb586` |
| `DOOM2-MAP04-GRATE-OLD.lmp` | `grate.lmp` | `8889ce0591862f0c6aabb7081053b50559352aab` |

The branch's `masked.lmp` and the other historical blobs match demos already
present in `demos/`, so they do not add distinct replay inputs. The additional
MAP26 speedrun recovered from its ZIP lives in `../doom2-uvmax-late/`.
