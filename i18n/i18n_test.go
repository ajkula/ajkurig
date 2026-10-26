package i18n

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// ===== Constants =====

var keyPattern = regexp.MustCompile(`(?:i18n\.(?:T|F|TOr)|Label|OnOffLabel|Hint)\("([a-z_]+\.[a-z0-9_.]+)"`)

var placeholderPattern = regexp.MustCompile(`%[-0-9.]*[a-zA-Z]|\{[a-z]+\}`)

// ===== Public API =====

func TestEveryFrameworkKeyExistsInEnglish(t *testing.T) {
	if err := Load(nil); err != nil {
		t.Fatal(err)
	}
	missing := []string{}
	keys := keysUsedByTheFramework(t)
	if len(keys) < 40 {
		t.Fatalf("only %d keys found in the framework code, the scan is broken", len(keys))
	}
	for _, key := range keys {
		missing = appendIfMissing(missing, key)
	}
	if len(missing) > 0 {
		t.Fatalf("keys used by the code but missing in en.json: %v", missing)
	}
}

func TestTranslationsKeepPlaceholders(t *testing.T) {
	if err := Load(nil); err != nil {
		t.Fatal(err)
	}
	english := languages[FallbackCode]
	for _, language := range Languages() {
		for key, text := range language.Strings {
			expected := placeholderPattern.FindAllString(english.Strings[key], -1)
			found := placeholderPattern.FindAllString(text, -1)
			slices.Sort(expected)
			slices.Sort(found)
			if !slices.Equal(expected, found) {
				t.Errorf("%s %s: placeholders %v, English has %v", language.Code, key, found, expected)
			}
		}
	}
}

func TestGameFilesOverrideAndAddLanguages(t *testing.T) {
	game := fstest.MapFS{
		"en.json": {Data: []byte(`{"code": "en", "strings": {"menu.play": "Launch", "game.only": "Only here"}}`)},
		"eo.json": {Data: []byte(`{"code": "eo", "name": "Esperanto", "strings": {"menu.play": "Ludi"}}`)},
	}
	if err := Load([]fs.FS{game}); err != nil {
		t.Fatal(err)
	}
	if T("menu.play") != "Launch" || T("game.only") != "Only here" || T("menu.quit") == "menu.quit" {
		t.Fatalf("override failed: %q %q %q", T("menu.play"), T("game.only"), T("menu.quit"))
	}
	Use("eo")
	if T("menu.play") != "Ludi" || T("menu.quit") != languages[FallbackCode].Strings["menu.quit"] {
		t.Fatal("a new language does not fall back to English for missing keys")
	}
	if !Use("xx") && Current().Code != FallbackCode {
		t.Fatal("an unknown language must fall back to English")
	}
}

func TestBrokenFilesAreSkippedNotFatal(t *testing.T) {
	game := fstest.MapFS{"zz.json": {Data: []byte(`{broken`)}}
	err := Load([]fs.FS{game})
	if err == nil || !Has(FallbackCode) {
		t.Fatal("a broken file must be reported and skipped while the rest loads")
	}
}

func TestMissingKeysFallBackToEnglishThenToTheKey(t *testing.T) {
	if err := Load(nil); err != nil {
		t.Fatal(err)
	}
	Use("fr")
	delete(languages["fr"].Strings, "menu.play")
	if T("menu.play") != languages[FallbackCode].Strings["menu.play"] || T("no.such.key") != "no.such.key" || TOr("no.such.key", "x") != "x" {
		t.Fatalf("fallbacks: %q %q", T("menu.play"), T("no.such.key"))
	}
	if Use("xx") || Current().Code != FallbackCode {
		t.Fatal("an unknown language did not fall back to English")
	}
}

func TestExternalFilesAddAndOverrideLanguages(t *testing.T) {
	directory := t.TempDir()
	folder := filepath.Join(directory, externalFolder)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"pt.json":     `{"code":"pt","name":"Português","script":"latin","strings":{"menu.play":"Jogar"}}`,
		"fr.json":     `{"code":"fr","name":"Français","strings":{"menu.quit":"Ciao"}}`,
		"broken.json": `{"code":`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Load(nil, directory); err != nil {
		t.Fatal(err)
	}
	english := languages[FallbackCode].Strings
	Use("pt")
	if T("menu.play") != "Jogar" || T("menu.quit") != english["menu.quit"] {
		t.Fatalf("external language: %q %q", T("menu.play"), T("menu.quit"))
	}
	Use("fr")
	french := map[string]string{}
	_ = json.Unmarshal(mustRead(t, filepath.Join("lang", "fr.json")), &struct {
		Strings *map[string]string `json:"strings"`
	}{&french})
	if T("menu.quit") != "Ciao" || T("menu.options") != french["menu.options"] {
		t.Fatalf("override: %q %q", T("menu.quit"), T("menu.options"))
	}
}

func TestNothingCrashesWithoutLanguages(t *testing.T) {
	saved, savedCurrent, savedFallback := languages, current, fallback
	defer func() { languages, current, fallback = saved, savedCurrent, savedFallback }()
	languages, current, fallback = map[string]*Language{}, nil, nil
	if T("menu.play") != "menu.play" || F("hud.level", 3) == "" || Current().Code != FallbackCode || Use("fr") {
		t.Fatal("empty catalog misbehaved")
	}
}

// ===== Internal =====

func keysUsedByTheFramework(t *testing.T) []string {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, folder := range []string{"rig", "shell", "ui"} {
		paths, _ := filepath.Glob(filepath.Join(root, folder, "*.go"))
		for _, path := range paths {
			keys = append(keys, keysIn(t, path)...)
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

func keysIn(t *testing.T, path string) []string {
	if strings.HasSuffix(path, "_test.go") {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, match := range keyPattern.FindAllStringSubmatch(string(content), -1) {
		keys = append(keys, match[1])
	}
	return keys
}

func mustRead(t *testing.T, path string) []byte {
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func appendIfMissing(missing []string, key string) []string {
	if HasKey(FallbackCode, key) {
		return missing
	}
	return append(missing, key)
}
