package mapdata

import "fmt"

type TriggerType string

const (
	TriggerUnknown TriggerType = "unknown"
	TriggerManual  TriggerType = "manual"
	TriggerUse     TriggerType = "use"
	TriggerWalk    TriggerType = "walk"
	TriggerShoot   TriggerType = "shoot"
)

type DoorAction string

const (
	DoorOpen            DoorAction = "open"
	DoorClose           DoorAction = "close"
	DoorRaise           DoorAction = "raise"
	DoorClose30ThenOpen DoorAction = "close30_then_open"
	DoorBlazeOpen       DoorAction = "blaze_open"
	DoorBlazeClose      DoorAction = "blaze_close"
	DoorBlazeRaise      DoorAction = "blaze_raise"
)

type KeyType string

const (
	KeyNone   KeyType = "none"
	KeyBlue   KeyType = "blue"
	KeyRed    KeyType = "red"
	KeyYellow KeyType = "yellow"
)

type LineSpecialInfo struct {
	Special  uint16
	Name     string
	Trigger  TriggerType
	Repeat   bool
	Door     *DoorInfo
	Exit     ExitType
	Floor    *FloorInfo
	Plat     *PlatInfo
	Stairs   *StairsInfo
	Light    *LightInfo
	Teleport *TeleportInfo
	Donut    bool
}

type DoorInfo struct {
	Action  DoorAction
	Key     KeyType
	UsesTag bool
}

type FloorAction string

const (
	FloorRaise          FloorAction = "raise"
	FloorRaiseToNearest FloorAction = "raise_to_nearest"
	FloorLowerToLowest  FloorAction = "lower_to_lowest"
	FloorTurboLower     FloorAction = "turbo_lower"
)

type FloorInfo struct {
	Action  FloorAction
	UsesTag bool
}

type PlatAction string

const (
	PlatRaiseToNearestAndChange PlatAction = "raise_to_nearest_and_change"
	PlatDownWaitUpStay          PlatAction = "down_wait_up_stay"
)

type PlatInfo struct {
	Action  PlatAction
	UsesTag bool
}

type StairsAction string

const (
	StairsBuild8  StairsAction = "build_8"
	StairsTurbo16 StairsAction = "build_16"
)

type StairsInfo struct {
	Action  StairsAction
	UsesTag bool
}

type LightAction string

const (
	LightVeryDark          LightAction = "very_dark"
	LightBrightestNeighbor LightAction = "brightest_neighbor"
	LightFullBright        LightAction = "full_bright"
	LightTurnTagOff        LightAction = "turn_tag_off"
	LightStartStrobing     LightAction = "start_strobing"
)

type LightInfo struct {
	Action  LightAction
	UsesTag bool
}

type TeleportInfo struct {
	UsesTag     bool
	MonsterOnly bool
}

type KeyRing struct {
	Blue   bool
	Red    bool
	Yellow bool
}

type DoorStats struct {
	Total               int
	Manual              int
	Use                 int
	Walk                int
	Shoot               int
	Repeat              int
	OneShot             int
	LockedBlue          int
	LockedRed           int
	LockedYellow        int
	TimedCloseIn30      int
	TimedRaiseIn5Minute int
}

type ExitType string

const (
	ExitNone   ExitType = ""
	ExitNormal ExitType = "normal"
	ExitSecret ExitType = "secret"
)

