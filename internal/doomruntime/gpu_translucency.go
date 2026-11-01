package doomruntime

func (r *gpuRenderer) tranmapOffset(table []byte) int {
	if r.tranmapRows == nil {
		r.tranmapRows = make(map[*byte]int)
	}
	if offset, ok := r.tranmapRows[&table[0]]; ok {
		return offset
	}
	offset := len(r.metadataPixels) / (256 * 4)
	pixels := make([]byte, len(r.metadataPixels)+65536*4)
	copy(pixels, r.metadataPixels)
	for i, index := range table {
		pixels[(offset*256+i)*4] = index
		pixels[(offset*256+i)*4+3] = 255
	}
	r.metadataPixels = pixels
	r.metadata.Deallocate()
	r.metadata = newUnmanagedImage(256, len(pixels)/(256*4))
	r.metadataDirty = true
	r.tranmapRows[&table[0]] = offset
	return offset
}
func (r *gpuRenderer) startTranslucentCutout(table []byte, x0, x1 int) *gpuCommands {
	offset := r.tranmapOffset(table)
	p := r.nextCutoutPass(false)
	p.tranmapOffset = offset
	p.tranmapX0, p.tranmapX1 = x0, x1
	return &p.color
}
