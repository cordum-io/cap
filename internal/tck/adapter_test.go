package tck

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestHelperAdapter is re-executed as a child to act as an adapter process. It
// is inert during an ordinary test run and only behaves as an adapter when the
// parent sets TCK_HELPER=1.
func TestHelperAdapter(t *testing.T) {
	if os.Getenv("TCK_HELPER") != "1" {
		return
	}
	runHelperAdapter(os.Getenv("TCK_ADAPTER_MODE"))
}

func runHelperAdapter(mode string) {
	out := bufio.NewWriter(os.Stdout)
	emit := func(m AdapterMessage) {
		b, _ := json.Marshal(m)
		out.Write(b)
		out.WriteByte('\n')
		out.Flush()
	}
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 4<<20)
	readCmd := func() (Command, bool) {
		if !in.Scan() {
			return Command{}, false
		}
		var c Command
		_ = json.Unmarshal(in.Bytes(), &c)
		return c, true
	}

	if mode == "hang-handshake" {
		blockForever()
	}
	readCmd() // consume hello
	if mode == "bad-version" {
		emit(AdapterMessage{Type: MsgHandshake, ProtocolVersion: 99, Role: RoleWorker, SDK: "helper"})
	} else {
		emit(AdapterMessage{Type: MsgHandshake, ProtocolVersion: ProtocolVersion, Role: RoleWorker, SDK: "helper", Transport: "none"})
	}
	for {
		cmd, ok := readCmd()
		if !ok || cmd.Type == MsgBye {
			os.Exit(0)
		}
		serveHelperCase(mode, cmd, emit)
	}
}

func serveHelperCase(mode string, cmd Command, emit func(AdapterMessage)) {
	switch mode {
	case "hang-run":
		blockForever()
	case "crash-on-run":
		os.Exit(3)
	case "noisy":
		// Exceed the runner's 64 KiB stderr cap so bounding is exercised.
		fmt.Fprint(os.Stderr, strings.Repeat("noise ", 40000))
		emit(AdapterMessage{Type: MsgResult, ID: cmd.ID, Status: StatusPass})
	case "wrong-id":
		emit(AdapterMessage{Type: MsgResult, ID: "MISMATCH", Status: StatusPass})
	case "dup-id":
		emit(AdapterMessage{Type: MsgResult, ID: cmd.ID, Status: StatusPass})
		emit(AdapterMessage{Type: MsgResult, ID: cmd.ID, Status: StatusPass})
	case "huge-line":
		emit(AdapterMessage{Type: MsgResult, ID: cmd.ID, Status: StatusPass, Detail: strings.Repeat("x", 2<<20)})
	case "echo-status":
		emit(AdapterMessage{Type: MsgResult, ID: cmd.ID, Status: Status(cmd.Params["status"])})
	case "cwd":
		wd, _ := os.Getwd()
		emit(AdapterMessage{Type: MsgResult, ID: cmd.ID, Status: StatusPass, Evidence: map[string]string{"cwd": wd}})
	case "spawn-grandchild":
		// Spawn a grandchild that inherits this process's stderr (exec's stderr
		// pipe write end) and outlives us, then return its pid so the test can
		// reap it. When the parent is killed the grandchild keeps the pipe open,
		// so Cmd.Wait blocks on the stderr copier until WaitDelay force-closes it.
		gc := exec.Command(os.Args[0], "-test.run=^TestHelperSleeper$")
		gc.Stderr = os.Stderr
		gc.Env = append(os.Environ(), "TCK_SLEEPER=1")
		pid := "0"
		if err := gc.Start(); err == nil && gc.Process != nil {
			pid = strconv.Itoa(gc.Process.Pid)
		}
		emit(AdapterMessage{Type: MsgResult, ID: cmd.ID, Status: StatusPass, Evidence: map[string]string{"grandchildPid": pid}})
	default:
		emit(AdapterMessage{Type: MsgResult, ID: cmd.ID, Status: StatusPass})
	}
}

// TestHelperSleeper is a grandchild process that lingers holding an inherited
// stderr handle. Inert unless TCK_SLEEPER=1.
func TestHelperSleeper(t *testing.T) {
	if os.Getenv("TCK_SLEEPER") != "1" {
		return
	}
	time.Sleep(10 * time.Second)
	os.Exit(0)
}

// Close must return within a bounded time even when a grandchild inherits the
// child's stderr and outlives it — WaitDelay force-closes the stderr copier so
// Cmd.Wait cannot block Close forever. Without WaitDelay this hangs ~10s.
func TestAdapterCloseIsBoundedWithLingeringGrandchild(t *testing.T) {
	a := helperAdapter("spawn-grandchild") // gracePeriod = WaitDelay = 500ms
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	res, err := a.Run(context.Background(), Command{Type: MsgRun, ID: "c1"}, 2*time.Second)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Reap the grandchild after the test so it does not linger holding a lock on
	// the test binary (Windows) — it must stay alive across Close() above.
	if pid, e := strconv.Atoi(res.Evidence["grandchildPid"]); e == nil && pid > 0 {
		t.Cleanup(func() {
			if p, e := os.FindProcess(pid); e == nil {
				_ = p.Kill()
			}
		})
	}
	done := make(chan struct{})
	go func() { a.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung > 5s: WaitDelay is not bounding the grandchild-held stderr copier")
	}
}

