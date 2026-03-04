package automap

type Options struct {
	Width           int
	Height          int
	StartZoom       float64
	WADHash         string
	Debug           bool
	PlayerSlot      int
	SkillLevel      int
	CheatLevel      int
	Invulnerable    bool
	LineColorMode   string
	SourcePortMode  bool
	AllCheats       bool
	StartInMapMode  bool
	FlatBank        map[string][]byte
	WallTexBank     map[string]WallTexture
	StatusPatchBank map[string]WallTexture
	SoundBank       SoundBank
	DemoScript      *DemoScript
	RecordDemoPath  string
}

type WallTexture struct {
	RGBA     []byte
	RGBA32   []uint32
	ColMajor []uint32
	Width    int
	Height   int
	OffsetX  int
	OffsetY  int
}

type RunResult struct {
	LevelExited bool
	SecretExit  bool
}

type PCMSample struct {
	SampleRate int
	Data       []byte
}

type SoundBank struct {
	DoorOpen   PCMSample
	DoorClose  PCMSample
	BlazeOpen  PCMSample
	BlazeClose PCMSample
	SwitchOn   PCMSample
	SwitchOff  PCMSample
	NoWay      PCMSample
	ItemUp     PCMSample
	WeaponUp   PCMSample
	PowerUp    PCMSample
	Oof        PCMSample
}

type DemoTic struct {
	Forward int64
	Side    int64
	Turn    int
	TurnRaw int64
	Run     bool
	Use     bool
	Fire    bool
}

type DemoScript struct {
	Path string
	Tics []DemoTic
}
