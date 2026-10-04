//go:build raylib && cgo && !js

package main

import (
	"fmt"
	"math"
	"strings"

	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/music"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/sessionflow"
)

type menuPage int

const (
	menuClosed menuPage = iota
	menuMain
	menuOptions
	menuSound
	menuGraphics
	menuNewGame
	menuHelp
	menuQuit
	menuSave
	menuLoad
	menuControls
	menuBindings
	menuMusicPlayer
	menuSkill
	menuSpeaker
)

type nativeSettings struct {
	sfxVolume, musicVolume                   float64
	speakerVolume, musicPan                  float64
	musicCompression                         float64
	musicBackend                             music.Backend
	soundFont, speakerVariant                string
	pcSpeaker                                bool
	musicMuted                               bool
	mouseSensitivity, keyboardSpeed          float64
	mouseLook, mouseInvert, autoWeaponSwitch bool
	smoothCameraYaw                          bool
	bindings                                 runtimecfg.InputBindings
	alwaysRun, debug, fullscreen             bool
	textureFilter                            raymesh.TextureFilter
	lighting                                 raymesh.LightingMode
	fps                                      int
	messages, showFPS                        bool
	screenBlocks, hudScale                   int
	gammaLevel                               int
	crtEffect                                bool
	noAspectCorrection                       bool
	noVsync                                  bool
	detailLevel                              int
	autoDetail                               bool
}
type menuInput struct {
	up, down, left, right, confirm, back bool
	mouseRow, mouseSlot                  int
	captured                             string
	clear, defaults                      bool
	quitYes, quitNo, anyKey              bool
}
type menuCommand int

const (
	menuNoCommand menuCommand = iota
	menuStart
	menuExit
	menuConfirmQuit
	menuSaveGame
	menuLoadGame
	menuPlayMusic
	menuStopMusic
)

type menuRow struct{ label, patch, value string }
type nativeMenu struct {
	musicWAD, musicGroup, musicTrack int
	nowPlaying                       string
	bindingSlot                      int
	capture                          bool
	saveSlots                        []doomruntime.NativeSaveSlot
	saveSlot                         int
	status                           string
	frontend                         bool // No active game until New Game is confirmed.
	page                             menuPage
	row                              int
	maps                             []mapdata.MapName
	mapIndex, skill                  int
	patches                          []levelmesh.Patch
	opts                             doomruntime.Options
	shared                           *doomruntime.NativeMenuRenderer
	flow                             sessionflow.Frontend
	quitPrompt                       sessionflow.QuitPrompt
	quitSequence                     int
	quitFrom                         menuPage
	quitFromRow                      int
	bindingsFrom                     menuPage
	bindingsFromRow                  int
}

