package storage

import (
	"database/sql"
	"errors"
	"time"
)

var ErrSessionNotFound = errors.New("session not found")
var ErrGitContextNotFound = errors.New("git context not found")
var ErrCommandOutputNotFound = errors.New("command output not found")

func (s *Store) LatestSession() (SessionRecord, error) {
	const query = `
SELECT
	id,
	name,
	started_at,
	ended_at
FROM sessions
ORDER BY id DESC
LIMIT 1;
`

	return s.scanSession(s.db.QueryRow(query))
}

func (s *Store) SessionByName(name string) (SessionRecord, error) {
	const query = `
SELECT
	id,
	name,
	started_at,
	ended_at
FROM sessions
WHERE name = ?
ORDER BY id DESC
LIMIT 1;
`

	return s.scanSession(s.db.QueryRow(query, name))
}

func (s *Store) scanSession(row *sql.Row) (SessionRecord, error) {
	var (
		session       SessionRecord
		startedAtText string
		endedAtText   sql.NullString
	)

	err := row.Scan(
		&session.ID,
		&session.Name,
		&startedAtText,
		&endedAtText,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return SessionRecord{}, ErrSessionNotFound
	}

	if err != nil {
		return SessionRecord{}, err
	}

	startedAt, err := time.Parse(time.RFC3339Nano, startedAtText)
	if err != nil {
		return SessionRecord{}, err
	}

	session.StartedAt = startedAt

	if endedAtText.Valid {
		endedAt, err := time.Parse(time.RFC3339Nano, endedAtText.String)
		if err != nil {
			return SessionRecord{}, err
		}

		session.EndedAt = &endedAt
	}

	return session, nil
}

func (s *Store) CommandEventsForSession(sessionID int64) ([]CommandEvent, error) {
	const query = `
SELECT
	id,
	command,
	cwd,
	exit_code,
	capture_mode,
	started_at,
	ended_at
FROM command_events
WHERE session_id = ?
ORDER BY started_at ASC;
`

	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []CommandEvent

	for rows.Next() {
		var (
			event         CommandEvent
			startedAtText string
			endedAtText   string
		)

		err := rows.Scan(
			&event.ID,
			&event.Command,
			&event.Cwd,
			&event.ExitCode,
			&event.CaptureMode,
			&startedAtText,
			&endedAtText,
		)
		if err != nil {
			return nil, err
		}

		startedAt, err := time.Parse(time.RFC3339Nano, startedAtText)
		if err != nil {
			return nil, err
		}

		endedAt, err := time.Parse(time.RFC3339Nano, endedAtText)
		if err != nil {
			return nil, err
		}

		event.SessionID = &sessionID
		event.StartedAt = startedAt
		event.EndedAt = endedAt

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (s *Store) GitContextForCommandEvent(commandEventID int64) (GitContext, error) {
	const query = `
SELECT
	command_event_id,
	repository_root,
	branch,
	commit_sha,
	dirty
FROM git_context
WHERE command_event_id = ?;
`

	var context GitContext

	err := s.db.QueryRow(
		query,
		commandEventID,
	).Scan(
		&context.CommandEventID,
		&context.RepositoryRoot,
		&context.Branch,
		&context.CommitSHA,
		&context.Dirty,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return GitContext{}, ErrGitContextNotFound
	}

	if err != nil {
		return GitContext{}, err
	}

	return context, nil
}

func (s *Store) TimelineEventsForSession(sessionID int64) ([]TimelineEvent, error) {
	const query = `
SELECT
	id,
	session_id,
	event_type,
	source,
	resource_type,
	resource_name,
	summary,
	occurred_at
FROM timeline_events
WHERE session_id = ?
ORDER BY occurred_at ASC;
`

	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []TimelineEvent

	for rows.Next() {
		var (
			event          TimelineEvent
			occurredAtText string
			storedSession  sql.NullInt64
		)

		err := rows.Scan(
			&event.ID,
			&storedSession,
			&event.EventType,
			&event.Source,
			&event.ResourceType,
			&event.Resource,
			&event.Summary,
			&occurredAtText,
		)
		if err != nil {
			return nil, err
		}

		if storedSession.Valid {
			sessionID := storedSession.Int64
			event.SessionID = &sessionID
		}

		occurredAt, err := time.Parse(time.RFC3339Nano, occurredAtText)
		if err != nil {
			return nil, err
		}

		event.OccurredAt = occurredAt

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (s *Store) CommandOutputForCommandEvent(
	commandEventID int64,
) (CommandOutput, error) {
	const query = `
SELECT
	command_event_id,
	stdout,
	stderr,
	stdout_bytes,
	stderr_bytes,
	stdout_truncated,
	stderr_truncated
FROM command_output
WHERE command_event_id = ?;
`

	var output CommandOutput

	err := s.db.QueryRow(
		query,
		commandEventID,
	).Scan(
		&output.CommandEventID,
		&output.Stdout,
		&output.Stderr,
		&output.StdoutBytes,
		&output.StderrBytes,
		&output.StdoutTruncated,
		&output.StderrTruncated,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return CommandOutput{}, ErrCommandOutputNotFound
	}

	if err != nil {
		return CommandOutput{}, err
	}

	return output, nil
}

type SessionSummary struct {
	ID         int64
	Name       string
	StartedAt  time.Time
	EndedAt    *time.Time
	EventCount int
}

func (s *Store) ListSessions() ([]SessionSummary, error) {
	const query = `
SELECT
	s.id,
	s.name,
	s.started_at,
	s.ended_at,
	(
		SELECT COUNT(*)
		FROM command_events ce
		WHERE ce.session_id = s.id
	) +
	(
		SELECT COUNT(*)
		FROM timeline_events te
		WHERE te.session_id = s.id
	) AS event_count
FROM sessions s
ORDER BY s.id DESC;
`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []SessionSummary

	for rows.Next() {
		var (
			session       SessionSummary
			startedAtText string
			endedAtText   sql.NullString
		)

		if err := rows.Scan(
			&session.ID,
			&session.Name,
			&startedAtText,
			&endedAtText,
			&session.EventCount,
		); err != nil {
			return nil, err
		}

		startedAt, err := time.Parse(
			time.RFC3339Nano,
			startedAtText,
		)
		if err != nil {
			return nil, err
		}

		session.StartedAt = startedAt

		if endedAtText.Valid {
			endedAt, err := time.Parse(
				time.RFC3339Nano,
				endedAtText.String,
			)
			if err != nil {
				return nil, err
			}

			session.EndedAt = &endedAt
		}

		sessions = append(
			sessions,
			session,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return sessions, nil
}
