package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/it-odyssey/waketrail/internal/localdata"
)

type Session struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	StartedAt time.Time `json:"started_at"`
}

func statePath() (string, error) {
	dir, err := localdata.Directory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "active-session.json"), nil
}

func SaveSession(session Session) error {
	path, err := statePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return localdata.WritePrivate(path, data)
}

func LoadSession() (Session, error) {
	path, err := statePath()
	if err != nil {
		return Session{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return Session{}, err
	}

	return session, nil
}

func HasActiveSession() (bool, error) {
	path, err := statePath()
	if err != nil {
		return false, err
	}

	_, err = os.Stat(path)

	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

func ClearSession() error {
	path, err := statePath()
	if err != nil {
		return err
	}

	err = os.Remove(path)

	if os.IsNotExist(err) {
		return nil
	}

	return err
}
