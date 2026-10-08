package storage

import (
	"encoding/json"
	"os"
	"path/filepath"

	dockercollector "github.com/it-odyssey/waketrail/internal/collectors/docker"
	"github.com/it-odyssey/waketrail/internal/localdata"
)

func dockerSnapshotPath() (string, error) {
	dir, err := localdata.Directory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "docker-snapshot.json"), nil
}

func SaveDockerSnapshot(
	containers []dockercollector.ContainerState,
) error {
	path, err := dockerSnapshotPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(
		containers,
		"",
		"  ",
	)
	if err != nil {
		return err
	}

	return localdata.WritePrivate(path, data)
}

func LoadDockerSnapshot() (
	[]dockercollector.ContainerState,
	error,
) {
	path, err := dockerSnapshotPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)

	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var containers []dockercollector.ContainerState

	if err := json.Unmarshal(
		data,
		&containers,
	); err != nil {
		return nil, err
	}

	return containers, nil
}
