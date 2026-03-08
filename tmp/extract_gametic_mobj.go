package main

import (
  "bufio"
  "encoding/json"
  "fmt"
  "os"
)

type Rec struct { Kind string `json:"kind"`; Gametic int `json:"gametic"`; Mobjs []map[string]any `json:"mobjs"` }

func main() {
  path := os.Args[1]
  want := 0
  idx := 0
  fmt.Sscanf(os.Args[2], "%d", &want)
  fmt.Sscanf(os.Args[3], "%d", &idx)
  f, _ := os.Open(path)
  defer f.Close()
  s := bufio.NewScanner(f)
  s.Buffer(make([]byte, 1024), 8*1024*1024)
  for s.Scan() {
    var r Rec
    if err := json.Unmarshal(s.Bytes(), &r); err != nil || r.Kind != "tic" || r.Gametic != want {
      continue
    }
    b, _ := json.MarshalIndent(r.Mobjs[idx], "", "  ")
    fmt.Printf("gametic=%d\n%s\n", r.Gametic, b)
    return
  }
}
