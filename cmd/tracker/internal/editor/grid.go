package editor

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/cmd/tracker/internal/song"
)

// ===== Constants =====

const (
	toolbarHeight   = 40
	sidebarWidth    = 200
	gridTop         = toolbarHeight
	headerHeight    = 58
	minimumRowH     = 18
	maximumRowH     = 34
	gridBottom      = 640
	rowNumberWidth  = 36
	channelWidth    = (LogicalWidth - sidebarWidth - rowNumberWidth) / song.Channels
	cellTextOffset  = 4
	rowsTop         = gridTop + headerHeight
	pianoHeight     = 48
	pianoTop        = gridBottom - pianoHeight - 2
	gridRowsHeight  = pianoTop - 4 - rowsTop
	rowsBottom      = rowsTop + gridRowsHeight
	beatRows        = 4
	barRows         = 16
	wheelRows       = 3
	headerButtonGap = 2
)

var fieldColumns = [3]struct {
	Start int
	Width int
}{{0, 3}, {4, 2}, {7, 3}}

// ===== Internal =====

func (e *Editor) layoutGrid() {
	pattern := e.currentPattern()
	e.ui.Fill(Rect{sidebarWidth, gridTop, LogicalWidth - sidebarWidth, gridBottom - gridTop}, colorPanel)
	for channel := range song.Channels {
		e.layoutChannelHeader(channel)
	}
	e.scrollGridIfWheeled(len(pattern.Rows))
	playingRow := e.playingRow()
	for visible := range e.visibleRows() {
		e.layoutRow(pattern, e.scrollRow+visible, playingRow)
	}
	e.drawChannelSeparators()
	e.layoutPiano()
	e.handleGridMouse(pattern)
}

func (e *Editor) drawChannelSeparators() {
	for channel := range song.Channels + 1 {
		x := float32(sidebarWidth + rowNumberWidth + channel*channelWidth - 1)
		e.ui.Fill(Rect{x, rowsTop, 1, rowsBottom - rowsTop}, colorSeparator)
	}
}

func (e *Editor) layoutChannelHeader(channel int) {
	x := float32(sidebarWidth + rowNumberWidth + channel*channelWidth)
	rect := Rect{x, gridTop, channelWidth - 1, headerHeight - 2}
	isCurrent := channel == e.cursorChannel
	e.ui.Fill(rect, [2]color.Color{colorPanelLight, colorButtonHot}[boolToIndex(isCurrent)])
	nameColor := [2]color.Color{colorText, colorAccent}[boolToIndex(isCurrent)]
	e.ui.Text(x+4, gridTop+3, fmt.Sprintf("%d %s", channel+1, e.document.ChannelNames[channel]), e.ui.Small, nameColor, text.AlignStart)
	layer := e.document.Layers[channel]
	e.ui.Text(x+4, gridTop+17, fmt.Sprintf("%s %s", layer.Signal, formatThreshold(layer.Threshold)), e.ui.Small, colorTextDim, text.AlignStart)
	half := (channelWidth - 8 - headerButtonGap) / 2
	if e.ui.Button(Rect{x + 3, gridTop + 33, float32(half), 15}, "M", e.muted[channel], "Mute this channel while previewing") {
		e.muted[channel] = !e.muted[channel]
		e.isStale = true
	}
	if e.ui.Button(Rect{x + 3 + float32(half+headerButtonGap), gridTop + 33, float32(half), 15}, "S", e.solo == channel, "Solo: hear only this channel") {
		e.solo = [2]int{channel, noSolo}[boolToIndex(e.solo == channel)]
		e.isStale = true
	}
	panWidth := (channelWidth - 8) * e.document.Pans[channel]
	e.ui.Fill(Rect{x + 3, gridTop + 50, channelWidth - 8, 3}, colorPanel)
	e.ui.Fill(Rect{x + 3 + panWidth - 1, gridTop + 49, 3, 5}, colorAccent)
	e.selectChannelIfClicked(rect, channel)
}

func (e *Editor) selectChannelIfClicked(rect Rect, channel int) {
	if !rect.Contains(e.ui.MouseX, e.ui.MouseY) || !e.ui.IsClickFree() {
		return
	}
	e.cursorChannel = channel
	e.ui.ConsumeClick()
}

