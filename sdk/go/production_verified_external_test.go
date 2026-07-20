package capsdk_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVerifiedProductionPacketCannotBeForgedByExternalPackage(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source")
	}
	moduleRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
	temp := t.TempDir()
	goMod := "module forge.invalid\n\ngo 1.24.0\n\nrequire github.com/cordum-io/cap/v2 v2.0.0\n" +
		"replace github.com/cordum-io/cap/v2 => " + filepath.ToSlash(moduleRoot) + "\n"
	forge := `package forge

import (
	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

var _ = capsdk.VerifiedProductionPacket{Packet: &agentv1.BusPacket{}}
`
	if err := os.WriteFile(filepath.Join(temp, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(temp, "forge.go"), []byte(forge), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-mod=mod", "-run", "^$", ".")
	cmd.Dir = temp
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("external package forged VerifiedProductionPacket:\n%s", output)
	}
	if !strings.Contains(string(output), "unknown field Packet") {
		t.Fatalf("external compilation failed for the wrong reason: %v\n%s", err, output)
	}
}
