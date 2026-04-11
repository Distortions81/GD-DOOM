package sessionaudio

import (
	"gddoom/internal/audiofx"
	"gddoom/internal/media"
	"gddoom/internal/sound"
)

type MenuController struct {
	player    *audiofx.MenuPlayer
	pcSpeaker audiofx.PCSpeaker
	pcBank    map[string][]sound.PCSpeakerTone
	ownsPC    bool
}

func NewMenuController(bank media.SoundBank, pcBank map[string][]sound.PCSpeakerTone, sharedPCSpeaker audiofx.PCSpeaker, volume float64, variant audiofx.PCSpeakerVariant) *MenuController {
	c := &MenuController{}
	if len(pcBank) > 0 {
		c.pcSpeaker = sharedPCSpeaker
		if c.pcSpeaker == nil {
			c.pcSpeaker = audiofx.NewPCSpeakerPlayer(volume, variant)
			c.ownsPC = true
		} else {
			c.pcSpeaker.SetVolume(volume)
		}
		c.pcBank = pcBank
	} else {
		c.player = audiofx.NewMenuPlayer(bank, volume)
	}
	return c
}

// pcPlay plays the first matching DS name from the pc speaker bank.
func (c *MenuController) pcPlay(names ...string) {
	if c.pcSpeaker == nil {
		return
	}
	for _, name := range names {
		if seq, ok := c.pcBank[name]; ok && len(seq) > 0 {
			c.pcSpeaker.Play(seq)
			return
		}
	}
}

func (c *MenuController) Close() {
	if c == nil {
		return
	}
	if c.player != nil {
		c.player.StopAll()
	}
	if c.pcSpeaker != nil && c.ownsPC {
		c.pcSpeaker.Stop()
	}
}

func (c *MenuController) SetVolume(v float64) {
	if c == nil {
		return
	}
	if c.player != nil {
		c.player.SetVolume(v)
	}
}

func (c *MenuController) PlayMove() {
	if c == nil {
		return
	}
	if c.pcSpeaker != nil {
		c.pcPlay("DSPSTOP", "DSSWTCHN")
		return
	}
	if c.player != nil {
		c.player.PlayMove()
	}
}

func (c *MenuController) PlayConfirm() {
	if c == nil {
		return
	}
	if c.pcSpeaker != nil {
		c.pcPlay("DSPISTOL", "DSSWTCHN")
		return
	}
	if c.player != nil {
		c.player.PlayConfirm()
	}
}

func (c *MenuController) PlayBack() {
	if c == nil {
		return
	}
	if c.pcSpeaker != nil {
		c.pcPlay("DSSWTCHX", "DSNOWAY")
		return
	}
	if c.player != nil {
		c.player.PlayBack()
	}
}

func (c *MenuController) PlayQuit(commercial bool, seq int) {
	if c == nil {
		return
	}
	if c.pcSpeaker != nil {
		// Mirror the quit sequence DS names from NewMenuPlayer.
		quit1 := []string{"DSPLDETH", "DSPOPAIN", "DSPOPAIN", "DSRXPLOD", "DSGETPOW", "DSPOSIT1", "DSPOSIT3", "DSSGTATK"}
		quit2 := []string{"DSVILACT", "DSGETPOW", "DSCYBSIT", "DSRXPLOD", "DSCLAW", "DSKNTDTH", "DSBSPACT", "DSSGTATK"}
		names := quit1
		if commercial {
			names = quit2
		}
		if len(names) == 0 {
			return
		}
		c.pcPlay(names[seq%len(names)])
		return
	}
	if c.player != nil {
		c.player.PlayQuit(commercial, seq)
	}
}
