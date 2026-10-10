package sprite

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// ===== Types =====

type Library struct {
	sheets  map[string]*Sheet
	atlases map[string]*Atlas
}

// ===== Constants =====

const (
	SheetExtension = ".spr"
	ClipSeparator  = "."
)

// ===== Public API =====

func LoadLibrary(fsys fs.FS, directory string) (*Library, error) {
	paths, err := fs.Glob(fsys, path.Join(directory, "*"+SheetExtension))
	if err != nil {
		return nil, err
	}
	library := &Library{sheets: map[string]*Sheet{}, atlases: map[string]*Atlas{}}
	for _, sheetPath := range paths {
		sheet, err := Load(fsys, sheetPath)
		if err != nil {
			return nil, err
		}
		library.sheets[SheetName(sheetPath)] = sheet
	}
	return library, nil
}

func SheetName(sheetPath string) string {
	return strings.TrimSuffix(path.Base(strings.ReplaceAll(sheetPath, "\\", "/")), SheetExtension)
}

func ClipName(sheetName, animationName string) string {
	return sheetName + ClipSeparator + animationName
}

func (l *Library) Names() []string {
	names := []string{}
	for sheetName, sheet := range l.sheets {
		for _, animation := range sheet.Animations {
			names = append(names, ClipName(sheetName, animation.Name))
		}
	}
	slices.Sort(names)
	return names
}

func (l *Library) Sheet(sheetName string) *Sheet {
	return l.sheets[sheetName]
}

func (l *Library) Animation(name string) (*Animation, error) {
	sheet, animationName, err := l.split(name)
	if err != nil {
		return nil, err
	}
	animation := sheet.FindAnimation(animationName)
	if animation == nil {
		return nil, fmt.Errorf("no move %q in sprite %q", animationName, strings.TrimSuffix(name, ClipSeparator+animationName))
	}
	return animation, nil
}

func (l *Library) Clip(name string) (*Clip, error) {
	if _, err := l.Animation(name); err != nil {
		return nil, err
	}
	sheetName, animationName, _ := strings.Cut(name, ClipSeparator)
	atlas, isBaked := l.atlases[sheetName]
	if !isBaked {
		atlas = Bake(l.sheets[sheetName])
		l.atlases[sheetName] = atlas
	}
	return atlas.Clips[animationName], nil
}

// ===== Internal =====

func (l *Library) split(name string) (*Sheet, string, error) {
	sheetName, animationName, isSplit := strings.Cut(name, ClipSeparator)
	if !isSplit {
		return nil, "", fmt.Errorf("a move name looks like sprite%smove, got %q", ClipSeparator, name)
	}
	sheet, isKnown := l.sheets[sheetName]
	if !isKnown {
		return nil, "", fmt.Errorf("no sprite file %s%s", sheetName, SheetExtension)
	}
	return sheet, animationName, nil
}
