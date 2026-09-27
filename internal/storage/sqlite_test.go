package storage

import (
	"testing"
	"time"
)

func TestInsertCommandEvent(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	store, err := Open()
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer store.Close()

	event := CommandEvent{
		Command:   "git status",
		Cwd:       "/tmp/project",
		ExitCode:  0,
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	eventID, err := store.InsertCommandEvent(event)
	if err != nil {
		t.Fatalf("InsertCommandEvent() returned error: %v", err)
	}

	if eventID <= 0 {
		t.Fatalf("InsertCommandEvent() returned invalid ID: %d", eventID)
	}
}

func TestCreateAndEndSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	store, err := Open()
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer store.Close()

	startedAt := time.Now()

	sessionID, err := store.CreateSession("homelab-debug", startedAt)
	if err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	if sessionID <= 0 {
		t.Fatalf("CreateSession() returned invalid ID: %d", sessionID)
	}

	endedAt := startedAt.Add(5 * time.Minute)

	if err := store.EndSession(sessionID, endedAt); err != nil {
		t.Fatalf("EndSession() returned error: %v", err)
	}

	var (
		name      string
		storedEnd string
	)

	err = store.db.QueryRow(
		`SELECT name, ended_at FROM sessions WHERE id = ?`,
		sessionID,
	).Scan(&name, &storedEnd)

	if err != nil {
		t.Fatalf("query session: %v", err)
	}

	if name != "homelab-debug" {
		t.Errorf("name = %q, want %q", name, "homelab-debug")
	}

	if storedEnd == "" {
		t.Error("ended_at is empty")
	}
}

func TestSessionQueries(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	store, err := Open()
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer store.Close()

	startedAt := time.Now().Truncate(time.Second)

	sessionID, err := store.CreateSession("query-test", startedAt)
	if err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	event := CommandEvent{
		SessionID: &sessionID,
		Command:   "git status",
		Cwd:       "/tmp/query-test",
		ExitCode:  0,
		StartedAt: startedAt.Add(2 * time.Second),
		EndedAt:   startedAt.Add(3 * time.Second),
	}

	eventID, err := store.InsertCommandEvent(event)
	if err != nil {
		t.Fatalf("InsertCommandEvent() returned error: %v", err)
	}

	if eventID <= 0 {
		t.Fatalf("InsertCommandEvent() returned invalid ID: %d", eventID)
	}

	session, err := store.SessionByName("query-test")
	if err != nil {
		t.Fatalf("SessionByName() returned error: %v", err)
	}

	if session.ID != sessionID {
		t.Errorf("session ID = %d, want %d", session.ID, sessionID)
	}

	if session.Name != "query-test" {
		t.Errorf("session name = %q, want %q", session.Name, "query-test")
	}

	events, err := store.CommandEventsForSession(sessionID)
	if err != nil {
		t.Fatalf("CommandEventsForSession() returned error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	if events[0].Command != "git status" {
		t.Errorf(
			"command = %q, want %q",
			events[0].Command,
			"git status",
		)
	}

	if events[0].ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", events[0].ExitCode)
	}
}

