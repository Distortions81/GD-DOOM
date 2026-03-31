package app

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
)

func findLocalWADOrSkip(t *testing.T, candidates ...string) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		for _, name := range candidates {
			cand := filepath.Join(dir, name)
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				return cand
			}
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	t.Skipf("none of %v found from %s", candidates, wd)
	return ""
}

func TestBuildMonsterSpriteBank_IncludesProjectileAndBossEffectFamilies(t *testing.T) {
	wadPath := findLocalWADOrSkip(t, "DOOM2.WAD", "doom2.wad", "DOOMU.WAD", "doomu.wad")
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open %s: %v", wadPath, err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatalf("load textures from %s: %v", wadPath, err)
	}

	bank := buildMonsterSpriteBank(ts)
	if len(bank) == 0 {
		t.Fatalf("buildMonsterSpriteBank(%s) returned empty bank", wadPath)
	}

	requiredFamilies := []string{
		"MISL",
		"BAL1",
		"BAL2",
		"BAL7",
		"PLSS",
		"PLSE",
		"BFS1",
		"BFE1",
		"FATB",
		"MANF",
		"FBXP",
		"BOSF",
		"FIRE",
		"BBRN",
		"TFOG",
		"PUFF",
		"BLUD",
	}
	for _, family := range requiredFamilies {
		found := ""
		for name, tex := range bank {
			if !strings.HasPrefix(name, family) {
				continue
			}
			if tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
				t.Fatalf("sprite bank entry %s invalid: %dx%d rgba=%d", name, tex.Width, tex.Height, len(tex.RGBA))
			}
			found = name
			break
		}
		if found == "" {
			t.Fatalf("sprite bank missing family %s from %s", family, wadPath)
		}
	}
}

func TestRuntimeSpriteLiteralsMatchDoomSourceAndImportBank(t *testing.T) {
	wadPath := findLocalWADOrSkip(t, "DOOM2.WAD", "doom2.wad", "DOOMU.WAD", "doomu.wad")
	repoRoot := findRepoRootOrSkip(t)
	sourcePrefixes := parseDoomSourceSpritePrefixes(t, filepath.Join(repoRoot, "doom-source/linuxdoom-1.10/info.c"))
	runtimeNames := parseRuntimeSpriteLiterals(t, repoRoot)

	var missingPrefixes []string
	for _, name := range runtimeNames {
		if len(name) < 4 {
			continue
		}
		if _, ok := sourcePrefixes[name[:4]]; !ok {
			missingPrefixes = append(missingPrefixes, name)
		}
	}
	if len(missingPrefixes) > 0 {
		slices.Sort(missingPrefixes)
		t.Fatalf("runtime sprite names missing from Doom source sprnames prefixes: %v", missingPrefixes)
	}

	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open %s: %v", wadPath, err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatalf("load textures from %s: %v", wadPath, err)
	}
	bank := buildMonsterSpriteBank(ts)

	var missingBank []string
	for _, name := range runtimeNames {
		tex, ok := bank[name]
		if !ok || tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
			missingBank = append(missingBank, name)
		}
	}
	if len(missingBank) > 0 {
		slices.Sort(missingBank)
		t.Fatalf("runtime sprite names missing from built sprite bank: %v", missingBank)
	}
}

func TestBuildMonsterSpriteBank_CoversDoomSourceSpecialItemAndCorpseFrames(t *testing.T) {
	wadPath := findLocalWADOrSkip(t, "DOOM2.WAD", "doom2.wad", "DOOMU.WAD", "doomu.wad")
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open %s: %v", wadPath, err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatalf("load textures from %s: %v", wadPath, err)
	}

	bank := buildMonsterSpriteBank(ts)
	required := []string{
		"SOULA0", "SOULB0", "PINVA0", "PINVB0", "PSTRA0", "PINSA0", "PMAPA0", "PVISA0", "MEGAA0",
		"SKULF0", "SSWVE0", "CPOSH0", "CPOSL0",
	}
	for _, name := range required {
		tex, ok := bank[name]
		if !ok || tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
			t.Fatalf("sprite bank missing Doom-source parity frame %s from %s", name, wadPath)
		}
	}
}

