package shell

import (
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/colorx"
	"ajkurig/i18n"
	"ajkurig/input"
	"ajkurig/mathx"
	"ajkurig/rig"
	"ajkurig/ui"
)

// ===== Types =====

type MenuScreen struct {
	Shell          *Shell
	Title          func() string
	TitleColor     func() colorx.RGB
	Menu           ui.Menu
	IsMain         bool
	Hints          []string
	OnEnter        func()
	OnBack         func()
	BeforeUpdate   func(frame *rig.Frame) bool
	DrawInstead    func(frame *rig.Frame, screen *ebiten.Image) bool
	DrawExtra      func(frame *rig.Frame, screen *ebiten.Image)
	DrawScene      func(frame *rig.Frame, screen *ebiten.Image)
	IsBackdropLive bool
	ShowsBench     bool
	ShowsDemoClip  bool
	ShowsMessage   bool
	Dim            float32
}

// ===== Constants =====

const volumeMaximumPercent = 100

// ===== Public API =====

func (m *MenuScreen) Enter(app *rig.App) {
	m.Menu.Selection = 0
	callIf(m.OnEnter)
}

func (m *MenuScreen) Update(frame *rig.Frame) {
	app := m.Shell.App
	if m.IsBackdropLive {
		m.Shell.UpdateBackdrop(frame)
	}
	if m.BeforeUpdate != nil && m.BeforeUpdate(frame) {
		return
	}
	isDrumFocused := m.Menu.Focused() != nil
	if m.OnBack != nil && !isDrumFocused && app.IsGoingBack() {
		m.OnBack()
		return
	}
	event := m.Menu.Navigate(frame.Controls, app.IsConfirming(), app.IsGoingBack(), frame.Delta)
	app.PlayMenuEvent(event)
	mathx.SetIf(&app.MenuLockSeconds, rig.MenuLockSeconds, event == ui.EventDrumChosen || event == ui.EventDrumCancelled)
}

func (m *MenuScreen) Draw(frame *rig.Frame, screen *ebiten.Image) {
	if m.DrawInstead != nil && m.DrawInstead(frame, screen) {
		return
	}
	shell, kit := m.Shell, m.Shell.App.Kit
	drawScene := mathx.Pick(m.DrawScene == nil, m.DrawScene, shell.DrawBackdrop)
	drawScene(frame, screen)
	kit.Dim(screen, mathx.Pick(m.Dim <= 0, m.Dim, kit.Theme.MenuDim))
	titleTint := mathx.Pick(m.TitleColor == nil, m.TitleColor, func() colorx.RGB { return kit.Theme.Accent })
	kit.Title(screen, m.Title(), titleTint())
	if m.ShowsMessage {
		kit.Message(screen, shell.App.Message)
	}
	callDrawIf(m.DrawExtra, frame, screen)
	kit.DrawMenu(screen, &m.Menu, mathx.Pick(m.IsMain, kit.SubmenuLayout(), kit.MainLayout()), frame.Clock)
	if m.ShowsBench {
		shell.drawBenchIf(screen)
	}
	if m.ShowsDemoClip {
		shell.drawDemoClip(screen)
	}
	shell.DrawMusicBanner(screen)
	hints := make([]string, 0, len(m.Hints))
	for _, key := range m.Hints {
		hints = append(hints, shell.App.Hint(key))
	}
	kit.Hints(screen, hints...)
}

func Label(key string) func() string {
	return func() string { return i18n.T(key) }
}

func OnOffLabel(key string, isOn func() bool) func() string {
	return func() string { return i18n.F(key, i18n.Value(rig.OnOffLabels[mathx.BoolToIndex(isOn())])) }
}

// ===== Internal =====

