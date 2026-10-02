package doomruntime

import (
	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
)

const (
	demoVersion109        = demo.Version109
	demoVersion110        = demo.Version110
	demoMarker            = demo.Marker
	demoButtonAttack      = demo.ButtonAttack
	demoButtonUse         = demo.ButtonUse
	demoButtonChange      = demo.ButtonChange
	demoButtonWeaponMask  = demo.ButtonWeaponMask
	demoButtonWeaponShift = demo.ButtonWeaponShift
	demoButtonSpecial     = demo.ButtonSpecial
	demoHeaderSize        = demo.HeaderSize
)

func demoButtonWeaponSlot(buttons byte) int {
	if buttons&demoButtonChange == 0 {
		return 0
	}
	return int((buttons&demoButtonWeaponMask)>>demoButtonWeaponShift) + 1
}

func (g *game) applyRecordedDemoPause(tc DemoTic) {
	// G_Ticker consumes the next command and toggles BTS_PAUSE before
	// P_Ticker checks paused. WI_Ticker and F_Ticker continue while paused.
	if tc.Buttons&demoButtonSpecial != 0 && tc.Buttons&3 == 1 {
		g.demoPaused = !g.demoPaused
	}
}

func (g *game) tickPausedDemoStatusWidgets() {
	// ST_Ticker still advances face state and M_Random while P_PlayerThink
	// is paused. Damage/bonus decay belongs to the skipped player thinker.
	damage, bonus := g.statusDamageCount, g.statusBonusCount
	attacker, hasAttacker := g.statusAttackerThing, g.statusHasAttacker
	g.tickStatusWidgets()
	g.statusDamageCount, g.statusBonusCount = damage, bonus
	g.statusAttackerThing, g.statusHasAttacker = attacker, hasAttacker
}

func LoadDemoScript(path string) (*DemoScript, error) {
	return demo.Load(path)
}

func ParseDemoScript(data []byte) (*DemoScript, error) {
	return demo.Parse(data)
}

func FormatDemoScript(script *DemoScript) ([]byte, error) {
	return demo.Format(script)
}

func SaveDemoScript(path string, script *DemoScript) error {
	return demo.Save(path, script)
}

func BuildRecordedDemo(mapName mapdata.MapName, opts Options, tics []DemoTic) (*DemoScript, error) {
	skill := normalizeSkillLevel(opts.SkillLevel) - 1
	if skill < 0 {
		skill = 0
	}
	return demo.BuildRecorded(mapName, demo.RecordingOptions{
		Skill:           skill,
		Deathmatch:      normalizeGameMode(opts.GameMode) == gameModeDeathmatch,
		FastMonsters:    opts.FastMonsters,
		RespawnMonsters: opts.RespawnMonsters,
		NoMonsters:      opts.NoMonsters,
	}, tics)
}
