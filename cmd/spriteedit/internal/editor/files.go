package editor

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/mathx"
	"ajkurig/sprite"
)

// ===== Types =====

type Dialog uint8

// ===== Constants =====

const (
	DialogNone Dialog = iota
	DialogOpen
	DialogImport
)

const (
	dialogWidth     = 820
	dialogHeight    = 560
	dialogMargin    = 24
	dialogColumn    = 360
	fileButtonWidth = 240
	allowedNameRune = "abcdefghijklmnopqrstuvwxyz0123456789_"
)

var dialogLayouts = map[Dialog]func(e *Editor, box Rect){
	DialogOpen:   (*Editor).layoutOpenDialog,
	DialogImport: (*Editor).layoutImportDialog,
}

var dialogTitles = map[Dialog]string{
	DialogOpen:   "OPEN A SPRITE",
	DialogImport: "IMPORT MOVES FROM ANOTHER SPRITE",
}

// ===== Opening =====

func (e *Editor) showDialog(dialog Dialog) {
	e.dialog, e.importPath, e.importSheet = dialog, "", nil
}

func (e *Editor) openSprite(path string) {
	if e.isDirty && !e.isBroken {
		e.save()
	}
	if e.isDirty && !e.isBroken {
		return
	}
	width, height := e.sheet.Width, e.sheet.Height
	e.path, e.history, e.future, e.reference = path, nil, nil, nil
	e.partIndex, e.animationIndex, e.frameIndex, e.snapshotIndex = 0, 0, 0, 0
	e.selected, e.workPose = nil, nil
	e.loadOrCreate(width, height)
	e.restoreReference()
	e.dialog = DialogNone
}

func (e *Editor) createSprite() {
	name := cleanSpriteName(e.newSpriteName)
	if name == "" {
		e.notify("Type a name first: letters, digits or _", colorDanger)
		return
	}
	e.newSpriteName = ""
	e.openSprite(filepath.Join(filepath.Dir(e.path), name+sprite.SheetExtension))
}

func (e *Editor) spriteFiles() []string {
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(e.path), "*"+sprite.SheetExtension))
	if err != nil {
		return nil
	}
	slices.Sort(paths)
	return paths
}

func cleanSpriteName(name string) string {
	return strings.Map(func(character rune) rune {
		return [2]rune{-1, character}[mathx.BoolToIndex(strings.ContainsRune(allowedNameRune, character))]
	}, strings.ToLower(strings.TrimSpace(name)))
}

// ===== Importing =====

func (e *Editor) chooseImportSource(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		e.notify("Cannot read "+filepath.Base(path)+": "+err.Error(), colorDanger)
		return
	}
	source, err := sprite.Parse(string(data))
	if err != nil {
		e.notify(filepath.Base(path)+" has an error: "+err.Error(), colorDanger)
		return
	}
	source.AddHangers()
	e.importPath, e.importSheet = path, source
}

func (e *Editor) importMove(index int) {
	e.record()
	animation := e.sheet.ImportAnimation(e.importSheet, e.importSheet.Animations[index])
	animation.Name = e.uniqueAnimationName(animation.Name)
	e.sheet.Animations = append(e.sheet.Animations, animation)
	e.selectAnimation(len(e.sheet.Animations) - 1)
	e.notify(animation.Name+" imported: its bones point the same way as in "+filepath.Base(e.importPath), colorGood)
}

func (e *Editor) importSnapshots() {
	e.record()
	for _, snapshot := range e.importSheet.Snapshots {
		imported := e.sheet.ImportSnapshot(e.importSheet, snapshot)
		imported.Name = e.uniqueSnapshotName(imported.Name)
		e.sheet.Snapshots = append(e.sheet.Snapshots, imported)
	}
	e.notify("Snapshots imported, find them in 2 POSES", colorGood)
}

// ===== Layout =====

