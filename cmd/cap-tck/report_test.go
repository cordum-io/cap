package main

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/cordum-io/cap/v2/internal/tck/publicreport"
)

func putFile(t *testing.T, path string, data []byte) string {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func fileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

func TestOutputUnsafeNamesO1(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "d"), 0o700); err != nil {
		t.Fatal(err)
	}
	names := []string{"../x", "a/../../x", filepath.Join(outside, "x"), "", ".", "d"}
	if runtime.GOOS == "windows" {
		names = append(names, "NUL", "COM1")
	}
	before := dirNames(t, dir)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if err := writeOutput(dir, name, []byte("new")); !errors.Is(err, errUnsafeOutput) {
				t.Fatalf("writeOutput = %v", err)
			}
			if !reflect.DeepEqual(dirNames(t, dir), before) || len(dirNames(t, outside)) != 0 {
				t.Fatal("directory changed")
			}
		})
	}
}

func TestOutputFinalSymlinksO2(t *testing.T) {
	for _, place := range []string{"inside", "outside"} {
		t.Run(place, func(t *testing.T) {
			dir, targetDir := t.TempDir(), t.TempDir()
			if place == "inside" {
				targetDir = dir
			}
			target := putFile(t, filepath.Join(targetDir, "target"), []byte("old"))
			link := filepath.Join(dir, "link")
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("os.Symlink: %v", err)
			}
			before := dirNames(t, dir)
			if err := writeOutput(dir, "link", []byte("new")); !errors.Is(err, errUnsafeOutput) {
				t.Fatalf("writeOutput = %v", err)
			}
			fi, err := os.Lstat(link)
			if err != nil || fi.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("link changed: %v", err)
			}
			if string(fileBytes(t, target)) != "old" || !reflect.DeepEqual(dirNames(t, dir), before) {
				t.Fatal("output changed")
			}
		})
	}
}

func TestOutputParentSymlinkO2(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "esc")); err != nil {
		t.Skipf("os.Symlink: %v", err)
	}
	if err := writeOutput(dir, "esc/x", []byte("new")); !errors.Is(err, errUnsafeOutput) {
		t.Fatalf("writeOutput = %v", err)
	}
	if len(dirNames(t, outside)) != 0 || !reflect.DeepEqual(dirNames(t, dir), []string{"esc"}) {
		t.Fatal("output changed")
	}
}

func TestOutputModeAndReplaceO3(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"new", "existing"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			if name == "existing" {
				putFile(t, path, []byte("old"))
				if err := os.Chmod(path, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := writeOutput(dir, name, []byte("replacement")); err != nil {
				t.Fatal(err)
			}
			if string(fileBytes(t, path)) != "replacement" {
				t.Fatal("wrong output")
			}
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
				t.Fatalf("mode = %o", fi.Mode().Perm())
			}
		})
	}
	if !reflect.DeepEqual(dirNames(t, dir), []string{"existing", "new"}) {
		t.Fatal("temp file left behind")
	}
}

func TestOutputWriteErrorO4(t *testing.T) {
	dir := t.TempDir()
	putFile(t, filepath.Join(dir, "existing"), []byte("old"))
	before := dirNames(t, dir)
	injected := errors.New("injected partial write failure")
	original := writeTemp
	t.Cleanup(func() { writeTemp = original })
	writeTemp = func(w io.Writer, b []byte) error {
		if _, err := w.Write(b[:1]); err != nil {
			return err
		}
		return injected
	}
	for _, name := range []string{"new", "existing"} {
		t.Run(name, func(t *testing.T) {
			if err := writeOutput(dir, name, []byte("new")); !errors.Is(err, injected) {
				t.Fatalf("writeOutput = %v", err)
			}
			if !reflect.DeepEqual(dirNames(t, dir), before) || string(fileBytes(t, filepath.Join(dir, "existing"))) != "old" {
				t.Fatal("partial write changed directory/target")
			}
		})
	}
}

func TestReadBoundedO5(t *testing.T) {
	path := putFile(t, filepath.Join(t.TempDir(), "input"), []byte("1234"))
	b, err := readBounded(path, 4)
	if err != nil || string(b) != "1234" {
		t.Fatalf("readBounded = %q, %v", b, err)
	}
	for _, max := range []int64{3, 0, -1} {
		b, err := readBounded(path, max)
		if !errors.Is(err, errTooLarge) || b != nil {
			t.Fatalf("max %d: %q, %v", max, b, err)
		}
	}
	if b, err := readBounded(path+"-missing", 4); err == nil || b != nil {
		t.Fatal("missing input accepted")
	}
	putFile(t, path, nil)
	if b, err := readBounded(path, 0); err != nil || len(b) != 0 {
		t.Fatalf("empty: %q, %v", b, err)
	}
}

