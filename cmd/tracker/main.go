package main

import (
	"flag"
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"ajkurig/cmd/tracker/internal/editor"
)

func main() {
	gameSong := flag.String("game", "", "path of the song file the game embeds, offered in Open and Save as")
	flag.Parse()
	editor.SetGameSongPath(*gameSong)
	path := flag.Arg(0)
	monitorWidth, monitorHeight := ebiten.Monitor().Size()
	scale := min(1, float64(monitorWidth)*0.92/editor.LogicalWidth, float64(monitorHeight)*0.88/editor.LogicalHeight)
	ebiten.SetWindowSize(int(editor.LogicalWidth*scale), int(editor.LogicalHeight*scale))
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("ajkurig Tracker")
	if err := ebiten.RunGame(editor.New(path)); err != nil {
		log.Fatal(err)
	}
}
