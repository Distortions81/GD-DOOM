package sessionflow

import (
	"math"
	"strconv"
	"strings"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
)

type FrontendMode int

const (
	FrontendModeNone FrontendMode = iota
	FrontendModeTitle
	FrontendModeReadThis
	FrontendModeOptions
	FrontendModeSound
	FrontendModeEpisode
	FrontendModeSkill
)

type Frontend struct {
	Mode             FrontendMode
	Active           bool
	MenuActive       bool
	ItemOn           int
	OptionsOn        int
	SoundOn          int
	EpisodeOn        int
	SelectedEpisode  int
	SkillOn          int
	ReadThisPage     int
	ReadThisFromGame bool
	SkullAnimCounter int
	WhichSkull       int
	Tic              int
	Status           string
	StatusTic        int
	AttractSeq       int
	AttractPage      string
	AttractPageTic   int
}

type FrontendSound int

const (
	FrontendSoundNone FrontendSound = iota
	FrontendSoundMove
	FrontendSoundConfirm
	FrontendSoundBack
)

type FrontendInput struct {
	Escape bool
	Up     bool
	Down   bool
	Left   bool
	Right  bool
	Select bool
	Skip   bool
}

type FrontendConfig struct {
	ReadThisPageCount int
	EpisodeChoices    []int
	OptionRows        []int
	MainMenuCount     int
	SkillMenuCount    int
	StatusTics        int
}

type FrontendResult struct {
	State            Frontend
	Sound            FrontendSound
	AdvanceAttract   bool
	ChangeMessages   bool
	ChangeDetail     bool
	ChangeMouse      int
	ChangeMusic      int
	ChangeSFX        int
	StartGameSkill   int
	RequestQuit      bool
	StatusMessage    string
	StatusMessageTic int
}

type AttractActionKind int

const (
	AttractActionNone AttractActionKind = iota
	AttractActionPage
	AttractActionDemo
)

type AttractAction struct {
	Kind      AttractActionKind
	Name      string
	PlayTitle bool
}

func StartFrontend() Frontend {
	return Frontend{
		Active:     true,
		Mode:       FrontendModeTitle,
		MenuActive: false,
		AttractSeq: -1,
	}
}

func AdvanceFrontendFrame(state Frontend, skullBlinkTics int) (Frontend, bool) {
	state.Tic++
	state.SkullAnimCounter++
	if state.SkullAnimCounter >= skullBlinkTics {
		state.SkullAnimCounter = 0
		state.WhichSkull ^= 1
	}
	if state.StatusTic > 0 {
		state.StatusTic--
		if state.StatusTic == 0 {
			state.Status = ""
		}
	}
	advanceAttract := false
	if state.AttractPageTic > 0 {
		state.AttractPageTic--
		if state.AttractPageTic == 0 {
			advanceAttract = true
		}
	}
	return state, advanceAttract
}

func AvailableEpisodeChoices(episodes []int) []int {
	if len(episodes) == 0 {
		return nil
	}
	out := make([]int, 0, len(episodes))
	for _, ep := range episodes {
		if ep >= 1 && ep <= 4 {
			out = append(out, ep)
		}
	}
	return out
}

