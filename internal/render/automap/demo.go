package automap

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const demoHeaderV1 = "gddoom-demo-v1"

func LoadDemoScript(path string) (*DemoScript, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read demo %s: %w", path, err)
	}
	script, err := ParseDemoScript(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse demo %s: %w", path, err)
	}
	script.Path = path
	return script, nil
}

func ParseDemoScript(text string) (*DemoScript, error) {
	sc := bufio.NewScanner(strings.NewReader(text))
	lineNo := 0
	headerSeen := false
	tics := make([]DemoTic, 0, 1024)
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !headerSeen {
			if !strings.EqualFold(line, demoHeaderV1) {
				return nil, fmt.Errorf("line %d: missing header %q", lineNo, demoHeaderV1)
			}
			headerSeen = true
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 7 {
			return nil, fmt.Errorf("line %d: expected 7 fields, got %d", lineNo, len(fields))
		}
		forward, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: parse forward: %w", lineNo, err)
		}
		side, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: parse side: %w", lineNo, err)
		}
		turn, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("line %d: parse turn: %w", lineNo, err)
		}
		turnRaw, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: parse turn_raw: %w", lineNo, err)
		}
		run, err := parseDemoBit(fields[4])
		if err != nil {
			return nil, fmt.Errorf("line %d: parse run: %w", lineNo, err)
		}
		use, err := parseDemoBit(fields[5])
		if err != nil {
			return nil, fmt.Errorf("line %d: parse use: %w", lineNo, err)
		}
		fire, err := parseDemoBit(fields[6])
		if err != nil {
			return nil, fmt.Errorf("line %d: parse fire: %w", lineNo, err)
		}
		tics = append(tics, DemoTic{
			Forward: forward,
			Side:    side,
			Turn:    turn,
			TurnRaw: turnRaw,
			Run:     run,
			Use:     use,
			Fire:    fire,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if !headerSeen {
		return nil, fmt.Errorf("missing header %q", demoHeaderV1)
	}
	if len(tics) == 0 {
		return nil, fmt.Errorf("demo has no tics")
	}
	return &DemoScript{Tics: tics}, nil
}

func FormatDemoScript(tics []DemoTic) string {
	var b strings.Builder
	b.WriteString(demoHeaderV1)
	b.WriteByte('\n')
	for _, tc := range tics {
		b.WriteString(strconv.FormatInt(tc.Forward, 10))
		b.WriteByte(' ')
		b.WriteString(strconv.FormatInt(tc.Side, 10))
		b.WriteByte(' ')
		b.WriteString(strconv.Itoa(tc.Turn))
		b.WriteByte(' ')
		b.WriteString(strconv.FormatInt(tc.TurnRaw, 10))
		b.WriteByte(' ')
		b.WriteString(formatDemoBit(tc.Run))
		b.WriteByte(' ')
		b.WriteString(formatDemoBit(tc.Use))
		b.WriteByte(' ')
		b.WriteString(formatDemoBit(tc.Fire))
		b.WriteByte('\n')
	}
	return b.String()
}

func SaveDemoScript(path string, tics []DemoTic) error {
	data := FormatDemoScript(tics)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		return fmt.Errorf("write demo %s: %w", path, err)
	}
	return nil
}

func parseDemoBit(v string) (bool, error) {
	switch strings.TrimSpace(v) {
	case "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("expected 0 or 1, got %q", v)
	}
}

func formatDemoBit(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
