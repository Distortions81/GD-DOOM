package doomruntime

import "testing"

func TestSortUseInterceptsPrioritizesSpecialOnTie(t *testing.T) {
	intercepts := []intercept{
		{frac: 0.5, line: 10},
		{frac: 0.5, line: 11},
	}
	lineSpecial := make([]uint16, 12)
	lineSpecial[10] = 0
	lineSpecial[11] = 11

	sortUseIntercepts(intercepts, lineSpecial)
	if intercepts[0].line != 11 {
		t.Fatalf("first intercept line=%d want=11", intercepts[0].line)
	}
}
