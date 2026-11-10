package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The test binary doubles as the fake `go` and the fake `govulncheck`: when
// CAPGATE_FAKE names a scenario, TestMain runs the fake instead of the tests.
// The role comes from the executable name, because the fake `go build` copies
// the test binary to <root>/bin/govulncheck[.exe].
const (
	fakeScenarioEnv   = "CAPGATE_FAKE"
	fakeLogEnv        = "CAPGATE_FAKE_LOG"
	fakeRepoToolchain = "go1.26.7" // differs from the tool module's toolchain on purpose
	traceLine         = "      #1: main.go:10:24: capseed.main calls url.URL.Parse\n"
)

var trackedEnv = []string{"GOTOOLCHAIN", "GOVERSION", "GOROOT", "GOFLAGS", "GOWORK", "GOTMPDIR"}

// Verified govulncheck v1.8.0 output (2026-10-26), split after the header.
const (
	seedBody = `=== Symbol Results ===

Vulnerability #1: GO-2026-6218
    Avoid quadratic complexity in resolvePath in net/url
  More info: https://pkg.go.dev/vuln/GO-2026-6218
  Standard library
    Found in: net/url@go1.25.12
    Fixed in: net/url@go1.25.13
    Example traces found:
` + traceLine + `
Your code is affected by 1 vulnerability from the Go standard library.
This scan also found 1 vulnerability in packages you import and 19
vulnerabilities in modules you require, but your code doesn't appear to call
these vulnerabilities.
Use '-show verbose' for more details.
`
	cleanBodyA = "No vulnerabilities found.\n"
	cleanBodyB = `=== Symbol Results ===

No vulnerabilities found.

Your code is affected by 0 vulnerabilities.
This scan also found 1 vulnerability in packages you import and 1 vulnerability
in modules you require, but your code doesn't appear to call these
vulnerabilities.
Use '-show verbose' for more details.
`
	repoVulnBody = `=== Symbol Results ===

Vulnerability #1: GO-2026-6809
    Excessive resource consumption in crypto/x509
  More info: https://pkg.go.dev/vuln/GO-2026-6809
  Standard library
    Found in: crypto/x509@go1.26.9
    Fixed in: crypto/x509@go1.26.10
    Example traces found:
      #1: main.go:9:2: repo.main calls x509.ParseCertificate

Your code is affected by 1 vulnerability from the Go standard library.
Use '-show verbose' for more details.
`
	wantSeedGoMod  = "module example.com/capseed\n\ngo 1.25.12\n"
	wantSeedMainGo = `package main

import (
	"fmt"
	"net/url"
)

func main() {
	base := &url.URL{Scheme: "https", Host: "example.com", Path: "/a/"}
	ref, err := base.Parse("b/../c")
	fmt.Println(ref, err)
}
`
)

func fakeHeader(goVersion, scanner string) string {
	return "Go: " + goVersion + "\nScanner: govulncheck@" + scanner +
		"\nDB: https://vuln.go.dev\nDB updated: 2026-10-21 00:27:29 +0000 UTC\n\n"
}

type fakeCall struct {
	Role  string              `json:"role"`
	Args  []string            `json:"args"`
	Cwd   string              `json:"cwd"`
	Pid   int                 `json:"pid"`
	Env   map[string][]string `json:"env"`   // every case-insensitive match of a tracked key
	Files map[string]string   `json:"files"` // go.mod and main.go in cwd, when present
}

func TestMain(m *testing.M) {
	if scenario := os.Getenv(fakeScenarioEnv); scenario != "" {
		os.Exit(fakeMain(scenario))
	}
	os.Exit(m.Run())
}

func fakeMain(scenario string) int {
	role := "go"
	if strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") == "govulncheck" {
		role = "govulncheck"
	}
	call, err := logFakeCall(role)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake:", err)
		return 97
	}
	if role == "go" {
		return fakeGo(scenario, call.Args)
	}
	return fakeScanner(scenario, call)
}

func logFakeCall(role string) (fakeCall, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return fakeCall{}, err
	}
	c := fakeCall{Role: role, Args: os.Args[1:], Cwd: cwd, Pid: os.Getpid(),
		Env: map[string][]string{}, Files: map[string]string{}}
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		for _, want := range trackedEnv {
			if strings.EqualFold(k, want) {
				c.Env[want] = append(c.Env[want], v)
			}
		}
	}
	for _, name := range []string{"go.mod", "main.go"} {
		if b, err := os.ReadFile(name); err == nil {
			c.Files[name] = string(b)
		}
	}
	line, err := json.Marshal(c)
	if err != nil {
		return c, err
	}
	f, err := os.OpenFile(os.Getenv(fakeLogEnv), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return c, err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return c, err
}

