package main

import (
  "fmt"
  "os"
  "gddoom/internal/wad"
)

func main() {
  wf, err := wad.Open("/home/dist/github/GD-DOOM/doom.wad")
  if err != nil { panic(err) }
  l, ok := wf.LumpByName("DEMO1")
  if !ok { panic("missing DEMO1") }
  data, err := wf.LumpData(l)
  if err != nil { panic(err) }
  out := "/home/dist/github/GD-DOOM/tmp/demo1.lmp"
  if err := os.WriteFile(out, data, 0o644); err != nil { panic(err) }
  fmt.Println(out, len(data))
}