func (e *Editor) layoutRow(pattern *song.Pattern, row, playingRow int) {
	if row >= len(pattern.Rows) {
		return
	}
	height := e.rowHeight()
	y := rowsTop + float32(row-e.scrollRow)*height
	rowRect := Rect{sidebarWidth, y, LogicalWidth - sidebarWidth, height}
	textY := y + (height-fontSize)/2 - 2
	e.fillRowBackground(rowRect, row, playingRow)
	numberColor := [2]color.Color{colorTextDim, colorAccent}[boolToIndex(row%barRows == 0)]
	e.ui.Text(sidebarWidth+rowNumberWidth-6, textY, fmt.Sprintf("%02X", row), e.ui.Regular, numberColor, text.AlignEnd)
	for channel := range song.Channels {
		e.layoutCell(pattern, row, channel, y, textY)
	}
}

func (e *Editor) fillRowBackground(rect Rect, row, playingRow int) {
	isBeat := row%beatRows == 0
	if isBeat {
		e.ui.Fill(rect, colorBeat)
	}
	if row == playingRow {
		e.ui.Fill(rect, colorPlayhead)
	}
	if row == e.cursorRow {
		e.ui.Fill(rect, color.NRGBA{255, 255, 255, 14})
	}
}

func (e *Editor) layoutCell(pattern *song.Pattern, row, channel int, y, textY float32) {
	x := float32(sidebarWidth + rowNumberWidth + channel*channelWidth)
	cell := pattern.Rows[row][channel]
	isGhost := cell.IsEmpty() && channel >= song.ArrangedChannel && e.document.IsChannelEmpty(pattern.ID, channel)
	shown := cell
	if isGhost {
		shown = e.arrangedCell(pattern.ID, row, channel)
	}
	if e.isSelected(row, channel) {
		e.ui.Fill(Rect{x, y, channelWidth, e.rowHeight()}, colorSelection)
	}
	e.drawCellText(shown, x+cellTextOffset, textY, isGhost)
	isCursor := row == e.cursorRow && channel == e.cursorChannel
	if !isCursor {
		return
	}
	field := fieldColumns[e.cursorField]
	e.ui.Outline(Rect{x + cellTextOffset - 1 + float32(field.Start)*charWidth, y, float32(field.Width)*charWidth + 2, e.rowHeight()}, colorAccent)
}

func (e *Editor) drawCellText(cell song.Cell, x, y float32, isGhost bool) {
	noteColors := [2]color.Color{colorText, colorGhost}
	detailColors := [2]color.Color{colorAccent, colorGhost}
	effectColors := [2]color.Color{rgb(0.55, 0.8, 1), colorGhost}
	emptyColor := rgb(0.25, 0.27, 0.35)
	ghost := boolToIndex(isGhost)
	e.ui.Text(x, y, song.FormatNote(cell.Note), e.ui.Regular, pickColor(cell.Note == song.NoteEmpty, emptyColor, noteColors[ghost]), text.AlignStart)
	e.ui.Text(x+4*charWidth, y, song.FormatInstrument(cell.Instrument), e.ui.Regular, pickColor(cell.Instrument == 0, emptyColor, detailColors[ghost]), text.AlignStart)
	e.ui.Text(x+7*charWidth, y, song.FormatEffect(cell.Effect, cell.Parameter), e.ui.Regular, pickColor(cell.Effect == song.EffectNone, emptyColor, effectColors[ghost]), text.AlignStart)
}

func (e *Editor) arrangedCell(patternID, row, channel int) song.Cell {
	if e.parsed == nil {
		return song.EmptyCell()
	}
	pattern, exists := e.parsed.Patterns[patternID]
	if !exists || row >= len(pattern.Rows) {
		return song.EmptyCell()
	}
	arranged := pattern.Rows[row][channel]
	return song.Cell{Note: arranged.Note, Instrument: arranged.Instrument, Effect: arranged.Effect, Parameter: arranged.Parameter}
}