func fakeGo(scenario string, args []string) int {
	if len(args) == 0 || args[0] != "build" {
		fmt.Fprintln(os.Stderr, "fake go: only build is supported")
		return 2
	}
	switch scenario {
	case "build-fail":
		fmt.Fprintln(os.Stderr, "build boom")
		os.Stderr.Write(bytes.Repeat([]byte("e"), 2<<20))
		return 1
	case "build-noexe":
		return 0
	case "build-hang":
		time.Sleep(10 * time.Minute)
		return 0
	}
	i := slices.Index(args, "-o")
	self, err := os.Executable()
	if i < 0 || i+1 >= len(args) || err != nil {
		fmt.Fprintln(os.Stderr, "fake go: need -o and os.Executable:", err)
		return 2
	}
	b, err := os.ReadFile(self)
	if err == nil {
		err = os.WriteFile(args[i+1], b, 0o755)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake go:", err)
		return 2
	}
	return 0
}

func fakeScanner(scenario string, c fakeCall) int {
	if !slices.Equal(c.Args, []string{"-show", "version", "./..."}) {
		fmt.Fprintln(os.Stderr, "fake govulncheck: unexpected args", c.Args)
		return 2
	}
	// v1.8.0 consults GOVERSION before `go env GOVERSION`.
	v, ok := os.LookupEnv("GOVERSION")
	if !ok {
		v = os.Getenv("GOTOOLCHAIN")
	}
	target := "repo"
	if strings.Contains(c.Files["go.mod"], "module example.com/capseed") {
		target = "seed"
	}
	stage, mode, _ := strings.Cut(scenario, "-")
	if stage != target {
		mode = ""
	}
	if target == "seed" {
		return fakeSeed(mode, v)
	}
	return fakeRepo(mode, v)
}

func fakeSeed(mode, v string) int {
	scanner := "v1.8.0"
	switch mode {
	case "db":
		fmt.Fprintln(os.Stderr, "govulncheck: creating client: unrecognized vulndb format")
		return 1
	case "clean":
		v = "go1.26.10" // the toolchain selection did not take effect
	case "wrong-scanner":
		scanner = "v1.7.1"
	}
	if v != "go1.25.12" {
		fmt.Print(fakeHeader(v, scanner) + cleanBodyA)
		return 0
	}
	body := seedBody
	switch mode {
	case "wrong-id":
		body = strings.Replace(body, ": GO-2026-6218", ": GO-2026-0001", 1)
	case "wrong-version":
		body = strings.Replace(body, "Found in: net/url@go1.25.12", "Found in: net/url@go1.25.13", 1)
	case "no-trace":
		body = strings.Replace(body, traceLine, "", 1)
	case "wrong-symbol":
		body = strings.Replace(body, "url.URL.Parse", "url.URL.String", 1)
	}
	fmt.Print(fakeHeader(v, scanner) + body)
	if mode == "exit0" {
		return 0
	}
	return 3
}

func fakeRepo(mode, v string) int {
	switch mode {
	case "db":
		fmt.Fprintln(os.Stderr, "govulncheck: creating client: unrecognized vulndb format")
		return 1
	case "vuln":
		fmt.Print(fakeHeader(v, "v1.8.0") + repoVulnBody)
		return 3
	case "malformed":
		fmt.Print(fakeHeader(v, "v1.8.0") + "unexpected scanner output\n")
		return 0
	case "exit2":
		fmt.Fprintln(os.Stderr, "govulncheck: internal error")
		return 2
	case "wrong-go":
		fmt.Print(fakeHeader("go1.26.6", "v1.8.0") + cleanBodyB)
		return 0
	case "flood":
		fmt.Print(fakeHeader(v, "v1.8.0") + cleanBodyB)
		os.Stdout.Write(bytes.Repeat([]byte("x\n"), 2<<20)) // 4 MiB
		return 0
	case "hang":
		time.Sleep(10 * time.Minute)
		return 0
	}
	fmt.Print(fakeHeader(v, "v1.8.0") + cleanBodyB)
	return 0
}

// ---- gate harness ----

type gateHarness struct {
	repo string
	log  string
	env  []string
	opts options
}

type gateResult struct {
	code           int
	stdout, stderr string
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func baseEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), "CAPGATE_") {
			env = append(env, kv)
		}
	}
	return env
}

func newGate(t *testing.T, scenario string) *gateHarness {
	t.Helper()
	dir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	h := &gateHarness{repo: filepath.Join(dir, "repo"), log: filepath.Join(dir, "calls.jsonl")}
	writeFile(t, filepath.Join(h.repo, "go.mod"),
		"module example.com/repo\n\ngo 1.26.0\n\ntoolchain "+fakeRepoToolchain+"\n")
	h.env = append(baseEnv(), fakeScenarioEnv+"="+scenario, fakeLogEnv+"="+h.log)
	h.opts = options{goBin: self, toolDir: ".", tempParent: t.TempDir(), waitDelay: 200 * time.Millisecond}
	return h
}

func (h *gateHarness) run(ctx context.Context, args ...string) gateResult {
	var stdout, stderr bytes.Buffer
	code := run(ctx, args, h.env, &stdout, &stderr, h.opts)
	return gateResult{code, stdout.String(), stderr.String()}
}

