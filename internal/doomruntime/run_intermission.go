package doomruntime

import (
	"fmt"
	"image/color"
	"strings"

	"gddoom/internal/mapdata"
	"gddoom/internal/runtimehost"
	"gddoom/internal/sessionflow"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	wiTitleY           = 2
	wiSpacingY         = 33
	wiStatsX           = 50
	wiStatsY           = 50
	wiTimeX            = 16
	wiTimeY            = doomLogicalH - 32
	wiShownextDelayTic = 4 * doomTicsPerSecond
	wiSkipInputDelay   = doomTicsPerSecond / 3
)

type intermissionScreenState uint8

const (
	intermissionScreenStats intermissionScreenState = iota
	intermissionScreenShowNextLoc
	intermissionScreenNoState
)

type intermissionAnimKind uint8

const (
	intermissionAnimAlways intermissionAnimKind = iota
	intermissionAnimRandom
	intermissionAnimLevel
)

type intermissionAnimDef struct {
	kind   intermissionAnimKind
	period int
	nanims int
	x      int
	y      int
	data1  int
	data2  int
}

type intermissionAnimState struct {
	def     intermissionAnimDef
	nextTic int
	ctr     int
}

type intermissionStats struct {
	MapName      mapdata.MapName
	NextMapName  mapdata.MapName
	KillsPct     int
	ItemsPct     int
	SecretsPct   int
	TimeSec      int
	ParSec       int
	KillsFound   int
	KillsTotal   int
	ItemsFound   int
	ItemsTotal   int
	SecretsFound int
	SecretsTotal int
}

type intermissionState struct {
	Active      bool
	Screen      intermissionScreenState
	Tic         int
	Bcnt        int
	Cnt         int
	SPState     int
	PauseTics   int
	Accelerate  bool
	Commercial  bool
	Retail      bool
	DidSecret   bool
	PointerOn   bool
	Episode     int
	Last        int
	Next        int
	Show        intermissionStats
	Target      intermissionStats
	Anims       []intermissionAnimState
	StartMusic  bool
	StageQueued bool
}

var (
	intermissionEpisodeNodePositions = [][]interNodePos{
		{{185, 164}, {148, 143}, {69, 122}, {209, 102}, {116, 89}, {166, 55}, {71, 56}, {135, 29}, {71, 24}},
		{{254, 25}, {97, 50}, {188, 64}, {128, 78}, {214, 92}, {133, 130}, {208, 136}, {148, 140}, {235, 158}},
		{{156, 168}, {48, 154}, {174, 95}, {265, 75}, {130, 48}, {279, 23}, {198, 48}, {140, 25}, {281, 136}},
	}
	intermissionEpisodeAnimDefs = [][]intermissionAnimDef{
		{
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 224, y: 104},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 184, y: 160},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 112, y: 136},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 72, y: 112},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 88, y: 96},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 64, y: 48},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 192, y: 40},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 136, y: 16},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 80, y: 16},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 64, y: 24},
		},
		{
			{kind: intermissionAnimLevel, period: doomTicsPerSecond / 3, nanims: 1, x: 128, y: 136, data1: 1},
			{kind: intermissionAnimLevel, period: doomTicsPerSecond / 3, nanims: 1, x: 128, y: 136, data1: 2},
			{kind: intermissionAnimLevel, period: doomTicsPerSecond / 3, nanims: 1, x: 128, y: 136, data1: 3},
			{kind: intermissionAnimLevel, period: doomTicsPerSecond / 3, nanims: 1, x: 128, y: 136, data1: 4},
			{kind: intermissionAnimLevel, period: doomTicsPerSecond / 3, nanims: 1, x: 128, y: 136, data1: 5},
			{kind: intermissionAnimLevel, period: doomTicsPerSecond / 3, nanims: 1, x: 128, y: 136, data1: 6},
			{kind: intermissionAnimLevel, period: doomTicsPerSecond / 3, nanims: 1, x: 128, y: 136, data1: 7},
			{kind: intermissionAnimLevel, period: doomTicsPerSecond / 3, nanims: 3, x: 192, y: 144, data1: 8},
			{kind: intermissionAnimLevel, period: doomTicsPerSecond / 3, nanims: 1, x: 128, y: 136, data1: 8},
		},
		{
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 104, y: 168},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 40, y: 136},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 160, y: 96},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 104, y: 80},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 3, nanims: 3, x: 120, y: 32},
			{kind: intermissionAnimAlways, period: doomTicsPerSecond / 4, nanims: 3, x: 40, y: 0},
		},
	}
)

