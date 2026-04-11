package main
import (
  "fmt"
  "gddoom/internal/music"
)
func main(){
  d:=music.NewDriver(49716,nil)
  _=d
  for _, n := range []int{36,48,60,72} {
    fw := call(n)
    fnum := int(fw & 0x03ff)
    block := int((fw>>10)&0x07)
    hz1 := float64(fnum) * 49716.0 * float64(uint32(1)<<(block+2)) / float64(uint32(1)<<19)
    hz2 := float64(fnum) * 49716.0 * float64(uint32(1)<<(block-1)) / float64(uint32(1)<<19)
    fmt.Printf("note=%d fw=%#x fnum=%d block=%d hz_now=%.3f hz_old=%.3f\n", n, fw, fnum, block, hz1, hz2)
  }
}
func call(note int) uint16 { return exposed(note) }
