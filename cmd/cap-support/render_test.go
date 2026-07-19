package main

import (
	"strings"
	"testing"

	sdksupport "github.com/cordum-io/cap/v2/tools/sdk-support"
)

func sampleManifest() *sdksupport.Manifest {
	return &sdksupport.Manifest{Version: 1, Entries: []sdksupport.Entry{
		{ID: "node", Kind: sdksupport.KindProtocolSDK, Tier: sdksupport.TierStable, Owner: "@o",
			Gates: []sdksupport.Gate{{ID: "real-nats", Kind: sdksupport.GateTest, Path: "x", Pending: true, BlockedBy: "task-de2ef8fb"}}},
		{ID: "go", Kind: sdksupport.KindProtocolSDK, Tier: sdksupport.TierStable, Owner: "@o"},
		{ID: "rust", Kind: sdksupport.KindProtocolSDK, Tier: sdksupport.TierExperimental, Owner: "@o"},
	}}
}

func TestRenderIsDeterministicAndOrdered(t *testing.T) {
	a := renderTable(sampleManifest())
	b := renderTable(sampleManifest())
	if a != b {
		t.Fatalf("render not deterministic:\n%s\n---\n%s", a, b)
	}
	// stable before experimental, and go before node within a tier
	goIdx := strings.Index(a, "| go |")
	nodeIdx := strings.Index(a, "| node |")
	rustIdx := strings.Index(a, "| rust |")
	if !(goIdx < nodeIdx && nodeIdx < rustIdx) {
		t.Fatalf("rows not ordered by tier then id: go=%d node=%d rust=%d", goIdx, nodeIdx, rustIdx)
	}
}

func TestRenderSurfacesPendingWithBlocker(t *testing.T) {
	out := renderTable(sampleManifest())
	if !strings.Contains(out, "real-nats (task-de2ef8fb)") {
		t.Fatalf("pending gate not surfaced with blocker: %s", out)
	}
	if !strings.Contains(out, "| go | protocol-sdk | stable | @o | — |") {
		t.Fatalf("fully-earned go row should show em dash: %s", out)
	}
}

func TestSpliceReplacesOnlyBetweenMarkers(t *testing.T) {
	doc := "intro\n" + beginMarker + "\nOLD\n" + endMarker + "\noutro\n"
	out, err := spliceTable(doc, "NEW\n")
	if err != nil {
		t.Fatalf("splice: %v", err)
	}
	if !strings.Contains(out, "NEW") || strings.Contains(out, "OLD") {
		t.Fatalf("splice did not replace body: %s", out)
	}
	if !strings.HasPrefix(out, "intro\n") || !strings.HasSuffix(out, "outro\n") {
		t.Fatalf("splice disturbed surrounding text: %s", out)
	}
}

func TestSpliceFailsWithoutMarkers(t *testing.T) {
	if _, err := spliceTable("no markers here", "x"); err == nil {
		t.Fatal("expected error when markers are absent")
	}
}

// currentTable of a freshly spliced doc must equal the block, so check's
// staleness comparison is exact.
func TestCurrentTableRoundTrips(t *testing.T) {
	block := renderTable(sampleManifest())
	doc, err := spliceTable(beginMarker+"\n"+endMarker, block)
	if err != nil {
		t.Fatalf("splice: %v", err)
	}
	got, err := currentTable(doc)
	if err != nil {
		t.Fatalf("currentTable: %v", err)
	}
	if got != block {
		t.Fatalf("round-trip mismatch:\n%q\n!=\n%q", got, block)
	}
}