func HasAttractDemo(demos []*demo.Script, name string) bool {
	for _, d := range demos {
		if d != nil && strings.EqualFold(strings.TrimSpace(d.Path), strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func FrontendAttractSequence(bootMap mapdata.MapName, episodes []int, demo4 bool) []string {
	commercial := strings.HasPrefix(strings.ToUpper(strings.TrimSpace(string(bootMap))), "MAP")
	retail := false
	for _, ep := range episodes {
		if ep == 4 {
			retail = true
			break
		}
	}
	if commercial {
		return []string{"TITLEPIC", "DEMO1", "CREDIT", "DEMO2", "TITLEPIC", "DEMO3"}
	}
	secondPage := "HELP2"
	if retail {
		secondPage = "CREDIT"
	}
	seq := []string{"TITLEPIC", "DEMO1", "CREDIT", "DEMO2", secondPage, "DEMO3"}
	if demo4 {
		seq = append(seq, "DEMO4")
	}
	return seq
}

func AdvanceAttract(state Frontend, seq []string, commercial bool, titleCommercialTics, titleNonCommercialTics, infoTics int) (Frontend, AttractAction, bool) {
	if len(seq) == 0 {
		return state, AttractAction{}, false
	}
	for i := 0; i < len(seq); i++ {
		state.AttractSeq = (state.AttractSeq + 1) % len(seq)
		step := seq[state.AttractSeq]
		state.AttractPage = ""
		state.AttractPageTic = 0
		if strings.HasPrefix(step, "DEMO") {
			return state, AttractAction{Kind: AttractActionDemo, Name: step}, true
		}
		if step == "TITLEPIC" {
			state.AttractPageTic = titleNonCommercialTics
			if commercial {
				state.AttractPageTic = titleCommercialTics
			}
			state.AttractPage = step
			return state, AttractAction{Kind: AttractActionPage, Name: step, PlayTitle: true}, true
		}
		state.AttractPageTic = infoTics
		state.AttractPage = step
		return state, AttractAction{Kind: AttractActionPage, Name: step}, true
	}
	return state, AttractAction{}, false
}

func NewGameStartMap(bootMap mapdata.MapName, episodeChoices []int, selectedEpisode int, customLoader bool) string {
	if !customLoader {
		return string(bootMap)
	}
	startMap := "MAP01"
	if len(episodeChoices) > 1 {
		ep := selectedEpisode
		if ep == 0 {
			ep = episodeChoices[0]
		}
		startMap = "E" + strconv.Itoa(ep) + "M1"
	}
	return startMap
}

func OpenReadThis(state Frontend, fromGame bool) Frontend {
	state.Active = true
	state.Mode = FrontendModeReadThis
	state.MenuActive = false
	state.ReadThisPage = 0
	state.ReadThisFromGame = fromGame
	return state
}

func CloseReadThis(state Frontend) Frontend {
	if state.ReadThisFromGame {
		return Frontend{}
	}
	state.Mode = FrontendModeTitle
	state.MenuActive = true
	state.ReadThisPage = 0
	state.ReadThisFromGame = false
	return state
}

func NextSelectableOptionRow(rows []int, cur, dir int) int {
	if len(rows) == 0 {
		return 0
	}
	idx := 0
	for i, row := range rows {
		if row == cur {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(rows)) % len(rows)
	return rows[idx]
}

func ClampMouseLookSpeed(v float64) float64 {
	if v < 0.5 {
		return 0.5
	}
	if v > 8.0 {
		return 8.0
	}
	return v
}

func MouseSensitivitySpeedForDot(dot int) float64 {
	if dot < 0 {
		dot = 0
	}
	if dot > 9 {
		dot = 9
	}
	const minSpeed = 0.5
	const maxSpeed = 8.0
	if dot == 0 {
		return minSpeed
	}
	if dot == 9 {
		return maxSpeed
	}
	return minSpeed * math.Pow(maxSpeed/minSpeed, float64(dot)/9.0)
}

func MouseSensitivityDot(speed float64) int {
	speed = ClampMouseLookSpeed(speed)
	const minSpeed = 0.5
	const maxSpeed = 8.0
	dot := int(math.Round(math.Log(speed/minSpeed) / math.Log(maxSpeed/minSpeed) * 9.0))
	if dot < 0 {
		return 0
	}
	if dot > 9 {
		return 9
	}
	return dot
}

func NextMouseSensitivity(speed float64, dir int) float64 {
	if dir == 0 {
		return ClampMouseLookSpeed(speed)
	}
	dot := MouseSensitivityDot(speed) + dir
	if dot < 0 {
		dot = 0
	}
	if dot > 9 {
		dot = 9
	}
	return MouseSensitivitySpeedForDot(dot)
}

func VolumeDot(v float64) int {
	dot := int(math.Round(clampUnit(v) * 15.0))
	if dot < 0 {
		return 0
	}
	if dot > 15 {
		return 15
	}
	return dot
}

func MessagesPatch(enabled bool) string {
	if enabled {
		return "M_MSGON"
	}
	return "M_MSGOFF"
}

func DetailPatch(low bool) string {
	if low {
		return "M_GDLOW"
	}
	return "M_GDHIGH"
}

func SourcePortDetailLabel(detail int) string {
	switch detail {
	case 1:
		return "FULL"
	case 2:
		return "1/2"
	case 3:
		return "1/3"
	case 4:
		return "1/4"
	default:
		return "1/" + strconv.Itoa(max(detail, 1))
	}
}

func StepFrontend(state Frontend, input FrontendInput, cfg FrontendConfig) FrontendResult {
	result := FrontendResult{State: state}
	switch state.Mode {
	case FrontendModeReadThis:
		if input.Escape {
			result.State = CloseReadThis(state)
			result.Sound = FrontendSoundBack
			return result
		}
		if input.Skip {
			if state.ReadThisPage+1 < max(cfg.ReadThisPageCount, 1) {
				result.State.ReadThisPage++
				result.Sound = FrontendSoundConfirm
			} else {
				result.State = CloseReadThis(state)
				result.Sound = FrontendSoundBack
			}
		}
		return result
	case FrontendModeSound:
		if input.Escape {
			result.State.Mode = FrontendModeOptions
			result.Sound = FrontendSoundBack
			return result
		}
		if input.Up || input.Down {
			result.State.SoundOn ^= 1
			result.Sound = FrontendSoundMove
		}
		if input.Left {
			if state.SoundOn == 0 {
				result.ChangeSFX = -1
			} else {
				result.ChangeMusic = -1
			}
		}
		if input.Right {
			if state.SoundOn == 0 {
				result.ChangeSFX = 1
			} else {
				result.ChangeMusic = 1
			}
		}
		return result
	case FrontendModeOptions:
		if input.Escape {
			result.State.Mode = FrontendModeTitle
			result.State.MenuActive = true
			result.Sound = FrontendSoundBack
			return result
		}
		if input.Up {
			result.State.OptionsOn = NextSelectableOptionRow(cfg.OptionRows, state.OptionsOn, -1)
			result.Sound = FrontendSoundMove
		}
		if input.Down {
			result.State.OptionsOn = NextSelectableOptionRow(cfg.OptionRows, result.State.OptionsOn, 1)
			result.Sound = FrontendSoundMove
		}
		if input.Left {
			switch state.OptionsOn {
			case 2:
				result.ChangeDetail = true
			case 5:
				result.ChangeMouse = -1
			}
		}
		if input.Right {
			switch state.OptionsOn {
			case 2:
				result.ChangeDetail = true
			case 5:
				result.ChangeMouse = 1
			}
		}
		if input.Select {
			switch state.OptionsOn {
			case 0:
				result.StatusMessage = "NOT IN GAME"
				result.StatusMessageTic = cfg.StatusTics
			case 1:
				result.ChangeMessages = true
				result.Sound = FrontendSoundConfirm
			case 2:
				result.ChangeDetail = true
				result.Sound = FrontendSoundConfirm
			case 5:
				result.ChangeMouse = 1
				result.Sound = FrontendSoundConfirm
			case 7:
				result.State.Mode = FrontendModeSound
				result.Sound = FrontendSoundConfirm
			}
		}
		return result
	case FrontendModeEpisode:
		if input.Escape {
			result.State.Mode = FrontendModeTitle
			result.State.MenuActive = true
			result.Sound = FrontendSoundBack
			return result
		}
		if len(cfg.EpisodeChoices) <= 1 {
			result.State.Mode = FrontendModeSkill
			return result
		}
		if input.Up {
			result.State.EpisodeOn = (state.EpisodeOn + len(cfg.EpisodeChoices) - 1) % len(cfg.EpisodeChoices)
			result.Sound = FrontendSoundMove
		}
		if input.Down {
			result.State.EpisodeOn = (state.EpisodeOn + 1) % len(cfg.EpisodeChoices)
			result.Sound = FrontendSoundMove
		}
		if input.Select {
			epOn := state.EpisodeOn
			if epOn < 0 || epOn >= len(cfg.EpisodeChoices) {
				epOn = 0
			}
			result.State.EpisodeOn = epOn
			result.State.SelectedEpisode = cfg.EpisodeChoices[epOn]
			result.State.Mode = FrontendModeSkill
			result.Sound = FrontendSoundConfirm
		}
		return result
	case FrontendModeSkill:
		if input.Escape {
			if len(cfg.EpisodeChoices) > 1 {
				result.State.Mode = FrontendModeEpisode
			} else {
				result.State.Mode = FrontendModeTitle
				result.State.MenuActive = true
			}
			result.Sound = FrontendSoundBack
			return result
		}
		if input.Up && cfg.SkillMenuCount > 0 {
			result.State.SkillOn = (state.SkillOn + cfg.SkillMenuCount - 1) % cfg.SkillMenuCount
			result.Sound = FrontendSoundMove
		}
		if input.Down && cfg.SkillMenuCount > 0 {
			result.State.SkillOn = (state.SkillOn + 1) % cfg.SkillMenuCount
			result.Sound = FrontendSoundMove
		}
		if input.Select {
			result.Sound = FrontendSoundConfirm
			result.StartGameSkill = state.SkillOn + 1
		}
		return result
	default:
		if input.Escape {
			result.State.MenuActive = !state.MenuActive
			if result.State.MenuActive {
				result.Sound = FrontendSoundMove
			} else {
				result.Sound = FrontendSoundBack
			}
		}
		if !result.State.MenuActive {
			if input.Skip {
				result.State.MenuActive = true
				result.Sound = FrontendSoundMove
			}
			return result
		}
		if input.Up && cfg.MainMenuCount > 0 {
			result.State.ItemOn = (state.ItemOn + cfg.MainMenuCount - 1) % cfg.MainMenuCount
			result.Sound = FrontendSoundMove
		}
		if input.Down && cfg.MainMenuCount > 0 {
			result.State.ItemOn = (state.ItemOn + 1) % cfg.MainMenuCount
			result.Sound = FrontendSoundMove
		}
		if input.Select {
			result.Sound = FrontendSoundConfirm
			switch state.ItemOn {
			case 0:
				if len(cfg.EpisodeChoices) > 1 {
					result.State.EpisodeOn = 0
					result.State.SelectedEpisode = cfg.EpisodeChoices[0]
					result.State.Mode = FrontendModeEpisode
				} else {
					result.State.Mode = FrontendModeSkill
				}
			case 1:
				result.State.Mode = FrontendModeOptions
				if len(cfg.OptionRows) > 0 {
					result.State.OptionsOn = cfg.OptionRows[0]
				}
			case 4:
				result.State = OpenReadThis(state, false)
			case 2, 3:
				result.StatusMessage = "MENU ITEM NOT WIRED YET"
				result.StatusMessageTic = cfg.StatusTics * 2
			case 5:
				result.RequestQuit = true
			}
		}
		return result
	}
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
