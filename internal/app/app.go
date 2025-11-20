package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/guidoenr/golizer/internal/analyzer"
	"github.com/guidoenr/golizer/internal/audio"
	"github.com/guidoenr/golizer/internal/params"
	"github.com/guidoenr/golizer/internal/render"
	"golang.org/x/term"
)

// Config configures the application runtime.
type Config struct {
	DeviceName     string
	Width          int
	Height         int
	TargetFPS      float64
	BufferSize     int
	DisableAudio   bool
	ShowStatusBar  bool
	Palette        string
	Pattern        string
	ColorMode      string
	UseANSI        bool
	Quality        string
	AutoRandomize  bool
	RandomInterval time.Duration
	Backend        string
	FrameStride    int
	Scale          float64
	Fullscreen     bool
	NoiseFloor     float64
	ProfileLog     string
	Log            *log.Logger
}

type inputEvent int

const (
	inputEventRandomize inputEvent = iota
	inputEventQuit
)

// App ties together audio capture, analysis, and rendering.
type App struct {
	mu              sync.RWMutex
	cfg             Config
	params          params.Parameters
	renderer        *render.Renderer
	capture         *audio.Capture
	analyzer        *analyzer.Analyzer
	fake            *fakeGenerator
	last            time.Time
	log             *log.Logger
	deviceLabel     string
	width           int
	height          int
	renderHeight    int
	inputEvents     chan inputEvent
	rng             *rand.Rand
	paletteOptions  []string
	patternOptions  []string
	colorOptions    []string
	autoRandomize   bool
	randomInterval  time.Duration
	lastRandom      time.Time
	sampleBuffer    []float32
	frameBuffer     strings.Builder
	prevLines       []string
	currentLines    []string
	profiler        *profiler
	windowMode      bool
	frameStride     int
	skipCounter     int
	frameScale      float64
	fullscreen      bool
	lastFeatures    analyzer.Features
	lastFPS         float64
	lastSizeCheck   time.Time
	sizeCheckEvery  time.Duration
	analysisSamples int
	tempPath        string
	tempCheckEvery  time.Duration
	lastTempSample  time.Time
	lastTempC       float64
	hasTemp         bool
	lastThrottle    string
	panelURL        string
}

