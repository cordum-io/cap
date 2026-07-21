package codegen

import (
	"regexp"
	"strings"
	"testing"
)

// The hermetic codegen image only lives up to the docs' "digest-pinned,
// checksum-locked" claim if the pins are actually committed AND enforced. These
// tests are that enforcement: they run in .github/workflows/ci.yml (`go test
// ./...`) and sdk-gates.yml (`go test ./tools/codegen/...`), so a future edit
// that drops the base digest, a download checksum, or reintroduces a mutable
// installer turns the tree red instead of silently un-pinning the supply chain.

// TestDockerfileBaseImageIsDigestPinned proves the base image is pinned by an
// immutable digest, not merely a movable tag.
func TestDockerfileBaseImageIsDigestPinned(t *testing.T) {
	body := readRepoFile(t, "tools", "codegen", "Dockerfile")
	if !regexp.MustCompile(`(?m)^FROM\s+\S+@sha256:[0-9a-f]{64}\b`).MatchString(body) {
		t.Error("Dockerfile FROM must pin the base image by an immutable @sha256 digest")
	}
}

// TestDockerfileHasNoMutableInstallers proves the supply chain carries no
// pipe-to-shell installer whose content can change under a fixed URL. Only
// executable lines are inspected, so documentation that names the anti-pattern
// (`setup_20.x | bash`) does not itself trip the check, and the `sha256sum`
// pipeline is not mistaken for a shell pipe.
func TestDockerfileHasNoMutableInstallers(t *testing.T) {
	code := dockerfileCode(t)
	// A pipe into a shell interpreter: `| bash`, `| sh` - but not `| sha256sum`.
	if loc := regexp.MustCompile(`\|\s*(?:ba)?sh\b`).FindString(code); loc != "" {
		t.Errorf("Dockerfile must not pipe a download into a shell (found %q); pin the artifact by checksum instead", loc)
	}
}

// TestWindowsCodegenWrappersExercisedInCI keeps the Windows path from silently
// regressing: mutation_check.ps1 must exist and drive the same container build +
// network-disabled probe as the bash wrapper, and the windows-latest CI job that
// parses it must stay wired up. The finding this closes was that the Windows
// wrapper was missing and its path never exercised.
func TestWindowsCodegenWrappersExercisedInCI(t *testing.T) {
	ps := readRepoFile(t, "tools", "codegen", "mutation_check.ps1")
	for _, want := range []string{"docker build", "mutation_probe.sh", "--network=none"} {
		if !strings.Contains(ps, want) {
			t.Errorf("mutation_check.ps1 must contain %q", want)
		}
	}
	wf := readRepoFile(t, ".github", "workflows", "sdk-gates.yml")
	for _, want := range []string{
		"windows-codegen-wrappers:",
		"runs-on: windows-latest",
		"tools/codegen/mutation_check.ps1",
	} {
		if !strings.Contains(wf, want) {
			t.Errorf("sdk-gates.yml must exercise the Windows wrappers on windows-latest: missing %q", want)
		}
	}
}

// dockerfileCode returns the Dockerfile with # comment lines removed, so tests
// assert on what the image actually runs rather than on documentation.
func dockerfileCode(t *testing.T) string {
	t.Helper()
	var code []string
	for _, line := range strings.Split(readRepoFile(t, "tools", "codegen", "Dockerfile"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		code = append(code, line)
	}
	return strings.Join(code, "\n")
}

// TestDockerfileChecksumsEveryDownload proves every direct archive/tarball
// download is verified against a committed sha256, and that enough checksums are
// present to cover the per-arch protoc, protobuf-javascript, and node inputs.
func TestDockerfileChecksumsEveryDownload(t *testing.T) {
	body := readRepoFile(t, "tools", "codegen", "Dockerfile")

	downloads := regexp.MustCompile(`https://[^\s"']+\.(?:zip|tar\.xz)`).FindAllString(body, -1)
	if len(downloads) < 3 {
		t.Fatalf("expected the pinned archive downloads (protoc, protobuf-javascript, node), found %d", len(downloads))
	}
	// Each downloaded archive must be checked with `sha256sum -c`. The protoc
	// fetch is a helper called per version, so a single check literal covers its
	// two downloads; protobuf-javascript and node each add one.
	if got := strings.Count(body, "sha256sum -c"); got < 3 {
		t.Errorf("every downloaded archive must be verified with `sha256sum -c`; found %d checks", got)
	}

	// The committed 64-hex checksums: the base digest plus per-arch archives
	// (protoc 21.12 x2, protoc 33.2 x2, protobuf-javascript x2, node x2).
	shas := regexp.MustCompile(`\b[0-9a-f]{64}\b`).FindAllString(body, -1)
	if len(shas) < 9 {
		t.Errorf("expected >=9 committed sha256 pins (base digest + per-arch archives), found %d", len(shas))
	}
}