func TestInsertGitContext(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	store, err := Open()
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer store.Close()

	event := CommandEvent{
		Command:   "git status",
		Cwd:       "/tmp/project",
		ExitCode:  0,
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	eventID, err := store.InsertCommandEvent(event)
	if err != nil {
		t.Fatalf("InsertCommandEvent() returned error: %v", err)
	}

	context := GitContext{
		CommandEventID: eventID,
		RepositoryRoot: "/tmp/project",
		Branch:         "main",
		CommitSHA:      "abc123",
		Dirty:          true,
	}

	if err := store.InsertGitContext(context); err != nil {
		t.Fatalf("InsertGitContext() returned error: %v", err)
	}

	var (
		repositoryRoot string
		branch         string
		commitSHA      string
		dirty          bool
	)

	err = store.db.QueryRow(`
SELECT
	repository_root,
	branch,
	commit_sha,
	dirty
FROM git_context
WHERE command_event_id = ?
`, eventID).Scan(
		&repositoryRoot,
		&branch,
		&commitSHA,
		&dirty,
	)

	if err != nil {
		t.Fatalf("query git context: %v", err)
	}

	if repositoryRoot != "/tmp/project" {
		t.Errorf(
			"repositoryRoot = %q, want %q",
			repositoryRoot,
			"/tmp/project",
		)
	}

	if branch != "main" {
		t.Errorf("branch = %q, want %q", branch, "main")
	}

	if commitSHA != "abc123" {
		t.Errorf("commitSHA = %q, want %q", commitSHA, "abc123")
	}

	if !dirty {
		t.Error("dirty = false, want true")
	}
}

func TestInsertTimelineEvent(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	store, err := Open()
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer store.Close()

	startedAt := time.Now()

	sessionID, err := store.CreateSession(
		"timeline-test",
		startedAt,
	)
	if err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	event := TimelineEvent{
		SessionID:  &sessionID,
		EventType:  "state_change",
		Source:     "docker",
		Summary:    "traefik: healthy -> unhealthy",
		OccurredAt: startedAt.Add(10 * time.Second),
	}

	eventID, err := store.InsertTimelineEvent(event)
	if err != nil {
		t.Fatalf("InsertTimelineEvent() returned error: %v", err)
	}

	if eventID <= 0 {
		t.Fatalf(
			"InsertTimelineEvent() returned invalid ID: %d",
			eventID,
		)
	}

	var (
		eventType string
		source    string
		summary   string
	)

	err = store.db.QueryRow(`
SELECT
	event_type,
	source,
	summary
FROM timeline_events
WHERE id = ?
`, eventID).Scan(
		&eventType,
		&source,
		&summary,
	)

	if err != nil {
		t.Fatalf("query timeline event: %v", err)
	}

	if eventType != "state_change" {
		t.Errorf(
			"eventType = %q, want %q",
			eventType,
			"state_change",
		)
	}

	if source != "docker" {
		t.Errorf(
			"source = %q, want %q",
			source,
			"docker",
		)
	}

	if summary != "traefik: healthy -> unhealthy" {
		t.Errorf(
			"summary = %q, want %q",
			summary,
			"traefik: healthy -> unhealthy",
		)
	}
}

func TestInsertAndQueryCommandOutput(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	store, err := Open()
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer store.Close()

	event := CommandEvent{
		Command:   `curl http://127.0.0.1:8080/health`,
		Cwd:       "/tmp/project",
		ExitCode:  0,
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	eventID, err := store.InsertCommandEvent(event)
	if err != nil {
		t.Fatalf(
			"InsertCommandEvent() returned error: %v",
			err,
		)
	}

	expected := CommandOutput{
		CommandEventID:  eventID,
		Stdout:          `{"status":"healthy"}`,
		Stderr:          "",
		StdoutBytes:     20,
		StderrBytes:     0,
		StdoutTruncated: false,
		StderrTruncated: false,
	}

	if err := store.InsertCommandOutput(expected); err != nil {
		t.Fatalf(
			"InsertCommandOutput() returned error: %v",
			err,
		)
	}

	actual, err := store.CommandOutputForCommandEvent(eventID)
	if err != nil {
		t.Fatalf(
			"CommandOutputForCommandEvent() returned error: %v",
			err,
		)
	}

	if actual.Stdout != expected.Stdout {
		t.Errorf(
			"Stdout = %q, want %q",
			actual.Stdout,
			expected.Stdout,
		)
	}

	if actual.Stderr != expected.Stderr {
		t.Errorf(
			"Stderr = %q, want %q",
			actual.Stderr,
			expected.Stderr,
		)
	}

	if actual.StdoutBytes != expected.StdoutBytes {
		t.Errorf(
			"StdoutBytes = %d, want %d",
			actual.StdoutBytes,
			expected.StdoutBytes,
		)
	}

	if actual.StdoutTruncated {
		t.Error(
			"StdoutTruncated = true, want false",
		)
	}
}
