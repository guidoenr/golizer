package app

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/guidoenr/chroma/go-implementation/internal/analyzer"
	"github.com/guidoenr/chroma/go-implementation/internal/audio"
	"github.com/guidoenr/chroma/go-implementation/internal/params"
	"github.com/guidoenr/chroma/go-implementation/internal/render"
	"golang.org/x/term"
)

// Config configures the application runtime.
type Config struct {
	DeviceName    string
	Width         int
	Height        int
	TargetFPS     float64
	BufferSize    int
	DisableAudio  bool
	ShowStatusBar bool
	Palette       string
	Pattern       string
	ColorMode     string
	ColorOnAudio  bool
	UseANSI       bool
	Log           *log.Logger
}

type inputEvent int

const (
	inputEventRandomize inputEvent = iota
	inputEventQuit
)

// App ties together audio capture, analysis, and rendering.
type App struct {
	cfg            Config
	params         params.Parameters
	renderer       *render.Renderer
	capture        *audio.Capture
	analyzer       *analyzer.Analyzer
	fake           *fakeGenerator
	last           time.Time
	log            *log.Logger
	deviceLabel    string
	width          int
	height         int
	renderHeight   int
	inputEvents    chan inputEvent
	rng            *rand.Rand
	colorOnAudio   bool
	paletteOptions []string
	patternOptions []string
	colorOptions   []string
}

// New constructs the application using the provided configuration.
func New(cfg Config) (*App, error) {
	if cfg.TargetFPS <= 0 {
		cfg.TargetFPS = 20
	}
	if cfg.Log == nil {
		cfg.Log = log.New(os.Stdout, "", log.LstdFlags)
	}

	if cfg.Width <= 0 {
		cfg.Width = 80
	}
	if cfg.Height <= 0 {
		cfg.Height = 24
	}
	renderHeight := cfg.Height
	if cfg.ShowStatusBar && renderHeight > 1 {
		renderHeight--
	}

	renderer, err := render.New(cfg.Width, renderHeight, cfg.Palette, cfg.Pattern, cfg.ColorMode, cfg.ColorOnAudio, cfg.UseANSI)
	if err != nil {
		return nil, err
	}

	app := &App{
		cfg:            cfg,
		params:         params.Defaults(),
		renderer:       renderer,
		log:            cfg.Log,
		width:          cfg.Width,
		height:         cfg.Height,
		renderHeight:   renderHeight,
		colorOnAudio:   cfg.ColorOnAudio,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
		paletteOptions: render.PaletteNames(),
		patternOptions: render.PatternNames(),
		colorOptions:   render.ColorModeNames(),
	}
	if len(app.paletteOptions) == 0 {
		app.paletteOptions = []string{"default"}
	}
	if len(app.patternOptions) == 0 {
		app.patternOptions = []string{"plasma"}
	}
	if len(app.colorOptions) == 0 {
		app.colorOptions = []string{"chromatic"}
	}

	if cfg.DisableAudio {
		app.fake = newFakeGenerator()
		app.log.Println("audio disabled, using synthetic generator")
	} else {
		capture, err := audio.NewCapture(audio.Config{
			DeviceName: cfg.DeviceName,
			BufferSize: cfg.BufferSize,
			Channels:   2,
		})
		if err != nil {
			return nil, fmt.Errorf("audio capture: %w", err)
		}
		app.capture = capture
		app.analyzer = analyzer.New(analyzer.Config{
			SampleRate:  capture.SampleRate(),
			HistorySize: 60,
		})
		if info := capture.Device(); info != nil {
			app.deviceLabel = info.Name
			app.log.Printf("audio capture started on \"%s\" @ %.0f Hz", info.Name, capture.SampleRate())
		} else {
			app.log.Printf("audio capture started @ %.0f Hz", capture.SampleRate())
		}
	}

	app.last = time.Now()
	if cfg.Pattern != "" {
		app.params.Pattern = strings.ToLower(cfg.Pattern)
	}
	if cfg.ColorMode != "" {
		app.params.ColorMode = strings.ToLower(cfg.ColorMode)
	}
	return app, nil
}

// Run starts the render loop until context cancellation.
func (a *App) Run(ctx context.Context) error {
	frameSeconds := 1.0 / a.cfg.TargetFPS
	frameDuration := time.Duration(frameSeconds * float64(time.Second))
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	enterAltScreen()
	clearScreen()
	hideCursor()
	defer func() {
		showCursor()
		exitAltScreen()
	}()

	inputCtx, cancelInput := context.WithCancel(ctx)
	defer cancelInput()
	a.startInputListener(inputCtx)
	a.ensureDimensions()

	for {
		select {
		case <-ctx.Done():
			moveCursorHome()
			return ctx.Err()
		case evt, ok := <-a.inputEvents:
			if !ok {
				a.inputEvents = nil
				continue
			}
			switch evt {
			case inputEventRandomize:
				a.randomizeVisuals()
			case inputEventQuit:
				moveCursorHome()
				return nil
			}
		case <-ticker.C:
			if err := a.step(); err != nil {
				return err
			}
		}
	}
}

