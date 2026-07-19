package tck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// ErrTimeout is returned when an adapter does not respond within a deadline.
var ErrTimeout = errors.New("tck: adapter timed out")

// ErrEmptyArgv is returned when an adapter spec carries no executable.
var ErrEmptyArgv = errors.New("tck: adapter argv is empty")

const (
	defaultHandshakeTimeout = 10 * time.Second
	defaultGracePeriod      = 2 * time.Second
	stderrCaptureBytes      = 64 * 1024
)

// AdapterSpec describes how to launch one adapter process. Argv is executed
// directly with no shell, so no element is ever interpreted as a command line.
type AdapterSpec struct {
	Name string
	Argv []string
	Env  []string // extra environment appended to the parent's
	Dir  string   // working directory; empty inherits the parent's
}

// Adapter is a running adapter process driven over adapter-v1. A single reader
// goroutine owns the child's stdout; callers interact only through Start/Run/
// Close, which are not safe for concurrent use on one Adapter.
type Adapter struct {
	spec             AdapterSpec
	handshakeTimeout time.Duration
	gracePeriod      time.Duration

	cmd    *exec.Cmd
	inW    *os.File
	outR   *os.File
	enc    *Encoder
	stderr *boundedBuffer

	msgs     chan AdapterMessage
	shutdown chan struct{}
	readDone chan struct{}
	readErr  error // read only after <-readDone

	handshake AdapterMessage
	closed    bool
}

// NewAdapter builds an adapter for spec with default timeouts.
func NewAdapter(spec AdapterSpec) *Adapter {
	return &Adapter{
		spec:             spec,
		handshakeTimeout: defaultHandshakeTimeout,
		gracePeriod:      defaultGracePeriod,
		stderr:           newBoundedBuffer(stderrCaptureBytes),
		msgs:             make(chan AdapterMessage),
		shutdown:         make(chan struct{}),
		readDone:         make(chan struct{}),
	}
}

// Handshake returns the capability declaration captured at Start.
func (a *Adapter) Handshake() AdapterMessage { return a.handshake }

// Name returns the adapter's display label.
func (a *Adapter) Name() string { return a.spec.Name }

// Stderr returns the bounded captured stderr for diagnostics.
func (a *Adapter) Stderr() string { return a.stderr.String() }

// Start launches the process, sends hello, and validates the handshake. A
// contradictory protocol version or a missing/late handshake fails closed and
// the process is killed.
func (a *Adapter) Start(ctx context.Context) (AdapterMessage, error) {
	if len(a.spec.Argv) == 0 || a.spec.Argv[0] == "" {
		return AdapterMessage{}, ErrEmptyArgv
	}
	if err := a.spawn(ctx); err != nil {
		return AdapterMessage{}, err
	}
	go a.readLoop(NewDecoder(a.outR))
	// After the reader goroutine exists, every failure path must fully tear down
	// (close both pipes, reap the child, join the reader) - not just kill - or a
	// caller that returns on a Start error leaks outR and a zombie child.
	if err := a.enc.Encode(Command{Type: MsgHello}); err != nil {
		_ = a.Close()
		return AdapterMessage{}, err
	}
	hs, err := a.read(ctx, a.handshakeTimeout)
	if err != nil {
		_ = a.Close()
		return AdapterMessage{}, err
	}
	if hs.Type != MsgHandshake {
		_ = a.Close()
		return AdapterMessage{}, fmt.Errorf("tck: expected handshake, got %q", hs.Type)
	}
	if hs.ProtocolVersion != ProtocolVersion {
		_ = a.Close()
		return AdapterMessage{}, fmt.Errorf("tck: adapter protocol version %d, want %d", hs.ProtocolVersion, ProtocolVersion)
	}
	a.handshake = hs
	return hs, nil
}

