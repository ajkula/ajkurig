package rig

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/ajkula/ajkurig/audio"
	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/input"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/postfx"
	"github.com/ajkula/ajkurig/scores"
	"github.com/ajkula/ajkurig/store"
	"github.com/ajkula/ajkurig/ui"
)

// ===== Types =====

type Config struct {
	Title           string
	SaveFolder      string
	Width           float32
	Height          float32
	TicksPerSecond  int
	Languages       []fs.FS
	Theme           *ui.Theme
	Fonts           ui.FontFiles
	Remaps          []input.RemapDefinition
	EssentialRemaps int
	Mouse           []input.MouseBinding
	Sounds          *audio.SoundBank
	Signals         []string
	Songs           []*audio.Song
	ScoreTableSize  int
	StartScreen     string
	IsSilent        bool
}

type Screen interface {
	Enter(app *App)
	Update(frame *Frame)
	Draw(frame *Frame, screen *ebiten.Image)
}

type App struct {
	Config          Config
	Store           store.Store
	Settings        Settings
	Input           *input.Reader
	Audio           *audio.Engine
	Kit             *ui.Kit
	Post            *postfx.Processor
	Batches         *draw.Batches
	Camera          *draw.Camera
	World           *Rack
	Global          *Rack
	Scores          scores.Table
	Jukebox         *Jukebox
	Bench           *Bench
	Message         string
	BindingsMessage string
	MenuLockSeconds float32
	FadeInSeconds   float32
	FadeInDuration  float32
	IsInRun         bool
	IsCursorHidden  bool
	IsDebugVisible  bool
	OnQuit          func()
	frame           Frame
	screens         map[string]Screen
	current         Screen
	currentName     string
	isQuitRequested bool
	hintReplacer    *strings.Replacer
	hintDevice      input.Device
}

// ===== Constants =====

const (
	DefaultWidth          = 1280
	DefaultHeight         = 720
	DefaultTicksPerSecond = 60
	MenuLockSeconds       = 0.45
	menuEffectsVolume     = 0.45
	gameEffectsVolume     = 0.8
	maximumMusicVolume    = 0.5
)

// ===== Public API =====

func NewApp(config Config) *App {
	config = withDefaults(config)
	app := &App{
		Config:  config,
		Store:   store.ForApplication(config.SaveFolder),
		World:   NewRack("world"),
		Global:  NewRack("global"),
		screens: map[string]Screen{},
		Batches: draw.NewBatches(),
		Camera:  draw.NewCamera(config.Width, config.Height),
		Post:    postfx.NewProcessor(),
	}
	app.frame = Frame{App: app, Delta: 1 / float32(config.TicksPerSecond)}
	_ = i18n.Load(config.Languages, languageDirectories()...)
	app.Kit = ui.NewKit(config.Theme, config.Fonts, config.Width, config.Height)
	app.Input = input.NewReader(newLayout(config))
	app.loadBindings()
	app.Audio = newEngine(config)
	app.Audio.SetSongs(config.Songs)
	app.Jukebox = NewJukebox(app)
	app.Bench = NewBench(app)
	app.Scores, _ = scores.Load(app.Store, config.ScoreTableSize)
	app.loadSettings()
	return app
}

func (a *App) Run() error {
	ebiten.SetWindowSize(int(a.Config.Width), int(a.Config.Height))
	ebiten.SetWindowTitle(a.Config.Title)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowClosingHandled(true)
	a.Start()
	return ebiten.RunGame(a)
}

func (a *App) Start() {
	ebiten.SetTPS(a.Config.TicksPerSecond)
	a.Switch(a.Config.StartScreen)
}

func (a *App) SetScreen(name string, screen Screen) {
	a.screens[name] = screen
}

func (a *App) Screen(name string) Screen {
	return a.screens[name]
}

func (a *App) Switch(name string) {
	screen, exists := a.screens[name]
	if !exists {
		return
	}
	a.current, a.currentName = screen, name
	a.MenuLockSeconds = MenuLockSeconds
	screen.Enter(a)
}

func (a *App) CurrentScreen() string {
	return a.currentName
}

func (a *App) Frame() *Frame {
	return &a.frame
}

func (a *App) Quit() {
	a.isQuitRequested = true
}

func (a *App) IsConfirming() bool {
	return a.MenuLockSeconds == 0 && a.frame.Controls.JustPressed&input.ActionConfirm != 0
}

func (a *App) IsGoingBack() bool {
	return a.MenuLockSeconds == 0 && a.frame.Controls.JustPressed&input.ActionBack != 0
}

func (a *App) LockMenu() {
	a.MenuLockSeconds = MenuLockSeconds
}