func (h *gateHarness) runRepo(extra ...string) gateResult {
	return h.run(context.Background(), append([]string{"--repo-root", h.repo}, extra...)...)
}

// readCalls parses only complete log lines, so it is safe while a fake runs.
func readCalls(path string) ([]fakeCall, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var calls []fakeCall
	for _, line := range strings.SplitAfter(string(b), "\n") {
		if !strings.HasSuffix(line, "\n") {
			break
		}
		var c fakeCall
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("bad log line %q: %w", line, err)
		}
		calls = append(calls, c)
	}
	return calls, nil
}

func (h *gateHarness) calls(t *testing.T) []fakeCall {
	t.Helper()
	calls, err := readCalls(h.log)
	if err != nil {
		t.Fatal(err)
	}
	return calls
}

func roles(calls []fakeCall) []string {
	var r []string
	for _, c := range calls {
		r = append(r, c.Role)
	}
	return r
}

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("%s is not empty after the run: %d entries, first %q", dir, len(entries), entries[0].Name())
	}
}

func assertProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		p, err := os.FindProcess(pid)
		if err != nil { // Windows: the process no longer exists
			return
		}
		if runtime.GOOS != "windows" && p.Signal(syscall.Signal(0)) != nil {
			return
		}
		p.Release()
		if time.Now().After(deadline) {
			t.Fatalf("child pid %d still exists after the gate returned", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// toolModule reads the pinned tool module with regexes independent of the
// parsers under test.
func toolModule(t *testing.T) (toolchain, pin string) {
	t.Helper()
	b, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	tc := regexp.MustCompile(`(?m)^toolchain (go\S+)\s*$`).FindSubmatch(b)
	vp := regexp.MustCompile(`(?m)^\s*golang\.org/x/vuln (v\S+)`).FindSubmatch(b)
	if tc == nil || vp == nil {
		t.Fatalf("tool go.mod lacks a toolchain line or an x/vuln require:\n%s", b)
	}
	return string(tc[1]), string(vp[1])
}

func summaryLines(toolchain, pin, repoToolchain, repo string) []string {
	return []string{
		"govulncheck-gate: scanner golang.org/x/vuln@" + pin + " built with -mod=readonly (toolchain " + toolchain + ")\n",
		"govulncheck-gate: seed refused: GO-2026-6218 net/url@go1.25.12 reachable: capseed.main calls url.URL.Parse\n",
		"govulncheck-gate: repository clean under " + repoToolchain + ": " + repo + "\n",
		"govulncheck-gate: PASS\n",
	}
}

func assertHappyRun(t *testing.T, h *gateHarness, res gateResult) {
	t.Helper()
	toolchain, pin := toolModule(t)
	if res.code != 0 || res.stderr != "" {
		t.Fatalf("exit %d, stderr:\n%s", res.code, res.stderr)
	}
	if want := strings.Join(summaryLines(toolchain, pin, fakeRepoToolchain, h.repo), ""); res.stdout != want {
		t.Fatalf("stdout:\n%q\nwant:\n%q", res.stdout, want)
	}
	calls := h.calls(t)
	if got := roles(calls); !slices.Equal(got, []string{"go", "govulncheck", "govulncheck"}) {
		t.Fatalf("calls %v, want build, seed, repo", got)
	}
	build, seed, repo := calls[0], calls[1], calls[2]
	if len(build.Args) != 6 {
		t.Fatalf("build args %q", build.Args)
	}
	root := filepath.Dir(filepath.Dir(build.Args[4]))
	if filepath.Dir(root) != h.opts.tempParent || !strings.HasPrefix(filepath.Base(root), "cap-govulncheck-gate-") {
		t.Fatalf("temp root %q is not a cap-govulncheck-gate-* dir under %q", root, h.opts.tempParent)
	}
	wantBuild := []string{"build", "-mod=readonly", "-buildvcs=false", "-o",
		filepath.Join(root, "bin", scannerName(runtime.GOOS)), "golang.org/x/vuln/cmd/govulncheck"}
	if !slices.Equal(build.Args, wantBuild) {
		t.Fatalf("build args %q, want %q", build.Args, wantBuild)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	scanArgs := []string{"-show", "version", "./..."}
	for _, c := range []struct {
		call      fakeCall
		cwd, tool string
	}{{build, wd, toolchain}, {seed, filepath.Join(root, "seed"), "go1.25.12"}, {repo, h.repo, fakeRepoToolchain}} {
		if c.call.Cwd != c.cwd {
			t.Errorf("%s cwd %q, want %q", c.call.Role, c.call.Cwd, c.cwd)
		}
		wantEnv := map[string][]string{"GOTOOLCHAIN": {c.tool}, "GOFLAGS": {"-mod=readonly"},
			"GOWORK": {"off"}, "GOTMPDIR": {filepath.Join(root, "gotmp")}}
		if !reflect.DeepEqual(c.call.Env, wantEnv) {
			t.Errorf("%s (cwd %s) env %v, want %v", c.call.Role, c.call.Cwd, c.call.Env, wantEnv)
		}
	}
	if !slices.Equal(seed.Args, scanArgs) || !slices.Equal(repo.Args, scanArgs) {
		t.Errorf("scan args seed=%q repo=%q, want %q", seed.Args, repo.Args, scanArgs)
	}
	if seed.Files["go.mod"] != wantSeedGoMod || seed.Files["main.go"] != wantSeedMainGo {
		t.Errorf("seed files differ from the verified program: %q", seed.Files)
	}
	assertEmptyDir(t, h.opts.tempParent)
}

// ---- tests ----

func TestHappyPathPinsEveryStage(t *testing.T) {
	t.Parallel()
	h := newGate(t, "ok")
	assertHappyRun(t, h, h.runRepo())
}

func TestInheritedGoOverridesAreReplacedPerStage(t *testing.T) {
	t.Parallel()
	h := newGate(t, "ok")
	bogus := filepath.Join(t.TempDir(), "bogus")
	h.env = append(h.env, "GOVERSION=go1.25.12", "GOROOT="+bogus, "GOFLAGS=-mod=mod",
		"GOTOOLCHAIN=go1.25.12+auto", "GOWORK="+filepath.Join(bogus, "go.work"), "GOTMPDIR="+bogus,
		"GoVersion=go1.25.12", "gotoolchain=local")
	assertHappyRun(t, h, h.runRepo())
}

func TestSeedRefusalHappensBeforeRepositoryScan(t *testing.T) {
	t.Parallel()
	notReported := "seeded vulnerability was not reported (govulncheck exit 0)"
	noTrace := `GO-2026-6218 block lacks a "#N: main.go:L:C: capseed.main calls url.URL.Parse" trace`
	cases := []struct {
		scenario string
		want     []string
	}{
		{"seed-clean", []string{notReported}},
		{"seed-exit0", []string{notReported}},
		{"seed-db", []string{"govulncheck exited 1", "unrecognized vulndb format"}},
		{"seed-wrong-id", []string{`missing "Vulnerability #N: GO-2026-6218" block`}},
		{"seed-wrong-version", []string{`GO-2026-6218 block lacks "Found in: net/url@go1.25.12"`}},
		{"seed-no-trace", []string{noTrace}},
		{"seed-wrong-symbol", []string{noTrace}},
		{"seed-wrong-scanner", []string{`missing "Scanner: govulncheck@v1.8.0" header`}},
	}
	for _, tc := range cases {
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()
			h := newGate(t, tc.scenario)
			res := h.runRepo()
			if res.code != 1 {
				t.Fatalf("exit %d, want 1; stderr:\n%s", res.code, res.stderr)
			}
			if !strings.HasPrefix(res.stderr, "govulncheck-gate: FAIL seed: "+tc.want[0]+"\n") {
				t.Fatalf("stderr does not start with the seed failure %q:\n%s", tc.want[0], res.stderr)
			}
			for _, w := range tc.want[1:] {
				if !strings.Contains(res.stderr, w) {
					t.Errorf("stderr lacks %q", w)
				}
			}
			if got := roles(h.calls(t)); !slices.Equal(got, []string{"go", "govulncheck"}) {
				t.Fatalf("calls %v: the repository must not be scanned after a seed failure", got)
			}
			if strings.Contains(res.stdout, "seed refused") || strings.Contains(res.stdout, "PASS") {
				t.Fatalf("stdout claims success:\n%s", res.stdout)
			}
			assertEmptyDir(t, h.opts.tempParent)
		})
	}
}

func TestRepositoryFailuresPropagate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		scenario string
		code     int
		want     []string
	}{
		{"repo-vuln", 3, []string{"vulnerabilities found (govulncheck exit 3)", "Vulnerability #1: GO-2026-6809"}},
		{"repo-db", 1, []string{"govulncheck exited 1", "unrecognized vulndb format"}},
		{"repo-exit2", 1, []string{"govulncheck exited 2", "internal error"}},
		{"repo-malformed", 1, []string{`missing "No vulnerabilities found." line`}},
		{"repo-wrong-go", 1, []string{`missing "Go: ` + fakeRepoToolchain + `" header`}},
		{"repo-flood", 1, []string{"stdout exceeded the 1048576-byte capture limit", "[truncated "}},
	}
	toolchain, pin := toolModule(t)
	for _, tc := range cases {
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()
			h := newGate(t, tc.scenario)
			res := h.runRepo()
			if res.code != tc.code {
				t.Fatalf("exit %d, want %d; stderr:\n%.4000s", res.code, tc.code, res.stderr)
			}
			if !strings.HasPrefix(res.stderr, "govulncheck-gate: FAIL repository: "+tc.want[0]+"\n") {
				t.Fatalf("stderr does not start with the repository failure %q:\n%.4000s", tc.want[0], res.stderr)
			}
			for _, w := range tc.want[1:] {
				if !strings.Contains(res.stderr, w) {
					t.Errorf("stderr lacks %q", w)
				}
			}
			if len(res.stderr) >= 200<<10 {
				t.Errorf("stderr is %d bytes, want under 200 KiB", len(res.stderr))
			}
			if want := strings.Join(summaryLines(toolchain, pin, "", "")[:2], ""); res.stdout != want {
				t.Fatalf("stdout:\n%q\nwant:\n%q", res.stdout, want)
			}
			if got := roles(h.calls(t)); len(got) != 3 {
				t.Fatalf("calls %v, want build, seed, repo", got)
			}
			assertEmptyDir(t, h.opts.tempParent)
		})
	}
}

