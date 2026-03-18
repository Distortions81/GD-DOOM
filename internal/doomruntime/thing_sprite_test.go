package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestWorldThingSpriteName_PickupAndDecor(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"STIMA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAR1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"POSSL0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"SOULA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PINVA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PSTRA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PINSA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PMAPA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PVISA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"MEGAA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TRE1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TRE2A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"ELECA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR1B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR1C0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR2A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR3A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR4A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR5A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLMPA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLMPB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLMPC0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLMPD0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLP2A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLP2B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLP2C0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLP2D0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"CEYEA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"CEYEB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"CEYEC0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"FSKUA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"FSKUB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"FSKUC0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"COL5A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"COL5B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"COLUA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"POL6A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"POL6B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"POL3A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"POL3B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"FCANA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"FCANB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"FCANC0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}
	if got := g.worldThingSpriteName(2011, 0); got != "STIMA0" {
		t.Fatalf("stimpack sprite=%q want STIMA0", got)
	}
	if got := g.worldThingSpriteName(2035, 0); got != "BAR1A0" {
		t.Fatalf("barrel sprite=%q want BAR1A0", got)
	}
	if got := g.worldThingSpriteName(18, 0); got != "POSSL0" {
		t.Fatalf("corpse sprite=%q want POSSL0", got)
	}
	if got := g.worldThingSpriteName(2013, 0); got != "SOULA0" {
		t.Fatalf("soulsphere sprite=%q want SOULA0", got)
	}
	if got := g.worldThingSpriteName(2022, 0); got != "PINVA0" {
		t.Fatalf("invulnerability sprite=%q want PINVA0", got)
	}
	if got := g.worldThingSpriteName(2023, 0); got != "PSTRA0" {
		t.Fatalf("berserk sprite=%q want PSTRA0", got)
	}
	if got := g.worldThingSpriteName(2024, 0); got != "PINSA0" {
		t.Fatalf("invisibility sprite=%q want PINSA0", got)
	}
	if got := g.worldThingSpriteName(2026, 0); got != "PMAPA0" {
		t.Fatalf("computer map sprite=%q want PMAPA0", got)
	}
	if got := g.worldThingSpriteName(2045, 0); got != "PVISA0" {
		t.Fatalf("light amp sprite=%q want PVISA0", got)
	}
	if got := g.worldThingSpriteName(83, 0); got != "MEGAA0" {
		t.Fatalf("megasphere sprite=%q want MEGAA0", got)
	}
	if got := g.worldThingSpriteName(43, 0); got != "TRE1A0" {
		t.Fatalf("torch tree sprite=%q want TRE1A0", got)
	}
	if got := g.worldThingSpriteName(48, 0); got != "ELECA0" {
		t.Fatalf("tech pillar sprite=%q want ELECA0", got)
	}
	if got := g.worldThingSpriteName(2028, 0); got != "COLUA0" {
		t.Fatalf("tech column sprite=%q want COLUA0", got)
	}
	if got := g.worldThingSpriteName(54, 0); got != "TRE2A0" {
		t.Fatalf("big tree sprite=%q want TRE2A0", got)
	}
	if got := g.worldThingSpriteName(53, 0); got != "GOR5A0" {
		t.Fatalf("hanging victim sprite=%q want GOR5A0", got)
	}
	if got := g.worldThingSpriteName(70, 0); got != "FCANA0" {
		t.Fatalf("burning barrel sprite tic0=%q want FCANA0", got)
	}
	if got := g.worldThingSpriteName(70, 4); got != "FCANB0" {
		t.Fatalf("burning barrel sprite tic4=%q want FCANB0", got)
	}
}