func (sg *sessionGame) startIntermission(next *mapdata.Map, nextName mapdata.MapName, secretExit bool) {
	sg.freezeDemoRecord()
	sg.stopAndClearMusic()
	if sg.g != nil {
		carry := sg.g.captureLevelCarryover()
		carry.Inventory.PendingWeapon = 0
		carry.Inventory.ReadyWeapon = sg.g.inventory.ReadyWeapon
		sg.levelCarryover = &carry
		sg.g.clearPendingSoundState()
		sg.g.clearSpritePatchCache()
	}
	if secretExit {
		sg.secretVisited = true
	}
	stats := collectIntermissionStats(sg.g, sg.current, nextName)
	stats.ParSec = intermissionParSeconds(stats.MapName)
	lastEp, lastSlot, _ := episodeMapSlot(stats.MapName)
	_, nextSlot, nextOK := episodeMapSlot(stats.NextMapName)
	commercial := strings.HasPrefix(strings.ToUpper(strings.TrimSpace(string(stats.MapName))), "MAP")
	retail := !commercial && lastEp == 4
	state := intermissionState{
		Active:     true,
		Screen:     intermissionScreenStats,
		SPState:    1,
		PauseTics:  doomTicsPerSecond,
		Commercial: commercial,
		Retail:     retail,
		DidSecret:  sg.secretVisited || lastSlot == 9,
		Episode:    lastEp,
		Last:       lastSlot - 1,
		Next:       nextSlot - 1,
		Show: intermissionStats{
			MapName:      stats.MapName,
			NextMapName:  stats.NextMapName,
			KillsFound:   stats.KillsFound,
			KillsTotal:   stats.KillsTotal,
			ItemsFound:   stats.ItemsFound,
			ItemsTotal:   stats.ItemsTotal,
			SecretsFound: stats.SecretsFound,
			SecretsTotal: stats.SecretsTotal,
			KillsPct:     -1,
			ItemsPct:     -1,
			SecretsPct:   -1,
			TimeSec:      -1,
			ParSec:       -1,
		},
		Target:     stats,
		StartMusic: true,
	}
	if !nextOK {
		state.Next = 0
	}
	state.Anims = newIntermissionAnims(lastEp, commercial, retail)
	sg.intermission = sessionIntermission{state: state, nextMap: next}
	sg.maybeStartIntermissionMusic()
}

func (sg *sessionGame) tickIntermission() bool {
	return sg.tickIntermissionAdvance(sg.anyIntermissionSkipInput())
}

func (sg *sessionGame) tickIntermissionAdvance(skipPressed bool) bool {
	if !sg.intermission.state.Active {
		return false
	}
	sg.tickIntermissionSoundSystem()
	state, sounds, finished := tickVanillaIntermission(sg.intermission.state, skipPressed)
	sg.intermission.state = state
	for i := 0; i < len(sounds); i++ {
		sg.playIntermissionSound(sounds[i])
	}
	sg.maybeStartIntermissionMusic()
	return finished
}

func tickVanillaIntermission(state intermissionState, skipPressed bool) (intermissionState, []soundEvent, bool) {
	if !state.Active {
		return state, nil, false
	}
	state.Tic++
	state.Bcnt++
	if skipPressed && state.Tic > wiSkipInputDelay {
		state.Accelerate = true
	}
	state = updateIntermissionAnimatedBack(state)
	var sounds []soundEvent
	switch state.Screen {
	case intermissionScreenStats:
		state, sounds = updateIntermissionStats(state)
	case intermissionScreenShowNextLoc:
		state.PointerOn = (state.Cnt & 31) < 20
		state.Cnt--
		if state.Cnt <= 0 || state.Accelerate {
			state = initIntermissionNoState(state)
		}
	case intermissionScreenNoState:
		state.Cnt--
		if state.Cnt <= 0 {
			return intermissionState{}, nil, true
		}
	}
	return state, sounds, false
}

