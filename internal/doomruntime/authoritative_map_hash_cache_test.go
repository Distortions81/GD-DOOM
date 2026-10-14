package doomruntime

import (
	"encoding/json"
	"strings"
	"testing"

	"gddoom/internal/mapdata"
	"github.com/zeebo/blake3"
)

func TestAuthorityMapHashCachesOnlyImmutableRestartCopy(t *testing.T) {
	live := &mapdata.Map{
		Name: "E1M1", Vertexes: []mapdata.Vertex{{X: 32, Y: 64}},
		Sectors: []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
	}
	g := &game{m: live, restartTemplate: cloneMapForRestart(live)}
	encoded, err := json.Marshal(g.restartTemplate)
	if err != nil {
		t.Fatal(err)
	}
	want := blake3.Sum256(encoded)
	first, err := authorityMapHash(g)
	if err != nil || first != want {
		t.Fatalf("initial fingerprint=%x err=%v want=%x", first, err, want)
	}
	cached := g.authorityMapHashCache
	if cached == nil || cached.template != g.restartTemplate {
		t.Fatal("immutable map fingerprint was not cached by template identity")
	}
	// A moving sector changes the live map but neither the restart copy nor
	// the static fingerprint used to establish map compatibility.
	live.Sectors[0].FloorHeight = 24
	live.Vertexes[0].X = 96
	again, err := authorityMapHash(g)
	if err != nil || again != want || g.authorityMapHashCache != cached {
		t.Fatal("live world changes rebuilt or changed the immutable map fingerprint")
	}
}

func TestAuthorityMapHashReplacementInvalidatesCachedTemplate(t *testing.T) {
	firstMap := &mapdata.Map{Name: "E1M1", Vertexes: []mapdata.Vertex{{X: 32}}}
	g := &game{m: cloneMapForRestart(firstMap), restartTemplate: firstMap}
	first, err := authorityMapHash(g)
	if err != nil {
		t.Fatal(err)
	}
	oldCache := g.authorityMapHashCache
	next := cloneMapForRestart(firstMap)
	next.Vertexes[0].X++
	g.restartTemplate = next
	second, err := authorityMapHash(g)
	if err != nil || first == second || g.authorityMapHashCache == oldCache || g.authorityMapHashCache.template != next {
		t.Fatal("replacement map reused a fingerprint from the preceding template")
	}
	// The same game object can also move back to a previously used template.
	g.restartTemplate = firstMap
	again, err := authorityMapHash(g)
	if err != nil || again != first {
		t.Fatal("restored template did not recover its own fingerprint")
	}
}

func TestAuthorityMapHashMutableFallbackAlwaysRehashes(t *testing.T) {
	live := &mapdata.Map{Name: "E1M1", Vertexes: []mapdata.Vertex{{X: 32}}}
	g := &game{m: live, restartTemplate: cloneMapForRestart(live)}
	if _, err := authorityMapHash(g); err != nil {
		t.Fatal(err)
	}
	g.restartTemplate = nil
	first, err := authorityMapHash(g)
	if err != nil || g.authorityMapHashCache != nil {
		t.Fatal("mutable fallback retained the previous template cache")
	}
	live.Vertexes[0].X++
	second, err := authorityMapHash(g)
	if err != nil || second == first || g.authorityMapHashCache != nil {
		t.Fatal("mutable fallback used a stale map fingerprint")
	}
}

func TestAuthorityCachedMapHashPreservesSnapshotCompatibilityChecks(t *testing.T) {
	_, client, data := snapshotFixture(t)
	r, err := decodeAuthorityReplica(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAuthorityReplica(client, r); err != nil {
		t.Fatal(err)
	}
	if client.authorityMapHashCache == nil {
		t.Fatal("validation did not warm the immutable map fingerprint")
	}
	wrongWAD := r
	wrongWAD.WADHash += "different-wad"
	if err := validateAuthorityReplica(client, wrongWAD); err == nil {
		t.Fatal("warm cache accepted a different WAD")
	}
	wrongHash := r
	wrongHash.MapHash[0] ^= 1
	if err := validateAuthorityReplica(client, wrongHash); err == nil {
		t.Fatal("warm cache accepted a different static map fingerprint")
	}
	replacement := cloneMapForRestart(client.restartTemplate)
	replacement.Vertexes[0].X++
	client.restartTemplate = replacement
	if err := validateAuthorityReplica(client, r); err == nil || !strings.Contains(err.Error(), "static map mismatch") {
		t.Fatalf("old snapshot accepted after replacing map geometry: %v", err)
	}
}
