# Free standalone games

GD-DOOM's built-in catalog offers games that need no separately owned base game:

| Game | Pinned version | Content | Terms |
| --- | --- | --- | --- |
| Doom Shareware | 1.9 | Original nine-map first episode, already bundled | Original shareware redistribution terms |
| Freedoom: Phase 1 | 0.13.0 | Four episodes; single player and co-op | BSD-3-Clause |
| Freedoom: Phase 2 | 0.13.0 | 32-map campaign; single player and co-op | BSD-3-Clause |
| FreeDM | 0.13.0 | 32 deathmatch maps | BSD-3-Clause |

This is a curated, versioned compatibility list, not a claim to include every
free Doom-engine game. "Free to download" alone is insufficient: each added
release needs permission to redistribute its complete required data and must
work with the engine. The catalog includes no paid Doom, Doom II, or Final Doom
data and no add-ons that require a separate base game.

Freedoom 0.13.0 is the first release whose levels are entirely vanilla
compatible. Its embedded DeHackEd data changes presentation (text, par times,
and a few weapon-flash frames); GD-DOOM does not apply those patches, so some
labels and effects retain Doom behavior. Map loading by itself does not prove
that every gameplay path has been tested. See the [official release notes](https://github.com/freedoom/freedoom/releases/tag/v0.13.0).

Validation for this catalog covered loading all 100 maps, checking referenced
textures and multiplayer starts, and running 70 authoritative simulation tics
per map. That audit found a missing vanilla floor action (linedef 129), now
implemented with regression coverage. Browser testing also covered Phase 2's
startup download and single-player launch, then downloading FreeDM through
Create Game and joining a deathmatch room. This is compatibility smoke testing,
not a complete playthrough of every map.

## Reproducible acquisition

From the repository root:

```sh
python3 scripts/fetch_free_games.py /path/to/free-games \
  --shareware DOOM1.WAD \
  --cache-dir /path/to/download-cache
```

The cache argument is optional. When omitted, downloaded ZIPs are temporary.
Cached archives must match their pinned hashes; a corrupted cache entry is
rejected rather than used. The script accepts only the bundled, unchanged
Doom shareware 1.9 digest for `--shareware`.

The installer verifies both official release ZIPs and every extracted WAD
against the exact SHA-256 and size recorded in
[`internal/freegames/catalog.json`](../internal/freegames/catalog.json). It
reads only the selected members, rejects symlinks and duplicate entries, and
never performs a general ZIP extraction. No release WADs or ZIPs are added to
Git. Output contains:

- `DOOM1.WAD`, `freedoom1.wad`, `freedoom2.wad`, and `freedm.wad`.
- Complete `COPYING.txt`, `CREDITS.txt`, and `CREDITS-MUSIC.txt` under
  `licenses/freedoom-0.13.0/`.
- `free-game-notices.txt`, containing all three documents for distribution with
  the app, website, or downloaded files.
- `catalog.json`, ready for `gdlobby -catalog`, with all four packs and explicit
  redistribution approvals tied to each exact file hash.

The output catalog replaces an existing `catalog.json` in the chosen directory.
Use a new directory and review the generated catalog before combining it with
an operator's existing custom catalog. The script does not change or restart a
running server. It writes the catalog last, after validating all payloads.

Retain and publish the complete notices wherever these game downloads are
offered. BSD-3-Clause permits redistribution and modification with its notices
retained and restricts use of contributor names for endorsement. The package's
`freegames.Notices()` API embeds the same complete legal and credit documents.

## Sources and selection

- [Official Freedoom project](https://freedoom.github.io/) and
  [0.13.0 release](https://github.com/freedoom/freedoom/releases/tag/v0.13.0).
- [Official release checksums](https://github.com/freedoom/freedoom/releases/download/v0.13.0/freedoom-0.13.0-CHECKSUM).
  Archive hashes were compared with this published file; this is checksum
  verification, not independent signature verification.
- [Freedoom license at the release tag](https://github.com/freedoom/freedoom/blob/v0.13.0/COPYING.adoc).
  The installer preserves the complete text and contributor/music credits from
  each original release ZIP.
- [Doom shareware license and John Carmack's redistribution clarification](https://sources.debian.org/src/doom-wad-shareware/1.9.fixed-4/debian/copyright/).
  The existing unchanged shareware remains a separate option. The catalog does
  not offer shareware add-ons: the original engine explicitly rejected loading
  them, and id's original policy requested they not be made for shareware.
  [Original engine check](https://github.com/id-Software/DOOM/blob/master/linuxdoom-1.10/d_main.c#L966-L981),
  [archived README and license excerpts](https://www.gamers.org/docs/FAQ/DOOM.FAQ.Specs.Chapters.1.html).

Other freeware conversions are not automatically eligible. Hacx and REKKR need
gameplay changes supplied through DeHackEd; their data cannot currently be
offered as faithful, working GD-DOOM games. Chex Quest requires game-specific
behavior and a verified redistribution basis. Games requiring Heretic, Hexen,
Strife, Boom/MBF extensions, or ZDoom scripting are also outside this catalog's
current compatibility scope.
