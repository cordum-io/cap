package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gov "github.com/cordum-io/cap/v2/internal/governance"
	"github.com/cordum-io/cap/v2/internal/tck/publicreport"
)

func cliReports(t *testing.T) [][]byte {
	t.Helper()
	b, err := os.ReadFile("../../internal/governance/testdata/attestations/pass-report.json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := publicreport.Validate(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")))
	if err != nil {
		t.Fatal(err)
	}
	var reports [][]byte
	for index, name := range []string{"acme-cap-worker", "beta-cap-worker"} {
		r.Implementation.ArtifactName = name
		r.TestedCommit = strings.Repeat(fmt.Sprint(index), 40)
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
		reports = append(reports, buf.Bytes())
	}
	return reports
}

func writeCLIFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func cliManifest(t *testing.T, root, evidence string) [][]byte {
	t.Helper()
	m := gov.Manifest{SchemaVersion: gov.SchemaVersion}
	reports := cliReports(t)
	for index, b := range reports {
		d := fmt.Sprintf("%x", sha256.Sum256(b))
		writeCLIFile(t, filepath.Join(evidence, d+".json"), b)
		m.Implementations = append(m.Implementations, gov.Implementation{
			ID: fmt.Sprint(index), URL: "https://example.test/source",
			Attestation: &gov.TCKAttestation{Profile: "core", Version: "1", TCKVersion: "cap-tck/1", ReportDigest: d},
		})
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	writeCLIFile(t, filepath.Join(root, manifestRel), b)
	return reports
}

func captureCommand(t *testing.T, command func([]string) (int, error), args ...string) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = saved; r.Close(); w.Close() })
	// Drain concurrently so a command cannot fill the pipe and deadlock.
	type output struct {
		b   []byte
		err error
	}
	done := make(chan output, 1)
	go func() { b, err := io.ReadAll(r); done <- output{b, err} }()
	code, runErr := command(args)
	closeErr := w.Close()
	os.Stdout = saved
	out := <-done
	if runErr != nil || code != 0 || closeErr != nil || out.err != nil {
		t.Fatalf("command: code=%d, err=%v, close=%v, read=%v", code, runErr, closeErr, out.err)
	}
	return string(out.b)
}

func TestReadinessEvidenceDirFlag(t *testing.T) {
	root, evidence := t.TempDir(), t.TempDir()
	reports := cliManifest(t, root, evidence)
	check := func(status, counts string, args ...string) {
		t.Helper()
		out := captureCommand(t, runReadiness, args...)
		if !strings.Contains(out, "readiness verdict: BLOCKED") {
			t.Fatal(out)
		}
		want := fmt.Sprintf("%-34s %-8s %s", gov.DimImplementations, status, counts)
		if !strings.Contains(out, want) {
			t.Fatalf("want %q in %q", want, out)
		}
	}
	check("PASS", "2/2", "--root", root, "--evidence-dir", evidence)
	check("UNKNOWN", "0/2", "--root", root)
	check("UNKNOWN", "0/2", "--root", root, "--evidence-dir", filepath.Join(root, "absent"))
	for _, b := range reports {
		writeCLIFile(t, filepath.Join(root, "governance", "evidence", fmt.Sprintf("%x.json", sha256.Sum256(b))), b)
	}
	check("PASS", "2/2", "--root", root)
	captureCommand(t, runRender, "--root", root, "--evidence-dir", evidence)
	b, err := os.ReadFile(filepath.Join(root, readinessDocRel))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "| two-independent-implementations | **PASS** | 2 / 2 |") {
		t.Fatal(string(b))
	}
	captureCommand(t, runRender, "--root", root, "--evidence-dir", evidence, "--check")
	captureCommand(t, runRender, "--root", root, "--check")
}
