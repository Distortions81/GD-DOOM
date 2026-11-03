#!/usr/bin/env python3
"""Build a two-sector locked-door dispatcher fixture with a real imp."""
from pathlib import Path
import struct
from make_boom_transfer_wad import map_lumps
from make_boom_smoke_wad import name as lump_name

def build(path):
    lumps = map_lumps(1, split=True)
    result=[]
    for name, raw in lumps:
        data=bytearray(raw)
        if name=='THINGS':data.extend(struct.pack('<5h',-192,64,0,3001,7))
        if name=='SECTORS':
            struct.pack_into('<h',data,26+2,32)
            struct.pack_into('<h',data,26+24,7)
        if name=='LINEDEFS':struct.pack_into('<H',data,2*14+8,7)
        result.append((name,bytes(data)))
    payload=bytearray(bytes(12));directory=bytearray()
    for name,data in result:
        directory.extend(struct.pack('<ii8s',len(payload),len(data),lump_name(name)))
        payload.extend(data)
    struct.pack_into('<4sii',payload,0,b'PWAD',len(result),len(payload))
    path.write_bytes(payload+directory)

if __name__=='__main__':build(Path(__file__).resolve().parents[1]/'testdata/boom/locks.wad')
