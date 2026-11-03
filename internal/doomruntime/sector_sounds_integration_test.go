//go:build integration

package doomruntime

import (
	"bufio"
	"fmt"
	"os"
	"testing"
)

type sectorSoundTrace struct{ out *bufio.Writer }

func newSectorSoundTrace(t *testing.T) *sectorSoundTrace {
	t.Helper()
	trace := &sectorSoundTrace{}
	if path := os.Getenv("GD_SECTOR_SOUND_OUT"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		trace.out = bufio.NewWriter(f)
		t.Cleanup(func() {
			if err := trace.out.Flush(); err != nil {
				t.Error(err)
			}
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		})
		fmt.Fprintln(trace.out, "name,tic,events")
	}
	return trace
}
func (s *sectorSoundTrace) frame(g *game, name string, tic int) {
	if s.out == nil {
		return
	}
	fmt.Fprintf(s.out, "%s,%d,", name, tic)
	for i, ev := range g.soundQueue {
		if i >= len(g.soundQueueOrigin) {
			continue
		}
		origin := g.soundQueueOrigin[i]
		if origin.sectorPlusOne == 0 {
			continue
		}
		lump, _ := soundEventDSName(ev)
		fmt.Fprintf(s.out, "%s:%d:%d:%d:%d;", lump, origin.sectorPlusOne-1, origin.x, origin.y, soundEventLoopTics(ev))
	}
	fmt.Fprintln(s.out)
	g.soundQueue = g.soundQueue[:0]
	g.soundQueueOrigin = g.soundQueueOrigin[:0]
}
