# Doom menu font artwork

`bigupper.png` and the glyph measurements in `glyphs.json` are the community-completed
Doom **BIGUPPER** face from UZDoom, assembled into an atlas by Eevee's Doom Text
Generator. The atlas is unchanged; the JSON contains the corresponding measurements.

Sources:

- [Eevee's Doom Text Generator](https://github.com/eevee/doom-text-generator)
- [Pinned atlas](https://github.com/eevee/doom-text-generator/blob/a104f393da0340735d8326589c21dd706542ff63/fonts/bigupper-uzdoom.png)
- [Pinned glyph metadata](https://github.com/eevee/doom-text-generator/blob/a104f393da0340735d8326589c21dd706542ff63/data.js)
- [UZDoom BIGUPPER originals](https://github.com/UZDoom/UZDoom/tree/trunk/wadsrc_extra/nonfree/filter/doom.id/fonts/bigupper)
- [Upstream game-content terms](https://github.com/UZDoom/UZDoom/blob/trunk/wadsrc_extra/nonfree/license.md), preserved in `UPSTREAM-LICENSE.md`

The upstream atlas credits id Software, Skulltag, Amuscaria, JNechaevsky, UZDoom,
and other contributors. Original Doom artwork is by id Software. This is
Doom-derived game artwork under the separate upstream terms, **not** artwork
relicensed under GD-DOOM's GPL source-code license. Keep these notices with copies.

Retrieved 2026-10-09. Atlas SHA-256:
`18d4455e6b0b86e367955403091f965d89ba547c76ffeb485e23d5da417c00ea`.

## Using the font

`menufont.New(menuPatches).Compose("Multiplayer")` creates a transparent
`media.WallTexture`. Uppercase input selects 15-pixel capitals; lowercase input
selects the original 12-pixel small capitals on the same baseline. Keep the
returned patch offsets when drawing accented text. `Measure` returns the same
pixel bounds as `Compose`. Spaces, multiple lines, numbers, common punctuation,
and the supplied extended alphabets are supported; unknown characters show `?`.

The face supplies 436 glyphs, including all A–Z/a–z letters, so no new letter
shapes were needed. The original grayscale atlas remains intact. At load time,
the renderer samples matching N/M/W artwork from the loaded Doom menu and maps
the atlas to its red shades (or a compatible WAD recolor). When the references
are absent or replaced with different shapes, it uses the standard Doom red ramp.
An authored WAD `M_MULTI` patch overrides the generated Multiplayer label.

The font and metadata are embedded for desktop and WASM. No online font download,
system font installation, or per-frame composition is required.
