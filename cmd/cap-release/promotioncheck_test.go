package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveLocalTagCommit_RequiresAncestorOfHead(t *testing.T) {
	repo := t.TempDir()
	gitForTest(t, repo, "init", "-b", "main")
	gitForTest(t, repo, "config", "user.email", "release-test@example.com")
	gitForTest(t, repo, "config", "user.name", "Release Test")
	writeGitFixture(t, repo, "one")
	first := gitForTest(t, repo, "rev-parse", "HEAD")
	writeGitFixture(t, repo, "two")
	second := gitForTest(t, repo, "rev-parse", "HEAD")
	gitForTest(t, repo, "tag", "-a", "v2.15.0", "-m", "release fixture")
	tagObject := gitForTest(t, repo, "rev-parse", "refs/tags/v2.15.0")
	if tagObject == second {
		t.Fatal("fixture tag is not annotated")
	}

	got, err := resolveLocalTagCommit(repo, "v2.15.0")
	if err != nil || got != second {
		t.Fatalf("resolveLocalTagCommit() = %q, %v; want %q, nil", got, err, second)
	}

	gitForTest(t, repo, "checkout", "--detach", first)
	if _, err := resolveLocalTagCommit(repo, "v2.15.0"); err == nil || !strings.Contains(err.Error(), "not an ancestor") {
		t.Fatalf("resolveLocalTagCommit(non-ancestor) error = %v, want not-an-ancestor error", err)
	}
}

func TestReleaseTruthWorkflowRunsPromotionCheckWithTags(t *testing.T) {
	path := filepath.Join("..", "..", ".github", "workflows", "release-truth.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	for _, want := range []string{"fetch-depth: 0", "cap-release promotion-check"} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("release-truth workflow lacks %q", want)
		}
	}
}

func writeGitFixture(t *testing.T, repo, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, "fixture.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitForTest(t, repo, "add", "fixture.txt")
	gitForTest(t, repo, "commit", "-m", content)
}

func gitForTest(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
