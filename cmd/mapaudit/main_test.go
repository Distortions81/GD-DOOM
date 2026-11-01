package main

import (
	"strings"
	"testing"
)

func TestBoomSmokeMapAudit(t *testing.T) {
	a, err := auditWAD("../../testdata/boom/simple.wad")
	if err != nil {
		t.Fatal(err)
	}
	if a.maps != 1 || len(a.unknownLineSpecials) != 0 || len(a.unknownSectorSpecs) != 0 {
		t.Fatalf("supported Boom map audit: %+v", a)
	}
}

func TestUnknownSpecialReportDoesNotClaimParity(t *testing.T) {
	a := newAudit()
	a.unknownLineSpecials[242] = []issue{{mapName: "MAP01", index: 3}}
	a.unknownSectorSpecs[256] = []issue{{mapName: "MAP01", index: 0}}
	doc := renderDoc(map[string]*wadAudit{"example.wad": a})
	if !strings.Contains(doc, "unsupported features") || !strings.Contains(doc, "`242`") || !strings.Contains(doc, "`256`") || strings.Contains(doc, "parity gaps from this audit: none") {
		t.Fatalf("unsupported Boom features misreported: %s", doc)
	}
}
