#!/usr/bin/env python3
"""Compare actual actor teleport/movement state against DSDA-Doom 0.27.5."""
import argparse
from collections import Counter
import csv
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess

from verify_boom_transfers import ROOT, build_reference, digest, merge_wads, run


def read_states(path, cases):
    with path.open() as stream:
        rows = list(csv.DictReader(stream))
    if len(rows) != len(cases) or [r['name'] for r in rows] != [c['name'] for c in cases]:
        raise ValueError(f'{path}: missing, reordered or duplicate cases')
    return rows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', required=True, type=Path)
    parser.add_argument('--pwad', default=ROOT/'testdata/boom/teleports.wad', type=Path)
    parser.add_argument('--cases', default=ROOT/'testdata/boom/teleport-cases.json', type=Path)
    reference = parser.add_mutually_exclusive_group(required=True)
    reference.add_argument('--reference', type=Path)
    reference.add_argument('--reference-source', type=Path)
    parser.add_argument('--cmake-arg', action='append', default=[])
    parser.add_argument('--out', default=ROOT/'tmp/boom-teleports', type=Path)
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    iwad, pwad, manifest = args.iwad.resolve(), args.pwad.resolve(), args.cases.resolve()
    cases = json.loads(manifest.read_text())
    if not cases or len({c['name'] for c in cases}) != len(cases):
        parser.error('cases must have unique names')
    executable = (args.reference.resolve() if args.reference else
                  build_reference(args.reference_source, out, args.cmake_arg))
    merged = out/'merged.wad'
    merge_wads([iwad, pwad], merged)
    if manifest != out/'input-cases.json':
        shutil.copy(manifest, out/'input-cases.json')
    fields = ['source', 'destination', 'special', 'back', 'blocked', 'missing',
              'z', 'y', 'momx', 'momy', 'momz', 'angle', 'ticks', 'x', 'nextx', 'nexty']
    (out/'cases.tsv').write_text(''.join(
        ' '.join(map(str, [c['name'], int(c['map'][3:]), c['actor']] +
                     [int(c[k]) for k in fields]))+'\n' for c in cases))
    env = dict(os.environ, SDL_AUDIODRIVER='dummy', GD_TELEPORT_CASES=str(manifest),
               GD_TELEPORT_WAD=str(merged), GD_TELEPORT_OUT=str(out/'gd.csv'))
    run(['xvfb-run', '-a', 'go', 'test', '-tags', 'integration', './internal/doomruntime',
         '-run', '^TestBoomTeleportCapture$', '-count=1'], out/'gd.log', env)
    env.update(GD_DSDA_TELEPORT_CASES=str(out/'cases.tsv'),
               GD_DSDA_TELEPORT_OUT=str(out/'dsda.csv'))
    env.pop('GD_DSDA_VISUAL_SCENES', None)
    # Deliberately no -nomonsters: map actors retain real monster flags/types.
    run(['xvfb-run', '-a', executable, '-iwad', iwad, '-file', pwad, '-complevel', '9',
         '-nosound', '-width', '320', '-height', '200', '-vidmode', '8bit',
         '-config', out/'dsda.cfg'], out/'dsda.log', env)
    gd, dsda = read_states(out/'gd.csv', cases), read_states(out/'dsda.csv', cases)
    # Assert that the reference actually exercised the intended success and
    # rejection paths, rather than accepting two captures that never crossed.
    for case, state in zip(cases, dsda):
        if case['ticks']:
            continue
        allowed = (case['actor'] != 'missile' and not case['back'] and
                   not case['missing'] and
                   not (case['actor'] == 'player' and case['special'] >= 264) and
                   not (case['blocked'] and case['actor'] != 'player'))
        distance = abs(int(state['x'])-case['nextx']) + abs(int(state['y'])-case['nexty'])
        if (distance > 128*65536) != allowed:
            raise ValueError(f"reference did not exercise expected teleport gate: {case['name']}")
    failures = []
    for case, actual, expected in zip(cases, gd, dsda):
        different = {k: dict(gd=actual[k], dsda=expected[k])
                     for k in expected if actual[k] != expected[k]}
        if different:
            failures.append(dict(name=case['name'], differences=different))
    source_hash = hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):
        source_hash.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    metadata = dict(
        iwad_sha256=digest(iwad), pwad_sha256=digest(pwad), cases_sha256=digest(manifest),
        reference_sha256=digest(executable), verifier_sha256=digest(Path(__file__)),
        probe_sha256=digest(ROOT/'scripts/visual_compare/dsda_teleport_probe.inc'),
        gd_source_sha256=source_hash.hexdigest(), actors=dict(Counter(c['actor'] for c in cases)),
        revision=subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
        dirty=bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, text=True)),
        reference='DSDA-Doom 0.27.5 complevel 9; spawned actors, P_TryMove crossings, '
                  'P_MobjThinker physics with state transitions delayed')
    result = dict(passed=not failures, cases=len(cases), matched=len(cases)-len(failures),
                  failures=failures, metadata=metadata)
    (out/'results.json').write_text(json.dumps(result, indent=2)+'\n')
    print(f"{result['matched']}/{len(cases)} actor-state comparisons match")
    if failures:
        print(json.dumps(failures[:10], indent=2))
        raise SystemExit(1)


if __name__ == '__main__':
    main()
