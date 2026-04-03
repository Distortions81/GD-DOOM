package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
)

type compareConfig struct {
	maxPlayerDistance float64
	ignoreTransientFX bool
}

func main() {
	fs := flag.NewFlagSet("demotracecmp", flag.ExitOnError)
	leftPath := fs.String("left", "", "left trace JSONL")
	rightPath := fs.String("right", "", "right trace JSONL")
	maxPlayerDistance := fs.Float64("max-player-distance", 0, "compare only mobjs within this many map units of the player (0 disables)")
	ignoreTransientFX := fs.Bool("ignore-transient-fx", false, "ignore transient FX/projectile mobjs during compare")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *leftPath == "" || *rightPath == "" {
		fmt.Fprintln(os.Stderr, "usage: demotracecmp -left <trace.jsonl> -right <trace.jsonl> [-max-player-distance n] [-ignore-transient-fx]")
		os.Exit(2)
	}
	cfg := compareConfig{
		maxPlayerDistance: *maxPlayerDistance,
		ignoreTransientFX: *ignoreTransientFX,
	}

	left, err := readJSONL(*leftPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read left: %v\n", err)
		os.Exit(1)
	}
	right, err := readJSONL(*rightPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read right: %v\n", err)
		os.Exit(1)
	}

	left = filterKind(left, "tic")
	right = filterKind(right, "tic")

	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	for i := 0; i < limit; i++ {
		var l any
		var r any
		if err := json.Unmarshal(left[i], &l); err != nil {
			fmt.Fprintf(os.Stderr, "parse left line %d: %v\n", i+1, err)
			os.Exit(1)
		}
		if err := json.Unmarshal(right[i], &r); err != nil {
			fmt.Fprintf(os.Stderr, "parse right line %d: %v\n", i+1, err)
			os.Exit(1)
		}
		l = normalizeTraceObject(l, cfg)
		r = normalizeTraceObject(r, cfg)
		if path, lv, rv, ok := firstDiff("root", l, r); ok {
			fmt.Printf("mismatch line=%d path=%s\n", i+1, path)
			fmt.Printf("left=%s\n", marshalCompact(lv))
			fmt.Printf("right=%s\n", marshalCompact(rv))
			if lm, rm, idx, ok := mobjForPath(path, l, r); ok {
				fmt.Printf("left_mobj[%d] type=%d x=%d y=%d z=%d\n", idx,
					int(numValue(lm["type"])), int(numValue(lm["x"])), int(numValue(lm["y"])), int(numValue(lm["z"])))
				fmt.Printf("right_mobj[%d] type=%d x=%d y=%d z=%d\n", idx,
					int(numValue(rm["type"])), int(numValue(rm["x"])), int(numValue(rm["y"])), int(numValue(rm["z"])))
			}
			os.Exit(1)
		}
	}

	if len(left) != len(right) {
		fmt.Printf("length mismatch left=%d right=%d\n", len(left), len(right))
		os.Exit(1)
	}
	fmt.Printf("traces match lines=%d\n", len(left))
}

func normalizeTraceObject(v any, cfg compareConfig) any {
	root, ok := v.(map[string]any)
	if !ok {
		return v
	}
	if cfg.maxPlayerDistance <= 0 && !cfg.ignoreTransientFX {
		return v
	}
	mobjs, ok := root["mobjs"].([]any)
	if !ok {
		return v
	}
	playerX, playerY, havePlayer := playerPosForTrace(mobjs)
	filtered := make([]any, 0, len(mobjs))
	for _, item := range mobjs {
		mobj, ok := item.(map[string]any)
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		if shouldDropMobj(mobj, havePlayer, playerX, playerY, cfg) {
			continue
		}
		filtered = append(filtered, item)
	}
	root["mobjs"] = filtered
	root["mobj_count"] = float64(len(filtered))
	return root
}

func playerPosForTrace(mobjs []any) (float64, float64, bool) {
	for _, item := range mobjs {
		mobj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if numValue(mobj["player"]) == 1 {
			return numValue(mobj["x"]), numValue(mobj["y"]), true
		}
	}
	return 0, 0, false
}

