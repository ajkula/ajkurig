package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"ajkurig/cmd/spriteedit/internal/editor"
)

func main() {
	size := flag.String("size", "48x64", "width x height of a new sprite, in pixels")
	snapshot := flag.String("snapshot", "", "saves the editor window to this PNG after a few frames, then quits")
	flag.Parse()
	path := flag.Arg(0)
	if path == "" {
		path = "sprite.spr"
	}
	var width, height int
	if _, err := fmt.Sscanf(*size, "%dx%d", &width, &height); err != nil || width < 1 || height < 1 {
		log.Fatalf("bad -size %q, expected something like 48x64", *size)
	}
	monitorWidth, monitorHeight := ebiten.Monitor().Size()
	scale := min(1, float64(monitorWidth)*0.94/editor.LogicalWidth, float64(monitorHeight)*0.9/editor.LogicalHeight)
	ebiten.SetWindowSize(int(editor.LogicalWidth*scale), int(editor.LogicalHeight*scale))
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("ajkurig Sprite Editor")
	sprites := editor.New(path, width, height)
	sprites.SetSnapshot(*snapshot)
	if err := ebiten.RunGame(sprites); err != nil {
		log.Fatal(err)
	}
}