// Close releases held resources.
func (a *App) Close() error {
	if a.capture != nil {
		return a.capture.Close()
	}
	return nil
}

func (a *App) step() error {
	a.ensureDimensions()

	now := time.Now()
	delta := now.Sub(a.last).Seconds()
	if delta <= 0 {
		delta = 1.0 / a.cfg.TargetFPS
	}
	a.last = now

	var features analyzer.Features
	if a.capture != nil && a.analyzer != nil {
		samples := a.capture.Samples()
		features = a.analyzer.Analyze(samples, delta)
	} else if a.fake != nil {
		features = a.fake.Next(delta)
	}

	a.params.ApplyFeatures(features, delta)
	a.params.UpdateTime(delta)

	fps := 1.0 / delta
	frame := a.renderer.Render(a.params, features, fps)
	statusText := frame.Status
	if a.deviceLabel != "" && a.cfg.DisableAudio == false {
		statusText = fmt.Sprintf("%s | mic=%s", statusText, a.deviceLabel)
	}

	moveCursorHome()
	for _, line := range frame.Lines {
		fmt.Println(line)
	}
	if a.cfg.ShowStatusBar {
		fmt.Println(statusBar(statusText, a.width))
	}
	return nil
}

func (a *App) ensureDimensions() {
	fd := int(os.Stdout.Fd())
	if fd < 0 {
		return
	}
	w, h, err := term.GetSize(fd)
	if err != nil || w <= 0 || h <= 0 {
		return
	}

	renderHeight := h
	if a.cfg.ShowStatusBar && renderHeight > 1 {
		renderHeight--
	}
	if renderHeight <= 0 {
		renderHeight = 1
	}

	if w == a.width && h == a.height && renderHeight == a.renderHeight {
		return
	}

	a.width = w
	a.height = h
	a.renderHeight = renderHeight
	a.renderer.Resize(w, renderHeight)
}

func (a *App) startInputListener(ctx context.Context) {
	if err := keyboard.Open(); err != nil {
		a.log.Printf("keyboard input disabled: %v", err)
		a.inputEvents = nil
		return
	}

	events := make(chan inputEvent, 16)
	a.inputEvents = events

	closeOnce := &sync.Once{}
	go func() {
		<-ctx.Done()
		closeOnce.Do(func() {
			_ = keyboard.Close()
		})
	}()

	go func() {
		defer close(events)
		defer closeOnce.Do(func() {
			_ = keyboard.Close()
		})
		for {
			char, key, err := keyboard.GetKey()
			if err != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
			switch {
			case key == keyboard.KeyEsc || key == keyboard.KeyCtrlC:
				events <- inputEventQuit
				return
			case char == 'q' || char == 'Q':
				events <- inputEventQuit
				return
			case char == 'r' || char == 'R':
				select {
				case events <- inputEventRandomize:
				default:
				}
			}
		}
	}()
}

func (a *App) randomizeVisuals() {
	if a.rng == nil {
		a.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	palette := pickRandom(a.paletteOptions, a.renderer.PaletteName(), a.rng)
	pattern := pickRandom(a.patternOptions, a.renderer.PatternName(), a.rng)
	color := pickRandom(a.colorOptions, a.renderer.ColorModeName(), a.rng)

	a.renderer.Configure(palette, pattern, color, a.colorOnAudio)
	a.params.Pattern = pattern
	a.params.ColorMode = color

	a.log.Printf("Randomize visuals -> palette=%s pattern=%s color=%s", palette, pattern, color)
}

func statusBar(text string, width int) string {
	if width <= 0 {
		return text
	}
	if len(text) >= width {
		return text[:width]
	}
	padding := width - len(text)
	return text + strings.Repeat(" ", padding)
}

func clearScreen() {
	fmt.Print("\x1b[2J")
	moveCursorHome()
}

func moveCursorHome() {
	fmt.Print("\x1b[H")
}

func hideCursor() {
	fmt.Print("\x1b[?25l")
}

func showCursor() {
	fmt.Print("\x1b[?25h")
}

func enterAltScreen() {
	fmt.Print("\x1b[?1049h")
}

func exitAltScreen() {
	fmt.Print("\x1b[?1049l\x1b[0m")
}

func pickRandom(options []string, current string, rng *rand.Rand) string {
	if len(options) == 0 {
		return current
	}
	if len(options) == 1 {
		return options[0]
	}
	var choice string
	for attempts := 0; attempts < 4; attempts++ {
		choice = options[rng.Intn(len(options))]
		if !strings.EqualFold(choice, current) {
			return choice
		}
	}
	return options[rng.Intn(len(options))]
}