func newNativeMenu(opts doomruntime.Options, maps []mapdata.MapName, current mapdata.MapName) *nativeMenu {
	m := &nativeMenu{opts: opts, maps: maps, skill: opts.SkillLevel, shared: doomruntime.NewNativeMenuRenderer(), flow: sessionflow.StartFrontend()}
	m.flow.SkillOn = max(0, min(4, opts.SkillLevel-1))
	if len(opts.Episodes) == 0 {
		for ep := 1; ep <= 4; ep++ {
			for _, name := range maps {
				if strings.HasPrefix(string(name), fmt.Sprintf("E%dM", ep)) {
					m.opts.Episodes = append(m.opts.Episodes, ep)
					break
				}
			}
		}
	}
	for i, name := range maps {
		if name == current {
			m.mapIndex = i
		}
	}
	return m
}
func (m *nativeMenu) open(page menuPage) {
	if m.page == menuQuit {
		return
	}
	if page == menuBindings {
		m.bindingsFrom, m.bindingsFromRow = m.page, m.row
		if m.bindingsFrom != menuOptions && m.bindingsFrom != menuControls {
			m.bindingsFrom, m.bindingsFromRow = menuOptions, nativeOptionRow(7)
		}
	}
	if page == menuQuit {
		m.quitFrom, m.quitFromRow = m.page, m.row
		m.quitPrompt, m.quitSequence = doomruntime.NativeStartQuitPrompt(m.quitSequence)
		m.page = menuQuit
		return
	}
	m.flow.Status, m.flow.StatusTic = "", 0
	m.page, m.row, m.status, m.capture = page, 0, "", false
	if page == menuMain && m.opts.LiveTicSource != nil && m.opts.LiveTicSink == nil {
		m.row = 1
	}
	if page == menuHelp {
		m.flow.ReadThisPage = 0
		m.flow.ReadThisFromGame = !m.frontend
	}
	if page == menuSkill {
		m.row = m.flow.SkillOn
	}
}
func (m *nativeMenu) rows(s nativeSettings) []menuRow {
	onoff := func(v bool) string {
		if v {
			return "ON"
		}
		return "OFF"
	}
	switch m.page {
	case menuMain:
		labels := []string{"NEW GAME", "OPTIONS", "LOAD GAME", "SAVE GAME", "READ THIS", "QUIT"}
		patches := doomruntime.NativeMenuPatchNames(sessionflow.FrontendModeTitle, nil)
		rows := make([]menuRow, len(labels))
		for i, label := range labels {
			rows[i] = menuRow{label: label, patch: patches[i]}
		}
		return rows
	case menuSave, menuLoad:
		rows := make([]menuRow, 0, len(m.saveSlots)+1)
		for _, slot := range m.saveSlots {
			label := fmt.Sprintf("%d: EMPTY SLOT", slot.Slot)
			if slot.Slot == 0 {
				label = "Q: EMPTY SLOT"
			}
			if slot.Present {
				label = fmt.Sprintf("%d: %s", slot.Slot, slot.Current)
				if slot.Slot == 0 {
					label = "Q: " + slot.Current
				}
			}
			rows = append(rows, menuRow{label: label})
		}
		return rows
	case menuOptions:
		return []menuRow{{label: "MESSAGES"}, {label: "STATUS BAR MODE"}, {label: "HUD SIZE"}, {label: "FPS"}, {label: "MOUSE SENSITIVITY"}, {label: "SOUND OPTIONS"}, {label: "KEY BINDINGS"}, {label: "RAYLIB OPTIONS"}}
	case menuControls:
		return []menuRow{{label: "KEY BINDINGS"}, {label: "MOUSE AIM", value: onoff(s.mouseLook)}, {label: "MOUSE INVERT", value: onoff(s.mouseInvert)}, {label: "MOUSE SPEED", value: fmt.Sprintf("%.1f", s.mouseSensitivity)}, {label: "KEY TURN SPEED", value: fmt.Sprintf("%.1f", s.keyboardSpeed)}, {label: "ALWAYS RUN", value: onoff(s.alwaysRun)}, {label: "AUTO WEAPONS", value: onoff(s.autoWeaponSwitch)}, {label: "SMOOTH CAMERA", value: onoff(s.smoothCameraYaw)}, {label: "BACK"}}
	case menuBindings:
		rows := []menuRow{}
		for _, row := range nativeBindingRows(s.bindings) {
			rows = append(rows, menuRow{label: row.Label})
		}
		return rows
	case menuMusicPlayer:
		return m.musicRows()
	case menuSound:
		return []menuRow{{label: "EFFECTS"}, {label: "MUSIC"}, {label: "SYNTH"}, {label: "SOUNDFONT"}, {label: "PLAYER"}}
	case menuSpeaker:
		return []menuRow{{label: "PC SPEAKER SFX", value: onoff(s.pcSpeaker)}, {label: "SPEAKER VOLUME", value: fmt.Sprintf("%d%%", int(math.Round(s.speakerVolume*100)))}, {label: "SPEAKER MODEL", value: strings.ToUpper(s.speakerVariant)}, {label: "BACK"}}
	case menuGraphics:
		fps := fmt.Sprintf("%d", s.fps)
		if s.fps == 0 {
			fps = "UNCAPPED"
		}
		return []menuRow{{label: "FILTER", value: strings.ToUpper(string(s.textureFilter))}, {label: "LIGHTING", value: strings.ToUpper(string(s.lighting))}, {label: "FRAME LIMIT", value: fps}, {label: "FULLSCREEN", value: onoff(s.fullscreen)}, {label: "DIAGNOSTICS", value: onoff(s.debug)}, {label: "CONTROLS"}, {label: "PC SPEAKER OPTIONS"}, {label: "BACK"}}
	case menuNewGame, menuSkill:
		mode := sessionflow.FrontendModeEpisode
		if m.page == menuSkill {
			mode = sessionflow.FrontendModeSkill
		}
		names := doomruntime.NativeMenuPatchNames(mode, m.opts.Episodes)
		rows := make([]menuRow, len(names))
		for i, name := range names {
			rows[i] = menuRow{patch: name}
		}
		return rows

	}
	return nil
}
func (m *nativeMenu) back() {
	if m.isSharedPage() {
		m.updateShared(menuInput{back: true, mouseRow: -1}, &nativeSettings{})
		return
	}
	switch m.page {
	case menuMusicPlayer:
		m.open(menuSound)
		m.row = 4
	case menuBindings:
		page, row := m.bindingsFrom, m.bindingsFromRow
		m.open(page)
		m.row = row
	case menuControls:
		m.open(menuGraphics)
		m.row = 5
	case menuSpeaker:
		m.open(menuGraphics)
		m.row = 6
	case menuGraphics:
		m.open(menuOptions)
		m.row = nativeOptionRow(8)
	case menuOptions, menuNewGame, menuHelp, menuSave, menuLoad:
		m.open(menuMain)
	default:
		m.open(menuClosed)
	}
}
func (m *nativeMenu) update(in menuInput, s *nativeSettings) menuCommand {
	if m.isSharedPage() {
		return m.updateShared(in, s)
	}
	if m.page == menuClosed {
		return menuNoCommand
	}
	if m.page == menuBindings && m.capture {
		if in.back {
			m.capture = false
			return menuNoCommand
		}
		if in.captured != "" {
			rows := nativeBindingRows(s.bindings)
			doomruntime.SetNativeBinding(&s.bindings, rows[m.row].ID, m.bindingSlot, in.captured)
			m.capture = false
		}
		return menuNoCommand
	}
	if m.page == menuBindings {
		rows := nativeBindingRows(s.bindings)
		if in.defaults {
			s.bindings = runtimecfg.DefaultInputBindings()
			m.setStatus("DEFAULT BINDINGS RESTORED", 70)
			return menuNoCommand
		}
		if in.clear && m.row < len(rows) {
			doomruntime.SetNativeBinding(&s.bindings, rows[m.row].ID, m.bindingSlot, "")
			return menuNoCommand
		}
	}
	if in.back {
		m.back()
		return menuNoCommand
	}
	rows := m.rows(*s)
	if len(rows) == 0 {
		if in.confirm {
			m.back()
		}
		return menuNoCommand
	}
	if in.up {
		if m.page == menuBindings {
			m.row = max(0, m.row-1)
		} else {
			m.row = (m.row + len(rows) - 1) % len(rows)
		}
	}
	if in.down {
		if m.page == menuBindings {
			m.row = min(len(rows)-1, m.row+1)
		} else {
			m.row = (m.row + 1) % len(rows)
		}
	}
	if in.mouseRow >= 0 && in.mouseRow < len(rows) {
		m.row = in.mouseRow
		if m.page == menuBindings {
			m.bindingSlot = in.mouseSlot
		}
	}
	dir := 0
	if in.left {
		dir = -1
	}
	if in.right || in.confirm {
		dir = 1
	}
	if dir == 0 {
		return menuNoCommand
	}
	cycle := func(index, n int) int { return (index + dir + n) % n }
	switch m.page {
	case menuControls:
		switch m.row {
		case 0:
			if in.confirm {
				m.open(menuBindings)
			}
		case 1:
			s.mouseLook = !s.mouseLook
		case 2:
			s.mouseInvert = !s.mouseInvert
		case 3:
			s.mouseSensitivity = math.Round(math.Max(.1, math.Min(5, s.mouseSensitivity+float64(dir)*.1))*10) / 10
		case 4:
			s.keyboardSpeed = math.Round(math.Max(.1, math.Min(5, s.keyboardSpeed+float64(dir)*.1))*10) / 10
		case 5:
			s.alwaysRun = !s.alwaysRun
		case 6:
			s.autoWeaponSwitch = !s.autoWeaponSwitch
		case 7:
			s.smoothCameraYaw = !s.smoothCameraYaw
		case 8:
			if in.confirm {
				m.back()
			}
		}
	case menuBindings:
		count := len(nativeBindingRows(s.bindings))
		if in.left || in.right {
			if in.left {
				m.bindingSlot = 0
			} else {
				m.bindingSlot = 1
			}
		}
		if in.confirm && m.row < count {
			m.capture = true
		}
	case menuSpeaker:
		switch m.row {
		case 0:
			s.pcSpeaker = !s.pcSpeaker
		case 1:
			s.speakerVolume = math.Round(math.Max(0, math.Min(1, s.speakerVolume+float64(dir)*.1))*10) / 10
		case 2:
			values := []string{"passthrough", "paper-speaker", "small-buzzer"}
			index := 1
			for i, value := range values {
				if value == s.speakerVariant {
					index = i
				}
			}
			s.speakerVariant = values[cycle(index, len(values))]
		case 3:
			if in.confirm {
				m.open(menuGraphics)
				m.row = 6
			}
		}
	case menuMusicPlayer:
		if m.row < 3 {
			if in.left || in.right {
				m.adjustMusicSelection(dir)
			}
			if in.confirm {
				return menuPlayMusic
			}

		}
	case menuGraphics:
		switch m.row {
		case 0:
			values := []raymesh.TextureFilter{raymesh.Nearest, raymesh.Trilinear, raymesh.Anisotropic}
			for i, v := range values {
				if v == s.textureFilter {
					s.textureFilter = values[cycle(i, len(values))]
					break
				}
			}
		case 1:
			values := []raymesh.LightingMode{raymesh.DoomLighting, raymesh.SectorLighting, raymesh.FullbrightLighting}
			for i, v := range values {
				if v == s.lighting {
					s.lighting = values[cycle(i, len(values))]
					break
				}
			}
		case 2:
			values := []int{0, 60, 120, 144, 240}
			i := 0
			for j, v := range values {
				if v == s.fps {
					i = j
				}
			}
			s.fps = values[cycle(i, len(values))]
		case 3:
			s.fullscreen = !s.fullscreen
		case 4:
			s.debug = !s.debug
		case 5:
			if in.confirm {
				m.open(menuControls)
			}
		case 6:
			if in.confirm {
				m.open(menuSpeaker)
			}
		case 7:
			if in.confirm {
				m.back()
			}
		}

	}
	return menuNoCommand
}

