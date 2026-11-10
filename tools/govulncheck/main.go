// Command govulncheck-gate runs the pinned govulncheck scanner as a fail-closed
// gate. Before it scans the requested repository it proves the scanner still
// detects a known reachable vulnerability: it builds
// golang.org/x/vuln/cmd/govulncheck from this module with -mod=readonly, scans
// a temporary Go 1.25.12 program that calls net/url.(*URL).Parse, and requires
// exit 3 with GO-2026-6218, net/url@go1.25.12 and a reachable example trace.
// Only then does it scan the repository, on the toolchain named by the
// repository's own go.mod.
//
// Run it from the CAP root as:
//
//	go -C tools/govulncheck run . --repo-root ../..
//
// Exit codes of the gate binary: 0 means the seed was refused and the
// repository is clean; 1 means any other failure; 2 means an invocation error;
// 3 means the repository has reachable vulnerabilities. Under `go run` every
// nonzero gate exit becomes go's own exit 1 with "exit status N" on stderr, so
// CI sees only zero or nonzero.
//
// On success stdout carries exactly these four lines:
//
//	govulncheck-gate: scanner golang.org/x/vuln@<pin> built with -mod=readonly (toolchain <tool toolchain>)
//	govulncheck-gate: seed refused: GO-2026-6218 net/url@go1.25.12 reachable: capseed.main calls url.URL.Parse
//	govulncheck-gate: repository clean under <repo toolchain>: <abs repo root>
//	govulncheck-gate: PASS
//
// Failures go to stderr as "govulncheck-gate: FAIL <stage>: <reason>", followed
// by at most 64 KiB of each captured child stream (1 MiB is captured).
//
// Every child runs with GOTOOLCHAIN set to an exact toolchain,
// GOFLAGS=-mod=readonly, GOWORK=off and GOTMPDIR inside a unique temporary
// root. Inherited GOVERSION, GOROOT, GOFLAGS, GOTOOLCHAIN, GOWORK and GOTMPDIR
// are dropped (case-insensitively), because govulncheck v1.8.0 reads GOVERSION
// before `go env GOVERSION`.
//
// Cleanup scope: the direct children (go build and govulncheck) are killed on
// timeout or interrupt and always reaped, and WaitDelay bounds the wait for
// pipes they leave open. Their descendants (go list, compile, link) are not
// tracked or killed; their temporary files land under GOTMPDIR inside the
// temporary root, which is removed on every exit path.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"
)

const usageText = `usage: govulncheck-gate --repo-root DIR [--timeout DURATION]

Run from the CAP root as:
  go -C tools/govulncheck run . --repo-root ../..

Builds the pinned govulncheck with -mod=readonly, proves it reports the seeded
GO-2026-6218 (net/url@go1.25.12), then scans DIR on the toolchain named by
DIR/go.mod.

  --repo-root DIR       module root to scan (required, once; must hold go.mod)
  --timeout DURATION    limit for all child processes together (default 30m)

Exit codes of the gate binary: 0 seed refused and repository clean; 1 any other
failure; 2 invocation error; 3 repository has reachable vulnerabilities. Under
go run any nonzero exit becomes exit 1 plus "exit status N" on stderr.

Stdout on success:
  govulncheck-gate: scanner golang.org/x/vuln@<pin> built with -mod=readonly (toolchain <tool toolchain>)
  govulncheck-gate: seed refused: GO-2026-6218 net/url@go1.25.12 reachable: capseed.main calls url.URL.Parse
  govulncheck-gate: repository clean under <repo toolchain>: <abs repo root>
  govulncheck-gate: PASS

Cleanup: direct children are killed on timeout or interrupt and always reaped.
Their descendants are not tracked; their temporary files live under GOTMPDIR in
the gate's temporary root, which is always removed.
`

const (
	toolModulePath = "github.com/cordum-io/cap/v2/tools/govulncheck"
	vulnModulePath = "golang.org/x/vuln"
	captureLimit   = 1 << 20  // bytes kept per child stream
	echoLimit      = 64 << 10 // bytes echoed per child stream on failure
)

