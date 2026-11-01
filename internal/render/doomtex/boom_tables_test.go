package doomtex

import (
	"encoding/binary"
	"path/filepath"
	"reflect"
	"testing"

	"gddoom/internal/wad"
)

func animatedEntry(kind byte, start, end string, speed int32) []byte {
	b := make([]byte, 23)
	b[0] = kind
	copy(b[1:10], end)
	copy(b[10:19], start)
	binary.LittleEndian.PutUint32(b[19:], uint32(speed))
	return b
}
func switchEntry(a, b string, ep uint16) []byte {
	out := make([]byte, 20)
	copy(out, a)
	copy(out[9:], b)
	binary.LittleEndian.PutUint16(out[18:], ep)
	return out
}
func TestBoomAnimatedBoundsAndReplacement(t *testing.T) {
	entry := animatedEntry(1, "A", "C", 3)
	for _, data := range [][]byte{nil, entry[:22], entry, append(animatedEntry(2, "A", "C", 3), 255), append(animatedEntry(1, "A", "C", 0), 255), append(animatedEntry(1, "A", "C", -1), 255)} {
		if _, _, err := parseAnimated(data); err == nil {
			t.Fatalf("accepted malformed table %x", data)
		}
	}
	walls, flats, err := parseAnimated(append(entry, 255))
	if err != nil || len(walls) != 1 || len(flats) != 0 || walls[0].Speed != 3 {
		t.Fatalf("parse: %v %v %v", walls, flats, err)
	}
	walls, flats, err = parseAnimated([]byte{255})
	if err != nil || walls == nil || flats == nil || len(walls) != 0 || len(flats) != 0 {
		t.Fatal("terminator-only table must replace defaults")
	}
}
func TestBoomAnimatedOverlapAndMissingAssets(t *testing.T) {
	names := []string{"UNUSED", "A", "B", "C", "D"}
	defs := []AnimDef{{StartName: "A", EndName: "C", Speed: 2}, {StartName: "B", EndName: "D", Speed: 5}, {StartName: "MISSING", EndName: "NOEND", Speed: 7}}
	seqs, speeds, err := resolveBoomAnims(names, defs)
	if err != nil || !reflect.DeepEqual(seqs["A"], []string{"A", "B", "C"}) || !reflect.DeepEqual(seqs["C"], []string{"C", "D", "B"}) || speeds["A"] != 2 || speeds["C"] != 5 {
		t.Fatalf("overlap: %v %v %v", seqs, speeds, err)
	}
	for _, end := range []string{"A", "MISSING", "UNUSED"} {
		if _, _, err := resolveBoomAnims(names, []AnimDef{{StartName: "A", EndName: end}}); err == nil {
			t.Fatalf("accepted invalid end %s", end)
		}
	}
}
func TestBoomSwitchTableBoundsEpisodesAndOrder(t *testing.T) {
	data := append(switchEntry("A", "B", 1), switchEntry("C", "D", 2)...)
	data = append(data, switchEntry("A", "C", 3)...)
	data = append(data, switchEntry("MISSING", "B", 1)...)
	data = append(data, make([]byte, 20)...)
	names := []string{"A", "B", "C", "D"}
	for ep := 1; ep <= 3; ep++ {
		pairs, err := parseSwitches(data, names, ep)
		if err != nil || len(pairs) != ep || pairs[0] != [2]string{"A", "B"} {
			t.Fatalf("episode %d: %v %v", ep, pairs, err)
		}
	}
	for _, data := range [][]byte{nil, make([]byte, 19), switchEntry("A", "B", 1)} {
		if _, err := parseSwitches(data, names, 3); err == nil {
			t.Fatal("accepted unterminated switches")
		}
	}
	pairs, err := parseSwitches(make([]byte, 20), names, 3)
	if err != nil || pairs == nil || len(pairs) != 0 {
		t.Fatal("empty SWITCHES must disable defaults")
	}
}
func TestBoomTablesFixture(t *testing.T) {
	f, err := wad.Open(filepath.Join("..", "..", "..", "testdata", "boom", "tables.wad"))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"BNULL"}
	for i := 1; i <= 7; i++ {
		for _, c := range []string{"A", "B", "C"} {
			names = append(names, string([]byte{'B', 'W', byte('0' + i)})+c)
		}
	}
	names = append(names, "BOV1", "BOV2", "BOV3", "BOV4", "BAOFF", "BAON", "BBOFF", "BBON", "BCON", "SW1BRCOM", "SW2BRCOM")
	tables, err := LoadBoomTables(f, &Set{textureOrder: names})
	if err != nil {
		t.Fatal(err)
	}
	if tables.WallSpeeds["BW3B"] != 3 || tables.FlatSpeeds["BF7C"] != 8 || len(tables.Switches) != 3 || tables.Switches[2] != [2]string{"BAOFF", "BCON"} {
		t.Fatalf("incorrect fixture resources: %+v", tables)
	}
	if _, ok := tables.Walls["FIREWALA"]; ok {
		t.Fatal("custom ANIMATED appended defaults")
	}
}
