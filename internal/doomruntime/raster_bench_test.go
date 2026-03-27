package doomruntime

import "testing"

func BenchmarkFillPackedRun(b *testing.B) {
	dst := make([]uint32, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fillPackedRun(dst, 0, len(dst), 0xA5A5A5A5)
	}
}

func BenchmarkDrawWallColumnTexturedIndexedLEColPow2Row(b *testing.B) {
	pix32 := make([]uint32, 512*256)
	col := make([]byte, 64)
	for i := range col {
		col[i] = byte((i * 37) & 0xFF)
	}
	row := make([]uint32, 256)
	for i := range row {
		row[i] = uint32(i) | 0xABCD0000
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		drawWallColumnTexturedIndexedLEColPow2Row(
			pix32,
			127,
			512,
			col,
			(7<<fracBits)+fracUnit/5,
			fracUnit/3,
			63,
			160,
			row,
		)
	}
}
