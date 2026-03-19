package doomruntime

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

type doomSourceSoundInfo struct {
	doomedNum   int16
	seeSound    string
	attackSound string
	deathSound  string
	activeSound string
}

func TestMonsterSoundTablesMatchDoomSourceInfoC(t *testing.T) {
	repoRoot := findRepoRootOrSkipRuntime(t)
	infoByType := parseDoomSourceMonsterSoundInfo(t, filepath.Join(repoRoot, "doom-source", "linuxdoom-1.10", "info.c"))

	seeCases := []int16{3004, 9, 65, 3001, 3002, 58, 3005, 3003, 69, 7, 68, 16, 71, 84, 64, 66}
	for _, typ := range seeCases {
		info, ok := infoByType[typ]
		if !ok {
			t.Fatalf("missing Doom source sound info for type %d", typ)
		}
		got, fullVolume := monsterSeeSoundEvent(typ)
		gotSet := doomSoundSetForEvent(got)
		if len(gotSet) == 0 {
			t.Fatalf("type=%d see event=%v has no Doom sfx mapping", typ, got)
		}
		if _, ok := gotSet[info.seeSound]; !ok {
			t.Fatalf("type=%d see sound=%q not in runtime set %v", typ, info.seeSound, keys(gotSet))
		}
		wantFullVolume := typ == 7 || typ == 16
		if fullVolume != wantFullVolume {
			t.Fatalf("type=%d fullVolume=%v want=%v", typ, fullVolume, wantFullVolume)
		}
	}

	activeCases := []int16{3004, 9, 65, 3001, 3002, 58, 3005, 3003, 69, 7, 68, 16, 71, 84, 64, 66, 67, 3006}
	for _, typ := range activeCases {
		info, ok := infoByType[typ]
		if !ok {
			t.Fatalf("missing Doom source sound info for type %d", typ)
		}
		got := monsterActiveSoundEvent(typ)
		gotSet := doomSoundSetForEvent(got)
		if len(gotSet) == 0 {
			t.Fatalf("type=%d active event=%v has no Doom sfx mapping", typ, got)
		}
		if _, ok := gotSet[info.activeSound]; !ok {
			t.Fatalf("type=%d active sound=%q not in runtime set %v", typ, info.activeSound, keys(gotSet))
		}
	}

	deathCases := []int16{3004, 9, 65, 3001, 3002, 58, 3005, 3003, 69, 7, 68, 16, 71, 84, 64, 66, 67, 3006}
	for _, typ := range deathCases {
		info, ok := infoByType[typ]
		if !ok {
			t.Fatalf("missing Doom source sound info for type %d", typ)
		}
		got := monsterDeathSoundEvent(typ)
		gotSet := doomSoundSetForEvent(got)
		if len(gotSet) == 0 {
			t.Fatalf("type=%d death event=%v has no Doom sfx mapping", typ, got)
		}
		if _, ok := gotSet[info.deathSound]; !ok {
			t.Fatalf("type=%d death sound=%q not in runtime set %v", typ, info.deathSound, keys(gotSet))
		}
	}

	entryAttackCases := []struct {
		typ  int16
		want string
	}{
		{3002, "sfx_sgtatk"},
		{58, "sfx_sgtatk"},
	}
	for _, tc := range entryAttackCases {
		info, ok := infoByType[tc.typ]
		if !ok {
			t.Fatalf("missing Doom source sound info for type %d", tc.typ)
		}
		got := monsterAttackStateEntrySoundEvent(tc.typ)
		gotSet := doomSoundSetForEvent(got)
		if _, ok := gotSet[info.attackSound]; !ok {
			t.Fatalf("type=%d attack entry sound=%q not in runtime set %v", tc.typ, info.attackSound, keys(gotSet))
		}
	}
}

func findRepoRootOrSkipRuntime(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		cand := filepath.Join(dir, "doom-source", "linuxdoom-1.10", "info.c")
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	t.Skipf("repo root with doom-source not found from %s", wd)
	return ""
}