func updateIntermissionStats(state intermissionState) (intermissionState, []soundEvent) {
	var sounds []soundEvent
	if state.Accelerate && state.SPState != 10 {
		state.Accelerate = false
		state.Show.KillsPct = state.Target.KillsPct
		state.Show.ItemsPct = state.Target.ItemsPct
		state.Show.SecretsPct = state.Target.SecretsPct
		state.Show.TimeSec = state.Target.TimeSec
		state.Show.ParSec = state.Target.ParSec
		state.SPState = 10
		sounds = append(sounds, soundEventIntermissionDone)
	}
	switch state.SPState {
	case 2:
		if state.Show.KillsPct < state.Target.KillsPct {
			state.Show.KillsPct += 2
			if (state.Bcnt & 3) == 0 {
				sounds = append(sounds, soundEventShootPistol)
			}
			if state.Show.KillsPct >= state.Target.KillsPct {
				state.Show.KillsPct = state.Target.KillsPct
				sounds = append(sounds, soundEventIntermissionDone)
				state.SPState++
			}
		}
	case 4:
		if state.Show.ItemsPct < state.Target.ItemsPct {
			state.Show.ItemsPct += 2
			if (state.Bcnt & 3) == 0 {
				sounds = append(sounds, soundEventShootPistol)
			}
			if state.Show.ItemsPct >= state.Target.ItemsPct {
				state.Show.ItemsPct = state.Target.ItemsPct
				sounds = append(sounds, soundEventIntermissionDone)
				state.SPState++
			}
		}
	case 6:
		if state.Show.SecretsPct < state.Target.SecretsPct {
			state.Show.SecretsPct += 2
			if (state.Bcnt & 3) == 0 {
				sounds = append(sounds, soundEventShootPistol)
			}
			if state.Show.SecretsPct >= state.Target.SecretsPct {
				state.Show.SecretsPct = state.Target.SecretsPct
				sounds = append(sounds, soundEventIntermissionDone)
				state.SPState++
			}
		}
	case 8:
		if (state.Bcnt & 3) == 0 {
			sounds = append(sounds, soundEventShootPistol)
		}
		if state.Show.TimeSec < state.Target.TimeSec {
			state.Show.TimeSec += 3
			if state.Show.TimeSec > state.Target.TimeSec {
				state.Show.TimeSec = state.Target.TimeSec
			}
		}
		if state.Show.ParSec < state.Target.ParSec {
			state.Show.ParSec += 3
			if state.Show.ParSec > state.Target.ParSec {
				state.Show.ParSec = state.Target.ParSec
			}
		}
		if state.Show.TimeSec >= state.Target.TimeSec && state.Show.ParSec >= state.Target.ParSec {
			sounds = append(sounds, soundEventIntermissionDone)
			state.SPState++
		}
	case 10:
		if state.Accelerate {
			state.Accelerate = false
			sounds = append(sounds, soundEventShotgunClose)
			if state.Commercial {
				state = initIntermissionNoState(state)
			} else {
				state = initIntermissionShowNextLoc(state)
			}
		}
	default:
		if (state.SPState & 1) != 0 {
			state.PauseTics--
			if state.PauseTics <= 0 {
				state.SPState++
				state.PauseTics = doomTicsPerSecond
			}
		}
	}
	return state, sounds
}

func initIntermissionShowNextLoc(state intermissionState) intermissionState {
	state.Screen = intermissionScreenShowNextLoc
	state.Accelerate = false
	state.PointerOn = false
	state.Cnt = wiShownextDelayTic
	state.Anims = newIntermissionAnims(state.Episode, state.Commercial, state.Retail)
	return state
}

func initIntermissionNoState(state intermissionState) intermissionState {
	state.Screen = intermissionScreenNoState
	state.Accelerate = false
	state.PointerOn = true
	state.Cnt = 10
	return state
}

func newIntermissionAnims(ep int, commercial bool, retail bool) []intermissionAnimState {
	if commercial || retail || ep < 1 || ep > len(intermissionEpisodeAnimDefs) {
		return nil
	}
	defs := intermissionEpisodeAnimDefs[ep-1]
	out := make([]intermissionAnimState, len(defs))
	for i, def := range defs {
		next := 1
		if def.kind == intermissionAnimAlways {
			next += def.period / 2
		}
		out[i] = intermissionAnimState{
			def:     def,
			nextTic: next,
			ctr:     -1,
		}
	}
	return out
}

func updateIntermissionAnimatedBack(state intermissionState) intermissionState {
	if len(state.Anims) == 0 {
		return state
	}
	for i := range state.Anims {
		a := &state.Anims[i]
		if state.Bcnt != a.nextTic {
			continue
		}
		switch a.def.kind {
		case intermissionAnimAlways:
			a.ctr++
			if a.ctr >= a.def.nanims {
				a.ctr = 0
			}
			a.nextTic = state.Bcnt + a.def.period
		case intermissionAnimLevel:
			// Match Doom's hardcoded Episode 2 map animation gating.
			if !(state.Screen == intermissionScreenStats && i == 7) && state.Next+1 == a.def.data1 {
				a.ctr++
				if a.ctr >= a.def.nanims {
					a.ctr = a.def.nanims - 1
				}
			}
			a.nextTic = state.Bcnt + a.def.period
		case intermissionAnimRandom:
			a.ctr++
			if a.ctr >= a.def.nanims {
				a.ctr = -1
				a.nextTic = state.Bcnt + a.def.data2 + max(a.def.data1, 1)
			} else {
				a.nextTic = state.Bcnt + a.def.period
			}
		}
	}
	return state
}

func (sg *sessionGame) maybeStartIntermissionMusic() {
	if sg == nil || sg.musicCtl == nil || !sg.intermission.state.StartMusic {
		return
	}
	sg.intermission.state.StartMusic = false
	sg.playIntermissionMusic(sg.intermission.state.Commercial)
	if sg.intermission.state.Commercial {
		sg.currentMusicSource.musicName = "Doom II Intermission"
		sg.setNowPlayingMusic("Doom II Intermission")
	} else {
		sg.currentMusicSource.musicName = "Intermission from Doom"
		sg.setNowPlayingMusic("Intermission from Doom")
	}
}

