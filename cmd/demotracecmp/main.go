package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"sort"
)

func main() {
	fs := flag.NewFlagSet("demotracecmp", flag.ExitOnError)
	leftPath := fs.String("left", "", "left trace JSONL")
	rightPath := fs.String("right", "", "right trace JSONL")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *leftPath == "" || *rightPath == "" {
		fmt.Fprintln(os.Stderr, "usage: demotracecmp -left <trace.jsonl> -right <trace.jsonl>")
		os.Exit(2)
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
		if path, lv, rv, ok := firstDiff("root", l, r); ok {
			fmt.Printf("mismatch line=%d path=%s\n", i+1, path)
			fmt.Printf("left=%s\n", marshalCompact(lv))
			fmt.Printf("right=%s\n", marshalCompact(rv))
			os.Exit(1)
		}
	}

	if len(left) != len(right) {
		fmt.Printf("length mismatch left=%d right=%d\n", len(left), len(right))
		os.Exit(1)
	}
	fmt.Printf("traces match lines=%d\n", len(left))
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
	}
	for _, suffix := range ignoredSuffixes {
		if len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

func shouldIgnoreMapKey(path string, key string, left, right map[string]any) bool {
	if len(path) >= len("root.mobjs[") && path[:len("root.mobjs[")] == "root.mobjs[" {
		lt, lok := left["type"].(float64)
		rt, rok := right["type"].(float64)
		if lok && rok && lt == rt && (lt == 37 || lt == 38) {
			switch key {
			case "z", "momz":
				return true
			}
		}
	}
	return false
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

func marshalCompact(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}
