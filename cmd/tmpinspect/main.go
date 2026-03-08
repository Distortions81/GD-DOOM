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
  wf,_:=wad.Open("/home/dist/github/GD-DOOM/doom.wad")
  m,_:=mapdata.LoadMap(wf,"E1M5")
  for i, th := range m.Things {
    x:=int64(th.X)<<16
    y:=int64(th.Y)<<16
    if x == -4194304 && y == 16777216 {
      fmt.Printf("thing idx=%d type=%d angle=%d flags=0x%04x\n", i, th.Type, th.Angle, uint16(th.Flags))
    }
  }
}