// New constructs the application using the provided configuration.
func New(cfg Config) (*App, error) {
	if cfg.TargetFPS <= 0 {
		cfg.TargetFPS = 90
	}
	if cfg.Log == nil {
		cfg.Log = log.New(os.Stdout, "", log.LstdFlags)
	}
	if cfg.Quality == "" {
		cfg.Quality = "high"
	}
	if cfg.RandomInterval <= 0 {
		cfg.RandomInterval = 10 * time.Second
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

	var backend render.Backend
	switch strings.ToLower(strings.TrimSpace(cfg.Backend)) {
	case "", "ascii", "terminal":
		backend = render.BackendASCII
	case "sdl", "window":
		backend = render.BackendSDL
	default:
		return nil, fmt.Errorf("unknown render backend %q", cfg.Backend)
	}

	renderer, err := render.NewWithBackend(backend, cfg.Width, renderHeight, cfg.Palette, cfg.Pattern, cfg.ColorMode, cfg.Quality, true, cfg.UseANSI)
	if err != nil {
		return nil, err
	}

	tempPath := strings.TrimSpace(os.Getenv("GOLIZER_TEMP_PATH"))
	if tempPath == "" {
		tempPath = "/sys/class/thermal/thermal_zone0/temp"
	}

	app := &App{
		cfg:             cfg,
		params:          params.Defaults(),
		renderer:        renderer,
		log:             cfg.Log,
		width:           cfg.Width,
		height:          cfg.Height,
		renderHeight:    renderHeight,
		autoRandomize:   cfg.AutoRandomize,
		randomInterval:  cfg.RandomInterval,
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
		paletteOptions:  render.PaletteNames(),
		patternOptions:  render.PatternNames(),
		colorOptions:    render.ColorModeNames(),
		sizeCheckEvery:  250 * time.Millisecond,
		analysisSamples: selectAnalysisWindow(cfg.BufferSize),
		tempPath:        tempPath,
		tempCheckEvery:  5 * time.Second,
	}
	app.lastSizeCheck = time.Now()
	app.lastRandom = time.Now()
	app.panelURL = detectPanelURL()
	app.windowMode = renderer.IsWindowed()
	if app.windowMode {
		app.cfg.ShowStatusBar = false
		renderer.SetScale(app.frameScale)
		renderer.SetFullscreen(app.fullscreen)
	}
	app.frameStride = cfg.FrameStride
	if app.frameStride <= 0 {
		app.frameStride = 1
	}
	app.frameScale = cfg.Scale
	if app.frameScale <= 0 {
		app.frameScale = 1.0
	}
	if len(app.paletteOptions) == 0 {
		app.paletteOptions = []string{"default"}
	}
	if len(app.patternOptions) == 0 {
		app.patternOptions = []string{"ripple"}
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
	if cfg.ProfileLog != "" {
		app.profiler = newProfiler(cfg.ProfileLog, cfg.Log)
	}
	return app, nil
}

// Run starts the render loop until context cancellation.
func (a *App) Run(ctx context.Context) error {
	frameSeconds := 1.0 / a.cfg.TargetFPS
	frameDuration := time.Duration(frameSeconds * float64(time.Second))
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	if !a.windowMode {
		enterAltScreen()
		clearScreen()
		hideCursor()
		defer func() {
			// always restore terminal state
			showCursor()
			exitAltScreen()
			// ensure we're back to normal mode
			fmt.Print("\x1b[0m")
		}()
	}

	inputCtx, cancelInput := context.WithCancel(ctx)
	defer cancelInput()
	a.startInputListener(inputCtx)
	a.ensureDimensions()

	for {
		select {
		case <-ctx.Done():
			if !a.windowMode {
				moveCursorHome()
				// restore terminal state immediately
				showCursor()
				exitAltScreen()
				fmt.Print("\x1b[0m")
			}
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
				if !a.windowMode {
					moveCursorHome()
					// restore terminal state immediately
					showCursor()
					exitAltScreen()
					fmt.Print("\x1b[0m")
				}
				return nil
			}
		case <-ticker.C:
			if err := a.step(); err != nil {
				if errors.Is(err, render.ErrRendererQuit) {
					return nil
				}
				return err
			}
			a.maybeAutoRandomize()
		}
	}
}

// Close releases held resources.
func (a *App) Close() error {
	if a.profiler != nil {
		_ = a.profiler.Close()
	}
	var firstErr error
	if a.renderer != nil {
		if err := a.renderer.Close(); err != nil {
			firstErr = err
		}
	}
	if a.capture != nil {
		if err := a.capture.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (a *App) step() error {
	if a.profiler != nil {
		a.profiler.beginFrame()
	}

	a.ensureDimensions()

	now := time.Now()
	delta := now.Sub(a.last).Seconds()
	if delta <= 0 {
		delta = 1.0 / a.cfg.TargetFPS
	}
	a.last = now

	var features analyzer.Features
	if a.capture != nil && a.analyzer != nil {
		if a.profiler != nil {
			a.profiler.markSection("capture")
		}
		a.sampleBuffer = a.capture.SamplesInto(a.sampleBuffer)
		samples := a.sampleBuffer
		if a.analysisSamples > 0 && len(samples) > a.analysisSamples {
			samples = samples[len(samples)-a.analysisSamples:]
		}
		if a.profiler != nil {
			a.profiler.markSection("analyze")
		}
		features = a.analyzer.Analyze(samples, delta)
		if a.cfg.NoiseFloor > 0 {
			features = analyzer.GateFeatures(features, a.cfg.NoiseFloor)
		}
	} else if a.fake != nil {
		features = a.fake.Next(delta)
	}
	if a.profiler != nil {
		a.profiler.markSection("params")
	}

	a.params.ApplyFeatures(features, delta)
	a.params.UpdateTime(delta)

	fps := 1.0 / delta

	// update last features and fps for web server
	a.mu.Lock()
	a.lastFeatures = features
	a.lastFPS = fps
	a.mu.Unlock()
	if a.profiler != nil {
		a.profiler.markSection("render")
	}
	if a.frameStride > 1 {
		if a.skipCounter < a.frameStride-1 {
			a.skipCounter++
			return nil
		}
		a.skipCounter = 0
	}

	frame := a.renderer.Render(a.params, features, fps)
	statusText := frame.Status
	if a.deviceLabel != "" && !a.cfg.DisableAudio {
		statusText = fmt.Sprintf("%s | mic=%s", statusText, a.deviceLabel)
	}

	if frame.Present != nil {
		if a.profiler != nil {
			a.profiler.markSection("present")
		}
		if err := frame.Present(statusText); err != nil {
			return err
		}
		if a.profiler != nil {
			a.profiler.endFrame()
		}
		return nil
	}

	a.frameBuffer.Reset()

	a.currentLines = a.currentLines[:0]
	a.currentLines = append(a.currentLines, frame.Lines...)
	if a.cfg.ShowStatusBar {
		a.overlayStatusLines(a.buildStatusLines(statusText, fps))
	}

	// ensure previous lines slice has capacity
	if len(a.prevLines) < len(a.currentLines) {
		a.prevLines = append(a.prevLines, make([]string, len(a.currentLines)-len(a.prevLines))...)
	}

	for i := len(a.currentLines); i < len(a.prevLines); i++ {
		a.prevLines[i] = ""
	}

	for idx, line := range a.currentLines {
		if idx < len(a.prevLines) && a.prevLines[idx] == line {
			continue
		}
		appendCursorMove(&a.frameBuffer, idx+1)
		a.frameBuffer.WriteString(line)
		a.frameBuffer.WriteString("\x1b[K")
	}

	if len(a.currentLines) < len(a.prevLines) {
		for idx := len(a.currentLines); idx < len(a.prevLines); idx++ {
			appendCursorMove(&a.frameBuffer, idx+1)
			a.frameBuffer.WriteString("\x1b[K")
		}
	}

	if a.frameBuffer.Len() > 0 {
		if _, err := os.Stdout.WriteString(a.frameBuffer.String()); err != nil {
			return err
		}
	}

	if cap(a.prevLines) < len(a.currentLines) {
		a.prevLines = make([]string, len(a.currentLines))
	} else {
		a.prevLines = a.prevLines[:len(a.currentLines)]
	}
	copy(a.prevLines, a.currentLines)

	if a.profiler != nil {
		a.profiler.markSection("flush")
		a.profiler.endFrame()
	}

	return nil
}

func (a *App) ensureDimensions() {
	if a.windowMode {
		return
	}

	if a.sizeCheckEvery > 0 {
		now := time.Now()
		if now.Sub(a.lastSizeCheck) < a.sizeCheckEvery {
			return
		}
		a.lastSizeCheck = now
	}

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
	a.prevLines = nil
}

func (a *App) startInputListener(ctx context.Context) {
	if a.windowMode {
		a.inputEvents = nil
		return
	}
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

	a.renderer.Configure(palette, pattern, color, true)
	a.params.Pattern = pattern
	a.params.ColorMode = color

	// commented out for now
	//a.log.Printf("Randomize visuals -> palette=%s pattern=%s color=%s", palette, pattern, color)
	a.mu.Lock()
	a.lastRandom = time.Now()
	a.mu.Unlock()
}

func (a *App) maybeAutoRandomize() {
	now := time.Now()

	a.mu.Lock()
	if !a.autoRandomize {
		a.mu.Unlock()
		return
	}

	if now.Sub(a.lastRandom) < a.randomInterval {
		a.mu.Unlock()
		return
	}

	a.lastRandom = now
	a.mu.Unlock()

	a.randomizeVisuals()
}

func (a *App) buildStatusLines(raw string, fps float64) []string {
	width := a.width
	temp, throttle := a.systemStats()

	entries := []statusEntry{
		{label: "PANEL", value: a.panelURL},
		{label: "TEMP", value: temp},
		{label: "THROTTLE", value: throttle},
		{label: "FPS", value: fmt.Sprintf("%.1f", fps)},
	}

	parts := strings.Split(raw, "|")
	if len(parts) > 0 {
		mode := strings.TrimSpace(parts[0])
		if mode != "" {
			entries = append(entries, statusEntry{label: "MODE", value: strings.ToUpper(mode)})
		}
	}
	if len(parts) > 1 {
		entries = append(entries, parseKeyValuePart(parts[1])...)
	}
	if len(parts) > 2 {
		entries = append(entries, parseMetricPart(parts[2])...)
	}

	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.value == "" {
			continue
		}
		lines = append(lines, padLine(formatStatusEntry(entry), width))
	}
	return lines
}

func (a *App) overlayStatusLines(lines []string) {
	if len(lines) == 0 || len(a.currentLines) == 0 {
		return
	}
	limit := len(lines)
	if limit > len(a.currentLines) {
		limit = len(a.currentLines)
	}
	for i := 0; i < limit; i++ {
		a.currentLines[i] = padLine(lines[i], a.width)
	}
}

func (a *App) systemStats() (string, string) {
	if a.tempPath != "" {
		now := time.Now()
		if now.Sub(a.lastTempSample) >= a.tempCheckEvery {
			if temp, throttle, err := readSystemStats(a.tempPath); err == nil {
				a.lastTempC = temp
				a.lastThrottle = throttle
				a.hasTemp = true
			} else {
				a.hasTemp = false
				a.lastThrottle = ""
			}
			a.lastTempSample = now
		}
	}

	temp := "-- °C"
	if a.hasTemp {
		temp = fmt.Sprintf("%.1f°C", a.lastTempC)
	}

	throttle := a.lastThrottle
	if throttle == "" {
		throttle = "NORMAL"
	}

	return temp, throttle
}

func padLine(text string, width int) string {
	if width <= 0 {
		return text
	}
	if len(text) >= width {
		return text[:width]
	}
	return text + strings.Repeat(" ", width-len(text))
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

func appendCursorMove(builder *strings.Builder, row int) {
	builder.WriteString("\x1b[")
	builder.WriteString(strconv.Itoa(row))
	builder.WriteString(";1H")
}

func selectAnalysisWindow(bufferSize int) int {
	if bufferSize <= 0 {
		bufferSize = 4096
	}
	window := bufferSize / 4
	if window < 256 {
		window = 256
	}
	if window > 2048 {
		window = 2048
	}
	return window
}

func readSystemStats(tempPath string) (float64, string, error) {
	data, err := os.ReadFile(tempPath)
	if err != nil {
		return 0, "", err
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return 0, "", err
	}

	throttle := readThrottleStatus()
	return value / 1000.0, throttle, nil
}

func readThrottleStatus() string {
	data, err := os.ReadFile("/sys/devices/platform/soc/soc:firmware/get_throttled")
	if err != nil {
		return ""
	}

	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return ""
	}

	value, err := strconv.ParseUint(raw, 0, 64)
	if err != nil {
		value, err = strconv.ParseUint(raw, 16, 64)
		if err != nil {
			return ""
		}
	}

	if value == 0 {
		return "NORMAL"
	}

	flags := []string{}
	if value&0x1 != 0 {
		flags = append(flags, "UNDER-VOLTAGE")
	}
	if value&0x2 != 0 {
		flags = append(flags, "ARM CAPPED")
	}
	if value&0x4 != 0 {
		flags = append(flags, "THROTTLED")
	}
	if value&0x10000 != 0 {
		flags = append(flags, "WAS UNDERVOLTED")
	}
	if value&0x20000 != 0 {
		flags = append(flags, "WAS CAPPED")
	}
	if value&0x40000 != 0 {
		flags = append(flags, "WAS THROTTLED")
	}
	if len(flags) == 0 {
		return "NORMAL"
	}
	return strings.Join(flags, ", ")
}

type statusEntry struct {
	label string
	value string
}

const (
	statusLabelColor = "\x1b[38;5;213m"
	statusValueColor = "\x1b[38;5;250m"
)

func formatStatusEntry(entry statusEntry) string {
	label := fmt.Sprintf("%s%-10s\x1b[0m", statusLabelColor, entry.label)
	value := fmt.Sprintf("%s%s\x1b[0m", statusValueColor, entry.value)
	return label + " " + value
}

func parseKeyValuePart(part string) []statusEntry {
	tokens := strings.Fields(part)
	entries := make([]statusEntry, 0, len(tokens))
	for _, token := range tokens {
		if !strings.Contains(token, "=") {
			continue
		}
		segments := strings.SplitN(token, "=", 2)
		if len(segments) != 2 {
			continue
		}
		label := strings.ToUpper(strings.ReplaceAll(segments[0], "_", " "))
		value := segments[1]
		if label == "" || value == "" {
			continue
		}
		entries = append(entries, statusEntry{label: label, value: value})
	}
	return entries
}

func parseMetricPart(part string) []statusEntry {
	tokens := strings.Fields(part)
	entries := make([]statusEntry, 0, len(tokens)/2)
	for i := 0; i+1 < len(tokens); i += 2 {
		label := strings.ToUpper(tokens[i])
		if label == "FPS" {
			continue
		}
		entries = append(entries, statusEntry{label: label, value: tokens[i+1]})
	}
	return entries
}

func detectPanelURL() string {
	if env := strings.TrimSpace(os.Getenv("GOLIZER_PANEL_URL")); env != "" {
		return env
	}

	port := 8080
	if env := strings.TrimSpace(os.Getenv("GOLIZER_WEB_PORT")); env != "" {
		if p, err := strconv.Atoi(env); err == nil && p > 0 {
			port = p
		}
	}

	ip := firstLocalIP()
	if ip == "" {
		ip = "golizer.local"
	}
	return fmt.Sprintf("http://%s:%d", ip, port)
}

func firstLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.IsLoopback() {
				continue
			}
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	return ""
}

// GetParams returns current parameters (thread-safe)
func (a *App) GetParams() params.Parameters {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.params
}

// SetParams updates parameters (thread-safe)
func (a *App) SetParams(p params.Parameters) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.params = p
}

