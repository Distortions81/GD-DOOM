//go:build raylib && cgo && !js

package main

import (
	"errors"
	"fmt"
	"gddoom/internal/demo"
	"gddoom/internal/wad"
	"os"
	"strings"
)

func validateNativeDemoFlags(play, record, trace string, stop int, pose string) error {
	play, record, trace = strings.TrimSpace(play), strings.TrimSpace(record), strings.TrimSpace(trace)
	if play != "" && record != "" {
		return fmt.Errorf("-demo and -record-demo are mutually exclusive")
	}
	if trace != "" && play == "" {
		return fmt.Errorf("-trace-demo-state requires -demo")
	}
	if stop < 0 {
		return fmt.Errorf("-demo-stop-after-tics must not be negative")
	}
	if play != "" && strings.TrimSpace(pose) != "" {
		return fmt.Errorf("-camera cannot change a demo's recorded player position")
	}
	return nil
}
func loadNativeDemo(wf *wad.File, path string) (*demo.Script, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	script, err := demo.Load(path)
	if err == nil {
		return script, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	lump, ok := wf.LumpByName(strings.ToUpper(path))
	if !ok {
		return nil, err
	}
	data, err := wf.LumpDataView(lump)
	if err != nil {
		return nil, err
	}
	script, err = demo.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse demo %s: %w", path, err)
	}
	script.Path = strings.ToUpper(path)
	return script, nil
}