var (
	toolchainRE   = regexp.MustCompile(`^go1\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	releaseRE     = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	overriddenEnv = []string{"GOVERSION", "GOROOT", "GOFLAGS", "GOTOOLCHAIN", "GOWORK", "GOTMPDIR"}
)

type options struct {
	goBin, toolDir, tempParent string
	waitDelay                  time.Duration
}

type config struct {
	repoRoot string
	timeout  time.Duration
}

type gate struct {
	opts           options
	env            []string
	stdout, stderr io.Writer
	timeout        time.Duration
	repoRoot       string
	toolToolchain  string
	repoToolchain  string
	pin            string
	gotmp          string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := 1
	if goBin, err := exec.LookPath("go"); err != nil {
		fmt.Fprintf(os.Stderr, "govulncheck-gate: FAIL setup: %v\n", err)
	} else {
		code = run(ctx, os.Args[1:], os.Environ(), os.Stdout, os.Stderr,
			options{goBin: goBin, toolDir: ".", waitDelay: 10 * time.Second})
	}
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args, env []string, stdout, stderr io.Writer, opts options) (code int) {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "govulncheck-gate: FAIL usage: %v\n\n%s", err, usageText)
		return 2
	}
	g := &gate{opts: opts, env: env, stdout: stdout, stderr: stderr, timeout: cfg.timeout}
	if err := g.setup(cfg.repoRoot); err != nil {
		return g.fail("setup", err.Error(), nil)
	}
	root, err := os.MkdirTemp(opts.tempParent, "cap-govulncheck-gate-")
	if err != nil {
		return g.fail("setup", err.Error(), nil)
	}
	// Children are reaped synchronously by runChild, so this runs after all
	// of them have exited, on every return path.
	defer func() {
		if err := removeAll(root); err != nil {
			g.report("cleanup", err.Error(), nil)
			if code == 0 {
				code = 1
			}
		}
	}()
	if root, err = filepath.Abs(root); err != nil {
		return g.fail("setup", err.Error(), nil)
	}
	g.gotmp = filepath.Join(root, "gotmp")
	for _, dir := range []string{"bin", "seed", "gotmp"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			return g.fail("setup", err.Error(), nil)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout) // one budget for every child
	defer cancel()
	return g.fail("setup", "stages are not implemented yet", nil)
}

// removeAll retries for up to 2s because Windows can briefly keep a just-killed
// executable locked.
func removeAll(dir string) error {
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := os.RemoveAll(dir)
		if err == nil || time.Now().After(deadline) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// report prints one FAIL line, then the bounded child output when there is
// a child result. The environment is never echoed.
func (g *gate) report(stage, reason string, r *childResult) {
	fmt.Fprintf(g.stderr, "govulncheck-gate: FAIL %s: %s\n", stage, reason)
	if r != nil {
		echo(g.stderr, "child stdout", &r.stdout)
		echo(g.stderr, "child stderr", &r.stderr)
	}
}

func (g *gate) fail(stage, reason string, r *childResult) int {
	g.report(stage, reason, r)
	return 1
}

// echo writes at most echoLimit bytes of c, then how many bytes were left out.
func echo(w io.Writer, name string, c *capture) {
	total := len(c.buf) + c.dropped
	if total == 0 {
		return
	}
	shown := c.buf[:min(len(c.buf), echoLimit)]
	fmt.Fprintf(w, "--- %s ---\n%s", name, shown)
	if len(shown) > 0 && shown[len(shown)-1] != '\n' {
		fmt.Fprintln(w)
	}
	if rest := total - len(shown); rest > 0 {
		fmt.Fprintf(w, "[truncated %d bytes]\n", rest)
	}
}

// capture keeps the first captureLimit bytes written to it and counts the
// rest. It always accepts the whole write, so a chatty child never blocks.
type capture struct {
	buf     []byte
	dropped int
}

func (c *capture) Write(p []byte) (int, error) {
	keep := min(len(p), captureLimit-len(c.buf))
	c.buf = append(c.buf, p[:keep]...)
	c.dropped += len(p) - keep
	return len(p), nil
}

type childResult struct {
	code           int
	failure        string // set when the child was not run to its own exit
	stdout, stderr capture
}

// problem returns why r is not a clean exit with code want, or "".
func (r *childResult) problem(name string, want int) string {
	switch {
	case r.failure != "":
		return r.failure
	case r.code != want:
		return fmt.Sprintf("%s exited %d", name, r.code)
	case r.stdout.dropped > 0:
		return fmt.Sprintf("stdout exceeded the %d-byte capture limit", captureLimit)
	case r.stderr.dropped > 0:
		return fmt.Sprintf("stderr exceeded the %d-byte capture limit", captureLimit)
	}
	return ""
}

// childEnv rebuilds a child's environment from the parent's on every call:
// the Go selection keys are dropped case-insensitively and set to exact
// values, so no stage inherits another stage's or the caller's selection.
func childEnv(base []string, toolchain, gotmp string) []string {
	env := make([]string, 0, len(base)+4)
	for _, kv := range base {
		key, _, _ := strings.Cut(kv, "=")
		if !slices.ContainsFunc(overriddenEnv, func(o string) bool { return strings.EqualFold(key, o) }) {
			env = append(env, kv)
		}
	}
	return append(env, "GOTOOLCHAIN="+toolchain, "GOFLAGS=-mod=readonly", "GOWORK=off", "GOTMPDIR="+gotmp)
}

// runChild runs one argv (never a shell string) to completion. On timeout or
// interrupt CommandContext kills the direct child; Wait always reaps it, and
// WaitDelay bounds the wait for pipes held open by its descendants.
func (g *gate) runChild(ctx context.Context, name string, args []string, dir, toolchain string) *childResult {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = childEnv(g.env, toolchain, g.gotmp)
	cmd.WaitDelay = g.opts.waitDelay
	r := &childResult{}
	cmd.Stdout, cmd.Stderr = &r.stdout, &r.stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		r.failure = "timed out after " + g.timeout.String()
	case errors.Is(ctx.Err(), context.Canceled):
		r.failure = "interrupted"
	case errors.Is(err, exec.ErrWaitDelay):
		r.failure = "output pipes still open after exit: " + err.Error()
	case errors.As(err, &exitErr):
		r.code = exitErr.ExitCode()
	case err != nil:
		r.failure = "start: " + err.Error()
	}
	return r
}

// onceString is a flag value that may be set only once.
type onceString struct {
	value string
	set   bool
}

func (o *onceString) String() string { return o.value }

func (o *onceString) Set(v string) error {
	if o.set {
		return errors.New("may be given only once")
	}
	o.value, o.set = v, true
	return nil
}

func parseArgs(args []string) (config, error) {
	flags := flag.NewFlagSet("govulncheck-gate", flag.ContinueOnError)
	flags.SetOutput(io.Discard) // run prints the error and usageText itself
	var repo onceString
	flags.Var(&repo, "repo-root", "module root to scan")
	timeout := flags.Duration("timeout", 30*time.Minute, "limit for all child processes")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	switch {
	case flags.NArg() != 0:
		return config{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	case repo.value == "":
		return config{}, errors.New("--repo-root is required")
	case *timeout <= 0:
		return config{}, fmt.Errorf("--timeout must be > 0, got %v", *timeout)
	}
	abs, err := filepath.Abs(repo.value)
	if err != nil {
		return config{}, err
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return config{}, fmt.Errorf("--repo-root %s is not a directory", abs)
	}
	if fi, err := os.Stat(filepath.Join(abs, "go.mod")); err != nil || !fi.Mode().IsRegular() {
		return config{}, fmt.Errorf("--repo-root %s has no regular go.mod file", abs)
	}
	return config{repoRoot: abs, timeout: *timeout}, nil
}

// setup reads the tool module's toolchain and scanner pin, and the
// repository's toolchain, before any temporary directory or child exists.
func (g *gate) setup(repoRoot string) error {
	tool, err := os.ReadFile(filepath.Join(g.opts.toolDir, "go.mod"))
	if err != nil {
		return fmt.Errorf("tool module: %w", err)
	}
	if !hasModuleLine(tool, toolModulePath) {
		return fmt.Errorf("tool module: %s is not module %s", filepath.Join(g.opts.toolDir, "go.mod"), toolModulePath)
	}
	if g.toolToolchain, err = parseToolchain(tool); err != nil {
		return fmt.Errorf("tool module: %w", err)
	}
	if g.pin, err = parseVulnPin(tool); err != nil {
		return fmt.Errorf("tool module: %w", err)
	}
	repo, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return fmt.Errorf("repository: %w", err)
	}
	if g.repoToolchain, err = parseToolchain(repo); err != nil {
		return fmt.Errorf("repository: %w", err)
	}
	g.repoRoot = repoRoot
	return nil
}

// modLines returns the whitespace-separated fields of every non-blank,
// non-comment go.mod line. strings.Fields also drops a CRLF line's "\r".
func modLines(gomod []byte) [][]string {
	var out [][]string
	for _, line := range strings.Split(string(gomod), "\n") {
		if f := strings.Fields(line); len(f) > 0 && !strings.HasPrefix(f[0], "//") {
			out = append(out, f)
		}
	}
	return out
}

func hasModuleLine(gomod []byte, path string) bool {
	for _, f := range modLines(gomod) {
		if len(f) >= 2 && f[0] == "module" && f[1] == path && (len(f) == 2 || strings.HasPrefix(f[2], "//")) {
			return true
		}
	}
	return false
}

// parseToolchain returns the single exact goX.Y.Z release named by the go.mod
// toolchain directive, rejecting default, local, +auto, rc/beta and
// language-only values.
func parseToolchain(gomod []byte) (string, error) {
	var found []string
	for _, f := range modLines(gomod) {
		if f[0] != "toolchain" {
			continue
		}
		if len(f) < 2 {
			return "", errors.New("toolchain directive has no value")
		}
		if len(f) > 2 && !strings.HasPrefix(f[2], "//") {
			return "", fmt.Errorf("unexpected text after toolchain %s", f[1])
		}
		found = append(found, f[1])
	}
	switch {
	case len(found) == 0:
		return "", errors.New("no toolchain directive")
	case len(found) > 1:
		return "", errors.New("duplicate toolchain directives")
	case !toolchainRE.MatchString(found[0]):
		return "", fmt.Errorf("toolchain %q is not an exact goX.Y.Z release", found[0])
	}
	return found[0], nil
}

// parseVulnPin returns the single golang.org/x/vuln release required by the
// go.mod, in a require block or on one line, and refuses a replaced module.
func parseVulnPin(gomod []byte) (string, error) {
	var pins []string
	block := ""
	for _, f := range modLines(gomod) {
		switch {
		case block != "" && f[0] == ")":
			block = ""
			continue
		case block != "":
			f = append([]string{block}, f...)
		case len(f) == 2 && f[1] == "(":
			block = f[0]
			continue
		}
		if len(f) < 2 || f[1] != vulnModulePath {
			continue
		}
		switch f[0] {
		case "replace":
			return "", errors.New(vulnModulePath + " is replaced")
		case "require":
			if len(f) < 3 {
				return "", errors.New(vulnModulePath + " require has no version")
			}
			pins = append(pins, f[2])
		}
	}
	switch {
	case len(pins) == 0:
		return "", errors.New("no " + vulnModulePath + " require")
	case len(pins) > 1:
		return "", errors.New("duplicate " + vulnModulePath + " requires")
	case !releaseRE.MatchString(pins[0]):
		return "", fmt.Errorf("%s %q is not a vX.Y.Z release", vulnModulePath, pins[0])
	}
	return pins[0], nil
}

// scannerName is the file name go build must write for goos.
func scannerName(goos string) string {
	if goos == "windows" {
		return "govulncheck.exe"
	}
	return "govulncheck"
}
