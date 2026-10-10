package doomruntime

import (
	"math"
	"reflect"
	"testing"
	"time"

	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
)

func TestAuthorityLiftRiderCameraSharesConfirmedFloorTimeline(t *testing.T) {
	for _, direction := range []int{-1, 1} {
		t.Run(map[int]string{-1: "descending", 1: "ascending"}[direction], func(t *testing.T) {
			a, p := predictionTestWorld(t)
			g := p.g
			g.clientPrediction = p
			g.opts.SourcePortMode = true
			// Use a genuine authoritative lift thinker, including player clipping,
			// while the client predicts three tics ahead on each confirmed floor.
			status := platStatusUp
			if direction < 0 {
				status = platStatusDown
			}
			a.g.plats = map[int]*platThinker{0: {order: 2, sector: 0, status: status, speed: fracUnit, low: -6 * fracUnit, high: 6 * fracUnit, wait: 3, typ: platTypePerpetualRaise}}
			for tic := uint32(1); tic <= 3; tic++ {
				if err := p.Predict(netgame.Input{Sequence: tic, Tick: tic}); err != nil {
					t.Fatal(err)
				}
			}
			start := time.Unix(900, 0)
			period := time.Second / doomTicsPerSecond
			id := uint32(1)
			lastFloor := int64(0)
			var rose, fell, waited bool
			for tic := uint32(1); tic <= 24; tic++ {
				if err := a.Step(nil); err != nil {
					t.Fatal(err)
				}
				delta := a.g.sectorFloor[0] - lastFloor
				rose, fell, waited = rose || delta > 0, fell || delta < 0, waited || delta == 0
				lastFloor = a.g.sectorFloor[0]
				if err := p.Predict(netgame.Input{Sequence: tic + 3, Tick: tic + 3}); err != nil {
					t.Fatal(err)
				}
				now := start.Add(time.Duration(tic) * period)
				// Include a missed snapshot interval without permitting prediction
				// to move the world while the client waits for confirmed endpoints.
				if tic%2 == 0 && tic != 10 {
					id++
					if _, err := p.reconcileAt(predictionSnapshot(t, a, id, netgame.InputAck{HasTick: true, Tick: tic}), now); err != nil {
						t.Fatal(err)
					}
				}
				body, view, floor := g.p, g.playerViewZ, g.sectorFloor[0]
				for _, fraction := range []time.Duration{0, period / 3, 2 * period / 3} {
					g.prepareRenderStateAt(now.Add(fraction))
					renderFloor, _, ok := g.authoritySectorRenderHeights(0)
					if !ok {
						continue
					}
					if height := g.playerEyeZ() - float64(renderFloor)/fracUnit; math.Abs(height-41) > 1e-9 {
						t.Fatalf("tic %d: rider height over visible floor = %v, want 41 (correction %v)", tic, height, p.renderCorrection.z)
					}
					if g.p != body || g.playerViewZ != view || g.sectorFloor[0] != floor {
						t.Fatal("rendering changed rider collision or authoritative floor")
					}
				}
				if p.renderCorrection.z != 0 {
					t.Fatalf("lift movement entered independent camera correction: %v", p.renderCorrection.z)
				}
			}
			if !rose || !fell || !waited {
				t.Fatalf("fixture did not exercise rise, fall and wait: %v/%v/%v", rose, fell, waited)
			}
		})
	}
}