func parseDoomSourceMonsterSoundInfo(t *testing.T, path string) map[int16]doomSourceSoundInfo {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	blockRE := regexp.MustCompile(`\{\s*//\s*(MT_[A-Z0-9_]+)(?s:(.*?))\n\s*\},`)
	lineRE := regexp.MustCompile(`^\s*([^,]+),\s*//\s*([a-z]+)`)
	out := make(map[int16]doomSourceSoundInfo)
	for _, match := range blockRE.FindAllStringSubmatch(string(data), -1) {
		var info doomSourceSoundInfo
		for _, line := range strings.Split(match[2], "\n") {
			sub := lineRE.FindStringSubmatch(line)
			if len(sub) != 3 {
				continue
			}
			value := strings.TrimSpace(sub[1])
			field := strings.TrimSpace(sub[2])
			switch field {
			case "doomednum":
				n, err := strconv.Atoi(value)
				if err != nil {
					t.Fatalf("parse doomednum %q in %s: %v", value, match[1], err)
				}
				info.doomedNum = int16(n)
			case "seesound":
				info.seeSound = value
			case "attacksound":
				info.attackSound = value
			case "deathsound":
				info.deathSound = value
			case "activesound":
				info.activeSound = value
			}
		}
		if info.doomedNum != 0 {
			out[info.doomedNum] = info
		}
	}
	if len(out) == 0 {
		t.Fatalf("no monster sound info parsed from %s", path)
	}
	return out
}

func doomSoundSetForEvent(ev soundEvent) map[string]struct{} {
	switch ev {
	case soundEventMonsterSeePosit1:
		return set("sfx_posit1", "sfx_posit2", "sfx_posit3")
	case soundEventMonsterSeePosit2:
		return set("sfx_posit1", "sfx_posit2", "sfx_posit3")
	case soundEventMonsterSeePosit3:
		return set("sfx_posit1", "sfx_posit2", "sfx_posit3")
	case soundEventMonsterSeeImp1:
		return set("sfx_bgsit1", "sfx_bgsit2")
	case soundEventMonsterSeeImp2:
		return set("sfx_bgsit1", "sfx_bgsit2")
	case soundEventMonsterSeeDemon:
		return set("sfx_sgtsit")
	case soundEventMonsterSeeCaco:
		return set("sfx_cacsit")
	case soundEventMonsterSeeBaron:
		return set("sfx_brssit")
	case soundEventMonsterSeeKnight:
		return set("sfx_kntsit")
	case soundEventMonsterSeeSpider:
		return set("sfx_spisit")
	case soundEventMonsterSeeArachnotron:
		return set("sfx_bspsit")
	case soundEventMonsterSeeCyber:
		return set("sfx_cybsit")
	case soundEventMonsterSeePainElemental:
		return set("sfx_pesit")
	case soundEventMonsterSeeWolfSS:
		return set("sfx_sssit")
	case soundEventMonsterSeeArchvile:
		return set("sfx_vilsit")
	case soundEventMonsterSeeRevenant:
		return set("sfx_skesit")
	case soundEventMonsterActivePosit:
		return set("sfx_posact")
	case soundEventMonsterActiveImp:
		return set("sfx_bgact")
	case soundEventMonsterActiveDemon:
		return set("sfx_dmact")
	case soundEventMonsterActiveArachnotron:
		return set("sfx_bspact")
	case soundEventMonsterActiveArchvile:
		return set("sfx_vilact")
	case soundEventMonsterActiveRevenant:
		return set("sfx_skeact")
	case soundEventDeathPodth1:
		return set("sfx_podth1", "sfx_podth2", "sfx_podth3")
	case soundEventDeathPodth2:
		return set("sfx_podth1", "sfx_podth2", "sfx_podth3")
	case soundEventDeathPodth3:
		return set("sfx_podth1", "sfx_podth2", "sfx_podth3")
	case soundEventDeathBgdth1:
		return set("sfx_bgdth1", "sfx_bgdth2")
	case soundEventDeathBgdth2:
		return set("sfx_bgdth1", "sfx_bgdth2")
	case soundEventDeathDemon:
		return set("sfx_sgtdth")
	case soundEventDeathCaco:
		return set("sfx_cacdth")
	case soundEventDeathBaron:
		return set("sfx_brsdth")
	case soundEventDeathKnight:
		return set("sfx_kntdth")
	case soundEventDeathCyber:
		return set("sfx_cybdth")
	case soundEventDeathSpider:
		return set("sfx_spidth")
	case soundEventDeathArachnotron:
		return set("sfx_bspdth")
	case soundEventDeathLostSoul:
		return set("sfx_firxpl")
	case soundEventDeathMancubus:
		return set("sfx_mandth")
	case soundEventDeathRevenant:
		return set("sfx_skedth")
	case soundEventDeathPainElemental:
		return set("sfx_pedth")
	case soundEventDeathWolfSS:
		return set("sfx_ssdth")
	case soundEventDeathArchvile:
		return set("sfx_vildth")
	case soundEventMonsterAttackSgt:
		return set("sfx_sgtatk")
	case soundEventMonsterAttackSkull:
		return set("sfx_sklatk")
	default:
		return nil
	}
}

func set(vals ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		out[v] = struct{}{}
	}
	return out
}

func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
