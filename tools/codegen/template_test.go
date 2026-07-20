package codegen

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The hermetic container is only a faithful mirror of the networked generator
// while the two buf templates pin the same plugin versions. buf emits identical
// bytes for a remote and a local plugin of the same version -- and different
// bytes as soon as the versions diverge -- so a silent drift between
// //buf.gen.yaml and tools/codegen/buf.gen.offline.yaml would turn the offline
// run into a second, competing generator. These tests are the lock.

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, parts...)...))
	if err != nil {
		t.Fatalf("read %v: %v", parts, err)
	}
	return string(body)
}

var remotePlugin = regexp.MustCompile(`buf\.build/[\w.-]+/([\w.-]+):(v[\w.-]+)`)

// remotePins maps plugin name -> version for every remote plugin in a template.
func remotePins(text string) map[string]string {
	pins := map[string]string{}
	for _, match := range remotePlugin.FindAllStringSubmatch(text, -1) {
		pins[match[1]] = match[2]
	}
	return pins
}

// Every remote plugin in //buf.gen.yaml must be named, at the same version, in
// the offline template -- in a comment or a protoc_path is fine, but the number
// must be there, so a version bump that misses the mirror fails here instead of
// silently producing different bytes in CI.
func TestOfflineTemplateMirrorsRemoteTemplate(t *testing.T) {
	online := readRepoFile(t, "buf.gen.yaml")
	offline := readRepoFile(t, "tools", "codegen", "buf.gen.offline.yaml")
	pins := remotePins(online)
	if len(pins) == 0 {
		t.Fatal("no remote plugins found in buf.gen.yaml; the parity check would be vacuous")
	}
	names := make([]string, 0, len(pins))
	for name := range pins {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.TrimPrefix(pins[name], "v")
		if !strings.Contains(offline, version) {
			t.Errorf("buf.gen.yaml pins %s at v%s but the offline template never mentions that version", name, version)
		}
	}
}

// The offline template must not reach the network: a `remote:` entry there
// would silently reintroduce the buf.build dependency the container exists to
// remove, and would fail only when the build host is offline.
func TestOfflineTemplateDeclaresNoRemotePlugins(t *testing.T) {
	offline := readRepoFile(t, "tools", "codegen", "buf.gen.offline.yaml")
	for _, line := range strings.Split(offline, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- remote:") || strings.Contains(trimmed, "remote: buf.build") {
			t.Errorf("offline template must resolve plugins locally, found: %s", trimmed)
		}
	}
}

// The container ENTRYPOINT must exist and must run the canonical generator.
// This is the "missing generator" failure the manifest check cannot see: a
// manifest of correct hashes stays green with no generator behind it at all.
func TestContainerEntrypointRunsCanonicalGenerator(t *testing.T) {
	dockerfile := readRepoFile(t, "tools", "codegen", "Dockerfile")
	entrypoint := "/src/tools/codegen/generate.sh"
	if !strings.Contains(dockerfile, entrypoint) {
		t.Fatalf("Dockerfile must declare %s as its ENTRYPOINT", entrypoint)
	}
	script := readRepoFile(t, "tools", "codegen", "generate.sh")
	if !strings.Contains(script, "tools/proto_codegen.py") {
		t.Error("generate.sh must invoke the canonical generator, not a parallel pipeline")
	}
	if !strings.Contains(script, "--offline") {
		t.Error("generate.sh must select the hermetic toolchain with --offline")
	}
	// A CRLF shebang makes the kernel look for "/usr/bin/env bash\r".
	if strings.Contains(script, "\r\n") {
		t.Error("generate.sh must be LF-only; see the *.sh eol=lf rule in .gitattributes")
	}
}

// The hermetic gate is only worth anything while CI runs it. This mirrors
// TestCrossLanguageMatrixIsEnforcedInCI: an opt-in gate that can be silently
// dropped from the workflow is not a gate.
func TestHermeticCodegenIsEnforcedInCI(t *testing.T) {
	workflow := readRepoFile(t, ".github", "workflows", "sdk-gates.yml")
	for _, want := range []string{
		"hermetic-codegen:",
		"tools/codegen/codegen.sh --check",
		"tools/codegen/mutation_check.sh",
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("sdk-gates.yml must contain %q so hermetic codegen stays enforced", want)
		}
	}
}

// codegen.sh --check must run the generator, not just re-hash the tree.
func TestCheckModeRunsTheGenerator(t *testing.T) {
	for _, name := range []string{"codegen.sh", "codegen.ps1"} {
		wrapper := readRepoFile(t, "tools", "codegen", name)
		if !strings.Contains(wrapper, "docker run") {
			t.Errorf("%s must run the pinned container", name)
		}
		checkSection := wrapper
		if index := strings.Index(wrapper, "verifying manifest"); index > 0 {
			checkSection = wrapper[:index]
		}
		if strings.Count(checkSection, "docker run") < 2 {
			t.Errorf("%s must run the container in BOTH check and write modes; "+
				"a manifest-only --check stays green when the generator is broken or absent", name)
		}
	}
}
