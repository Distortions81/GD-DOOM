#!/usr/bin/env python3
"""Build original geometry for Boom sky/light screenshot regression tests."""
from pathlib import Path
import struct
from make_boom_smoke_wad import name


def map_lumps(number, *, sky=None, flip=False, xoffset=0, yoffset=128,
              lights=False, split=False, floor_sky=False):
    vertices = [(-256, -128), (-256, 128), (0 if split else 256, 128),
                (0 if split else 256, -128)]
    edges = [(0, 1, 0, -1), (1, 2, 0, -1), (2, 3, 0, 1 if split else -1), (3, 0, 0, -1)]
    if split:
        vertices += [(256, 128), (256, -128)]
        edges += [(2, 4, 1, -1), (4, 5, 1, -1), (5, 3, 1, -1)]
    sides, lines = [], []
    for a, b, front, back in edges:
        first = len(sides)
        sides.append((0, 0, "-", "-", "STARTAN3" if back < 0 else "-", front))
        other = 65535
        if back >= 0:
            other = len(sides)
            sides.append((0, 0, "-", "-", "-", back))
        lines.append((a, b, 1 if back < 0 else 4, 0, 0, first, other))
    sectors = [(0, 128, "F_SKY1" if floor_sky else "BTFLAT",
                "F_SKY1" if sky and not floor_sky else "BTFLAT", 160, 0, 1)]
    if split:
        sectors.append((0, 128, "BTFLAT", "F_SKY1", 176, 0, 2))

    def control(special, tag, light=160, texture="-", xo=0, yo=0):
        sector = len(sectors)
        sectors.append((0, 128, "FLOOR0_1", "CEIL1_1", light, 0, 0))
        side = len(sides)
        sides.append((xo, yo, texture, "-", "-", sector))
        # Setup-only degenerate lines have no rendered segs.
        lines.append((0, 0, 0, special, tag, side, 65535))

    if sky:
        control(272 if flip else 271, 1, texture=sky, xo=xoffset, yo=yoffset)
        if split:
            control(272, 2, texture="SKY3", xo=-8192, yo=144)
    if lights:
        control(213, 1, light=48)
        control(261, 1, light=224)
        if split:
            control(213, 2, light=224)
            control(261, 2, light=48)
    segs = [(0, 1, 16384, 0, 0, 0), (1, 2, 0, 1, 0, 0),
            (2, 3, 49152, 2, 0, 0), (3, 0, 32768, 3, 0, 0)]
    nodes = b""
    subsectors = struct.pack("<HH", 4, 0)
    if split:
        segs += [(3, 2, 16384, 2, 1, 0), (2, 4, 0, 4, 0, 0),
                 (4, 5, 49152, 5, 0, 0), (5, 3, 32768, 6, 0, 0)]
        subsectors += struct.pack("<HH", 4, 4)
        nodes = struct.pack("<12h2H", 0, 0, 0, 1, 128, -128, 0, 256,
                            128, -128, -256, 0, 0x8001, 0x8000)
    # All cells share a complete line list, including a dummy zero word.
    words = [-256, -128, 5, 3] + [19] * 15 + [0] + list(range(len(lines))) + [-1]
    return [(f"MAP{number:02}", b""),
            ("THINGS", struct.pack("<5h", -128, 0, 0, 1, 7)),
            ("LINEDEFS", b"".join(struct.pack("<7H", *v) for v in lines)),
            ("SIDEDEFS", b"".join(struct.pack("<hh8s8s8sH", x, y, name(t), name(b), name(m), sec)
                                  for x, y, t, b, m, sec in sides)),
            ("VERTEXES", b"".join(struct.pack("<hh", *v) for v in vertices)),
            ("SEGS", b"".join(struct.pack("<6H", *v) for v in segs)),
            ("SSECTORS", subsectors), ("NODES", nodes),
            ("SECTORS", b"".join(struct.pack("<hh8s8shhh", f, c, name(fp), name(cp), l, s, tag)
                                 for f, c, fp, cp, l, s, tag in sectors)),
            ("REJECT", bytes((len(sectors)**2 + 7)//8)),
            ("BLOCKMAP", struct.pack("<" + "h"*len(words), *words))]


def build():
    lumps = []
    for number, kwargs in enumerate([
        dict(lights=True), dict(sky="SKY2"),
        dict(sky="SKY2", flip=True, xoffset=8192, yoffset=160),
        dict(sky="SKY2", split=True, lights=True),
        dict(sky="SKY2", floor_sky=True),
    ], 1):
        lumps.extend(map_lumps(number, **kwargs))
    lumps.extend([("F_START", b""), ("BTFLAT", bytes([128])*4096), ("F_END", b"")])
    payload = bytearray(bytes(12))
    directory = bytearray()
    for label, data in lumps:
        directory.extend(struct.pack("<ii8s", len(payload), len(data), name(label)))
        payload.extend(data)
    payload[:12] = struct.pack("<4sii", b"PWAD", len(lumps), len(payload))
    return payload + directory


if __name__ == "__main__":
    output = Path(__file__).resolve().parents[1] / "testdata/boom/transfers.wad"
    output.write_bytes(build())
    print(output)
