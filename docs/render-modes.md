# Render Modes

This project now has a clear mode split:

## Faithful Doom Mode (Default)

- Active when launching normally (no `-sourceport-mode`).
- Uses the Doom-emulation software walk renderer with wall-driven clipping plus visplane/span floor-ceiling rendering.
- Uses textured wall columns (mid/top/bottom) from Doom texture data.
- Keeps automap north-up with parity-oriented defaults such as `line-color-mode=parity`.
- Keeps non-vanilla conveniences disabled.

## Sourceport Mode (`-sourceport-mode`)

- Keeps the same core software walk renderer, but enables convenience behaviors that are not strict vanilla parity.
- Adds sourceport-style automap and presentation controls such as heading-up map rotation (`R`), big-map shortcut (`B`), allmap/`IDDT` toggles (`O`/`I`), line-color toggle (`L`), legend toggle (`V`), thing render mode cycling (`T`), and runtime mouselook toggle (`\`).
- Makes optional presentation features such as GPU sky and CRT practical to use in normal play.
- If no explicit sky override is provided, sourceport mode currently defaults to GPU sky with `-sky-upscale=sharp`.
- Sourceport mode also normalizes automap thing rendering to `items` unless overridden with `-sourceport-thing-render-mode`.

## Practical Rule

- If behavior is not faithful to vanilla Doom rendering/gameplay semantics, keep it behind `-sourceport-mode`.
