package doomruntime

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"

	"github.com/zeebo/blake3"
)

// Keep snapshot application and lazy automap cache rebuilding separate. The
// latter must not be charged to every first-person frame when measuring stalls.
func BenchmarkAuthorityReplicaRenderCaches(b *testing.B) {
	for _, mapName := range []string{"E1M1", "E1M3"} {
		b.Run(mapName, func(b *testing.B) {
			wf, err := wad.Open(findDOOM1WAD(b))
			if err != nil {
				b.Fatal(err)
			}
			m, err := mapdata.LoadMap(wf, mapdata.MapName(mapName))
			if err != nil {
				b.Fatal(err)
			}
			a, err := NewAuthority(m, Options{SkillLevel: 3, NoMonsters: true})
			if err != nil {
				b.Fatal(err)
			}
			if err := a.AddPlayer(1); err != nil {
				b.Fatal(err)
			}
			data, err := a.Snapshot(1)
			if err != nil {
				b.Fatal(err)
			}
			r, err := decodeAuthorityReplica(data)
			if err != nil {
				b.Fatal(err)
			}
			client := newGame(cloneMapForRestart(a.g.restartTemplate), Options{
				Headless: true, SourcePortMode: true, PlayerSlot: 1, SkillLevel: 3, NoMonsters: true,
			})
			if err := validateAuthorityReplica(client, r); err != nil {
				b.Fatal(err)
			}
			jsonPayload, err := json.Marshal(r)
			if err != nil {
				b.Fatal(err)
			}
			jsonData := append(append([]byte(nil), authorityReplicaMagic...), jsonPayload...)
			jsonSum := blake3.Sum256(jsonData)
			jsonData = append(jsonData, jsonSum[:]...)
			b.Logf("snapshot bytes: binary=%d JSON=%d", len(data), len(jsonData))
			b.Run("Encode", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := encodeAuthorityReplica(r); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("EncodeJSON", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := json.Marshal(r); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("Capture", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := a.Snapshot(1); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("DecodeJSON", func(b *testing.B) {
				// Preserve the old strict JSON path as a same-run comparison.
				b.ReportAllocs()
				b.SetBytes(int64(len(jsonData)))
				for b.Loop() {
					end := len(jsonData) - 32
					sum := blake3.Sum256(jsonData[:end])
					if !bytes.Equal(sum[:], jsonData[end:]) {
						b.Fatal("checksum")
					}
					var got authorityReplica
					d := json.NewDecoder(bytes.NewReader(jsonData[len(authorityReplicaMagic):end]))
					d.DisallowUnknownFields()
					if err := d.Decode(&got); err != nil {
						b.Fatal(err)
					}
					if err := d.Decode(new(any)); err != io.EOF {
						b.Fatal(err)
					}
				}
			})

			b.Run("Decode", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(data)))
				for b.Loop() {
					if _, err := decodeAuthorityReplica(data); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("Validate", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if err := validateAuthorityReplica(client, r); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("ValidateRehash", func(b *testing.B) {
				// Reproduce the former per-snapshot static-map serialization in
				// the same binary as the cached measurement above.
				b.ReportAllocs()
				for b.Loop() {
					client.authorityMapHashCache = nil
					if err := validateAuthorityReplica(client, r); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("Apply", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					client.applyValidatedAuthorityReplica(r)
				}
			})
			b.Run("ThingRenderRefsWarm", func(b *testing.B) {
				client.initThingRenderState()
				b.ReportAllocs()
				for b.Loop() {
					client.initThingRenderState()
				}
			})
			b.Run("PlaneCacheRebuild", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					client.sectorPlaneCache, client.sectorPlaneTris = nil, nil
					client.ensureSectorPlaneLevelCacheFresh()
				}
			})
			b.Run("PlaneCacheWarm", func(b *testing.B) {
				client.ensureSectorPlaneLevelCacheFresh()
				b.ReportAllocs()
				for b.Loop() {
					client.ensureSectorPlaneLevelCacheFresh()
				}
			})
			b.Run("ApplyAndRebuildPlanes", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					client.applyValidatedAuthorityReplica(r)
					client.ensureSectorPlaneLevelCacheFresh()
				}
			})
		})
	}
}