var doorSpecials = map[uint16]LineSpecialInfo{
	1:   {Special: 1, Name: "manual door raise", Trigger: TriggerManual, Repeat: true, Door: &DoorInfo{Action: DoorRaise, Key: KeyNone, UsesTag: false}},
	2:   {Special: 2, Name: "walk open door", Trigger: TriggerWalk, Repeat: false, Door: &DoorInfo{Action: DoorOpen, Key: KeyNone, UsesTag: true}},
	3:   {Special: 3, Name: "walk close door", Trigger: TriggerWalk, Repeat: false, Door: &DoorInfo{Action: DoorClose, Key: KeyNone, UsesTag: true}},
	4:   {Special: 4, Name: "walk raise door", Trigger: TriggerWalk, Repeat: false, Door: &DoorInfo{Action: DoorRaise, Key: KeyNone, UsesTag: true}},
	16:  {Special: 16, Name: "walk close30 open", Trigger: TriggerWalk, Repeat: false, Door: &DoorInfo{Action: DoorClose30ThenOpen, Key: KeyNone, UsesTag: true}},
	26:  {Special: 26, Name: "manual blue raise", Trigger: TriggerManual, Repeat: true, Door: &DoorInfo{Action: DoorRaise, Key: KeyBlue, UsesTag: false}},
	27:  {Special: 27, Name: "manual yellow raise", Trigger: TriggerManual, Repeat: true, Door: &DoorInfo{Action: DoorRaise, Key: KeyYellow, UsesTag: false}},
	28:  {Special: 28, Name: "manual red raise", Trigger: TriggerManual, Repeat: true, Door: &DoorInfo{Action: DoorRaise, Key: KeyRed, UsesTag: false}},
	29:  {Special: 29, Name: "switch raise door", Trigger: TriggerUse, Repeat: false, Door: &DoorInfo{Action: DoorRaise, Key: KeyNone, UsesTag: true}},
	31:  {Special: 31, Name: "manual open", Trigger: TriggerManual, Repeat: false, Door: &DoorInfo{Action: DoorOpen, Key: KeyNone, UsesTag: false}},
	32:  {Special: 32, Name: "manual blue open", Trigger: TriggerManual, Repeat: false, Door: &DoorInfo{Action: DoorOpen, Key: KeyBlue, UsesTag: false}},
	33:  {Special: 33, Name: "manual red open", Trigger: TriggerManual, Repeat: false, Door: &DoorInfo{Action: DoorOpen, Key: KeyRed, UsesTag: false}},
	34:  {Special: 34, Name: "manual yellow open", Trigger: TriggerManual, Repeat: false, Door: &DoorInfo{Action: DoorOpen, Key: KeyYellow, UsesTag: false}},
	42:  {Special: 42, Name: "button close door", Trigger: TriggerUse, Repeat: true, Door: &DoorInfo{Action: DoorClose, Key: KeyNone, UsesTag: true}},
	46:  {Special: 46, Name: "shoot open door", Trigger: TriggerShoot, Repeat: true, Door: &DoorInfo{Action: DoorOpen, Key: KeyNone, UsesTag: true}},
	50:  {Special: 50, Name: "switch close door", Trigger: TriggerUse, Repeat: false, Door: &DoorInfo{Action: DoorClose, Key: KeyNone, UsesTag: true}},
	61:  {Special: 61, Name: "button open door", Trigger: TriggerUse, Repeat: true, Door: &DoorInfo{Action: DoorOpen, Key: KeyNone, UsesTag: true}},
	63:  {Special: 63, Name: "button raise door", Trigger: TriggerUse, Repeat: true, Door: &DoorInfo{Action: DoorRaise, Key: KeyNone, UsesTag: true}},
	75:  {Special: 75, Name: "walk close door", Trigger: TriggerWalk, Repeat: true, Door: &DoorInfo{Action: DoorClose, Key: KeyNone, UsesTag: true}},
	76:  {Special: 76, Name: "walk close30 open", Trigger: TriggerWalk, Repeat: true, Door: &DoorInfo{Action: DoorClose30ThenOpen, Key: KeyNone, UsesTag: true}},
	86:  {Special: 86, Name: "walk open door", Trigger: TriggerWalk, Repeat: true, Door: &DoorInfo{Action: DoorOpen, Key: KeyNone, UsesTag: true}},
	90:  {Special: 90, Name: "walk raise door", Trigger: TriggerWalk, Repeat: true, Door: &DoorInfo{Action: DoorRaise, Key: KeyNone, UsesTag: true}},
	99:  {Special: 99, Name: "button blazing open blue", Trigger: TriggerUse, Repeat: true, Door: &DoorInfo{Action: DoorBlazeOpen, Key: KeyBlue, UsesTag: true}},
	103: {Special: 103, Name: "switch open door", Trigger: TriggerUse, Repeat: false, Door: &DoorInfo{Action: DoorOpen, Key: KeyNone, UsesTag: true}},
	105: {Special: 105, Name: "walk blazing raise", Trigger: TriggerWalk, Repeat: true, Door: &DoorInfo{Action: DoorBlazeRaise, Key: KeyNone, UsesTag: true}},
	106: {Special: 106, Name: "walk blazing open", Trigger: TriggerWalk, Repeat: true, Door: &DoorInfo{Action: DoorBlazeOpen, Key: KeyNone, UsesTag: true}},
	107: {Special: 107, Name: "walk blazing close", Trigger: TriggerWalk, Repeat: true, Door: &DoorInfo{Action: DoorBlazeClose, Key: KeyNone, UsesTag: true}},
	108: {Special: 108, Name: "walk blazing raise", Trigger: TriggerWalk, Repeat: false, Door: &DoorInfo{Action: DoorBlazeRaise, Key: KeyNone, UsesTag: true}},
	109: {Special: 109, Name: "walk blazing open", Trigger: TriggerWalk, Repeat: false, Door: &DoorInfo{Action: DoorBlazeOpen, Key: KeyNone, UsesTag: true}},
	110: {Special: 110, Name: "walk blazing close", Trigger: TriggerWalk, Repeat: false, Door: &DoorInfo{Action: DoorBlazeClose, Key: KeyNone, UsesTag: true}},
	111: {Special: 111, Name: "switch blazing raise", Trigger: TriggerUse, Repeat: false, Door: &DoorInfo{Action: DoorBlazeRaise, Key: KeyNone, UsesTag: true}},
	112: {Special: 112, Name: "switch blazing open", Trigger: TriggerUse, Repeat: false, Door: &DoorInfo{Action: DoorBlazeOpen, Key: KeyNone, UsesTag: true}},
	113: {Special: 113, Name: "switch blazing close", Trigger: TriggerUse, Repeat: false, Door: &DoorInfo{Action: DoorBlazeClose, Key: KeyNone, UsesTag: true}},
	114: {Special: 114, Name: "button blazing raise", Trigger: TriggerUse, Repeat: true, Door: &DoorInfo{Action: DoorBlazeRaise, Key: KeyNone, UsesTag: true}},
	115: {Special: 115, Name: "button blazing open", Trigger: TriggerUse, Repeat: true, Door: &DoorInfo{Action: DoorBlazeOpen, Key: KeyNone, UsesTag: true}},
	116: {Special: 116, Name: "button blazing close", Trigger: TriggerUse, Repeat: true, Door: &DoorInfo{Action: DoorBlazeClose, Key: KeyNone, UsesTag: true}},
	117: {Special: 117, Name: "manual blazing raise", Trigger: TriggerManual, Repeat: true, Door: &DoorInfo{Action: DoorBlazeRaise, Key: KeyNone, UsesTag: false}},
	118: {Special: 118, Name: "manual blazing open", Trigger: TriggerManual, Repeat: false, Door: &DoorInfo{Action: DoorBlazeOpen, Key: KeyNone, UsesTag: false}},
	133: {Special: 133, Name: "switch blazing open blue", Trigger: TriggerUse, Repeat: false, Door: &DoorInfo{Action: DoorBlazeOpen, Key: KeyBlue, UsesTag: true}},
	134: {Special: 134, Name: "button blazing open red", Trigger: TriggerUse, Repeat: true, Door: &DoorInfo{Action: DoorBlazeOpen, Key: KeyRed, UsesTag: true}},
	135: {Special: 135, Name: "switch blazing open red", Trigger: TriggerUse, Repeat: false, Door: &DoorInfo{Action: DoorBlazeOpen, Key: KeyRed, UsesTag: true}},
	136: {Special: 136, Name: "button blazing open yellow", Trigger: TriggerUse, Repeat: true, Door: &DoorInfo{Action: DoorBlazeOpen, Key: KeyYellow, UsesTag: true}},
	137: {Special: 137, Name: "switch blazing open yellow", Trigger: TriggerUse, Repeat: false, Door: &DoorInfo{Action: DoorBlazeOpen, Key: KeyYellow, UsesTag: true}},
}