// GetRenderer returns the renderer (thread-safe)
func (a *App) GetRenderer() *render.Renderer {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.renderer
}

// GetFeatures returns last analyzed features (thread-safe)
func (a *App) GetFeatures() analyzer.Features {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastFeatures
}

// GetFPS returns last FPS (thread-safe)
func (a *App) GetFPS() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastFPS
}

// ConfigGetter interface for accessing config values (matches web.AppInterface)
type ConfigGetter interface {
	NoiseFloor() float64
	BufferSize() int
	TargetFPS() float64
	Quality() string
	Width() int
	Height() int
	AutoRandomize() bool
	RandomInterval() time.Duration
	ShowStatusBar() bool
}

// GetConfig returns current configuration (thread-safe)
func (a *App) GetConfig() ConfigGetter {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return &configWrapper{cfg: a.cfg}
}

type configWrapper struct {
	cfg Config
}

func (c *configWrapper) NoiseFloor() float64           { return c.cfg.NoiseFloor }
func (c *configWrapper) BufferSize() int               { return c.cfg.BufferSize }
func (c *configWrapper) TargetFPS() float64            { return c.cfg.TargetFPS }
func (c *configWrapper) Quality() string               { return c.cfg.Quality }
func (c *configWrapper) Width() int                    { return c.cfg.Width }
func (c *configWrapper) Height() int                   { return c.cfg.Height }
func (c *configWrapper) AutoRandomize() bool           { return c.cfg.AutoRandomize }
func (c *configWrapper) RandomInterval() time.Duration { return c.cfg.RandomInterval }
func (c *configWrapper) ShowStatusBar() bool           { return c.cfg.ShowStatusBar }

