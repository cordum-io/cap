// Command cap-reference-scheduler runs the Apache-2.0 CAP reference scheduler
// (internal/referencescheduler) as a standalone control plane over NATS.
//
// Usage: cap-reference-scheduler -config scheduler.json [flags]
//
// It exits 0 after SIGINT or SIGTERM (os.Interrupt on Windows) drains the
// runtime, 1 when the NATS transport fails, and 2 for a usage or
// configuration error. See README.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/cordum-io/cap/v2/internal/referencescheduler"
)

const (
	exitOK         = 0
	exitTransport  = 1
	exitConfig     = 2
	tickEvery      = 250 * time.Millisecond
	maxConfigBytes = 1 << 20
)

var (
	errUsage      = errors.New("usage: cap-reference-scheduler -config FILE [flags]")
	errConfigFile = errors.New("config file is unreadable or not a valid configuration")
	errKeyFile    = errors.New("private key file is missing")
	errKeySymlink = errors.New("private key file is a symlink")
	errKeyMode    = errors.New("private key file is readable or writable by group or others")
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stderr)
	stop()
	os.Exit(code)
}

// run serves until ctx is cancelled. It writes only fixed messages, sentinel
// errors and ids to stderr, never key material or packet payloads.
func run(ctx context.Context, args []string, stderr io.Writer) int {
	cfg, err := loadConfig(args, stderr)
	if err == nil {
		var resolved *referencescheduler.ResolvedConfig
		if resolved, err = cfg.Resolve(); err == nil {
			return serve(ctx, resolved, stderr)
		}
	}
	fmt.Fprintf(stderr, "cap-reference-scheduler: config error: %v\n", err)
	return exitConfig
}

func serve(ctx context.Context, cfg *referencescheduler.ResolvedConfig, stderr io.Writer) int {
	logger := slog.New(slog.NewTextHandler(stderr, nil))
	rt, err := referencescheduler.NewRuntime(referencescheduler.RuntimeOptions{
		Config: cfg, Now: time.Now, TickEvery: tickEvery, Logger: logger,
	})
	if err != nil {
		fmt.Fprintf(stderr, "cap-reference-scheduler: config error: %v\n", err)
		return exitConfig
	}
	go func() {
		select {
		case <-rt.Ready():
			logger.Info("cap-reference-scheduler: ready", "scheduler_id", cfg.SchedulerID)
		case <-ctx.Done():
		}
	}()
	if err := rt.Run(ctx); err != nil {
		fmt.Fprintf(stderr, "cap-reference-scheduler: %v\n", err)
		return exitTransport
	}
	logger.Info("cap-reference-scheduler: drained")
	return exitOK
}

// loadConfig reads the optional -config file, then applies the flags that
// were set on top of it, and checks every private key file it names.
func loadConfig(args []string, stderr io.Writer) (referencescheduler.RuntimeConfig, error) {
	var cfg, set referencescheduler.RuntimeConfig
	fs := flag.NewFlagSet("cap-reference-scheduler", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "JSON configuration file (see README.md)")
	fs.StringVar(&set.NATSURL, "nats-url", "", "NATS endpoint, tls://host:port (nats:// only with -dev-plaintext-loopback)")
	fs.StringVar(&set.TLSCAFile, "tls-ca", "", "PEM CA bundle that signs the NATS server certificate")
	fs.StringVar(&set.TLSCertFile, "tls-cert", "", "PEM client certificate for NATS")
	fs.StringVar(&set.TLSKeyFile, "tls-key", "", "PEM private key of the NATS client certificate")
	fs.BoolVar(&set.DevPlaintextLoopback, "dev-plaintext-loopback", false, "allow plaintext NATS to a loopback host (development only)")
	fs.StringVar(&set.SchedulerID, "scheduler-id", "", "scheduler identity, the sender of dispatches and terminals")
	fs.StringVar(&set.SigningKeyID, "signing-key-id", "", "key id of the production signing key")
	fs.StringVar(&set.SigningKeyFile, "signing-key-file", "", "production signing P-256 private key PEM")
	fs.StringVar(&set.TrustKeyID, "trust-key-id", "", "key id of the worker-trust signing key")
	fs.StringVar(&set.TrustKeyFile, "trust-key-file", "", "worker-trust signing P-256 private key PEM")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return cfg, errUsage
	}
	if *path != "" {
		var err error
		if cfg, err = readConfigFile(*path); err != nil {
			return cfg, err
		}
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "nats-url":
			cfg.NATSURL = set.NATSURL
		case "tls-ca":
			cfg.TLSCAFile = set.TLSCAFile
		case "tls-cert":
			cfg.TLSCertFile = set.TLSCertFile
		case "tls-key":
			cfg.TLSKeyFile = set.TLSKeyFile
		case "dev-plaintext-loopback":
			cfg.DevPlaintextLoopback = set.DevPlaintextLoopback
		case "scheduler-id":
			cfg.SchedulerID = set.SchedulerID
		case "signing-key-id":
			cfg.SigningKeyID = set.SigningKeyID
		case "signing-key-file":
			cfg.SigningKeyFile = set.SigningKeyFile
		case "trust-key-id":
			cfg.TrustKeyID = set.TrustKeyID
		case "trust-key-file":
			cfg.TrustKeyFile = set.TrustKeyFile
		}
	})
	for name, p := range map[string]string{"signing-key-file": cfg.SigningKeyFile, "trust-key-file": cfg.TrustKeyFile, "tls-key": cfg.TLSKeyFile} {
		if err := checkPrivateKeyFile(name, p); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

// duration is a time.Duration written as a string such as "30s".
type duration time.Duration

func (d *duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := time.ParseDuration(s)
	*d = duration(v)
	return err
}

// fileLimits and fileConfig are RuntimeConfig with its durations as strings.
type fileLimits struct {
	referencescheduler.Limits
	DispatchTimeout, RunTimeout, MaxRunTimeout duration
}

type fileConfig struct {
	referencescheduler.RuntimeConfig
	Limits                   fileLimits
	ChallengeTTL, SessionTTL duration
}

// readConfigFile decodes one JSON object whose keys are RuntimeConfig field
// names. Unknown fields and trailing data are refused, and the error never
// quotes the file.
func readConfigFile(path string) (referencescheduler.RuntimeConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return referencescheduler.RuntimeConfig{}, errConfigFile
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, maxConfigBytes))
	dec.DisallowUnknownFields()
	var fc fileConfig
	if err := dec.Decode(&fc); err != nil || dec.More() {
		return referencescheduler.RuntimeConfig{}, errConfigFile
	}
	c := fc.RuntimeConfig
	c.Limits = fc.Limits.Limits
	c.Limits.DispatchTimeout = time.Duration(fc.Limits.DispatchTimeout)
	c.Limits.RunTimeout = time.Duration(fc.Limits.RunTimeout)
	c.Limits.MaxRunTimeout = time.Duration(fc.Limits.MaxRunTimeout)
	c.ChallengeTTL, c.SessionTTL = time.Duration(fc.ChallengeTTL), time.Duration(fc.SessionTTL)
	return c, nil
}

// checkPrivateKeyFile refuses a private key file that is a symlink or, on
// POSIX, one that group or others may read or write. Resolve reports an
// unset path.
func checkPrivateKeyFile(name, path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Lstat(path)
	switch {
	case err != nil:
		return fmt.Errorf("%s: %w", name, errKeyFile)
	case info.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("%s: %w", name, errKeySymlink)
	case runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0:
		return fmt.Errorf("%s: %w", name, errKeyMode)
	}
	return nil
}
