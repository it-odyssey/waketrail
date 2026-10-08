package storage

import (
	"strings"
	"testing"
	"time"
)

func TestStorageSanitizesTextFromEveryProducer(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now()
	sid, err := store.CreateSession("privacy", now)
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.InsertCommandEvent(CommandEvent{SessionID: &sid, Command: "DB_PASSWORD=synthetic-direct-command", StartedAt: now, EndedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertCommandOutput(CommandOutput{CommandEventID: id, Stdout: `{"api_key":"synthetic-direct-output"}`, Stderr: "Authorization: Basic synthetic-direct-stderr"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertTimelineEvent(TimelineEvent{SessionID: &sid, Source: "user", EventType: "note", Summary: "TOKEN=synthetic-direct-note", Resource: "https://user:synthetic-resource-password@localhost", OccurredAt: now}); err != nil {
		t.Fatal(err)
	}
	commands, err := store.CommandEventsForSession(sid)
	if err != nil {
		t.Fatal(err)
	}
	output, err := store.CommandOutputForCommandEvent(id)
	if err != nil {
		t.Fatal(err)
	}
	timeline, err := store.TimelineEventsForSession(sid)
	if err != nil {
		t.Fatal(err)
	}
	combined := commands[0].Command + output.Stdout + output.Stderr + timeline[0].Summary + timeline[0].Resource
	if strings.Contains(combined, "synthetic-") {
		t.Fatalf("raw secret persisted: %s", combined)
	}
	if !strings.Contains(timeline[0].Summary, "[REDACTED]") {
		t.Fatal("note was not redacted")
	}
	// New-write protection must not silently rewrite historical evidence.
	if _, err := store.db.Exec("UPDATE command_events SET command = ? WHERE id = ?", "TOKEN=synthetic-historical", id); err != nil {
		t.Fatal(err)
	}
	commands, err = store.CommandEventsForSession(sid)
	if err != nil {
		t.Fatal(err)
	}
	if commands[0].Command != "TOKEN=synthetic-historical" {
		t.Fatal("read path rewrote historical evidence")
	}
}
