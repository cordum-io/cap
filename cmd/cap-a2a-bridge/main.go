// Command cap-a2a-bridge serves the A2A 1.0 JSON-RPC bridge into CAP jobs
// (spec/interop/a2a-mapping.md). Configuration is by environment only; see
// examples/a2a-bridge/README.md.
package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/cordum-io/cap/v2/internal/a2abridge"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

var (
	senderRe = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
	hashRe   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type config struct {
	listen, publicURL, natsURL, redisURL, topic, senderID string
	cert                                                  *tls.Certificate
	natsTLS, redisTLS                                     *tls.Config
	apiKeys                                               map[string]a2abridge.Identity
	signingKey                                            *ecdsa.PrivateKey
	resultKeys                                            map[string]*ecdsa.PublicKey
	unsigned, devInsecure                                 bool
	maxTasks                                              int
	inputTTL, cancelGrace, maxWait, shutdown              time.Duration
}

func main() { os.Exit(run(context.Background(), os.Getenv, os.Stderr)) }

// run returns 0 after a signal-triggered shutdown, 1 on a connection, start
// or serve failure, and 2 on a configuration error.
func run(ctx context.Context, getenv func(string) string, stderr io.Writer) int {
	cfg, warnings, err := loadConfig(getenv)
	if err != nil {
		fmt.Fprintf(stderr, "cap-a2a-bridge: configuration: %v\n", err)
		return 2
	}
	log := slog.New(slog.NewJSONHandler(stderr, nil))
	for _, w := range warnings {
		log.Warn(w)
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	b, code := start(cfg, log)
	if b == nil {
		return code
	}
	served := make(chan error, 1)
	go func() {
		if cfg.cert == nil {
			served <- b.srv.ListenAndServe()
		} else {
			served <- b.srv.ListenAndServeTLS("", "")
		}
	}()
	log.Info("cap-a2a-bridge started", "listen", cfg.listen, "topic", cfg.topic, "sender_id", cfg.senderID)
	select {
	case <-ctx.Done():
	case err := <-served:
		log.Error("HTTP server failed", "error", err)
		code = 1
	}
	b.shutdown(cfg.shutdown)
	return code
}

// bridge holds what shutdown releases.
type bridge struct {
	srv        *http.Server
	stopEgress func() error
	nc         *nats.Conn
	rdb        io.Closer
	cancel     context.CancelFunc // ends every request context
	once       sync.Once
}

// newBridge derives every request context from one that shutdown cancels:
// closing a connection alone does not end the context of a handler that has
// not read its body to EOF.
func newBridge(srv *http.Server, stopEgress func() error, nc *nats.Conn, rdb io.Closer) *bridge {
	base, cancel := context.WithCancel(context.Background())
	srv.BaseContext = func(net.Listener) context.Context { return base }
	return &bridge{srv: srv, stopEgress: stopEgress, nc: nc, rdb: rdb, cancel: cancel}
}

func start(c config, log *slog.Logger) (*bridge, int) {
	opts := []nats.Option{nats.Name("cap-a2a-bridge"), nats.Timeout(5 * time.Second)}
	if c.natsTLS != nil {
		opts = append(opts, nats.Secure(c.natsTLS))
	}
	nc, err := nats.Connect(c.natsURL, opts...)
	if err != nil {
		log.Error("NATS connect failed", "url", redact(c.natsURL), "error", err)
		return nil, 1
	}
	ropts, err := redis.ParseURL(c.redisURL)
	if err != nil {
		nc.Close()
		log.Error("CAP_REDIS_URL is not a valid Redis URL", "url", redact(c.redisURL))
		return nil, 2
	}
	if c.redisTLS != nil {
		ropts.TLSConfig = c.redisTLS
	}
	rdb := redis.NewClient(ropts)
	pctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = rdb.Ping(pctx).Err()
	cancel()
	if err != nil {
		nc.Close()
		_ = rdb.Close()
		log.Error("Redis ping failed", "url", redact(c.redisURL), "error", err)
		return nil, 1
	}
	rio := redisIO{c: rdb, ttl: c.inputTTL}
	ing, err := a2abridge.New(a2abridge.Config{APIKeys: c.apiKeys, Topic: c.topic, SenderID: c.senderID,
		SigningKey: c.signingKey, AllowUnsignedDev: c.unsigned, Publisher: nc, Inputs: rio, Results: rio,
		MaxTasks: c.maxTasks, ResultKeys: c.resultKeys, MaxBlockingWait: c.maxWait, CancelGrace: c.cancelGrace})
	code, stopEgress := 2, func() error { return nil }
	if err == nil {
		code = 1
		stopEgress, err = ing.Start(nc)
	}
	if err != nil {
		nc.Close()
		_ = rdb.Close()
		log.Error("bridge start failed", "error", err)
		return nil, code
	}
	mux := http.NewServeMux()
	mux.Handle("/", ing.Handler()) // Handler bounds the request body to 1 MiB
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(agentCard(c)))
	srv := &http.Server{Addr: c.listen, Handler: mux, ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second, WriteTimeout: c.maxWait + 15*time.Second,
		MaxHeaderBytes: 64 << 10, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelWarn)}
	if c.cert != nil {
		srv.TLSConfig.Certificates = []tls.Certificate{*c.cert}
	}
	return newBridge(srv, stopEgress, nc, rdb), 0
}

