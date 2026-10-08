package localdata

import (
	"os"
	"path/filepath"
	"testing"
)

func assertMode(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != expected {
		t.Fatalf("%s mode=%o want=%o", path, info.Mode().Perm(), expected)
	}
}

func TestDirectoryRepairsExistingEvidenceWithoutChangingParent(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", base)
	if err := os.Chmod(base, 0755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "waketrail")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	files := []string{"waketrail.db", "waketrail.db-wal", "waketrail.db-shm", "waketrail.db-journal", "active-session.json", "watch.json", "docker-snapshot.json", "watch.log"}
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("evidence"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
	}
	actual, err := Directory()
	if err != nil || actual != dir {
		t.Fatalf("Directory()=%q %v", actual, err)
	}
	assertMode(t, dir, 0700)
	assertMode(t, base, 0755)
	for _, name := range files {
		assertMode(t, filepath.Join(dir, name), 0600)
		value, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(value) != "evidence" {
			t.Fatal("permissions repair changed evidence")
		}
	}
}

func TestWritePrivateRepairsExportsAndPreservesAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.md")
	if err := os.WriteFile(path, []byte("old report with a long suffix"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivate(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	assertMode(t, path, 0600)
	file, err := OpenPrivate(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(" appended"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	value, err := os.ReadFile(path)
	if err != nil || string(value) != "new appended" {
		t.Fatalf("write/append changed: %q %v", value, err)
	}
}

func TestPrivateWritesRejectSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("unchanged"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivate(link, []byte("changed")); err == nil {
		t.Fatal("symlink accepted")
	}
	value, err := os.ReadFile(target)
	if err != nil || string(value) != "unchanged" {
		t.Fatal("symlink target modified")
	}
}
