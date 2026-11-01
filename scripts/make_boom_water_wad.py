#!/usr/bin/env python3
"""Original two-room height-transfer fixture; real water floor is -64, surface 0."""
from pathlib import Path
import struct
from make_boom_smoke_wad import name
from make_boom_transfer_wad import map_lumps


def build():
    lumps = []
    for number in range(1, 4):
        room = map_lumps(number, split=True)
        data = dict(room)
        lines = list(struct.iter_unpack("<7H", data["LINEDEFS"]))
        sides = [list(v) for v in struct.iter_unpack("<hh8s8s8sH", data["SIDEDEFS"])]
        for side in lines[2][5:]:
            sides[side][2] = name("STARTAN3")
            sides[side][3] = name("STARTAN3")
        control_side = len(sides)
        sides.append([0, 0, name("-"), name("-"), name("-"), 2])
        lines.append((0, 0, 0, 242, 7, control_side, 65535))
        sectors = [(0, 160, "BWLAND", "BWROOM", 192, 0, 0),
                   (-64, 160, "BWSURF", "BWROOM", 160, 0, 7),
                   (0, 128, "F_SKY1" if number == 2 else "BWDEEP",
                    "F_SKY1" if number == 3 else "BWCEIL", 96, 0, 0)]
        data["LINEDEFS"] = b"".join(struct.pack("<7H", *v) for v in lines)
        data["SIDEDEFS"] = b"".join(struct.pack("<hh8s8s8sH", *v) for v in sides)
        data["SECTORS"] = b"".join(struct.pack("<hh8s8shhh", f, c, name(fp), name(cp), light, special, tag)
                                    for f, c, fp, cp, light, special, tag in sectors)
        data["REJECT"] = bytes(2)
        words = [-256, -128, 5, 3] + [19]*15 + [0] + list(range(len(lines))) + [-1]
        data["BLOCKMAP"] = struct.pack("<"+"h"*len(words), *words)
        lumps.extend((label, data[label]) for label, _ in room)
    # Uniform original flats isolate fake-plane visibility and lighting from UV differences.
    lumps.append(("F_START", b""))
    for label, index in [("BWLAND", 128), ("BWROOM", 176), ("BWSURF", 204), ("BWDEEP", 196), ("BWCEIL", 200)]:
        lumps.append((label, bytes([index])*4096))
    lumps.append(("F_END", b""))
    out, directory = bytearray(bytes(12)), bytearray()
    for label, data in lumps:
        directory.extend(struct.pack("<ii8s", len(out), len(data), name(label)))
        out.extend(data)
    struct.pack_into("<4sii", out, 0, b"PWAD", len(lumps), len(out))
    return out+directory


if __name__ == "__main__":
    output = Path(__file__).resolve().parents[1]/"testdata/boom/water.wad"
    output.write_bytes(build())
    print(output)