// shutdown releases everything within timeout and is safe to call twice. A
// request still running at the deadline has its context canceled.
func (b *bridge) shutdown(timeout time.Duration) {
	b.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		defer b.cancel()
		if b.srv.Shutdown(ctx) != nil {
			b.cancel()
			_ = b.srv.Close()
		}
		_ = b.stopEgress()
		closed := make(chan struct{})
		b.nc.SetClosedHandler(func(*nats.Conn) { close(closed) })
		if b.nc.Drain() == nil {
			select {
			case <-closed:
			case <-ctx.Done():
			}
		}
		b.nc.Close()
		_ = b.rdb.Close()
	})
}

func agentCard(c config) *a2a.AgentCard {
	version := "dev"
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version = bi.Main.Version
	}
	const bearer = a2a.SecuritySchemeName("bearer")
	modes := []string{"text/plain", "application/json"}
	return &a2a.AgentCard{Name: "cap-a2a-bridge", Version: version,
		Description:         "A2A 1.0 JSON-RPC bridge that runs each message as a CAP job on one topic.",
		SupportedInterfaces: []*a2a.AgentInterface{a2a.NewAgentInterface(c.publicURL, a2a.TransportProtocolJSONRPC)},
		DefaultInputModes:   modes, DefaultOutputModes: modes,
		Skills: []a2a.AgentSkill{{ID: c.topic, Name: c.topic, Tags: []string{"cap"},
			Description: "Runs the message as a CAP job on topic " + c.topic + "."}},
		SecuritySchemes:      a2a.NamedSecuritySchemes{bearer: a2a.HTTPAuthSecurityScheme{Scheme: "Bearer"}},
		SecurityRequirements: a2a.SecurityRequirementsOptions{{bearer: a2a.SecuritySchemeScopes{}}}}
}

// loadConfig parses and checks every variable before any network I/O and
// reports every problem at once. Errors never echo key material, file
// contents or URL credentials.
func loadConfig(getenv func(string) string) (config, []string, error) {
	env := func(name, fallback string) string {
		if v := strings.TrimSpace(getenv(name)); v != "" {
			return v
		}
		return fallback
	}
	c := config{listen: env("CAP_A2A_LISTEN", ":8443"), publicURL: env("CAP_A2A_PUBLIC_URL", ""),
		natsURL: env("CAP_NATS_URL", ""), redisURL: env("CAP_REDIS_URL", ""), topic: env("CAP_A2A_TOPIC", ""),
		senderID: env("CAP_A2A_SENDER_ID", "cap-a2a-bridge"),
		unsigned: getenv("CAP_A2A_DEV_UNSIGNED") == "1", devInsecure: getenv("CAP_A2A_DEV_INSECURE") == "1"}
	var errs []error
	for _, v := range [][2]string{{"CAP_A2A_PUBLIC_URL", c.publicURL}, {"CAP_NATS_URL", c.natsURL},
		{"CAP_REDIS_URL", c.redisURL}, {"CAP_A2A_TOPIC", c.topic}} {
		if v[1] == "" {
			errs = append(errs, fmt.Errorf("%s is required", v[0]))
		}
	}
	if strings.ContainsAny(c.topic, ", ") {
		errs = append(errs, errors.New("CAP_A2A_TOPIC must be exactly one topic"))
	}
	errs = append(errs, c.loadLimits(env)...)
	errs = append(errs, c.loadKeys(env)...)
	warnings, terrs := c.checkTransport(env)
	if c.unsigned {
		warnings = append(warnings, "CAP_A2A_DEV_UNSIGNED=1, development only: unsigned packets are accepted, "+
			"and jobs are published unsigned when no signing key is set")
	}
	return c, warnings, errors.Join(append(errs, terrs...)...)
}