func supportTransitionTestWorld(t *testing.T, entering bool) (*Authority, *ClientPrediction, time.Time) {
	t.Helper()
	m := predictionTestMap()
	m.Sectors[0].FloorHeight = 8
	m.Sectors = append(m.Sectors, m.Sectors[0])
	m.Sidedefs = append(m.Sidedefs, mapdata.Sidedef{Sector: 1})
	m.Linedefs[0].SideNum[0] = 1
	m.SubSectors = []mapdata.SubSector{{SegCount: 1, FirstSeg: 2}, {SegCount: 1}}
	m.Nodes = []mapdata.Node{{DY: 128, ChildID: [2]uint16{0x8000, 0x8001}}}
	m.Things[0].X = 2
	if entering {
		m.Things[0].X = -2
	}
	a, err := NewAuthority(m, Options{SkillLevel: 3, NoMonsters: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	g := newGame(cloneMapForRestart(m), Options{Headless: true, SourcePortMode: true, SkillLevel: 3, NoMonsters: true})
	p, err := newClientPrediction(g, netgame.Welcome{Epoch: 7, PlayerID: 1})
	if err != nil {
		t.Fatal(err)
	}
	g.clientPrediction = p
	now := time.Unix(903, 0)
	if _, err := p.reconcileAt(predictionSnapshot(t, a, 1, netgame.InputAck{}), now); err != nil {
		t.Fatal(err)
	}
	from, to := g.snapshotAuthorityRenderFrame(), g.snapshotAuthorityRenderFrame()
	from.tic, to.tic = 0, 2
	from.sectors[0].floor = 0
	g.authorityRender = newAuthorityRenderTimeline(from, to, now)
	// Hold the confirmed floor midway while checking only support transitions.
	g.authorityRender.tic = 1
	g.authorityRender.stamp = now.Add(time.Second)
	g.authorityRender.selectFrames()
	return a, p, now
}

func TestAuthorityPredictedSupportTransitionsPreserveEyeContinuity(t *testing.T) {
	for _, entering := range []bool{false, true} {
		t.Run(map[bool]string{false: "leave-lift", true: "enter-lift"}[entering], func(t *testing.T) {
			a, p, now := supportTransitionTestWorld(t, entering)
			g := p.g
			independent := predictionRenderCorrection{z: 2, started: now}
			p.renderCorrection = independent
			g.prepareRenderStateAt(now)
			before := g.playerEyeZ()
			g.p.momx = -4 * fracUnit
			wantSector := 1
			if entering {
				g.p.momx = 4 * fracUnit
				wantSector = 0
			}
			a.players[1].p.momx = g.p.momx
			if err := p.Predict(netgame.Input{Sequence: 1, Tick: 1}); err != nil {
				t.Fatal(err)
			}
			if g.p.sector != wantSector {
				t.Fatalf("prediction did not cross support boundary: sector %d", g.p.sector)
			}
			pending := p.supportCorrection.pending
			if _, err := p.reconcileAt(predictionSnapshot(t, a, 2, netgame.InputAck{}), now); err != nil {
				t.Fatal(err)
			}
			if p.supportCorrection.pending != pending {
				t.Fatal("replaying the same boundary crossing queued its correction twice")
			}
			body := g.p
			g.prepareRenderStateAt(now)
			if got := g.playerEyeZ(); got != before {
				t.Fatalf("support transition jumped from %v to %v", before, got)
			}
			g.prepareRenderStateAt(now.Add(predictionCorrectionDuration / 2))
			target := g.playerBaseEyeZ() + g.authoritySupportEyeOffset()
			if got := g.playerEyeZ(); math.Abs(got-(before+target)/2) > 1e-9 {
				t.Fatalf("support correction did not decay once: %v", got)
			}
			g.prepareRenderStateAt(now.Add(predictionCorrectionDuration))
			if g.playerEyeZ() != target || g.p != body || p.renderCorrection != independent {
				t.Fatal("support correction altered body/independent correction or did not converge")
			}
		})
	}
}

func TestAuthorityReconciledSupportTransitionsPreserveEyeContinuity(t *testing.T) {
	for _, entering := range []bool{false, true} {
		t.Run(map[bool]string{false: "leave-lift", true: "enter-lift"}[entering], func(t *testing.T) {
			a, p, now := supportTransitionTestWorld(t, entering)
			g := p.g
			g.prepareRenderStateAt(now)
			before := g.playerEyeZ()
			target := a.players[1]
			target.p.x = -target.p.x
			target.p.sector = 1 - target.p.sector
			target.p.subsector = 1 - target.p.subsector
			if _, err := p.reconcileAt(predictionSnapshot(t, a, 2, netgame.InputAck{}), now); err != nil {
				t.Fatal(err)
			}
			g.prepareRenderStateAt(now)
			if got := g.playerEyeZ(); got != before {
				t.Fatalf("snapshot support transition jumped from %v to %v", before, got)
			}
			correction := p.supportCorrection
			if _, err := p.reconcileAt(predictionSnapshot(t, a, 3, netgame.InputAck{}), now); err != nil {
				t.Fatal(err)
			}
			g.prepareRenderStateAt(now)
			if p.supportCorrection != correction {
				t.Fatal("matching follow-up snapshot restarted support correction")
			}
		})
	}
}

func TestAuthorityGroundToAirSupportTransitionStopsFollowingFloor(t *testing.T) {
	_, p, now := supportTransitionTestWorld(t, false)
	g := p.g
	g.prepareRenderStateAt(now)
	before := g.playerEyeZ()
	g.p.momz = fracUnit
	if err := p.Predict(netgame.Input{Sequence: 1, Tick: 1}); err != nil {
		t.Fatal(err)
	}
	if g.p.z <= g.p.floorz || g.authoritySupportEyeOffset() != 0 {
		t.Fatal("airborne body still follows support plane")
	}
	g.prepareRenderStateAt(now)
	if got := g.playerEyeZ(); got != before {
		t.Fatalf("leaving support jumped from %v to %v", before, got)
	}
	g.prepareRenderStateAt(now.Add(predictionCorrectionDuration))
	if g.playerEyeZ() != g.playerBaseEyeZ() {
		t.Fatal("airborne camera retained support offset beyond bounded transition")
	}
}

func TestAuthorityObserverSupportTransitionsAndCuts(t *testing.T) {
	for _, cut := range []string{"enter", "leave", "teleport", "respawn", "death", "viewer"} {
		t.Run(cut, func(t *testing.T) {
			a, p, now := supportTransitionTestWorld(t, cut == "enter")
			g := p.g
			connection := &fakeAuthorityClient{welcome: netgame.Welcome{Epoch: 7, PlayerID: 0}}
			g.opts.AuthorityClient = connection
			g.prepareRenderStateAt(now)
			before := g.playerEyeZ()
			target := a.players[1]
			target.p.x = -target.p.x
			target.p.sector, target.p.subsector = 1-target.p.sector, 1-target.p.subsector
			connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 2, netgame.InputAck{})}
			if err := g.updateAuthoritativeObserverAt(now); err != nil {
				t.Fatal(err)
			}
			g.prepareRenderStateAt(now)
			if got := g.playerEyeZ(); got != before || p.supportCorrection.z == 0 {
				t.Fatalf("observer support change jumped: before %v after %v correction %+v", before, got, p.supportCorrection)
			}
			if cut == "enter" || cut == "leave" {
				g.prepareRenderStateAt(now.Add(predictionCorrectionDuration))
				if g.playerEyeZ() != g.playerBaseEyeZ()+g.authoritySupportEyeOffset() {
					t.Fatal("observer support correction did not converge")
				}
				return
			}
			switch cut {
			case "teleport":
				a.g.authorityRules.Scores[1].MovementEpoch++
			case "respawn":
				a.g.authorityRules.Scores[1].Generation++
			case "death":
				a.players[1].isDead, a.players[1].stats.Health = true, 0
			case "viewer":
				a.players[1].p.x = -64 * fracUnit
				if err := a.AddPlayer(2); err != nil {
					t.Fatal(err)
				}
			}
			snapshot := predictionSnapshot(t, a, 3, netgame.InputAck{})
			if cut == "viewer" {
				var err error
				snapshot.State, err = a.Snapshot(2)
				if err != nil {
					t.Fatal(err)
				}
			}
			connection.snapshots = []netgame.Snapshot{snapshot}
			if err := g.updateAuthoritativeObserverAt(now); err != nil {
				t.Fatal(err)
			}
			g.prepareRenderStateAt(now)
			if p.supportCorrection != (predictionSupportCorrection{}) || p.renderEyeOffset != 0 {
				t.Fatalf("%s retained old support correction", cut)
			}
		})
	}
}