func (m *nativeMenu) addPatch(name string, x, y float64) bool {
	tex, ok := m.opts.MenuPatchBank[name]
	if !ok {
		return false
	}
	m.patches = append(m.patches, levelmesh.Patch{Texture: levelmesh.Texture{RGBA: tex.RGBA, Width: tex.Width, Height: tex.Height}, X: x - float64(tex.OffsetX), Y: y - float64(tex.OffsetY), W: float64(tex.Width), H: float64(tex.Height)})
	return true
}
func (m *nativeMenu) textWidth(text string) float64 {
	width := 0.0
	for _, r := range strings.ToUpper(text) {
		if p, ok := m.opts.MessageFontBank[r]; ok {
			width += float64(p.Width)
		} else {
			width += 4
		}
	}
	return width
}
func (m *nativeMenu) text(text string, x, y float64) {
	for _, r := range strings.ToUpper(text) {
		if r == '\n' {
			x = 16
			y += 12
			continue
		}
		tex, ok := m.opts.MessageFontBank[r]
		if !ok {
			x += 4
			continue
		}
		m.patches = append(m.patches, levelmesh.Patch{Texture: levelmesh.Texture{RGBA: tex.RGBA, Width: tex.Width, Height: tex.Height}, X: x - float64(tex.OffsetX), Y: y - float64(tex.OffsetY), W: float64(tex.Width), H: float64(tex.Height)})
		x += float64(tex.Width)
	}
}
func (m *nativeMenu) centered(text string, y float64) { m.text(text, (320-m.textWidth(text))/2, y) }