func (c *config) loadLimits(env func(string, string) string) []error {
	var errs []error
	dur := func(name, fallback string, lo, hi time.Duration) time.Duration {
		d, err := time.ParseDuration(env(name, fallback))
		if err != nil || d < lo || d > hi {
			errs = append(errs, fmt.Errorf("%s must be a duration from %v to %v", name, lo, hi))
		}
		return d
	}
	c.inputTTL = dur("CAP_A2A_INPUT_TTL", "24h", time.Minute, 168*time.Hour)
	c.cancelGrace = dur("CAP_A2A_CANCEL_GRACE", "30s", time.Second, time.Minute)
	c.maxWait = dur("CAP_A2A_MAX_BLOCKING_WAIT", "60s", time.Second, 5*time.Minute)
	c.shutdown = dur("CAP_A2A_SHUTDOWN_TIMEOUT", "10s", time.Second, time.Minute)
	n, err := strconv.Atoi(env("CAP_A2A_MAX_TASKS", "0"))
	if err != nil || n < 0 {
		errs = append(errs, errors.New("CAP_A2A_MAX_TASKS must be a non-negative integer (0 means the default)"))
	}
	c.maxTasks = n
	return errs
}

func (c *config) loadKeys(env func(string, string) string) []error {
	var errs []error
	keyFile, keysDir, apiFile := env("CAP_A2A_SIGNING_KEY_FILE", ""), env("CAP_A2A_RESULT_KEYS_DIR", ""), env("CAP_A2A_API_KEYS_FILE", "")
	switch {
	case c.unsigned && keysDir != "":
		errs = append(errs, errors.New("CAP_A2A_RESULT_KEYS_DIR must be unset with CAP_A2A_DEV_UNSIGNED=1, "+
			"because result keys make the bridge drop every unsigned packet"))
	case !c.unsigned && keyFile == "":
		errs = append(errs, errors.New("CAP_A2A_SIGNING_KEY_FILE is required unless CAP_A2A_DEV_UNSIGNED=1"))
	}
	if !c.unsigned && keysDir == "" {
		errs = append(errs, errors.New("CAP_A2A_RESULT_KEYS_DIR is required unless CAP_A2A_DEV_UNSIGNED=1"))
	}
	var err error
	if keyFile != "" {
		if c.signingKey, err = loadPrivateKey(keyFile); err != nil {
			errs = append(errs, fmt.Errorf("CAP_A2A_SIGNING_KEY_FILE: %w", err))
		}
	}
	if keysDir != "" && !c.unsigned {
		if c.resultKeys, err = loadResultKeys(keysDir); err != nil {
			errs = append(errs, fmt.Errorf("CAP_A2A_RESULT_KEYS_DIR: %w", err))
		}
	}
	if apiFile == "" {
		errs = append(errs, errors.New("CAP_A2A_API_KEYS_FILE is required"))
	} else if c.apiKeys, err = loadAPIKeys(apiFile); err != nil {
		errs = append(errs, fmt.Errorf("CAP_A2A_API_KEYS_FILE: %w", err))
	}
	return errs
}

// checkTransport refuses plaintext HTTP, NATS and Redis, an http public URL
// and certificate checks turned off, unless CAP_A2A_DEV_INSECURE=1, which
// permits each of them with one warning.
func (c *config) checkTransport(env func(string, string) string) ([]string, []error) {
	var errs, problems []error
	var err error
	if c.natsTLS, err = capsdk.NATSTLSConfigFromEnv(); err != nil {
		errs = append(errs, fmt.Errorf("NATS_TLS_*: %w", err))
	}
	if c.redisTLS, err = capsdk.RedisTLSConfigFromEnv(); err != nil {
		errs = append(errs, fmt.Errorf("REDIS_TLS_*: %w", err))
	}
	certFile, keyFile := env("CAP_A2A_TLS_CERT_FILE", ""), env("CAP_A2A_TLS_KEY_FILE", "")
	switch {
	case (certFile == "") != (keyFile == ""):
		errs = append(errs, errors.New("CAP_A2A_TLS_CERT_FILE and CAP_A2A_TLS_KEY_FILE must be set together"))
	case certFile == "":
		problems = append(problems, errors.New("the HTTP listener would be plaintext: set CAP_A2A_TLS_CERT_FILE and CAP_A2A_TLS_KEY_FILE"))
	default:
		if cert, err := tls.LoadX509KeyPair(certFile, keyFile); err != nil {
			errs = append(errs, fmt.Errorf("CAP_A2A_TLS_CERT_FILE and CAP_A2A_TLS_KEY_FILE: %w", err))
		} else {
			c.cert = &cert
		}
	}
	if c.natsURL != "" && c.natsTLS == nil && !allPrefixed(c.natsURL, "tls://") {
		problems = append(problems, errors.New("NATS would be plaintext: use tls:// or NATS_TLS_CA"))
	}
	if c.redisURL != "" && c.redisTLS == nil && !allPrefixed(c.redisURL, "rediss://") {
		problems = append(problems, errors.New("Redis would be plaintext: use rediss:// or REDIS_TLS_CA"))
	}
	if u, err := url.Parse(c.publicURL); c.publicURL != "" && (err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http")) {
		errs = append(errs, errors.New("CAP_A2A_PUBLIC_URL must be an absolute https URL"))
	} else if c.publicURL != "" && u.Scheme == "http" {
		problems = append(problems, errors.New("the public URL is not https: use https"))
	}
	for i, cfg := range []*tls.Config{c.natsTLS, c.redisTLS} {
		if cfg != nil && cfg.InsecureSkipVerify {
			problems = append(problems, fmt.Errorf("%s_TLS_INSECURE turns off certificate verification: unset it", []string{"NATS", "REDIS"}[i]))
		}
	}
	if len(problems) > 0 && !c.devInsecure {
		errs = append(errs, fmt.Errorf("refusing insecure transport: %w; set CAP_A2A_DEV_INSECURE=1 only for local development",
			errors.Join(problems...)))
		return nil, errs
	}
	warnings := make([]string, 0, len(problems))
	for _, p := range problems {
		warnings = append(warnings, "CAP_A2A_DEV_INSECURE=1, development only: "+p.Error())
	}
	return warnings, errs
}

