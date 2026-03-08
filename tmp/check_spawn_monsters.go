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

func monsterLike(t float64) bool {
  return t == 1 || t == 2 || t == 9 || t == 11 || t == 12 || t == 13 || t == 3001 || t == 3002 || t == 3003 || t == 3004 || t == 3005 || t == 3006 || t == 7 || t == 16 || t == 58 || t == 64 || t == 65 || t == 66 || t == 67 || t == 68 || t == 69 || t == 71 || t == 84
}

func main() {
  left := load("/tmp/linuxdoom-demo1-trace.jsonl")
  right := load("/tmp/gddoom-demo1-trace.jsonl")
  if len(left.Mobjs) != len(right.Mobjs) {
    fmt.Printf("mobj count mismatch left=%d right=%d\n", len(left.Mobjs), len(right.Mobjs))
  }
  mismatches := 0
  checked := 0
  limit := len(left.Mobjs)
  if len(right.Mobjs) < limit { limit = len(right.Mobjs) }
  for i := 0; i < limit; i++ {
    lt, lok := left.Mobjs[i]["type"].(float64)
    rt, rok := right.Mobjs[i]["type"].(float64)
    if !lok || !rok || lt != rt || !monsterLike(lt) {
      continue
    }
    checked++
    fields := []string{"x", "y", "z", "floorz", "sector", "angle"}
    bad := false
    for _, field := range fields {
      if left.Mobjs[i][field] != right.Mobjs[i][field] {
        if !bad {
          fmt.Printf("idx=%d type=%.0f", i, lt)
          bad = true
          mismatches++
        }
        fmt.Printf(" %s left=%v right=%v", field, left.Mobjs[i][field], right.Mobjs[i][field])
      }
    }
    if bad {
      fmt.Println()
    }
  }
  fmt.Printf("checked_monsters=%d mismatched_monsters=%d\n", checked, mismatches)
}