func (sg *sessionGame) startEpisodeFinale(current mapdata.MapName, secret bool) bool {
	state, ok := sessionflow.StartFinale(current, secret)
	if !ok {
		return false
	}
	sg.stopAndClearMusic()
	sg.levelCarryover = nil
	if sg.g != nil {
		sg.g.clearPendingSoundState()
	}
	sg.finale = state
	return true
}

func (sg *sessionGame) tickFinale() bool {
	return sg.tickFinaleAdvance(sg.anyIntermissionSkipInput())
}

func (sg *sessionGame) tickFinaleAdvance(skipPressed bool) bool {
	if !sg.finale.Active {
		return false
	}
	state, done := sessionflow.TickFinale(sg.finale, skipPressed)
	sg.finale = state
	return done
}

func (sg *sessionGame) playIntermissionSound(ev soundEvent) {
	if sg == nil || sg.g == nil || sg.g.snd == nil {
		return
	}
	sg.g.snd.playEvent(ev)
}

func (sg *sessionGame) tickIntermissionSoundSystem() {
	if sg == nil || sg.g == nil || sg.g.snd == nil {
		return
	}
	sg.g.snd.tick()
}

func (sg *sessionGame) finishIntermission() {
	im := &sg.intermission
	if im.nextMap == nil {
		return
	}
	if sg.g != nil {
		sg.g.clearPendingSoundState()
	}
	sg.current = im.nextMap.Name
	sg.currentTemplate = cloneMapForRestart(im.nextMap)
	sg.rebuildGameWithPersistentSettings(im.nextMap)
	if sg.levelCarryover != nil && sg.g != nil {
		sg.g.applyLevelCarryover(*sg.levelCarryover)
		sg.levelCarryover = nil
	}
	sg.queueTransition(transitionLevel, 0)
	sg.playMusicForMap(im.nextMap.Name)
	sg.announceMapMusic(im.nextMap.Name)
	ebiten.SetWindowTitle(runtimehost.WindowTitle(im.nextMap.Name))
	sg.intermission = sessionIntermission{}
}

func (sg *sessionGame) drawIntermission(screen *ebiten.Image) {
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	scale := float64(sw) / 320.0
	scaleY := float64(sh) / 200.0
	if scaleY < scale {
		scale = scaleY
	}
	if scale < 1 {
		scale = 1
	}
	ox := (float64(sw) - 320.0*scale) * 0.5
	oy := (float64(sh) - 200.0*scale) * 0.5
	im := &sg.intermission

	switch im.state.Screen {
	case intermissionScreenShowNextLoc, intermissionScreenNoState:
		sg.drawIntermissionMapScreen(screen, scale, ox, oy, im)
	default:
		sg.drawIntermissionStatsScreen(screen, scale, ox, oy, im)
	}
}

func (sg *sessionGame) drawIntermissionPresented(screen *ebiten.Image) {
	if screen == nil {
		return
	}
	dw := max(screen.Bounds().Dx(), 1)
	dh := max(screen.Bounds().Dy(), 1)
	present := sg.ensureFrontendSurface(dw, dh)
	present.Clear()
	sg.drawIntermission(present)
	screen.Fill(color.Black)
	screen.DrawImage(present, nil)
	sg.transition.SetLastFrame(present)
}

func (sg *sessionGame) drawIntermissionStatsScreen(screen *ebiten.Image, scale, ox, oy float64, im *sessionIntermission) {
	screen.Fill(color.Black)
	sg.drawIntermissionBackdrop(screen, scale, ox, oy, im.state)
	sg.drawIntermissionAnimatedBack(screen, scale, ox, oy, im.state)
	sg.drawIntermissionFinished(screen, scale, ox, oy, im.state)
	lh := (3 * sg.intermissionPatchHeight("WINUM0")) / 2
	if lh <= 0 {
		lh = 16
	}
	_ = sg.drawIntermissionPatch(screen, "WIOSTK", wiStatsX, wiStatsY, scale, ox, oy, false)
	sg.drawIntermissionPercent(screen, doomLogicalW-wiStatsX, wiStatsY, im.state.Show.KillsPct, scale, ox, oy)
	_ = sg.drawIntermissionPatch(screen, "WIOSTI", wiStatsX, wiStatsY+lh, scale, ox, oy, false)
	sg.drawIntermissionPercent(screen, doomLogicalW-wiStatsX, wiStatsY+lh, im.state.Show.ItemsPct, scale, ox, oy)
	_ = sg.drawIntermissionPatch(screen, "WISCRT2", wiStatsX, wiStatsY+2*lh, scale, ox, oy, false)
	sg.drawIntermissionPercent(screen, doomLogicalW-wiStatsX, wiStatsY+2*lh, im.state.Show.SecretsPct, scale, ox, oy)
	_ = sg.drawIntermissionPatch(screen, "WITIME", wiTimeX, wiTimeY, scale, ox, oy, false)
	sg.drawIntermissionTime(screen, doomLogicalW/2-wiTimeX, wiTimeY, im.state.Show.TimeSec, scale, ox, oy)
	if !im.state.Commercial && im.state.Episode >= 1 && im.state.Episode <= 3 {
		_ = sg.drawIntermissionPatch(screen, "WIPAR", doomLogicalW/2+wiTimeX, wiTimeY, scale, ox, oy, false)
		sg.drawIntermissionTime(screen, doomLogicalW-wiTimeX, wiTimeY, im.state.Show.ParSec, scale, ox, oy)
	}
}

