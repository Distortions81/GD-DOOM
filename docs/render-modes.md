# Render Modes

This project now has a clear mode split:

## Faithful Doom Mode (Default)

- Active when launching normally (no `-sourceport-mode`).
- Uses the Doom-emulation software 3D renderer (`doom-basic`) as the normal walk view.
- Uses wall-driven clip arrays + visplane/span floor/ceiling path in 3D.
- Uses textured wall columns (mid/top/bottom) from Doom texture data.
- Keeps non-vanilla conveniences disabled.

## Sourceport Mode (`-sourceport-mode`)

- Enables convenience and debug behaviors that are not strict vanilla parity.
- Includes sourceport-style extras like:
  - wireframe toggle (`P`)
  - plane/debug toggles (`J/K/U/Y`)
  - additional automap convenience controls (`R`, `B`, `O`, `I`, `L`, `HOME`, etc.)
- 2D automap floor texture overlay is sourceport-only.

## Practical Rule

- If behavior is not faithful to vanilla Doom rendering/gameplay semantics, keep it behind `-sourceport-mode`.
