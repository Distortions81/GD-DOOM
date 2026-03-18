//go:build js && wasm

package audiofx

func maxSpatialVoices() int {
	return 6
}

func maxMenuVoices() int {
	return 4
}
