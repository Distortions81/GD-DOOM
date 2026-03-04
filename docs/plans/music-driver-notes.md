# OPL Music Driver Notes

## External components reviewed

- `musparser` (`https://github.com/trondhumbor/musparser`)
  - Contains working MUS decoding logic.
  - Current repo layout exposes parser code under `internal/musparser`, so it is not importable as a normal Go library from this project without vendoring/forking.

- `DMXOPL` (`https://github.com/sneakernets/DMXOPL`)
  - Provides GENMIDI/OPL patch bank assets (`.op2`, `.wopl`).
  - Intended as source data for patch/operator parameters.

- `Nuked-OPL3` (`https://github.com/nukeykt/Nuked-OPL3`)
  - High-accuracy OPL3 core.
  - Cloned for local reference under `_research`.

## Implemented in GD-DOOM

- `internal/sound/opl3_basic.go`
  - Minimal register-driven OPL3-like emulator scaffold.

- `internal/music/driver.go`
  - New playback driver skeleton:
    - event scheduler (delta tic -> sample frames)
    - channel state (program/volume/expression/pan/pitch bend)
    - voice allocation (18 voices + simple oldest-voice steal)
    - OPL register writes (patch, pan, key-on/key-off, FNUM/BLOCK)

## Next steps

1. MUS parser adapter:
   - either vendor/fork `musparser` API or port parser logic into `internal/music`.

2. DMX patch bank loader:
   - parse `GENMIDI.op2` into `PatchBank`.

3. Runtime integration:
   - feed driver PCM into an Ebiten audio player path for background music.

4. Accuracy pass:
   - replace/bridge `BasicOPL3` with a cgo/ffi wrapper for Nuked-OPL3.