func TestTimeoutKillsAndReapsTheChild(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ scenario, stage, role string }{
		{"build-hang", "build", "go"},
		{"repo-hang", "repository", "govulncheck"},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()
			h := newGate(t, tc.scenario)
			start := time.Now()
			res := h.runRepo("--timeout", "2s")
			if elapsed := time.Since(start); elapsed > 15*time.Second {
				t.Fatalf("gate took %v after a 2s timeout", elapsed)
			}
			want := "govulncheck-gate: FAIL " + tc.stage + ": timed out after 2s\n"
			if res.code != 1 || !strings.HasPrefix(res.stderr, want) {
				t.Fatalf("exit %d, stderr:\n%s\nwant prefix %q", res.code, res.stderr, want)
			}
			calls := h.calls(t)
			if len(calls) == 0 || calls[len(calls)-1].Role != tc.role {
				t.Fatalf("calls %v, want the last one to be %s", roles(calls), tc.role)
			}
			assertProcessGone(t, calls[len(calls)-1].Pid)
			assertEmptyDir(t, h.opts.tempParent)
		})
	}
}

func TestInterruptKillsAndReapsTheChild(t *testing.T) {
	t.Parallel()
	h := newGate(t, "repo-hang")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan gateResult, 1)
	go func() { done <- h.run(ctx, "--repo-root", h.repo) }()
	var calls []fakeCall
	for deadline := time.Now().Add(30 * time.Second); len(calls) < 3; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("repository scan never started; calls %v", roles(calls))
		}
		var err error
		if calls, err = readCalls(h.log); err != nil {
			t.Fatal(err)
		}
	}
	cancel()
	var res gateResult
	select {
	case res = <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("gate did not return after cancellation")
	}
	if want := "govulncheck-gate: FAIL repository: interrupted\n"; res.code != 1 || !strings.HasPrefix(res.stderr, want) {
		t.Fatalf("exit %d, stderr:\n%s\nwant prefix %q", res.code, res.stderr, want)
	}
	assertProcessGone(t, calls[2].Pid)
	assertEmptyDir(t, h.opts.tempParent)
}

