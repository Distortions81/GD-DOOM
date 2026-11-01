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
			return i.Ceiling != nil && i.Ceiling.HighestNeighbor.Speed == 3 && !i.Ceiling.HighestNeighbor.Up
		}},
		{208, TriggerWalk, true, func(i LineSpecialInfo) bool { return i.Teleport != nil && i.Teleport.Silent }},
		{195, TriggerUse, true, func(i LineSpecialInfo) bool { return i.Teleport != nil && !i.Teleport.MonsterOnly }},
	} {
		i := LookupLineSpecial(tc.special)
		if i.Special != tc.special || !i.Boom || i.Trigger != tc.trigger || i.Repeat != tc.repeat || !tc.check(i) {
			t.Fatalf("special %d decoded incorrectly: %+v", tc.special, i)
		}
	}
	for _, special := range []uint16{242, 223, 252, 0x4080} {
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
