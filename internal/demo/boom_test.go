package demo

import "testing"

func boomDemoBytes() []byte {
	b := make([]byte, 109)
	b[0] = VersionBoom202
	copy(b[1:7], []byte{0x1d, 'B', 'o', 'o', 'm', 0xe6})
	b[8] = 3
	b[9] = 1
	b[10] = 1
	b[77] = 1
	b[21] = 1
	return append(b, 50, 0, 128, ButtonUse, Marker)
}

func TestBoomDemoHeaderAndCommands(t *testing.T) {
	for _, version := range []byte{VersionBoom201, VersionBoom202} {
		b := boomDemoBytes()
		b[0] = version
		s, err := Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		if s.Header.Version != version || s.Header.Skill != 3 || !s.Header.NoMonsters || s.Header.Map != 1 || !s.Header.PlayerInGame[0] || len(s.Tics) != 1 || s.Tics[0].Forward != 50 || s.Tics[0].AngleTurn != -32768 || s.Tics[0].Buttons != ButtonUse {
			t.Fatalf("incorrect Boom header/command: %+v", s)
		}
	}
}

func TestBoomDemoRejectsMalformedAndMultiplayerStreams(t *testing.T) {
	for _, edit := range []func([]byte) []byte{
		func(b []byte) []byte { return b[:108] },
		func(b []byte) []byte { b[1] = 0; return b },
		func(b []byte) []byte { b[78] = 1; return b },
		func(b []byte) []byte { b[81] = 1; return b },
		func(b []byte) []byte { b[77] = 0; return b },
		func(b []byte) []byte { return b[:len(b)-1] },
	} {
		if _, err := Parse(edit(boomDemoBytes())); err == nil {
			t.Fatal("invalid Boom stream accepted")
		}
	}
}
