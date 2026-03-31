# `-record-demo` Parity TODO

- [ ] Stop recording at the same points as vanilla Doom.
  GD-DOOM currently preserves recorded tics across map rebuilds and can emit a recording that spans level transitions or post-death restarts.
  Vanilla ends demo recording when the level ends or the player dies by finalizing the demo immediately.
  Relevant refs: `internal/doomruntime/run_state.go`, `linuxdoom-1.10/g_game.c`.

- [ ] Write parity-correct demo header flags.
  GD-DOOM currently records `Skill`, `Deathmatch`, and `FastMonsters`, but hard-codes `Respawn=false` and `NoMonsters=false`.
  Vanilla writes `deathmatch`, `respawnparm`, `fastparm`, and `nomonsters` into the demo header.
  Relevant refs: `internal/demo/demo.go`, `linuxdoom-1.10/g_game.c`.

- [ ] Decide whether CLI behavior should match vanilla `-record`.
  Vanilla `-record` takes a demo basename, appends `.lmp`, and supports `-maxdemo` buffer sizing.
  GD-DOOM `-record-demo` writes to the exact path provided and has no `-maxdemo` equivalent.
  This is a parity gap, but it may be an intentional product-level divergence rather than a bug.
  Relevant refs: `internal/app/run_parse.go`, `internal/demo/demo.go`, `linuxdoom-1.10/d_main.c`, `linuxdoom-1.10/g_game.c`.
