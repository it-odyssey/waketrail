package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/it-odyssey/waketrail/internal/capture"
)

func TestStorageCapsDirectOutputAndKeepsOriginalMetadata(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	value := "first evidence\n" + strings.Repeat("ordinary diagnostic line\n", 10000) + "final evidence\n"
	for _, mode := range []string{"output", "bounded"} {
		now := time.Now()
		id, err := store.InsertCommandEvent(CommandEvent{Command: "docker ps", CaptureMode: mode, StartedAt: now, EndedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.InsertCommandOutput(CommandOutput{CommandEventID: id, Stdout: value, Stderr: value}); err != nil {
			t.Fatal(err)
		}
		actual, err := store.CommandOutputForCommandEvent(id)
		if err != nil {
			t.Fatal(err)
		}
		if len(actual.Stdout) > capture.OutputLimit || len(actual.Stderr) > capture.OutputLimit || !actual.StdoutTruncated || !actual.StderrTruncated || actual.StdoutBytes != int64(len(value)) || actual.StderrBytes != int64(len(value)) || !strings.Contains(actual.Stdout, "final evidence") {
			t.Fatalf("mode=%s: limit, tail or metadata incorrect", mode)
		}
		// Redaction can expand short inputs. Preserve explicitly supplied raw
		// counts instead of replacing them with sanitized text lengths.
		shortID, err := store.InsertCommandEvent(CommandEvent{Command: "docker ps", CaptureMode: mode, StartedAt: now, EndedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.InsertCommandOutput(CommandOutput{CommandEventID: shortID, Stdout: "TOKEN=[REDACTED]", StdoutBytes: 7, StdoutTruncated: true}); err != nil {
			t.Fatal(err)
		}
		short, err := store.CommandOutputForCommandEvent(shortID)
		if err != nil || short.StdoutBytes != 7 || !short.StdoutTruncated {
			t.Fatal("existing raw counts or truncation flag changed")
		}
	}
}

func TestSQLiteDatabaseAndActiveJournalArePrivate(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", base)
	store, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tx, err := store.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("INSERT INTO sessions (name, started_at) VALUES ('permissions', '2026-10-08T00:00:00Z')"); err != nil {
		t.Fatal(err)
	}
	for name, mode := range map[string]os.FileMode{"": 0700, "waketrail.db": 0600, "waketrail.db-journal": 0600} {
		path := filepath.Join(base, "waketrail", name)
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("%s: private mode %o required; info=%v err=%v", name, mode, info, err)
		}
	}
}
