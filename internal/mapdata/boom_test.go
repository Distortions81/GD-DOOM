package mapdata

import "testing"

func TestBoomExtendedActionSemantics(t *testing.T) {
	for _, tc := range []struct {
		special uint16
		trigger TriggerType
		repeat  bool
		check   func(LineSpecialInfo) bool
	}{
		{142, TriggerWalk, false, func(i LineSpecialInfo) bool { return i.Floor != nil && i.Floor.Action == FloorRaise512 }},
		{143, TriggerWalk, false, func(i LineSpecialInfo) bool { return i.Plat != nil && i.Plat.Action == PlatRaiseAndChange24 }},
		{160, TriggerUse, false, func(i LineSpecialInfo) bool { return i.Floor != nil && i.Floor.Action == FloorRaise24AndChange }},
		{180, TriggerUse, true, func(i LineSpecialInfo) bool { return i.Floor != nil && i.Floor.Action == FloorRaise24 }},
		{175, TriggerUse, false, func(i LineSpecialInfo) bool { return i.Door != nil && i.Door.Action == DoorClose30ThenOpen }},
		{164, TriggerUse, false, func(i LineSpecialInfo) bool { return i.Ceiling != nil && i.Ceiling.Action == CeilingFastCrushRaise }},
		{194, TriggerUse, true, func(i LineSpecialInfo) bool { return i.Light != nil && i.Light.Action == LightTurnTagOff }},
		{16408, TriggerWalk, false, func(i LineSpecialInfo) bool {
			return i.Ceiling != nil && i.Ceiling.Generalized.Speed == 3 && !i.Ceiling.Generalized.Up
		}},
		{208, TriggerWalk, true, func(i LineSpecialInfo) bool { return i.Teleport != nil && i.Teleport.Silent }},
		{195, TriggerUse, true, func(i LineSpecialInfo) bool { return i.Teleport != nil && !i.Teleport.MonsterOnly }},
	} {
		i := LookupLineSpecial(tc.special)
		if i.Special != tc.special || !i.Boom || i.Trigger != tc.trigger || i.Repeat != tc.repeat || !tc.check(i) {
			t.Fatalf("special %d decoded incorrectly: %+v", tc.special, i)
		}
	}
	for _, special := range []uint16{242, 223, 252, 0x3840} {
		if LookupLineSpecial(special).Trigger != TriggerUnknown {
			t.Fatalf("unsupported special %d advertised as implemented", special)
		}
	}
}

func TestBoomSectorSupportAndSecretClearing(t *testing.T) {
	for _, tc := range []struct {
		special           int16
		supported, secret bool
		cleared           int16
	}{
		{9, true, true, 0},
		{128 | 8, true, true, 0},
		{128 | 32 | 8, true, true, 32 | 8},
		{128 | 256, false, true, 256},
		{512, false, false, 512},
		{6, false, false, 0},
	} {
		if SectorSpecialSupported(tc.special) != tc.supported || SectorIsSecret(tc.special) != tc.secret || ClearSectorSecret(tc.special) != tc.cleared {
			t.Fatalf("sector %d classified or cleared incorrectly", tc.special)
		}
	}
}

func TestBoomGeneralizedFloorParameterBits(t *testing.T) {
	info := LookupLineSpecial(0x7fff)
	f := info.Floor.Generalized
	if info.Trigger != TriggerManual || !info.Repeat || info.Floor.UsesTag || f.Speed != 3 || f.Target != 7 || f.Change != 3 || !f.Up || !f.NumericModel || !f.Crush || f.Monsters {
		t.Fatalf("maximal generalized floor: %+v %+v", info, f)
	}
	info = LookupLineSpecial(0x6020)
	if !info.Floor.Generalized.Monsters || info.Trigger != TriggerWalk || info.Repeat || !info.Floor.UsesTag {
		t.Fatalf("monster-triggered generalized floor: %+v", info)
	}
	if LookupLineSpecial(0x8000).Floor != nil {
		t.Fatal("out-of-range special decoded as generalized floor")
	}
}

func TestBoomGeneralizedCeilingAndLiftParameterBits(t *testing.T) {
	c := LookupLineSpecial(0x5fff)
	g := c.Ceiling.Generalized
	if c.Trigger != TriggerManual || !c.Repeat || c.Ceiling.UsesTag || g.Speed != 3 || g.Target != 7 || g.Change != 3 || !g.Up || !g.NumericModel || !g.Crush || g.Monsters {
		t.Fatalf("ceiling parameters: %+v %+v", c, g)
	}
	for trigger := uint16(0); trigger < 8; trigger++ {
		special := uint16(0x37f8) | trigger
		i := LookupLineSpecial(special)
		l := i.Plat.Generalized
		if i.Trigger != [...]TriggerType{TriggerWalk, TriggerWalk, TriggerUse, TriggerUse, TriggerShoot, TriggerShoot, TriggerManual, TriggerManual}[trigger] || i.Repeat != (trigger&1 != 0) || i.Plat.UsesTag != (trigger < 6) || l.Speed != 3 || l.Target != 3 || l.Delay != 3 || !l.Monsters {
			t.Fatalf("lift parameters: %+v %+v", i, l)
		}
	}
	if !LookupLineSpecial(0x4020).Ceiling.Generalized.Monsters || LookupLineSpecial(0x4420).Ceiling.Generalized.Monsters {
		t.Fatal("ceiling model bit incorrectly grants monster activation")
	}
}
func TestBoomGeneralizedEquivalentKeyLocks(t *testing.T) {
	for key := uint16(0); key < 8; key++ {
		i := LookupLineSpecial(0x3a00 | key<<6 | 0x1f)
		d := i.Door
		if d == nil || !d.Generalized.Locked || d.Generalized.Monsters || d.Generalized.Speed != 3 || i.Trigger != TriggerManual || !i.Repeat {
			t.Fatalf("lock %d: %+v", key, i)
		}
		for mask := 0; mask < 8; mask++ {
			keys := KeyRing{Red: mask&1 != 0, Blue: mask&2 != 0, Yellow: mask&4 != 0}
			want := mask != 0
			if key == 7 {
				want = mask == 7
			} else if key != 0 {
				want = mask&(1<<((key-1)%3)) != 0
			}
			if d.CanActivate(keys) != want {
				t.Fatalf("lock %d keys %d allowed=%t", key, mask, d.CanActivate(keys))
			}
		}
	}
	for _, special := range []uint16{0x3840, 0x3980, 0x39c0} {
		if LookupLineSpecial(special).Door != nil {
			t.Fatalf("distinct-card lock %x incorrectly advertised", special)
		}
	}
}
