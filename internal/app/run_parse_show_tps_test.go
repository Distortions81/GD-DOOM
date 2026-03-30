package app

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestRunParseAcceptsShowTPSFlag(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{"-wad", wadPath, "-render=false", "-show-tps"}, &out, &errb)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, errb.String())
	}
}
