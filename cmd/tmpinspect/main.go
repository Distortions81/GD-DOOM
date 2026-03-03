package main

import (
  "fmt"
  "gddoom/internal/mapdata"
  "gddoom/internal/wad"
)

func secForSide(m *mapdata.Map, side int16) int {
  if side < 0 || int(side) >= len(m.Sidedefs) { return -1 }
  return int(m.Sidedefs[int(side)].Sector)
}

func main(){
  wf,_:=wad.Open("DOOM1.WAD")
  m,_:=mapdata.LoadMap(wf,"E1M1")
  for _, li := range []int{151,152} {
    ld := m.Linedefs[li]
    v1:=m.Vertexes[ld.V1]
    v2:=m.Vertexes[ld.V2]
    fmt.Printf("L%d v1=(%d,%d) v2=(%d,%d) sides=[%d,%d] sec=[%d,%d] flags=0x%04x\n", li, v1.X,v1.Y,v2.X,v2.Y, ld.SideNum[0],ld.SideNum[1], secForSide(m,ld.SideNum[0]),secForSide(m,ld.SideNum[1]), ld.Flags)
  }
  // door sector bbox
  dsec:=4
  minX,minY:=int16(32767),int16(32767)
  maxX,maxY:=int16(-32768),int16(-32768)
  for _, ld := range m.Linedefs {
    if secForSide(m,ld.SideNum[0])==dsec || secForSide(m,ld.SideNum[1])==dsec {
      for _, vi := range []uint16{ld.V1,ld.V2} {
        v:=m.Vertexes[vi]
        if v.X<minX {minX=v.X}; if v.X>maxX {maxX=v.X}
        if v.Y<minY {minY=v.Y}; if v.Y>maxY {maxY=v.Y}
      }
    }
  }
  fmt.Printf("door sector bbox approx x:[%d,%d] y:[%d,%d]\n", minX,maxX,minY,maxY)
}
