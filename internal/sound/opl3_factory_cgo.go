//go:build cgo

package sound

func newOPL3(sampleRate int) OPL3 {
	return NewNukedOPL3(sampleRate)
}
