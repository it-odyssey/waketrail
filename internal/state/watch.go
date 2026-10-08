package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/it-odyssey/waketrail/internal/localdata"
)

var ErrWatchNotRunning = errors.New("watch is not running")

type WatchState struct {
	PID       int       `json:"pid"`
	SessionID int64     `json:"session_id"`
	Collector string    `json:"collector"`
	StartedAt time.Time `json:"started_at"`
}

func watchStatePath() (string, error) {
	dir, err := localdata.Directory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "watch.json"), nil
}

func SaveWatchState(watch WatchState) error {
	path, err := watchStatePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(
		watch,
		"",
		"  ",
	)
	if err != nil {
		return err
	}

	return localdata.WritePrivate(path, data)
}

func LoadWatchState() (WatchState, error) {
	path, err := watchStatePath()
	if err != nil {
		return WatchState{}, err
	}

	data, err := os.ReadFile(path)

	if errors.Is(err, os.ErrNotExist) {
		return WatchState{}, ErrWatchNotRunning
	}

	if err != nil {
		return WatchState{}, err
	}

	var watch WatchState

	if err := json.Unmarshal(
		data,
		&watch,
	); err != nil {
		return WatchState{}, err
	}

	return watch, nil
}

func ClearWatchState() error {
	path, err := watchStatePath()
	if err != nil {
		return err
	}

	err = os.Remove(path)

	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}
