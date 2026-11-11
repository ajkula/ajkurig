package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ===== Types =====

type Store struct {
	Directory string
}

type DamagedError struct {
	FileName string
	KeptAs   string
	Cause    error
}

// ===== Constants =====

const (
	directoryMode    = 0o755
	fileMode         = 0o644
	jsonIndentUnit   = "  "
	damagedSuffix    = ".damaged"
	temporaryPattern = ".*.tmp"
)

var (
	ErrDamaged     = errors.New("store: damaged save")
	errNoDirectory = errors.New("store: no save directory")
	errInvalidJSON = errors.New("store: invalid JSON")
)

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
	if !json.Valid(content) {
		return setDamagedAside(path, fileName, errInvalidJSON)
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
	return replaceAtomically(path, content)
}

func (e *DamagedError) Error() string {
	return fmt.Sprintf("store: damaged save %s kept as %s: %v", e.FileName, e.KeptAs, e.Cause)
}

func (e *DamagedError) Unwrap() []error {
	return []error{ErrDamaged, e.Cause}
}

// ===== Internal =====

func setDamagedAside(path, fileName string, cause error) error {
	damaged := &DamagedError{FileName: fileName, KeptAs: fileName + damagedSuffix, Cause: cause}
	if err := os.Rename(path, path+damagedSuffix); err != nil {
		return errors.Join(damaged, err)
	}
	return damaged
}

func replaceAtomically(path string, content []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+temporaryPattern)
	if err != nil {
		return err
	}
	if err := errors.Join(writeDurably(file, content), file.Close()); err != nil {
		return errors.Join(err, os.Remove(file.Name()))
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return errors.Join(err, os.Remove(file.Name()))
	}
	return nil
}

func writeDurably(file *os.File, content []byte) error {
	if _, err := file.Write(content); err != nil {
		return err
	}
	return errors.Join(file.Chmod(fileMode), file.Sync())
}
