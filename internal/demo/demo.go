package demo

import (
	"fmt"
	"os"
	"strings"

	"gddoom/internal/mapdata"
)

const (
	Version109 = 109
	Version110 = 110
	Marker     = 0x80
	HeaderSize = 13
)

const (
	ButtonAttack = 1
	ButtonUse    = 2
)

type Tic struct {
	Forward   int8
	Side      int8
	AngleTurn int16
	Buttons   byte
}

type Header struct {
	Version       byte
	Skill         byte
	Episode       byte
	Map           byte
	Deathmatch    bool
	Respawn       bool
	Fast          bool
	NoMonsters    bool
	ConsolePlayer byte
	PlayerInGame  [4]bool
}

type Script struct {
	Path   string
	Header Header
	Tics   []Tic
}

type RecordingOptions struct {
	Skill        int
	Deathmatch   bool
	FastMonsters bool
}

func Load(path string) (*Script, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read demo %s: %w", path, err)
	}
	script, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse demo %s: %w", path, err)
	}
	script.Path = path
	return script, nil
}

func Parse(data []byte) (*Script, error) {
	if len(data) < HeaderSize+1 {
		return nil, fmt.Errorf("demo too short")
	}
	h := Header{
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
	if h.Version != Version109 && h.Version != Version110 {
		return nil, fmt.Errorf("unsupported demo version %d", h.Version)
	}
	tics := make([]Tic, 0, maxInt(1, (len(data)-HeaderSize-1)/4))
	for i := HeaderSize; i < len(data); {
		if data[i] == Marker {
			if i != len(data)-1 {
				return nil, fmt.Errorf("trailing bytes after demo marker")
			}
			break
		}
		if i+4 > len(data) {
			return nil, fmt.Errorf("truncated demo tic")
		}
		tics = append(tics, Tic{
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
	if data[len(data)-1] != Marker {
		return nil, fmt.Errorf("missing demo marker")
	}
	return &Script{Header: h, Tics: tics}, nil
}

func Format(script *Script) ([]byte, error) {
	if script == nil {
		return nil, fmt.Errorf("demo is nil")
	}
	if len(script.Tics) == 0 {
		return nil, fmt.Errorf("demo has no tics")
	}
	h := script.Header
	if h.Version == 0 {
		h.Version = Version110
	}
	if h.Version != Version109 && h.Version != Version110 {
		return nil, fmt.Errorf("unsupported demo version %d", h.Version)
	}
	buf := make([]byte, 0, HeaderSize+len(script.Tics)*4+1)
	buf = append(buf, h.Version, h.Skill, h.Episode, h.Map)
	buf = append(buf, boolByte(h.Deathmatch), boolByte(h.Respawn), boolByte(h.Fast), boolByte(h.NoMonsters), h.ConsolePlayer)
	for _, v := range h.PlayerInGame {
		buf = append(buf, boolByte(v))
	}
	for _, tc := range script.Tics {
		buf = append(buf, byte(tc.Forward), byte(tc.Side), byte((uint16(tc.AngleTurn)+128)>>8), tc.Buttons)
	}
	buf = append(buf, Marker)
	return buf, nil
}

func Save(path string, script *Script) error {
	data, err := Format(script)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write demo %s: %w", path, err)
	}
	return nil
}

func BuildRecorded(mapName mapdata.MapName, opts RecordingOptions, tics []Tic) (*Script, error) {
	if len(tics) == 0 {
		return nil, fmt.Errorf("demo has no tics")
	}
	header, err := HeaderForRecording(mapName, opts)
	if err != nil {
		return nil, err
	}
	return &Script{
		Header: header,
		Tics:   append([]Tic(nil), tics...),
	}, nil
}

func HeaderForRecording(mapName mapdata.MapName, opts RecordingOptions) (Header, error) {
	episode, slot, ok := MapSlot(mapName)
	if !ok {
		return Header{}, fmt.Errorf("unsupported demo map %q", mapName)
	}
	skill := opts.Skill
	if skill < 0 {
		skill = 0
	}
	header := Header{
		Version:       Version110,
		Skill:         byte(skill),
		Episode:       byte(episode),
		Map:           byte(slot),
		Deathmatch:    opts.Deathmatch,
		Fast:          opts.FastMonsters,
		NoMonsters:    false,
		ConsolePlayer: 0,
	}
	header.PlayerInGame[0] = true
	return header, nil
}

func MapSlot(name mapdata.MapName) (episode int, slot int, ok bool) {
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
