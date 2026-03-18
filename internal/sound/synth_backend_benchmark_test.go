package sound

import "testing"

type synthBenchmarkCase struct {
	name string
	regs []uint16
}

var synthBenchmarkCases = []synthBenchmarkCase{
	{
		name: "melodic_fm",
		regs: []uint16{0x20, 0x21, 0x23, 0x01, 0x40, 0x08, 0x43, 0x00, 0x60, 0xF2, 0x63, 0xF2, 0x80, 0x24, 0x83, 0x24, 0xC0, 0x30, 0xA0, 0x98, 0xB0, 0x31},
	},
	{
		name: "bright_feedback",
		regs: []uint16{0x20, 0x21, 0x23, 0x21, 0x40, 0x04, 0x43, 0x00, 0x60, 0xF4, 0x63, 0xF4, 0x80, 0x22, 0x83, 0x22, 0xC0, 0x3C, 0xA0, 0xC0, 0xB0, 0x35},
	},
	{
		name: "trem_vib",
		regs: []uint16{0xBD, 0xC0, 0x20, 0xC1, 0x23, 0xC1, 0x40, 0x18, 0x43, 0x00, 0x60, 0xF3, 0x63, 0xF3, 0x80, 0x34, 0x83, 0x34, 0xC0, 0x30, 0xA0, 0x88, 0xB0, 0x33},
	},
}

func benchmarkSynthReferenceCorpus[B interface {
	GenerateStereoS16(frames int) []int16
	WriteReg(addr uint16, value uint8)
}](b *testing.B, backendName string, newBackend func() B) {
	for _, tc := range synthBenchmarkCases {
		b.Run(backendName+"/"+tc.name, func(b *testing.B) {
			synth := newBackend()
			synth.WriteReg(0x01, 0x20)
			for i := 0; i+1 < len(tc.regs); i += 2 {
				synth.WriteReg(tc.regs[i], uint8(tc.regs[i+1]))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = synth.GenerateStereoS16(2048)
			}
		})
	}
}

func BenchmarkImpSynthReferenceCorpus(b *testing.B) {
	benchmarkSynthReferenceCorpus(b, "impsynth", func() *ImpSynth {
		return NewImpSynth(49716)
	})
}