func TestWorldThingSpriteFullBright_ParityCases(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "SOULA0", want: true},
		{name: "PINSA0", want: true},
		{name: "PMAPA0", want: true},
		{name: "SUITA0", want: true},
		{name: "MEGAA0", want: true},
		{name: "PVISA0", want: true},
		{name: "PVISB0", want: false},
		{name: "CEYEA0", want: true},
		{name: "FSKUA0", want: true},
		{name: "TLMPA0", want: true},
		{name: "TLP2A0", want: true},
		{name: "POL3A0", want: true},
		{name: "FCANA0", want: true},
		{name: "BKEYA0", want: false},
		{name: "BKEYB0", want: true},
		{name: "ARM1A0", want: false},
		{name: "ARM1B0", want: true},
		{name: "BON1A0", want: false},
		{name: "TRE1A0", want: false},
	}
	for _, tc := range tests {
		if got := worldThingSpriteFullBright(tc.name); got != tc.want {
			t.Fatalf("fullbright(%q)=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestDoomSourceSpriteFullBright_ParityCases(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "CPOSE1", want: true},
		{name: "TROOA1", want: false},
		{name: "BOSFA0", want: true},
		{name: "BAL1A0", want: true},
		{name: "BFE1A0", want: true},
		{name: "PUFFA0", want: true},
		{name: "BAR1A0", want: false},
		{name: "BEXPA0", want: true},
		{name: "TLMPA0", want: true},
		{name: "TRE1A0", want: false},
	}
	for _, tc := range tests {
		if got := doomSourceSpriteFullBright(tc.name); got != tc.want {
			t.Fatalf("doomSourceSpriteFullBright(%q)=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestSpriteRenderRef_UsesDoomSourceFullBrightParity(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"CPOSE1": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLMPA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TROOA1": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}

	for _, tc := range []struct {
		name string
		want bool
	}{
		{name: "CPOSE1", want: true},
		{name: "BAL1A0", want: true},
		{name: "TLMPA0", want: true},
		{name: "TROOA1", want: false},
	} {
		ref, ok := g.spriteRenderRef(tc.name)
		if !ok || ref == nil {
			t.Fatalf("spriteRenderRef(%q) returned no ref", tc.name)
		}
		if ref.fullBright != tc.want {
			t.Fatalf("spriteRenderRef(%q).fullBright=%v want %v", tc.name, ref.fullBright, tc.want)
		}
	}
}

func TestWorldThingSpriteName_DoomTimingParity(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"BAR1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAR1B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BEXPA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BKEYA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BKEYB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BON1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BON1B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BON1C0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BON1D0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"KEENA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TRE1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"FCANA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLMPA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLMPB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLMPC0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"TLMPD0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR1B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR1C0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}

	if got := g.worldThingSpriteName(2035, 0); got != "BAR1A0" {
		t.Fatalf("barrel tic0=%q want BAR1A0", got)
	}
	if got := g.worldThingSpriteName(2035, 5); got != "BAR1A0" {
		t.Fatalf("barrel tic5=%q want BAR1A0", got)
	}
	if got := g.worldThingSpriteName(2035, 6); got != "BAR1B0" {
		t.Fatalf("barrel tic6=%q want BAR1B0", got)
	}
	if got := g.worldThingSpriteName(2035, 12); got != "BEXPA0" {
		t.Fatalf("barrel tic12=%q want BEXPA0", got)
	}
	if got := g.worldThingSpriteName(2035, 18); got != "BAR1A0" {
		t.Fatalf("barrel tic18=%q want BAR1A0", got)
	}

	if got := g.worldThingSpriteName(5, 0); got != "BKEYA0" {
		t.Fatalf("blue key tic0=%q want BKEYA0", got)
	}
	if got := g.worldThingSpriteName(5, 10); got != "BKEYB0" {
		t.Fatalf("blue key tic10=%q want BKEYB0", got)
	}
	if got := g.worldThingSpriteName(5, 20); got != "BKEYA0" {
		t.Fatalf("blue key tic20=%q want BKEYA0", got)
	}

	if got := g.worldThingSpriteName(2014, 0); got != "BON1A0" {
		t.Fatalf("bonus tic0=%q want BON1A0", got)
	}
	if got := g.worldThingSpriteName(2014, 18); got != "BON1D0" {
		t.Fatalf("bonus tic18=%q want BON1D0", got)
	}
	if got := g.worldThingSpriteName(2014, 24); got != "BON1C0" {
		t.Fatalf("bonus tic24=%q want BON1C0", got)
	}

	if got := g.worldThingSpriteName(72, 0); got != "KEENA0" {
		t.Fatalf("keen tic0=%q want KEENA0", got)
	}
	if got := g.worldThingSpriteName(72, 999); got != "KEENA0" {
		t.Fatalf("keen tic999=%q want KEENA0", got)
	}
	if got := g.worldThingSpriteName(43, 0); got != "TRE1A0" {
		t.Fatalf("thing 43 tic0=%q want TRE1A0", got)
	}
	if got := g.worldThingSpriteName(85, 0); got != "TLMPA0" {
		t.Fatalf("tech lamp tic0=%q want TLMPA0", got)
	}
	if got := g.worldThingSpriteName(85, 4); got != "TLMPB0" {
		t.Fatalf("tech lamp tic4=%q want TLMPB0", got)
	}
	if got := g.worldThingSpriteName(49, 10); got != "GOR1B0" {
		t.Fatalf("bloody twitch tic10=%q want GOR1B0", got)
	}
}

func TestRuntimeWorldThingSpriteRef_CachesStaticThingRef(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"STIMA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BON1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BON1B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BON1C0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BON1D0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}

	staticAnim := g.buildThingWorldAnimRef(mapdata.Thing{Type: 2011})
	if staticAnim.staticRef == nil {
		t.Fatal("static thing missing cached sprite ref")
	}
	if got := staticAnim.staticRef.key; got != "STIMA0" {
		t.Fatalf("static ref key=%q want STIMA0", got)
	}

	animatedAnim := g.buildThingWorldAnimRef(mapdata.Thing{Type: 2014})
	if animatedAnim.staticRef != nil {
		t.Fatalf("animated thing unexpectedly cached static ref %q", animatedAnim.staticRef.key)
	}

	g.thingWorldAnimRef = []thingAnimRefState{staticAnim}
	ref, ok := g.runtimeWorldThingSpriteRef(0, mapdata.Thing{Type: 2011}, 37, 1)
	if !ok || ref == nil {
		t.Fatal("runtimeWorldThingSpriteRef returned no ref for static thing")
	}
	if ref != staticAnim.staticRef {
		t.Fatal("runtimeWorldThingSpriteRef did not return cached static ref")
	}
}

func TestWorldThingSpriteName_SmallGreenTorchDiscreteTiming(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"SMGTA0": {Width: 1, Height: 1, RGBA: []byte{255, 0, 0, 255}},
				"SMGTB0": {Width: 1, Height: 1, RGBA: []byte{0, 255, 0, 255}},
				"SMGTC0": {Width: 1, Height: 1, RGBA: []byte{0, 0, 255, 255}},
				"SMGTD0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 0, 255}},
			},
		},
	}

	want := map[int]string{
		0:  "SMGTA0",
		1:  "SMGTA0",
		2:  "SMGTA0",
		3:  "SMGTA0",
		4:  "SMGTB0",
		5:  "SMGTB0",
		6:  "SMGTB0",
		7:  "SMGTB0",
		8:  "SMGTC0",
		9:  "SMGTC0",
		10: "SMGTC0",
		11: "SMGTC0",
		12: "SMGTD0",
		13: "SMGTD0",
		14: "SMGTD0",
		15: "SMGTD0",
		16: "SMGTA0",
	}

	for tic, expected := range want {
		if got := g.worldThingSpriteName(56, tic); got != expected {
			t.Fatalf("thing 56 tic%d=%q want %q", tic, got, expected)
		}
	}
}

