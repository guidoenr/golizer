package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/guidoenr/chroma/go-implementation/internal/app"
	"github.com/guidoenr/chroma/go-implementation/internal/audio"
	"golang.org/x/term"
)

func main() {
	var (
		deviceName = flag.String("audio-device", "", "Optional PortAudio device name (substring match)")
		width      = flag.Int("width", 80, "ASCII frame width")
		height     = flag.Int("height", 24, "ASCII frame height")
		targetFPS  = flag.Float64("fps", 60, "Target frames per second")
		bufferSize = flag.Int("buffer-size", 2048, "FFT buffer size (power of two recommended)")
		noAudio    = flag.Bool("no-audio", false, "Run with synthetic audio (for testing)")
		debug      = flag.Bool("debug", false, "Enable verbose logging")
		showStatus = flag.Bool("status", true, "Display status bar")
		palette    = flag.String("palette", "default", "ASCII palette (default|box|lines|spark)")
		pattern    = flag.String("pattern", "plasma", "Visual pattern (plasma|waves|ripples|nebula|noise)")
		colorMode  = flag.String("color-mode", "chromatic", "Color mode (chromatic|fire|aurora|mono)")
		listDevs   = flag.Bool("list-audio-devices", false, "List available audio input devices and exit")
		colorBurst = flag.Bool("color-on-audio", false, "Fade from monochrome to color based on audio energy")
		noColor    = flag.Bool("no-color", false, "Disable ANSI color output")
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

	logger := log.New(os.Stdout, "[chroma-go] ", log.LstdFlags)
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

	appConfig := app.Config{
		DeviceName:    *deviceName,
		Width:         *width,
		Height:        *height,
		TargetFPS:     *targetFPS,
		BufferSize:    *bufferSize,
		DisableAudio:  *noAudio,
		ShowStatusBar: *showStatus,
		Palette:       *palette,
		Pattern:       *pattern,
		ColorMode:     *colorMode,
		ColorOnAudio:  *colorBurst,
		UseANSI:       !*noColor,
		Log:           logger,
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
