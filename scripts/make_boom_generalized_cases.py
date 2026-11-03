#!/usr/bin/env python3
"""Deterministic generalized stair/crusher cases on the original legacy fixture."""
import json
from pathlib import Path
from make_boom_legacy_wad import build


def cases():
    result=[]
    def add(name,special,**extra):
        stair=special>=0x3000
        c=dict(name=name,special=special,actor=0,activator=0,side=0,tag=7,busy=0,secret=0,position=0,target=0 if stair and special&7<6 else 1,tics=32,retry=20,late=-1,stop=-1,resume=-1,retag=0,floors=[0,0,0],ceilings=[96*65536]*3,flats=['FLOOR0_1']*3)
        c.update(extra);result.append(c)
    for special in range(0x2f80,0x3400):add(f'encoded_{special}',special)
    for speed in range(4):
        for step in range(4):
            for up in range(2):
                for ignore in range(2):
                    for trigger in [3,7]:
                        special=0x3000|speed<<3|step<<6|up<<8|ignore<<9|trigger
                        for actor in range(3):
                            add(f'stairs_{special}_a{actor}',special,actor=actor,tics=432,late=400,ceilings=[256*65536,(56 if actor else 256)*65536,256*65536])
                        add(f'texture_{special}',special,tics=432,late=400,flats=['FLOOR0_1','FLOOR0_1','FLOOR0_3'],floors=[-32*65536,-16*65536,0],ceilings=[256*65536]*3)
    for special in range(0x2f80,0x3000):
        for actor in range(3):add(f'crusher_{special}_a{actor}',special,actor=actor,tics=512,late=400)
    for stair in [False,True]:
        for trigger in range(8):
            for monsters in range(2):
                special=(0x3140 if stair else 0x2f80)|monsters<<5|trigger
                for label,extra in [('monster',dict(activator=1)),('back',dict(side=1)),('zero',dict(tag=0)),('missing',dict(tag=99)),('floorbusy',dict(busy=1)),('ceilbusy',dict(busy=2)),('elevatorbusy',dict(busy=3)),('secret_monster',dict(secret=1,activator=1))]:
                    add(f'{label}_{special}',special,tics=48,**extra)
    for speed in range(4):
        for silent in range(2):
            for trigger in [3,7]:
                special=0x2f80|speed<<3|silent<<6|trigger
                for actor in [1,2]:
                    add(f'stasis_{special}_a{actor}',special,actor=actor,tics=512,retry=-1,late=-1,stop=60,resume=100)
                    add(f'retag_{special}_a{actor}',special,actor=actor,tics=160,retry=-1,late=-1,stop=60,resume=100,retag=1)
    for speed in range(4):
        for up in [0,1]:
            special=0x3003|speed<<3|up<<8
            add(f'skipbusy_{special}',special,busy=4,tics=192,late=100)
    for trigger in [2,3,4,5,6,7]:
        for actor in [1,2]:
            special=0x2fa0|2<<3|trigger
            for position in [1,2]:add(f'edge_{special}_{actor}_{position}',special,actor=actor,position=position,tics=128,floors=[0,0,32*65536],ceilings=[256*65536,96*65536,256*65536])
    return result

if __name__=='__main__':
    root=Path(__file__).resolve().parents[1]/'testdata/boom'
    build(root/'generalized-movers.wad')
    (root/'generalized-mover-cases.json').write_text(json.dumps(cases(),indent=2)+'\n')