func (sg *sessionGame) drawIntermissionMapScreen(screen *ebiten.Image, scale, ox, oy float64, im *sessionIntermission) {
	screen.Fill(color.Black)
	sg.drawIntermissionBackdrop(screen, scale, ox, oy, im.state)
	sg.drawIntermissionAnimatedBack(screen, scale, ox, oy, im.state)
	if !im.state.Commercial {
		if im.state.Episode > 3 {
			sg.drawIntermissionEntering(screen, scale, ox, oy, im.state)
			return
		}
		last := im.state.Last
		if last == 8 {
			last = im.state.Next - 1
		}
		nodes := intermissionEpisodeNodePos(im.state.Episode)
		for i := 0; i <= last && i < len(nodes); i++ {
			sg.drawIntermissionNodeSplat(screen, scale, ox, oy, nodes, i+1)
		}
		if im.state.DidSecret && len(nodes) >= 9 {
			sg.drawIntermissionNodeSplat(screen, scale, ox, oy, nodes, 9)
		}
		if im.state.PointerOn && im.state.Next >= 0 && im.state.Next < len(nodes) {
			pt := nodes[im.state.Next]
			if !sg.drawIntermissionPointer(screen, pt.x, pt.y, scale, ox, oy) {
				sg.drawIntermissionText(screen, ">", pt.x, pt.y, scale, ox, oy, true)
			}
		}
	}
	if !im.state.Commercial || im.state.Next != 30 {
		sg.drawIntermissionEntering(screen, scale, ox, oy, im.state)
	}
}

func (sg *sessionGame) drawFinale(screen *ebiten.Image) {
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	scale := float64(sw) / 320.0
	scaleY := float64(sh) / 200.0
	if scaleY < scale {
		scale = scaleY
	}
	if scale < 1 {
		scale = 1
	}
	ox := (float64(sw) - 320.0*scale) * 0.5
	oy := (float64(sh) - 200.0*scale) * 0.5
	f := &sg.finale

	screen.Fill(color.Black)
	if strings.TrimSpace(f.Screen) != "" {
		_ = sg.drawIntermissionPatch(screen, f.Screen, 0, 0, scale, ox, oy, false)
	}
	sg.drawIntermissionText(screen, fmt.Sprintf("EPISODE COMPLETE: %s", f.MapName), 160, 186, scale, ox, oy, true)
	if (f.Tic/16)&1 == 0 {
		sg.drawIntermissionText(screen, "PRESS ANY KEY OR CLICK TO CONTINUE", 160, 174, scale, ox, oy, true)
	}
}

func (sg *sessionGame) drawFinalePresented(screen *ebiten.Image) {
	if screen == nil {
		return
	}
	dw := max(screen.Bounds().Dx(), 1)
	dh := max(screen.Bounds().Dy(), 1)
	present := sg.ensureFrontendSurface(dw, dh)
	present.Clear()
	sg.drawFinale(present)
	screen.Fill(color.Black)
	screen.DrawImage(present, nil)
	sg.transition.SetLastFrame(present)
}

func (sg *sessionGame) drawIntermissionBackdrop(screen *ebiten.Image, scale, ox, oy float64, state intermissionState) {
	if bg, ok := intermissionBackgroundName(state); ok {
		_ = sg.drawIntermissionPatch(screen, bg, 0, 0, scale, ox, oy, false)
		return
	}
	_ = sg.drawIntermissionPatch(screen, "INTERPIC", 0, 0, scale, ox, oy, false)
}

func intermissionBackgroundName(state intermissionState) (string, bool) {
	if state.Commercial || state.Retail {
		return "INTERPIC", true
	}
	switch state.Episode {
	case 1:
		return "WIMAP0", true
	case 2:
		return "WIMAP1", true
	case 3:
		return "WIMAP2", true
	default:
		return "", false
	}
}

