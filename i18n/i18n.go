package i18n

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ajkula/ajkurig/mathx"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
)

// ===== Types =====

type Language struct {
	Code    string            `json:"code"`
	Name    string            `json:"name"`
	Script  string            `json:"script"`
	Strings map[string]string `json:"strings"`
}

type catalog struct {
	languages map[string]*Language
	current   *Language
	fallback  *Language
}

// ===== Constants =====

var rightToLeftScripts = map[string]bool{"arabic": true}

const (
	FallbackCode    = "en"
	DefaultCode     = "default"
	externalFolder  = "lang"
	languagePattern = "*.json"
)

//go:embed lang/*.json
var frameworkLanguages embed.FS

var active = newActiveCatalog()

// ===== Public API =====

func FrameworkSource() fs.FS {
	source, _ := fs.Sub(frameworkLanguages, "lang")
	return source
}

func Load(sources []fs.FS, externalDirectories ...string) error {
	loaded := newCatalog()
	problems := []error{loaded.loadSource(FrameworkSource())}
	for _, source := range sources {
		problems = append(problems, loaded.loadSource(source))
	}
	for _, directory := range externalDirectories {
		loaded.loadDirectory(filepath.Join(directory, externalFolder))
	}
	loaded.fallback = loaded.languages[FallbackCode]
	loaded.current = loaded.fallback
	active.Store(loaded)
	return errors.Join(problems...)
}

func Languages() []Language {
	languages := active.Load().languages
	list := make([]Language, 0, len(languages))
	for _, language := range languages {
		list = append(list, *language)
	}
	slices.SortFunc(list, compareLanguages)
	return list
}

func Use(code string) bool {
	for {
		snapshot := active.Load()
		language, exists := snapshot.languages[code]
		switched := &catalog{languages: snapshot.languages, fallback: snapshot.fallback, current: mathx.Pick(exists, snapshot.fallback, language)}
		if active.CompareAndSwap(snapshot, switched) {
			return exists
		}
	}
}

func IsRightToLeft() bool {
	current := active.Load().current
	return current != nil && rightToLeftScripts[current.Script]
}

func Current() Language {
	current := active.Load().current
	if current == nil {
		return Language{Code: FallbackCode, Name: "English", Script: "latin"}
	}
	return *current
}

func Has(code string) bool {
	_, exists := active.Load().languages[code]
	return exists
}

func HasKey(code, key string) bool {
	_, exists := lookup(active.Load().languages[code], key)
	return exists
}

func T(key string) string {
	return TOr(key, key)
}

func TOr(key, missing string) string {
	snapshot := active.Load()
	if text, exists := lookup(snapshot.current, key); exists {
		return text
	}
	if text, exists := lookup(snapshot.fallback, key); exists {
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

func newActiveCatalog() *atomic.Pointer[catalog] {
	var pointer atomic.Pointer[catalog]
	pointer.Store(newCatalog())
	return &pointer
}

func newCatalog() *catalog {
	return &catalog{languages: map[string]*Language{}}
}

func lookup(language *Language, key string) (string, bool) {
	if language == nil {
		return "", false
	}
	text, exists := language.Strings[key]
	return text, exists && text != ""
}

func (c *catalog) loadSource(source fs.FS) error {
	if source == nil {
		return nil
	}
	paths, err := fs.Glob(source, languagePattern)
	if err != nil {
		return err
	}
	var problems []error
	for _, path := range paths {
		problems = append(problems, c.registerFile(source, path))
	}
	return errors.Join(problems...)
}

func (c *catalog) registerFile(source fs.FS, path string) error {
	data, err := fs.ReadFile(source, path)
	if err == nil {
		err = c.register(data)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func (c *catalog) loadDirectory(directory string) {
	paths, _ := filepath.Glob(filepath.Join(directory, languagePattern))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		_ = c.register(data)
	}
}

func (c *catalog) register(data []byte) error {
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
	existing, isOverride := c.languages[language.Code]
	if isOverride {
		mergeLanguage(existing, &language)
		return nil
	}
	if language.Name == "" {
		return fmt.Errorf("a new language needs a name")
	}
	c.languages[language.Code] = &language
	return nil
}

func mergeLanguage(language, addition *Language) {
	language.Name = [2]string{language.Name, addition.Name}[mathx.BoolToIndex(addition.Name != "")]
	language.Script = [2]string{language.Script, addition.Script}[mathx.BoolToIndex(addition.Script != "")]
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
	order := mathx.BoolToIndex(a.Code != FallbackCode) - mathx.BoolToIndex(b.Code != FallbackCode)
	if order != 0 {
		return order
	}
	return strings.Compare(a.Code, b.Code)
}
