package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"ajkurig/audio"
	"ajkurig/input"
	"ajkurig/rig"
	"ajkurig/shell"
)

// ===== Constants =====

//go:embed lang/*.json
var languageFiles embed.FS

var (
	ActionFire = input.GameAction(0)
	ActionDash = input.GameAction(1)
)

var remaps = []input.RemapDefinition{
	{
		Key: "fire", Primary: ActionFire, KeyboardActions: ActionFire | input.ActionConfirm, GamepadActions: ActionFire | input.ActionConfirm,
		DefaultKeys: []ebiten.Key{ebiten.KeyJ, ebiten.KeyZ}, DefaultButtons: []ebiten.StandardGamepadButton{ebiten.StandardGamepadButtonRightBottom}, DefaultGamepadLabel: "A",
	},
	{
		Key: "dash", Primary: ActionDash, KeyboardActions: ActionDash | input.ActionBack, GamepadActions: ActionDash | input.ActionBack,
		DefaultKeys: []ebiten.Key{ebiten.KeySpace, ebiten.KeyK}, DefaultButtons: []ebiten.StandardGamepadButton{ebiten.StandardGamepadButtonRightRight}, DefaultGamepadLabel: "B",
	},
}

var mouse = []input.MouseBinding{
	{Button: ebiten.MouseButtonLeft, Action: ActionFire},
	{Button: ebiten.MouseButtonRight, Action: ActionDash},
}

// ===== Public API =====

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// ===== Internal =====

func run() error {
	languages, err := fs.Sub(languageFiles, "lang")
	if err != nil {
		return err
	}
	sounds := audio.NewSoundBank()
	app := rig.NewApp(rig.Config{
		Title:           "DISCS",
		SaveFolder:      "ajkurig-discs",
		Languages:       []fs.FS{languages},
		Remaps:          remaps,
		EssentialRemaps: len(remaps),
		Mouse:           mouse,
		Sounds:          sounds,
		Signals:         []string{SignalAction, SignalTension, SignalCombo, SignalDanger},
		Songs:           composeSongs(),
	})
	world := NewWorld(app, RegisterSounds(sounds))
	world.MountModules()
	shell.Mount(app, world, shell.Options{
		Title:         "DISCS",
		GameOverTitle: "DISCS WIN",
		ControlsHint:  "Move: stick / WASD    Fire: A / J / mouse    Dash: B / Space    Pause: Start / Esc",
		AttractArt:    world.DrawAttractArt,
	})
	return app.Run()
}

func composeSongs() []*audio.Song {
	template := audio.DefaultTemplate()
	songs := make([]*audio.Song, 0, len(audio.ExampleStyles))
	for index := range audio.ExampleStyles {
		songs = append(songs, audio.ComposeSong(&audio.ExampleStyles[index], template, uint32(index+1)*7919))
	}
	return songs
}
