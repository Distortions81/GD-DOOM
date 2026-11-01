#!/usr/bin/env python3
"""Original layered/crossing masked walls and actors for Boom TRANMAP tests."""
from pathlib import Path
import json
import math
import struct
from make_boom_smoke_wad import name


def patch(width,height,index,holes=False,origin=False):
    columns=[]
    for x in range(width):
        columns.append(bytes([255]) if holes and x%32>=24 else bytes([0,height,0])+bytes([index])*height+bytes([0,255]))
    offsets=[];offset=8+width*4
    for col in columns:offsets.append(offset);offset+=len(col)
    return struct.pack('<4h',width,height,width//2 if origin else 0,height if origin else 0)+struct.pack('<'+'I'*width,*offsets)+b''.join(columns)


def clip(poly,a,b,side):
    def cross(p):return (b[0]-a[0])*(p[1]-a[1])-(b[1]-a[1])*(p[0]-a[0])
    out=[]
    for i,p in enumerate(poly):
        q=poly[(i+1)%len(poly)];cp,cq=cross(p),cross(q)
        inside=cp<=1e-7 if side==0 else cp>=-1e-7
        other=cq<=1e-7 if side==0 else cq>=-1e-7
        if inside:out.append(p)
        if inside!=other:
            t=cp/(cp-cq);out.append((p[0]+t*(q[0]-p[0]),p[1]+t*(q[1]-p[1])))
    clean=[]
    for p in out:
        p=tuple(round(v) for v in p)
        if not clean or clean[-1]!=p:clean.append(p)
    if len(clean)>1 and clean[0]==clean[-1]:clean.pop()
    return clean


def map_lumps(number):
    crossing=number==5
    panes=[((64,128),(192,-128)),((192,128),(64,-128))] if crossing else [((0,128),(0,-128)),((128,128),(128,-128))]
    outer=[(-256,-128),(-256,128),(512,128),(512,-128)]
    vertices=[]
    def vertex(p):
        if p not in vertices:vertices.append(p)
        return vertices.index(p)
    sides=[];lines=[]
    def line(a,b,texture,back=False,tag=0,special=0,sector=0):
        front=len(sides);sides.append((0,0,'-','-',texture,sector));other=65535
        if back:other=len(sides);sides.append((0,0,'-','-',texture,sector))
        lines.append((vertex(a),vertex(b),4 if back else 1,special,tag,front,other))
        return len(lines)-1
    for i,p in enumerate(outer):line(p,outer[(i+1)%4],'BGBACK')
    for i,(a,b) in enumerate(panes):
        # Direct tag-zero 260 retains a normal middle texture. Tagged control
        # lines below supply named maps without consuming these textures.
        line(a,b,('BGFULL' if crossing else 'BGLASSA') if i==0 else 'BGLASSB',True,17+i if number in (2,3) else 0,260 if number in (1,4,5) and not(number==4 and i==0) else 0)
    if number in (2,3):
        for i in range(2):line((-256,-128),(-256,-128),'CUSTMAP' if i==0 or number==2 else 'REVMAP',False,17+i,260)
    segs=[];subsectors=[];nodes=[]
    def bounds(poly):return (max(p[1] for p in poly),min(p[1] for p in poly),min(p[0] for p in poly),max(p[0] for p in poly))
    def leaf(poly):
        first=len(segs)
        for i,p in enumerate(poly):
            q=poly[(i+1)%len(poly)]
            for li,ld in enumerate(lines[:6]):
                a,b=vertices[ld[0]],vertices[ld[1]];dx,dy=b[0]-a[0],b[1]-a[1]
                if dx==dy==0:continue
                def on(v):return abs(dx*(v[1]-a[1])-dy*(v[0]-a[0]))<1e-6 and min(a[0],b[0])<=v[0]<=max(a[0],b[0]) and min(a[1],b[1])<=v[1]<=max(a[1],b[1])
                if on(p) and on(q):
                    direction=0 if (q[0]-p[0])*dx+(q[1]-p[1])*dy>0 else 1
                    start=a if direction==0 else b
                    offset=int(math.hypot(p[0]-start[0],p[1]-start[1]))
                    angle=int(math.atan2(q[1]-p[1],q[0]-p[0])*65536/(2*math.pi))&65535
                    segs.append((vertex(p),vertex(q),angle,li,direction,offset));break
            else:raise ValueError(('unmatched edge',p,q))
        index=len(subsectors);subsectors.append((len(segs)-first,first));return 0x8000|index
    def bsp(poly,depth):
        if depth==len(panes):return leaf(poly)
        a,b=panes[depth];children=[clip(poly,a,b,side) for side in (0,1)]
        if len(children[0])<3:return bsp(children[1],depth+1)
        if len(children[1])<3:return bsp(children[0],depth+1)
        ids=[bsp(child,depth+1) for child in children]
        index=len(nodes);nodes.append((a[0],a[1],b[0]-a[0],b[1]-a[1],*bounds(children[0]),*bounds(children[1]),*ids));return index
    bsp(outer,0)
    things=[(-192,0,0,1,7),(-64,72,0,48,7),(96 if crossing else 64,0,0,48,7),(320 if crossing else 256,-64,0,48,7)]
    sectors=[(0,128,'BGFLOOR','BGCEIL',160,0,0)]
    words=[-256,-128,7,3]+[25]*21+[0]+list(range(len(lines)))+[-1]
    return [(f'MAP{number:02}',b''),('THINGS',b''.join(struct.pack('<5h',*v) for v in things)),('LINEDEFS',b''.join(struct.pack('<7H',*v) for v in lines)),('SIDEDEFS',b''.join(struct.pack('<hh8s8s8sH',x,y,name(t),name(b),name(m),sec) for x,y,t,b,m,sec in sides)),('VERTEXES',b''.join(struct.pack('<hh',*v) for v in vertices)),('SEGS',b''.join(struct.pack('<6H',*v) for v in segs)),('SSECTORS',b''.join(struct.pack('<2H',*v) for v in subsectors)),('NODES',b''.join(struct.pack('<12h2H',*v) for v in nodes)),('SECTORS',b''.join(struct.pack('<hh8s8shhh',f,c,name(fp),name(cp),l,sp,tag) for f,c,fp,cp,l,sp,tag in sectors)),('REJECT',bytes(1)),('BLOCKMAP',struct.pack('<'+'h'*len(words),*words))]


def texture(label,width,height,p):return struct.pack('<8sIhhIh',name(label),0,width,height,0,1)+struct.pack('<5h',0,0,p,0,0)


def build(override=False):
    lumps=[]
    for n in range(1,6):lumps.extend(map_lumps(n))
    labels=['BGBACK','BGLASSA','BGLASSB','BGFULL','RSKY1']
    lumps.extend([('P_START',b''),('BGBACK',patch(64,128,128)),('BGLASSA',patch(64,128,112,True)),('BGLASSB',patch(64,128,160)),('BGFULL',patch(64,128,112)),('P_END',b'')])
    textures=[texture('BGNULL',64,128,0)]+[texture(label,64,128,i) for i,label in enumerate(labels[:4])]+[texture(f'SKY{i}',256,128,4) for i in (1,2,3)]
    offset=4+4*len(textures);offsets=[]
    for t in textures:offsets.append(offset);offset+=len(t)
    lumps.append(('PNAMES',struct.pack('<I',len(labels))+b''.join(name(l) for l in labels)))
    lumps.append(('TEXTURE1',struct.pack('<I',len(textures))+struct.pack('<'+'I'*len(offsets),*offsets)+b''.join(textures)))
    lumps.extend([('F_START',b''),('BGFLOOR',bytes([128])*4096),('BGCEIL',bytes([176])*4096),('F_END',b''),('S_START',b''),('ELECA0',patch(32,80,208,False,True)),('S_END',b'')])
    table=bytes((d*3+s*5)//8 for d in range(256) for s in range(256))
    reverse=bytes((d*5+s*3)//8 for d in range(256) for s in range(256))
    lumps.extend([('CUSTMAP',table),('REVMAP',reverse)])
    if override:lumps.append(('TRANMAP',reverse))
    out,directory=bytearray(bytes(12)),bytearray()
    for label,data in lumps:directory.extend(struct.pack('<ii8s',len(out),len(data),name(label)));out.extend(data)
    struct.pack_into('<4sii',out,0,b'PWAD',len(lumps),len(out));return out+directory


def scenes():
    views=[]
    for number in range(1,6):
        for suffix,x,y,angle in [('front',-192,0,0),('angled',-192,32,0),('reverse',448,0,2147483648)]:
            views.append(dict(name=f'trans-{number}-{suffix}',map=f'MAP{number:02}',X=x,Y=y,Z=41,angle=angle,tic=0,category='boom-translucency',checks=[dict(name='layer-center',box=[150,85,170,100]),dict(name='layer-left',box=[110,85,125,100]),dict(name='layer-right',box=[200,85,215,100])]))
    for view in views:
        if view['name']=='trans-5-reverse':
            view['checks'][0]['max_different_percent']=5
            view['checks'][0]['note']='One-column crossing-wall raster boundary; other checks require exact pixels.'
    return views


if __name__=='__main__':
    root=Path(__file__).resolve().parents[1]/'testdata/boom'
    (root/'translucency.wad').write_bytes(build())
    (root/'translucency-override.wad').write_bytes(build(True))
    (root/'translucency-scenes.json').write_text(json.dumps(scenes(),indent=2)+'\n')
    override=scenes()
    for view in override:
        if view['name'] in ('trans-1-angled','trans-5-angled'):
            view['checks'][2]['max_different_percent']=100/15
            view['checks'][2]['note']='One-column sprite/wall raster boundary exposed by the asymmetric override.'
    (root/'translucency-override-scenes.json').write_text(json.dumps(override,indent=2)+'\n')
    print(root/'translucency.wad')