func (sg *sessionGame) drawIntermissionAnimatedBack(screen *ebiten.Image, scale, ox, oy float64, state intermissionState) {
	for i, a := range state.Anims {
		if a.ctr < 0 {
			continue
		}
		if !sg.drawIntermissionPatch(screen, intermissionAnimPatchName(state.Episode, i, a.ctr), a.def.x, a.def.y, scale, ox, oy, false) && state.Episode == 2 && i == 8 {
			_ = sg.drawIntermissionPatch(screen, intermissionAnimPatchName(state.Episode, 4, a.ctr), a.def.x, a.def.y, scale, ox, oy, false)
		}
	}
}

func intermissionAnimPatchName(ep int, index int, frame int) string {
	return fmt.Sprintf("WIA%d%02d%02d", ep-1, index, frame)
}

func (sg *sessionGame) drawIntermissionFinished(screen *ebiten.Image, scale, ox, oy float64, state intermissionState) {
	y := wiTitleY
	if name := intermissionLevelPatchName(state.Target.MapName); name != "" {
		sg.drawHorizCenteredPatch(screen, name, doomLogicalW/2, y, scale, ox, oy)
		h := sg.intermissionPatchHeight(name)
		if h > 0 {
			y += (5 * h) / 4
		}
	}
	sg.drawHorizCenteredPatch(screen, "WIF", doomLogicalW/2, y, scale, ox, oy)
}

func (sg *sessionGame) drawIntermissionEntering(screen *ebiten.Image, scale, ox, oy float64, state intermissionState) {
	y := wiTitleY
	sg.drawHorizCenteredPatch(screen, "WIENTER", doomLogicalW/2, y, scale, ox, oy)
	if name := intermissionLevelPatchName(state.Target.NextMapName); name != "" {
		h := sg.intermissionPatchHeight(name)
		if h > 0 {
			y += (5 * h) / 4
		}
		sg.drawHorizCenteredPatch(screen, name, doomLogicalW/2, y, scale, ox, oy)
	}
}

func intermissionLevelPatchName(name mapdata.MapName) string {
	s := strings.ToUpper(strings.TrimSpace(string(name)))
	if len(s) == 4 && s[0] == 'E' && s[2] == 'M' && s[1] >= '1' && s[1] <= '9' && s[3] >= '1' && s[3] <= '9' {
		return fmt.Sprintf("WILV%d%d", s[1]-'1', s[3]-'1')
	}
	if strings.HasPrefix(s, "MAP") && len(s) == 5 && s[3] >= '0' && s[3] <= '9' && s[4] >= '0' && s[4] <= '9' {
		n := int(s[3]-'0')*10 + int(s[4]-'0')
		if n >= 1 && n <= 32 {
			return fmt.Sprintf("CWILV%02d", n-1)
		}
	}
	return ""
}

func (sg *sessionGame) drawIntermissionPointer(screen *ebiten.Image, x, y int, scale, ox, oy float64) bool {
	if sg.drawIntermissionPatch(screen, "WIURH0", x, y, scale, ox, oy, true) {
		return true
	}
	return sg.drawIntermissionPatch(screen, "WIURH1", x, y, scale, ox, oy, true)
}

func (sg *sessionGame) drawIntermissionNodeSplat(screen *ebiten.Image, scale, ox, oy float64, nodes []interNodePos, slot int) {
	if slot < 1 || slot > len(nodes) {
		return
	}
	pt := nodes[slot-1]
	if !sg.drawIntermissionPatch(screen, "WISPLAT", pt.x, pt.y, scale, ox, oy, true) {
		sg.drawIntermissionText(screen, "X", pt.x, pt.y, scale, ox, oy, true)
	}
}

type interNodePos struct {
	x int
	y int
}

func intermissionEpisodeNodePos(ep int) []interNodePos {
	if ep < 1 || ep > len(intermissionEpisodeNodePositions) {
		return nil
	}
	return intermissionEpisodeNodePositions[ep-1]
}

func (sg *sessionGame) drawCenteredPatch(screen *ebiten.Image, name string, x, y int, scale, ox, oy float64) bool {
	return sg.drawIntermissionPatch(screen, name, x, y, scale, ox, oy, true)
}

func (sg *sessionGame) drawHorizCenteredPatch(screen *ebiten.Image, name string, x, y int, scale, ox, oy float64) bool {
	img, p, ok := sg.intermissionPatch(name)
	if !ok || img == nil || p.Width <= 0 || p.Height <= 0 {
		return false
	}
	px := ox + float64(x)*scale - float64(p.Width)*scale*0.5 - float64(p.OffsetX)*scale
	py := oy + float64(y)*scale - float64(p.OffsetY)*scale
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(px, py)
	screen.DrawImage(img, op)
	return true
}

