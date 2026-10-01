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