func TestReadPEMO6(t *testing.T) {
	good := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("der")})
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"valid", good, nil},
		{"whitespace", append(bytes.Clone(good), ' ', '\n'), nil},
		{"wrong-type", pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("der")}), errKeyFile},
		{"two-blocks", append(bytes.Clone(good), good...), errKeyFile},
		{"trailing", append(bytes.Clone(good), []byte("junk")...), errKeyFile},
		{"leading-garbage", append([]byte("junk\n"), good...), errKeyFile},
		{"malformed-prefix", append([]byte("-----BEGIN PRIVATE KEY-----\n???\n-----END PRIVATE KEY-----\n"), good...), errKeyFile},
		{"leading-whitespace", append([]byte(" \n"), good...), nil},
		{"headers", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Headers: map[string]string{"X": "Y"}, Bytes: []byte("der")}), errKeyFile},
		{"not-pem", []byte("not a PEM"), errKeyFile},
		{"empty", nil, errKeyFile},
		{"oversize", bytes.Repeat([]byte{'x'}, maxKeyBytes+1), errTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := putFile(t, filepath.Join(t.TempDir(), "key.pem"), tc.data)
			b, err := readPEM(path, "PRIVATE KEY")
			if !errors.Is(err, tc.want) {
				t.Fatalf("readPEM = %q, %v; want %v", b, err, tc.want)
			}
			if tc.want != nil && b != nil || tc.want == nil && string(b) != "der" {
				t.Fatalf("unexpected DER: %q", b)
			}
		})
	}
}

const reportSuite = `{"version":1,"scenarios":[
{"id":"w/pass","role":"worker","profile":"core","required":true,"params":{"status":"PASS"}},
{"id":"cp/skip","role":"control-plane","profile":"core","required":true}]}`

func reportSubject() publicreport.Subject {
	return publicreport.Subject{ArtifactName: "test-worker", ArtifactSha256: strings.Repeat("a", 64), TestedCommit: strings.Repeat("b", 40)}
}

func reportRun(t *testing.T, status string) (run, scenarios string) {
	t.Helper()
	suite := writeSuite(t, strings.Replace(reportSuite, `"status":"PASS"`, `"status":"`+status+`"`, 1))
	run = filepath.Join(t.TempDir(), "run.json")
	var out, errb bytes.Buffer
	code := cmdRun([]string{"--suite", suite, "--adapter", refAdapterFlag(), "--json", run}, &out, &errb)
	want := 0
	if status != "PASS" {
		want = 1
	}
	if code != want {
		t.Fatalf("run exit %d, stderr=%s", code, &errb)
	}
	return run, filepath.Dir(suite)
}

func projectArgs(run, dir string) []string {
	s := reportSubject()
	return []string{"report", "project", "--tck", run, "--scenarios", dir, "--artifact-name", s.ArtifactName, "--artifact-sha256", s.ArtifactSha256, "--commit", s.TestedCommit}
}

func runReportCLI(t *testing.T, want int, args ...string) (string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := dispatch(args, strings.NewReader(""), &out, &errb)
	if code != want {
		t.Fatalf("%v exit %d, want %d; stdout=%s stderr=%s", args, code, want, &out, &errb)
	}
	return out.String(), errb.String()
}