func (sg *sessionGame) drawIntermissionPatch(screen *ebiten.Image, name string, x, y int, scale, ox, oy float64, centered bool) bool {
	img, p, ok := sg.intermissionPatch(name)
	if !ok || img == nil || p.Width <= 0 || p.Height <= 0 {
		return false
	}
	px := ox + float64(x)*scale
	py := oy + float64(y)*scale
	if centered {
		px -= float64(p.Width) * scale * 0.5
		py -= float64(p.Height) * scale * 0.5
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(px-float64(p.OffsetX)*scale, py-float64(p.OffsetY)*scale)
	screen.DrawImage(img, op)
	return true
}

func (sg *sessionGame) intermissionPatch(name string) (*ebiten.Image, WallTexture, bool) {
	if sg == nil {
		return nil, WallTexture{}, false
	}
	key := strings.ToUpper(strings.TrimSpace(name))
	p, ok := sg.opts.IntermissionPatchBank[key]
	if !ok {
		return nil, WallTexture{}, false
	}
	img, ok := sg.cachedPatchImage(&sg.intermissionImages, key, p)
	if !ok {
		return nil, WallTexture{}, false
	}
	return img, p, true
}

func (sg *sessionGame) intermissionPatchHeight(name string) int {
	_, p, ok := sg.intermissionPatch(name)
	if !ok {
		return 0
	}
	return p.Height
}

func (sg *sessionGame) intermissionNumWidth() int {
	_, p, ok := sg.intermissionPatch("WINUM0")
	if !ok || p.Width <= 0 {
		return 8
	}
	return p.Width
}

func (sg *sessionGame) intermissionPatchWidth(name string) int {
	_, p, ok := sg.intermissionPatch(name)
	if !ok {
		return 0
	}
	return p.Width
}

func (sg *sessionGame) drawIntermissionNum(screen *ebiten.Image, x, y, n, digits int, scale, ox, oy float64) int {
	fontWidth := sg.intermissionNumWidth()
	if digits < 0 {
		if n == 0 {
			digits = 1
		} else {
			tmp := n
			if tmp < 0 {
				tmp = -tmp
			}
			digits = 0
			for tmp > 0 {
				tmp /= 10
				digits++
			}
		}
	}
	neg := n < 0
	if neg {
		n = -n
	}
	if n == 1994 {
		return 0
	}
	for digits > 0 {
		x -= fontWidth
		sg.drawIntermissionPatch(screen, fmt.Sprintf("WINUM%d", n%10), x, y, scale, ox, oy, false)
		n /= 10
		digits--
	}
	if neg {
		x -= 8
		sg.drawIntermissionPatch(screen, "WIMINUS", x, y, scale, ox, oy, false)
	}
	return x
}

func (sg *sessionGame) drawIntermissionPercent(screen *ebiten.Image, x, y, p int, scale, ox, oy float64) {
	if p < 0 {
		return
	}
	sg.drawIntermissionPatch(screen, "WIPCNT", x, y, scale, ox, oy, false)
	sg.drawIntermissionNum(screen, x, y, p, -1, scale, ox, oy)
}

func (sg *sessionGame) drawIntermissionTime(screen *ebiten.Image, x, y, t int, scale, ox, oy float64) {
	if t < 0 {
		return
	}
	if t <= 61*59 {
		div := 1
		colonW := sg.intermissionPatchWidth("WICOLON")
		for {
			n := (t / div) % 60
			x = sg.drawIntermissionNum(screen, x, y, n, 2, scale, ox, oy) - colonW
			div *= 60
			if div == 60 || t/div > 0 {
				sg.drawIntermissionPatch(screen, "WICOLON", x, y, scale, ox, oy, false)
			}
			if t/div == 0 {
				break
			}
		}
		return
	}
	sg.drawIntermissionPatch(screen, "WISUCKS", x-sg.intermissionPatchWidth("WISUCKS"), y, scale, ox, oy, false)
}

func (sg *sessionGame) drawIntermissionText(screen *ebiten.Image, text string, x, y int, scale, ox, oy float64, centered bool) {
	px := ox + float64(x)*scale
	py := oy + float64(y)*scale
	if centered {
		px -= float64(sg.intermissionTextWidth(text)) * scale * 0.5
	}
	if sg == nil || sg.g == nil || len(sg.g.opts.MessageFontBank) == 0 {
		ebitenutil.DebugPrintAt(screen, text, int(px), int(py))
		return
	}
	for _, ch := range text {
		uc := ch
		if uc >= 'a' && uc <= 'z' {
			uc -= 'a' - 'A'
		}
		if uc == ' ' || uc < huFontStart || uc > huFontEnd {
			px += 4 * scale
			continue
		}
		img, w, _, gx, gy, ok := sg.g.messageFontGlyph(uc)
		if !ok {
			px += 4 * scale
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(px-float64(gx)*scale, py-float64(gy)*scale)
		screen.DrawImage(img, op)
		px += float64(w) * scale
	}
}

func (sg *sessionGame) intermissionTextWidth(text string) int {
	if sg == nil || sg.g == nil || len(sg.g.opts.MessageFontBank) == 0 {
		return len(text) * 7
	}
	w := 0
	for _, ch := range text {
		uc := ch
		if uc >= 'a' && uc <= 'z' {
			uc -= 'a' - 'A'
		}
		if uc == ' ' || uc < huFontStart || uc > huFontEnd {
			w += 4
			continue
		}
		_, gw, _, _, _, ok := sg.g.messageFontGlyph(uc)
		if !ok {
			w += 4
			continue
		}
		w += gw
	}
	return w
}

func (sg *sessionGame) intermissionTextLineHeight() int {
	if sg == nil || sg.g == nil || len(sg.g.opts.MessageFontBank) == 0 {
		return 8
	}
	lineHeight := 0
	for ch := huFontStart; ch <= huFontEnd; ch++ {
		_, _, gh, _, _, ok := sg.g.messageFontGlyph(ch)
		if ok && gh > lineHeight {
			lineHeight = gh
		}
	}
	if lineHeight <= 0 {
		return 8
	}
	return lineHeight
}

func episodeMapSlot(name mapdata.MapName) (episode int, slot int, ok bool) {
	s := string(name)
	if len(s) != 4 || s[0] != 'E' || s[2] != 'M' {
		return 0, 0, false
	}
	e := int(s[1] - '0')
	m := int(s[3] - '0')
	if e < 1 || e > 9 || m < 1 || m > 9 {
		return 0, 0, false
	}
	return e, m, true
}

func collectIntermissionStats(g *game, mapName, nextName mapdata.MapName) intermissionStats {
	out := intermissionStats{
		MapName:     mapName,
		NextMapName: nextName,
	}
	if g == nil || g.m == nil {
		return out
	}
	for i, th := range g.m.Things {
		if !thingSpawnsInSession(th, g.opts.SkillLevel, g.opts.GameMode, g.opts.ShowNoSkillItems, g.opts.ShowAllItems, g.opts.NoMonsters) {
			continue
		}
		if isMonster(th.Type) {
			out.KillsTotal++
			if i >= 0 && i < len(g.thingHP) && g.thingHP[i] <= 0 {
				out.KillsFound++
			}
			continue
		}
		if isPickupType(th.Type) {
			out.ItemsTotal++
			if i >= 0 && i < len(g.thingCollected) && g.thingCollected[i] {
				out.ItemsFound++
			}
		}
	}
	out.SecretsTotal = g.secretsTotal
	if out.SecretsTotal <= 0 {
		out.SecretsTotal = 1
	}
	out.SecretsFound = g.secretsFound
	if out.SecretsFound > out.SecretsTotal {
		out.SecretsFound = out.SecretsTotal
	}
	out.KillsPct = intermissionPercent(out.KillsFound, max(out.KillsTotal, 1))
	out.ItemsPct = intermissionPercent(out.ItemsFound, max(out.ItemsTotal, 1))
	out.SecretsPct = intermissionPercent(out.SecretsFound, out.SecretsTotal)
	out.TimeSec = g.worldTic / doomTicsPerSecond
	return out
}

func intermissionPercent(n, d int) int {
	if d <= 0 || n <= 0 {
		return 0
	}
	if n >= d {
		return 100
	}
	return (n * 100) / d
}

func intermissionParSeconds(name mapdata.MapName) int {
	s := strings.ToUpper(strings.TrimSpace(string(name)))
	if len(s) == 4 && s[0] == 'E' && s[2] == 'M' && s[1] >= '1' && s[1] <= '4' && s[3] >= '1' && s[3] <= '9' {
		pars := [5][10]int{
			{},
			{0, 30, 75, 120, 90, 165, 180, 180, 30, 165},
			{0, 90, 90, 90, 120, 90, 360, 240, 30, 170},
			{0, 90, 45, 90, 150, 90, 90, 165, 30, 135},
			{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		}
		return pars[s[1]-'0'][s[3]-'0']
	}
	if strings.HasPrefix(s, "MAP") && len(s) == 5 && s[3] >= '0' && s[3] <= '9' && s[4] >= '0' && s[4] <= '9' {
		idx := int(s[3]-'0')*10 + int(s[4]-'0')
		if idx >= 1 && idx <= 32 {
			cpars := [32]int{
				30, 90, 120, 120, 90, 150, 120, 120, 270, 90,
				210, 150, 150, 150, 210, 150, 420, 150, 210, 150,
				240, 150, 180, 150, 150, 300, 330, 420, 300, 180,
				120, 30,
			}
			return cpars[idx-1]
		}
	}
	return 0
}

func episodeFinaleScreen(current mapdata.MapName, secret bool) (string, bool) {
	return sessionflow.EpisodeFinaleScreen(current, secret)
}

func (sg *sessionGame) anyIntermissionSkipInput() bool {
	if sg == nil {
		return false
	}
	return sg.skipInputTriggered()
}
