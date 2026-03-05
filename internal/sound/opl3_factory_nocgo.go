//go:build !cgo

package sound

func newOPL3(sampleRate int) OPL3 {
	return NewBasicOPL3(sampleRate)
}
