package main

import (
	"io"
	"strings"
	"testing"
)

func TestTicReaderFiltersMetadataAndCountsUnmatchedTail(t *testing.T) {
	reader := newTicReader(strings.NewReader("{\"kind\":\"meta\"}\n\ninvalid JSON\n{\"kind\":\"tic\",\"gametic\":0}\n{\"kind\":\"tic\",\"gametic\":1}\n{\"kind\":\"end\"}\n"))
	line, err := reader.next()
	if err != nil || string(line) != "{\"kind\":\"tic\",\"gametic\":0}" || reader.count != 1 {
		t.Fatalf("first tic=%s count=%d err=%v", line, reader.count, err)
	}
	if err := reader.drain(); err != nil || reader.count != 2 {
		t.Fatalf("tail count=%d err=%v", reader.count, err)
	}
	if _, err := reader.next(); err != io.EOF || reader.count != 2 {
		t.Fatalf("EOF changed count: count=%d err=%v", reader.count, err)
	}
}

func TestTicReaderHandlesLargeFramesAndReportsScannerErrors(t *testing.T) {
	frame := "{\"kind\":\"tic\",\"padding\":\"" + strings.Repeat("x", 2*1024*1024) + "\"}"
	reader := newTicReader(strings.NewReader(frame + "\n"))
	if line, err := reader.next(); err != nil || string(line) != frame {
		t.Fatalf("large frame length=%d err=%v", len(line), err)
	}
	reader = newTicReader(strings.NewReader(frame))
	reader.scanner.Buffer(make([]byte, 16), 64)
	if _, err := reader.next(); err == nil || err == io.EOF {
		t.Fatal("scanner error was treated as an empty trace")
	}
}

func TestCeilingOldDirectionComparedOnlyInStasis(t *testing.T) {
	for _, direction := range []float64{-1, 0, 1} {
		left := map[string]any{"kind": "ceiling", "direction": direction, "olddirection": float64(16843009)}
		right := map[string]any{"kind": "ceiling", "direction": direction, "olddirection": float64(-1)}
		_, _, _, differs := firstDiff("root.specials[0]", left, right)
		if differs != (direction == 0) {
			t.Fatalf("direction=%v differs=%t; olddirection matters only in stasis", direction, differs)
		}
	}
}

func TestSemanticCeilingComparisonIsOptInAndLimitedToUnusedField(t *testing.T) {
	ceiling := func(topheight float64) map[string]any {
		return map[string]any{"kind": "ceiling", "type": float64(2), "direction": float64(-1),
			"topheight": topheight, "bottomheight": float64(524288), "speed": float64(65536), "sector": float64(83)}
	}
	semantic := compareConfig{ignoreUnusedCeilingTopheight: true}
	left, right := ceiling(16843009), ceiling(0)
	if _, _, _, differs := firstDiff("root.specials[0]", left, right); !differs {
		t.Fatal("strict comparison hid uninitialized topheight")
	}
	if _, _, _, differs := firstDiffWithConfig("root.specials[0]", left, right, semantic); differs {
		t.Fatal("semantic comparison rejected only the unused topheight")
	}
	if left["topheight"] != float64(16843009) {
		t.Fatal("semantic comparison rewrote the raw input")
	}
	for _, field := range []string{"bottomheight", "speed", "sector"} {
		right = ceiling(0)
		right[field] = float64(1)
		if _, _, _, differs := firstDiffWithConfig("root.specials[0]", left, right, semantic); !differs {
			t.Fatalf("semantic comparison hid live field %s", field)
		}
	}
	for _, path := range []string{"root.player", "root.specials[0].nested", "root.specials[0][1]"} {
		if _, _, _, differs := firstDiffWithConfig(path, left, ceiling(0), semantic); !differs {
			t.Fatalf("semantic comparison applied outside a special record: %s", path)
		}
	}
	for _, direction := range []float64{0, 1} {
		left, right = ceiling(16843009), ceiling(0)
		left["direction"], right["direction"] = direction, direction
		if _, _, _, differs := firstDiffWithConfig("root.specials[0]", left, right, semantic); !differs {
			t.Fatalf("ignored potentially live topheight for direction=%v", direction)
		}
	}
	for _, field := range []string{"kind", "type", "direction"} {
		left, right = ceiling(16843009), ceiling(0)
		switch field {
		case "kind":
			right[field] = "floor"
		case "type":
			right[field] = float64(3)
		case "direction":
			right[field] = float64(1)
		}
		if shouldIgnoreUnusedCeilingTopheight("root.specials[0]", "topheight", left, right) {
			t.Fatalf("unpaired %s allowed semantic normalization", field)
		}
	}
	left, right = ceiling(16843009), ceiling(0)
	left["type"], right["type"] = float64(3), float64(3)
	if _, _, _, differs := firstDiffWithConfig("root.specials[0]", left, right, semantic); !differs {
		t.Fatal("ignored crusher's live topheight")
	}
	// Verify the opt-in configuration survives recursive traversal of a tic.
	leftRoot := map[string]any{"specials": []any{ceiling(16843009)}, "gametic": float64(12)}
	rightRoot := map[string]any{"specials": []any{ceiling(0)}, "gametic": float64(12)}
	if _, _, _, differs := firstDiffWithConfig("root", leftRoot, rightRoot, semantic); differs {
		t.Fatal("semantic config was lost while descending into specials")
	}
	rightRoot["gametic"] = float64(13)
	if _, _, _, differs := firstDiffWithConfig("root", leftRoot, rightRoot, semantic); !differs {
		t.Fatal("semantic comparison hid a different tic")
	}
}
