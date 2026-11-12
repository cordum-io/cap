package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cordum-io/cap/v2/internal/tck"
	"github.com/cordum-io/cap/v2/internal/tck/publicreport"
)

// Input bounds: publicreport leaves size limits to its caller.
const (
	maxTCKBytes      = 8 << 20
	maxReportBytes   = 1 << 20
	maxKeyBytes      = 16 << 10
	maxScenarioBytes = 1 << 20
	maxScenarioFiles = 64
)

var (
	errTooLarge         = errors.New("input file is over its size limit")
	errKeyFile          = errors.New("key file is not exactly one PEM block of the expected type")
	errNotEd25519       = errors.New("key is not an Ed25519 key")
	errUnsafeOutput     = errors.New("refusing unsafe output path")
	errManifest         = errors.New("bad scenario directory")
	errManifestMismatch = errors.New("TCK report cases differ from the scenario directory")
)

var scenarioFileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,122}\.json$`)

// writeTemp fills the output temp file. Tests replace it to inject a write error.
var writeTemp = func(w io.Writer, b []byte) error {
	_, err := w.Write(b)
	return err
}

func cmdReport(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		reportUsage(stderr)
		return 2
	}
	switch args[0] {
	case "project":
		return reportProject(args[1:], stdout, stderr)
	case "sign":
		return reportSign(args[1:], stdout, stderr)
	case "verify":
		return reportVerify(args[1:], stdout, stderr)
	case "badge":
		return reportBadge(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		reportUsage(stdout)
		return 0
	}
	fmt.Fprintf(stderr, "cap-tck report: unknown subcommand %q\n", args[0])
	reportUsage(stderr)
	return 2
}

func reportUsage(w io.Writer) {
	fmt.Fprint(w, `cap-tck report — offline public conformance reports

Usage:
  cap-tck report project --tck run.json --scenarios DIR --artifact-name NAME
                         --artifact-sha256 HEX --commit HEX --out FILE [--out-dir DIR]
  cap-tck report sign    --report FILE --key KEY.pem --out FILE [--out-dir DIR]
  cap-tck report verify  --report FILE --sig FILE --pub PUB.pem
  cap-tck report badge   --report FILE --out FILE [--out-dir DIR]

project  turns a 'cap-tck run --json' report into a public report
         (cap-public-report/v1). --scenarios is the directory holding exactly
         the scenario files the run used; it sets the required cases and the
         scenario manifest digest.
sign     writes a detached, raw 64-byte Ed25519 signature over the report
         bytes, using your PKCS#8 PEM private key.
verify   checks that signature with your PKIX PEM public key, then the report.
badge    writes a fixed SVG badge, only for a valid report with verdict PASS.

--out is a relative path inside --out-dir (default "."). Symlinks, directories
and paths that leave --out-dir are refused. Files are written with mode 0600
through a temporary file and a rename.

Everything runs offline: no network, no telemetry. cap-tck never creates,
uploads or keeps keys. A public report is self-hosted evidence of one TCK run,
not an endorsement.

