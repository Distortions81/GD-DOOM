package doomruntime

import (
	"image/color"
	"strings"

	"gddoom/internal/render/levelmesh"
	"gddoom/internal/sessionflow"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// NativeMenuRenderer collects the existing frontend's drawing commands. It
// never creates an Ebiten image or starts an Ebiten input/window loop.
type NativeMenuRenderer struct {
	session sessionGame
	view    game
	patches []levelmesh.Patch
}

type NativeMenuView struct {
	State                                      sessionflow.Frontend
	Frame                                      int
	BindingRow, BindingSlot                    int
	BindingCapture                             bool
	BindingActions                             []int
	MusicRow, MusicWAD, MusicGroup, MusicTrack int
	NowPlaying                                 string
	Slots                                      []NativeSaveSlot
	Messages, ShowFPS                          bool
	ScreenBlocks, HUDScale                     int
	QuitLines                                  []string
}

func NewNativeMenuRenderer() *NativeMenuRenderer {
	r := &NativeMenuRenderer{}
	r.session.g, r.session.rt = &r.view, &r.view
	return r
}

func (r *NativeMenuRenderer) Draw(opts Options, v NativeMenuView) []levelmesh.Patch {
	r.patches = r.patches[:0]
	sg := &r.session
	sg.opts, r.view.opts = opts, opts
	r.view.hudMessagesEnabled = v.Messages
	r.view.opts.NoFPS = !v.ShowFPS
	r.view.screenBlocks, r.view.hudScaleStep = v.ScreenBlocks, v.HUDScale
	sg.frontend = v.State
	sg.frontend.Tic = v.Frame
	sg.frontend.WhichSkull = (v.Frame / menuSkullBlinkTics) & 1
	sg.frontendKeybindRow, sg.frontendKeybindSlot, sg.frontendKeybindCapture = v.BindingRow, v.BindingSlot, v.BindingCapture
	sg.musicPlayer = frontendMusicPlayerState{v.MusicRow, v.MusicWAD, v.MusicGroup, v.MusicTrack}
	sg.nowPlayingLevel, sg.nowPlayingMusic = v.NowPlaying, ""
	sg.nativePatches, sg.nativeSaveSlots = &r.patches, v.Slots
	sg.nativeBindingActions = v.BindingActions
	defer func() { sg.nativePatches, sg.nativeSaveSlots, sg.nativeBindingActions = nil, nil, nil }()
	sg.quitPrompt = sessionflow.QuitPrompt{Active: len(v.QuitLines) > 0, Lines: v.QuitLines}
	if sg.quitPrompt.Active {
		sg.drawQuitPrompt(nil)
	} else {
		sg.drawFrontendContents(nil, 320, 200)
	}
	return r.patches
}

func (sg *sessionGame) drawFrontendTextAt(screen *ebiten.Image, text string, x, y, sx, sy float64) {
	if sg.nativePatches != nil {
		sg.appendNativeMenuText(text, x, y, sx, sy, 1)
		return
	}
	if sg.rt != nil {
		sg.rt.sessionDrawHUTextAt(screen, text, x, y, sx, sy)
	}
}

func (sg *sessionGame) appendNativeMenuText(text string, x, y, sx, sy, alpha float64) {
	px, py := x, y
	for _, ch := range strings.ToUpper(text) {
		if ch == '\n' {
			px, py = x, py+9*sy
			continue
		}
		p, ok := sg.g.messageFontTexture(ch)
		if !ok || ch == ' ' {
			px += 4 * sx
			continue
		}
		*sg.nativePatches = append(*sg.nativePatches, levelmesh.Patch{Texture: nativeTexture(&p), X: px - float64(p.OffsetX)*sx, Y: py - float64(p.OffsetY)*sy, W: float64(p.Width) * sx, H: float64(p.Height) * sy, Alpha: alpha})
		px += float64(p.Width) * sx
	}
}

// NativeFrontendConfig is the same navigation configuration used by Ebiten.
func NativeFrontendConfig(episodes []int, saveCount int) sessionflow.FrontendConfig {
	return sessionflow.FrontendConfig{EpisodeChoices: sessionflow.AvailableEpisodeChoices(episodes), ReadThisPageCount: 2, OptionRows: frontendOptionsSelectableRows[:], SoundMenuCount: frontendSoundMenuRowCount, VoiceMenuCount: frontendVoiceMenuRowCount, MainMenuCount: len(frontendMainMenuNames), SkillMenuCount: len(frontendSkillMenuNames), SaveLoadCount: saveCount, StatusTics: doomTicsPerSecond}
}

// NativeMenuPatchNames exposes the shared artwork ordering to host hit testing.
func NativeMenuPatchNames(mode sessionflow.FrontendMode, episodes []int) []string {
	switch mode {
	case frontendModeEpisode:
		var names []string
		for _, ep := range sessionflow.AvailableEpisodeChoices(episodes) {
			names = append(names, frontendEpisodeMenuNames[ep])
		}
		return names
	case frontendModeSkill:
		return frontendSkillMenuNames[:]
	default:
		return frontendMainMenuNames[:]
	}
}

func NativeBindingMenuWindow(selected int, counts ...int) (int, int) {
	count := int(bindingActionCount)
	if len(counts) > 0 {
		count = counts[0]
	}
	return keybindMenuStartRowCount(selected, count), keybindMenuVisibleRows
}

func NativeDefaultMenuHUD(opts Options) (int, int) {
	return defaultScreenBlocks(opts), defaultHUDScaleStep(opts)
}

func (n *NativeMeshGame) SetMenuHUDSettings(messages bool, blocks, scale int) {
	n.g.hudMessagesEnabled = messages
	n.g.screenBlocks, n.g.hudScaleStep = blocks, scale
}

func (n *NativeMeshGame) AdjustMenuHUDSettings(blockDirection, scaleDirection int) (int, int) {
	n.g.adjustScreenBlocks(blockDirection)
	n.g.adjustHUDScale(scaleDirection)
	return n.g.screenBlocks, n.g.hudScaleStep
}

const (
	NativeMenuBindings    = frontendModeKeybinds
	NativeMenuMusicPlayer = frontendModeMusicPlayer
)

func NativeWatchMenuRows() []int { return append([]int(nil), frontendWatchMenuSelectableRows...) }

func NativeQuitPromptLines() []string {
	prompt, _ := NativeStartQuitPrompt(0)
	return prompt.Lines
}

func NativeReadThisPageCount(opts Options) int {
	var patches []levelmesh.Patch
	sg := sessionGame{opts: opts, nativePatches: &patches}
	return len(sg.readThisPageNames())
}

func (sg *sessionGame) frontendFitTextLines(text string, width int) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = sg.ellipsizeIntermissionText(line, width)
	}
	return strings.Join(lines, "\n")
}