func TestBuildAndSpawnFailures(t *testing.T) {
	t.Parallel()
	toolchain, pin := toolModule(t)
	built := summaryLines(toolchain, pin, "", "")[0]
	cases := []struct {
		name, scenario string
		missingGo      bool
		want           []string
		stdout         string
		calls          []string
	}{
		{"missing go binary", "ok", true, []string{"build: start: "}, "", nil},
		{"build fails", "build-fail", false, []string{"build: go build exited 1", "build boom", "[truncated "}, "", []string{"go"}},
		{"build writes no scanner", "build-noexe", false, []string{"seed: start: "}, built, []string{"go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newGate(t, tc.scenario)
			if tc.missingGo {
				h.opts.goBin = filepath.Join(t.TempDir(), "missing-go")
			}
			res := h.runRepo()
			if res.code != 1 || !strings.HasPrefix(res.stderr, "govulncheck-gate: FAIL "+tc.want[0]) {
				t.Fatalf("exit %d, stderr:\n%.4000s\nwant prefix FAIL %s", res.code, res.stderr, tc.want[0])
			}
			for _, w := range tc.want[1:] {
				if !strings.Contains(res.stderr, w) {
					t.Errorf("stderr lacks %q", w)
				}
			}
			if len(res.stderr) >= 200<<10 {
				t.Errorf("stderr is %d bytes, want under 200 KiB", len(res.stderr))
			}
			if res.stdout != tc.stdout {
				t.Errorf("stdout %q, want %q", res.stdout, tc.stdout)
			}
			if got := roles(h.calls(t)); !slices.Equal(got, tc.calls) {
				t.Errorf("calls %v, want %v", got, tc.calls)
			}
			assertEmptyDir(t, h.opts.tempParent)
		})
	}
}

