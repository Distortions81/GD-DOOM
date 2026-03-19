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
		Skill:        skill,
		Deathmatch:   normalizeGameMode(opts.GameMode) == gameModeDeathmatch,
		FastMonsters: opts.FastMonsters,
	}, tics)
}