type apiKeyEntry struct {
	SHA256    string `json:"sha256"`
	Tenant    string `json:"tenant"`
	Principal string `json:"principal"`
}

// loadAPIKeys reads a JSON array of key hashes; the file never holds a key.
func loadAPIKeys(path string) (map[string]a2abridge.Identity, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- operator-configured path
	if err != nil {
		return nil, err
	}
	var entries []apiKeyEntry
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&entries) != nil || len(entries) == 0 {
		return nil, errors.New(`want a non-empty JSON array of {"sha256","tenant","principal"}`)
	}
	keys := make(map[string]a2abridge.Identity, len(entries))
	for i, e := range entries {
		if !hashRe.MatchString(e.SHA256) {
			return nil, fmt.Errorf("entry %d: sha256 must be 64 lowercase hex characters", i)
		}
		if _, dup := keys[e.SHA256]; dup {
			return nil, fmt.Errorf("entry %d: duplicate sha256", i)
		}
		if e.Tenant == "" || e.Principal == "" {
			return nil, fmt.Errorf("entry %d: tenant and principal are required", i)
		}
		keys[e.SHA256] = a2abridge.Identity{Tenant: e.Tenant, Principal: e.Principal}
	}
	return keys, nil
}

// loadResultKeys reads one <sender_id>.pem public key per trusted sender.
func loadResultKeys(dir string) (map[string]*ecdsa.PublicKey, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	keys := map[string]*ecdsa.PublicKey{}
	for _, e := range entries {
		sender, ok := strings.CutSuffix(e.Name(), ".pem")
		if !ok || e.IsDir() {
			continue
		}
		if !senderRe.MatchString(sender) {
			return nil, fmt.Errorf("%q is not a valid sender id (%s)", sender, senderRe)
		}
		if keys[sender], err = loadPublicKey(filepath.Join(dir, e.Name())); err != nil {
			return nil, err
		}
	}
	if len(keys) == 0 {
		return nil, errors.New("no <sender_id>.pem public key found")
	}
	return keys, nil
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
		return nil, err
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

// redisCmds is the part of *redis.Client that redisIO uses. It has no Get,
// so a result is never read unbounded.
type redisCmds interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) *redis.StatusCmd
	GetRange(ctx context.Context, key string, start, end int64) *redis.StringCmd
}

var _ redisCmds = (*redis.Client)(nil)

// redisIO is the bridge's a2abridge.InputWriter and a2abridge.ResultReader.
type redisIO struct {
	c   redisCmds
	ttl time.Duration
}

func (r redisIO) PutTurnInput(ctx context.Context, key string, data []byte) error {
	if !strings.HasPrefix(key, "ctx:") {
		return fmt.Errorf("redis: refusing to write %q", key)
	}
	return r.c.Set(ctx, key, data, r.ttl).Err()
}

// ReadResult reads at most maxBytes+1 bytes with one GETRANGE, so an
// oversized result is refused without being read in full.
func (r redisIO) ReadResult(ctx context.Context, key string, maxBytes int) ([]byte, error) {
	if !strings.HasPrefix(key, "res:") || maxBytes <= 0 {
		return nil, fmt.Errorf("redis: refusing to read %q with limit %d", key, maxBytes)
	}
	v, err := r.c.GetRange(ctx, key, 0, int64(maxBytes)).Bytes()
	switch {
	case err != nil:
		return nil, err
	case len(v) == 0:
		return nil, fmt.Errorf("redis: %s is missing or empty", key)
	case len(v) > maxBytes:
		return nil, fmt.Errorf("redis: %s exceeds %d bytes", key, maxBytes)
	}
	return v, nil
}
