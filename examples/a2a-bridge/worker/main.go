// Command worker is the example A2A bridge worker: a CAP runtime.Agent that
// runs examples/a2a-bridge/turns and heartbeats so that Cordum's scheduler can
// dispatch to it. Configuration is by environment only; see ../README.md.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/examples/a2a-bridge/turns"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/cordum-io/cap/v2/sdk/go/runtime"
	"github.com/nats-io/nats.go"
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

type config struct {
	natsURL, redisURL, topic, workerID, pool, schedulerID string
	maxParallel                                           int32
	interval                                              time.Duration
	key                                                   *ecdsa.PrivateKey
	schedulerKey                                          *ecdsa.PublicKey
	unsigned                                              bool
	natsTLS                                               *tls.Config
}

func main() { os.Exit(run()) }

func run() int {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg, warnings, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "a2a-bridge worker: configuration: %v\n", err)
		return 2
	}
	for _, w := range warnings {
		log.Warn(w)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serve(ctx, cfg, log)
}

// loadConfig reads and checks every variable before any network I/O and
// reports every problem at once.
func loadConfig() (config, []string, error) {
	c := config{natsURL: os.Getenv("CAP_NATS_URL"), redisURL: os.Getenv("CAP_REDIS_URL"),
		topic: or(os.Getenv("CAP_A2A_TOPIC"), "job.a2a.demo"), workerID: or(os.Getenv("CAP_WORKER_ID"), "a2a-demo-worker"),
		pool: os.Getenv("CAP_WORKER_POOL"), schedulerID: or(os.Getenv("CAP_SCHEDULER_ID"), "cordum-scheduler"),
		unsigned: os.Getenv("CAP_A2A_DEV_UNSIGNED") == "1"}
	var errs []error
	for name, v := range map[string]string{"CAP_NATS_URL": c.natsURL, "CAP_REDIS_URL": c.redisURL} {
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", name))
		}
	}
	for name, v := range map[string]string{"CAP_WORKER_POOL": c.pool, "CAP_WORKER_ID": c.workerID, "CAP_SCHEDULER_ID": c.schedulerID} {
		if !idRe.MatchString(v) {
			errs = append(errs, fmt.Errorf("%s must match %s", name, idRe))
		}
	}
	n, err := strconv.Atoi(or(os.Getenv("CAP_WORKER_MAX_PARALLEL"), "4"))
	if err != nil || n < 1 || n > 64 {
		errs = append(errs, errors.New("CAP_WORKER_MAX_PARALLEL must be 1-64"))
	}
	c.maxParallel = int32(n)
	c.interval, err = time.ParseDuration(or(os.Getenv("CAP_WORKER_HEARTBEAT_INTERVAL"), "5s"))
	if err != nil || c.interval < time.Second || c.interval > time.Minute {
		errs = append(errs, errors.New("CAP_WORKER_HEARTBEAT_INTERVAL must be 1s-60s"))
	}
	errs = append(errs, loadKeys(&c)...)
	warnings, terrs := checkTransport(&c)
	return c, warnings, errors.Join(append(errs, terrs...)...)
}

