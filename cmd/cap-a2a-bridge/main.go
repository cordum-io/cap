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
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cordum-io/cap/v2/internal/a2abridge"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
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

func main() {
	if _, _, err := loadConfig(os.Getenv); err != nil {
		fmt.Fprintf(os.Stderr, "cap-a2a-bridge: configuration: %v\n", err)
		os.Exit(2)
	}
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