func TestAuthoritySupportEyeOffsetRequiresActualSupport(t *testing.T) {
	_, p := predictionTestWorld(t)
	g := p.g
	g.clientPrediction = p
	g.sectorFloor[0] = 8 * fracUnit
	g.p.floorz, g.p.z = 8*fracUnit, 8*fracUnit
	g.playerViewZ = 49 * fracUnit
	from, to := g.snapshotAuthorityRenderFrame(), g.snapshotAuthorityRenderFrame()
	from.tic, to.tic = 0, 2
	from.sectors[0].floor = 0
	g.authorityRender = newAuthorityRenderTimeline(from, to, time.Unix(901, 0))
	if got := g.authoritySupportEyeOffset(); got != -8 {
		t.Fatalf("support offset %v, want -8", got)
	}
	for _, kind := range []string{"airborne", "neighbor-floor", "dead", "single-player", "first-baseline"} {
		t.Run(kind, func(t *testing.T) {
			body, dead, prediction, timeline := g.p, g.isDead, g.clientPrediction, g.authorityRender
			t.Cleanup(func() { g.p, g.isDead, g.clientPrediction, g.authorityRender = body, dead, prediction, timeline })
			switch kind {
			case "airborne":
				g.p.z += fracUnit
			case "neighbor-floor":
				g.p.floorz, g.p.z = 9*fracUnit, 9*fracUnit
			case "dead":
				g.isDead = true
			case "single-player":
				g.clientPrediction = nil
			case "first-baseline":
				g.authorityRender = nil
			}
			if offset := g.authoritySupportEyeOffset(); offset != 0 {
				t.Fatalf("%s incorrectly follows floor: %v", kind, offset)
			}
		})
	}
}

