#!/usr/bin/env python3
"""Trim a completed replay trace without copying its retained prefix."""

import argparse
from pathlib import Path
import re


TIC = b'"kind":"tic"'
PLAYER_DEAD = re.compile(rb'"player":\{"playerstate":1([,}])')


def trim(path, max_tics=0):
    if not path.is_file():
        return
    with path.open("r+b") as trace:
        count = 0
        while True:
            start = trace.tell()
            line = trace.readline()
            if not line:
                return
            if TIC not in line:
                continue
            count += 1
            if max_tics:
                if count > max_tics:
                    trace.truncate(start)
                    return
            elif PLAYER_DEAD.search(line):
                trace.truncate(trace.tell())
                return


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("trace", type=Path)
    parser.add_argument("--max-tics", type=int, default=0)
    args = parser.parse_args()
    if args.max_tics < 0:
        parser.error("--max-tics must be nonnegative")
    trim(args.trace, args.max_tics)


if __name__ == "__main__":
    main()