var nativeBeginPromptBackground = []byte{0, 0, 0, 144}

func (m *nativeMenu) drawBeginPrompt() []levelmesh.Patch {
	m.patches = m.patches[:0]
	const prompt = "PRESS ANY KEY TO START"
	width := m.textWidth(prompt)
	m.patches = append(m.patches, levelmesh.Patch{
		Texture: levelmesh.Texture{RGBA: nativeBeginPromptBackground, Width: 1, Height: 1},
		X:       (320-width)/2 - 8, Y: 12, W: width + 16, H: 17,
	})
	m.centered(prompt, 16)
	return m.patches
}

func (m *nativeMenu) textScaled(text string, x, y, scale float64) {
	start := len(m.patches)
	m.text(text, x, y)
	for i := start; i < len(m.patches); i++ {
		patch := &m.patches[i]
		patch.X, patch.Y = x+(patch.X-x)*scale, y+(patch.Y-y)*scale
		patch.W, patch.H = patch.W*scale, patch.H*scale
	}
}

func (m *nativeMenu) draw(s nativeSettings, frame int) []levelmesh.Patch {
	if m.isSharedPage() || m.page == menuBindings || m.page == menuMusicPlayer {
		return m.drawShared(s, frame)
	}
	m.patches = m.patches[:0]
	if m.page == menuClosed {
		return m.patches
	}
	title := map[menuPage]string{menuGraphics: "RAYLIB OPTIONS", menuControls: "CONTROLS", menuSpeaker: "PC SPEAKER OPTIONS"}[m.page]
	m.centered(title, 35)
	rows := m.rows(s)
	first, step, offset, count := m.rowLayout(len(rows))
	for i := offset; i < offset+count; i++ {
		row := rows[i]
		y := first + float64(i-offset)*step
		m.text(row.label, 52, y)
		value := row.value
		limit := 304 - 52 - m.textWidth(row.label) - 12
		for len(value) > 0 && m.textWidth(value) > limit {
			value = value[:len(value)-1]
		}
		m.text(value, 304-m.textWidth(value), y)
	}
	skull := "M_SKULL1"
	if (frame/8)&1 != 0 {
		skull = "M_SKULL2"
	}
	tex, haveSkull := m.opts.MenuPatchBank[skull]
	if !haveSkull || !m.addPatch(skull, 24+float64(tex.OffsetX), first-2+float64(m.row-offset)*step+float64(tex.OffsetY)) {
		m.text(">", 30, first+float64(m.row-offset)*step)
	}
	m.centered("ARROWS: SELECT  ENTER: OK  ESC: BACK", 190)
	return m.patches
}

