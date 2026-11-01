package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

const (
	skillEasy      = 0x0001
	skillMedium    = 0x0002
	skillHard      = 0x0004
	skillMask      = skillEasy | skillMedium | skillHard
	thingAmbush    = 0x0008
	thingNotSingle = 0x0010
	thingNotDM     = 0x0020
	thingNotCoop   = 0x0040

	lineBlockMonsters = 0x0002
	lineDontPegTop    = 0x0008
	lineDontPegBottom = 0x0010
	lineSoundBlock    = 0x0040
)

type issue struct {
	mapName string
	index   int
	detail  string
}

type wadAudit struct {
	maps int

	unknownLineSpecials map[uint16][]issue
	unknownSectorSpecs  map[int16][]issue

	thingNoSkillBits []issue
	thingUnknownBits []issue
}

func newAudit() *wadAudit {
	return &wadAudit{
		unknownLineSpecials: make(map[uint16][]issue),
		unknownSectorSpecs:  make(map[int16][]issue),
	}
}

func isPlayerStart(t int16) bool {
	return t >= 1 && t <= 4
}

func appendIssue(dst []issue, mapName string, index int, detail string) []issue {
	return append(dst, issue{mapName: mapName, index: index, detail: detail})
}

func formatIssues(list []issue) string {
	if len(list) == 0 {
		return "none"
	}
	sample := list
	if len(sample) > 12 {
		sample = sample[:12]
	}
	parts := make([]string, 0, len(sample))
	for _, it := range sample {
		if it.detail != "" {
			parts = append(parts, fmt.Sprintf("%s#%d (%s)", it.mapName, it.index, it.detail))
		} else {
			parts = append(parts, fmt.Sprintf("%s#%d", it.mapName, it.index))
		}
	}
	return strings.Join(parts, ", ")
}

func auditWAD(path string) (*wadAudit, error) {
	wf, err := wad.Open(path)
	if err != nil {
		return nil, err
	}
	out := newAudit()
	for _, name := range mapdata.AvailableMapNames(wf) {
		m, err := mapdata.LoadMap(wf, name)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", name, err)
		}
		out.maps++
		mapName := string(name)
		for i, ld := range m.Linedefs {
			info := mapdata.LookupLineSpecial(ld.Special)
			if mapdata.IsScrollingWallSpecial(ld.Special) || mapdata.IsTransferSpecial(ld.Special) {
				continue
			}
			if ld.Special != 0 && info.Trigger == mapdata.TriggerUnknown {
				out.unknownLineSpecials[ld.Special] = appendIssue(out.unknownLineSpecials[ld.Special], mapName, i, "")
			}
		}
		for i, sec := range m.Sectors {
			if !mapdata.SectorSpecialSupported(sec.Special) {
				out.unknownSectorSpecs[sec.Special] = appendIssue(out.unknownSectorSpecs[sec.Special], mapName, i, "")
			}
		}
		for i, th := range m.Things {
			flags := int(th.Flags)
			if !isPlayerStart(th.Type) && (flags&skillMask) == 0 {
				out.thingNoSkillBits = appendIssue(out.thingNoSkillBits, mapName, i, fmt.Sprintf("type=%d", th.Type))
			}
			extra := flags & ^(skillMask | thingAmbush | thingNotSingle | thingNotDM | thingNotCoop)
			if extra != 0 {
				out.thingUnknownBits = appendIssue(out.thingUnknownBits, mapName, i, fmt.Sprintf("type=%d flags=0x%04x", th.Type, extra))
			}
		}
	}
	return out, nil
}

func writeSpecialTable[K ~int16 | ~uint16](b *strings.Builder, title string, m map[K][]issue) {
	b.WriteString("### ")
	b.WriteString(title)
	b.WriteString("\n\n")
	if len(m) == 0 {
		b.WriteString("None.\n\n")
		return
	}
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	b.WriteString("| Special | Count | Examples |\n")
	b.WriteString("| --- | ---: | --- |\n")
	for _, key := range keys {
		list := m[K(key)]
		b.WriteString(fmt.Sprintf("| `%d` | %d | %s |\n", key, len(list), formatIssues(list)))
	}
	b.WriteString("\n")
}

func writeCountRow(b *strings.Builder, label string, list []issue, note string) {
	b.WriteString(fmt.Sprintf("| %s | %d | %s | %s |\n", label, len(list), note, formatIssues(list)))
}

func flattenMap[K comparable](m map[K][]issue) []issue {
	out := make([]issue, 0)
	for _, list := range m {
		out = append(out, list...)
	}
	return out
}

func renderDoc(audits map[string]*wadAudit) string {
	var b strings.Builder
	b.WriteString("# Map Audit\n\n")
	b.WriteString("Map-data audit against the implemented Doom and initial Boom subset.\n\n")
	b.WriteString("Unknown specials can indicate unsupported features or malformed data. An empty report does not establish full map compatibility; resource extensions, node formats, and interactions require separate checks.\n\n")
	b.WriteString("Notes:\n")
	b.WriteString("- In vanilla Doom, a non-player thing with no skill bits set does not spawn. This is not harmless data.\n")
	b.WriteString("- Unknown thing flag bits means map data outside the normal Doom thing-option mask.\n\n")

	wadNames := make([]string, 0, len(audits))
	for name := range audits {
		wadNames = append(wadNames, name)
	}
	sort.Strings(wadNames)
	for _, name := range wadNames {
		a := audits[name]
		b.WriteString("## ")
		b.WriteString(name)
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Maps scanned: %d\n\n", a.maps))
		b.WriteString("### Summary\n\n")
		b.WriteString("| Category | Count | Why it matters | Examples |\n")
		b.WriteString("| --- | ---: | --- | --- |\n")
		writeCountRow(&b, "Unknown linedef specials", flattenMap(a.unknownLineSpecials), "Malformed data or unsupported/non-Doom specials.")
		writeCountRow(&b, "Unknown sector specials", flattenMap(a.unknownSectorSpecs), "Sector behavior not recognized by current runtime.")
		writeCountRow(&b, "Things with no skill bits", a.thingNoSkillBits, "Vanilla Doom will not spawn these non-player things. This is map data, not a missing feature.")
		writeCountRow(&b, "Things with unknown flag bits", a.thingUnknownBits, "Flag bits outside the current Doom thing mask.")
		b.WriteString("\n")
		writeSpecialTable(&b, "Unknown Linedef Specials", a.unknownLineSpecials)
		writeSpecialTable(&b, "Unknown Sector Specials", a.unknownSectorSpecs)
	}
	return b.String()
}

func main() {
	inputs := os.Args[1:]
	if len(inputs) == 0 {
		inputs = []string{"doom.wad", "doom2.wad"}
	}
	audits := make(map[string]*wadAudit)
	for _, path := range inputs {
		if _, err := os.Stat(path); err != nil {
			if len(os.Args) > 1 {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			continue
		}
		audit, err := auditWAD(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		audits[path] = audit
	}
	if len(audits) == 0 {
		fmt.Fprintln(os.Stderr, "no local IWADs found; usage: go run ./cmd/mapaudit WAD [WAD ...]")
		os.Exit(1)
	}
	fmt.Print(renderDoc(audits))
}
