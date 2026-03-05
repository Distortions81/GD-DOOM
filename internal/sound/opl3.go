package sound

// OPL3 is the runtime synth interface used by the music driver.
type OPL3 interface {
	Reset()
	WriteReg(addr uint16, value uint8)
	GenerateStereoS16(frames int) []int16
	GenerateMonoU8(frames int) []byte
}

// NewOPL3 creates the default OPL3 backend for the current build.
func NewOPL3(sampleRate int) OPL3 {
	return newOPL3(sampleRate)
}
