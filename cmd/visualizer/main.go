package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	rdebug "runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/guidoenr/golizer/internal/app"
	"github.com/guidoenr/golizer/internal/audio"
	"github.com/guidoenr/golizer/internal/render"
	"golang.org/x/term"
)

func main() {
	var (
		deviceName = flag.String("audio-device", "", "Optional PortAudio device name (substring match)")
		width      = flag.Int("width", 120, "Frame width (ASCII columns or SDL resolution)")
		height     = flag.Int("height", 40, "Frame height (ASCII rows or SDL resolution)")
		targetFPS  = flag.Float64("fps", 0, "Target frames per second (0 = unlimited)")
		bufferSize = flag.Int("buffer-size", 2048, "FFT buffer size (power of two recommended)")
		noAudio    = flag.Bool("no-audio", false, "Run with synthetic audio (for testing)")
		debug      = flag.Bool("debug", false, "Enable verbose logging")
		showStatus = flag.Bool("status", true, "Display status bar")
		palette    = flag.String("palette", "auto", "ASCII palette (auto|default|box|lines|spark|retro|minimal|block|bubble)")
		pattern    = flag.String("pattern", "auto", "Visual pattern (auto|flash|spark|scatter|beam|ripple|strobe|laser|orbit|explosion|rings|zigzag|cross|spiral|star)")
		colorMode  = flag.String("color-mode", "chromatic", "Color mode (chromatic|fire|aurora|mono)")
		listDevs   = flag.Bool("list-audio-devices", false, "List available audio input devices and exit")
		colorBurst = flag.Bool("color-on-audio", true, "Fade from monochrome to color based on audio energy")
		noColor    = flag.Bool("no-color", false, "Disable ANSI color output")
		quality    = flag.String("quality", "balanced", "Quality preset (auto|high|balanced|eco)")
		autoRandom = flag.Bool("auto-randomize", true, "Automatically randomize visuals periodically")
		randomFreq = flag.Duration("randomize-interval", 10*time.Second, "Interval between automatic visual randomization")
		backend    = flag.String("backend", "ascii", "Renderer backend (auto|ascii|sdl)")
		stride     = flag.Int("stride", 1, "Render every Nth frame (1 = no skip)")
		frameScale = flag.Float64("scale", 1.0, "Pixel scale multiplier (SDL)")
		fullscreen = flag.Bool("fullscreen", false, "Use fullscreen SDL window")
		profileLog = flag.String("profile-log", "", "Optional path to append frame timing metrics")
		noiseFloor = flag.Float64("noise-floor", 0.20, "Energy gate to ignore ambient noise (0-0.5)")
	)

	flag.Parse()

	runtime.GOMAXPROCS(runtime.NumCPU())
	rdebug.SetGCPercent(200)

	if *profileLog == "" {
		if envPath := strings.TrimSpace(os.Getenv("GOLIZER_PROFILE_LOG")); envPath != "" {
			*profileLog = envPath
		} else {
			*profileLog = filepath.Join(os.TempDir(), "golizer_profile.csv")
		}
	}

	backendName, err := resolveBackend(*backend)
	if err != nil {
		log.Fatalf("backend: %v", err)
	}

	if *width <= 0 || *height <= 0 {
		log.Fatalf("invalid dimensions: width=%d height=%d", *width, *height)
	}

	if *targetFPS < 0 {
		log.Fatalf("fps must be positive or 0 for unlimited (got %.2f)", *targetFPS)
	}

	if *bufferSize <= 0 {
		log.Fatalf("buffer-size must be positive (got %d)", *bufferSize)
	}

	if fd := int(os.Stdout.Fd()); fd >= 0 {
		if w, h, err := term.GetSize(fd); err == nil {
			if w > 0 {
				*width = w
			}
			if h > 0 {
				*height = h
			}
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := log.New(os.Stdout, "[golizer] ", log.LstdFlags)
	if !*debug {
		logger.SetOutput(os.Stderr)
		logger.SetFlags(0)
	}

	if *profileLog != "" {
		logger.Printf("profile log -> %s", *profileLog)
	}
	if backendName != "" {
		logger.Printf("render backend -> %s", backendName)
	}

	needAudio := !*noAudio || *listDevs
	if needAudio {
		if err := audio.Initialize(); err != nil {
			logger.Fatalf("failed to initialize PortAudio: %v", err)
		}
		defer audio.Terminate()
	}

	if *listDevs {
		devices, err := audio.ListDevices()
		if err != nil {
			logger.Fatalf("list devices: %v", err)
		}
		fmt.Printf("\n=== Audio Input Devices ===\n\n")
		for _, dev := range devices {
			if dev.MaxInput == 0 {
				continue
			}
			markers := ""
			if dev.IsDefaultInput {
				markers += " (default)"
			}
			fmt.Printf("- %s [%s]%s\n    inputs:%d outputs:%d sample:%.0f Hz\n",
				dev.Name, dev.HostAPI, markers, dev.MaxInput, dev.MaxOutput, dev.DefaultSampleHz)
		}
		if dev, err := audio.AutoDetectDevice(); err == nil && dev != nil {
			fmt.Printf("\nAuto-detected input: %s (%.0f Hz, %d channels)\n", dev.Name, dev.DefaultSampleRate, dev.MaxInputChannels)
		}
		return
	}

	qualityName, err := resolveQualityPreset(*quality)
	if err != nil {
		logger.Fatalf("quality: %v", err)
	}
	if strings.EqualFold(*quality, "auto") {
		logger.Printf("quality auto -> %s (arch=%s cores=%d)", qualityName, runtime.GOARCH, runtime.NumCPU())
	}

	paletteName := resolvePaletteName(*palette, qualityName)
	patternName := resolvePatternName(*pattern, qualityName)
	colorModeName := strings.ToLower(strings.TrimSpace(*colorMode))
	if colorModeName == "" {
		colorModeName = "chromatic"
	}

	targetFPSValue := *targetFPS
	if !flagIsPassed("fps") || targetFPSValue == 0 {
		targetFPSValue = defaultFPSForQuality(qualityName)
	}

	if strings.EqualFold(*palette, "auto") || strings.TrimSpace(*palette) == "" {
		logger.Printf("palette auto -> %s", paletteName)
	}
	if strings.EqualFold(*pattern, "auto") || strings.TrimSpace(*pattern) == "" {
		logger.Printf("pattern auto -> %s", patternName)
	}

	appConfig := app.Config{
		DeviceName:     *deviceName,
		Width:          *width,
		Height:         *height,
		TargetFPS:      targetFPSValue,
		BufferSize:     *bufferSize,
		DisableAudio:   *noAudio,
		ShowStatusBar:  *showStatus,
		Palette:        paletteName,
		Pattern:        patternName,
		ColorMode:      colorModeName,
		ColorOnAudio:   *colorBurst,
		UseANSI:        !*noColor,
		Quality:        qualityName,
		AutoRandomize:  *autoRandom,
		RandomInterval: *randomFreq,
		ProfileLog:     *profileLog,
		Backend:        backendName,
		FrameStride:    maxInt(1, *stride),
		Scale:          clampFloat(*frameScale, 0.25, 4.0),
		Fullscreen:     *fullscreen,
		NoiseFloor:     clampFloat(*noiseFloor, 0.0, 0.5),
		Log:            logger,
	}

	if appConfig.Quality == "eco" && !flagIsPassed("fps") && appConfig.TargetFPS > 48 {
		appConfig.TargetFPS = 48
	}
	if appConfig.Quality == "balanced" && !flagIsPassed("fps") && appConfig.TargetFPS > 90 {
		appConfig.TargetFPS = 90
	}

	a, err := app.New(appConfig)
	if err != nil {
		logger.Fatalf("failed to create app: %v", err)
	}
	defer func() {
		if err := a.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup error: %v\n", err)
		}
	}()

	if err := a.Run(ctx); err != nil {
		if ctx.Err() != nil {
			fmt.Println("\nExiting...")
			return
		}
		logger.Fatalf("runtime error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
}

func resolveQualityPreset(input string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(input))
	if value == "" || value == "auto" {
		if env := strings.TrimSpace(os.Getenv("GOLIZER_QUALITY")); env != "" {
			value = strings.ToLower(env)
		}
	}
	switch value {
	case "", "auto":
		return autoQualityPreset(), nil
	case "high", "balanced", "eco":
		return value, nil
	default:
		return "", fmt.Errorf("unknown quality preset %q", input)
	}
}

func autoQualityPreset() string {
	arch := runtime.GOARCH
	cores := runtime.NumCPU()
	if arch == "arm64" || arch == "arm" {
		if cores <= 4 {
			return "eco"
		}
		return "balanced"
	}
	if cores <= 4 {
		return "balanced"
	}
	return "high"
}

func resolvePaletteName(requested string, quality string) string {
	name := strings.ToLower(strings.TrimSpace(requested))
	if name == "" || name == "auto" {
		switch quality {
		case "eco":
			return "minimal"
		case "balanced":
			return "retro"
		default:
			return "default"
		}
	}
	return name
}

func resolvePatternName(requested string, quality string) string {
	name := strings.ToLower(strings.TrimSpace(requested))
	if name == "" || name == "auto" {
		switch quality {
		case "eco":
			return "flash"
		case "balanced":
			return "ripple"
		default:
			return "spiral"
		}
	}
	return name
}

func defaultFPSForQuality(quality string) float64 {
	switch quality {
	case "eco":
		return 48
	case "balanced":
		return 90
	default:
		return 1000
	}
}

func flagIsPassed(name string) bool {
	found := false
	flag.CommandLine.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func resolveBackend(input string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(input))
	if value == "" || value == "auto" {
		if env := strings.TrimSpace(os.Getenv("GOLIZER_BACKEND")); env != "" {
			value = strings.ToLower(env)
		}
	}
	switch value {
	case "", "auto":
		if render.SupportsSDL() && runtime.GOOS == "linux" && (runtime.GOARCH == "arm64" || runtime.GOARCH == "arm") {
			return "sdl", nil
		}
		return "ascii", nil
	case "ascii", "terminal", "tty":
		return "ascii", nil
	case "sdl", "window":
		if !render.SupportsSDL() {
			return "", fmt.Errorf("SDL backend not available in this build (rebuild with -tags sdl)")
		}
		return "sdl", nil
	default:
		return "", fmt.Errorf("unknown backend %q", input)
	}
}

func clampFloat(v, minVal, maxVal float64) float64 {
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
