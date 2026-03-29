//go:build js && wasm

package audiofx

func maxSpatialVoices() int {
	return 8
}

func maxMenuVoices() int {
	return 8
}