func TestBuildMonsterSpriteBank_CoversMonsterDeathFrameFamilies(t *testing.T) {
	wadPath := findLocalWADOrSkip(t, "DOOM2.WAD", "doom2.wad", "DOOMU.WAD", "doomu.wad")
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open %s: %v", wadPath, err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatalf("load textures from %s: %v", wadPath, err)
	}

	bank := buildMonsterSpriteBank(ts)
	cases := []struct {
		family string
		frames string
	}{
		{family: "POSS", frames: "HIJKL"},
		{family: "SPOS", frames: "HIJKL"},
		{family: "SSWV", frames: "IJKLM"},
		{family: "TROO", frames: "IJKLM"},
		{family: "SARG", frames: "IJKLMN"},
		{family: "SKUL", frames: "FGHIJK"},
		{family: "HEAD", frames: "GHIJKL"},
		{family: "BOSS", frames: "IJKLMNO"},
		{family: "BOS2", frames: "IJKLMNO"},
		{family: "VILE", frames: "QRSTU"},
		{family: "CPOS", frames: "HIJKL"},
		{family: "SKEL", frames: "LMNOPQ"},
		{family: "FATT", frames: "KLMNOPQRST"},
		{family: "BSPI", frames: "JKLMNOP"},
		{family: "CYBR", frames: "HIJKLMNOP"},
		{family: "SPID", frames: "JKLMNOPQRS"},
		{family: "PAIN", frames: "HIJKLM"},
	}
	for _, tc := range cases {
		for _, frame := range tc.frames {
			found := false
			prefix := tc.family + string(frame)
			for name, tex := range bank {
				if !strings.HasPrefix(name, prefix) {
					continue
				}
				if tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
					t.Fatalf("sprite bank entry %s invalid for %s", name, wadPath)
				}
				found = true
				break
			}
			if !found {
				t.Fatalf("sprite bank missing monster death frame family %s* from %s", prefix, wadPath)
			}
		}
	}
}

func findRepoRootOrSkip(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		infoPath := filepath.Join(dir, "doom-source", "linuxdoom-1.10", "info.c")
		goModPath := filepath.Join(dir, "go.mod")
		runtimePath := filepath.Join(dir, "internal", "doomruntime", "game.go")
		if info, err := os.Stat(infoPath); err == nil && !info.IsDir() {
			if gomod, err := os.Stat(goModPath); err == nil && !gomod.IsDir() {
				if runtime, err := os.Stat(runtimePath); err == nil && !runtime.IsDir() {
					return dir
				}
			}
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	t.Skipf("repo root not found from %s", wd)
	return ""
}

func parseDoomSourceSpritePrefixes(t *testing.T, path string) map[string]struct{} {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	re := regexp.MustCompile(`"[A-Z0-9]{4}"`)
	out := make(map[string]struct{}, 128)
	for _, match := range re.FindAllString(string(data), -1) {
		out[strings.Trim(match, `"`)] = struct{}{}
	}
	if len(out) == 0 {
		t.Fatalf("no Doom source sprite prefixes found in %s", path)
	}
	return out
}

func parseRuntimeSpriteLiterals(t *testing.T, repoRoot string) []string {
	t.Helper()

	re := regexp.MustCompile(`"[A-Z0-9]{4}[A-Z][0-9]"`)
	blocks := []struct {
		path  string
		start string
		end   string
	}{
		{
			path:  filepath.Join(repoRoot, "internal/doomruntime/game.go"),
			start: "func (g *game) worldThingAnimRefs(typ int16) thingAnimRefState {",
			end:   "func (g *game) initThingRenderState() {",
		},
		{
			path:  filepath.Join(repoRoot, "internal/doomruntime/weapon_psprite.go"),
			start: "var weaponPspriteDefs = map[weaponPspriteState]weaponPspriteDef{",
			end:   "func weaponStateForReady(id weaponID) weaponPspriteState {",
		},
	}
	seen := make(map[string]struct{}, 256)
	for _, block := range blocks {
		data, err := os.ReadFile(block.path)
		if err != nil {
			t.Fatalf("read %s: %v", block.path, err)
		}
		src := string(data)
		start := strings.Index(src, block.start)
		end := strings.Index(src, block.end)
		if start == -1 || end == -1 || end <= start {
			t.Fatalf("failed to isolate sprite block in %s", block.path)
		}
		for _, match := range re.FindAllString(src[start:end], -1) {
			seen[strings.Trim(match, `"`)] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}
