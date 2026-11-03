#!/usr/bin/env python3
"""Compare ordered, spatial sector sound requests on every mover-probe frame."""
import argparse
import csv
import itertools
import json
import os
from pathlib import Path
from verify_boom_transfers import ROOT, run, digest, build_reference


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--iwad',required=True,type=Path)
    ref=p.add_mutually_exclusive_group(required=True)
    ref.add_argument('--reference',type=Path)
    ref.add_argument('--reference-source',type=Path)
    p.add_argument('--cmake-arg',action='append',default=[])
    p.add_argument('--out',type=Path,default=ROOT/'tmp/boom-sector-sounds')
    args=p.parse_args();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
    executable=args.reference.resolve() if args.reference else build_reference(args.reference_source,out,args.cmake_arg)
    results=[]
    for suite in ['legacy','generalized','manual','sound-matrix']:
        folder=out/suite;folder.mkdir(exist_ok=True)
        gd=folder/'gd-sounds.csv';dsda=folder/'dsda-sounds.csv'
        env=dict(os.environ)
        extra=['--sector-sounds'] if suite=='sound-matrix' else []
        script='manual' if suite=='sound-matrix' else suite
        run(['python3',ROOT/f'scripts/verify_boom_{script}.py','--iwad',args.iwad.resolve(),'--reference',executable,'--out',folder,'--sector-sound-trace',*extra],folder/'verify.log',env)
        count=matched=events=0;failures=[];origins=set();loops=oneshots=0
        with gd.open() as a,dsda.open() as b:
            for actual,expected in itertools.zip_longest(csv.DictReader(a),csv.DictReader(b)):
                count+=1
                if actual is None or expected is None or actual['name']!=expected['name'] or actual['tic']!=expected['tic']:raise ValueError('missing/reordered sound rows')
                if actual==expected:matched+=1
                elif len(failures)<30:failures.append(dict(gd=actual,dsda=expected))
                for event in expected['events'].split(';'):
                    if not event:continue
                    lump,sector,x,y,timeout=event.split(':');events+=1;origins.add((sector,x,y));loops+=int(timeout)>0;oneshots+=int(timeout)==0
        result=dict(suite=suite,passed=matched==count,rows=count,matched=matched,events=events,loops=loops,oneshots=oneshots,origins=sorted(origins),failures=failures,gd_trace=digest(gd),dsda_trace=digest(dsda),movement=json.loads((folder/'results.json').read_text()))
        (folder/'sound-results.json').write_text(json.dumps(result,indent=2)+'\n');results.append(result)
        print(f'{suite}: {matched}/{count} sound frames match; {events} requests',flush=True)
        if failures:print(json.dumps(failures[:4],indent=2),flush=True)
    result=dict(passed=all(r['passed'] for r in results),suites=results,metadata={label:digest(path) for label,path in dict(reference=executable,probe=ROOT/'scripts/visual_compare/dsda_sector_sound_probe.inc',verifier=Path(__file__)).items()},scope='Sector sound requests, order, loop timeout, fixed-point origins and empty frames; movers/dispatchers unmodified in DSDA. Initial perpetual lift choices are seeded to exercise both directions; this is not a comparison of Boom RNG streams, mixed waveforms or listening.')
    for path in [ROOT/'scripts/make_boom_sector_sound_cases.py', ROOT/'scripts/verify_boom_transfers.py', *sorted((ROOT/'scripts/visual_compare').glob('dsda_*probe.inc'))]:result['metadata'][str(path.relative_to(ROOT))]=digest(path)
    (out/'results.json').write_text(json.dumps(result,indent=2)+'\n')
    if not result['passed']:raise SystemExit(1)

if __name__=='__main__':main()