// rowAt converts the centered 320x200 menu back into logical coordinates.
func (m *nativeMenu) rowAt(x, y float64, width, height int) int {
	scale, ox, oy := raymesh.MenuTransform(width, height)
	lx := (x - ox) / scale
	ly := (y - oy) / scale
	first, step, offset, count := m.rowLayout(len(m.rows(nativeSettings{})))
	right := 310.0
	if m.page == menuSave || m.page == menuLoad {
		right = 210
	}
	if lx < 24 || lx >= right || ly < first-2 || ly >= first-2+float64(count)*step {
		return -1
	}
	return offset + int((ly-(first-2))/step)
}

// The same row geometry drives drawing and mouse hit testing, including scroll.
func (m *nativeMenu) rowLayout(total int) (first, step float64, offset, count int) {
	first, step, count = 60, 15, total
	if m.page == menuMain {
		first, step = 64, 16
	}
	if m.page == menuSound || m.page == menuMusicPlayer {
		first, step = 65, 17
	}
	if m.page == menuSound {
		first, step = 44, 16
	}
	if m.page == menuControls {
		first, step = 54, 14
	}
	if m.page == menuSave || m.page == menuLoad || m.page == menuBindings {
		first, step, count = 54, 16, min(total, 7)
		offset = max(0, min(m.row-count/2, total-count))
	}
	switch m.page {
	case menuNewGame, menuSkill:
		first, step = 63, 16
	case menuOptions:
		first, step = 37, 16
	case menuMusicPlayer:
		first, step = 42, 16
	case menuBindings:
		first, step = 40, 16
		offset, count = doomruntime.NativeBindingMenuWindow(m.row, total)
		count = min(count, total-offset)
	}
	return
}

func (m *nativeMenu) bindingSlotAt(x float64, width, height int) int {
	scale, ox, _ := raymesh.MenuTransform(width, height)
	lx := (x - ox) / scale
	if lx >= 245 {
		return 1
	}
	return 0
}

func (m *nativeMenu) setStatus(message string, tics int) {
	m.status, m.flow.Status, m.flow.StatusTic = message, message, tics
}

// advanceFrame is called at Doom's 35 Hz, independently of the renderer rate.
func (m *nativeMenu) advanceFrame() menuCommand {
	if m.page == menuClosed {
		return menuNoCommand
	}
	if m.page == menuQuit {
		if m.quitPrompt.ExitDelayTic > 0 {
			var done bool
			m.quitPrompt, done = sessionflow.TickQuitPrompt(m.quitPrompt, false, false)
			if done {
				return menuExit
			}
		}
		return menuNoCommand
	}
	m.flow, _ = sessionflow.AdvanceFrontendFrame(m.flow, 8)
	m.status = m.flow.Status
	return menuNoCommand
}