func newMainMenu(shell *Shell) *MenuScreen {
	app := shell.App
	screen := &MenuScreen{
		Shell: shell, IsMain: true, IsBackdropLive: true, ShowsBench: true, ShowsDemoClip: true,
		Title: func() string { return shell.Options.Title },
		Menu: ui.Menu{Options: []ui.Option{
			{Label: Label("menu.play"), Activate: shell.StartRun},
			{Label: Label("menu.options"), Activate: func() { openOptions(shell, ScreenMenu) }},
			{Label: Label("menu.quit"), Activate: func() { app.Quit() }},
		}},
	}
	screen.BeforeUpdate = func(frame *rig.Frame) bool {
		if shell.dismissBoardIfPressed(frame) {
			return true
		}
		if frame.Controls.JustPressed&input.ActionSelect == 0 {
			return false
		}
		app.ChangeSettings(func(settings *rig.Settings) { settings.IsBenchmarkHidden = !settings.IsBenchmarkHidden })
		return true
	}
	screen.DrawInstead = func(frame *rig.Frame, image *ebiten.Image) bool {
		if shell.BoardSeconds <= 0 {
			return false
		}
		shell.drawRankingAttract(frame, image, attractBoardSeconds-shell.BoardSeconds)
		return true
	}
	screen.DrawExtra = func(frame *rig.Frame, image *ebiten.Image) {
		kit := app.Kit
		kit.Text.Draw(image, shell.HighScoreLine(), ui.FaceBold, kit.Width/2, kit.Theme.TitleTop+kit.Theme.MessageGap, kit.Theme.Record, 1, text.AlignCenter)
		kit.Hints(image, i18n.T(shell.Options.ControlsHint), i18n.T(app.BindingsMessage))
	}
	return screen
}

func openOptions(shell *Shell, returnTo string) {
	options := shell.App.Screen(ScreenOptions).(*MenuScreen)
	options.OnBack = func() { shell.App.Switch(returnTo) }
	options.Menu.Options[len(options.Menu.Options)-1].Activate = options.OnBack
	shell.App.Message = ""
	shell.App.Switch(ScreenOptions)
}

func submenu(shell *Shell, titleKey, backTo string, hints []string, options []ui.Option) *MenuScreen {
	screen := &MenuScreen{
		Shell: shell, Title: Label(titleKey), IsBackdropLive: true, ShowsMessage: true, Hints: hints,
		OnBack: func() { shell.App.Switch(backTo) },
	}
	screen.Menu.Options = append(options, ui.Option{Label: Label("menu.back"), Activate: screen.OnBack})
	return screen
}

func newOptionsMenu(shell *Shell) *MenuScreen {
	app := shell.App
	screen := submenu(shell, "title.options", ScreenMenu, []string{"hint.menu_back", "hint.settings_saved"}, []ui.Option{
		{Label: Label("menu.controls"), Activate: func() { app.Switch(ScreenControls) }},
		{Label: Label("menu.gameplay"), Activate: func() { app.Switch(ScreenGameplay) }},
		{Label: Label("menu.graphics"), Activate: func() { app.Switch(ScreenGraphics) }},
		{Label: Label("menu.audio"), Activate: func() { app.Switch(ScreenAudio) }},
	})
	screen.ShowsBench, screen.ShowsDemoClip = true, true
	return screen
}

func newGraphicsMenu(shell *Shell) *MenuScreen {
	app := shell.App
	settings := &app.Settings
	change := func(update func(settings *rig.Settings)) func() { return func() { app.ChangeSettings(update) } }
	levelLabel := func(key string, levels *[rig.LevelCount]rig.Level, level *int) func() string {
		return func() string { return i18n.F(key, i18n.Value(levels[*level].Label)) }
	}
	screen := submenu(shell, "title.graphics", ScreenOptions, []string{"hint.graphics", "hint.graphics_native"}, []ui.Option{
		{Label: OnOffLabel("graphics.fullscreen", func() bool { return settings.IsFullscreen }), Activate: change(func(s *rig.Settings) { s.IsFullscreen = !s.IsFullscreen })},
		{Label: OnOffLabel("graphics.vsync", func() bool { return settings.IsVsyncOn }), Activate: change(func(s *rig.Settings) { s.IsVsyncOn = !s.IsVsyncOn })},
		{Label: func() string {
			return i18n.F("graphics.resolution", i18n.Value(rig.ResolutionLabels[settings.Resolution]))
		}, Activate: change(func(s *rig.Settings) { s.Resolution = (s.Resolution + 1) % rig.ResolutionCount })},
		{Label: OnOffLabel("graphics.show_fps", func() bool { return settings.IsFPSShown }), Activate: change(func(s *rig.Settings) { s.IsFPSShown = !s.IsFPSShown })},
		{Label: levelLabel("graphics.shake", &rig.ShakeLevels, &settings.ShakeLevel), Activate: change(func(s *rig.Settings) { s.ShakeLevel = (s.ShakeLevel + 1) % rig.LevelCount })},
		{Label: levelLabel("graphics.effects", &rig.EffectsLevels, &settings.EffectsLevel), Activate: change(func(s *rig.Settings) { s.EffectsLevel = (s.EffectsLevel + 1) % rig.LevelCount })},
		{Label: levelLabel("graphics.bloom", &rig.BloomLevels, &settings.BloomLevel), Activate: change(func(s *rig.Settings) { s.BloomLevel = (s.BloomLevel + 1) % rig.LevelCount })},
		{Label: OnOffLabel("graphics.crt", func() bool { return settings.IsCRTOn }), Activate: change(func(s *rig.Settings) { s.IsCRTOn = !s.IsCRTOn })},
	})
	screen.ShowsBench = true
	return screen
}

