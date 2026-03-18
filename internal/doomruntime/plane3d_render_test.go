package doomruntime

import "testing"

func planeFixed(cell int64, frac int64) int64 {
	return cell*fracUnit + frac
}

func planeTestIndexedTexture() []byte {
	tex := make([]byte, 64*64)
	for i := range tex {
		tex[i] = byte((i*37 + 11) & 0xFF)
	}
	return tex
}

func planeTestPackedRow() []uint32 {
	row := make([]uint32, 256)
	for i := range row {
		row[i] = packRGBA(uint8(i), uint8(255-i), uint8((i*53)&0xFF))
	}
	return row
}

func drawPlaneSpanIndexedPackedReference(dst []uint32, dstI, count int, texIndexed []byte, packedRow []uint32, stepper planeTexStepper) planeTexStepper {
	for ; count > 0; count-- {
		dst[dstI] = packedRow[texIndexed[stepper.texelIndex()]]
		stepper.advance(1)
		dstI++
	}
	return stepper
}

func TestStepsUntilTexelChange(t *testing.T) {
	tests := []struct {
		name  string
		fixed int64
		step  int64
		want  int
	}{
		{name: "zero step", fixed: planeFixed(3, 17), step: 0, want: planeTexelChangeSentinel},
		{name: "positive interior", fixed: planeFixed(2, fracUnit/2), step: fracUnit / 4, want: 2},
		{name: "positive boundary", fixed: planeFixed(2, 0), step: fracUnit / 4, want: 4},
		{name: "negative interior", fixed: planeFixed(2, fracUnit/2), step: -fracUnit / 4, want: 3},
		{name: "negative boundary", fixed: planeFixed(2, 0), step: -fracUnit / 4, want: 1},
		{name: "negative coordinate positive step", fixed: planeFixed(-1, fracUnit/2), step: fracUnit / 4, want: 2},
		{name: "negative coordinate negative step", fixed: planeFixed(-1, fracUnit/2), step: -fracUnit / 4, want: 3},
	}
	for _, tt := range tests {
		if got := stepsUntilTexelChange(tt.fixed, tt.step); got != tt.want {
			t.Fatalf("%s: stepsUntilTexelChange(%d, %d)=%d want=%d", tt.name, tt.fixed, tt.step, got, tt.want)
		}
	}
}

func TestDrawPlaneSpanIndexedPackedFastMatchesReference(t *testing.T) {
	texIndexed := planeTestIndexedTexture()
	packedRow := planeTestPackedRow()
	tests := []struct {
		name    string
		stepper planeTexStepper
		count   int
	}{
		{
			name: "horizontal",
			stepper: planeTexStepper{
				uFixed:     planeFixed(5, fracUnit/3),
				vFixed:     planeFixed(9, fracUnit/7),
				stepUFixed: fracUnit / 5,
				stepVFixed: 0,
			},
			count: 64,
		},
		{
			name: "diagonal mixed sign",
			stepper: planeTexStepper{
				uFixed:     planeFixed(-3, fracUnit/2),
				vFixed:     planeFixed(12, fracUnit-9),
				stepUFixed: fracUnit / 7,
				stepVFixed: -fracUnit / 9,
			},
			count: 73,
		},
		{
			name: "reverse",
			stepper: planeTexStepper{
				uFixed:     planeFixed(15, 0),
				vFixed:     planeFixed(-8, 0),
				stepUFixed: -fracUnit / 3,
				stepVFixed: fracUnit / 6,
			},
			count: 41,
		},
		{
			name: "tiny steps",
			stepper: planeTexStepper{
				uFixed:     planeFixed(1, fracUnit-3),
				vFixed:     planeFixed(7, 2),
				stepUFixed: 2,
				stepVFixed: -3,
			},
			count: 96,
		},
	}
	for _, tt := range tests {
		if !tt.stepper.repeatEligible() {
			t.Fatalf("%s: expected repeat-eligible stepper", tt.name)
		}
		wantDst := make([]uint32, tt.count)
		gotDst := make([]uint32, tt.count)
		wantStepper := drawPlaneSpanIndexedPackedReference(wantDst, 0, tt.count, texIndexed, packedRow, tt.stepper)
		gotStepper := drawPlaneSpanIndexedPackedFast(gotDst, 0, tt.count, texIndexed, packedRow, tt.stepper)
		for i := range wantDst {
			if gotDst[i] != wantDst[i] {
				t.Fatalf("%s: dst[%d]=%08x want %08x", tt.name, i, gotDst[i], wantDst[i])
			}
		}
		if gotStepper != wantStepper {
			t.Fatalf("%s: stepper=%+v want %+v", tt.name, gotStepper, wantStepper)
		}
	}
}

func TestDrawPlaneSpanIndexedPackedDispatchMatchesReference(t *testing.T) {
	texIndexed := planeTestIndexedTexture()
	packedRow := planeTestPackedRow()
	tests := []struct {
		name    string
		stepper planeTexStepper
		count   int
	}{
		{
			name: "eligible constant",
			stepper: planeTexStepper{
				uFixed:     planeFixed(2, fracUnit/2),
				vFixed:     planeFixed(4, fracUnit/4),
				stepUFixed: 0,
				stepVFixed: 0,
			},
			count: 33,
		},
		{
			name: "eligible diagonal",
			stepper: planeTexStepper{
				uFixed:     planeFixed(11, 5),
				vFixed:     planeFixed(-6, fracUnit/3),
				stepUFixed: fracUnit / 8,
				stepVFixed: fracUnit / 6,
			},
			count: 57,
		},
		{
			name: "ineligible u step",
			stepper: planeTexStepper{
				uFixed:     planeFixed(3, fracUnit/2),
				vFixed:     planeFixed(1, fracUnit/5),
				stepUFixed: fracUnit + fracUnit/3,
				stepVFixed: fracUnit / 7,
			},
			count: 29,
		},
		{
			name: "ineligible v step",
			stepper: planeTexStepper{
				uFixed:     planeFixed(-4, fracUnit/6),
				vFixed:     planeFixed(9, fracUnit/2),
				stepUFixed: -fracUnit / 5,
				stepVFixed: -fracUnit - 5,
			},
			count: 37,
		},
	}
	for _, tt := range tests {
		wantDst := make([]uint32, tt.count)
		gotDst := make([]uint32, tt.count)
		wantStepper := drawPlaneSpanIndexedPackedReference(wantDst, 0, tt.count, texIndexed, packedRow, tt.stepper)
		gotStepper := drawPlaneSpanIndexedPacked(gotDst, 0, tt.count, texIndexed, packedRow, tt.stepper)
		for i := range wantDst {
			if gotDst[i] != wantDst[i] {
				t.Fatalf("%s: dst[%d]=%08x want %08x", tt.name, i, gotDst[i], wantDst[i])
			}
		}
		if gotStepper != wantStepper {
			t.Fatalf("%s: stepper=%+v want %+v", tt.name, gotStepper, wantStepper)
		}
	}
}