var exitSpecials = map[uint16]LineSpecialInfo{
	11:  {Special: 11, Name: "switch exit level", Trigger: TriggerUse, Repeat: false, Exit: ExitNormal},
	51:  {Special: 51, Name: "switch secret exit", Trigger: TriggerUse, Repeat: false, Exit: ExitSecret},
	52:  {Special: 52, Name: "walk exit level", Trigger: TriggerWalk, Repeat: false, Exit: ExitNormal},
	124: {Special: 124, Name: "walk secret exit", Trigger: TriggerWalk, Repeat: false, Exit: ExitSecret},
	197: {Special: 197, Name: "shoot exit level", Trigger: TriggerShoot, Repeat: false, Exit: ExitNormal},
	198: {Special: 198, Name: "shoot secret exit", Trigger: TriggerShoot, Repeat: false, Exit: ExitSecret},
}

var floorSpecials = map[uint16]LineSpecialInfo{
	5:  {Special: 5, Name: "walk raise floor", Trigger: TriggerWalk, Repeat: false, Floor: &FloorInfo{Action: FloorRaise, UsesTag: true}},
	18: {Special: 18, Name: "switch raise floor to nearest", Trigger: TriggerUse, Repeat: false, Floor: &FloorInfo{Action: FloorRaiseToNearest, UsesTag: true}},
	23: {Special: 23, Name: "switch lower floor to lowest", Trigger: TriggerUse, Repeat: false, Floor: &FloorInfo{Action: FloorLowerToLowest, UsesTag: true}},
	36: {Special: 36, Name: "walk turbo lower floor", Trigger: TriggerWalk, Repeat: false, Floor: &FloorInfo{Action: FloorTurboLower, UsesTag: true}},
	70: {Special: 70, Name: "button turbo lower floor", Trigger: TriggerUse, Repeat: true, Floor: &FloorInfo{Action: FloorTurboLower, UsesTag: true}},
	82: {Special: 82, Name: "walk lower floor to lowest", Trigger: TriggerWalk, Repeat: true, Floor: &FloorInfo{Action: FloorLowerToLowest, UsesTag: true}},
	91: {Special: 91, Name: "walk raise floor", Trigger: TriggerWalk, Repeat: true, Floor: &FloorInfo{Action: FloorRaise, UsesTag: true}},
	98: {Special: 98, Name: "walk turbo lower floor", Trigger: TriggerWalk, Repeat: true, Floor: &FloorInfo{Action: FloorTurboLower, UsesTag: true}},
}

