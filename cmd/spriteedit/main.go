package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"ajkurig/spriteeditor"
)

func main() {
	size := flag.String("size", "48x64", "width x height of a new sprite, in pixels")
	snapshot := flag.String("snapshot", "", "saves the editor window to this PNG after a few frames, then quits")
	reference := flag.String("ref", "", "a picture shown under the drawing to trace over (dropping one on the window works too)")
	step := flag.Int("step", 1, "step to open on: 1 skeleton, 2 poses, 3 draw, 4 animate")
	move := flag.String("move", "", "move to open in the animate step")
	part := flag.String("part", "", "part to select")
	turn := flag.Float64("turn", 0, "turn of the whole body while drawing, in degrees")
	tilt := flag.Float64("tilt", 0, "tilt of the whole body while drawing, in degrees: 90 looks from the top")
	flag.Parse()
	path := flag.Arg(0)
	if path == "" {
		path = "sprite.spr"
	}
	var width, height int
	if _, err := fmt.Sscanf(*size, "%dx%d", &width, &height); err != nil || width < 1 || height < 1 {
		log.Fatalf("bad -size %q, expected something like 48x64", *size)
	}
	spriteeditor.ConfigureWindow()
	sprites := spriteeditor.New(path, width, height)
	sprites.SetSnapshot(*snapshot)
	sprites.Open(spriteeditor.Opening{Step: *step, Move: *move, Part: *part, Turn: *turn, Tilt: *tilt})
	if *reference != "" {
		sprites.SetReference(*reference)
	}
	if err := ebiten.RunGame(sprites); err != nil {
		log.Fatal(err)
	}
}
