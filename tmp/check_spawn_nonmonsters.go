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

func load(path string) Rec {
  f, err := os.Open(path)
  if err != nil { panic(err) }
  defer f.Close()
  s := bufio.NewScanner(f)
  s.Buffer(make([]byte, 1024), 8*1024*1024)
  for s.Scan() {
    var r Rec
    if err := json.Unmarshal(s.Bytes(), &r); err != nil {
      continue
    }
    if r.Kind == "tic" && r.Gametic == 0 {
      return r
    }
  }
  panic("gametic 0 not found")
}

func main() {
  left := load("/tmp/linuxdoom-demo1-trace.jsonl")
  right := load("/tmp/gddoom-demo1-trace.jsonl")
  for i := 0; i < len(left.Mobjs) && i < len(right.Mobjs); i++ {
    if left.Mobjs[i]["x"] != right.Mobjs[i]["x"] || left.Mobjs[i]["y"] != right.Mobjs[i]["y"] {
      fmt.Printf("first_xy_mismatch idx=%d left_type=%v right_type=%v left=(%v,%v) right=(%v,%v)\n", i, left.Mobjs[i]["type"], right.Mobjs[i]["type"], left.Mobjs[i]["x"], left.Mobjs[i]["y"], right.Mobjs[i]["x"], right.Mobjs[i]["y"])
      return
    }
  }
  fmt.Println("all_initial_xy_match")
}
