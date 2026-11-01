#!/usr/bin/env python3
"""Original disconnected rooms with differently oriented, stepped teleport exits."""
from pathlib import Path
import json
import math
import struct
from make_boom_smoke_wad import name

SPECIALS=[207,208,209,210,243,244,262,263,264,265,266,267,268,269]

def room(number,orientation,floors):
    source=[(-448,256),(-448,-256)]
    polygons=[ [(-768,-256),(-768,256),source[0],source[1]], [source[1],source[0],(-128,256),(-128,-256)] ]
    if orientation in (0,1):
        dest=[(448,256),(448,-256)]
        polygons += [[(128,-256),(128,256),dest[0],dest[1]], [dest[1],dest[0],(768,256),(768,-256)]]
    elif orientation in (2,3):
        dest=[(128,0),(768,0)]
        polygons += [[(128,-256),dest[0],dest[1],(768,-256)], [dest[0],(128,256),(768,256),dest[1]]]
    else:
        dest=[(128,256),(768,-256)]
        polygons += [[dest[0],dest[1],(128,-256)], [dest[1],dest[0],(768,256)]]
    if orientation%2:dest=dest[::-1]
    vertices=[];lines=[];sides=[];segs=[];subsectors=[];edge_indices={}
    def vertex(p):
        if p not in vertices:vertices.append(p)
        return vertices.index(p)
    for sector,poly in enumerate(polygons):
        first=len(segs)
        for a,b in zip(poly,poly[1:]+poly[:1]):
            va,vb=vertex(a),vertex(b);key=tuple(sorted((va,vb)))
            side=len(sides);sides.append([0,0,name('-'),name('-'),name('STARTAN3'),sector])
            if key in edge_indices:
                li=edge_indices[key];line=lines[li];line[2]=4;line[6]=side;sides[line[5]][4]=name('-');sides[side][4]=name('-');direction=1
            else:
                li=len(lines);edge_indices[key]=li;lines.append([va,vb,1,0,0,side,65535]);direction=0
            angle=int(math.atan2(b[1]-a[1],b[0]-a[0])*65536/(2*math.pi))&65535
            segs.append([va,vb,angle,li,direction,0])
        subsectors.append([len(segs)-first,first])
    def line_for(a,b):return edge_indices[tuple(sorted((vertices.index(a),vertices.index(b))))]
    source_index=line_for(*source);dest_index=line_for(*dest)
    d=lines[dest_index]
    if vertices[d[0]]!=dest[0]:
        d[0],d[1],d[5],d[6]=d[1],d[0],d[6],d[5]
        for s in segs:
            if s[3]==dest_index:s[4]^=1
    lines[source_index][3:5]=[244,17];lines[dest_index][4]=17
    def bbox(poly):
        return [max(y for x,y in poly),min(y for x,y in poly),min(x for x,y in poly),max(x for x,y in poly)]
    def node(a,b,front,back,children):
        return struct.pack('<12h2H',a[0],a[1],b[0]-a[0],b[1]-a[1],*bbox(front),*bbox(back),*children)
    nodes=node(*source,polygons[0],polygons[1],[0x8000,0x8001])
    fi,bi=(3,2) if orientation%2 else (2,3)
    nodes+=node(*dest,polygons[fi],polygons[bi],[0x8000+fi,0x8000+bi])
    nodes+=node((0,0),(0,1),sum(polygons[2:],[]),sum(polygons[:2],[]),[1,0])
    sectors=[(0,256,0),(0,256,0),(floors[0],256,0),(floors[1],256,17)]
    things=[(-704,-192,0,1,7),(-464,64,45,3001,7),(-704,192,135,3001,7),(640,128,135,14,7)]
    words=[-768,-256,13,5]+[69]*65+[0]+list(range(len(lines)))+[-1]
    lumps=[(f'MAP{number:02}',b''),('THINGS',b''.join(struct.pack('<5h',*v) for v in things)),('LINEDEFS',b''.join(struct.pack('<7H',*v) for v in lines)),('SIDEDEFS',b''.join(struct.pack('<hh8s8s8sH',*v) for v in sides)),('VERTEXES',b''.join(struct.pack('<hh',*v) for v in vertices)),('SEGS',b''.join(struct.pack('<6H',*v) for v in segs)),('SSECTORS',b''.join(struct.pack('<2H',*v) for v in subsectors)),('NODES',nodes),('SECTORS',b''.join(struct.pack('<hh8s8shhh',f,c,name('FLOOR0_1'),name('CEIL1_1'),160,0,tag) for f,c,tag in sectors)),('REJECT',bytes(2)),('BLOCKMAP',struct.pack('<'+'h'*len(words),*words))]
    return lumps,source_index,dest_index