Exit codes: 0 success · 1 refused, invalid or mismatched input · 2 usage error.
`)
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	return flags
}

func outFlags(flags *flag.FlagSet) (out, dir *string) {
	return flags.String("out", "", "output file, a relative path inside --out-dir (required)"),
		flags.String("out-dir", ".", "directory the output is confined to")
}

// parseRequired parses args and fails unless every named flag is non-empty.
func parseRequired(flags *flag.FlagSet, args []string, required ...string) bool {
	if flags.Parse(args) != nil {
		return false
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(flags.Output(), "%s: unexpected argument %q\n", flags.Name(), flags.Arg(0))
		return false
	}
	for _, name := range required {
		if flags.Lookup(name).Value.String() == "" {
			fmt.Fprintf(flags.Output(), "%s: --%s is required\n", flags.Name(), name)
			return false
		}
	}
	return true
}

func reportFail(stderr io.Writer, cmd string, err error) int {
	fmt.Fprintf(stderr, "cap-tck report %s: %v\n", cmd, err)
	return 1
}

func reportProject(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("report project", stderr)
	tckPath := flags.String("tck", "", "TCK JSON report from 'cap-tck run --json' (required)")
	dir := flags.String("scenarios", "", "directory holding exactly the scenario files the run used (required)")
	name := flags.String("artifact-name", "", "tested artifact name (required)")
	digest := flags.String("artifact-sha256", "", "tested artifact SHA-256, 64 lowercase hex (required)")
	commit := flags.String("commit", "", "tested commit, 40 lowercase hex (required)")
	out, outDir := outFlags(flags)
	if !parseRequired(flags, args, "tck", "scenarios", "artifact-name", "artifact-sha256", "commit", "out") {
		return 2
	}
	s := publicreport.Subject{TestedCommit: *commit, ArtifactName: *name, ArtifactSha256: *digest}
	b, err := projectReport(*tckPath, *dir, s)
	if err == nil {
		err = writeOutput(*outDir, *out, b)
	}
	if err != nil {
		return reportFail(stderr, "project", err)
	}
	fmt.Fprintf(stdout, "wrote %s sha256=%x\n", *out, sha256.Sum256(b))
	return 0
}

// projectReport derives the required case ids and the manifest digest from the
// scenario directory, projects the TCK report, and checks that its cases are
// exactly the directory's cases for the report's profile.
func projectReport(tckPath, dir string, s publicreport.Subject) ([]byte, error) {
	src, err := readBounded(tckPath, maxTCKBytes)
	if err != nil {
		return nil, err
	}
	var head tck.Report
	if err := json.Unmarshal(src, &head); err != nil {
		return nil, fmt.Errorf("%w: %v", publicreport.ErrMalformedJSON, err)
	}
	scenarios, manifest, err := loadManifest(dir)
	if err != nil {
		return nil, err
	}
	s.ScenarioManifestSha256 = manifest
	var want []string
	for _, sc := range scenarios {
		if sc.Profile == head.Profile {
			want = append(want, sc.ID)
			if sc.Required && sc.Role == head.Role {
				s.RequiredCaseIDs = append(s.RequiredCaseIDs, sc.ID)
			}
		}
	}
	r, b, err := publicreport.Project(src, s)
	if err != nil {
		return nil, err
	}
	got := make([]string, len(r.Cases))
	for i, c := range r.Cases {
		got[i] = c.ID
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return nil, errManifestMismatch
	}
	return b, nil
}

// loadManifest reads every *.json suite directly in dir, in name order. The
// manifest digest is the SHA-256 of the sha256sum-style lines
// "<sha256 hex>  <name>\n", one per file, each file hashed after CRLF is folded
// to LF so a Windows autocrlf checkout gives the same digest.
func loadManifest(dir string) ([]tck.Scenario, string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", errManifest, err)
	}
	var lines bytes.Buffer
	var all []tck.Scenario
	seen := map[string]bool{}
	files := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		files++
		if files > maxScenarioFiles || !e.Type().IsRegular() || !scenarioFileName.MatchString(e.Name()) {
			return nil, "", fmt.Errorf("%w: %s: not a plain scenario file, or over %d files", errManifest, e.Name(), maxScenarioFiles)
		}
		b, err := readBounded(filepath.Join(dir, e.Name()), maxScenarioBytes)
		if err != nil {
			return nil, "", err
		}
		b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
		suite, err := tck.LoadSuite(b)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %s: %v", errManifest, e.Name(), err)
		}
		for _, sc := range suite.Scenarios {
			if seen[sc.ID] {
				return nil, "", fmt.Errorf("%w: scenario %s appears twice", errManifest, sc.ID)
			}
			seen[sc.ID] = true
		}
		all = append(all, suite.Scenarios...)
		fmt.Fprintf(&lines, "%x  %s\n", sha256.Sum256(b), e.Name())
	}
	if files == 0 {
		return nil, "", fmt.Errorf("%w: no *.json scenario files in %s", errManifest, dir)
	}
	return all, fmt.Sprintf("%x", sha256.Sum256(lines.Bytes())), nil
}

func reportSign(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("report sign", stderr)
	report := flags.String("report", "", "public report file (required)")
	key := flags.String("key", "", "PKCS#8 PEM Ed25519 private key file (required)")
	out, outDir := outFlags(flags)
	if !parseRequired(flags, args, "report", "key", "out") {
		return 2
	}
	b, err := readBounded(*report, maxReportBytes)
	var sig []byte
	if err == nil {
		sig, err = signReport(b, *key)
	}
	if err == nil {
		err = writeOutput(*outDir, *out, sig)
	}
	if err != nil {
		return reportFail(stderr, "sign", err)
	}
	fmt.Fprintf(stdout, "wrote %s\n", *out)
	return 0
}

// signReport clears its DER and Ed25519 key buffers before returning. Copies
// internal to x509/asn1 or the Go runtime cannot be reliably erased here.
func signReport(report []byte, keyPath string) ([]byte, error) {
	der, err := readPEM(keyPath, "PRIVATE KEY")
	if err != nil {
		return nil, err
	}
	defer clear(der)
	k, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errKeyFile, err)
	}
	key, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, errNotEd25519
	}
	defer clear(key)
	return publicreport.Sign(report, key)
}

func reportVerify(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("report verify", stderr)
	report := flags.String("report", "", "public report file (required)")
	sig := flags.String("sig", "", "signature file from 'cap-tck report sign' (required)")
	pub := flags.String("pub", "", "PKIX PEM Ed25519 public key file (required)")
	if !parseRequired(flags, args, "report", "sig", "pub") {
		return 2
	}
	r, err := verifyReport(*report, *sig, *pub)
	if err != nil {
		return reportFail(stderr, "verify", err)
	}
	fmt.Fprintf(stdout, "verified: verdict %s, profile %s, role %s\n", r.Verdict, r.Profile, r.Role)
	return 0
}

func verifyReport(reportPath, sigPath, pubPath string) (publicreport.Report, error) {
	b, err := readBounded(reportPath, maxReportBytes)
	if err != nil {
		return publicreport.Report{}, err
	}
	sig, err := readBounded(sigPath, maxKeyBytes)
	if err != nil {
		return publicreport.Report{}, err
	}
	der, err := readPEM(pubPath, "PUBLIC KEY")
	if err != nil {
		return publicreport.Report{}, err
	}
	defer clear(der)
	k, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return publicreport.Report{}, fmt.Errorf("%w: %v", errKeyFile, err)
	}
	pub, ok := k.(ed25519.PublicKey)
	if !ok {
		return publicreport.Report{}, errNotEd25519
	}
	defer clear(pub)
	return publicreport.Verify(b, sig, pub)
}

func reportBadge(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("report badge", stderr)
	report := flags.String("report", "", "public report file (required)")
	out, outDir := outFlags(flags)
	if !parseRequired(flags, args, "report", "out") {
		return 2
	}
	b, err := readBounded(*report, maxReportBytes)
	var svg []byte
	if err == nil {
		svg, err = publicreport.Badge(b)
	}
	if err == nil {
		err = writeOutput(*outDir, *out, svg)
	}
	if err != nil {
		return reportFail(stderr, "badge", err)
	}
	fmt.Fprintf(stdout, "wrote %s\n", *out)
	return 0
}

// readBounded reads a whole file and refuses one over max bytes. A refused or
// failed read is zeroed, since key files go through here too.
func readBounded(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err == nil && int64(len(b)) > max {
		err = fmt.Errorf("%w: %s is over %d bytes", errTooLarge, path, max)
	}
	if err != nil {
		clear(b)
		return nil, err
	}
	return b, nil
}

// readPEM returns the DER bytes of the one PEM block in a key file. The file
// buffer is zeroed here; the caller zeroes the returned DER.
func readPEM(path, blockType string) ([]byte, error) {
	raw, err := readBounded(path, maxKeyBytes)
	if err != nil {
		return nil, err
	}
	defer clear(raw)
	// pem.Decode skips leading non-PEM text and malformed blocks; refuse both.
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("-----BEGIN "+blockType+"-----")) || bytes.Count(raw, []byte("-----BEGIN ")) != 1 {
		return nil, fmt.Errorf("%w: want one %q block", errKeyFile, blockType)
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != blockType || len(block.Headers) > 0 || len(bytes.TrimSpace(rest)) > 0 {
		if block != nil {
			clear(block.Bytes)
		}
		return nil, fmt.Errorf("%w: want one %q block", errKeyFile, blockType)
	}
	return block.Bytes, nil
}

// writeOutput writes data to name inside dir: a 0600 temp file in the same
// directory, then a rename. Every step goes through an os.Root, so no "..",
// absolute path or symlink can leave dir. An existing name must be a regular
// file. The temp file is removed on every error.
func writeOutput(dir, name string, data []byte) (err error) {
	if !filepath.IsLocal(name) {
		return fmt.Errorf("%w: %q is not a relative path inside --out-dir", errUnsafeOutput, name)
	}
	name = filepath.Clean(name)
	scope, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer scope.Close()
	// Pin the parent so renaming it during a failed write cannot strand a temp.
	root, err := scope.OpenRoot(filepath.Dir(name))
	if err != nil {
		return fmt.Errorf("%w: %v", errUnsafeOutput, err)
	}
	defer root.Close()
	name = filepath.Base(name)
	if fi, lerr := root.Lstat(name); lerr == nil && !fi.Mode().IsRegular() {
		return fmt.Errorf("%w: %s exists and is not a regular file", errUnsafeOutput, name)
	} else if lerr != nil && !errors.Is(lerr, fs.ErrNotExist) {
		return fmt.Errorf("%w: %v", errUnsafeOutput, lerr)
	}
	tmp := filepath.Join(filepath.Dir(name), "."+filepath.Base(name)+"."+rand.Text()+".tmp")
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = root.Remove(tmp)
		}
	}()
	if err = writeTemp(f, data); err == nil {
		err = f.Chmod(0o600)
	}
	if err == nil {
		err = f.Sync()
	}
	if err == nil {
		err = f.Close()
	}
	if err == nil {
		err = root.Rename(tmp, name)
	}
	return err
}
