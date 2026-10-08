package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"github.com/it-odyssey/waketrail/internal/redact"
	_ "modernc.org/sqlite"
)

type SessionRecord struct {
	ID        int64
	Name      string
	StartedAt time.Time
	EndedAt   *time.Time
}

type CommandEvent struct {
	ID          int64
	SessionID   *int64
	Command     string
	Cwd         string
	ExitCode    int
	CaptureMode string
	StartedAt   time.Time
	EndedAt     time.Time
}

type GitContext struct {
	CommandEventID int64
	RepositoryRoot string
	Branch         string
	CommitSHA      string
	Dirty          bool
}

type TimelineEvent struct {
	ID           int64
	SessionID    *int64
	EventType    string
	Source       string
	ResourceType string
	Resource     string
	Summary      string
	OccurredAt   time.Time
}

type CommandOutput struct {
	CommandEventID  int64
	Stdout          string
	Stderr          string
	StdoutBytes     int64
	StderrBytes     int64
	StdoutTruncated bool
	StderrTruncated bool
}

type Store struct {
	db *sql.DB
}

func stateHome() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "waketrail"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "state", "waketrail"), nil
}

func Open() (*Store, error) {
	dir, err := stateHome()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dir, "waketrail.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// WakeTrail can have the interactive shell recorder and a detached
	// watcher accessing SQLite at the same time. Use one connection per
	// Store and allow short-lived write locks to clear instead of failing
	// immediately with SQLITE_BUSY.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(
		"PRAGMA busy_timeout = 5000;",
	); err != nil {
		db.Close()
		return nil, err
	}

	store := &Store{db: db}

	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS sessions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	started_at TEXT NOT NULL,
	ended_at TEXT
);

CREATE TABLE IF NOT EXISTS timeline_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id INTEGER,
	event_type TEXT NOT NULL,
	source TEXT NOT NULL,
	resource_type TEXT NOT NULL DEFAULT '',
	resource_name TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL,
	occurred_at TEXT NOT NULL,
	FOREIGN KEY (session_id)
		REFERENCES sessions(id)
		ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS command_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id INTEGER,
	command TEXT NOT NULL,
	cwd TEXT NOT NULL,
	exit_code INTEGER NOT NULL,
	capture_mode TEXT NOT NULL DEFAULT 'none',
	started_at TEXT NOT NULL,
	ended_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS command_output (
	command_event_id INTEGER PRIMARY KEY,
	stdout TEXT NOT NULL DEFAULT '',
	stderr TEXT NOT NULL DEFAULT '',
	stdout_bytes INTEGER NOT NULL DEFAULT 0,
	stderr_bytes INTEGER NOT NULL DEFAULT 0,
	stdout_truncated INTEGER NOT NULL DEFAULT 0,
	stderr_truncated INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY (command_event_id)
		REFERENCES command_events(id)
		ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS git_context (
	command_event_id INTEGER PRIMARY KEY,
	repository_root TEXT NOT NULL,
	branch TEXT NOT NULL,
	commit_sha TEXT NOT NULL,
	dirty INTEGER NOT NULL,
	FOREIGN KEY (command_event_id)
		REFERENCES command_events(id)
		ON DELETE CASCADE
);
`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	hasSessionID, err := s.commandEventsHasColumn("session_id")
	if err != nil {
		return err
	}

	if !hasSessionID {
		if _, err := s.db.Exec(
			`ALTER TABLE command_events ADD COLUMN session_id INTEGER`,
		); err != nil {
			return err
		}
	}

	hasCaptureMode, err := s.commandEventsHasColumn("capture_mode")
	if err != nil {
		return err
	}

	if !hasCaptureMode {
		if _, err := s.db.Exec(
			`ALTER TABLE command_events ADD COLUMN capture_mode TEXT NOT NULL DEFAULT 'none'`,
		); err != nil {
			return err
		}
	}

	hasResourceType, err := s.timelineEventsHasColumn(
		"resource_type",
	)
	if err != nil {
		return err
	}

	if !hasResourceType {
		if _, err := s.db.Exec(
			`ALTER TABLE timeline_events ADD COLUMN resource_type TEXT NOT NULL DEFAULT ''`,
		); err != nil {
			return err
		}
	}

	hasResourceName, err := s.timelineEventsHasColumn(
		"resource_name",
	)
	if err != nil {
		return err
	}

	if !hasResourceName {
		if _, err := s.db.Exec(
			`ALTER TABLE timeline_events ADD COLUMN resource_name TEXT NOT NULL DEFAULT ''`,
		); err != nil {
			return err
		}
	}

	if _, err := s.db.Exec(`
CREATE INDEX IF NOT EXISTS idx_command_events_session_id
ON command_events(session_id);
`); err != nil {
		return err
	}

	return nil
}

func (s *Store) commandEventsHasColumn(columnName string) (bool, error) {
	rows, err := s.db.Query(`PRAGMA table_info(command_events)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal any
			primaryKey int
		)

		if err := rows.Scan(
			&cid,
			&name,
			&columnType,
			&notNull,
			&defaultVal,
			&primaryKey,
		); err != nil {
			return false, err
		}

		if name == columnName {
			return true, nil
		}
	}

	return false, rows.Err()
}