func newAudioMenu(shell *Shell) *MenuScreen {
	app := shell.App
	settings := &app.Settings
	setVolume := func(step *int, value int) {
		app.ChangeSettings(func(*rig.Settings) { *step = mathx.ClampInt(value, 0, rig.VolumeSteps) })
	}
	volumeLabel := func(key string, step *int) func() string {
		return func() string {
			return i18n.T(key) + ": " + strconv.Itoa(*step*volumeMaximumPercent/rig.VolumeSteps) + "%"
		}
	}
	musicSlider := &ui.Slider{Steps: rig.VolumeSteps, Level: func() int { return settings.MusicVolumeStep }, Adjust: func(delta int) { setVolume(&settings.MusicVolumeStep, settings.MusicVolumeStep+delta) }}
	effectsSlider := &ui.Slider{Steps: rig.VolumeSteps, Level: func() int { return settings.EffectsVolumeStep }, Adjust: func(delta int) { setVolume(&settings.EffectsVolumeStep, settings.EffectsVolumeStep+delta) }}
	screen := submenu(shell, "title.audio", ScreenOptions, []string{"hint.audio", "hint.audio_music_off"}, []ui.Option{
		{Label: func() string { return i18n.T(mathx.Pick(settings.IsMusicOn, "menu.music_off", "menu.music_on")) }, Activate: app.ToggleMusic},
		{Label: volumeLabel("audio.music_volume", &settings.MusicVolumeStep), Slider: musicSlider, Activate: func() { setVolume(&settings.MusicVolumeStep, (settings.MusicVolumeStep+1)%(rig.VolumeSteps+1)) }},
		{Label: volumeLabel("audio.effects_volume", &settings.EffectsVolumeStep), Slider: effectsSlider, Activate: func() { setVolume(&settings.EffectsVolumeStep, (settings.EffectsVolumeStep+1)%(rig.VolumeSteps+1)) }},
		{Label: Label("audio.playlist"), Activate: func() { app.Switch(ScreenPlaylist) }},
	})
	screen.ShowsBench = true
	return screen
}

func newPlaylistMenu(shell *Shell) *MenuScreen {
	app := shell.App
	screen := submenu(shell, "title.playlist", ScreenAudio, []string{"hint.playlist", "hint.playlist_enabled"}, nil)
	screen.OnEnter = func() {
		options := make([]ui.Option, 0, app.Audio.SongCount()+1)
		for index := range app.Audio.SongCount() {
			options = append(options, ui.Option{
				Label: func() string {
					return app.Audio.SongTitle(index) + ": " + rig.OnOffLabels[mathx.BoolToIndex(app.Jukebox.IsSongEnabled(index))]
				},
				Activate: func() { app.Jukebox.ToggleSong(index) },
			})
		}
		screen.Menu.Options = append(options, ui.Option{Label: Label("menu.back"), Activate: screen.OnBack})
	}
	screen.ShowsBench = true
	return screen
}

