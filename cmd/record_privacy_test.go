package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/it-odyssey/waketrail/internal/state"
	"github.com/it-odyssey/waketrail/internal/storage"
)

func TestRecorderPersistsSanitizedEvidenceAndEnforcesCapturePolicy(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now()
	sid, err := store.CreateSession("privacy-fixture", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveSession(state.Session{ID: sid, Name: "privacy-fixture", StartedAt: now}); err != nil {
		t.Fatal(err)
	}
	oldCwd, oldMode, oldOut, oldErr, oldBefore := recordCwd, recordCaptureMode, recordStdoutFile, recordStderrFile, recordGitBeforeFile
	oldStart, oldEnd, oldExit := recordStartedAt, recordEndedAt, recordExitCode
	t.Cleanup(func() {
		recordCwd, recordCaptureMode, recordStdoutFile, recordStderrFile, recordGitBeforeFile = oldCwd, oldMode, oldOut, oldErr, oldBefore
		recordStartedAt, recordEndedAt, recordExitCode = oldStart, oldEnd, oldExit
	})
	recordCwd = t.TempDir()
	recordGitBeforeFile = ""
	recordExitCode = 0
	stdoutPath := filepath.Join(t.TempDir(), "stdout")
	stderrPath := filepath.Join(t.TempDir(), "stderr")
	stdout := "GITHUB_TOKEN=synthetic-output-token\n{\"password\":\"synthetic-json-password\"}\nordinary status evidence\n"
	stderr := "Authorization: Bearer synthetic-stderr-token\n"
	if err := os.WriteFile(stdoutPath, []byte(stdout), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stderrPath, []byte(stderr), 0600); err != nil {
		t.Fatal(err)
	}
	commands := []string{
		"GITHUB_TOKEN=synthetic-command-token",
		"cat > config.yaml <<'EOF'\nunlabelled-synthetic-body\nEOF",
		"kubectl get secret/db -o yaml", "docker compose config", "terraform output -json", "curl https://example.test",
		"docker ps", "terraform apply -auto-approve",
	}
	for i, command := range commands {
		recordStartedAt = now.Add(time.Duration(i) * time.Second).UnixNano()
		recordEndedAt = recordStartedAt + 1000000
		recordCaptureMode = "output"
		recordStdoutFile = stdoutPath
		recordStderrFile = stderrPath
		// Denied captures must not even read the supplied files.
		if i < 6 {
			recordStdoutFile = stdoutPath + "-missing"
			recordStderrFile = stderrPath + "-missing"
		}
		if err := recordCmd.RunE(recordCmd, []string{command}); err != nil {
			t.Fatalf("%q: %v", command, err)
		}
	}
	events, err := store.CommandEventsForSession(sid)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != len(commands) {
		t.Fatalf("events=%d", len(events))
	}
	for i, event := range events {
		if strings.Contains(event.Command, "synthetic-command-token") || strings.Contains(event.Command, "unlabelled-synthetic-body") {
			t.Fatalf("raw body persisted: %q", event.Command)
		}
		output, err := store.CommandOutputForCommandEvent(event.ID)
		if i < 6 {
			if event.CaptureMode != "none" || !errors.Is(err, storage.ErrCommandOutputNotFound) {
				t.Fatalf("denied output persisted: %+v, %v", event, err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"synthetic-output-token", "synthetic-json-password", "synthetic-stderr-token"} {
			if strings.Contains(output.Stdout+output.Stderr, secret) {
				t.Fatalf("secret in SQLite: %q", secret)
			}
		}
		if !strings.Contains(output.Stdout, "ordinary status evidence") || output.StdoutBytes != int64(len(stdout)) || output.StderrBytes != int64(len(stderr)) {
			t.Fatal("useful output or original byte counts lost")
		}
		if i == 7 && event.CaptureMode != "bounded" {
			t.Fatal("recorder allowed output-mode bypass for a bounded command")
		}
	}
}
