package doomrand

// Class numbers are part of the Boom demo format. Never reorder them.
type Class int

const (
	Skullfly     Class = 0
	Damage       Class = 1
	Crush        Class = 2
	Genlift      Class = 3
	Killtics     Class = 4
	Damagemobj   Class = 5
	Painchance   Class = 6
	Lights       Class = 7
	Explode      Class = 8
	Respawn      Class = 9
	Lastlook     Class = 10
	Spawnthing   Class = 11
	Spawnpuff    Class = 12
	Spawnblood   Class = 13
	Missile      Class = 14
	Shadow       Class = 15
	Plats        Class = 16
	Punch        Class = 17
	Punchangle   Class = 18
	Saw          Class = 19
	Plasma       Class = 20
	Gunshot      Class = 21
	Misfire      Class = 22
	Shotgun      Class = 23
	Bfg          Class = 24
	Slimehurt    Class = 25
	Dmspawn      Class = 26
	Missrange    Class = 27
	Trywalk      Class = 28
	Newchase     Class = 29
	Newchasedir  Class = 30
	See          Class = 31
	Facetarget   Class = 32
	Posattack    Class = 33
	Sposattack   Class = 34
	Cposattack   Class = 35
	Spidrefire   Class = 36
	Troopattack  Class = 37
	Sargattack   Class = 38
	Headattack   Class = 39
	Bruisattack  Class = 40
	Tracer       Class = 41
	Skelfist     Class = 42
	Scream       Class = 43
	Brainscream  Class = 44
	Cposrefire   Class = 45
	Brainexp     Class = 46
	Spawnfly     Class = 47
	Misc         Class = 48
	AllInOne     Class = 49
	Opendoor     Class = 50
	Targetsearch Class = 51
	Friends      Class = 52
	Threshold    Class = 53
	Skiptarget   Class = 54
	Enemystrafe  Class = 55
	Avoidcrush   Class = 56
	Stayonlift   Class = 57
	Helpfriend   Class = 58
	Dropoff      Class = 59
	Randomjump   Class = 60
	Defect       Class = 61
	Heretic      Class = 62
	Mbf21        Class = 63
	Hexen        Class = 64
	ClassCount         = 65
)

const DefaultBoomSeed uint32 = 1993

// Snapshot includes configuration, both compatibility indices, every seeded
// stream and the logic-tic shuffle phase. It is comparable and contains no aliases.
type Snapshot struct {
	MenuIndex, PlayIndex int
	Boom, Insurance      bool
	InitialSeed          uint32
	LogicTic             uint32
	Seeds                [ClassCount]uint32
}

func (r *RNG) Capture() Snapshot {
	return Snapshot{MenuIndex: r.rndIndex, PlayIndex: r.prndIndex, Boom: r.boom, Insurance: r.insurance, InitialSeed: r.initialSeed, LogicTic: r.logicTic, Seeds: r.seeds}
}
func (r *RNG) Restore(s Snapshot) {
	r.rndIndex = s.MenuIndex & 255
	r.prndIndex = s.PlayIndex & 255
	r.boom = s.Boom
	r.insurance = s.Insurance
	r.initialSeed = s.InitialSeed
	r.logicTic = s.LogicTic
	r.seeds = s.Seeds
}
func (r *RNG) ConfigureBoom(seed uint32, insurance bool) {
	r.boom = true
	r.insurance = insurance
	r.initialSeed = seed
	r.logicTic = 0
	r.rndIndex = 0
	r.prndIndex = 0
	value := seed*2 + 1
	for i := range r.seeds {
		value *= 69069
		r.seeds[i] = value
	}
}
func (r *RNG) Random(class Class) int {
	if class < 0 || class >= ClassCount {
		panic("invalid Boom RNG class")
	}
	var compat int
	if class == Misc {
		r.rndIndex = (r.rndIndex + 1) & 255
		compat = r.rndIndex
	} else {
		r.prndIndex = (r.prndIndex + 1) & 255
		compat = r.prndIndex
	}
	if !r.boom {
		return int(table[compat])
	}
	if class != Misc && !r.insurance {
		class = AllInOne
	}
	old := r.seeds[class]
	r.seeds[class] = old*1664525 + 221297 + uint32(class)*2
	value := old >> 20
	if r.insurance {
		value += r.logicTic * 7
	}
	return int(value & 255)
}
func Capture() Snapshot                         { return global.Capture() }
func Restore(s Snapshot)                        { global.Restore(s) }
func ConfigureBoom(seed uint32, insurance bool) { global.ConfigureBoom(seed, insurance) }
func AdvanceLogicTic()                          { global.logicTic++ }
func SetLogicTic(tic uint32)                    { global.logicTic = tic }

// LogicTic is the completed-world-tic phase, preserved across maps and saves.
func LogicTic() uint32 { return global.logicTic }
