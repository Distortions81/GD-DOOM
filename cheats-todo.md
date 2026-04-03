# Doom Cheat TODO

Track typed Doom-style cheat support in GD-DOOM.

## Implemented

- [x] `iddqd`
  Toggles invulnerability through the typed cheat input path.
- [x] `idfa`
  Grants weapons, ammo, armor.
- [x] `idkfa`
  Grants weapons, ammo, armor, and keys.
- [x] `iddt`
  Cycles automap reveal and thing-display parity state.
- [x] `idclip`
  Toggles noclip movement.
- [x] `idspispopd`
  Vanilla alias mapped to the same noclip toggle.
- [x] `idclev`
  Queues a fresh start on the typed destination map through the normal new-game loader.
- [x] `idmypos`
  Prints and shows the current angle and position in a Doom-style debug string.
- [x] `idbehold`
  Shows the original Doom powerup-prompt message.
- [x] `idbeholdv`
  Invulnerability powerup.
- [x] `idbeholds`
  Berserk strength toggle.
- [x] `idbeholdi`
  Partial invisibility powerup.
- [x] `idbeholdr`
  Radiation suit powerup.
- [x] `idbeholda`
  Automap powerup toggle.
- [x] `idbeholdl`
  Light amplification visor powerup.
- [x] `idchoppers`
  Grants chainsaw and applies Doom's direct cheat-style invulnerability flagging.
- [x] `idmus`
  Changes music using Doom-style two-digit selection, including Doom II's non-map slots.

## Next Cheats

- [ ] Doom cheat timeout / key-sequence behavior polish
  Typed input works, but it does not yet emulate Doom's original cheat sequence decoder semantics exactly.

## Notes

- Current typed cheat input is implemented in [internal/doomruntime/cheats.go](/home/dist/github/GD-DOOM/internal/doomruntime/cheats.go).
- Input capture uses Ebiten text input in [internal/doomruntime/game.go](/home/dist/github/GD-DOOM/internal/doomruntime/game.go).
- Tests for the currently supported typed cheats live in [internal/doomruntime/cheats_test.go](/home/dist/github/GD-DOOM/internal/doomruntime/cheats_test.go).