func (e *Editor) layoutDialog() {
	layout, isOpen := dialogLayouts[e.dialog]
	if !isOpen {
		return
	}
	box := Rect{(LogicalWidth - dialogWidth) / 2, (LogicalHeight - dialogHeight) / 2, dialogWidth, dialogHeight}
	e.ui.Fill(Rect{0, 0, LogicalWidth, LogicalHeight}, colorShade)
	e.ui.Fill(box, colorPanel)
	e.ui.Outline(box, colorAccent)
	e.ui.Text(box.X+dialogMargin, box.Y+dialogMargin, dialogTitles[e.dialog], colorAccent, text.AlignStart)
	layout(e, box)
	closeRect := Rect{box.X + box.W - dialogMargin - 100, box.Y + box.H - dialogMargin - buttonHeight, 100, buttonHeight}
	isClosing := e.ui.Button(closeRect, "CLOSE", false, "dialog.close") || inpututil.IsKeyJustPressed(ebiten.KeyEscape)
	e.dialog = [2]Dialog{e.dialog, DialogNone}[mathx.BoolToIndex(isClosing)]
}

func (e *Editor) layoutOpenDialog(box Rect) {
	x, y := box.X+dialogMargin, box.Y+dialogMargin+2*lineHeight
	e.ui.Text(x, y, "Sprites next to this one:", colorTextDim, text.AlignStart)
	y = e.layoutFileButtons(x, y+lineHeight+4, e.path, "open.file", e.openSprite)
	column := x + dialogColumn
	top := box.Y + dialogMargin + 2*lineHeight
	e.ui.Text(column, top, "Or start a new one, with a ready skeleton:", colorTextDim, text.AlignStart)
	e.ui.TextField("open.name", Rect{column, top + lineHeight + 4, fileButtonWidth, buttonHeight}, &e.newSpriteName)
	if e.ui.Button(Rect{column + fileButtonWidth + 8, top + lineHeight + 4, 100, buttonHeight}, "CREATE", true, "open.create") {
		e.createSprite()
	}
	note := [2]string{"", "Your changes to " + filepath.Base(e.path) + " are saved before another sprite opens."}[mathx.BoolToIndex(e.isDirty)]
	e.ui.Text(column, top+lineHeight+buttonHeight+16, note, colorAccent, text.AlignStart)
}

func (e *Editor) layoutImportDialog(box Rect) {
	x, y := box.X+dialogMargin, box.Y+dialogMargin+2*lineHeight
	e.ui.Text(x, y, "1. Pick the sprite that has the moves:", colorTextDim, text.AlignStart)
	e.layoutFileButtons(x, y+lineHeight+4, e.importPath, "import.file", e.chooseImportSource)
	column := x + dialogColumn
	if e.importSheet == nil {
		return
	}
	e.ui.Text(column, y, "2. Click a move to add it here:", colorTextDim, text.AlignStart)
	row := y + lineHeight + 4
	for index, animation := range e.importSheet.Animations {
		if e.ui.Button(Rect{column, row, fileButtonWidth, buttonHeight}, animation.Name, false, "import.move") {
			e.importMove(index)
		}
		row += buttonHeight + rowGap
	}
	if len(e.importSheet.Snapshots) > 0 && e.ui.Button(Rect{column, row + 8, fileButtonWidth, buttonHeight}, "ITS SNAPSHOTS", false, "import.snapshots") {
		e.importSnapshots()
	}
}

func (e *Editor) layoutFileButtons(x, y float32, chosen, id string, choose func(path string)) float32 {
	for _, path := range e.spriteFiles() {
		if samePath(path, e.path) && id == "import.file" {
			continue
		}
		if e.ui.Button(Rect{x, y, fileButtonWidth, buttonHeight}, sprite.SheetName(path), samePath(path, chosen), id) {
			choose(path)
		}
		y += buttonHeight + rowGap
	}
	return y
}

func samePath(first, second string) bool {
	return filepath.Clean(first) == filepath.Clean(second)
}
