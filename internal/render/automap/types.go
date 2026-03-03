package automap

type Options struct {
	Width          int
	Height         int
	StartZoom      float64
	PlayerSlot     int
	SkillLevel     int
	CheatLevel     int
	Invulnerable   bool
	LineColorMode  string
	SourcePortMode bool
	AllCheats      bool
	StartInMapMode bool
	MapFloorTex2D  bool
	FlatBank       map[string][]byte
	WallTexBank    map[string]WallTexture
	SoundBank      SoundBank
}

type WallTexture struct {
	RGBA   []byte
	Width  int
	Height int
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
