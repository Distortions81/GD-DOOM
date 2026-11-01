#!/usr/bin/env python3
"""Compare Boom friction/force state and full movement tics with DSDA-Doom."""
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


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad',required=True,type=Path)
    parser.add_argument('--pwad',default=ROOT/'testdata/boom/environment.wad',type=Path)
    parser.add_argument('--cases',default=ROOT/'testdata/boom/environment-cases.json',type=Path)
    ref=parser.add_mutually_exclusive_group(required=True)
    ref.add_argument('--reference',type=Path)
    ref.add_argument('--reference-source',type=Path)
    parser.add_argument('--cmake-arg',action='append',default=[])
    parser.add_argument('--out',default=ROOT/'tmp/boom-environment',type=Path)
    args=parser.parse_args();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
    iwad,pwad,manifest=args.iwad.resolve(),args.pwad.resolve(),args.cases.resolve()
    cases=json.loads(manifest.read_text())
    if not cases or len({c['name'] for c in cases})!=len(cases):parser.error('cases must have unique names')
    executable=args.reference.resolve() if args.reference else build_reference(args.reference_source,out,args.cmake_arg)
    merged=out/'merged.wad';merge_wads([iwad,pwad],merged)
    if manifest!=out/'input-cases.json':shutil.copy(manifest,out/'input-cases.json')
    fields=['x','y','z','momx','momy','momz','forward','side','tics','noclip','disable','enable']
    (out/'cases.tsv').write_text(''.join(' '.join(map(str,[c['name'],int(c['map'][3:]),c['actor'],c['mode']]+[int(c[k]) for k in fields]))+'\n' for c in cases))
    env=dict(os.environ,SDL_AUDIODRIVER='dummy',GD_ENV_CASES=str(manifest),GD_ENV_WAD=str(merged),GD_ENV_OUT=str(out/'gd.csv'))
    run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomEnvironmentCapture$','-count=1'],out/'gd.log',env)
    env.update(GD_DSDA_ENV_CASES=str(out/'cases.tsv'),GD_DSDA_ENV_OUT=str(out/'dsda.csv'))
    for key in ['GD_DSDA_TELEPORT_CASES','GD_DSDA_VISUAL_SCENES']:env.pop(key,None)
    run(['xvfb-run','-a',executable,'-iwad',iwad,'-file',pwad,'-complevel','9','-nosound','-width','320','-height','200','-vidmode','8bit','-config',out/'dsda.cfg'],out/'dsda.log',env)
    expected_keys=[(c['name'],str(tic)) for c in cases for tic in range(c['tics']+1)]
    def read(path):
        with path.open() as f:rows=list(csv.DictReader(f))
        if [(r['name'],r['tic']) for r in rows]!=expected_keys:raise ValueError(f'{path}: missing or duplicate capture rows')
        return rows
    gd,dsda=read(out/'gd.csv'),read(out/'dsda.csv');failures=[];fields=Counter()
    for actual,expected in zip(gd,dsda):
        different={k:dict(gd=actual[k],dsda=expected[k]) for k in expected if actual[k]!=expected[k]}
        if different:failures.append(dict(name=actual['name'],tic=actual['tic'],differences=different));fields.update(different.keys())
    source_hash=hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):source_hash.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    result=dict(passed=not failures,cases=len(cases),rows=len(gd),matched=len(gd)-len(failures),differing_fields=dict(fields),failures=failures,metadata=dict(iwad_sha256=digest(iwad),pwad_sha256=digest(pwad),cases_sha256=digest(manifest),verifier_sha256=digest(Path(__file__)),probe_sha256=digest(ROOT/'scripts/visual_compare/dsda_environment_probe.inc'),reference_sha256=digest(executable),gd_source_sha256=source_hash.hexdigest(),reference='DSDA-Doom 0.27.5 complevel 9, real players and map monsters, AI transitions delayed',revision=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()))
    (out/'results.json').write_text(json.dumps(result,indent=2)+'\n')
    print(f"{result['matched']}/{len(gd)} rows match in {len(cases)} cases; differing fields {dict(fields)}")
    if failures:print(json.dumps(failures[:8],indent=2));raise SystemExit(1)

if __name__=='__main__':main()
