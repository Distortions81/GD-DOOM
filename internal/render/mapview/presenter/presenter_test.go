package presenter

import "testing"

func TestShouldDrawThings(t *testing.T) {
	if ShouldDrawThings(1) {
		t.Fatalf("iddt1 should not draw things")
	}
	if !ShouldDrawThings(2) {
		t.Fatalf("iddt2 should draw things")
	}
}