func shouldDropMobj(mobj map[string]any, havePlayer bool, playerX, playerY float64, cfg compareConfig) bool {
	if numValue(mobj["player"]) == 1 {
		return false
	}
	if cfg.ignoreTransientFX && isTransientFXType(int(numValue(mobj["type"]))) {
		return true
	}
	if cfg.maxPlayerDistance > 0 && havePlayer {
		dx := (numValue(mobj["x"]) - playerX) / 65536.0
		dy := (numValue(mobj["y"]) - playerY) / 65536.0
		if math.Hypot(dx, dy) > cfg.maxPlayerDistance {
			return true
		}
	}
	return false
}

func numValue(v any) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	return 0
}

func isTransientFXType(typ int) bool {
	switch typ {
	case 6, 9, 16, 31, 32, 33, 34, 35, 36, 37, 38, 39:
		return true
	default:
		return false
	}
}

func readJSONL(path string) ([][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	lines := make([][]byte, 0, 1024)
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for s.Scan() {
		line := append([]byte(nil), s.Bytes()...)
		lines = append(lines, line)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func filterKind(lines [][]byte, want string) [][]byte {
	out := make([][]byte, 0, len(lines))
	for _, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			continue
		}
		if kind, _ := obj["kind"].(string); kind == want {
			out = append(out, line)
		}
	}
	return out
}

func firstDiff(path string, left, right any) (string, any, any, bool) {
	if shouldIgnorePath(path) {
		return "", nil, nil, false
	}
	switch l := left.(type) {
	case map[string]any:
		r, ok := right.(map[string]any)
		if !ok {
			return path, left, right, true
		}
		keys := unionKeys(l, r)
		for _, k := range keys {
			if shouldIgnoreMapKey(path, k, l, r) {
				continue
			}
			lp, lok := l[k]
			rp, rok := r[k]
			if !lok || !rok {
				childPath := path + "." + k
				if shouldIgnorePath(childPath) {
					continue
				}
				return childPath, lp, rp, true
			}
			if p, lv, rv, ok := firstDiff(path+"."+k, lp, rp); ok {
				return p, lv, rv, true
			}
		}
		return "", nil, nil, false
	case []any:
		r, ok := right.([]any)
		if !ok {
			return path, left, right, true
		}
		if len(l) != len(r) {
			return path + ".len", len(l), len(r), true
		}
		for i := range l {
			if p, lv, rv, ok := firstDiff(fmt.Sprintf("%s[%d]", path, i), l[i], r[i]); ok {
				return p, lv, rv, true
			}
		}
		return "", nil, nil, false
	case float64:
		r, ok := right.(float64)
		if !ok || l != r {
			return path, left, right, true
		}
		return "", nil, nil, false
	case string:
		r, ok := right.(string)
		if !ok || l != r {
			return path, left, right, true
		}
		return "", nil, nil, false
	case bool:
		r, ok := right.(bool)
		if !ok || l != r {
			return path, left, right, true
		}
		return "", nil, nil, false
	case nil:
		if right != nil {
			return path, left, right, true
		}
		return "", nil, nil, false
	default:
		if !reflect.DeepEqual(left, right) {
			return path, left, right, true
		}
		return "", nil, nil, false
	}
}

func shouldIgnorePath(path string) bool {
	if len(path) >= len("root.mobjs[0].") && path[:len("root.mobjs[0].")] == "root.mobjs[0]." {
		switch path[len("root.mobjs[0]."):] {
		case "target", "threshold":
			return true
		}
	}
	ignoredSuffixes := []string{
		".rndindex",
		".prndindex",
		".flags",
		".state",
		".tics",
		".kind",
		".lastlook",
		".radius",
		".height",
		".target_type",
		".tracer_type",
		".texture",
		".action",
		".dropped",
	}
	for _, suffix := range ignoredSuffixes {
		if len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

func shouldIgnoreMapKey(path string, key string, left, right map[string]any) bool {
	if len(path) >= len("root.specials[") && path[:len("root.specials[")] == "root.specials[" {
		if key == "topcountdown" {
			if isDoorSpecial(left) && isDoorSpecial(right) {
				ldir, lok := left["direction"].(float64)
				rdir, rok := right["direction"].(float64)
				if lok && rok && ldir != 0 && ldir != 2 && rdir != 0 && rdir != 2 {
					return true
				}
			}
		}
		if isPlatSpecial(left) && isPlatSpecial(right) {
			if key == "low" {
				ltyp, lok := left["type"].(float64)
				rtyp, rok := right["type"].(float64)
				if lok && rok && (int(ltyp) == 2 || int(ltyp) == 3) && int(ltyp) == int(rtyp) {
					return true
				}
			}
			if key == "count" {
				lstatus, lok := left["status"].(float64)
				rstatus, rok := right["status"].(float64)
				if lok && rok && int(lstatus) != 2 && int(rstatus) != 2 {
					return true
				}
			}
			if key == "oldstatus" {
				lstatus, lok := left["status"].(float64)
				rstatus, rok := right["status"].(float64)
				if lok && rok && int(lstatus) != 16 && int(rstatus) != 16 {
					return true
				}
			}
		}
		if isFloorSpecial(left) && isFloorSpecial(right) {
			if key == "newspecial" {
				ltyp, lok := left["type"].(float64)
				rtyp, rok := right["type"].(float64)
				if lok && rok && int(ltyp) == int(rtyp) && int(ltyp) != 6 && int(ltyp) != 11 {
					return true
				}
			}
		}
	}
	if len(path) >= len("root.mobjs[") && path[:len("root.mobjs[")] == "root.mobjs[" {
		lt, lok := left["type"].(float64)
		rt, rok := right["type"].(float64)
		if lok && rok && lt == rt && (lt == 37 || lt == 38) {
			switch key {
			case "x", "y", "z", "momz":
				return true
			}
		}
	}
	return false
}

func isPlatSpecial(v map[string]any) bool {
	kind, _ := v["kind"].(string)
	return kind == "plat"
}

func isDoorSpecial(v map[string]any) bool {
	kind, ok := v["kind"].(string)
	return ok && kind == "door"
}

func isFloorSpecial(v map[string]any) bool {
	kind, ok := v["kind"].(string)
	return ok && kind == "floor"
}

func unionKeys(left, right map[string]any) []string {
	keys := make([]string, 0, len(left)+len(right))
	seen := make(map[string]struct{}, len(left)+len(right))
	for k := range left {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	for k := range right {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// mobjForPath extracts the mobj objects from both sides when path is inside root.mobjs[N].
func mobjForPath(path string, l, r any) (lm, rm map[string]any, idx int, ok bool) {
	const prefix = "root.mobjs["
	if len(path) <= len(prefix) || path[:len(prefix)] != prefix {
		return nil, nil, 0, false
	}
	rest := path[len(prefix):]
	n := 0
	for n < len(rest) && rest[n] >= '0' && rest[n] <= '9' {
		n++
	}
	if n == 0 {
		return nil, nil, 0, false
	}
	for _, c := range rest[:n] {
		idx = idx*10 + int(c-'0')
	}
	lroot, lok := l.(map[string]any)
	rroot, rok := r.(map[string]any)
	if !lok || !rok {
		return nil, nil, 0, false
	}
	lmobjs, lok := lroot["mobjs"].([]any)
	rmobjs, rok := rroot["mobjs"].([]any)
	if !lok || !rok || idx >= len(lmobjs) || idx >= len(rmobjs) {
		return nil, nil, 0, false
	}
	lm, lok = lmobjs[idx].(map[string]any)
	rm, rok = rmobjs[idx].(map[string]any)
	if !lok || !rok {
		return nil, nil, 0, false
	}
	return lm, rm, idx, true
}

func marshalCompact(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}