var nativeMenuShade = []byte{0, 0, 0, 255}

func (sg *sessionGame) drawFrontendShade(screen *ebiten.Image, alpha uint8) {
	if sg.nativePatches != nil {
		*sg.nativePatches = append(*sg.nativePatches, levelmesh.Patch{Texture: levelmesh.Texture{RGBA: nativeMenuShade, Width: 1, Height: 1}, W: 320, H: 200, Alpha: float64(alpha) / 255})
		return
	}
	if screen != nil {
		ebitenutil.DrawRect(screen, 0, 0, float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy()), color.RGBA{A: alpha})
	}
}

func NativeStartQuitPrompt(seq int) (sessionflow.QuitPrompt, int) {
	return sessionflow.StartQuitPrompt(seq, doomQuitMessages)
}

// NextMouseSensitivity uses the main frontend's WAD-dependent slider spacing.
func (r *NativeMenuRenderer) NextMouseSensitivity(opts Options, speed float64, dir int, cycle bool) float64 {
	r.session.opts, r.view.opts = opts, opts
	label := "M_MSENS"
	if cycle {
		label = "MOUSE SENSITIVITY"
		dir = 1
	}
	_, count, _ := r.session.frontendMouseSensitivityLayout(36, label)
	next := sessionflow.NextMouseSensitivityForCount(speed, dir, count)
	if cycle && next == speed {
		next = sessionflow.MouseSensitivitySpeedForDotCount(0, count)
	}
	return next
}

// AdjustMusicSelection reuses the frontend's catalog bounds and dependent resets.
func (r *NativeMenuRenderer) AdjustMusicSelection(opts Options, row, wad, group, track, dir int) (int, int, int) {
	r.session.opts = opts
	r.session.musicPlayer = frontendMusicPlayerState{row, wad, group, track}
	r.session.frontendMusicPlayerAdjust(dir)
	v := r.session.musicPlayer
	return v.WADOn, v.EpisodeOn, v.TrackOn
}
