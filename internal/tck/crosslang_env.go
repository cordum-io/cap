package tck

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// CrossLangEnv owns the installed SDK artifacts and the fixtures they exchange.
// Building the artifacts costs tens of seconds, so one environment is shared by
// every test in the package.
type CrossLangEnv struct {
	root      string
	workDir   string
	corpus    corpusFile
	corpusRaw []byte
	drivers   map[string]driverSpec
	keys      map[string]keyPair
	wrongKey  keyPair

	once      sync.Once
	fixtures  []Fixture
	consumers []Consumer
	negatives []NegativeResult
	runErr    error
}

type driverSpec struct {
	Argv     []string `json:"argv"`
	Artifact string   `json:"artifact"`
}

var (
	sharedEnvOnce sync.Once
	sharedEnv     *CrossLangEnv
	sharedEnvErr  error
)

// NewCrossLangEnv builds every stable SDK's installed artifact once per test
// binary. A missing toolchain is a hard failure once the gate is enabled: the
// matrix must never report success because a language was quietly absent.
func NewCrossLangEnv(t *testing.T) *CrossLangEnv {
	t.Helper()
	if os.Getenv(MatrixEnvVar) == "" {
		t.Skipf("cross-language matrix disabled: set %s=1 (requires go, python, node)", MatrixEnvVar)
	}
	sharedEnvOnce.Do(func() { sharedEnv, sharedEnvErr = buildCrossLangEnv() })
	if sharedEnvErr != nil {
		t.Fatalf("build cross-language environment: %v", sharedEnvErr)
	}
	return sharedEnv
}

func buildCrossLangEnv() (*CrossLangEnv, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	corpus, raw, err := loadCorpus(root)
	if err != nil {
		return nil, err
	}
	workDir, err := os.MkdirTemp("", matrixWorkDirPrefix)
	if err != nil {
		return nil, err
	}
	// The artifact tree is ~84 MB (wheel venv, packed tarball, module proxy).
	// Every failure path below must drop it, or a broken toolchain silently
	// fills the disk one run at a time.
	built := false
	defer func() {
		if !built {
			removeMatrixWorkDir(workDir)
		}
	}()

	drivers, err := buildArtifacts(root, workDir)
	if err != nil {
		return nil, err
	}
	env := &CrossLangEnv{
		root: root, workDir: workDir, corpus: corpus, corpusRaw: raw,
		drivers: drivers, keys: map[string]keyPair{},
	}
	for _, sdk := range StableSDKs {
		if _, ok := drivers[sdk]; !ok {
			return nil, fmt.Errorf("no driver built for stable SDK %q", sdk)
		}
		if env.keys[sdk], err = newKeyPair(); err != nil {
			return nil, err
		}
	}
	if env.wrongKey, err = newKeyPair(); err != nil {
		return nil, err
	}
	built = true
	return env, nil
}

// matrixWorkDirPrefix names the shared artifact directory.
const matrixWorkDirPrefix = "cap-tck-matrix-"

// removeMatrixWorkDir deletes a matrix artifact directory. It refuses any path
// whose base name lacks the prefix: CrossLangEnv also carries the repository
// root, and a future refactor that passes the wrong field here must not be able
// to delete the working tree.
func removeMatrixWorkDir(dir string) {
	if dir == "" || !strings.HasPrefix(filepath.Base(dir), matrixWorkDirPrefix) {
		return
	}
	_ = os.RemoveAll(dir)
}

// CleanupCrossLangEnv drops the shared artifact tree. Callable when no
// environment was ever built (the matrix gate is off), in which case it is a
// no-op.
func CleanupCrossLangEnv() {
	if sharedEnv != nil {
		removeMatrixWorkDir(sharedEnv.workDir)
	}
}

// buildArtifacts shells out to the packaging builder, which owns every
// per-language detail (module proxy, wheel venv, packed tarball).
func buildArtifacts(root, workDir string) (map[string]driverSpec, error) {
	script := filepath.Join(root, "tools", "tck", "matrix", "build_artifacts.py")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, pythonExe(), script, "--root", root, "--out", workDir)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("build_artifacts.py: %w\n%s", err, stderr.String())
	}
	raw, err := os.ReadFile(filepath.Join(workDir, "drivers.json"))
	if err != nil {
		return nil, err
	}
	drivers := map[string]driverSpec{}
	if err := json.Unmarshal(raw, &drivers); err != nil {
		return nil, fmt.Errorf("parse drivers.json: %w", err)
	}
	return drivers, nil
}