func (s *Store) timelineEventsHasColumn(
	columnName string,
) (bool, error) {
	rows, err := s.db.Query(
		`PRAGMA table_info(timeline_events)`,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal any
			primaryKey int
		)

		if err := rows.Scan(
			&cid,
			&name,
			&columnType,
			&notNull,
			&defaultVal,
			&primaryKey,
		); err != nil {
			return false, err
		}

		if name == columnName {
			return true, nil
		}
	}

	return false, rows.Err()
}

func (s *Store) CreateSession(name string, startedAt time.Time) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO sessions (name, started_at) VALUES (?, ?)`,
		name,
		startedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (s *Store) EndSession(id int64, endedAt time.Time) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET ended_at = ? WHERE id = ?`,
		endedAt.Format(time.RFC3339Nano),
		id,
	)

	return err
}

func (s *Store) InsertCommandEvent(event CommandEvent) (int64, error) {
	// Enforce text sanitization at the write boundary for every producer.
	event.Command = redact.Command(event.Command)
	const query = `
INSERT INTO command_events (
	session_id,
	command,
	cwd,
	exit_code,
	capture_mode,
	started_at,
	ended_at
)
VALUES (?, ?, ?, ?, ?, ?, ?);
`

	result, err := s.db.Exec(
		query,
		event.SessionID,
		event.Command,
		event.Cwd,
		event.ExitCode,
		event.CaptureMode,
		event.StartedAt.Format(time.RFC3339Nano),
		event.EndedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (s *Store) InsertGitContext(context GitContext) error {
	const query = `
INSERT INTO git_context (
	command_event_id,
	repository_root,
	branch,
	commit_sha,
	dirty
)
VALUES (?, ?, ?, ?, ?);
`

	_, err := s.db.Exec(
		query,
		context.CommandEventID,
		context.RepositoryRoot,
		context.Branch,
		context.CommitSHA,
		context.Dirty,
	)

	return err
}

func (s *Store) InsertTimelineEvent(
	event TimelineEvent,
) (int64, error) {
	event.Summary = redact.String(event.Summary)
	event.Resource = redact.String(event.Resource)
	const query = `
INSERT INTO timeline_events (
	session_id,
	event_type,
	source,
	resource_type,
	resource_name,
	summary,
	occurred_at
)
VALUES (?, ?, ?, ?, ?, ?, ?);
`

	result, err := s.db.Exec(
		query,
		event.SessionID,
		event.EventType,
		event.Source,
		event.ResourceType,
		event.Resource,
		event.Summary,
		event.OccurredAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (s *Store) InsertCommandOutput(output CommandOutput) error {
	output.Stdout = redact.String(output.Stdout)
	output.Stderr = redact.String(output.Stderr)
	const query = `
INSERT INTO command_output (
	command_event_id,
	stdout,
	stderr,
	stdout_bytes,
	stderr_bytes,
	stdout_truncated,
	stderr_truncated
)
VALUES (?, ?, ?, ?, ?, ?, ?);
`

	_, err := s.db.Exec(
		query,
		output.CommandEventID,
		output.Stdout,
		output.Stderr,
		output.StdoutBytes,
		output.StderrBytes,
		output.StdoutTruncated,
		output.StderrTruncated,
	)

	return err
}