// blockForever keeps the helper process alive without responding. A bare
// select{} would trip Go's all-goroutines-asleep deadlock detector and make the
// child exit (the parent would see EOF, not a hang), so block on a long timer.
func blockForever() {
	for {
		time.Sleep(time.Hour)
	}
}

func helperAdapter(mode string) *Adapter {
	a := NewAdapter(AdapterSpec{
		Name: "helper-" + mode,
		Argv: []string{os.Args[0], "-test.run=^TestHelperAdapter$"},
		Env:  []string{"TCK_HELPER=1", "TCK_ADAPTER_MODE=" + mode},
	})
	a.handshakeTimeout = 3 * time.Second
	a.gracePeriod = 500 * time.Millisecond
	return a
}

func TestAdapterStartAndRunHappyPath(t *testing.T) {
	a := helperAdapter("well-behaved")
	hs, err := a.Start(context.Background())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()
	if hs.Role != RoleWorker || hs.SDK != "helper" {
		t.Fatalf("unexpected handshake: %+v", hs)
	}
	res, err := a.Run(context.Background(), Command{Type: MsgRun, ID: "c1", Scenario: "x"}, 2*time.Second)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.ID != "c1" || res.Status != StatusPass {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestAdapterRejectsEmptyArgv(t *testing.T) {
	a := NewAdapter(AdapterSpec{Name: "empty"})
	if _, err := a.Start(context.Background()); !errors.Is(err, ErrEmptyArgv) {
		t.Fatalf("expected ErrEmptyArgv, got %v", err)
	}
}

func TestAdapterStartTimesOutOnHungHandshake(t *testing.T) {
	a := helperAdapter("hang-handshake")
	a.handshakeTimeout = 300 * time.Millisecond
	_, err := a.Start(context.Background())
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
	a.Close() // must return promptly; a hang here fails the test
}

func TestAdapterRejectsBadProtocolVersion(t *testing.T) {
	a := helperAdapter("bad-version")
	_, err := a.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "protocol version") {
		t.Fatalf("expected protocol version error, got %v", err)
	}
	a.Close()
}

func TestAdapterRunTimesOutOnHungRun(t *testing.T) {
	a := helperAdapter("hang-run")
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()
	_, err := a.Run(context.Background(), Command{Type: MsgRun, ID: "c1"}, 300*time.Millisecond)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
}

func TestAdapterRunDetectsCrash(t *testing.T) {
	a := helperAdapter("crash-on-run")
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()
	_, err := a.Run(context.Background(), Command{Type: MsgRun, ID: "c1"}, 2*time.Second)
	if err == nil || errors.Is(err, ErrTimeout) {
		t.Fatalf("expected a crash error distinct from timeout, got %v", err)
	}
}

func TestAdapterRunRejectsIdMismatch(t *testing.T) {
	a := helperAdapter("wrong-id")
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()
	_, err := a.Run(context.Background(), Command{Type: MsgRun, ID: "c1"}, 2*time.Second)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected id mismatch error, got %v", err)
	}
}

func TestAdapterRunRejectsOverlongResult(t *testing.T) {
	a := helperAdapter("huge-line")
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()
	_, err := a.Run(context.Background(), Command{Type: MsgRun, ID: "c1"}, 2*time.Second)
	if !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("expected ErrLineTooLong, got %v", err)
	}
}

func TestAdapterToleratesNoisyStderr(t *testing.T) {
	a := helperAdapter("noisy")
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()
	res, err := a.Run(context.Background(), Command{Type: MsgRun, ID: "c1"}, 2*time.Second)
	if err != nil || res.Status != StatusPass {
		t.Fatalf("noisy child should still PASS, got res=%+v err=%v", res, err)
	}
	if a.stderr.Dropped() == 0 {
		t.Fatal("expected stderr capture to have dropped bytes past its cap")
	}
}

// A duplicate response for the current id must not corrupt the next case: the
// stale line surfaces as an id mismatch on the following Run rather than being
// silently attributed.
func TestAdapterDuplicateIdSurfacesOnNextRun(t *testing.T) {
	a := helperAdapter("dup-id")
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()
	first, err := a.Run(context.Background(), Command{Type: MsgRun, ID: "c1"}, 2*time.Second)
	if err != nil || first.Status != StatusPass {
		t.Fatalf("first run: res=%+v err=%v", first, err)
	}
	_, err = a.Run(context.Background(), Command{Type: MsgRun, ID: "c2"}, 2*time.Second)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected duplicate to surface as id mismatch, got %v", err)
	}
}

// A cancelled context must abort a running case with context.Canceled,
// distinct from a deadline timeout, and still tear the child down.
func TestAdapterRunHonorsContextCancel(t *testing.T) {
	a := helperAdapter("hang-run")
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	_, err := a.Run(ctx, Command{Type: MsgRun, ID: "c1"}, 5*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestAdapterRunsInSpecifiedDir(t *testing.T) {
	dir := t.TempDir()
	a := helperAdapter("cwd")
	a.spec.Dir = dir
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()
	res, err := a.Run(context.Background(), Command{Type: MsgRun, ID: "c1"}, 2*time.Second)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got, _ := filepath.EvalSymlinks(res.Evidence["cwd"])
	want, _ := filepath.EvalSymlinks(dir)
	if got != want {
		t.Fatalf("adapter cwd = %q, want %q", got, want)
	}
}

func TestAdapterCloseIsIdempotent(t *testing.T) {
	a := helperAdapter("well-behaved")
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}