def transform_room(lumps, diagonal):
    def point(x,y):return (x-y,x+y) if diagonal else (-y,x)
    data=dict(lumps)
    for label,fmt in [('VERTEXES','<2h'),('THINGS','<5h')]:
        values=[]
        for row in struct.iter_unpack(fmt,data[label]):
            row=list(row);row[:2]=point(*row[:2])
            if label=='THINGS':row[2]=(row[2]+(45 if diagonal else 90))%360
            values.append(row)
        data[label]=b''.join(struct.pack(fmt,*v) for v in values)
    vertices=list(struct.iter_unpack('<2h',data['VERTEXES']))
    segs=[list(v) for v in struct.iter_unpack('<6H',data['SEGS'])]
    for seg in segs:
        a,b=vertices[seg[0]],vertices[seg[1]]
        seg[2]=int(math.atan2(b[1]-a[1],b[0]-a[0])*65536/(2*math.pi))&65535
    data['SEGS']=b''.join(struct.pack('<6H',*v) for v in segs)
    nodes=[]
    for row in struct.iter_unpack('<12h2H',data['NODES']):
        row=list(row);row[:2]=point(*row[:2]);row[2:4]=point(*row[2:4])
        for pos in [4,8]:
            top,bottom,left,right=row[pos:pos+4]
            corners=[point(x,y) for x in [left,right] for y in [bottom,top]]
            row[pos:pos+4]=[max(y for x,y in corners),min(y for x,y in corners),min(x for x,y in corners),max(x for x,y in corners)]
        nodes.append(row)
    data['NODES']=b''.join(struct.pack('<12h2H',*v) for v in nodes)
    left,bottom=min(x for x,y in vertices),min(y for x,y in vertices)
    width=(max(x for x,y in vertices)-left)//128+1;height=(max(y for x,y in vertices)-bottom)//128+1
    count=len(data['LINEDEFS'])//14
    words=[left,bottom,width,height]+[4+width*height]*(width*height)+[0]+list(range(count))+[-1]
    data['BLOCKMAP']=struct.pack('<'+'h'*len(words),*words)
    return [(label,data[label]) for label,_ in lumps]

def build():
    lumps=[];cases=[]
    for orientation in range(6):
        for profile,floors in enumerate([(32,64),(64,32),(48,48)]):
            number=orientation*3+profile+1
            data,src,dst=room(number,orientation,floors);lumps.extend(data)
            for special in SPECIALS:
                for actor in ['player','imp','caco']:
                    # A fixed-point position inside the source line, airborne
                    # offset and oblique, signed momentum expose rotation errors.
                    cases.append(dict(name=f'{number:02}-{special}-{actor}',map=f'MAP{number:02}',source=src,destination=dst,special=special,actor=actor,back=False,blocked=False,missing=False,z=13*65536+31,y=64*65536+123,momx=7*65536+13,momy=-3*65536-17,momz=65536+7,angle=0x12345678,ticks=0))
    # Negative paths retain one-shot specials. No destination, blocked first
    # destination, back-side crossing and missiles exercise the real gate.
    base=cases[0]
    for special in SPECIALS:
        for actor in ['player','imp']:
            for flag in ['back','blocked','missing']:
                c=dict(base,name=f'gate-{special}-{actor}-{flag}',special=special,actor=actor);c[flag]=True;cases.append(c)
        cases.append(dict(base,name=f'gate-{special}-missile',special=special,actor='missile'))
    # Momentum movement uses actual monster thinkers with state advancement
    # delayed, so no AI randomness obscures the teleport physics comparison.
    for special in SPECIALS:
        for ticks in [1,2,5]:
            cases.append(dict(base,name=f'move-{special}-{ticks}',special=special,actor='imp',ticks=ticks))
    for special in SPECIALS:
        for actor in ['player','imp']:
            for grounded in [False,True]:
                cases.append(dict(base,name=f'height-{special}-{actor}-{int(grounded)}',special=special,actor=actor,z=0 if grounded else 9*65536+37,y=-117*65536-31,momx=2*65536+3,momy=5*65536+19,momz=0 if grounded else -2*65536-5))
    for c in cases:
        c.update(x=-449*65536,nextx=-447*65536,nexty=c['y'])
    for diagonal in [False,True]:
        for orientation in range(6):
            number=19+int(diagonal)*6+orientation
            data,src,dst=room(number,orientation,(32,64))
            lumps.extend(transform_room(data,diagonal))
            for special in SPECIALS:
                for actor in ['player','imp','caco']:
                    c=dict(cases[0],name=f'source-{number}-{special}-{actor}',map=f'MAP{number:02}',source=src,destination=dst,special=special,actor=actor)
                    x,y,nx,ny=c['x'],c['y'],c['nextx'],c['nexty'];mx,my=c['momx'],c['momy']
                    if diagonal:c.update(x=x-y,y=x+y,nextx=nx-ny,nexty=nx+ny,momx=mx-my,momy=mx+my)
                    else:c.update(x=-y,y=x,nextx=-ny,nexty=nx,momx=-my,momy=mx)
                    c['angle']=(c['angle']+(0x20000000 if diagonal else 0x40000000))&0xffffffff
                    cases.append(c)
    payload=bytearray(12);directory=bytearray()
    for label,data in lumps:
        directory.extend(struct.pack('<ii8s',len(payload),len(data),name(label)));payload.extend(data)
    struct.pack_into('<4sii',payload,0,b'PWAD',len(lumps),len(payload))
    return payload+directory,cases

if __name__=='__main__':
    root=Path(__file__).resolve().parents[1]/'testdata/boom';data,cases=build()
    (root/'teleports.wad').write_bytes(data)
    (root/'teleport-cases.json').write_text(json.dumps(cases,indent=2)+'\n')
    print(root/'teleports.wad',len(cases),'cases')
