package sessionflow

import "gddoom/internal/mapdata"

const (
	FinaleTextSpeed       = 3
	FinaleTextWaitTics    = 250
	finaleTextStartDelay  = 10
	FinalePictureHoldTics = 35 * 7
)

type FinaleStage uint8

const (
	FinaleStageText FinaleStage = iota
	FinaleStagePicture
)

type Finale struct {
	Active  bool
	Tic     int
	MapName mapdata.MapName
	Stage   FinaleStage
	Flat    string
	Text    string
	Screen  string
	WaitTic int
}

func StartFinale(current mapdata.MapName, secret bool) (Finale, bool) {
	spec, ok := episodeFinaleSpec(current, secret)
	if !ok {
		return Finale{}, false
	}
	return Finale{
		Active:  true,
		MapName: current,
		Stage:   FinaleStageText,
		Flat:    spec.flat,
		Text:    spec.text,
		Screen:  spec.screen,
		WaitTic: FinalePictureHoldTics,
	}, true
}

func TickFinale(state Finale, skipPressed bool) (Finale, bool) {
	if !state.Active {
		return state, false
	}
	state.Tic++
	if skipPressed && state.Tic <= IntermissionSkipInputDelayTics {
		skipPressed = false
	}
	switch state.Stage {
	case FinaleStageText:
		if skipPressed {
			state.Stage = FinaleStagePicture
			state.Tic = 0
			state.WaitTic = IntermissionSkipExitHoldTics
			return state, false
		}
		if state.Tic > finaleTextTotalTics(state.Text) {
			state.Stage = FinaleStagePicture
			state.Tic = 0
			state.WaitTic = FinalePictureHoldTics
		}
		return state, false
	case FinaleStagePicture:
		if skipPressed && state.WaitTic > IntermissionSkipExitHoldTics {
			state.WaitTic = IntermissionSkipExitHoldTics
		}
		if state.WaitTic > 0 {
			state.WaitTic--
			return state, false
		}
		return Finale{}, true
	default:
		return Finale{}, true
	}
}

func EpisodeFinaleScreen(current mapdata.MapName, secret bool) (string, bool) {
	spec, ok := episodeFinaleSpec(current, secret)
	return spec.screen, ok
}

func FinaleVisibleText(text string, tic int) string {
	count := (tic - finaleTextStartDelay) / FinaleTextSpeed
	if count < 0 {
		count = 0
	}
	if count >= len(text) {
		return text
	}
	return text[:count]
}

func finaleTextTotalTics(text string) int {
	return len(text)*FinaleTextSpeed + FinaleTextWaitTics
}

type finaleSpec struct {
	flat   string
	text   string
	screen string
}

func episodeFinaleSpec(current mapdata.MapName, secret bool) (finaleSpec, bool) {
	if secret {
		return finaleSpec{}, false
	}
	ep, slot, ok := episodeMapSlot(current)
	if !ok || slot != 8 {
		return finaleSpec{}, false
	}
	switch ep {
	case 1:
		return finaleSpec{flat: "FLOOR4_8", text: e1Text, screen: "CREDIT"}, true
	case 2:
		return finaleSpec{flat: "SFLR6_1", text: e2Text, screen: "VICTORY2"}, true
	case 3:
		return finaleSpec{flat: "MFLR8_4", text: e3Text, screen: "ENDPIC"}, true
	case 4:
		return finaleSpec{flat: "MFLR8_3", text: e4Text, screen: "ENDPIC"}, true
	default:
		return finaleSpec{}, false
	}
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

const e1Text = "Once you beat the big badasses and\n" +
	"clean out the moon base you're supposed\n" +
	"to win, aren't you? Aren't you? Where's\n" +
	"your fat reward and ticket home? What\n" +
	"the hell is this? It's not supposed to\n" +
	"end this way!\n" +
	"\n" +
	"It stinks like rotten meat, but looks\n" +
	"like the lost Deimos base.  Looks like\n" +
	"you're stuck on The Shores of Hell.\n" +
	"The only way out is through.\n" +
	"\n" +
	"To continue the DOOM experience, play\n" +
	"The Shores of Hell and its amazing\n" +
	"sequel, Inferno!\n"

const e2Text = "You've done it! The hideous cyber-\n" +
	"demon lord that ruled the lost Deimos\n" +
	"moon base has been slain and you\n" +
	"are triumphant! But ... where are\n" +
	"you? You clamber to the edge of the\n" +
	"moon and look down to see the awful\n" +
	"truth.\n" +
	"\n" +
	"Deimos floats above Hell itself!\n" +
	"You've never heard of anyone escaping\n" +
	"from Hell, but you'll make the bastards\n" +
	"sorry they ever heard of you! Quickly,\n" +
	"you rappel down to  the surface of\n" +
	"Hell.\n" +
	"\n" +
	"Now, it's on to the final chapter of\n" +
	"DOOM! -- Inferno."

const e3Text = "The loathsome spiderdemon that\n" +
	"masterminded the invasion of the moon\n" +
	"bases and caused so much death has had\n" +
	"its ass kicked for all time.\n" +
	"\n" +
	"A hidden doorway opens and you enter.\n" +
	"You've proven too tough for Hell to\n" +
	"contain, and now Hell at last plays\n" +
	"fair -- for you emerge from the door\n" +
	"to see the green fields of Earth!\n" +
	"Home at last.\n" +
	"\n" +
	"You wonder what's been happening on\n" +
	"Earth while you were battling evil\n" +
	"unleashed. It's good that no Hell-\n" +
	"spawn could have come through that\n" +
	"door with you ..."

const e4Text = "the spider mastermind must have sent forth\n" +
	"its legions of hellspawn before your\n" +
	"final confrontation with that terrible\n" +
	"beast from hell.  but you stepped forward\n" +
	"and brought forth eternal damnation and\n" +
	"suffering upon the horde as a true hero\n" +
	"would in the face of something so evil.\n" +
	"\n" +
	"besides, someone was gonna pay for what\n" +
	"happened to daisy, your pet rabbit.\n" +
	"\n" +
	"but now, you see spread before you more\n" +
	"potential pain and gibbitude as a nation\n" +
	"of demons run amok among our cities.\n" +
	"\n" +
	"next stop, hell on earth!"