func TestUsageErrorsExit2BeforeAnyWork(t *testing.T) {
	t.Parallel()
	h := newGate(t, "ok")
	noMod := t.TempDir()
	modIsDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(modIsDir, "go.mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]string{
		"no args":                 {},
		"repo-root has no value":  {"--repo-root"},
		"empty repo-root":         {"--repo-root", ""},
		"duplicate repo-root":     {"--repo-root", h.repo, "--repo-root", h.repo},
		"unknown flag":            {"--repo-root", h.repo, "--bogus"},
		"positional argument":     {"--repo-root", h.repo, "extra"},
		"nonexistent repo root":   {"--repo-root", filepath.Join(h.repo, "missing")},
		"repo root is a file":     {"--repo-root", filepath.Join(h.repo, "go.mod")},
		"repo root has no go.mod": {"--repo-root", noMod},
		"go.mod is a directory":   {"--repo-root", modIsDir},
		"zero timeout":            {"--repo-root", h.repo, "--timeout", "0"},
		"negative timeout":        {"--repo-root", h.repo, "--timeout", "-1s"},
		"malformed timeout":       {"--repo-root", h.repo, "--timeout", "soon"},
	}
	for name, args := range cases {
		res := h.run(context.Background(), args...)
		if res.code != 2 || !strings.HasPrefix(res.stderr, "govulncheck-gate: FAIL usage: ") ||
			!strings.Contains(res.stderr, "usage: govulncheck-gate --repo-root DIR") || res.stdout != "" {
			t.Errorf("%s: exit %d, stdout %q, stderr:\n%s", name, res.code, res.stdout, res.stderr)
		}
	}
	if calls := h.calls(t); calls != nil {
		t.Fatalf("usage errors spawned children: %v", roles(calls))
	}
	assertEmptyDir(t, h.opts.tempParent)
}

func TestSetupErrorsExit1BeforeAnyChild(t *testing.T) {
	t.Parallel()
	const (
		module = "module github.com/cordum-io/cap/v2/tools/govulncheck\n\ngo 1.26.0\n"
		tc     = "\ntoolchain go1.26.10\n"
		pin    = "\nrequire golang.org/x/vuln v1.8.0 // indirect\n"
	)
	cases := []struct{ name, toolGoMod, repoGoMod, want string }{
		{"wrong tool module", "module example.com/other\n\ngo 1.26.0\n" + tc + pin, "", "github.com/cordum-io/cap/v2/tools/govulncheck"},
		{"tool module has no toolchain", module + pin, "", "tool module: no toolchain directive"},
		{"tool module has no x/vuln pin", module + tc, "", "tool module: no golang.org/x/vuln require"},
		{"tool dir has no go.mod", "", "", "go.mod"},
		{"repo has no toolchain", module + tc + pin, "module example.com/repo\n\ngo 1.26.0\n", "repository: no toolchain directive"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := newGate(t, "ok")
			h.opts.toolDir = t.TempDir()
			if c.toolGoMod != "" {
				writeFile(t, filepath.Join(h.opts.toolDir, "go.mod"), c.toolGoMod)
			}
			if c.repoGoMod != "" {
				writeFile(t, filepath.Join(h.repo, "go.mod"), c.repoGoMod)
			}
			res := h.runRepo()
			if res.code != 1 || !strings.HasPrefix(res.stderr, "govulncheck-gate: FAIL setup: ") ||
				!strings.Contains(res.stderr, c.want) || res.stdout != "" {
				t.Fatalf("exit %d, stdout %q, stderr:\n%s\nwant FAIL setup containing %q", res.code, res.stdout, res.stderr, c.want)
			}
			if calls := h.calls(t); calls != nil {
				t.Fatalf("setup errors spawned children: %v", roles(calls))
			}
			assertEmptyDir(t, h.opts.tempParent)
		})
	}
}

