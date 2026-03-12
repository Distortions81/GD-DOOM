package sessionaudio

import (
	"gddoom/internal/audiofx"
	"gddoom/internal/media"
)

type MenuController struct {
	player *audiofx.MenuPlayer
}

func NewMenuController(bank media.SoundBank, volume float64) *MenuController {
	return &MenuController{player: audiofx.NewMenuPlayer(bank, volume)}
}

func (c *MenuController) Close() {
	if c == nil || c.player == nil {
		return
	}
	c.player.StopAll()
}

func (c *MenuController) SetVolume(v float64) {
	if c == nil || c.player == nil {
		return
	}
	c.player.SetVolume(v)
}

func (c *MenuController) PlayMove() {
	if c == nil || c.player == nil {
		return
	}
	c.player.PlayMove()
}

func (c *MenuController) PlayConfirm() {
	if c == nil || c.player == nil {
		return
	}
	c.player.PlayConfirm()
}

func (c *MenuController) PlayBack() {
	if c == nil || c.player == nil {
		return
	}
	c.player.PlayBack()
}

func (c *MenuController) PlayQuit(commercial bool, seq int) {
	if c == nil || c.player == nil {
		return
	}
	c.player.PlayQuit(commercial, seq)
}