func cliKeyPair(t *testing.T, seed byte) (priv, pub string) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	priv = putFile(t, filepath.Join(dir, "private.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	der, err = x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		t.Fatal(err)
	}
	pub = putFile(t, filepath.Join(dir, "public.pem"), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	return priv, pub
}

func reportGolden(t *testing.T, name string) []byte {
	t.Helper()
	b := fileBytes(t, filepath.Join("../../internal/tck/publicreport/testdata", name))
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}

func TestReportEndToEndC1(t *testing.T) {
	run, scenarios := reportRun(t, "PASS")
	dir := t.TempDir()
	priv, pub := cliKeyPair(t, 1)
	runReportCLI(t, 0, append(projectArgs(run, scenarios), "--out", "report.json", "--out-dir", dir)...)
	report := filepath.Join(dir, "report.json")
	r, err := publicreport.Validate(fileBytes(t, report))
	if err != nil || r.Verdict != publicreport.VerdictPass {
		t.Fatalf("Validate = %+v, %v", r, err)
	}
	runReportCLI(t, 0, "report", "sign", "--report", report, "--key", priv, "--out", "report.sig", "--out-dir", dir)
	sig := filepath.Join(dir, "report.sig")
	if len(fileBytes(t, sig)) != 64 {
		t.Fatal("signature length")
	}
	out, _ := runReportCLI(t, 0, "report", "verify", "--report", report, "--sig", sig, "--pub", pub)
	if out != "verified: verdict PASS, profile core, role worker\n" {
		t.Fatalf("verify stdout=%q", out)
	}
	runReportCLI(t, 0, "report", "badge", "--report", report, "--out", "badge.svg", "--out-dir", dir)
	if !bytes.Equal(fileBytes(t, filepath.Join(dir, "badge.svg")), reportGolden(t, "badge-pass.golden.svg")) {
		t.Fatal("badge differs from golden")
	}
}

func manifestRecipe(t *testing.T, dir string) string {
	t.Helper()
	names := dirNames(t, dir)
	sort.Strings(names)
	var lines strings.Builder
	for _, name := range names {
		if strings.HasSuffix(name, ".json") {
			fmt.Fprintf(&lines, "%x  %s\n", sha256.Sum256(fileBytes(t, filepath.Join(dir, name))), name)
		}
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(lines.String())))
}

func TestManifestDigestAndRoleC2(t *testing.T) {
	run, dir := reportRun(t, "PASS")
	putFile(t, filepath.Join(dir, "a.json"), []byte(`{"version":1,"scenarios":[{"id":"other/case","role":"worker","profile":"other","required":true}]}`))
	b, err := projectReport(run, dir, reportSubject())
	if err != nil {
		t.Fatal(err)
	}
	r, err := publicreport.Validate(b)
	if err != nil || r.ScenarioManifestSha256 != manifestRecipe(t, dir) || r.Verdict != "PASS" {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	if len(r.Cases) != 2 || r.Cases[0].ID != "cp/skip" || r.Cases[0].Required || !r.Cases[1].Required {
		t.Fatalf("role filtering: %+v", r.Cases)
	}
	crlf := t.TempDir()
	for _, name := range dirNames(t, dir) {
		putFile(t, filepath.Join(crlf, name), bytes.ReplaceAll(fileBytes(t, filepath.Join(dir, name)), []byte("\n"), []byte("\r\n")))
	}
	again, err := projectReport(run, crlf, reportSubject())
	if err != nil || !bytes.Equal(b, again) {
		t.Fatalf("CRLF changed report: %v", err)
	}
}

func expectCLIRefusal(t *testing.T, want error, args []string) {
	t.Helper()
	dir := t.TempDir()
	out, errb := runReportCLI(t, 1, append(args, "--out", "result", "--out-dir", dir)...)
	if out != "" || !strings.Contains(errb, want.Error()) || len(dirNames(t, dir)) != 0 {
		t.Fatalf("refusal: stdout=%q stderr=%q files=%v", out, errb, dirNames(t, dir))
	}
}

func TestManifestRefusalsC2(t *testing.T) {
	run, original := reportRun(t, "PASS")
	valid := fileBytes(t, filepath.Join(original, "suite.json"))
	cases := []struct {
		name   string
		change func(string)
		want   error
	}{
		{"missing", func(d string) {
			putFile(t, filepath.Join(d, "suite.json"), []byte(`{"version":1,"scenarios":[{"id":"w/pass","role":"worker","profile":"core","required":true}]}`))
		}, errManifestMismatch},
		{"extra", func(d string) {
			putFile(t, filepath.Join(d, "extra.json"), []byte(`{"version":1,"scenarios":[{"id":"w/extra","role":"worker","profile":"core"}]}`))
		}, errManifestMismatch},
		{"required", func(d string) {
			putFile(t, filepath.Join(d, "extra.json"), []byte(`{"version":1,"scenarios":[{"id":"w/extra","role":"worker","profile":"core","required":true}]}`))
		}, publicreport.ErrMissingRequiredCase},
		{"duplicate", func(d string) { putFile(t, filepath.Join(d, "extra.json"), valid) }, errManifest},
		{"directory", func(d string) {
			if err := os.Mkdir(filepath.Join(d, "x.json"), 0o700); err != nil {
				t.Fatal(err)
			}
		}, errManifest},
		{"dot-name", func(d string) { putFile(t, filepath.Join(d, ".x.json"), valid) }, errManifest},
		{"space-name", func(d string) { putFile(t, filepath.Join(d, "a b.json"), valid) }, errManifest},
		{"unknown", func(d string) {
			putFile(t, filepath.Join(d, "suite.json"), bytes.Replace(valid, []byte(`"version"`), []byte(`"extra":true,"version"`), 1))
		}, errManifest},
		{"empty", func(d string) {
			if err := os.Remove(filepath.Join(d, "suite.json")); err != nil {
				t.Fatal(err)
			}
		}, errManifest},
		{"oversize-suite", func(d string) {
			putFile(t, filepath.Join(d, "suite.json"), bytes.Repeat([]byte{' '}, maxScenarioBytes+1))
		}, errTooLarge},
		{"too-many", func(d string) {
			for i := range maxScenarioFiles {
				putFile(t, filepath.Join(d, fmt.Sprintf("%02d.json", i)), []byte(`{"version":1,"scenarios":[]}`))
			}
		}, errManifest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			putFile(t, filepath.Join(dir, "suite.json"), valid)
			tc.change(dir)
			b, err := projectReport(run, dir, reportSubject())
			if !errors.Is(err, tc.want) || b != nil {
				t.Fatalf("projectReport = %s, %v", b, err)
			}
			expectCLIRefusal(t, tc.want, projectArgs(run, dir))
		})
	}
}

