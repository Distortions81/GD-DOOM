#!/usr/bin/env python3
"""Additional moving-sector sound cases and a translated three-room fixture."""
import struct
from make_boom_manual_cases import build as build_manual


def build(path):
    build_manual(path)
    raw=path.read_bytes();count,directory=struct.unpack_from('<ii',raw,4);lumps=[]
    dx,dy=37,-19
    for i in range(count):
        start,size,label=struct.unpack_from('<ii8s',raw,directory+16*i);data=bytearray(raw[start:start+size]);name=label.rstrip(b'\0').decode()
        if name in ['VERTEXES','THINGS','NODES','BLOCKMAP']:
            stride={'VERTEXES':4,'THINGS':10,'NODES':28,'BLOCKMAP':len(data)}[name]
            for offset in range(0,len(data),stride):
                x,y=struct.unpack_from('<hh',data,offset);struct.pack_into('<hh',data,offset,x+dx,y+dy)
                if name=='NODES':
                    for side in range(2):
                        pos=offset+8+side*8;top,bottom,left,right=struct.unpack_from('<hhhh',data,pos)
                        struct.pack_into('<hhhh',data,pos,top+dy,bottom+dy,left+dx,right+dx)
        lumps.append((label,data))
    payload=bytearray(bytes(12));directory=bytearray()
    for label,data in lumps:directory.extend(struct.pack('<ii8s',len(payload),len(data),label));payload.extend(data)
    struct.pack_into('<4sii',payload,0,b'PWAD',len(lumps),len(payload));path.write_bytes(payload+directory)


def cases():
    result=[]
    def add(name,special,**extra):
        c=dict(name=name,special=special,second=special,kind=0,age=0,actor=0,activator=0,side=0,secret=0,keys=1,tics=512,at2=180,at3=360,late=-1,floors=[0,64*65536,96*65536],ceilings=[256*65536,192*65536,256*65536])
        c.update(extra);result.append(c)
    # Door kind, speed and wait combinations, including locked doors, both
    # opening-at-destination and normal movement, plus blocked close reversals.
    for speed in range(4):
        for kind in range(4):
            special=0x3c03|speed<<3|kind<<5
            for actor in [0,1,2]:
                add(f'door_s{speed}_k{kind}_a{actor}',special,actor=actor)
                add(f'door_idle_s{speed}_k{kind}_a{actor}',special,actor=actor,ceilings=[256*65536,252*65536,256*65536])
        for actor in [0,2]:add(f'locked_s{speed}_a{actor}',0x3803|speed<<3,actor=actor)
    for base in [0x6000,0x4000]:
        for speed in range(4):
            for up in range(2):
                for actor in [0,2]:add(f'plane_{base}_s{speed}_u{up}_a{actor}',base|3|speed<<3|up<<6|6<<7,actor=actor)
    for speed in range(4):
        for target in range(4):
            for actor in [0,2]:add(f'lift_s{speed}_t{target}_a{actor}',0x3403|speed<<3|target<<8,actor=actor)
    # Vanilla raise/change platforms have a one-shot STNMOV activation followed
    # by the moving loop; blocked ascent reverses with PSTART. Toggles stay quiet.
    for special in [14,15,20,21,62,66,67,68,87,88,89,120,123,211,212]:
        for actor in [0,1,2]:
            add(f'plat_{special}_a{actor}',special,actor=actor)
            add(f'plat_block_{special}_a{actor}',special,actor=actor,ceilings=[256*65536,136*65536,256*65536])
    result += [dict(c,name='rng1_'+c['name']) for c in result if c['name'].startswith('lift_') and '_t3_' in c['name']]
    return result