func (a *App) PlaySound(id audio.SoundID) {
	a.Audio.Play(id, a.frame.Clock)
}

func (a *App) PlaySoundIf(id audio.SoundID, shouldPlay bool) {
	if !shouldPlay {
		return
	}
	a.PlaySound(id)
}

func (a *App) PlayMenuEvent(event ui.MenuEvent) {
	sounds := map[ui.MenuEvent]audio.SoundID{
		ui.EventMoved: audio.SoundMenuMove, ui.EventAdjusted: audio.SoundMenuMove, ui.EventDrumMoved: audio.SoundMenuMove,
		ui.EventActivated: audio.SoundMenuSelect, ui.EventDrumChosen: audio.SoundMenuSelect,
	}
	sound, hasSound := sounds[event]
	a.PlaySoundIf(sound, hasSound)
}

func (a *App) Hint(key string) string {
	return a.hintReplacer.Replace(i18n.T(key))
}

func (a *App) SaveSettings() {
	a.Message = ""
	if err := SaveSettings(a.Store, a.Settings); err != nil {
		a.Message = i18n.F("settings.save_failed", err.Error())
	}
}

func (a *App) ChangeSettings(change func(settings *Settings)) {
	change(&a.Settings)
	a.ApplySettings()
	a.SaveSettings()
}

func (a *App) ApplySettings() {
	ebiten.SetFullscreen(a.Settings.IsFullscreen)
	ebiten.SetVsyncEnabled(a.Settings.IsVsyncOn)
	a.Audio.SetMusicOn(a.Settings.IsMusicOn)
	a.Audio.SetMusicVolume(maximumMusicVolume * float32(a.Settings.MusicVolumeStep) / VolumeSteps)
	a.ApplyLanguage()
	a.Jukebox.RefreshPlaylist()
}

func (a *App) ApplyLanguage() {
	code := mathx.Pick(a.Settings.IsLanguageChosen(), i18n.FallbackCode, a.Settings.Language)
	a.PreviewLanguage(code)
}

func (a *App) PreviewLanguage(code string) {
	i18n.Use(code)
	a.Kit.Text.ResetFaces()
	a.refreshHints()
}

func (a *App) SaveScores() {
	if err := scores.Save(a.Store, a.Scores); err != nil {
		a.Message = i18n.F("scores.save_failed", err.Error())
	}
}

func (a *App) ApplyBindings(bindings input.Bindings) error {
	a.Input.UseBindings(bindings)
	a.refreshHints()
	a.BindingsMessage = "remap.custom_loaded"
	return input.SaveBindings(a.Store, a.Input.Layout(), bindings)
}

func (a *App) SimulateWorld(frame *Frame) {
	a.World.Update(frame)
}

func (a *App) DrawWorld(frame *Frame, screen *ebiten.Image) {
	a.World.Draw(frame, screen)
	a.Batches.Flush(screen)
	a.World.DrawLate(frame, screen)
	a.Post.ApplyBloom(a.Settings.BloomFactor())
}

func (a *App) DrawHUD(frame *Frame, screen *ebiten.Image) {
	a.World.DrawHUD(frame, screen)
}

func (a *App) ResetWorld(isDemo bool) {
	a.frame.IsDemo = isDemo
	a.World.Reset(&a.frame)
}

func (a *App) Update() error {
	if ebiten.IsWindowBeingClosed() || a.isQuitRequested {
		callIf(a.OnQuit)
		return ebiten.Termination
	}
	a.Step(a.Input.Read())
	ebiten.SetCursorMode(mathx.Pick(a.IsCursorHidden, ebiten.CursorModeVisible, ebiten.CursorModeHidden))
	return nil
}

func (a *App) Step(controls input.Controls) {
	frame := &a.frame
	frame.Controls = controls
	a.refreshHintsIfDeviceChanged()
	frame.Tick++
	frame.Clock += frame.Delta
	a.IsDebugVisible = a.IsDebugVisible != (frame.Controls.JustPressed&input.ActionDebug != 0)
	a.FadeInSeconds = max(0, a.FadeInSeconds-frame.Delta)
	a.MenuLockSeconds = max(0, a.MenuLockSeconds-frame.Delta)
	a.updateScreen(frame)
	a.Global.Update(frame)
	a.toggleFullscreenIfRequested(frame)
	a.updateAudio(frame)
}

func (a *App) Draw(screen *ebiten.Image) {
	settings := postfx.Settings{BloomIntensity: a.Settings.BloomFactor(), IsCRTOn: a.Settings.IsCRTOn}
	scene := a.Post.Begin(screen, settings)
	a.drawScreen(scene)
	a.Global.DrawHUD(&a.frame, scene)
	a.Kit.Dim(scene, a.FadeInSeconds/max(a.FadeInDuration, 0.001))
	a.Post.Finish(screen, settings, draw.Scale)
	if !a.IsDebugVisible {
		return
	}
	a.Kit.DebugOverlay(screen, a.Bench.DebugLines())
}

