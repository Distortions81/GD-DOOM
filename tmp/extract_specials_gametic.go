package main

import (
  "bufio"
  "encoding/json"
  "fmt"
  "os"
)

type Rec struct {
  Kind     string           `json:"kind"`
  Gametic  int              `json:"gametic"`
  Specials []map[string]any `json:"specials"`
}

func main() {
  path := os.Args[1]
  want := 0
  fmt.Sscanf(os.Args[2], "%d", &want)
  f, err := os.Open(path)
  if err != nil { panic(err) }
  defer f.Close()
  s := bufio.NewScanner(f)
  s.Buffer(make([]byte, 1024), 8*1024*1024)
  for s.Scan() {
    var r Rec
    if err := json.Unmarshal(s.Bytes(), &r); err != nil || r.Kind != "tic" || r.Gametic != want {
      continue
    }
    fmt.Printf("gametic=%d specials=%d\n", r.Gametic, len(r.Specials))
    for i, sp := range r.Specials {
      fmt.Printf("idx=%d kind=%v sector=%v dir=%v type=%v low=%v high=%v floorDest=%v top=%v bottom=%v speed=%v status=%v count=%v\n", i, sp["kind"], sp["sector"], sp["direction"], sp["type"], sp["low"], sp["high"], sp["floordestheight"], sp["topheight"], sp["bottomheight"], sp["speed"], sp["status"], sp["count"])
    }
    return
  }
}
