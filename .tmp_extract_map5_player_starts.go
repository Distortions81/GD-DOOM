package main

import (
  "fmt"
  "gddoom/internal/mapdata"
  "gddoom/internal/wad"
)

func main() {
  w, err := wad.Load("/home/dist/github/GD-DOOM/doom.wad")
  if err != nil { panic(err) }
  m, err := mapdata.Load(w, "E1M5")
  if err != nil { panic(err) }
  for i, th := range m.Things {
    if th.Type >= 1 && th.Type <= 4 {
      fmt.Printf("thing[%d] playerstart type=%d x=%d y=%d angle=%d flags=%d\n", i, th.Type, th.X, th.Y, th.Angle, th.Flags)
    }
  }
}
