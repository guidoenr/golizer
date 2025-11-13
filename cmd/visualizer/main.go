package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/guidoenr/golizer/internal/app"
	"github.com/guidoenr/golizer/internal/audio"
	"golang.org/x/term"
)

func main() {
	var (
		deviceName = flag.String("audio-device", "", "Optional PortAudio device name (substring match)")
		width      = flag.Int("width", 80, "ASCII frame width")
		height     = flag.Int("height", 24, "ASCII frame height")
		targetFPS  = flag.Float64("fps", 1000, "Target frames per second")
		bufferSize = flag.Int("buffer-size", 2048, "FFT buffer size (power of two recommended)")
		noAudio    = flag.Bool("no-audio", false, "Run with synthetic audio (for testing)")
		debug      = flag.Bool("debug", false, "Enable verbose logging")
		showStatus = flag.Bool("status", true, "Display status bar")
		palette    = flag.String("palette", "auto", "ASCII palette (auto|default|box|lines|spark|retro|minimal|block|bubble)")
		pattern    = flag.String("pattern", "auto", "Visual pattern (auto|plasma|waves|ripples|nebula|noise|bands|strata|orbits)")
		colorMode  = flag.String("color-mode", "chromatic", "Color mode (chromatic|fire|aurora|mono)")
		listDevs   = flag.Bool("list-audio-devices", false, "List available audio input devices and exit")
		colorBurst = flag.Bool("color-on-audio", true, "Fade from monochrome to color based on audio energy")
		noColor    = flag.Bool("no-color", false, "Disable ANSI color output")
		quality    = flag.String("quality", "auto", "Quality preset (auto|high|balanced|eco)")
		autoRandom = flag.Bool("auto-randomize", true, "Automatically randomize visuals periodically")
		randomFreq = flag.Duration("randomize-interval", 10*time.Second, "Interval between automatic visual randomization")
	)

	flag.Parse()

	if *width <= 0 || *height <= 0 {
		log.Fatalf("invalid dimensions: width=%d height=%d", *width, *height)
	}

	if *targetFPS <= 0 {
		log.Fatalf("fps must be positive (got %.2f)", *targetFPS)
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
	if !flagIsPassed("fps") {
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
		Log:            logger,
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
			return "bands"
		case "balanced":
			return "waves"
		default:
			return "plasma"
		}
	}
	return name
}

func defaultFPSForQuality(quality string) float64 {
	switch quality {
	case "eco":
		return 72
	case "balanced":
		return 180
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
