package automap

import "testing"

func TestQuitPromptScaleForSizeAllowsSub320Surfaces(t *testing.T) {
	scale := quitPromptScaleForSize(160, 100)
	if scale != 0.5 {
		t.Fatalf("expected 0.5 scale for 160x100 surface, got %v", scale)
	}
}

func TestQuitPromptLinesForRenderSizeFallsBackWhenMessageDoesNotFit(t *testing.T) {
	sg := &sessionGame{
		g: &game{},
		quitPrompt: quitPromptState{
			Active: true,
			Lines: []string{
				"THIS MESSAGE IS FAR TOO WIDE FOR THE PROMPT AREA AND SHOULD FALL BACK",
				"(PRESS Y TO QUIT)",
			},
		},
	}

	lines := sg.quitPromptLinesForRenderSize(320, 200)
	want := defaultQuitPromptLines()
	if len(lines) != len(want) {
		t.Fatalf("expected %d fallback lines, got %d", len(want), len(lines))
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("expected fallback line %d to be %q, got %q", i, want[i], lines[i])
		}
	}
}

func TestQuitPromptFitsRenderSizeRejectsTooManyLines(t *testing.T) {
	sg := &sessionGame{g: &game{}}
	lines := []string{
		"LINE 1",
		"LINE 2",
		"LINE 3",
		"LINE 4",
		"LINE 5",
		"LINE 6",
		"LINE 7",
		"LINE 8",
		"LINE 9",
		"LINE 10",
		"LINE 11",
		"LINE 12",
		"LINE 13",
		"LINE 14",
		"LINE 15",
		"LINE 16",
	}
	if sg.quitPromptFitsRenderSize(lines, 320, 200) {
		t.Fatal("expected oversized multi-line prompt to be rejected")
	}
}
