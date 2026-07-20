package tck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	sdksupport "github.com/cordum-io/cap/v2/tools/sdk-support"
)

// TestStableSDKsMatchSupportTiers guards the claim in StableSDKs' doc comment:
// it must be exactly the stable-tier protocol SDKs in sdk/support-tiers.json,
// so promoting or demoting an SDK there is caught here instead of silently
// leaving the 3x3 cross-language matrix stale.
func TestStableSDKsMatchSupportTiers(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repoRoot: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "sdk", "support-tiers.json"))
	if err != nil {
		t.Fatalf("read support-tiers.json: %v", err)
	}
	var m sdksupport.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse support-tiers.json: %v", err)
	}

	var stableProtocolSDKs []string
	for _, e := range m.Entries {
		if e.Kind == sdksupport.KindProtocolSDK && e.Tier == sdksupport.TierStable {
			stableProtocolSDKs = append(stableProtocolSDKs, e.ID)
		}
	}
	sort.Strings(stableProtocolSDKs)

	want := append([]string(nil), StableSDKs...)
	sort.Strings(want)

	if len(stableProtocolSDKs) != len(want) {
		t.Fatalf("sdk/support-tiers.json stable protocol SDKs = %v, internal/tck.StableSDKs = %v (update StableSDKs and the crosslang matrix drivers)", stableProtocolSDKs, want)
	}
	for i := range want {
		if stableProtocolSDKs[i] != want[i] {
			t.Fatalf("sdk/support-tiers.json stable protocol SDKs = %v, internal/tck.StableSDKs = %v (update StableSDKs and the crosslang matrix drivers)", stableProtocolSDKs, want)
		}
	}
}
