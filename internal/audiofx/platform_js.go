//go:build js && wasm

package audiofx

func maxSpatialVoices() int {
	return 4
}

func maxMenuVoices() int {
	return 2
}
