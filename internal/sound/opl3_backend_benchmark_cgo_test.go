//go:build cgo

package sound

import "testing"

func BenchmarkNukedReferenceCorpus(b *testing.B) {
	benchmarkOPLReferenceCorpus(b, "nuked", func() *NukedOPL3 {
		return NewNukedOPL3(49716)
	})
}
