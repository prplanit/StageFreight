package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repo with a linear commit history
// and the specified tags. Returns the repo directory.
// Each commit is a trivial file change so tags land on distinct commits.
func setupTestRepo(t *testing.T, commits int, tags map[int][]string) string {
	t.Helper()

	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %s", args, err, out)
		}
	}

	git("init", "-b", "main")

	for i := 1; i <= commits; i++ {
		f := filepath.Join(dir, "file.txt")
		if err := os.WriteFile(f, []byte{byte(i)}, 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", "file.txt")
		git("commit", "-m", "commit "+string(rune('0'+i)))

		if tagNames, ok := tags[i]; ok {
			for _, tag := range tagNames {
				git("tag", tag)
			}
		}
	}

	return dir
}

func TestPreviousReleaseTag_SkipsLatest(t *testing.T) {
	// Commit history: 1(v0.0.2) -> 2(latest) -> 3(v0.1.0)
	repo := setupTestRepo(t, 3, map[int][]string{
		1: {"v0.0.2"},
		2: {"latest"},
		3: {"v0.1.0"},
	})

	got, err := PreviousReleaseTag(repo, "v0.1.0", []string{`^v?\d+\.\d+\.\d+$`})
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.0.2" {
		t.Errorf("got %q, want %q", got, "v0.0.2")
	}
}

func TestPreviousReleaseTag_SkipsSameVersionAlias(t *testing.T) {
	// Commit history: 1(v0.0.2) -> 2(0.1.0) -> 3(v0.1.0)
	// 0.1.0 is a stale bare-version alias from a failed release attempt.
	repo := setupTestRepo(t, 3, map[int][]string{
		1: {"v0.0.2"},
		2: {"0.1.0"},
		3: {"v0.1.0"},
	})

	got, err := PreviousReleaseTag(repo, "v0.1.0", []string{`^v?\d+\.\d+\.\d+$`})
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.0.2" {
		t.Errorf("got %q, want %q", got, "v0.0.2")
	}
}

func TestPreviousReleaseTag_SkipsSameCommitAlias(t *testing.T) {
	// v0.1.0 and 0.1.0 on the SAME commit (rolling alias created during release).
	// Must still find v0.0.2.
	repo := setupTestRepo(t, 2, map[int][]string{
		1: {"v0.0.2"},
		2: {"v0.1.0", "0.1.0", "latest"},
	})

	got, err := PreviousReleaseTag(repo, "v0.1.0", []string{`^v?\d+\.\d+\.\d+$`})
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.0.2" {
		t.Errorf("got %q, want %q", got, "v0.0.2")
	}
}

func TestPreviousReleaseTag_DefaultPatternFallback(t *testing.T) {
	// No patterns provided — should fall back to default semver matcher.
	repo := setupTestRepo(t, 3, map[int][]string{
		1: {"v0.0.2"},
		2: {"latest"},
		3: {"v0.1.0"},
	})

	got, err := PreviousReleaseTag(repo, "v0.1.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.0.2" {
		t.Errorf("got %q, want %q", got, "v0.0.2")
	}
}

func TestPreviousReleaseTag_PatternExcludesBareVersion(t *testing.T) {
	// Policy only matches v-prefixed tags. Bare 0.1.0 must be ignored
	// even though it's a valid ancestor.
	repo := setupTestRepo(t, 3, map[int][]string{
		1: {"v0.0.2"},
		2: {"0.1.0"},
		3: {"v0.2.0"},
	})

	got, err := PreviousReleaseTag(repo, "v0.2.0", []string{`^v\d+\.\d+\.\d+$`})
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.0.2" {
		t.Errorf("got %q, want %q", got, "v0.0.2")
	}
}

