package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// ===== Tests =====

type testSave struct {
	Level int
	Name  string
}

func TestSaveThenLoadKeepsTheValue(t *testing.T) {
	saves := InDirectory(filepath.Join(t.TempDir(), "nested"))
	if err := saves.Save("game.json", testSave{Level: 7, Name: "kettle"}); err != nil {
		t.Fatal(err)
	}
	var loaded testSave
	if err := saves.Load("game.json", &loaded); err != nil || loaded != (testSave{Level: 7, Name: "kettle"}) {
		t.Fatalf("loaded %+v, error %v", loaded, err)
	}
}

func TestSaveLeavesNoTemporaryFile(t *testing.T) {
	directory := t.TempDir()
	saves := InDirectory(directory)
	for level := range 3 {
		if err := saves.Save("game.json", testSave{Level: level}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("%d files after three saves, want only game.json", len(entries))
	}
}

func TestMissingSaveIsReportedAsNotExisting(t *testing.T) {
	var loaded testSave
	err := InDirectory(t.TempDir()).Load("game.json", &loaded)
	if !errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrDamaged) {
		t.Fatalf("missing save gave %v", err)
	}
}

func TestDamagedSaveIsSetAsideInsteadOfOverwritten(t *testing.T) {
	directory := t.TempDir()
	saves := InDirectory(directory)
	damagedContent := []byte(`{"Level": 9, "Na`)
	if err := os.WriteFile(filepath.Join(directory, "game.json"), damagedContent, fileMode); err != nil {
		t.Fatal(err)
	}
	var loaded testSave
	err := saves.Load("game.json", &loaded)
	var damaged *DamagedError
	if !errors.Is(err, ErrDamaged) || !errors.As(err, &damaged) || damaged.KeptAs != "game.json.damaged" {
		t.Fatalf("damaged save gave %v", err)
	}
	if err := saves.Save("game.json", testSave{Level: 1}); err != nil {
		t.Fatal(err)
	}
	kept, err := os.ReadFile(filepath.Join(directory, damaged.KeptAs))
	if err != nil || string(kept) != string(damagedContent) {
		t.Fatalf("the damaged content was not kept: %q, %v", kept, err)
	}
}

func TestStoreWithoutDirectoryRefusesToWork(t *testing.T) {
	var loaded testSave
	empty := Store{}
	if empty.Save("game.json", loaded) == nil || empty.Load("game.json", &loaded) == nil {
		t.Fatal("a store without directory accepted a save or a load")
	}
}

func TestSaveOfAnotherShapeIsNotSetAside(t *testing.T) {
	directory := t.TempDir()
	saves := InDirectory(directory)
	if err := saves.Save("game.json", map[string][]int{"Level": {1, 2}}); err != nil {
		t.Fatal(err)
	}
	var loaded testSave
	err := saves.Load("game.json", &loaded)
	if err == nil || errors.Is(err, ErrDamaged) {
		t.Fatalf("a valid save of another shape gave %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "game.json")); err != nil {
		t.Fatalf("a valid save was moved: %v", err)
	}
}