func TestAuthorityPredictionDoesNotAdvanceWorldMovers(t *testing.T) {
	a, p := predictionTestWorld(t)
	a.g.floors = map[int]*floorThinker{0: {order: 2, sector: 0, direction: 1, speed: fracUnit, destHeight: 64 * fracUnit}}
	a.g.doors[0] = &doorThinker{order: 3, sector: 0, typ: doorOpen, direction: 1, speed: 2 * fracUnit, topHeight: 200 * fracUnit}
	if err := a.Step(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Reconcile(predictionSnapshot(t, a, 2, netgame.InputAck{HasTick: true, Tick: 1})); err != nil {
		t.Fatal(err)
	}
	g := p.g
	floor, ceiling, floorThinker, doorThinker := g.sectorFloor[0], g.sectorCeil[0], *g.floors[0], *g.doors[0]
	for tic := uint32(2); tic <= 20; tic++ {
		if err := p.Predict(netgame.Input{Sequence: tic, Tick: tic}); err != nil {
			t.Fatal(err)
		}
	}
	if g.worldTic != 1 || g.sectorFloor[0] != floor || g.sectorCeil[0] != ceiling || !reflect.DeepEqual(*g.floors[0], floorThinker) || !reflect.DeepEqual(*g.doors[0], doorThinker) {
		t.Fatal("local movement replay advanced authoritative world movers")
	}
}

func TestAuthorityObserverLiftCameraSharesConfirmedFloorTimeline(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 3)
	connection.welcome.PlayerID = 0
	g.opts.SourcePortMode = true
	now := time.Unix(902, 0)
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	if err := g.updateAuthoritativeClientAt(now, nil); err != nil {
		t.Fatal(err)
	}
	// Two confirmed floor/body poses isolate snapshot-only presentation from
	// player prediction; observers never submit movement or replay local tics.
	a.g.sectorFloor[0] = 8 * fracUnit
	a.players[1].p.floorz, a.players[1].p.z = 8*fracUnit, 8*fracUnit
	a.players[1].playerViewZ = 49 * fracUnit
	a.g.worldTic = 2
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 2, netgame.InputAck{})}
	arrival := now.Add(2 * time.Second / doomTicsPerSecond)
	if err := g.updateAuthoritativeClientAt(arrival, nil); err != nil {
		t.Fatal(err)
	}
	body, view := g.p, g.playerViewZ
	g.prepareRenderStateAt(arrival.Add(2 * time.Second / doomTicsPerSecond))
	floor, _, ok := g.authoritySectorRenderHeights(0)
	if !ok || floor <= 0 || floor >= 8*fracUnit {
		t.Fatalf("expected intermediate lift floor, got %v, %v", floor, ok)
	}
	if height := g.playerEyeZ() - float64(floor)/fracUnit; math.Abs(height-41) > 1e-9 {
		t.Fatalf("observer camera did not follow visible lift: relative height %v", height)
	}
	if g.p != body || g.playerViewZ != view || g.worldTic != 2 || len(g.clientPrediction.history) != 0 {
		t.Fatal("observer rendering changed confirmed body/view or predicted input")
	}
}