func TestPreviousReleaseTag_NonAncestorSkipped(t *testing.T) {
	// Create a branch with a higher version tag that's not an ancestor of main.
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %s", args, err, out)
		}
	}

	git("init", "-b", "main")

	// Commit 1: base
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("1"), 0o644)
	git("add", "f.txt")
	git("commit", "-m", "base")
	git("tag", "v0.0.1")

	// Branch off
	git("checkout", "-b", "other")
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("other"), 0o644)
	git("add", "f.txt")
	git("commit", "-m", "other branch")
	git("tag", "v0.9.0") // higher version, not on main's history

	// Back to main
	git("checkout", "main")
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("2"), 0o644)
	git("add", "f.txt")
	git("commit", "-m", "main commit")
	git("tag", "v0.1.0")

	// v0.9.0 exists but is NOT an ancestor of v0.1.0
	got, err := PreviousReleaseTag(dir, "v0.1.0", []string{`^v?\d+\.\d+\.\d+$`})
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.0.1" {
		t.Errorf("got %q, want %q", got, "v0.0.1")
	}
}

func TestPreviousReleaseTag_NoPreviousTag(t *testing.T) {
	// Only the current tag exists. Should return empty, not error.
	repo := setupTestRepo(t, 1, map[int][]string{
		1: {"v0.1.0"},
	})

	got, err := PreviousReleaseTag(repo, "v0.1.0", []string{`^v?\d+\.\d+\.\d+$`})
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestPreviousReleaseTag_BareCurrentRef(t *testing.T) {
	// currentRef passed as bare version "0.1.0" — same-version exclusion
	// must still work against v0.1.0 tags in history.
	repo := setupTestRepo(t, 3, map[int][]string{
		1: {"v0.0.2"},
		2: {"v0.1.0"},
		3: {"0.1.0"},
	})

	got, err := PreviousReleaseTag(repo, "0.1.0", []string{`^v?\d+\.\d+\.\d+$`})
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.0.2" {
		t.Errorf("got %q, want %q", got, "v0.0.2")
	}
}

func TestPreviousReleaseTag_PrereleaseIncluded(t *testing.T) {
	// Prerelease tags matching the prerelease policy should be eligible.
	repo := setupTestRepo(t, 3, map[int][]string{
		1: {"v0.0.2"},
		2: {"v0.1.0-rc1"},
		3: {"v0.1.0"},
	})

	patterns := []string{
		`^v?\d+\.\d+\.\d+$`,
		`^v?\d+\.\d+\.\d+-.+`,
	}

	got, err := PreviousReleaseTag(repo, "v0.1.0", patterns)
	if err != nil {
		t.Fatal(err)
	}
	// v0.1.0-rc1 is a different normalized version (0.1.0-rc1 != 0.1.0)
	// and is closer than v0.0.2, so it should be found first.
	if got != "v0.1.0-rc1" {
		t.Errorf("got %q, want %q", got, "v0.1.0-rc1")
	}
}

func TestNormalizeReleaseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v0.1.0", "0.1.0"},
		{"0.1.0", "0.1.0"},
		{"refs/tags/v0.1.0", "0.1.0"},
		{"refs/tags/0.1.0", "0.1.0"},
		{"v1.2.3-rc1", "1.2.3-rc1"},
		{"latest", "latest"},
	}
	for _, tt := range tests {
		got := normalizeReleaseVersion(tt.input)
		if got != tt.want {
			t.Errorf("normalizeReleaseVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCompileReleaseTagMatchers_InvalidPattern(t *testing.T) {
	_, err := compileReleaseTagMatchers([]string{`[invalid`})
	if err == nil {
		t.Error("expected error for invalid regex pattern, got nil")
	}
}

func TestCompileReleaseTagMatchers_EmptyFallback(t *testing.T) {
	matchers, err := compileReleaseTagMatchers(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(matchers) != 1 {
		t.Fatalf("expected 1 default matcher, got %d", len(matchers))
	}
	if !matchers[0].MatchString("v0.1.0") {
		t.Error("default matcher should match v0.1.0")
	}
	if !matchers[0].MatchString("0.1.0") {
		t.Error("default matcher should match 0.1.0")
	}
	if matchers[0].MatchString("latest") {
		t.Error("default matcher should NOT match latest")
	}
}