func pythonExe() string {
	if override := os.Getenv("CAP_TCK_PYTHON"); override != "" {
		return override
	}
	return "python"
}

// Artifacts reports which packaged artifact backs each SDK driver, so evidence
// can name the wheel and tarball rather than claiming "installed".
func (e *CrossLangEnv) Artifacts() map[string]string {
	out := map[string]string{}
	for sdk, spec := range e.drivers {
		out[sdk] = spec.Artifact
	}
	return out
}

// Run produces fixtures from every SDK and drives them through every SDK,
// exactly once per test binary. It returns the fixtures and the consumers that
// RunMatrix needs to compute the full producer->consumer matrix.
func (e *CrossLangEnv) Run(t *testing.T) ([]Fixture, []Consumer) {
	t.Helper()
	e.once.Do(func() { e.runErr = e.exchange() })
	if e.runErr != nil {
		t.Fatalf("run cross-language matrix: %v", e.runErr)
	}
	return e.fixtures, e.consumers
}

// NegativeResults returns every consumer verdict for a deliberately corrupted
// fixture or a wrong verification key.
func (e *CrossLangEnv) NegativeResults() []NegativeResult { return e.negatives }

func (e *CrossLangEnv) exchange() error {
	now := time.Now()
	produced, err := e.produceAll(now)
	if err != nil {
		return err
	}
	jobs, kinds := e.buildJobs(produced)
	for _, sdk := range StableSDKs {
		verdicts, err := e.consume(sdk, jobs)
		if err != nil {
			return err
		}
		consumer, negatives, err := e.collect(sdk, verdicts, jobs, kinds)
		if err != nil {
			return err
		}
		e.consumers = append(e.consumers, consumer)
		e.negatives = append(e.negatives, negatives...)
	}
	e.fixtures = produced
	return nil
}

func (e *CrossLangEnv) produceAll(now time.Time) ([]Fixture, error) {
	var out []Fixture
	for _, sdk := range StableSDKs {
		request := produceRequest{
			SDK: sdk, Corpus: e.corpusRaw, KeyID: keyIDFor(sdk),
			PrivateKeyPEM: e.keys[sdk].privatePEM,
			CreatedAtUnix: now.Unix(),
			ExpiresAtUnix: now.Add(signatureLifetime).Unix(),
		}
		var response produceResponse
		if err := e.invoke(sdk, "produce", request, &response); err != nil {
			return nil, err
		}
		if len(response.Fixtures) != len(e.corpus.Cases) {
			return nil, fmt.Errorf("producer %s emitted %d fixtures, want %d",
				sdk, len(response.Fixtures), len(e.corpus.Cases))
		}
		for _, f := range response.Fixtures {
			wire, err := base64.StdEncoding.DecodeString(f.Wire)
			if err != nil {
				return nil, fmt.Errorf("producer %s case %s: %w", sdk, f.Case, err)
			}
			publicKey, err := base64.StdEncoding.DecodeString(e.keys[sdk].publicDERB64)
			if err != nil {
				return nil, err
			}
			out = append(out, Fixture{
				Producer: sdk, Case: f.Case, Wire: wire,
				NormalizedDigest: f.NormalizedDigest, PreimageDigest: f.PreimageDigest,
				KeyID: f.KeyID, PublicKey: publicKey,
			})
		}
	}
	return out, nil
}

func keyIDFor(sdk string) string { return "k-" + sdk + "-1" }

func (e *CrossLangEnv) invoke(sdk, mode string, request, response any) error {
	spec := e.drivers[sdk]
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, spec.Argv[0], append(append([]string{}, spec.Argv[1:]...), mode)...)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s driver %s: %w\n%s", sdk, mode, err, stderr.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), response); err != nil {
		return fmt.Errorf("%s driver %s: parse response: %w", sdk, mode, err)
	}
	return nil
}

func wireKey(wire, publicKey []byte) string {
	sum := sha256.Sum256(wire)
	key := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:]) + "|" + hex.EncodeToString(key[:])
}
