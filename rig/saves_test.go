package rig

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajkula/ajkurig/store"
)

// ===== Tests =====

func TestDamagedAndUnreadableSavesAreNamedInTheMessage(t *testing.T) {
	directory := t.TempDir()
	saves := store.InDirectory(directory)
	if err := os.WriteFile(filepath.Join(directory, SettingsFileName), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, damaged := LoadSettings(saves)
	unreadable := fmt.Errorf("scores: %w", &fs.PathError{Op: "open", Path: filepath.Join(directory, "scores.json"), Err: fs.ErrPermission})
	_, missing := LoadSettings(store.InDirectory(t.TempDir()))
	app := &App{Message: "kept"}
	app.ReportSaveProblems(damaged, unreadable, missing, nil)
	isNamed := strings.Contains(app.Message, SettingsFileName+".damaged") && strings.Contains(app.Message, "scores.json")
	if !isNamed {
		t.Fatalf("message %q does not name both saves", app.Message)
	}
}

func TestMissingSavesLeaveTheMessageAlone(t *testing.T) {
	_, missing := LoadSettings(store.InDirectory(t.TempDir()))
	app := &App{Message: "kept"}
	app.ReportSaveProblems(missing, nil)
	if app.Message != "kept" {
		t.Fatalf("a first launch changed the message to %q", app.Message)
	}
}

func TestSaveDirectoryReplacesTheSystemFolder(t *testing.T) {
	directory := t.TempDir()
	app := NewApp(Config{Title: "Test", SaveDirectory: directory, IsSilent: true})
	if app.Store.Directory != directory {
		t.Fatalf("saves go to %q instead of %q", app.Store.Directory, directory)
	}
}
