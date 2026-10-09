package i18n

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ===== Types =====

type Language struct {
	Code    string            `json:"code"`
	Name    string            `json:"name"`
	Script  string            `json:"script"`
	Strings map[string]string `json:"strings"`
}

// ===== Constants =====

const (
	FallbackCode    = "en"
	DefaultCode     = "default"
	externalFolder  = "lang"
	languagePattern = "*.json"
)

//go:embed lang/*.json
var frameworkLanguages embed.FS

var (
	languages = map[string]*Language{}
	current   *Language
	fallback  *Language
)

// ===== Public API =====

func FrameworkSource() fs.FS {
	source, _ := fs.Sub(frameworkLanguages, "lang")
	return source
}

func Load(sources []fs.FS, externalDirectories ...string) error {
	clear(languages)
	problems := []error{loadSource(FrameworkSource())}
	for _, source := range sources {
		problems = append(problems, loadSource(source))
	}
	for _, directory := range externalDirectories {
		loadDirectory(filepath.Join(directory, externalFolder))
	}
	fallback = languages[FallbackCode]
	current = fallback
	return errors.Join(problems...)
}

func Languages() []Language {
	list := make([]Language, 0, len(languages))
	for _, language := range languages {
		list = append(list, *language)
	}
	slices.SortFunc(list, compareLanguages)
	return list
}

func Use(code string) bool {
	language, exists := languages[code]
	current = [2]*Language{fallback, language}[boolToIndex(exists)]
	return exists
}

func Current() Language {
	if current == nil {
		return Language{Code: FallbackCode, Name: "English", Script: "latin"}
	}
	return *current
}

func Has(code string) bool {
	_, exists := languages[code]
	return exists
}

func HasKey(code, key string) bool {
	_, exists := lookup(languages[code], key)
	return exists
}

func T(key string) string {
	return TOr(key, key)
}

func TOr(key, missing string) string {
	if text, exists := lookup(current, key); exists {
		return text
	}
	if text, exists := lookup(fallback, key); exists {
		return text
	}
	return missing
}

func F(key string, arguments ...any) string {
	return fmt.Sprintf(T(key), arguments...)
}

func Value(label string) string {
	return TOr("value."+strings.ToLower(label), label)
}

func Detect() string {
	for _, locale := range systemLocales() {
		code := matchLocale(locale)
		if code != "" {
			return code
		}
	}
	return FallbackCode
}

// ===== Internal =====

func lookup(language *Language, key string) (string, bool) {
	if language == nil {
		return "", false
	}
	text, exists := language.Strings[key]
	return text, exists && text != ""
}

func loadSource(source fs.FS) error {
	if source == nil {
		return nil
	}
	paths, err := fs.Glob(source, languagePattern)
	if err != nil {
		return err
	}
	var problems []error
	for _, path := range paths {
		problems = append(problems, registerFile(source, path))
	}
	return errors.Join(problems...)
}

func registerFile(source fs.FS, path string) error {
	data, err := fs.ReadFile(source, path)
	if err == nil {
		err = register(data)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func loadDirectory(directory string) {
	paths, _ := filepath.Glob(filepath.Join(directory, languagePattern))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		_ = register(data)
	}
}

func register(data []byte) error {
	var language Language
	if err := json.Unmarshal(data, &language); err != nil {
		return err
	}
	if language.Code == "" {
		return fmt.Errorf("a language file needs a code")
	}
	if language.Strings == nil {
		language.Strings = map[string]string{}
	}
	existing, isOverride := languages[language.Code]
	if isOverride {
		mergeLanguage(existing, &language)
		return nil
	}
	if language.Name == "" {
		return fmt.Errorf("a new language needs a name")
	}
	languages[language.Code] = &language
	return nil
}

func mergeLanguage(language, addition *Language) {
	language.Name = [2]string{language.Name, addition.Name}[boolToIndex(addition.Name != "")]
	language.Script = [2]string{language.Script, addition.Script}[boolToIndex(addition.Script != "")]
	for key, text := range addition.Strings {
		language.Strings[key] = text
	}
}

func matchLocale(locale string) string {
	normalized := strings.ToLower(strings.ReplaceAll(locale, "_", "-"))
	primary, _, _ := strings.Cut(normalized, "-")
	primary, _, _ = strings.Cut(primary, ".")
	if Has(primary) {
		return primary
	}
	return ""
}

func compareLanguages(a, b Language) int {
	order := boolToIndex(a.Code != FallbackCode) - boolToIndex(b.Code != FallbackCode)
	if order != 0 {
		return order
	}
	return strings.Compare(a.Code, b.Code)
}

func boolToIndex(value bool) int {
	if value {
		return 1
	}
	return 0
}
