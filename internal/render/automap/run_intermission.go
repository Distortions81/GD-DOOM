package automap

import (
	"fmt"
	"image/color"
	"strings"

	"gddoom/internal/mapdata"
	"gddoom/internal/runtimehost"
	"gddoom/internal/sessionflow"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func (sg *sessionGame) startIntermission(next *mapdata.Map, nextName mapdata.MapName) {
	sg.stopAndClearMusic()
	if sg.g != nil {
		sg.g.clearPendingSoundState()
		sg.g.clearSpritePatchCache()
	}
	stats := collectIntermissionStats(sg.g, sg.current, nextName)
	state, res := sessionflow.Start(stats)
	sg.intermission = sessionIntermission{state: state, nextMap: next}
	if res.PlayTick {
		sg.playIntermissionSound(soundEventIntermissionTick)
	}
}

func (sg *sessionGame) tickIntermission() bool {
	return sg.tickIntermissionAdvance(anyIntermissionSkipInput())
}

func (sg *sessionGame) tickIntermissionAdvance(skipPressed bool) bool {
	if !sg.intermission.state.Active {
		return false
	}
	sg.tickIntermissionSoundSystem()
	state, res := sessionflow.Tick(sg.intermission.state, skipPressed)
	sg.intermission.state = state
	if res.PlayTick {
		sg.playIntermissionSound(soundEventIntermissionTick)
	}
	if res.PlayDone {
		sg.playIntermissionSound(soundEventIntermissionDone)
	}
	return res.Finished
}

func (sg *sessionGame) startEpisodeFinale(current mapdata.MapName, secret bool) bool {
	state, ok := sessionflow.StartFinale(current, secret)
	if !ok {
		return false
	}
	sg.stopAndClearMusic()
	if sg.g != nil {
		sg.g.clearPendingSoundState()
	}
	sg.finale = state
	return true
}

func (sg *sessionGame) tickFinale() bool {
	return sg.tickFinaleAdvance(anyIntermissionSkipInput())
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
	if !im.state.Active || im.nextMap == nil {
		return
	}
	if sg.g != nil {
		sg.g.clearPendingSoundState()
	}
	sg.current = im.state.Target.NextMapName
	sg.currentTemplate = cloneMapForRestart(im.nextMap)
	sg.rebuildGameWithPersistentSettings(im.nextMap)
	sg.playMusicForMap(im.nextMap.Name)
	ebiten.SetWindowTitle(runtimehost.WindowTitle(im.nextMap.Name))
	sg.intermission = sessionIntermission{}
	sg.queueTransition(transitionLevel, 0)
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

	if im.state.Phase >= sessionflow.PhaseEntering && im.state.ShowEntering {
		sg.drawIntermissionMapScreen(screen, scale, ox, oy, im)
		return
	}

	screen.Fill(color.Black)
	sg.drawIntermissionBackdrop(screen, scale, ox, oy, im.state.Target.MapName)
	sg.drawIntermissionText(screen, fmt.Sprintf("FINISHED %s", im.state.Target.MapName), 160, 24, scale, ox, oy, true)
	sg.drawIntermissionText(screen, fmt.Sprintf("KILLS   %3d%%", im.state.Show.KillsPct), 80, 70, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("ITEMS   %3d%%", im.state.Show.ItemsPct), 80, 90, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("SECRETS %3d%%", im.state.Show.SecretsPct), 80, 110, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("TIME %s", formatIntermissionTime(im.state.Show.TimeSec)), 80, 138, scale, ox, oy, false)
	if (im.state.Tic/16)&1 == 0 {
		sg.drawIntermissionText(screen, "PRESS ANY KEY OR CLICK TO SKIP", 160, 186, scale, ox, oy, true)
	}
}

func (sg *sessionGame) drawIntermissionMapScreen(screen *ebiten.Image, scale, ox, oy float64, im *sessionIntermission) {
	screen.Fill(color.Black)
	sg.drawIntermissionBackdrop(screen, scale, ox, oy, im.state.Target.MapName)
	sg.drawIntermissionText(screen, fmt.Sprintf("ENTERING %s", im.state.Target.NextMapName), 160, 24, scale, ox, oy, true)
	if im.state.Phase == sessionflow.PhaseYouAreHere && im.state.ShowYouAreHere {
		sg.drawYouAreHerePanel(screen, scale, ox, oy, im.state.Target.MapName, im.state.Target.NextMapName)
	} else {
		sg.drawCurrentIntermissionNode(screen, scale, ox, oy, im.state.Target.MapName)
	}
	if (im.state.Tic/16)&1 == 0 {
		sg.drawIntermissionText(screen, "PRESS ANY KEY OR CLICK TO SKIP", 160, 186, scale, ox, oy, true)
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

func (sg *sessionGame) drawIntermissionBackdrop(screen *ebiten.Image, scale, ox, oy float64, current mapdata.MapName) {
	if bg, ok := sg.intermissionBackgroundName(current); ok {
		_ = sg.drawIntermissionPatch(screen, bg, 0, 0, scale, ox, oy, false)
		return
	}
	_ = sg.drawIntermissionPatch(screen, "INTERPIC", 0, 0, scale, ox, oy, false)
}

func (sg *sessionGame) drawYouAreHerePanel(screen *ebiten.Image, scale, ox, oy float64, current, next mapdata.MapName) {
	if !sg.drawIntermissionPatch(screen, "WIURH0", 208, 38, scale, ox, oy, false) {
		sg.drawIntermissionText(screen, "YOU ARE HERE", 240, 46, scale, ox, oy, true)
	}
	epCur, mapCur, okCur := episodeMapSlot(current)
	epNext, mapNext, okNext := episodeMapSlot(next)
	if !okCur || !okNext || epCur != epNext {
		return
	}
	nodes := intermissionEpisodeNodePos(epCur)
	if len(nodes) != 9 {
		return
	}
	sg.drawIntermissionNodeSplat(screen, scale, ox, oy, nodes, mapCur)
	if mapNext >= 1 && mapNext <= 9 && (sg.intermission.state.Tic/8)&1 == 0 {
		pt := nodes[mapNext-1]
		if !sg.drawIntermissionPatch(screen, "WIURH0", pt.x, pt.y, scale, ox, oy, true) {
			sg.drawIntermissionText(screen, ">", pt.x, pt.y, scale, ox, oy, true)
		}
	}
}

func (sg *sessionGame) drawCurrentIntermissionNode(screen *ebiten.Image, scale, ox, oy float64, current mapdata.MapName) {
	ep, slot, ok := episodeMapSlot(current)
	if !ok {
		return
	}
	nodes := intermissionEpisodeNodePos(ep)
	if len(nodes) != 9 {
		return
	}
	sg.drawIntermissionNodeSplat(screen, scale, ox, oy, nodes, slot)
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
	switch ep {
	case 1:
		return []interNodePos{{185, 164}, {148, 143}, {69, 122}, {209, 102}, {116, 89}, {166, 55}, {71, 56}, {135, 29}, {71, 24}}
	case 2:
		return []interNodePos{{254, 25}, {97, 50}, {188, 64}, {128, 78}, {214, 92}, {133, 130}, {208, 136}, {148, 140}, {235, 158}}
	case 3:
		return []interNodePos{{156, 168}, {48, 154}, {174, 95}, {265, 75}, {130, 48}, {279, 23}, {198, 48}, {140, 25}, {281, 136}}
	default:
		return nil
	}
}

func (sg *sessionGame) intermissionBackgroundName(current mapdata.MapName) (string, bool) {
	ep, _, ok := episodeMapSlot(current)
	if !ok {
		return "", false
	}
	switch ep {
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
	if sg == nil || sg.g == nil {
		return nil, WallTexture{}, false
	}
	key := strings.ToUpper(strings.TrimSpace(name))
	p, ok := sg.g.opts.IntermissionPatchBank[key]
	if !ok || p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
		return nil, WallTexture{}, false
	}
	if sg.interPatchCache == nil {
		sg.interPatchCache = make(map[string]*ebiten.Image, 64)
	}
	if img, ok := sg.interPatchCache[key]; ok {
		return img, p, true
	}
	img := ebiten.NewImage(p.Width, p.Height)
	img.WritePixels(p.RGBA)
	sg.interPatchCache[key] = img
	return img, p, true
}

func (sg *sessionGame) drawIntermissionText(screen *ebiten.Image, text string, x, y int, scale, ox, oy float64, centered bool) {
	px := ox + float64(x)*scale
	py := oy + float64(y)*scale
	if centered {
		px -= float64(sg.intermissionTextWidth(text)) * scale * 0.5
	}
	if len(sg.g.opts.MessageFontBank) == 0 {
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
	if sg == nil || sg.g == nil {
		return len(text) * 7
	}
	if len(sg.g.opts.MessageFontBank) == 0 {
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
		if !thingSpawnsInSession(th, g.opts.SkillLevel, g.opts.GameMode, g.opts.ShowNoSkillItems, g.opts.ShowAllItems) {
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
	out.SecretsFound = g.secretsFound
	if out.SecretsFound > out.SecretsTotal {
		out.SecretsFound = out.SecretsTotal
	}
	out.KillsPct = intermissionPercent(out.KillsFound, out.KillsTotal)
	out.ItemsPct = intermissionPercent(out.ItemsFound, out.ItemsTotal)
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

func shouldShowYouAreHere(current, next mapdata.MapName) bool {
	return sessionflow.ShouldShowYouAreHere(current, next)
}

func shouldShowEnteringScreen(current, next mapdata.MapName) bool {
	return sessionflow.ShouldShowEnteringScreen(current, next)
}

func episodeFinaleScreen(current mapdata.MapName, secret bool) (string, bool) {
	return sessionflow.EpisodeFinaleScreen(current, secret)
}

func formatIntermissionTime(sec int) string {
	if sec < 0 {
		sec = 0
	}
	return fmt.Sprintf("%02d:%02d", sec/60, sec%60)
}

func anyIntermissionSkipInput() bool {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton4) {
		return true
	}
	var keys []ebiten.Key
	keys = inpututil.AppendJustPressedKeys(keys)
	return len(keys) > 0
}
