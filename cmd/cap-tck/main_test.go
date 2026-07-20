package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain re-routes the test binary into the reference adapter when launched
// with the __adapter subcommand, so end-to-end tests can spawn a real adapter
// process without a separate binary.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "__adapter" {
		os.Exit(runRefAdapter(os.Args[2:], os.Stdin, os.Stdout))
	}
	os.Exit(m.Run())
}

func TestParseAdapterSpecValid(t *testing.T) {
	spec, err := parseAdapterSpec("go=/bin/adapter|--role|worker")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Name != "go" || len(spec.Argv) != 3 || spec.Argv[0] != "/bin/adapter" || spec.Argv[2] != "worker" {
		t.Fatalf("unexpected spec: %+v", spec)
	}
}

// A Windows path carries a drive-letter colon; the label split must be on '='
// only, never the first colon.
func TestParseAdapterSpecHandlesWindowsPath(t *testing.T) {
	spec, err := parseAdapterSpec(`node=C:\bin\adapter.exe|--role|control-plane`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Argv[0] != `C:\bin\adapter.exe` {
		t.Fatalf("drive-letter path mangled: %+v", spec)
	}
}

func TestParseAdapterSpecRejectsBad(t *testing.T) {
	for _, bad := range []string{"noequals", "=onlyargv", "label="} {
		if _, err := parseAdapterSpec(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestDispatchNoArgsIsUsageError(t *testing.T) {
	var out, errb bytes.Buffer
	if code := dispatch(nil, strings.NewReader(""), &out, &errb); code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestDispatchUnknownCommand(t *testing.T) {
	var out, errb bytes.Buffer
	if code := dispatch([]string{"frobnicate"}, strings.NewReader(""), &out, &errb); code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(errb.String(), "unknown command") {
		t.Fatalf("expected unknown-command message, got %q", errb.String())
	}
}

func writeSuite(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "suite.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}
	return p
}

func refAdapterFlag() string {
	return "ref=" + os.Args[0] + "|__adapter|--role|worker"
}

func TestCmdListPrintsScenarios(t *testing.T) {
	suite := writeSuite(t, `{"version":1,"scenarios":[
      {"id":"w/b","role":"worker","profile":"core","required":true},
      {"id":"w/a","role":"worker","profile":"core"}]}`)
	var out, errb bytes.Buffer
	if code := cmdList([]string{"--suite", suite}, &out, &errb); code != 0 {
		t.Fatalf("list exit %d, stderr=%s", code, errb.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "w/a\t") || !strings.HasPrefix(lines[1], "w/b\t") {
		t.Fatalf("unexpected list output: %q", out.String())
	}
}

func TestRunEndToEndConformant(t *testing.T) {
	suite := writeSuite(t, `{"version":1,"scenarios":[
      {"id":"w/pass","role":"worker","profile":"core","required":true,"params":{"status":"PASS"}},
      {"id":"cp/skip","role":"control-plane","profile":"core","required":true}]}`)
	var out, errb bytes.Buffer
	code := cmdRun([]string{"--suite", suite, "--adapter", refAdapterFlag(), "--json", "-"}, &out, &errb)
	if code != 0 {
		t.Fatalf("expected conformant exit 0, got %d; stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), `"conformant": true`) {
		t.Fatalf("expected conformant:true in JSON: %s", out.String())
	}
	if !strings.Contains(out.String(), "CONFORMANT") {
		t.Fatalf("expected summary line: %s", out.String())
	}
}

func TestRunEndToEndNonConformant(t *testing.T) {
	suite := writeSuite(t, `{"version":1,"scenarios":[
      {"id":"w/fail","role":"worker","profile":"core","required":true,"params":{"status":"FAIL"}}]}`)
	var out, errb bytes.Buffer
	code := cmdRun([]string{"--suite", suite, "--adapter", refAdapterFlag()}, &out, &errb)
	if code != 1 {
		t.Fatalf("expected non-conformant exit 1, got %d; stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "NON-CONFORMANT") {
		t.Fatalf("expected NON-CONFORMANT summary: %s", out.String())
	}
}
