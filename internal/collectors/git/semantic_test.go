package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseStatusUnbornDetachedAndDirty(t *testing.T) {
	initial := ParseStatus("# branch.oid (initial)\n# branch.head main\n? new file.txt\n")
	if initial.Commit != "" || initial.Branch != "main" || !initial.Dirty || len(initial.Files) != 1 {
		t.Fatalf("initial: %+v", initial)
	}
	detached := ParseStatus("# branch.oid abcdef123456789\n# branch.head (detached)\n")
	if detached.Branch != "HEAD (detached)" || detached.Commit != "abcdef123456789" {
		t.Fatalf("detached: %+v", detached)
	}
}
func TestCompareGitSemantics(t *testing.T) {
	before := Context{IsRepository: true, Root: "/tmp/example", Branch: "main", Commit: "111111111111", Dirty: true}
	after := Context{IsRepository: true, Root: "/tmp/example", Branch: "main", Commit: "222222222222"}
	changes := Compare("git commit -m message", before, after)
	if len(changes) != 1 || changes[0].Kind != "commit" {
		t.Fatalf("commit changes: %+v", changes)
	}
	after.Branch = "feature"
	changes = Compare("git switch feature", before, after)
	if len(changes) == 0 || changes[0].Kind != "branch_switch" {
		t.Fatalf("branch changes: %+v", changes)
	}
	after.Root = "/tmp/unrelated"
	if changes := Compare("git commit -m x", before, after); len(changes) != 0 {
		t.Fatalf("cross repo correlation: %+v", changes)
	}
}
func TestRealGitTransitions(t *testing.T) {
	repo := createTestRepository(t)
	before, err := Detect(repo)
	if err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repo, "checkout", "-b", "git-test")
	after, err := Detect(repo)
	if err != nil {
		t.Fatal(err)
	}
	changes := Compare("git checkout -b git-test", before, after)
	if len(changes) != 1 || changes[0].Kind != "branch_switch" {
		t.Fatalf("switch: %+v", changes)
	}
	before = after
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repo, "add", "new.txt")
	runTestGit(t, repo, "commit", "-m", "test")
	after, err = Detect(repo)
	if err != nil {
		t.Fatal(err)
	}
	changes = Compare("git commit -m test", before, after)
	if len(changes) != 1 || changes[0].Kind != "commit" {
		t.Fatalf("commit: %+v", changes)
	}
}
func TestDetectEmptyRepositoryAndOutside(t *testing.T) {
	repo := t.TempDir()
	c := exec.Command("git", "-C", repo, "init")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	got, err := Detect(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsRepository || got.Commit != "" || got.Branch == "" {
		t.Fatalf("unborn repo: %+v", got)
	}
	outside := t.TempDir()
	got, err = Detect(outside)
	if err != nil && !strings.Contains(err.Error(), "not a git repository") {
		t.Fatal(err)
	}
	if got.IsRepository {
		t.Fatalf("outside: %+v", got)
	}
}

func TestGitAddCapturesIndexTransition(t *testing.T) {
	repo := createTestRepository(t)
	path := filepath.Join(repo, "sample.txt")
	if err := os.WriteFile(path, []byte("metadata only"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := Detect(repo)
	if err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repo, "add", "sample.txt")
	after, err := Detect(repo)
	if err != nil {
		t.Fatal(err)
	}
	changes := Compare("git add sample.txt", before, after)
	if len(changes) != 1 || changes[0].Kind != "files_changed" || !strings.Contains(changes[0].Summary, "sample.txt") {
		t.Fatalf("git add changes: %+v", changes)
	}
}
