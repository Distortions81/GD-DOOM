package doomsession

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"gddoom/internal/mapdata"
	"gddoom/internal/media"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/runtimehost"
	"gddoom/internal/session"
)

type replacementRuntime struct {
	stubRuntime
	bundle     runtimecfg.AuthorityContentBundle
	initialize func(session.Runtime)
	taken      bool
}

func (r *replacementRuntime) TakeAuthorityContentReplacement() (runtimecfg.AuthorityContentBundle, func(session.Runtime), bool) {
	if r.taken {
		return runtimecfg.AuthorityContentBundle{}, nil, false
	}
	r.taken = true
	return r.bundle, r.initialize, true
}

func TestSessionContentReplacementClosesOldOwnerAndInstallsFreshRuntime(t *testing.T) {
	newMap := &mapdata.Map{Name: "MAP02", Things: []mapdata.Thing{{Type: 1}},
		Vertexes: []mapdata.Vertex{{X: -128, Y: -128}, {X: -128, Y: 128}, {X: 128, Y: 128}, {X: 128, Y: -128}},
		Sidedefs: []mapdata.Sidedef{{Sector: 0}}, Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		SubSectors: []mapdata.SubSector{{SegCount: 4}}}
	for i := range 4 {
		newMap.Linedefs = append(newMap.Linedefs, mapdata.Linedef{V1: uint16(i), V2: uint16((i + 1) % 4), Flags: 1, SideNum: [2]int16{0, -1}})
		newMap.Segs = append(newMap.Segs, mapdata.Seg{StartVertex: uint16(i), EndVertex: uint16((i + 1) % 4), Linedef: uint16(i)})
	}
	var oldCloses, newCloses atomic.Int32
	joinedAfterClose := make(chan bool, 1)
	request := runtimecfg.AuthorityJoinRequest{Address: "wss://room.test/game", Name: "Doomer", Spectator: true}
	old := &replacementRuntime{bundle: runtimecfg.AuthorityContentBundle{Map: newMap, Options: Options{
		Headless: true, StartInMapMode: true, NoMonsters: true, SkillLevel: 3,
		AuthorityAutoJoin: true, AuthorityJoinDefaults: request,
		AuthorityWADHashes:      []string{"new-content"},
		MenuPatchBank:           map[string]media.WallTexture{"NEW": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 255}}},
		AuthorityContentCleanup: func() { newCloses.Add(1) },
		AuthorityJoin: func(_ context.Context, got runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
			joinedAfterClose <- oldCloses.Load() == 1 && got == request
			return runtimecfg.AuthorityJoinResult{}, errors.New("test join finished")
		},
	}}}
	sess := &Session{game: old, lastPeriodicKeyframeTic: periodicKeyframeIntervalTics,
		meta: runtimehost.Meta{Close: func() { oldCloses.Add(1) }}}
	initialized := false
	old.initialize = func(fresh session.Runtime) {
		initialized = true
		if fresh == old || sess.game != fresh || oldCloses.Load() != 1 || sess.StartMapName() != "MAP02" {
			t.Fatal("initialization saw an old runtime, metadata, or live old owner")
		}
	}
	if err := sess.Update(); err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	select {
	case ok := <-joinedAfterClose:
		if !ok {
			t.Fatal("new runtime joined before closing old owner or lost the room request")
		}
	case <-time.After(time.Second):
		t.Fatal("replacement did not begin its normal join")
	}
	if !initialized || sess.lastPeriodicKeyframeTic != 0 || newCloses.Load() != 0 || sess.Options().AuthorityWADHashes[0] != "new-content" || sess.Options().MenuPatchBank["NEW"].Width != 1 {
		t.Fatal("full replacement lost metadata/content or released new resources early")
	}
	if err := sess.Update(); err != nil {
		t.Fatal(err)
	}
	sess.Close()
	sess.Close()
	if oldCloses.Load() != 1 || newCloses.Load() != 1 {
		t.Fatalf("runtime resource ownership: old closes=%d new closes=%d", oldCloses.Load(), newCloses.Load())
	}
}