func (a *App) Layout(outsideWidth, outsideHeight int) (int, int) {
	draw.Scale = a.Settings.RenderScale(outsideWidth, outsideHeight, a.Config.Width, a.Config.Height)
	return int(a.Config.Width * draw.Scale), int(a.Config.Height * draw.Scale)
}

func (a *App) ToggleMusic() {
	a.ChangeSettings(func(settings *Settings) { settings.IsMusicOn = !settings.IsMusicOn })
}

// ===== Internal =====

func withDefaults(config Config) Config {
	config.Width = mathx.Pick(config.Width <= 0, config.Width, DefaultWidth)
	config.Height = mathx.Pick(config.Height <= 0, config.Height, DefaultHeight)
	config.TicksPerSecond = mathx.Pick(config.TicksPerSecond <= 0, config.TicksPerSecond, DefaultTicksPerSecond)
	config.SaveFolder = mathx.Pick(config.SaveFolder == "", config.SaveFolder, strings.ToLower(strings.ReplaceAll(config.Title, " ", "")))
	config.Theme = mathx.Pick(config.Theme == nil, config.Theme, &ui.DefaultTheme)
	config.Fonts = mathx.Pick(config.Fonts.Bold == nil, config.Fonts, ui.DefaultFontFiles())
	config.Sounds = mathx.Pick(config.Sounds == nil, config.Sounds, audio.NewSoundBank())
	config.StartScreen = mathx.Pick(config.StartScreen == "", config.StartScreen, "intro")
	return config
}

func newLayout(config Config) *input.Layout {
	layout := input.NewLayout(config.Remaps, config.EssentialRemaps)
	layout.Mouse = config.Mouse
	return layout
}

func newEngine(config Config) *audio.Engine {
	board := audio.NewSignalBoard(config.Signals...)
	if config.IsSilent {
		return audio.NewSilentEngine(config.Sounds, board)
	}
	return audio.NewEngine(config.Sounds, board)
}

func languageDirectories() []string {
	directories := make([]string, 0, 2)
	if executable, err := os.Executable(); err == nil {
		directories = append(directories, filepath.Dir(executable))
	}
	if workingDirectory, err := os.Getwd(); err == nil {
		directories = append(directories, workingDirectory)
	}
	return directories
}

func (a *App) loadBindings() {
	bindings, err := input.LoadBindings(a.Store, a.Input.Layout())
	a.Input.UseBindings(bindings)
	a.BindingsMessage = mathx.Pick(err != nil, "remap.custom_loaded", "remap.default_loaded")
}

func (a *App) loadSettings() {
	a.Settings, _ = LoadSettings(a.Store)
	a.ApplySettings()
}

func (a *App) refreshHints() {
	a.hintReplacer = strings.NewReplacer(
		"{confirm}", a.Input.ButtonLabel(input.ActionConfirm),
		"{back}", a.Input.ButtonLabels(input.ActionBack),
		"{pause}", a.Input.ButtonLabel(input.ActionPause),
		"{select}", a.Input.ButtonLabel(input.ActionSelect),
	)
}

func (a *App) updateScreen(frame *Frame) {
	if a.current == nil {
		return
	}
	a.current.Update(frame)
}

func (a *App) refreshHintsIfDeviceChanged() {
	if a.Input.LastDevice() == a.hintDevice {
		return
	}
	a.hintDevice = a.Input.LastDevice()
	a.refreshHints()
}

func (a *App) drawScreen(screen *ebiten.Image) {
	if a.current == nil {
		return
	}
	a.current.Draw(&a.frame, screen)
}

func (a *App) toggleFullscreenIfRequested(frame *Frame) {
	if frame.Controls.JustPressed&input.ActionFullscreen == 0 {
		return
	}
	a.ChangeSettings(func(settings *Settings) { settings.IsFullscreen = !settings.IsFullscreen })
}

func (a *App) updateAudio(frame *Frame) {
	if frame.Controls.JustPressed&input.ActionMute != 0 {
		a.ToggleMusic()
	}
	context := mathx.Pick(a.IsInRun, float32(menuEffectsVolume), gameEffectsVolume)
	a.Audio.SetEffectsVolume(context * float32(a.Settings.EffectsVolumeStep) / VolumeSteps)
	a.Audio.PushSignals()
}

func callIf(action func()) {
	if action == nil {
		return
	}
	action()
}
