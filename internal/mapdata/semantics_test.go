package mapdata

import "testing"

func TestLookupLineSpecialDoor(t *testing.T) {
	info := LookupLineSpecial(26)
	if info.Door == nil {
		t.Fatal("special 26 should decode as a door")
	}
	if info.Door.Key != KeyBlue {
		t.Fatalf("special 26 key = %q, want %q", info.Door.Key, KeyBlue)
	}
	if info.Trigger != TriggerManual {
		t.Fatalf("special 26 trigger = %q, want %q", info.Trigger, TriggerManual)
	}
}

func TestLookupLineSpecialExit(t *testing.T) {
	info := LookupLineSpecial(11)
	if info.Exit != ExitNormal {
		t.Fatalf("special 11 exit = %q, want %q", info.Exit, ExitNormal)
	}
	if info.Trigger != TriggerUse {
		t.Fatalf("special 11 trigger = %q, want %q", info.Trigger, TriggerUse)
	}
}

func TestLookupLineSpecialFloor(t *testing.T) {
	info := LookupLineSpecial(18)
	if info.Floor == nil {
		t.Fatal("special 18 should decode as a floor special")
	}
	if info.Floor.Action != FloorRaiseToNearest {
		t.Fatalf("special 18 floor action = %q, want %q", info.Floor.Action, FloorRaiseToNearest)
	}
	if info.Trigger != TriggerUse {
		t.Fatalf("special 18 trigger = %q, want %q", info.Trigger, TriggerUse)
	}
}

func TestLookupLineSpecialTeleport(t *testing.T) {
	info := LookupLineSpecial(97)
	if info.Teleport == nil {
		t.Fatal("special 97 should decode as a teleport special")
	}
	if info.Trigger != TriggerWalk {
		t.Fatalf("special 97 trigger = %q, want %q", info.Trigger, TriggerWalk)
	}
	if !info.Repeat {
		t.Fatal("special 97 should be repeatable")
	}
}

func TestLookupLineSpecialCeiling(t *testing.T) {
	info := LookupLineSpecial(41)
	if info.Ceiling == nil {
		t.Fatal("special 41 should decode as a ceiling special")
	}
	if info.Ceiling.Action != CeilingLowerToFloor {
		t.Fatalf("special 41 ceiling action = %q, want %q", info.Ceiling.Action, CeilingLowerToFloor)
	}
	if info.Trigger != TriggerUse {
		t.Fatalf("special 41 trigger = %q, want %q", info.Trigger, TriggerUse)
	}
}

func TestLookupLineSpecialButtonLightTurnOff(t *testing.T) {
	info := LookupLineSpecial(139)
	if info.Light == nil {
		t.Fatal("special 139 should decode as a light special")
	}
	if info.Light.Action != LightTurnTagOff {
		t.Fatalf("special 139 light action = %q, want %q", info.Light.Action, LightTurnTagOff)
	}
	if info.Trigger != TriggerUse {
		t.Fatalf("special 139 trigger = %q, want %q", info.Trigger, TriggerUse)
	}
	if !info.Repeat {
		t.Fatal("special 139 should be repeatable")
	}
}

func TestRejectMatrixRejectsBounds(t *testing.T) {
	r := &RejectMatrix{SectorCount: 2, Data: []byte{0x00}}
	_, err := r.Rejects(2, 0)
	if err == nil {
		t.Fatal("Rejects should fail for out-of-range sectors")
	}
}

func TestDecodeRejectMatrixZeroLengthFallsBackToNoRejects(t *testing.T) {
	r, err := decodeRejectMatrix(nil, 3)
	if err != nil {
		t.Fatalf("decodeRejectMatrix() error = %v", err)
	}
	if r == nil {
		t.Fatal("decodeRejectMatrix() returned nil matrix")
	}
	if len(r.Data) != 2 {
		t.Fatalf("len(r.Data) = %d, want 2", len(r.Data))
	}
	rejects, err := r.Rejects(0, 2)
	if err != nil {
		t.Fatalf("Rejects() error = %v", err)
	}
	if rejects {
		t.Fatal("Rejects() = true, want false")
	}
}

func TestDoorStatsCountsSectorTimedDoors(t *testing.T) {
	m := &Map{
		Linedefs: []Linedef{{Special: 1}, {Special: 26}, {Special: 103}},
		Sectors:  []Sector{{Special: 10}, {Special: 14}},
	}
	stats := m.DoorStats()
	if stats.Total != 3 {
		t.Fatalf("stats.Total = %d, want 3", stats.Total)
	}
	if stats.TimedCloseIn30 != 1 || stats.TimedRaiseIn5Minute != 1 {
		t.Fatalf("timed stats mismatch: %+v", stats)
	}
}

func TestDoorTargetSectorsManualDoorUsesBackSector(t *testing.T) {
	m := &Map{
		Linedefs: []Linedef{{Special: 1, SideNum: [2]int16{0, 1}}},
		Sidedefs: []Sidedef{{Sector: 0}, {Sector: 2}},
		Sectors:  []Sector{{}, {}, {}},
	}
	targets, err := m.DoorTargetSectors(0)
	if err != nil {
		t.Fatalf("DoorTargetSectors() error = %v", err)
	}
	if len(targets) != 1 || targets[0] != 2 {
		t.Fatalf("DoorTargetSectors() = %v, want [2]", targets)
	}
}
