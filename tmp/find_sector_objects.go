package main

import (
  "bufio"
  "encoding/json"
  "fmt"
  "os"
)

type Rec struct { Kind string `json:"kind"`; Gametic int `json:"gametic"`; Mobjs []map[string]any `json:"mobjs"` }

func main(){ path:=os.Args[1]; want:=0; sector:=0; fmt.Sscanf(os.Args[2], "%d", &want); fmt.Sscanf(os.Args[3], "%d", &sector); f,_:=os.Open(path); defer f.Close(); s:=bufio.NewScanner(f); s.Buffer(make([]byte,1024),8*1024*1024); for s.Scan(){ var r Rec; if err:=json.Unmarshal(s.Bytes(), &r); err!=nil || r.Kind!="tic" || r.Gametic!=want { continue }; fmt.Printf("gametic=%d sector=%d\n", r.Gametic, sector); found:=0; for i,m := range r.Mobjs { sv,ok:=m["sector"].(float64); if !ok || int(sv)!=sector { continue }; fmt.Printf("idx=%d type=%v x=%v y=%v z=%v floorz=%v angle=%v\n", i, m["type"], m["x"], m["y"], m["z"], m["floorz"], m["angle"]); found++ }; fmt.Printf("found=%d\n", found); return } }
