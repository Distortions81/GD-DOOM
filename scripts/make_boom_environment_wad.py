#!/usr/bin/env python3
"""Original Boom friction, wind/current and point-force physics fixtures."""
from pathlib import Path
import json
import struct
from make_boom_smoke_wad import name
from make_boom_transfer_wad import map_lumps

CONFIGS=[
 dict(label='mud',controls=[(223,48,0,7)],flags=256),
 dict(label='normal-friction',controls=[(223,100,0,7)],flags=256),
 dict(label='ice',controls=[(223,160,0,7)],flags=256),
 dict(label='unclamped-ice',controls=[(223,220,0,7)],flags=256),
 dict(label='mud-ice-boundary',controls=[(223,48,0,7),(223,160,0,8)],flags=256,split=True),
 dict(label='wind',controls=[(224,81,-33,7)],flags=512),
 dict(label='water-wind',controls=[(224,81,-33,7)],flags=512,water=True),
 dict(label='current',controls=[(225,81,-33,7)],flags=512),
 dict(label='water-current',controls=[(225,81,-33,7)],flags=512,water=True),
 dict(label='push',controls=[(226,160,0,7)],flags=512,source=5001),
 dict(label='water-push',controls=[(226,160,0,7)],flags=512,source=5001,water=True),
 dict(label='pull',controls=[(226,160,0,7)],flags=512,source=5002),
 dict(label='water-pull',controls=[(226,160,0,7)],flags=512,source=5002,water=True),
 dict(label='additive-wind-current',controls=[(224,81,-33,7),(225,-41,17,7)],flags=512),
 dict(label='mud-wind',controls=[(223,48,0,7),(224,81,-33,7)],flags=768),
 dict(label='point-occlusion',controls=[(226,160,0,8)],flags=512,source=5001,split=True,occluded=True),
 dict(label='point-cross-sector',controls=[(226,160,0,8)],flags=512,source=5001,split=True),
 dict(label='last-point-source',controls=[(226,160,0,7)],flags=512,source=5001,second_source=True),
 dict(label='negative-wind',controls=[(224,-81,33,7)],flags=512),
 dict(label='muddy-overlap',controls=[(223,160,0,7),(223,48,0,7)],flags=256),
]

def room(number,cfg):
    room=map_lumps(number,split=cfg.get('split',False));data=dict(room)
    vertices=[(x*4,y*4) for x,y in struct.iter_unpack('<hh',data['VERTEXES'])]
    lines=[list(v) for v in struct.iter_unpack('<7H',data['LINEDEFS'])]
    sides=[list(v) for v in struct.iter_unpack('<hh8s8s8sH',data['SIDEDEFS'])]
    sectors=[(0,256,'BENVFLR','BENVFLR',160,cfg['flags'],7)]
    if cfg.get('split'):sectors.append((0,0 if cfg.get('occluded') else 256,'BENVFLR','BENVFLR',160,cfg['flags'],8))
    def control(sp,dx,dy,tag,sector=0):
        vi=len(vertices);vertices.extend([(2048,2048),(2048+dx,2048+dy)])
        sd=len(sides);sides.append([0,0,name('-'),name('-'),name('-'),sector])
        lines.append([vi,vi+1,0,sp,tag,sd,65535])
    for args in cfg['controls']:control(*args)
    if cfg.get('water'):
        sector=len(sectors);sectors.append((64,256,'BENVFLR','BENVFLR',160,0,0));control(242,0,0,7,sector)
    # Source inserted after actors: its sector-list link is newest.
    things=[(-384,-192,0,1,7),(-384,192,0,3001,7)]
    if cfg.get('source'):
        things.append((128 if cfg.get('split') else 0,0,0,cfg['source'],7))
    if cfg.get('second_source'):things.append((64,64,0,5002,7))
    nodes=[list(v) for v in struct.iter_unpack('<12h2H',data['NODES'])]
    for node in nodes:
        for i in range(12):node[i]*=4
    data.update(THINGS=b''.join(struct.pack('<5h',*v) for v in things),VERTEXES=b''.join(struct.pack('<hh',*v) for v in vertices),LINEDEFS=b''.join(struct.pack('<7H',*v) for v in lines),SIDEDEFS=b''.join(struct.pack('<hh8s8s8sH',*v) for v in sides),NODES=b''.join(struct.pack('<12h2H',*v) for v in nodes),SECTORS=b''.join(struct.pack('<hh8s8shhh',f,c,name(fp),name(cp),l,s,t) for f,c,fp,cp,l,s,t in sectors),REJECT=bytes((len(sectors)**2+7)//8))
    words=[-1024,-512,17,9]+[157]*153+[0]+list(range(len(lines)))+[-1]
    data['BLOCKMAP']=struct.pack('<'+'h'*len(words),*words)
    return [(label,data[label]) for label,_ in room]

def build():
    lumps=[];cases=[]
    for number,cfg in enumerate(CONFIGS,1):
        lumps.extend(room(number,cfg))
        for actor in ['player','imp','caco']:
            for z in [0,24,64,80]:
                for mode in ['force','simulate']:
                    for moving in [False,True]:
                        c=dict(name=f'{cfg["label"]}-{actor}-{z}-{mode}-{int(moving)}',map=f'MAP{number:02}',actor=actor,mode=mode,x=-96*65536,y=0,z=z*65536,momx=3*65536+13 if moving else 0,momy=-65536-7 if moving else 0,momz=0,forward=25 if moving else 0,side=-7 if moving else 0,tics=12 if mode=='simulate' else 2,noclip=False,disable=-1,enable=-1)
                        cases.append(c)
        base=cases[-1]
        for flag in ['noclip','disable','enable']:
            c=dict(base,name=f'{cfg["label"]}-gate-{flag}',actor='player',mode='simulate',z=0,tics=8)
            c[flag]=True if flag=='noclip' else 3;cases.append(c)
        if cfg.get('source'):
            for x,y in [(0,0),(64,64),(319,0),(320,0),(321,0),(-319,0),(0,-128)]:
                cases.append(dict(base,name=f'{cfg["label"]}-radius-{x}-{y}',actor='player',mode='force',x=x*65536,y=y*65536,z=0,momx=0,momy=0,tics=1))
        if cfg.get('split') or number==3:
            for x in [-1,1,1000]:
                cases.append(dict(base,name=f'{cfg["label"]}-edge-{x}',actor='player',mode='simulate',x=x*65536,y=0,z=0,momx=4*65536,momy=0,forward=25,side=0,tics=12))
    lumps.extend([('F_START',b''),('BENVFLR',bytes([128])*4096),('F_END',b'')])
    payload=bytearray(12);directory=bytearray()
    for label,data in lumps:
        directory.extend(struct.pack('<ii8s',len(payload),len(data),name(label)));payload.extend(data)
    struct.pack_into('<4sii',payload,0,b'PWAD',len(lumps),len(payload))
    return payload+directory,cases

if __name__=='__main__':
    root=Path(__file__).resolve().parents[1]/'testdata/boom';data,cases=build()
    (root/'environment.wad').write_bytes(data)
    (root/'environment-cases.json').write_text(json.dumps(cases,indent=2)+'\n')
    print(len(cases),'cases')