func TestWorldThingSpriteNameScaled_SmallGreenTorchUsesDiscreteFrames(t *testing.T) {
	g := &game{
		opts: Options{
			SourcePortMode: true,
			SpritePatchBank: map[string]WallTexture{
				"SMGTA0": {Width: 1, Height: 1, RGBA: []byte{255, 0, 0, 255}},
				"SMGTB0": {Width: 1, Height: 1, RGBA: []byte{0, 255, 0, 255}},
				"SMGTC0": {Width: 1, Height: 1, RGBA: []byte{0, 0, 255, 255}},
				"SMGTD0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 0, 255}},
			},
		},
	}

	if got := g.worldThingSpriteNameScaled(56, 9, 5); got != "SMGTA0" {
		t.Fatalf("thing 56 unit9=%q want SMGTA0", got)
	}
	if got := g.worldThingSpriteNameScaled(56, 10, 5); got != "SMGTA0" {
		t.Fatalf("thing 56 unit10=%q want SMGTA0", got)
	}
	if got := g.worldThingSpriteNameScaled(56, 19, 5); got != "SMGTA0" {
		t.Fatalf("thing 56 unit19=%q want SMGTA0", got)
	}
	if got := g.worldThingSpriteNameScaled(56, 20, 5); got != "SMGTB0" {
		t.Fatalf("thing 56 unit20=%q want SMGTB0", got)
	}
}

func TestWorldThingSpriteName_DoomSourceDecorCoverage(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"COL6A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR2A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR4A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR3A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"GOR5A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"HDB1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"HDB2A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"HDB3A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"HDB4A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"HDB5A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"HDB6A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"POB1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"POB2A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BRS1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}

	for _, tc := range []struct {
		typ  int16
		want string
	}{
		{typ: 37, want: "COL6A0"},
		{typ: 59, want: "GOR2A0"},
		{typ: 60, want: "GOR4A0"},
		{typ: 61, want: "GOR3A0"},
		{typ: 62, want: "GOR5A0"},
		{typ: 73, want: "HDB1A0"},
		{typ: 74, want: "HDB2A0"},
		{typ: 75, want: "HDB3A0"},
		{typ: 76, want: "HDB4A0"},
		{typ: 77, want: "HDB5A0"},
		{typ: 78, want: "HDB6A0"},
		{typ: 79, want: "POB1A0"},
		{typ: 80, want: "POB2A0"},
		{typ: 81, want: "BRS1A0"},
	} {
		if got := g.worldThingSpriteName(tc.typ, 0); got != tc.want {
			t.Fatalf("thing %d sprite=%q want %q", tc.typ, got, tc.want)
		}
	}
}
