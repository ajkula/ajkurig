package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// ===== Tests =====

func TestOnlyWorkspaceModulesAreMountedAtTheirRelativePlaces(t *testing.T) {
	root := t.TempDir()
	game, library := filepath.Join(root, "game"), filepath.Join(root, "library")
	writeFiles(t, map[string]string{
		filepath.Join(game, "go.mod"):           "module example.com/game\n\ngo 1.26.0\n",
		filepath.Join(game, "go.work"):          "go 1.26.0\n\nuse (\n\t.\n\t../library\n)\n",
		filepath.Join(library, "go.mod"):        "module example.com/library\n\ngo 1.26.0\n",
		filepath.Join(root, "unrelated", "big"): "never mounted",
	})
	dirs := localModuleDirs(game)
	if !slices.Equal(dirs, []string{game, library}) {
		t.Fatalf("module dirs %v, want only the game and the library", dirs)
	}
	mounted := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		target, err := containerPath(mountRoot(dirs), dir)
		if err != nil {
			t.Fatal(err)
		}
		mounted = append(mounted, target)
	}
	if !slices.Equal(mounted, []string{"/src/game", "/src/library"}) {
		t.Fatalf("container paths %v", mounted)
	}
}

func TestLoneModuleIsMountedAlone(t *testing.T) {
	game := t.TempDir()
	writeFiles(t, map[string]string{filepath.Join(game, "go.mod"): "module example.com/game\n\ngo 1.26.0\n"})
	t.Setenv("GOWORK", "")
	dirs := localModuleDirs(game)
	if !slices.Equal(dirs, []string{game}) {
		t.Fatalf("module dirs %v", dirs)
	}
	if target, err := containerPath(mountRoot(dirs), game); err != nil || target != "/src" {
		t.Fatalf("container path %q, %v", target, err)
	}
}

func TestLicenseFilesIncludeFontLicensesButSkipVendoredCode(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, map[string]string{
		filepath.Join(root, "LICENSE"):                "BSD",
		filepath.Join(root, "ui", "fonts", "OFL.txt"): "OFL",
		filepath.Join(root, "vendor", "x", "LICENSE"): "vendored",
		filepath.Join(root, "README.md"):              "readme",
		filepath.Join(root, "licenses.go"):            "package main",
		filepath.Join(root, "examples", "LICENSE"):    "example only",
	})
	paths, err := licenseFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "LICENSE"), filepath.Join(root, "ui", "fonts", "OFL.txt")}
	if !slices.Equal(paths, want) {
		t.Fatalf("license files %v, want %v", paths, want)
	}
}

// ===== Internal =====

func writeFiles(t *testing.T, files map[string]string) {
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