func loadKeys(c *config) []error {
	keyFile, schedFile := os.Getenv("CAP_WORKER_SIGNING_KEY_FILE"), os.Getenv("CAP_SCHEDULER_PUBLIC_KEY_FILE")
	var errs []error
	switch {
	case c.unsigned && schedFile != "":
		errs = append(errs, errors.New("CAP_SCHEDULER_PUBLIC_KEY_FILE cannot be combined with CAP_A2A_DEV_UNSIGNED=1"))
	case !c.unsigned && (keyFile == "" || schedFile == ""):
		errs = append(errs, errors.New("CAP_WORKER_SIGNING_KEY_FILE and CAP_SCHEDULER_PUBLIC_KEY_FILE are required unless CAP_A2A_DEV_UNSIGNED=1"))
	}
	var err error
	if keyFile != "" {
		if c.key, err = loadPrivateKey(keyFile); err != nil {
			errs = append(errs, err)
		}
	}
	if schedFile != "" {
		if c.schedulerKey, err = loadPublicKey(schedFile); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// checkTransport refuses plaintext NATS or Redis and certificate checks turned
// off, unless CAP_A2A_DEV_INSECURE=1, which permits them with one warning each.
func checkTransport(c *config) ([]string, []error) {
	natsTLS, err := capsdk.NATSTLSConfigFromEnv()
	if err != nil {
		return nil, []error{fmt.Errorf("NATS TLS: %w", err)}
	}
	redisTLS, err := capsdk.RedisTLSConfigFromEnv()
	if err != nil {
		return nil, []error{fmt.Errorf("Redis TLS: %w", err)}
	}
	c.natsTLS = natsTLS
	var problems []string
	if natsTLS == nil && !allPrefixed(c.natsURL, "tls://") {
		problems = append(problems, "NATS would be plaintext (use tls:// or set NATS_TLS_CA)")
	}
	if redisTLS == nil && !allPrefixed(c.redisURL, "rediss://") {
		problems = append(problems, "Redis would be plaintext (use rediss:// or set REDIS_TLS_CA)")
	}
	if natsTLS != nil && natsTLS.InsecureSkipVerify {
		problems = append(problems, "NATS_TLS_INSECURE turns off certificate verification")
	}
	if redisTLS != nil && redisTLS.InsecureSkipVerify {
		problems = append(problems, "REDIS_TLS_INSECURE turns off certificate verification")
	}
	if os.Getenv("CAP_A2A_DEV_INSECURE") != "1" {
		if len(problems) == 0 {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("refusing insecure transport: %s; set CAP_A2A_DEV_INSECURE=1 only for local development",
			strings.Join(problems, "; "))}
	}
	warnings := make([]string, 0, len(problems)+1)
	for _, p := range problems {
		warnings = append(warnings, "CAP_A2A_DEV_INSECURE=1, development only: "+p)
	}
	if c.unsigned {
		warnings = append(warnings, "CAP_A2A_DEV_UNSIGNED=1, development only: unsigned jobs are accepted")
	}
	return warnings, nil
}

func serve(ctx context.Context, c config, log *slog.Logger) int {
	closed := make(chan struct{})
	opts := []nats.Option{nats.Name(c.workerID), nats.Timeout(5 * time.Second),
		nats.ClosedHandler(func(*nats.Conn) { close(closed) })}
	if c.natsTLS != nil {
		opts = append(opts, nats.Secure(c.natsTLS))
	}
	nc, err := nats.Connect(c.natsURL, opts...)
	if err != nil {
		log.Error("NATS connect failed", "url", redact(c.natsURL), "error", err)
		return 1
	}
	store, err := runtime.NewRedisBlobStoreWithPing(c.redisURL)
	if err != nil {
		nc.Close()
		log.Error("Redis connect failed", "url", redact(c.redisURL), "error", err)
		return 1
	}
	agent := &runtime.Agent{NATS: nc, Store: store, PrivateKey: c.key, AllowUnsigned: c.unsigned, SenderID: c.workerID, Logger: log}
	if c.schedulerKey != nil {
		agent.PublicKeys = map[string]*ecdsa.PublicKey{c.schedulerID: c.schedulerKey}
	}
	if err = turns.Register(agent, c.topic, c.workerID); err == nil {
		err = agent.Start()
	}
	if err != nil {
		nc.Close()
		_ = store.Close()
		log.Error("worker start failed", "error", err)
		return 1
	}
	hbCtx, stopBeats := context.WithCancel(ctx)
	beats := make(chan struct{})
	go func() {
		defer close(beats)
		turns.RunHeartbeats(hbCtx, nc, c.interval, func() *agentv1.BusPacket {
			return turns.HeartbeatPacket(c.workerID, c.pool, c.maxParallel, []string{c.topic})
		}, c.sign())
	}()
	log.Info("worker started", "worker_id", c.workerID, "pool", c.pool, "topic", c.topic)
	<-ctx.Done()
	stopBeats()
	<-beats
	_ = agent.Close() // also drains nc and closes store: Close acts on injected resources too
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		nc.Close()
	}
	return 0
}

func (c config) sign() func(*agentv1.BusPacket) error {
	if c.key == nil {
		return nil
	}
	return func(p *agentv1.BusPacket) error { return capsdk.SignPacket(p, c.key) }
}

func loadPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	block, err := readPEM(path)
	if err != nil {
		return nil, err
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		parsed, perr := x509.ParsePKCS8PrivateKey(block.Bytes)
		key, _ = parsed.(*ecdsa.PrivateKey)
		if perr != nil || key == nil {
			return nil, fmt.Errorf("%s: not a PKCS#8 or SEC1 EC private key", path)
		}
	}
	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("%s: not a P-256 key", path)
	}
	return key, nil
}

func loadPublicKey(path string) (*ecdsa.PublicKey, error) {
	block, err := readPEM(path)
	if err != nil {
		return nil, err
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	key, _ := parsed.(*ecdsa.PublicKey)
	if err != nil || key == nil || key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("%s: not a PKIX P-256 public key", path)
	}
	return key, nil
}

func readPEM(path string) (*pem.Block, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- operator-configured path
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("%s: no PEM block", path)
	}
	return block, nil
}

func allPrefixed(list, prefix string) bool {
	for _, u := range strings.Split(list, ",") {
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(u)), prefix) {
			return false
		}
	}
	return true
}

func redact(list string) string {
	parts := strings.Split(list, ",")
	for i, p := range parts {
		if u, err := url.Parse(strings.TrimSpace(p)); err == nil {
			parts[i] = u.Redacted()
		} else {
			parts[i] = "<unparseable>"
		}
	}
	return strings.Join(parts, ",")
}

func or(v, fallback string) string {
	if v = strings.TrimSpace(v); v != "" {
		return v
	}
	return fallback
}
