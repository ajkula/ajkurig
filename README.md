# ajkurig

Ajkula's rig: a Go + Ebitengine framework for arcade games, extracted from Hordefall. It brings the parts that made Hordefall work (procedural sound and music, adaptive tracker, procedural animation, living ground, statuses and reactions, weather, menus, options, remapping, ranking, dialogue bubbles, intro, benchmarks, multi-platform release) without Hordefall's own characters and songs. Everything is a value you can change, a table you fill, or a screen you replace.

`examples/discs` is a complete small game built on it: discs with a heading dot, their sounds and statuses, burning grass, oil, infernos, weather, a four-legged boss walking with inverse kinematics, level-up cards, a talkative pilot, music that follows the action, attract demo with live benchmark.

```
go run ./examples/discs        play the example
go test ./...                  every test (headless, no window needed)
go run ./cmd/tracker           the tracker editor
go run ./cmd/release -config examples/discs/release.json   build the example's archives
```

The example lives entirely in `examples/discs` (code, languages, `release.json`, build scripts): delete that folder and the framework still builds and passes its tests.

## The rack

A game is a stack of modules mounted on a rack. A module is any struct with a `Name()` and the methods it needs; nothing else to register or inherit:

| Method | Phase |
|---|---|
| `Update(frame *rig.Frame)` | simulation, 60 ticks per second |
| `Draw(frame, screen)` | world layer: queue into `app.Batches` or draw directly |
| `DrawLate(frame, screen)` | after the batches are flushed, before bloom (colour grading) |
| `DrawHUD(frame, screen)` | after bloom (bars, text, bubbles) |
| `Reset(frame)` | new run or new demo clip |
| `Report(report)` | rows of the live benchmark panel |

```go
type Shield struct{ w *World }

func (s *Shield) Name() string                 { return "shield" }
func (s *Shield) Update(frame *rig.Frame)      { /* ... */ }
func (s *Shield) Draw(frame *rig.Frame, screen *ebiten.Image) { /* ... */ }

app.World.Mount(&Shield{w: world})
```

For a quick hook without a type: `app.World.Mount(&rig.Func{ID: "pulse", OnUpdate: func(frame *rig.Frame) { ... }})`.

Modules run in mount order. `Enable(name, false)` pauses one, `Unmount(name)` removes it, `Find(name)` returns it. Every module is timed per phase (one frame in eight, so timing costs nothing): press F1 for the table, and the main menu shows the live benchmark panel (Select hides it). `app.World` holds the game simulation; `app.Global` holds modules that run on every screen.

## Packages

| Package | What it does |
|---|---|
| `rig` | App, screens, rack, frame, settings (saved), jukebox (playlist, rotation, banner), bench |
| `shell` | Standard screens: intro, main menu over the attract demo, options, graphics, audio, playlist, controls, gameplay (language drum, high scores, reset), pause, level up, name entry, game over, ranking board. Each is a struct with exported fields; `app.SetScreen(name, yourScreen)` replaces one |
| `audio` | Synth voices, tracker with adaptive layers bound to named signals, procedural composer and arranger, sound bank, engine, music preview, WAV export |
| `input` | Actions as bits, keyboard, mouse, gamepad (standard and raw), dual sticks, remapping saved by action name |
| `ui` | Theme, text with CJK and Cyrillic fallbacks, pixel font, menus with sliders and drums, cards, bars, banners, dialogue bubbles |
| `draw` | Render scale, camera, sprite batches (solid, glow, rings), primitives |
| `postfx` | Bloom and CRT shaders |
| `fx` | Particles, rings, bolts, smoke, popups, screen shake, skid marks, weather rendering |
| `world` | Ground materials and their reactions, statuses and reactions between elements, weather |
| `procanim` | Servo, springs, two-bone IK, leg gait |
| `progress` | Experience curve, offer deck |
| `scores` | High score table, name entry, formatting |
| `intro` | The GREG's intro: voxel logo, fall, impact, shattered glass |
| `i18n` | Languages from the framework, the game and a `lang` folder next to the executable |
| `spatial`, `rng`, `store`, `mathx`, `colorx` | Grid queries, seeded random, JSON saves, math, the colour palette |

## Starting a game

