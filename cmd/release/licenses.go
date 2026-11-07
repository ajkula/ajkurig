package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// ===== Types =====

type BuildModule struct {
	Path    string
	Version string
	Dir     string
	Files   []string
}

// ===== Constants =====

const (
	licensesFileName = "THIRD_PARTY_LICENSES.txt"
	moduleListFormat = "{{with .Module}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}"
)

var licenseFilePrefixes = []string{"LICENSE", "LICENCE", "COPYING", "NOTICE", "OFL"}

var skippedLicenseDirectories = []string{".git", "vendor", "testdata", "node_modules", "examples", "_examples", distDirectory}

var licenseFileExtensions = []string{"", ".txt", ".md"}

var licenseSystems = []string{"windows", "darwin", "linux"}

// ===== Internal =====

func collectLicenses(release *Release) ([]byte, error) {
	modules, err := buildModules(release)
	if err != nil {
		return nil, err
	}
	text := new(bytes.Buffer)
	fmt.Fprintf(text, "%s bundles the following software. Each license text follows its module.\n", release.Config.Name)
	for _, module := range modules {
		if err := appendModuleLicenses(text, module); err != nil {
			return nil, err
		}
	}
	return text.Bytes(), nil
}

func buildModules(release *Release) ([]BuildModule, error) {
	modules := make([]BuildModule, 0, 16)
	for _, goos := range licenseSystems {
		command := exec.Command("go", "list", "-deps", "-f", moduleListFormat, release.Config.Package)
		command.Dir = release.SourceDir
		command.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=1")
		output, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("list the modules built for %s: %w", goos, err)
		}
		modules = append(modules, parseBuildModules(string(output), release.SourceDir)...)
	}
	goRoot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return nil, err
	}
	root := strings.TrimSpace(string(goRoot))
	modules = append(modules, BuildModule{Path: "Go standard library", Version: runtime.Version(), Dir: root, Files: []string{filepath.Join(root, "LICENSE")}})
	slices.SortFunc(modules, func(first, second BuildModule) int { return strings.Compare(first.Path, second.Path) })
	return slices.CompactFunc(modules, func(first, second BuildModule) bool { return first.Path == second.Path }), nil
}

func parseBuildModules(output, gameDir string) []BuildModule {
	modules := make([]BuildModule, 0, 16)
	for line := range strings.Lines(output) {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		isDependency := len(fields) == 3 && filepath.Clean(fields[len(fields)-1]) != filepath.Clean(gameDir)
		modules = appendIf(modules, BuildModule{Path: fields[0], Version: fields[min(1, len(fields)-1)], Dir: fields[len(fields)-1]}, isDependency)
	}
	return modules
}

func appendModuleLicenses(text *bytes.Buffer, module BuildModule) error {
	paths, err := module.Files, error(nil)
	if paths == nil {
		paths, err = licenseFiles(module.Dir)
	}
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		fmt.Printf("warn   %-16s no license file in %s\n", "licenses", module.Path)
		fmt.Fprintf(text, "\n\n==== %s: no license file found ====\n", module.Title())
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(module.Dir, path)
		fmt.Fprintf(text, "\n\n==== %s: %s ====\n\n%s", module.Title(), filepath.ToSlash(relative), bytes.TrimSpace(content))
	}
	return nil
}

func licenseFiles(root string) ([]string, error) {
	paths := make([]string, 0, 4)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return skipDirectoryIf(path != root && slices.Contains(skippedLicenseDirectories, entry.Name()))
		}
		paths = appendIf(paths, path, isLicenseFile(entry.Name()))
		return nil
	})
	return paths, err
}

func skipDirectoryIf(shouldSkip bool) error {
	if !shouldSkip {
		return nil
	}
	return filepath.SkipDir
}

func isLicenseFile(name string) bool {
	upper := strings.ToUpper(name)
	hasLicenseName := slices.ContainsFunc(licenseFilePrefixes, func(prefix string) bool { return strings.HasPrefix(upper, prefix) })
	return hasLicenseName && slices.Contains(licenseFileExtensions, strings.ToLower(filepath.Ext(name)))
}

func (m BuildModule) Title() string {
	return strings.TrimSpace(m.Path + " " + m.Version)
}

func licenseEntry(release *Release, folder string) ArchiveEntry {
	return ArchiveEntry{Name: folder + licensesFileName, Mode: 0o644, Content: release.Licenses}
}
