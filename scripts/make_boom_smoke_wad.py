#!/usr/bin/env python3
"""Build the original, asset-free Boom smoke map used by runtime tests.

Run from any directory. The output is deterministic and requires an IWAD to play.
"""

from pathlib import Path
import struct


def name(value):
    return value.encode("ascii").ljust(8, b"\0")


def build():
    # Clockwise square: every front sidedef faces the room. No BSP split is
    # needed for one convex subsector; BLOCKMAP lists every boundary wall.
    vertices = [(-128, -128), (-128, 128), (128, 128), (128, -128)]
    specials = [85, 161, 197, 0]
    textures = ["STARTAN3", "SW1BRCOM", "SW1EXIT", "STARTAN3"]
    lumps = [("MAP01", b""), ("THINGS", struct.pack("<hhhhh", 0, 0, 90, 1, 7))]
    lumps.append(("LINEDEFS", b"".join(
        struct.pack("<HHHHHHh", i, (i + 1) % 4, 1, specials[i],
                    1 if i == 1 else 0, i, -1) for i in range(4))))
    lumps.append(("SIDEDEFS", b"".join(
        struct.pack("<hh8s8s8sH", 0, 0, name("-"), name("-"), name(tex), 0)
        for tex in textures)))
    lumps.append(("VERTEXES", b"".join(struct.pack("<hh", *v) for v in vertices)))
    lumps.append(("SEGS", b"".join(
        struct.pack("<HHHHHH", i, (i + 1) % 4, angle, i, 0, 0)
        for i, angle in enumerate([16384, 0, 49152, 32768]))))
    lumps.extend([("SSECTORS", struct.pack("<HH", 4, 0)), ("NODES", b"")])
    lumps.append(("SECTORS", struct.pack("<hh8s8shhh", 0, 128,
        name("FLOOR0_1"), name("CEIL1_1"), 192, 128 | 32 | 8, 1)))
    lumps.append(("REJECT", b"\0"))
    # 3x3 cells, shared list at word 13: dummy zero, lines 0..3, terminator.
    words = [-128, -128, 3, 3] + [13] * 9 + [0, 0, 1, 2, 3, -1]
    lumps.append(("BLOCKMAP", struct.pack("<" + "h" * len(words), *words)))
    payload = bytearray(b"\0" * 12)
    directory = bytearray()
    for lump_name, data in lumps:
        directory.extend(struct.pack("<ii8s", len(payload), len(data), name(lump_name)))
        payload.extend(data)
    payload[:12] = struct.pack("<4sii", b"PWAD", len(lumps), len(payload))
    return payload + directory


if __name__ == "__main__":
    output = Path(__file__).resolve().parents[1] / "testdata/boom/simple.wad"
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_bytes(build())
    print(output)
