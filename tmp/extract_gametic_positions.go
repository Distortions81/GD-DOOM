package main

import (
  "bufio"
  "encoding/json"
  "fmt"
  "os"
)

type Rec struct {
  Kind    string           `json:"kind"`
  Gametic int              `json:"gametic"`
  Mobjs   []map[string]any `json:"mobjs"`
}

func main() {
  path := os.Args[1]
  want := 0
  fmt.Sscanf(os.Args[2], "%d", &want)
  f, _ := os.Open(path)
  defer f.Close()
  s := bufio.NewScanner(f)
  s.Buffer(make([]byte, 1024), 8*1024*1024)
  for s.Scan() {
    var r Rec
    if err := json.Unmarshal(s.Bytes(), &r); err != nil || r.Kind != "tic" || r.Gametic != want {
      continue
    }
    fmt.Printf("gametic=%d mobj_count=%d\n", r.Gametic, len(r.Mobjs))
    for i := 0; i < len(r.Mobjs) && i < 20; i++ {
      m := r.Mobjs[i]
      fmt.Printf("idx=%d type=%v x=%v y=%v z=%v floorz=%v sector=%v angle=%v\n", i, m["type"], m["x"], m["y"], m["z"], m["floorz"], m["sector"], m["angle"])
    }
    return
  }
}
