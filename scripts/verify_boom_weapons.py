#!/usr/bin/env python3
"""Compare empty-weapon commands, psprites, ammo and RNG against DSDA-Doom."""
import argparse,hashlib,json,os
from pathlib import Path
from verify_boom_rng import fixture,compare
from verify_boom_transfers import ROOT,run,digest,merge_wads,build_reference


def cases():
    result=[]
    # Source IDs are Doom's weapon enum. Each profile exercises preference and
    # shared-ammo boundaries; the selected source is always owned as well.
    profiles=[('none',3,[0,0,0,0]),('plasma',3|1<<5,[0,0,1,0]),('ssg2',3|1<<8,[0,2,0,0]),('ssg3',3|1<<8,[0,3,0,0]),('chain',3|1<<3,[1,0,0,0]),('shotgun',3|1<<2,[0,1,0,0]),('pistol',3,[1,0,0,0]),('saw',3|1<<7,[0,0,0,0]),('rocket',3|1<<4,[0,0,0,1]),('bfg40',3|1<<6,[0,0,40,0]),('bfg41',3|1<<6,[0,0,41,0]),('all',511,[2,2,40,1])]
    for boom,automatic in [(0,0),(1,0),(1,1)]:
      for ready in range(9):
       for label,owned,ammo in profiles:
        for pattern in range(4):
            for loaded in [False,True]:
                values=list(ammo)
                if loaded:
                    if ready in [1,3]:values[0]=1
                    if ready in [2,8]:values[1]=2 if ready==8 else 1
                    if ready in [5,6]:values[2]=40 if ready==6 else 1
                    if ready==4:values[3]=1
                result.append(dict(Name=f'b{boom}_auto{automatic}_w{ready}_{label}_p{pattern}_loaded{int(loaded)}',Boom=boom,Automatic=automatic,Ready=ready,Owned=owned|1<<ready,Bullets=values[0],Shells=values[1],Cells=values[2],Rockets=values[3],Pattern=pattern,Slot=4 if ready!=4 else 6,Tics=240,Seed=1993,Insurance=boom))
    return result


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--iwad',required=True,type=Path)
    ref=p.add_mutually_exclusive_group(required=True);ref.add_argument('--reference',type=Path);ref.add_argument('--reference-source',type=Path)
    p.add_argument('--cmake-arg',action='append',default=[]);p.add_argument('--out',type=Path,default=ROOT/'tmp/boom-weapons');p.add_argument('--limit',type=int);p.add_argument('--filter')
    args=p.parse_args();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
    inputs=cases()
    if args.filter:inputs=[c for c in inputs if args.filter in c['Name']]
    if args.limit:inputs=inputs[:args.limit]
    if not inputs:p.error('no selected cases')
    (out/'cases.json').write_text(json.dumps(inputs,indent=2)+'\n')
    keys=['Name','Boom','Automatic','Ready','Owned','Bullets','Shells','Cells','Rockets','Pattern','Slot','Tics','Seed','Insurance']
    (out/'cases.tsv').write_text(''.join(' '.join(str(c[k]) for k in keys)+'\n' for c in inputs))
    pwad=out/'weapons.wad';fixture(pwad);merged=out/'merged.wad';merge_wads([args.iwad.resolve(),pwad],merged)
    exe=args.reference.resolve() if args.reference else build_reference(args.reference_source,out,args.cmake_arg)
    env=dict(os.environ,SDL_AUDIODRIVER='dummy',GD_WEAPON_CASES=str(out/'cases.json'),GD_WEAPON_WAD=str(merged),GD_WEAPON_OUT=str(out/'gd.csv'))
    run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomWeaponsCapture$','-count=1'],out/'gd.log',env)
    env.update(GD_DSDA_WEAPON_CASES=str(out/'cases.tsv'),GD_DSDA_WEAPON_OUT=str(out/'dsda.csv'))
    run(['xvfb-run','-a',exe,'-iwad',args.iwad.resolve(),'-file',pwad,'-complevel','9','-nosound','-config',out/'dsda.cfg'],out/'dsda.log',env)
    result=compare(out/'gd.csv',out/'dsda.csv',sum(c['Tics'] for c in inputs));result['cases']=len(inputs)
    result['metadata']={k:digest(v) for k,v in dict(reference=exe,iwad=args.iwad,pwad=pwad,manifest=out/'cases.json',probe=ROOT/'scripts/visual_compare/dsda_weapons_probe.inc',verifier=Path(__file__)).items()}
    source=hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):source.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    result['metadata']['gd_source']=source.hexdigest()
    result['scope']='Real P_PlayerThink/psprite and world ticks with monsters disabled. Boom recorded commands remain exact; automatic input decisions use the reference P_CheckAmmo/P_SwitchWeapon and default preferences. Boolean refire is compared; cosmetic misc RNG is excluded.'
    (out/'results.json').write_text(json.dumps(result,indent=2)+'\n');print(f"{result['matched']}/{result['rows']} rows match across {len(inputs)} cases",flush=True)
    if not result['passed']:print(json.dumps(result['failures'][:3],indent=2));raise SystemExit(1)
if __name__=='__main__':main()