// SetNoiseFloor updates noise floor (thread-safe)
func (a *App) SetNoiseFloor(v float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.NoiseFloor = v
	// noise floor is used during analysis, no need to update analyzer
}

// SetBufferSize updates buffer size (thread-safe)
func (a *App) SetBufferSize(v int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.BufferSize = v
	// note: buffer size change requires restart to take effect
}

// SetTargetFPS updates target FPS (thread-safe)
func (a *App) SetTargetFPS(v float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.TargetFPS = v
}

// SetDimensions updates dimensions (thread-safe)
func (a *App) SetDimensions(width, height int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.width = width
	a.height = height
	a.cfg.Width = width
	a.cfg.Height = height
	a.renderHeight = height
	if a.cfg.ShowStatusBar && a.renderHeight > 1 {
		a.renderHeight--
	}
	if a.renderer != nil {
		a.renderer.Resize(width, a.renderHeight)
	}
}

// SetShowStatusBar toggles the ASCII status bar (thread-safe)
func (a *App) SetShowStatusBar(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.windowMode {
		enabled = false
	}

	if a.cfg.ShowStatusBar == enabled {
		return
	}

	a.cfg.ShowStatusBar = enabled

	if a.windowMode {
		return
	}

	a.renderHeight = a.height
	if a.cfg.ShowStatusBar && a.renderHeight > 1 {
		a.renderHeight--
	}
	if a.renderer != nil {
		a.renderer.Resize(a.width, a.renderHeight)
	}
}

// SetAutoRandomize updates auto randomize (thread-safe)
func (a *App) SetAutoRandomize(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.autoRandomize = v
	a.cfg.AutoRandomize = v
}

// SetRandomInterval updates random interval (thread-safe)
func (a *App) SetRandomInterval(v time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.randomInterval = v
	a.cfg.RandomInterval = v
}