func TestManifestSymlinkC2(t *testing.T) {
	run, original := reportRun(t, "PASS")
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(original, "suite.json"), filepath.Join(dir, "suite.json")); err != nil {
		t.Skipf("os.Symlink: %v", err)
	}
	if _, _, err := loadManifest(dir); !errors.Is(err, errManifest) {
		t.Fatalf("loadManifest = %v", err)
	}
	expectCLIRefusal(t, errManifest, projectArgs(run, dir))
}

func TestProjectOversizeC2(t *testing.T) {
	run := putFile(t, filepath.Join(t.TempDir(), "huge.json"), nil)
	if err := os.Truncate(run, maxTCKBytes+1); err != nil {
		t.Fatal(err)
	}
	if b, err := projectReport(run, t.TempDir(), reportSubject()); !errors.Is(err, errTooLarge) || b != nil {
		t.Fatalf("projectReport = %s, %v", b, err)
	}
	expectCLIRefusal(t, errTooLarge, projectArgs(run, t.TempDir()))
}

func TestProjectNonPassEvidenceC2(t *testing.T) {
	for _, status := range []string{"FAIL", "UNSUPPORTED"} {
		t.Run(status, func(t *testing.T) {
			run, dir := reportRun(t, status)
			out := t.TempDir()
			runReportCLI(t, 0, append(projectArgs(run, dir), "--out", "report", "--out-dir", out)...)
			r, err := publicreport.Validate(fileBytes(t, filepath.Join(out, "report")))
			want := "FAIL"
			if status == "UNSUPPORTED" {
				want = "INCOMPLETE"
			}
			if err != nil || r.Verdict != want {
				t.Fatalf("Validate = %+v, %v", r, err)
			}
		})
	}
}

func TestSignKeyRefusalsC3(t *testing.T) {
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	xKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	eDER, err := x509.MarshalPKCS8PrivateKey(ecdsaKey)
	if err != nil {
		t.Fatal(err)
	}
	xDER, err := x509.MarshalPKCS8PrivateKey(xKey)
	if err != nil {
		t.Fatal(err)
	}
	priv, pub := cliKeyPair(t, 1)
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"ecdsa", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: eDER}), errNotEd25519},
		{"x25519", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: xDER}), errNotEd25519},
		{"encrypted", pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: eDER}), errKeyFile},
		{"public", fileBytes(t, pub), errKeyFile},
		{"trailing", append(fileBytes(t, priv), []byte("junk")...), errKeyFile},
		{"bad-der", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("bad DER")}), errKeyFile},
	}
	report := reportGolden(t, "pass.golden.json")
	path := putFile(t, filepath.Join(t.TempDir(), "report"), report)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := putFile(t, filepath.Join(t.TempDir(), "key.pem"), tc.data)
			if b, err := signReport(report, key); !errors.Is(err, tc.want) || b != nil {
				t.Fatalf("signReport = %x, %v", b, err)
			}
			expectCLIRefusal(t, tc.want, []string{"report", "sign", "--report", path, "--key", key})
		})
	}
}

