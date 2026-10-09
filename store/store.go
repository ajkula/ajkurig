package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// ===== Types =====

type Store struct {
	Directory string
}

// ===== Constants =====

const (
	directoryMode  = 0o755
	fileMode       = 0o644
	jsonIndentUnit = "  "
)

var errNoDirectory = errors.New("store: no save directory")

// ===== Public API =====

func ForApplication(applicationFolder string) Store {
	base, err := os.UserConfigDir()
	if err != nil {
		return Store{}
	}
	return Store{Directory: filepath.Join(base, applicationFolder)}
}

func InDirectory(directory string) Store {
	return Store{Directory: directory}
}

func (s Store) Path(fileName string) (string, error) {
	if s.Directory == "" {
		return "", errNoDirectory
	}
	return filepath.Join(s.Directory, fileName), nil
}

func (s Store) Load(fileName string, target any) error {
	path, err := s.Path(fileName)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(content, target)
}

func (s Store) Save(fileName string, value any) error {
	path, err := s.Path(fileName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), directoryMode); err != nil {
		return err
	}
	content, err := json.MarshalIndent(value, "", jsonIndentUnit)
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, fileMode)
}
