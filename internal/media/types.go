package media

type WallTexture struct {
	RGBA            []byte
	RGBA32          []uint32
	ColMajor        []uint32
	Indexed         []byte
	IndexedColMajor []byte
	OpaqueMask      []byte
	OpaqueColumnTop []int16
	OpaqueColumnBot []int16
	OpaqueRunOffs   []uint32
	OpaqueRuns      []uint32
	OpaqueRowOffs   []uint32
	OpaqueRowRuns   []uint32
	Width           int
	Height          int
	OffsetX         int
	OffsetY         int
	MaskedMidClass  uint8
}

func PackOpaqueRun(start, end int) uint32 {
	return uint32(uint8(start)) | (uint32(uint8(end)) << 8)
}

func UnpackOpaqueRun(run uint32) (int, int) {
	return int(uint8(run)), int(uint8(run >> 8))
}

func (t *WallTexture) EnsureOpaqueColumnBounds() bool {
	if t == nil || t.Width <= 0 || t.Height <= 0 {
		return false
	}
	t.EnsureOpaqueMask()
	if len(t.OpaqueColumnTop) == t.Width &&
		len(t.OpaqueColumnBot) == t.Width &&
		len(t.OpaqueRunOffs) == t.Width+1 &&
		len(t.OpaqueRowOffs) == t.Height+1 {
		return true
	}
	top := make([]int16, t.Width)
	bot := make([]int16, t.Width)
	runOffs := make([]uint32, t.Width+1)
	runs := make([]uint32, 0, t.Width)
	rowOffs := make([]uint32, t.Height+1)
	rowRuns := make([]uint32, 0, t.Height)
	for x := 0; x < t.Width; x++ {
		top[x] = int16(t.Height)
		bot[x] = -1
	}
	switch {
	case len(t.OpaqueMask) == t.Width*t.Height:
		for x := 0; x < t.Width; x++ {
			runOffs[x] = uint32(len(runs))
			runStart := -1
			for y := 0; y < t.Height; y++ {
				opaque := t.OpaqueMask[y*t.Width+x] != 0
				if opaque {
					if y < int(top[x]) {
						top[x] = int16(y)
					}
					if y > int(bot[x]) {
						bot[x] = int16(y)
					}
					if runStart < 0 {
						runStart = y
					}
					continue
				}
				if runStart >= 0 {
					runs = append(runs, PackOpaqueRun(runStart, y-1))
					runStart = -1
				}
			}
			if runStart >= 0 {
				runs = append(runs, PackOpaqueRun(runStart, t.Height-1))
			}
		}
		for y := 0; y < t.Height; y++ {
			rowOffs[y] = uint32(len(rowRuns))
			runStart := -1
			row := y * t.Width
			for x := 0; x < t.Width; x++ {
				opaque := t.OpaqueMask[row+x] != 0
				if opaque {
					if runStart < 0 {
						runStart = x
					}
					continue
				}
				if runStart >= 0 {
					rowRuns = append(rowRuns, PackOpaqueRun(runStart, x-1))
					runStart = -1
				}
			}
			if runStart >= 0 {
				rowRuns = append(rowRuns, PackOpaqueRun(runStart, t.Width-1))
			}
		}
	case len(t.RGBA) == t.Width*t.Height*4:
		for x := 0; x < t.Width; x++ {
			runOffs[x] = uint32(len(runs))
			runStart := -1
			for y := 0; y < t.Height; y++ {
				opaque := t.RGBA[(y*t.Width+x)*4+3] != 0
				if opaque {
					if y < int(top[x]) {
						top[x] = int16(y)
					}
					if y > int(bot[x]) {
						bot[x] = int16(y)
					}
					if runStart < 0 {
						runStart = y
					}
					continue
				}
				if runStart >= 0 {
					runs = append(runs, PackOpaqueRun(runStart, y-1))
					runStart = -1
				}
			}
			if runStart >= 0 {
				runs = append(runs, PackOpaqueRun(runStart, t.Height-1))
			}
		}
		for y := 0; y < t.Height; y++ {
			rowOffs[y] = uint32(len(rowRuns))
			runStart := -1
			rowBase := y * t.Width * 4
			for x := 0; x < t.Width; x++ {
				opaque := t.RGBA[rowBase+x*4+3] != 0
				if opaque {
					if runStart < 0 {
						runStart = x
					}
					continue
				}
				if runStart >= 0 {
					rowRuns = append(rowRuns, PackOpaqueRun(runStart, x-1))
					runStart = -1
				}
			}
			if runStart >= 0 {
				rowRuns = append(rowRuns, PackOpaqueRun(runStart, t.Width-1))
			}
		}
	default:
		return false
	}
	runOffs[t.Width] = uint32(len(runs))
	rowOffs[t.Height] = uint32(len(rowRuns))
	t.OpaqueColumnTop = top
	t.OpaqueColumnBot = bot
	t.OpaqueRunOffs = runOffs
	t.OpaqueRuns = runs
	t.OpaqueRowOffs = rowOffs
	t.OpaqueRowRuns = rowRuns
	return true
}