func TestVerifyRefusalsC3(t *testing.T) {
	priv, pub := cliKeyPair(t, 1)
	_, wrongPub := cliKeyPair(t, 2)
	eKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&eKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	ePub := putFile(t, filepath.Join(t.TempDir(), "pub.pem"), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	b := reportGolden(t, "pass.golden.json")
	sig, err := signReport(b, priv)
	if err != nil {
		t.Fatal(err)
	}
	flipped := bytes.Clone(b)
	flipped[0] ^= 1
	cases := []struct {
		name        string
		report, sig []byte
		pub         string
		want        error
	}{
		{"wrong-key", b, sig, wrongPub, publicreport.ErrSignatureMismatch},
		{"flipped-report", flipped, sig, pub, publicreport.ErrSignatureMismatch},
		{"short-signature", b, sig[:63], pub, publicreport.ErrBadSignature},
		{"ecdsa", b, sig, ePub, errNotEd25519},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			rp := putFile(t, filepath.Join(dir, "report"), tc.report)
			sp := putFile(t, filepath.Join(dir, "sig"), tc.sig)
			r, err := verifyReport(rp, sp, tc.pub)
			if !errors.Is(err, tc.want) || !reflect.DeepEqual(r, publicreport.Report{}) {
				t.Fatalf("verifyReport = %+v, %v", r, err)
			}
			out, errb := runReportCLI(t, 1, "report", "verify", "--report", rp, "--sig", sp, "--pub", tc.pub)
			if out != "" || !strings.Contains(errb, tc.want.Error()) {
				t.Fatalf("stdout=%q stderr=%q", out, errb)
			}
		})
	}
}

func TestBadgeCLIRefusalsC4(t *testing.T) {
	for _, name := range []string{"fail", "incomplete", "invalid"} {
		t.Run(name, func(t *testing.T) {
			b, want := []byte("{"), publicreport.ErrMalformedJSON
			if name != "invalid" {
				b, want = reportGolden(t, name+".golden.json"), publicreport.ErrBadgeVerdict
			}
			path := putFile(t, filepath.Join(t.TempDir(), "report"), b)
			args := []string{"report", "badge", "--report", path}
			expectCLIRefusal(t, want, args)
			dir := t.TempDir()
			putFile(t, filepath.Join(dir, "existing"), []byte("old"))
			runReportCLI(t, 1, append(args, "--out", "existing", "--out-dir", dir)...)
			if string(fileBytes(t, filepath.Join(dir, "existing"))) != "old" || !reflect.DeepEqual(dirNames(t, dir), []string{"existing"}) {
				t.Fatal("refused badge changed target/directory")
			}
		})
	}
}

func TestReportUsageC5(t *testing.T) {
	for _, help := range []string{"--help", "-h", "help"} {
		out, errb := runReportCLI(t, 0, "report", help)
		for _, cmd := range []string{"project", "sign", "verify", "badge"} {
			if !strings.Contains(out, "cap-tck report "+cmd) {
				t.Fatalf("missing %s help", cmd)
			}
		}
		if errb != "" || regexp.MustCompile(`(?i)certified|compliant|official`).MatchString(out) {
			t.Fatal("help output")
		}
	}
	for _, args := range [][]string{{"report"}, {"report", "unknown"}, {"report", "project"}, {"report", "sign"}, {"report", "verify"}, {"report", "badge"}, {"report", "sign", "--bad"}, {"report", "badge", "--report", "x", "--out", "y", "stray"}} {
		runReportCLI(t, 2, args...)
	}
	var mainHelp bytes.Buffer
	usage(&mainHelp)
	if !strings.Contains(mainHelp.String(), "cap-tck report") || regexp.MustCompile(`(?i)certified|compliant|official`).Match(mainHelp.Bytes()) {
		t.Fatal("main help")
	}
}

func TestOutputConcurrentWriters(t *testing.T) {
	dir := t.TempDir()
	results := make(chan error, 8)
	for i := range 8 {
		go func() { results <- writeOutput(dir, "result", bytes.Repeat([]byte{byte('a' + i)}, 8192)) }()
	}
	for range 8 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	b := fileBytes(t, filepath.Join(dir, "result"))
	if len(b) != 8192 || b[0] < 'a' || b[0] > 'h' || !bytes.Equal(b, bytes.Repeat(b[:1], 8192)) {
		t.Fatal("partial/mixed output")
	}
	if !reflect.DeepEqual(dirNames(t, dir), []string{"result"}) {
		t.Fatal("temp file left behind")
	}
}
