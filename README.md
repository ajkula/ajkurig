# ajkurig

Ajkula's rig: a Go + Ebitengine framework for arcade games, extracted from Hordefall. It brings the parts that made Hordefall work (procedural sound and music, adaptive tracker, procedural animation, living ground, statuses and reactions, weather, menus, options, remapping, ranking, dialogue bubbles, intro, benchmarks, multi-platform release) without Hordefall's own characters and songs. Everything is a value you can change, a table you fill, or a screen you replace.

`examples/discs` is a complete small game built on it: discs with a heading dot, their sounds and statuses, burning grass, oil, infernos, weather, a four-legged boss walking with inverse kinematics, level-up cards, a talkative pilot, music that follows the action, attract demo with live benchmark.

```
go run ./examples/discs        play the example
go test ./...                  every test (headless, no window needed)
go run ./cmd/tracker           the tracker editor
go run ./cmd/spriteedit hero.spr   the sprite editor
go run ./cmd/spritetool          compose, turn and check sprites from the command line
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
| `sprite` | Pixel sprites made of parts on a skeleton: text format, key poses, computed in-betweens, pixel-art rotation, baked frames, `Bake` into Ebitengine images |
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

## Sprites

A sprite is a `.spr` text file: a size, a palette (one character per colour), parts drawn as pixel grids in their rest position, each with a joint pixel and a parent, and animations. A frame is either a key pose (an angle and a move per part) or an in-between computed from the keys around it; any frame can be baked into plain pixels and retouched. Turned parts are drawn with a RotSprite-style rotation (Scale2x three times, then nearest sampling), quarter turns stay exact.

`go run ./cmd/spriteedit -size 64x64 path/hero.spr` opens or creates a sprite. The editor walks through four steps, 1 SKELETON, 2 POSES, 3 DRAW, 4 ANIMATE, each showing only its own tools; hovering a button shows a tip and ? HELP turns the next click into a short guide. In SKELETON, ERASE ALL PAINT clears the example drawing and the body can be moved, resized, mirrored or turned (its moves follow), inside a canvas from 8 to 256 pixels. A new sprite starts as a humanoid skeleton (bones with joint limits: knees bend backwards only, elbows forwards) with idle, walk, punch, kick and hurt already posed. Pose the skeleton first (grab a bone, drag; front and back limbs can swing as mirror twins), then draw each part while the others stay visible in transparency, bake and retouch where needed. To trace, drop a picture on the window (or pass `-ref picture.png`): it shows under the drawing, TRACE (F) drags it and the wheel resizes it; its place is remembered in `hero.ref`. The wheel zooms the view around the cursor for pixel-by-pixel work, the middle button or HAND (P) moves it, 0 fits it back. `go run ./cmd/spritesheet hero.spr hero.png` exports every frame. The file reloads by itself when edited elsewhere. Shoulder and hip hangers carry both arms and both legs and tilt like a balance; postures are kept as named snapshots and clicked in order to build a move. In the game, `library, _ := sprite.LoadLibrary(files, "sprites")` loads every .spr of a folder and `clip, _ := library.Clip("hero.punch")` (sprite file name, a dot, the move name) gives `clip.FrameAt(tick)`, or `clip.Draw(screen, tick, x, y, scale, isFacingLeft)` to draw it on its anchor; `animation.PeakTick()` (start of the longest key, where a strike lands) and `animation.KeyStart(key)` time game events on the moves. OPEN / NEW switches to another sprite of the folder or creates one; IMPORT (step 4) copies the moves of another sprite and retargets them, so every bone points the same way even on a skeleton with another rest pose. COPY INTO ALL PARTS paints the parts from the reference picture through the current pose (each pixel goes to the closest bone, only empty pixels, new colours join the palette); INTO SELECTED fills hidden sides of a limb from a second picture.

Sprites turn around their spine in 3D. Each joint has a turn besides its angle; each part has a depth (how far behind the body it hangs) and can hold views, its drawings for other sides (`view 135` in the file, then its pixels). The renderer places every joint in 3D, foreshortens limbs pointing at the camera, mirrors limbs seen from behind, draws the nearest parts last (seen from behind, forearms pass behind upper arms and limbs behind the body, from the front the order reverses, in profile the width of the shoulders and hips decides) and shows each part the view closest to the side it faces; a hanger chooses the view of the part it hangs from, so turning the shoulders bar turns the torso drawing. In the editor, POSES and ANIMATE have TURN AROUND THE SPINE, and DRAW has SIDES: turn the whole drawing, NEW VIEW HERE, paint that side. `-step 3 -turn 180 -part torso -move cross` opens the editor on a step, a turn, a part and a move, which also works with `-snapshot`.

Moves can be written as text and composed (`spritetool compose hero.spr hero.moves`, or `sheet.Compose(script)` from `sprite.ParseMoves`): `move cross once`, then `key 8 plant leg_front_shin lead 5` and one line per part, `arm_front_upper -3 face 0`, giving the screen direction of the bone and optionally its absolute turn; `tween 2` blends, `shape guard` names a set of lines that `use guard` reuses. `plant` keeps a foot where it stands, `brace` keeps another foot down by bending its knee forward (`plant` and `brace` take an optional offset of the foot from its rest spot, so a walk places each foot and lets the knees be solved; a bar like `pelvis 15` tilts that hanger), `lead` leans the whole body like a pendulum until its weight sits that many pixels ahead of the planted foot. `spritetool moves -wrap 8 hero.spr out.png cross` draws every frame of moves, `turnaround` draws the body at every turn, `parts` every part with its views, `bones` the bone ends, facings and depths of a frame, `view` derives a mirrored or narrowed view, `depth` sets depths. `Part.FillHoles` fills the hidden areas of a part (under a crossing arm) so nothing tears open when it moves.

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
