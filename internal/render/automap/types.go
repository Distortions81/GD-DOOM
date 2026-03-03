package automap

type Options struct {
	Width          int
	Height         int
	StartZoom      float64
	LineColorMode  string
	SourcePortMode bool
	AllCheats      bool
	StartInMapMode bool
	SoundBank      SoundBank
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