var platSpecials = map[uint16]LineSpecialInfo{
	20: {Special: 20, Name: "switch plat raise to nearest and change", Trigger: TriggerUse, Repeat: false, Plat: &PlatInfo{Action: PlatRaiseToNearestAndChange, UsesTag: true}},
	22: {Special: 22, Name: "walk plat raise to nearest and change", Trigger: TriggerWalk, Repeat: false, Plat: &PlatInfo{Action: PlatRaiseToNearestAndChange, UsesTag: true}},
	62: {Special: 62, Name: "button plat down wait up stay", Trigger: TriggerUse, Repeat: true, Plat: &PlatInfo{Action: PlatDownWaitUpStay, UsesTag: true}},
	88: {Special: 88, Name: "walk plat down wait up stay", Trigger: TriggerWalk, Repeat: true, Plat: &PlatInfo{Action: PlatDownWaitUpStay, UsesTag: true}},
}

var stairSpecials = map[uint16]LineSpecialInfo{
	7: {Special: 7, Name: "switch build stairs", Trigger: TriggerUse, Repeat: false, Stairs: &StairsInfo{Action: StairsBuild8, UsesTag: true}},
	8: {Special: 8, Name: "walk build stairs", Trigger: TriggerWalk, Repeat: false, Stairs: &StairsInfo{Action: StairsBuild8, UsesTag: true}},
}

var lightSpecials = map[uint16]LineSpecialInfo{
	35:  {Special: 35, Name: "walk lights very dark", Trigger: TriggerWalk, Repeat: false, Light: &LightInfo{Action: LightVeryDark, UsesTag: true}},
	104: {Special: 104, Name: "walk turn tag lights off", Trigger: TriggerWalk, Repeat: false, Light: &LightInfo{Action: LightTurnTagOff, UsesTag: true}},
	138: {Special: 138, Name: "button light turn on brightest nearby", Trigger: TriggerUse, Repeat: true, Light: &LightInfo{Action: LightBrightestNeighbor, UsesTag: true}},
	139: {Special: 139, Name: "button light turn off", Trigger: TriggerUse, Repeat: true, Light: &LightInfo{Action: LightVeryDark, UsesTag: true}},
}

