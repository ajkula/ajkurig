package probe

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/ajkula/ajkurig/input"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rig"
)

// ===== Types =====

type Phase struct {
	Name  string
	Enter func()
	Draw  func(screen *ebiten.Image)
}

type Result struct {
	Name           string
	MillisPerFrame float64
}

type Difference struct {
	Worst     int
	Differing int
	Covered   int
}

type Options struct {
	Layout         func(outsideWidth, outsideHeight int) (int, int)
	WarmupUpdate   func()
	WarmupDraw     func(screen *ebiten.Image)
	OnFinish       func(screen *ebiten.Image)
	WarmupFrames   int
	FramesPerPhase int
	SettleFrames   int
	IsWindowed     bool
}

type runner struct {
	options      Options
	phases       []Phase
	results      []Result
	frame        int
	lastDraw     time.Time
	elapsedMilli float64
	measured     int
}

// ===== Constants =====

const (
	defaultFramesPerPhase = 240
	defaultSettleFrames   = 40
	windowWidth           = 1600
	windowHeight          = 900
)

var (
	errNoPhases      = errors.New("probe: no phase to measure")
	errNoLayout      = errors.New("probe: Options.Layout is required")
	errSettleTooLong = errors.New("probe: SettleFrames must be shorter than FramesPerPhase")
	errUnnamedPhase  = errors.New("probe: every phase needs a name and a Draw")
)

// ===== Public API =====

func Run(options Options, phases []Phase) ([]Result, error) {
	options = withDefaults(options)
	if err := validate(options, phases); err != nil {
		return nil, err
	}
	ebiten.SetVsyncEnabled(false)
	ebiten.SetTPS(ebiten.SyncWithFPS)
	ebiten.SetFullscreen(!options.IsWindowed)
	ebiten.SetWindowSize(windowWidth, windowHeight)
	probe := &runner{options: options, phases: phases, results: make([]Result, 0, len(phases))}
	if err := ebiten.RunGame(probe); err != nil {
		return probe.results, err
	}
	return probe.results, nil
}

func ForApp(app *rig.App, warmupFrames int) Options {
	return Options{
		Layout:       app.Layout,
		WarmupUpdate: func() { app.Step(input.Controls{}) },
		WarmupDraw:   app.Draw,
		WarmupFrames: warmupFrames,
	}
}

func Compare(first, second *ebiten.Image, tolerance int) Difference {
	return CompareBytes(pixelsOf(first), pixelsOf(second), tolerance)
}

func CompareBytes(first, second []byte, tolerance int) Difference {
	var difference Difference
	for index := range min(len(first), len(second)) {
		delta := mathx.AbsInt(int(first[index]) - int(second[index]))
		difference.Worst = max(difference.Worst, delta)
		difference.Differing += mathx.BoolToIndex(delta > tolerance)
		difference.Covered += mathx.BoolToIndex(first[index] != 0 || second[index] != 0)
	}
	return difference
}

func Format(results []Result) string {
	width := 0
	for _, result := range results {
		width = max(width, len(result.Name))
	}
	var report strings.Builder
	for _, result := range results {
		fmt.Fprintf(&report, "%-*s %7.2f ms/frame %7.1f fps\n", width, result.Name, result.MillisPerFrame, result.FramesPerSecond())
	}
	return report.String()
}

func (r Result) FramesPerSecond() float64 {
	return 1000 / max(r.MillisPerFrame, 1e-9)
}

func (d Difference) String() string {
	return fmt.Sprintf("worst channel delta %d, %d of %d covered channels beyond tolerance", d.Worst, d.Differing, d.Covered)
}

// ===== Internal =====

func (r *runner) Update() error {
	if r.isFinished() {
		return ebiten.Termination
	}
	if r.frame < r.options.WarmupFrames {
		r.options.WarmupUpdate()
	}
	return nil
}

func (r *runner) Draw(screen *ebiten.Image) {
	if r.isFinished() {
		return
	}
	r.frame++
	index, offset, isWarmup := phaseAt(r.frame, r.options.WarmupFrames, r.options.FramesPerPhase)
	if isWarmup {
		r.options.WarmupDraw(screen)
		r.lastDraw = time.Now()
		return
	}
	phase := &r.phases[index]
	r.enterIfFirst(phase, offset)
	phase.Draw(screen)
	r.recordFrame(offset)
	r.finishPhaseIfLast(phase, offset, screen)
}

func (r *runner) Layout(outsideWidth, outsideHeight int) (int, int) {
	return r.options.Layout(outsideWidth, outsideHeight)
}

func (r *runner) isFinished() bool {
	return len(r.results) == len(r.phases)
}

func (r *runner) enterIfFirst(phase *Phase, offset int) {
	if offset != 0 {
		return
	}
	r.elapsedMilli, r.measured = 0, 0
	phase.Enter()
}

func (r *runner) recordFrame(offset int) {
	now := time.Now()
	isSettled := offset >= r.options.SettleFrames
	r.elapsedMilli += float64(now.Sub(r.lastDraw).Microseconds()) / 1000 * float64(mathx.BoolToIndex(isSettled))
	r.measured += mathx.BoolToIndex(isSettled)
	r.lastDraw = now
}

func (r *runner) finishPhaseIfLast(phase *Phase, offset int, screen *ebiten.Image) {
	if offset != r.options.FramesPerPhase-1 {
		return
	}
	r.results = append(r.results, Result{Name: phase.Name, MillisPerFrame: r.elapsedMilli / float64(r.measured)})
	if r.isFinished() {
		r.options.OnFinish(screen)
	}
}

func phaseAt(frame, warmupFrames, framesPerPhase int) (index, offset int, isWarmup bool) {
	if frame <= warmupFrames {
		return 0, 0, true
	}
	measured := frame - warmupFrames - 1
	return measured / framesPerPhase, measured % framesPerPhase, false
}

func withDefaults(options Options) Options {
	options.FramesPerPhase = mathx.Pick(options.FramesPerPhase <= 0, options.FramesPerPhase, defaultFramesPerPhase)
	options.SettleFrames = mathx.Pick(options.SettleFrames <= 0, options.SettleFrames, defaultSettleFrames)
	options.WarmupUpdate = orNothing(options.WarmupUpdate)
	options.WarmupDraw = orNothingOnScreen(options.WarmupDraw)
	options.OnFinish = orNothingOnScreen(options.OnFinish)
	return options
}

func validate(options Options, phases []Phase) error {
	if len(phases) == 0 {
		return errNoPhases
	}
	if options.Layout == nil {
		return errNoLayout
	}
	if options.SettleFrames >= options.FramesPerPhase {
		return errSettleTooLong
	}
	for index := range phases {
		phases[index].Enter = orNothing(phases[index].Enter)
		if phases[index].Name == "" || phases[index].Draw == nil {
			return errUnnamedPhase
		}
	}
	return nil
}

func orNothing(action func()) func() {
	if action == nil {
		return func() {}
	}
	return action
}

func orNothingOnScreen(action func(screen *ebiten.Image)) func(screen *ebiten.Image) {
	if action == nil {
		return func(*ebiten.Image) {}
	}
	return action
}

func pixelsOf(image *ebiten.Image) []byte {
	bounds := image.Bounds()
	pixels := make([]byte, 4*bounds.Dx()*bounds.Dy())
	image.ReadPixels(pixels)
	return pixels
}
