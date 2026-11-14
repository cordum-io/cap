package governance

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"

	"github.com/cordum-io/cap/v2/internal/tck/publicreport"
)

// CAP-owned names from docs/adoption/framework-adapters-design.md section 5,
// excluded from independent implementation evidence under AD7.
var capOwnedArtifacts = map[string]bool{
	"cap-reference-scheduler": true, "cap-reference-submit": true,
	"cap-adapter-langgraph": true, "cap-adapter-crewai": true,
	"cap-tck-reference-adapter": true, "cap-framework-tck": true,
	"cap-sdk-go": true, "cap-sdk-python": true, "cap-sdk-node": true,
}

const maxEvidenceBytes = 1 << 20

var evidenceDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

func verifyAttestation(root *os.Root, a *TCKAttestation) (artifact, why string) {
	if !evidenceDigest.MatchString(a.ReportDigest) {
		return "", "reportDigest is not 64 lowercase hex — UNKNOWN"
	}
	b, why := readEvidence(root, a.ReportDigest+".json")
	if why != "" {
		return "", why
	}
	if fmt.Sprintf("%x", sha256.Sum256(b)) != a.ReportDigest {
		return "", "evidence SHA-256 does not match reportDigest — UNKNOWN"
	}
	r, err := publicreport.Validate(b)
	if err != nil {
		return "", "evidence is not a valid public report v1 — UNKNOWN"
	}
	if r.Verdict != publicreport.VerdictPass {
		return "", "public report verdict " + r.Verdict + ", not PASS — UNKNOWN"
	}
	if r.Profile != a.Profile {
		return "", "public report profile does not match the attestation — UNKNOWN"
	}
	if r.TCKToolVersion != a.TCKVersion {
		return "", "public report TCK version does not match the attestation — UNKNOWN"
	}
	return r.Implementation.ArtifactName, ""
}

func readEvidence(root *os.Root, name string) ([]byte, string) {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "evidence file missing — UNKNOWN"
	}
	if err != nil {
		return nil, "evidence unreadable — UNKNOWN"
	}
	if !info.Mode().IsRegular() {
		return nil, "evidence is not a regular file — UNKNOWN"
	}
	if info.Size() > maxEvidenceBytes {
		return nil, "evidence file over 1 MiB — UNKNOWN"
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, "evidence unreadable — UNKNOWN"
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, "evidence unreadable — UNKNOWN"
	}
	if !info.Mode().IsRegular() {
		return nil, "evidence is not a regular file — UNKNOWN"
	}
	b, err := io.ReadAll(io.LimitReader(f, maxEvidenceBytes+1))
	if len(b) > maxEvidenceBytes {
		return nil, "evidence file over 1 MiB — UNKNOWN"
	}
	if err != nil {
		return nil, "evidence unreadable — UNKNOWN"
	}
	return b, ""
}

// evidenceSet opens the root lazily, and only once per evaluation. It records
// only reports that counted, so rejected evidence cannot reserve an identity.
type evidenceSet struct {
	dir       string
	root      *os.Root
	attempted bool
	digests   map[string]string
	artifacts map[string]string
}

func (e *evidenceSet) close() {
	if e.root != nil {
		e.root.Close()
	}
}

func (e *evidenceSet) check(i Implementation) string {
	if e.dir == "" {
		return "no evidence directory — UNKNOWN"
	}
	if !e.attempted {
		e.attempted = true
		var err error
		e.root, err = os.OpenRoot(e.dir)
		if err != nil {
			return "evidence directory unavailable — UNKNOWN"
		}
	}
	if e.root == nil {
		return "evidence directory unavailable — UNKNOWN"
	}
	artifact, why := verifyAttestation(e.root, i.Attestation)
	if why != "" {
		return why
	}
	if capOwnedArtifacts[artifact] {
		return "CAP-owned artifact " + artifact + " — excluded"
	}
	prior, exists := e.digests[i.Attestation.ReportDigest]
	if !exists {
		prior, exists = e.artifacts[artifact]
	}
	if exists {
		return "duplicate of " + prior + " (same report or artifact) — excluded"
	}
	e.digests[i.Attestation.ReportDigest] = i.ID
	e.artifacts[artifact] = i.ID
	return ""
}
