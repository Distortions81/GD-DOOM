package automap

import (
	"fmt"
	"os"
	"strings"

	"gddoom/internal/mapdata"
)

const (
	demoVersion109   = 109
	demoVersion110   = 110
	demoMarker       = 0x80
	demoButtonAttack = 1
	demoButtonUse    = 2
	demoHeaderSize   = 13
)

func LoadDemoScript(path string) (*DemoScript, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read demo %s: %w", path, err)
	}
	script, err := ParseDemoScript(data)
	if err != nil {
		return nil, fmt.Errorf("parse demo %s: %w", path, err)
	}
	script.Path = path
	return script, nil
}

func ParseDemoScript(data []byte) (*DemoScript, error) {
	if len(data) < demoHeaderSize+1 {
		return nil, fmt.Errorf("demo too short")
	}
	h := DemoHeader{
		Version:       data[0],
		Skill:         data[1],
		Episode:       data[2],
		Map:           data[3],
		Deathmatch:    data[4] != 0,
		Respawn:       data[5] != 0,
		Fast:          data[6] != 0,
		NoMonsters:    data[7] != 0,
		ConsolePlayer: data[8],
		PlayerInGame: [4]bool{
			data[9] != 0,
			data[10] != 0,
			data[11] != 0,
			data[12] != 0,
		},
	}
	if h.Version != demoVersion109 && h.Version != demoVersion110 {
		return nil, fmt.Errorf("unsupported demo version %d", h.Version)
	}
	tics := make([]DemoTic, 0, max(1, (len(data)-demoHeaderSize-1)/4))
	for i := demoHeaderSize; i < len(data); {
		if data[i] == demoMarker {
			if i != len(data)-1 {
				return nil, fmt.Errorf("trailing bytes after demo marker")
			}
			break
		}
		if i+4 > len(data) {
			return nil, fmt.Errorf("truncated demo tic")
		}
		tics = append(tics, DemoTic{
			Forward:   int8(data[i]),
			Side:      int8(data[i+1]),
			AngleTurn: int16(uint16(data[i+2]) << 8),
			Buttons:   data[i+3],
		})
		i += 4
	}
	if len(tics) == 0 {
		return nil, fmt.Errorf("demo has no tics")
	}
	if data[len(data)-1] != demoMarker {
		return nil, fmt.Errorf("missing demo marker")
	}
	return &DemoScript{Header: h, Tics: tics}, nil
}

func FormatDemoScript(script *DemoScript) ([]byte, error) {
	if script == nil {
		return nil, fmt.Errorf("demo is nil")
	}
	if len(script.Tics) == 0 {
		return nil, fmt.Errorf("demo has no tics")
	}
	h := script.Header
	if h.Version == 0 {
		h.Version = demoVersion110
	}
	if h.Version != demoVersion109 && h.Version != demoVersion110 {
		return nil, fmt.Errorf("unsupported demo version %d", h.Version)
	}
	buf := make([]byte, 0, demoHeaderSize+len(script.Tics)*4+1)
	buf = append(buf, h.Version, h.Skill, h.Episode, h.Map)
	buf = append(buf, boolByte(h.Deathmatch), boolByte(h.Respawn), boolByte(h.Fast), boolByte(h.NoMonsters), h.ConsolePlayer)
	for _, v := range h.PlayerInGame {
		buf = append(buf, boolByte(v))
	}
	for _, tc := range script.Tics {
		buf = append(buf, byte(tc.Forward), byte(tc.Side), byte((uint16(tc.AngleTurn)+128)>>8), tc.Buttons)
	}
	buf = append(buf, demoMarker)
	return buf, nil
}

func SaveDemoScript(path string, script *DemoScript) error {
	data, err := FormatDemoScript(script)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write demo %s: %w", path, err)
	}
	return nil
}

func BuildRecordedDemo(mapName mapdata.MapName, opts Options, tics []DemoTic) (*DemoScript, error) {
	if len(tics) == 0 {
		return nil, fmt.Errorf("demo has no tics")
	}
	header, err := demoHeaderForRecording(mapName, opts)
	if err != nil {
		return nil, err
	}
	return &DemoScript{
		Header: header,
		Tics:   append([]DemoTic(nil), tics...),
	}, nil
}

func demoHeaderForRecording(mapName mapdata.MapName, opts Options) (DemoHeader, error) {
	episode, slot, ok := demoMapSlot(mapName)
	if !ok {
		return DemoHeader{}, fmt.Errorf("unsupported demo map %q", mapName)
	}
	skill := normalizeSkillLevel(opts.SkillLevel) - 1
	if skill < 0 {
		skill = 0
	}
	header := DemoHeader{
		Version:       demoVersion110,
		Skill:         byte(skill),
		Episode:       byte(episode),
		Map:           byte(slot),
		Deathmatch:    normalizeGameMode(opts.GameMode) == gameModeDeathmatch,
		Fast:          opts.FastMonsters,
		NoMonsters:    false,
		ConsolePlayer: 0,
	}
	header.PlayerInGame[0] = true
	return header, nil
}

func demoMapSlot(name mapdata.MapName) (episode int, slot int, ok bool) {
	s := strings.ToUpper(strings.TrimSpace(string(name)))
	if len(s) == 4 && s[0] == 'E' && s[2] == 'M' && s[1] >= '1' && s[1] <= '9' && s[3] >= '1' && s[3] <= '9' {
		return int(s[1] - '0'), int(s[3] - '0'), true
	}
	if len(s) == 5 && strings.HasPrefix(s, "MAP") && s[3] >= '0' && s[3] <= '9' && s[4] >= '0' && s[4] <= '9' {
		return 0, int(s[3]-'0')*10 + int(s[4]-'0'), true
	}
	return 0, 0, false
}

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}