```go
app := rig.NewApp(rig.Config{
    Title: "MYGAME", Languages: []fs.FS{gameLanguages},
    Remaps: remaps, EssentialRemaps: 2, Mouse: mouse,
    Sounds: sounds, Signals: []string{"action", "tension"}, Songs: songs,
})
world := NewWorld(app)                        // your shared state
app.World.Mount(world, &Arena{}, &Player{})   // your modules
shell.Mount(app, world, shell.Options{Title: "MYGAME"})
app.Run()
```

The game implements `shell.Game` (`StartRun`, `StartDemo`, `Autopilot`, `Status`) and, for level-up cards, `shell.LevelUpGame` (`OfferCards`, `ChooseOffer`). In a separate folder, its `go.mod` holds `require ajkurig v0.0.0` and `replace ajkurig => ../ajkurig`.

## Music

A song is a `.trk` text file: instruments, 16 channels, patterns, and `layer <channel> <signal> <threshold>` lines. Signal names are free; the game sets them on the engine (`board := app.Audio.Signals(); id := board.Register("action"); board.Set(id, 0.7)`). `intensity a b` lists the signals that speed the tempo up by `tempoboost`. `audio.ComposeSong(style, template, seed)` writes a new song in a style over a template's instruments and layers; `audio.DefaultTemplate()` is a ready kit, `audio.ExampleStyles` two styles. Empty channels 11 to 16 are arranged from the chords (shaker, clap, pad, pluck, offbeat, harmony).

`go run ./cmd/tracker -game path/to/song.trk` edits a song with live preview, mute and solo, presets, and sliders for each signal of the song.

Sounds are `audio.SoundDefinition` values added to a `SoundBank` (`bank.Add` gives an id). The framework's own sounds (menu, voice chirp, intro) come first and can be replaced with `bank.Replace`.

## World systems

- Ground: a grid of materials. Each material may emit a stimulus to its neighbours (fire emits heat), expire into another one, glow. Rules say what a stimulus turns a material into, with a chance out of 1024. `Ground.SpreadMultiplier` makes propagation stronger; a weather definition raises it with its intensity (a heatwave makes fire spread three times faster).
- Chemistry: statuses as bits with levels, durations, tints, auras, damage and slowdown; reaction rules element x status (with a minimum level); `StatusStore` keeps them per entity in flat arrays.
- Weather: calm and active periods, fades, colour grade, precipitation, strikes, ground stimulation around a point, temperature.

## Release

`release.json` (in the game folder) names the game: `{"name": "Discs", "binary": "discs", "package": "./examples/discs", "bundleId": "...", "category": "..."}`. `go run ajkurig/cmd/release` (or `./cmd/release` inside this repository) builds Windows amd64 and arm64 zips, a universal macOS `.app` in a tar.gz, Linux amd64 through Docker when not on Linux (the mounted folder includes every local `replace` of `go.mod`), FreeBSD natively. Archives are streamed straight from the build, so the host never touches the binaries. `-targets windows-amd64,linux-amd64` picks targets.

## Fonts and languages

Rajdhani for Latin and Devanagari, Exo 2 for Cyrillic, Noto Sans KR, JP and SC, then the Go fonts; all under the SIL Open Font License (`ui/fonts/OFL*.txt`). The fallback fonts are subsets; after adding text in another script, rebuild them with `py -3 tools/fonts/subset.py ../mygame/lang`. Hints use `{confirm}`, `{back}`, `{pause}` and `{select}`, replaced by the real button names of the current device. Tests check that every key used by the framework exists in English, that translations keep their placeholders, that every character has a glyph and that every text fits its space.

## Measured

On an i5-6200U:

| What | Before | After |
|---|---|---|
| Ground pixel refresh, 65 536 cells, every frame | 1 014 us | 53 us |
| Ground tick, 65 536 cells, every 6 frames | 969 us | 137 us |
| Rack timing overhead, 20 modules x 2 phases | 12.1 us | 3.7 us |
| Example, 10 000 discs, one tick | 28 ms | 3.3 ms |

Each optimisation is checked against the original algorithm by a test that compares the results byte for byte. Run `go test ./... -run x -bench .` to measure again.

## Colours

Every default colour comes from the palette in `colorx`: Raisin Black, Blood Red, Rich Lemon, Keppel, Electric Blue, Blushing Purple, Frostbite, Steel Pink, Pale Silver. `ui.Theme`, `intro.Settings`, `fx.Palette` and the bubble style take any colour.