// Run sends one scenario command and returns the adapter's single result. A
// non-result message, an id mismatch (stale or duplicate response), or an
// invalid status is a protocol violation reported as an error, never silently
// accepted.
func (a *Adapter) Run(ctx context.Context, cmd Command, deadline time.Duration) (AdapterMessage, error) {
	if err := a.enc.Encode(cmd); err != nil {
		return AdapterMessage{}, err
	}
	msg, err := a.read(ctx, deadline)
	if err != nil {
		return AdapterMessage{}, err
	}
	if msg.Type != MsgResult {
		return AdapterMessage{}, fmt.Errorf("tck: expected result, got %q", msg.Type)
	}
	if msg.ID != cmd.ID {
		return AdapterMessage{}, fmt.Errorf("tck: result id %q does not match command id %q", msg.ID, cmd.ID)
	}
	if !ValidStatus(msg.Status) {
		return AdapterMessage{}, fmt.Errorf("tck: invalid status %q", msg.Status)
	}
	return msg, nil
}

// Close asks the adapter to exit, waits a grace period, then kills it, and
// releases the reader goroutine and pipes. It is idempotent.
func (a *Adapter) Close() error {
	if a.closed {
		return nil
	}
	a.closed = true
	if a.enc != nil {
		_ = a.enc.Encode(Command{Type: MsgBye})
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- a.cmd.Wait() }()
	select {
	case <-time.After(a.gracePeriod):
		a.kill()
		<-waitDone
	case <-waitDone:
	}
	close(a.shutdown)
	if a.inW != nil {
		_ = a.inW.Close()
	}
	if a.outR != nil {
		_ = a.outR.Close()
	}
	<-a.readDone
	return nil
}

// spawn wires os.Pipe stdin/stdout directly so exec starts no copier goroutine
// for them and Wait never contends with our reader. stderr is a bounded
// io.Writer, so exec DOES run an internal copier for it; a grandchild that
// inherits the stderr handle and outlives the child would otherwise block
// Wait (and thus Close) forever. WaitDelay bounds that: after the process
// exits, Wait force-closes the stderr pipe and returns within the delay.
func (a *Adapter) spawn(ctx context.Context) error {
	inR, inW, err := os.Pipe()
	if err != nil {
		return err
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		_ = inR.Close()
		_ = inW.Close()
		return err
	}
	cmd := exec.CommandContext(ctx, a.spec.Argv[0], a.spec.Argv[1:]...)
	cmd.Stdin = inR
	cmd.Stdout = outW
	cmd.Stderr = a.stderr
	cmd.Dir = a.spec.Dir
	cmd.WaitDelay = a.gracePeriod
	if len(a.spec.Env) > 0 {
		cmd.Env = append(os.Environ(), a.spec.Env...)
	}
	if err := cmd.Start(); err != nil {
		closeAll(inR, inW, outR, outW)
		return err
	}
	_ = inR.Close()  // child holds its own copy
	_ = outW.Close() // child holds its own copy
	a.cmd, a.inW, a.outR = cmd, inW, outR
	a.enc = NewEncoder(inW)
	return nil
}

// read waits for the next message, the reader stopping, or a deadline. On
// timeout or cancellation it kills the child so Close cannot hang.
func (a *Adapter) read(ctx context.Context, d time.Duration) (AdapterMessage, error) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case m := <-a.msgs:
		return m, nil
	case <-a.readDone:
		if a.readErr != nil && !errors.Is(a.readErr, io.EOF) {
			return AdapterMessage{}, a.readErr
		}
		return AdapterMessage{}, io.ErrUnexpectedEOF
	case <-timer.C:
		a.kill()
		return AdapterMessage{}, ErrTimeout
	case <-ctx.Done():
		a.kill()
		return AdapterMessage{}, ctx.Err()
	}
}

// readLoop is the sole reader of the child's stdout. It publishes each decoded
// message and exits on decode error or shutdown, recording the terminal error.
func (a *Adapter) readLoop(dec *Decoder) {
	defer close(a.readDone)
	for {
		m, err := dec.Decode()
		if err != nil {
			a.readErr = err
			return
		}
		select {
		case a.msgs <- m:
		case <-a.shutdown:
			return
		}
	}
}

func (a *Adapter) kill() {
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
	}
	if a.inW != nil {
		_ = a.inW.Close()
	}
}

func closeAll(files ...*os.File) {
	for _, f := range files {
		_ = f.Close()
	}
}