func languageDrum(shell *Shell) *ui.Drum {
	app := shell.App
	languages := func() []i18n.Language { return i18n.Languages() }
	currentIndex := func() int {
		current := i18n.Current().Code
		for index, language := range languages() {
			if language.Code == current {
				return index
			}
		}
		return 0
	}
	return &ui.Drum{
		Items: func() []string {
			names := []string{}
			for _, language := range languages() {
				names = append(names, language.Name)
			}
			return names
		},
		Current:   currentIndex,
		Preview:   func(index int) { app.PreviewLanguage(languages()[index].Code) },
		Choose:    func(index int) { chooseLanguage(shell, languages()[index].Code) },
		Cancel:    app.ApplyLanguage,
		CanCancel: app.Settings.IsLanguageChosen,
	}
}

func chooseLanguage(shell *Shell, code string) {
	app := shell.App
	app.Settings.Language = code
	app.SaveSettings()
	app.ApplyLanguage()
	if app.CurrentScreen() == ScreenLanguage {
		app.Switch(ScreenMenu)
	}
}

func newGameplayMenu(shell *Shell) *MenuScreen {
	app := shell.App
	return submenu(shell, "title.gameplay", ScreenOptions, []string{"hint.menu_back"}, []ui.Option{
		{Label: Label("gameplay.language"), Drum: languageDrum(shell)},
		{Label: Label("gameplay.high_scores"), Activate: func() { app.Switch(ScreenHighScores) }},
		{Label: Label("gameplay.reset_scores"), Activate: func() { app.Switch(ScreenResetScores) }},
	})
}

func newResetScoresMenu(shell *Shell) *MenuScreen {
	app := shell.App
	screen := &MenuScreen{
		Shell: shell, Title: Label("title.erase_scores"), IsBackdropLive: true, ShowsMessage: true, Hints: []string{"hint.erase_scores"},
		OnBack: func() { app.Switch(ScreenGameplay) },
	}
	screen.Menu.Options = []ui.Option{
		{Label: Label("gameplay.keep_scores"), Activate: screen.OnBack},
		{Label: Label("gameplay.erase_scores"), Activate: func() {
			app.Scores.Clear()
			app.SaveScores()
			app.Switch(ScreenGameplay)
			app.Message = i18n.T("gameplay.scores_erased")
		}},
	}
	return screen
}

func newLanguageScreen(shell *Shell) *MenuScreen {
	drum := languageDrum(shell)
	screen := &MenuScreen{
		Shell: shell, Title: Label("title.language"), IsBackdropLive: true, Hints: []string{"hint.language"},
		Menu: ui.Menu{Options: []ui.Option{{Label: Label("gameplay.language"), Drum: drum}}},
	}
	screen.OnEnter = func() {
		shell.App.PreviewLanguage(i18n.Detect())
		drum.Focus()
	}
	return screen
}

func newPauseMenu(shell *Shell) *MenuScreen {
	app := shell.App
	screen := &MenuScreen{
		Shell: shell, Title: Label("title.paused"), TitleColor: func() colorx.RGB { return app.Kit.Theme.Text }, Hints: []string{"hint.menu_resume"},
		OnBack: func() { app.Switch(ScreenPlay) },
		DrawScene: func(frame *rig.Frame, screen *ebiten.Image) {
			app.DrawWorld(frame, screen)
			app.DrawHUD(frame, screen)
		},
	}
	screen.Menu.Options = []ui.Option{
		{Label: Label("menu.resume"), Activate: screen.OnBack},
		{Label: Label("menu.options"), Activate: func() { openOptions(shell, ScreenPause) }},
		{Label: func() string { return i18n.T(mathx.Pick(app.Settings.IsMusicOn, "menu.music_off", "menu.music_on")) }, Activate: app.ToggleMusic},
		{Label: Label("menu.back_to_main"), Activate: func() {
			shell.RecordScore()
			shell.GoHome()
		}},
	}
	return screen
}

func callIf(action func()) {
	if action == nil {
		return
	}
	action()
}

func callDrawIf(action func(frame *rig.Frame, screen *ebiten.Image), frame *rig.Frame, screen *ebiten.Image) {
	if action == nil {
		return
	}
	action(frame, screen)
}
