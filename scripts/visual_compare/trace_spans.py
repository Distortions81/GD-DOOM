#!/usr/bin/env python3
"""Explain floor/ceiling pixel differences using --trace-spans captures."""
import argparse
import csv
import json
from pathlib import Path
from PIL import Image


def expand(path):
    pixels = {}
    with path.open() as f:
        for row in csv.DictReader(f):
            s = {k: v if k == 'flat' else int(v) for k, v in row.items()}
            for x in range(s['x1'], s['x2'] + 1):
                u = (s['u'] + (x-s['x1'])*s['du']) & 0xffffffff
                v = (s['v'] + (x-s['x1'])*s['dv']) & 0xffffffff
                pixels[x, s['y']] = (s, u, v, ((v >> 16) & 63)*64 + ((u >> 16) & 63))
    return pixels


def summarize(directory):
    scenes = json.loads((directory / 'scenes.json').read_text())
    results = []
    for scene in scenes:
        name = scene['name']
        a = expand(directory / (name+'-classic-spans.csv'))
        b = expand(directory / (name+'-cpu-spans.csv'))
        with Image.open(directory / (name+'-classic.png')) as im:
            classic = im.convert('RGB')
        with Image.open(directory / (name+'-cpu.png')) as im:
            cpu = im.convert('RGB')
        counts = {k: 0 for k in ('shared_plane_pixels', 'uv_differences', 'texel_differences', 'visible_texel_differences', 'different_span_start', 'different_step', 'same_start_and_step', 'coverage_or_material_differences')}
        examples = []
        for p in sorted(a.keys() | b.keys(), key=lambda p: (p[1], p[0])):
            aa, bb = a.get(p), b.get(p)
            if not aa or not bb or (aa[0]['flat'], aa[0]['height']) != (bb[0]['flat'], bb[0]['height']):
                counts['coverage_or_material_differences'] += 1
                continue
            sa, ua, va, ta = aa
            sb, ub, vb, tb = bb
            counts['shared_plane_pixels'] += 1
            counts['uv_differences'] += (ua, va) != (ub, vb)
            if ta == tb:
                continue
            counts['texel_differences'] += 1
            different_start = sa['x1'] != sb['x1']
            different_step = (sa['du'], sa['dv']) != (sb['du'], sb['dv'])
            counts['different_span_start'] += different_start
            counts['different_step'] += different_step
            counts['same_start_and_step'] += not different_start and not different_step
            if max(abs(x-y) for x,y in zip(classic.getpixel(p), cpu.getpixel(p))) <= 2:
                continue
            counts['visible_texel_differences'] += 1
            if p[1] >= 150 and sa['height'] < scene['Z']*65536 and len(examples) < 8:
                examples.append({'pixel':p, 'classic':dict(sa, pixel_u=ua, pixel_v=va, texel=ta), 'cpu':dict(sb, pixel_u=ub, pixel_v=vb, texel=tb)})
        results.append({'scene':name, 'counts':counts, 'floor_examples':examples})
    return results


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=Path)
    parser.add_argument('--require-plane-parity', action='store_true',
                        help='fail on any UV difference where both renderers draw the same flat at the same height')
    args = parser.parse_args()
    results = summarize(args.directory)
    (args.directory / 'span-differences.json').write_text(json.dumps(results, indent=2))
    for row in results:
        print(row['scene'], row['counts'])
        if row['floor_examples']:
            print('  First floor example:', json.dumps(row['floor_examples'][0]))
    if args.require_plane_parity and any(row['counts']['uv_differences'] for row in results):
        raise SystemExit('Shared floor/ceiling coordinates differ (trace retained)')