func (e *Editor) scrollGridIfWheeled(rowCount int) {
	isOverGrid := Rect{sidebarWidth, rowsTop, LogicalWidth - sidebarWidth, rowsBottom - rowsTop}.Contains(e.ui.MouseX, e.ui.MouseY)
	if !isOverGrid || e.ui.WheelY == 0 {
		return
	}
	e.scrollRow = min(max(e.scrollRow-int(sign(e.ui.WheelY))*wheelRows, 0), max(0, rowCount-e.visibleRows()))
}

func (e *Editor) handleGridMouse(pattern *song.Pattern) {
	row, channel, field, isInside := e.cellAt(e.ui.MouseX, e.ui.MouseY, len(pattern.Rows))
	if isInside && e.ui.IsRightClicked && e.overlay == OverlayNone {
		e.edit("", func() { pattern.Rows[row][channel] = song.EmptyCell() })
	}
	if isInside && e.ui.IsClickFree() && e.overlay == OverlayNone {
		e.moveCursorTo(row, channel, field)
		e.selection = Selection{AnchorRow: row, AnchorColumn: channel, EndRow: row, EndColumn: channel}
		e.isMouseSelecting = true
		e.ui.ConsumeClick()
	}
	if e.isMouseSelecting && e.ui.IsMouseDown && isInside {
		e.selection.EndRow, e.selection.EndColumn = row, channel
		e.selection.IsActive = row != e.selection.AnchorRow || channel != e.selection.AnchorColumn
	}
	e.isMouseSelecting = e.isMouseSelecting && e.ui.IsMouseDown
}

func (e *Editor) cellAt(x, y float32, rowCount int) (int, int, int, bool) {
	column := int((x - sidebarWidth - rowNumberWidth) / channelWidth)
	row := e.scrollRow + int((y-rowsTop)/e.rowHeight())
	isInside := x >= sidebarWidth+rowNumberWidth && y >= rowsTop && y < rowsBottom && column < song.Channels && row < rowCount
	offset := (x - sidebarWidth - rowNumberWidth - float32(column*channelWidth) - cellTextOffset) / charWidth
	field := min(boolToIndex(offset >= 3.5)+boolToIndex(offset >= 6.5), fieldEffect)
	return row, column, field, isInside
}

func (e *Editor) moveCursorTo(row, channel, field int) {
	e.cursorRow, e.cursorChannel, e.cursorField, e.digitIndex = row, channel, field, 0
	e.clampCursor()
}

func (e *Editor) clampCursor() {
	pattern := e.currentPattern()
	e.cursorRow = min(max(e.cursorRow, 0), len(pattern.Rows)-1)
	e.cursorChannel = min(max(e.cursorChannel, 0), song.Channels-1)
	e.ensureRowVisible(e.cursorRow)
}

func (e *Editor) ensureRowVisible(row int) {
	rowCount := len(e.currentPattern().Rows)
	e.scrollRow = min(max(e.scrollRow, row-e.visibleRows()+1), row)
	e.scrollRow = min(max(e.scrollRow, 0), max(0, rowCount-e.visibleRows()))
}

func (e *Editor) rowHeight() float32 {
	rowCount := float32(len(e.currentPattern().Rows))
	return clamp(float32(int(gridRowsHeight/rowCount)), minimumRowH, maximumRowH)
}

func (e *Editor) visibleRows() int {
	return int(gridRowsHeight / e.rowHeight())
}

func (e *Editor) isSelected(row, channel int) bool {
	if !e.selection.IsActive {
		return false
	}
	top, bottom, left, right := e.selectionBounds()
	return row >= top && row <= bottom && channel >= left && channel <= right
}

func (e *Editor) selectionBounds() (int, int, int, int) {
	selection := e.selection
	return min(selection.AnchorRow, selection.EndRow), max(selection.AnchorRow, selection.EndRow),
		min(selection.AnchorColumn, selection.EndColumn), max(selection.AnchorColumn, selection.EndColumn)
}

func formatThreshold(value float32) string {
	return fmt.Sprintf("%g", value)
}

func pickColor(isEmpty bool, empty, filled color.Color) color.Color {
	return [2]color.Color{filled, empty}[boolToIndex(isEmpty)]
}