func (t *WallTexture) EnsureOpaqueMask() bool {
	if t == nil || t.Width <= 0 || t.Height <= 0 {
		return false
	}
	n := t.Width * t.Height
	if len(t.OpaqueMask) == n {
		return true
	}
	if len(t.RGBA) != n*4 {
		return false
	}
	mask := make([]byte, n)
	for i := 0; i < n; i++ {
		if t.RGBA[i*4+3] != 0 {
			mask[i] = 1
		}
	}
	t.OpaqueMask = mask
	return true
}

type PCMSample struct {
	SampleRate           int
	Data                 []byte
	PreparedRate         int
	PreparedMono         []int16
	FaithfulPreparedRate int
	FaithfulPreparedMono []int16
}

type SoundBank struct {
	MenuCursor          PCMSample
	DoorOpen            PCMSample
	DoorClose           PCMSample
	BlazeOpen           PCMSample
	BlazeClose          PCMSample
	SwitchOn            PCMSample
	SwitchOff           PCMSample
	NoWay               PCMSample
	Tink                PCMSample
	ItemUp              PCMSample
	WeaponUp            PCMSample
	PowerUp             PCMSample
	Teleport            PCMSample
	BossBrainSpit       PCMSample
	BossBrainCube       PCMSample
	BossBrainAwake      PCMSample
	BossBrainPain       PCMSample
	BossBrainDeath      PCMSample
	Oof                 PCMSample
	Pain                PCMSample
	ShootPistol         PCMSample
	ShootShotgun        PCMSample
	ShootSuperShotgun   PCMSample
	ShootPlasma         PCMSample
	ShootBFG            PCMSample
	Punch               PCMSample
	ShootFireball       PCMSample
	ShootRocket         PCMSample
	SawUp               PCMSample
	SawIdle             PCMSample
	SawFull             PCMSample
	SawHit              PCMSample
	ShotgunOpen         PCMSample
	ShotgunLoad         PCMSample
	ShotgunClose        PCMSample
	AttackClaw          PCMSample
	AttackSgt           PCMSample
	AttackSkull         PCMSample
	AttackArchvile      PCMSample
	AttackMancubus      PCMSample
	ImpactFire          PCMSample
	ImpactRocket        PCMSample
	BarrelExplode       PCMSample
	SeePosit1           PCMSample
	SeePosit2           PCMSample
	SeePosit3           PCMSample
	SeeBGSit1           PCMSample
	SeeBGSit2           PCMSample
	SeeSgtSit           PCMSample
	SeeCacoSit          PCMSample
	SeeBruiserSit       PCMSample
	SeeKnightSit        PCMSample
	SeeSpiderSit        PCMSample
	SeeBabySit          PCMSample
	SeeCyberSit         PCMSample
	SeePainSit          PCMSample
	SeeSSSit            PCMSample
	SeeVileSit          PCMSample
	SeeSkeSit           PCMSample
	ActivePosAct        PCMSample
	ActiveBGAct         PCMSample
	ActiveDMAct         PCMSample
	ActiveBSPAct        PCMSample
	ActiveVilAct        PCMSample
	ActiveSkeAct        PCMSample
	MonsterPainHumanoid PCMSample
	MonsterPainDemon    PCMSample
	DeathPodth1         PCMSample
	DeathPodth2         PCMSample
	DeathPodth3         PCMSample
	DeathBgdth1         PCMSample
	DeathBgdth2         PCMSample
	DeathSgtDth         PCMSample
	DeathCacoRaw        PCMSample
	DeathBaronRaw       PCMSample
	DeathKnightRaw      PCMSample
	DeathCyberRaw       PCMSample
	DeathSpiderRaw      PCMSample
	DeathArachRaw       PCMSample
	DeathLostSoulRaw    PCMSample
	DeathMancubusRaw    PCMSample
	DeathRevenantRaw    PCMSample
	DeathPainElemRaw    PCMSample
	DeathWolfSSRaw      PCMSample
	DeathArchvileRaw    PCMSample
	DeathZombie         PCMSample
	DeathShotgunGuy     PCMSample
	DeathChaingunner    PCMSample
	DeathImp            PCMSample
	DeathDemon          PCMSample
	DeathCaco           PCMSample
	DeathBaron          PCMSample
	DeathKnight         PCMSample
	DeathCyber          PCMSample
	DeathSpider         PCMSample
	DeathArachnotron    PCMSample
	DeathLostSoul       PCMSample
	DeathMancubus       PCMSample
	DeathRevenant       PCMSample
	DeathPainElemental  PCMSample
	DeathWolfSS         PCMSample
	DeathArchvile       PCMSample
	MonsterDeath        PCMSample
	PlayerDeath         PCMSample
	InterTick           PCMSample
	InterDone           PCMSample
}