func TestParseToolchain(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, gomod, want string }{
		{"plain", "module x\n\ngo 1.26.0\n\ntoolchain go1.26.10\n", "go1.26.10"},
		{"trailing comment", "module x\ntoolchain go1.26.10 // pinned\n", "go1.26.10"},
		{"crlf", "module x\r\n\r\ngo 1.26.0\r\n\r\ntoolchain go1.26.10\r\n", "go1.26.10"},
		{"zero patch", "toolchain go1.25.0\n", "go1.25.0"},
		{"missing", "module x\n\ngo 1.26.0\n", ""},
		{"duplicate", "toolchain go1.26.10\ntoolchain go1.26.10\n", ""},
		{"default", "toolchain default\n", ""},
		{"local", "toolchain local\n", ""},
		{"language version only", "toolchain go1.26\n", ""},
		{"release candidate", "toolchain go1.27rc1\n", ""},
		{"auto suffix", "toolchain go1.26.10+auto\n", ""},
		{"leading zero", "toolchain go1.026.1\n", ""},
		{"trailing junk", "toolchain go1.26.10 extra\n", ""},
		{"no value", "toolchain\n", ""},
		{"commented out", "// toolchain go1.26.10\n", ""},
	}
	for _, c := range cases {
		got, err := parseToolchain([]byte(c.gomod))
		if got != c.want || (err == nil) != (c.want != "") {
			t.Errorf("%s: parseToolchain = %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}

func TestParseVulnPin(t *testing.T) {
	t.Parallel()
	pinned, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, gomod, want string }{
		{"pinned tool module", string(pinned), "v1.8.0"},
		{"require block", "tool golang.org/x/vuln/cmd/govulncheck\n\nrequire (\n\tgolang.org/x/mod v0.41.0 // indirect\n\tgolang.org/x/vuln v1.8.0 // indirect\n)\n", "v1.8.0"},
		{"single-line require", "require golang.org/x/vuln v1.8.0\n", "v1.8.0"},
		{"crlf block", "require (\r\n\tgolang.org/x/vuln v1.8.0 // indirect\r\n)\r\n", "v1.8.0"},
		{"missing", "require (\n\tgolang.org/x/mod v0.41.0\n)\n", ""},
		{"duplicate", "require golang.org/x/vuln v1.8.0\nrequire (\n\tgolang.org/x/vuln v1.8.0\n)\n", ""},
		{"not a release", "require golang.org/x/vuln latest\n", ""},
		{"replaced", "require golang.org/x/vuln v1.8.0\nreplace golang.org/x/vuln => ../fork\n", ""},
		{"replaced in block", "require golang.org/x/vuln v1.8.0\nreplace (\n\tgolang.org/x/vuln v1.8.0 => ../fork\n)\n", ""},
	}
	for _, c := range cases {
		got, err := parseVulnPin([]byte(c.gomod))
		if got != c.want || (err == nil) != (c.want != "") {
			t.Errorf("%s: parseVulnPin = %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}

func TestScannerName(t *testing.T) {
	t.Parallel()
	for goos, want := range map[string]string{"windows": "govulncheck.exe", "linux": "govulncheck", "darwin": "govulncheck"} {
		if got := scannerName(goos); got != want {
			t.Errorf("scannerName(%q) = %q, want %q", goos, got, want)
		}
	}
}

func TestCheckSeedOutput(t *testing.T) {
	t.Parallel()
	golden := fakeHeader("go1.25.12", "v1.8.0") + seedBody
	trace, err := checkSeedOutput([]byte(golden), "v1.8.0")
	if err != nil || trace != "#1: main.go:10:24: capseed.main calls url.URL.Parse" {
		t.Fatalf("golden: trace %q, err %v", trace, err)
	}
	if _, err := checkSeedOutput([]byte(strings.ReplaceAll(golden, "\n", "\r\n")), "v1.8.0"); err != nil {
		t.Fatalf("crlf golden: %v", err)
	}
	noTrace := `GO-2026-6218 block lacks a "#N: main.go:L:C: capseed.main calls url.URL.Parse" trace`
	otherBlock := "Vulnerability #2: GO-2026-9999\n  Standard library\n    Found in: net/url@go1.25.12\n    Example traces found:\n" + traceLine
	cases := []struct{ name, old, new, want string }{
		{"wrong go", "Go: go1.25.12", "Go: go1.26.10", `missing "Go: go1.25.12" header`},
		{"wrong scanner", "govulncheck@v1.8.0", "govulncheck@v1.7.1", `missing "Scanner: govulncheck@v1.8.0" header`},
		{"no symbol section", "=== Symbol Results ===\n", "", `missing "=== Symbol Results ===" section`},
		{"wrong id", "Vulnerability #1: GO-2026-6218", "Vulnerability #1: GO-2026-0001", `missing "Vulnerability #N: GO-2026-6218" block`},
		{"indented header", "Vulnerability #1: GO-2026-6218", "  Vulnerability #1: GO-2026-6218", `missing "Vulnerability #N: GO-2026-6218" block`},
		{"wrong found-in", "net/url@go1.25.12", "net/url@go1.25.13", `GO-2026-6218 block lacks "Found in: net/url@go1.25.12"`},
		{"no traces heading", "    Example traces found:\n", "", `GO-2026-6218 block lacks "Example traces found:"`},
		{"no trace", traceLine, "", noTrace},
		{"wrong symbol", "url.URL.Parse", "url.URL.String", noTrace},
		{"wrong caller", "capseed.main calls", "capseed.helper calls", noTrace},
		{"trace outside block", traceLine + "\n", "\n" + strings.TrimSpace(traceLine) + "\n", noTrace},
		{"evidence in another block", "    Found in: net/url@go1.25.12\n    Fixed in: net/url@go1.25.13\n    Example traces found:\n" + traceLine,
			"\n" + otherBlock, `GO-2026-6218 block lacks "Found in: net/url@go1.25.12"`},
	}
	for _, c := range cases {
		mutated := strings.Replace(golden, c.old, c.new, 1)
		if mutated == golden {
			t.Fatalf("%s: mutation did not apply", c.name)
		}
		if _, err := checkSeedOutput([]byte(mutated), "v1.8.0"); err == nil || err.Error() != c.want {
			t.Errorf("%s: err %v, want %q", c.name, err, c.want)
		}
	}
}

func TestCheckCleanOutput(t *testing.T) {
	t.Parallel()
	header := fakeHeader("go1.26.10", "v1.8.0")
	for name, out := range map[string]string{"variant A": header + cleanBodyA, "variant B": header + cleanBodyB,
		"crlf": strings.ReplaceAll(header+cleanBodyB, "\n", "\r\n")} {
		if err := checkCleanOutput([]byte(out), "go1.26.10", "v1.8.0"); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	cases := []struct{ name, out, want string }{
		{"wrong go", fakeHeader("go1.26.9", "v1.8.0") + cleanBodyB, `missing "Go: go1.26.10" header`},
		{"wrong scanner", fakeHeader("go1.26.10", "v1.7.1") + cleanBodyB, `missing "Scanner: govulncheck@v1.8.0" header`},
		{"no clean line", header + "=== Symbol Results ===\n", `missing "No vulnerabilities found." line`},
		{"empty", "", `missing "Go: go1.26.10" header`},
		{"findings listed", header + cleanBodyA + repoVulnBody, `reports vulnerabilities ("Vulnerability #" line present)`},
	}
	for _, c := range cases {
		if err := checkCleanOutput([]byte(c.out), "go1.26.10", "v1.8.0"); err == nil || err.Error() != c.want {
			t.Errorf("%s: err %v, want %q", c.name, err, c.want)
		}
	}
}

func TestConcurrentRunsUseDistinctTempRoots(t *testing.T) {
	t.Parallel()
	shared := t.TempDir()
	hs := make([]*gateHarness, 4)
	for i := range hs {
		hs[i] = newGate(t, "ok")
		hs[i].opts.tempParent = shared
	}
	results := make([]gateResult, len(hs))
	var wg sync.WaitGroup
	for i, h := range hs {
		wg.Go(func() { results[i] = h.runRepo() })
	}
	wg.Wait()
	roots := map[string]bool{}
	for i, h := range hs {
		if results[i].code != 0 {
			t.Fatalf("run %d: exit %d, stderr:\n%s", i, results[i].code, results[i].stderr)
		}
		calls := h.calls(t)
		if len(calls) != 3 || len(calls[0].Args) != 6 {
			t.Fatalf("run %d: calls %v", i, roles(calls))
		}
		roots[filepath.Dir(filepath.Dir(calls[0].Args[4]))] = true
	}
	if len(roots) != len(hs) {
		t.Fatalf("temp roots %v are not distinct", roots)
	}
	assertEmptyDir(t, shared)
}

func TestRepoRootWithSpaceAndUnicodeIsResolvedVerbatim(t *testing.T) {
	t.Parallel()
	h := newGate(t, "ok")
	h.repo = filepath.Join(t.TempDir(), "my repo ü")
	writeFile(t, filepath.Join(h.repo, "go.mod"), "module example.com/repo\n\ngo 1.26.0\n\ntoolchain "+fakeRepoToolchain+"\n")
	arg := h.repo
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, h.repo); err == nil {
			arg = rel // a relative --repo-root must be resolved to the absolute path
		}
	}
	assertHappyRun(t, h, h.run(context.Background(), "--repo-root", arg))
}

func TestRealScannerEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("needs proxy.golang.org (go1.25.12 toolchain) and vuln.go.dev")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	toolchain, pin := toolModule(t)
	opts := options{goBin: goBin, toolDir: ".", tempParent: t.TempDir(), waitDelay: 10 * time.Second}
	runReal := func(t *testing.T, repo string) gateResult {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"--repo-root", repo, "--timeout", "20m"}, baseEnv(), &stdout, &stderr, opts)
		return gateResult{code, stdout.String(), stderr.String()}
	}
	t.Run("clean repository passes", func(t *testing.T) {
		repo := t.TempDir()
		writeFile(t, filepath.Join(repo, "go.mod"), "module example.com/clean\n\ngo 1.26.0\n\ntoolchain "+toolchain+"\n")
		writeFile(t, filepath.Join(repo, "main.go"), "package main\n\nfunc main() {}\n")
		res := runReal(t, repo)
		if want := strings.Join(summaryLines(toolchain, pin, toolchain, repo), ""); res.code != 0 || res.stdout != want {
			t.Fatalf("exit %d\nstdout:\n%s\nwant:\n%s\nstderr:\n%s", res.code, res.stdout, want, res.stderr)
		}
	})
	t.Run("vulnerable repository exits 3", func(t *testing.T) {
		repo := t.TempDir()
		writeFile(t, filepath.Join(repo, "go.mod"), "module example.com/vulnrepo\n\ngo 1.25.12\n\ntoolchain go1.25.12\n")
		writeFile(t, filepath.Join(repo, "main.go"), wantSeedMainGo)
		res := runReal(t, repo)
		want := "govulncheck-gate: FAIL repository: vulnerabilities found (govulncheck exit 3)\n"
		if res.code != 3 || !strings.HasPrefix(res.stderr, want) || !strings.Contains(res.stderr, "GO-2026-6218") {
			t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", res.code, res.stdout, res.stderr)
		}
		if wantOut := strings.Join(summaryLines(toolchain, pin, "", "")[:2], ""); res.stdout != wantOut {
			t.Fatalf("stdout:\n%q\nwant:\n%q", res.stdout, wantOut)
		}
	})
	assertEmptyDir(t, opts.tempParent)
}
