#!/usr/bin/env python3
import argparse
import struct
import sys
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Extract a DEMO# lump from a Doom WAD into a .lmp file."
    )
    parser.add_argument("--wad", default="DOOM1.WAD", help="path to source WAD")
    parser.add_argument("--demo", default="DEMO1", help="demo lump name (e.g. DEMO1)")
    parser.add_argument(
        "--out",
        default="demos/DOOM1-DEMO1.lmp",
        help="output .lmp path",
    )
    args = parser.parse_args()

    wad_path = Path(args.wad)
    out_path = Path(args.out)
    demo_name = args.demo.strip().upper()

    data = wad_path.read_bytes()
    if len(data) < 12:
        raise SystemExit(f"wad too short: {wad_path}")
    ident, num_lumps, info_table_ofs = struct.unpack_from("<4sii", data, 0)
    if ident not in (b"IWAD", b"PWAD"):
        raise SystemExit(f"invalid wad header {ident!r}: {wad_path}")

    for i in range(num_lumps):
        off = info_table_ofs + i * 16
        file_pos, size, raw_name = struct.unpack_from("<ii8s", data, off)
        name = raw_name.split(b"\0", 1)[0].rstrip(b" ").decode("ascii", "ignore").upper()
        if name != demo_name:
            continue
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_bytes(data[file_pos : file_pos + size])
        print(f"extracted {demo_name} from {wad_path} to {out_path} ({size} bytes)")
        return 0

    raise SystemExit(f"demo lump not found: {demo_name} in {wad_path}")


if __name__ == "__main__":
    sys.exit(main())