var teleportSpecials = map[uint16]LineSpecialInfo{
	97: {Special: 97, Name: "walk teleport", Trigger: TriggerWalk, Repeat: true, Teleport: &TeleportInfo{UsesTag: true}},
}

var donutSpecials = map[uint16]LineSpecialInfo{
	9: {Special: 9, Name: "switch donut", Trigger: TriggerUse, Repeat: false, Donut: true},
}

func LookupLineSpecial(special uint16) LineSpecialInfo {
	if info, ok := doorSpecials[special]; ok {
		return info
	}
	if info, ok := exitSpecials[special]; ok {
		return info
	}
	if info, ok := floorSpecials[special]; ok {
		return info
	}
	if info, ok := platSpecials[special]; ok {
		return info
	}
	if info, ok := stairSpecials[special]; ok {
		return info
	}
	if info, ok := lightSpecials[special]; ok {
		return info
	}
	if info, ok := teleportSpecials[special]; ok {
		return info
	}
	if info, ok := donutSpecials[special]; ok {
		return info
	}
	return LineSpecialInfo{Special: special, Name: "", Trigger: TriggerUnknown}
}

func (m *Map) DoorStats() DoorStats {
	stats := DoorStats{}
	for _, ld := range m.Linedefs {
		info := LookupLineSpecial(ld.Special)
		if info.Door == nil {
			continue
		}
		stats.Total++
		if info.Repeat {
			stats.Repeat++
		} else {
			stats.OneShot++
		}
		switch info.Trigger {
		case TriggerManual:
			stats.Manual++
		case TriggerUse:
			stats.Use++
		case TriggerWalk:
			stats.Walk++
		case TriggerShoot:
			stats.Shoot++
		}
		switch info.Door.Key {
		case KeyBlue:
			stats.LockedBlue++
		case KeyRed:
			stats.LockedRed++
		case KeyYellow:
			stats.LockedYellow++
		}
	}
	for _, s := range m.Sectors {
		switch s.Special {
		case 10:
			stats.TimedCloseIn30++
		case 14:
			stats.TimedRaiseIn5Minute++
		}
	}
	return stats
}

func (d DoorInfo) CanActivate(keys KeyRing) bool {
	switch d.Key {
	case KeyBlue:
		return keys.Blue
	case KeyRed:
		return keys.Red
	case KeyYellow:
		return keys.Yellow
	default:
		return true
	}
}

func (m *Map) DoorTargetSectors(lineIndex int) ([]int, error) {
	if lineIndex < 0 || lineIndex >= len(m.Linedefs) {
		return nil, fmt.Errorf("linedef index out of range: %d", lineIndex)
	}
	ld := m.Linedefs[lineIndex]
	info := LookupLineSpecial(ld.Special)
	if info.Door == nil {
		return nil, nil
	}

	if info.Door.UsesTag {
		out := make([]int, 0, 4)
		for si, sec := range m.Sectors {
			if sec.Tag >= 0 && uint16(sec.Tag) == ld.Tag {
				out = append(out, si)
			}
		}
		return out, nil
	}

	if ld.SideNum[1] < 0 || int(ld.SideNum[1]) >= len(m.Sidedefs) {
		return nil, nil
	}
	sec := int(m.Sidedefs[int(ld.SideNum[1])].Sector)
	if sec < 0 || sec >= len(m.Sectors) {
		return nil, fmt.Errorf("door linedef %d points to out-of-range sector %d", lineIndex, sec)
	}
	return []int{sec}, nil
}

func (r *RejectMatrix) Rejects(fromSector, toSector int) (bool, error) {
	if r == nil {
		return false, fmt.Errorf("nil reject matrix")
	}
	if fromSector < 0 || toSector < 0 || fromSector >= r.SectorCount || toSector >= r.SectorCount {
		return false, fmt.Errorf("sector out of range: from=%d to=%d count=%d", fromSector, toSector, r.SectorCount)
	}
	pnum := fromSector*r.SectorCount + toSector
	bytenum := pnum >> 3
	bitmask := byte(1 << (pnum & 7))
	if bytenum >= len(r.Data) {
		return false, fmt.Errorf("reject byte out of range: %d >= %d", bytenum, len(r.Data))
	}
	return (r.Data[bytenum] & bitmask) != 0, nil
}
