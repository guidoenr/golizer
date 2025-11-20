package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	rdebug "runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/guidoenr/golizer/internal/app"
	"github.com/guidoenr/golizer/internal/audio"
	"github.com/guidoenr/golizer/internal/params"
	"github.com/guidoenr/golizer/internal/render"
	"github.com/guidoenr/golizer/internal/web"
	"golang.org/x/term"
)

func main() {
	var (
		deviceName = flag.String("audio-device", "", "Optional PortAudio device name (substring match)")
		width      = flag.Int("width", 120, "Frame width (ASCII columns or SDL resolution)")
		height     = flag.Int("height", 40, "Frame height (ASCII rows or SDL resolution)")
		// FPS removed - always unlimited, each machine runs at its max
		bufferSize = flag.Int("buffer-size", 2048, "FFT buffer size (power of two recommended)")
		noAudio    = flag.Bool("no-audio", false, "Run with synthetic audio (for testing)")
		debug      = flag.Bool("debug", false, "Enable verbose logging")
		showStatus = flag.Bool("status", true, "Display status bar")
		palette    = flag.String("palette", "auto", "ASCII palette (auto|default|box|lines|spark|retro|minimal|block|bubble)")
		pattern    = flag.String("pattern", "auto", "Visual pattern (auto|flash|spark|scatter|beam|ripple|laser|orbit|explosion|rings|zigzag|cross|spiral|star|tunnel|neurons|fractal)")
		colorMode  = flag.String("color-mode", "chromatic", "Color mode (chromatic|fire|aurora|mono)")
		listDevs   = flag.Bool("list-audio-devices", false, "List available audio input devices and exit")
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
		webPort    = flag.Int("web-port", 8080, "Web server port (0 = disabled, default: 8080)")
		noWeb      = flag.Bool("no-web", false, "Disable web server")
		showWebURL = flag.Bool("show-web-url", true, "Show web panel URL in status bar")
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

	// FPS always unlimited
	targetFPSValue := 0.0

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

	// ensure terminal is restored on any exit (including panic)
	defer func() {
		// restore terminal state
		fmt.Print("\x1b[?25h")   // show cursor
		fmt.Print("\x1b[?1049l") // exit alternate screen
		fmt.Print("\x1b[0m")     // reset colors
	}()

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

	// FPS always unlimited - let each machine run at max
	targetFPSValue = 0.0 // always unlimited

	if strings.EqualFold(*palette, "auto") || strings.TrimSpace(*palette) == "" {
		logger.Printf("palette auto -> %s", paletteName)
	}
	if strings.EqualFold(*pattern, "auto") || strings.TrimSpace(*pattern) == "" {
		logger.Printf("pattern auto -> %s", patternName)
	}

	// load saved config if exists
	savedConfig := loadSavedConfig()
	if savedConfig != nil {
		logger.Printf("loaded saved config from %s", getConfigPath())
		// apply saved config only if flags weren't passed
		if !flagIsPassed("palette") && savedConfig.Palette != "" {
			paletteName = savedConfig.Palette
		}
		if !flagIsPassed("pattern") && savedConfig.Pattern != "" {
			patternName = savedConfig.Pattern
		}
		if !flagIsPassed("color-mode") && savedConfig.ColorMode != "" {
			colorModeName = savedConfig.ColorMode
		}
		if !flagIsPassed("noise-floor") && savedConfig.NoiseFloor > 0 {
			*noiseFloor = savedConfig.NoiseFloor
		}
		if !flagIsPassed("buffer-size") && savedConfig.BufferSize > 0 {
			*bufferSize = savedConfig.BufferSize
		}
		// FPS always unlimited - ignore saved FPS
		if !flagIsPassed("quality") && savedConfig.Quality != "" {
			qualityName = savedConfig.Quality
		}
		if !flagIsPassed("width") && savedConfig.Width > 0 {
			*width = savedConfig.Width
		}
		if !flagIsPassed("height") && savedConfig.Height > 0 {
			*height = savedConfig.Height
		}
		if !flagIsPassed("status") {
			*showStatus = savedConfig.ShowStatusBar
		}
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

	// FPS always unlimited - removed quality-based FPS limits

	a, err := app.New(appConfig)
	if err != nil {
		logger.Fatalf("failed to create app: %v", err)
	}
	defer func() {
		if err := a.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup error: %v\n", err)
		}
	}()

	// apply saved parameters if config was loaded
	if savedConfig != nil {
		a.SetParams(savedConfig.Params)
	}

	// start web server automatically (unless disabled)
	if !*noWeb && *webPort > 0 {
		webServer := web.NewServer(a)
		go func() {
			if err := webServer.Start(*webPort); err != nil {
				logger.Printf("web server error: %v", err)
			}
		}()

		// try to setup mDNS automatically
		go setupMDNS(*webPort, logger)

		// get local IP for display
		localIP := getLocalIP()
		logger.Printf("web control panel:")
		logger.Printf("  local:  http://localhost:%d", *webPort)
		if localIP != "" {
			logger.Printf("  network: http://%s:%d", localIP, *webPort)
		}
		logger.Printf("  mDNS:   http://golizer.local:%d (if configured)", *webPort)

		// set web panel URL in renderer status bar
		if *showWebURL {
			webURL := fmt.Sprintf("http://localhost:%d", *webPort)
			if localIP != "" {
				webURL = fmt.Sprintf("http://%s:%d", localIP, *webPort)
			}
			a.GetRenderer().SetWebPanelURL(webURL)
			a.GetRenderer().SetShowWebPanelURL(true)
		}
	}

	if err := a.Run(ctx); err != nil {
		if ctx.Err() != nil {
			// signal received, restore terminal and exit cleanly
			fmt.Print("\n")
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

// FPS function removed - always unlimited

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

// setupMDNS tries to configure avahi-daemon for golizer.local
func setupMDNS(port int, logger *log.Logger) {
	// check if avahi-daemon is installed and running
	if _, err := exec.LookPath("avahi-daemon"); err != nil {
		logger.Printf("[web] mDNS: avahi-daemon not found (install: sudo apt install avahi-daemon)")
		return
	}

	// check if avahi-daemon is running
	cmd := exec.Command("systemctl", "is-active", "--quiet", "avahi-daemon")
	if err := cmd.Run(); err != nil {
		logger.Printf("[web] mDNS: avahi-daemon not running (start: sudo systemctl start avahi-daemon)")
		return
	}

	// check if service file already exists
	serviceFile := "/etc/avahi/services/golizer.service"
	if _, err := os.Stat(serviceFile); err == nil {
		logger.Printf("[web] mDNS: already configured at http://golizer.local:%d", port)
		return
	}

	// try to create service file (might need sudo)
	serviceContent := fmt.Sprintf(`<?xml version="1.0" standalone='no'?>
<!DOCTYPE service-group SYSTEM "avahi-service.dtd">
<service-group>
  <name replace-wildcards="yes">golizer</name>
  <service>
    <type>_http._tcp</type>
    <port>%d</port>
    <txt-record>path=/</txt-record>
  </service>
</service-group>
`, port)

	// try without sudo first (if user has write access)
	if err := os.WriteFile(serviceFile, []byte(serviceContent), 0644); err != nil {
		// try with sudo
		writeCmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | sudo tee %s > /dev/null", serviceContent, serviceFile))
		if err := writeCmd.Run(); err != nil {
			logger.Printf("[web] mDNS: can't configure automatically (run: sudo tee /etc/avahi/services/golizer.service)")
			return
		}
	}

	// reload avahi
	reloadCmd := exec.Command("sudo", "systemctl", "reload", "avahi-daemon")
	reloadCmd.Run() // ignore error

	logger.Printf("[web] mDNS: configured at http://golizer.local:%d", port)
}

// getLocalIP returns the first non-loopback IP address
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// saved config type (matches web.SavedConfig)
type savedConfig struct {
	Params        params.Parameters `json:"params"`
	Palette       string            `json:"palette"`
	Pattern       string            `json:"pattern"`
	ColorMode     string            `json:"colorMode"`
	NoiseFloor    float64           `json:"noiseFloor"`
	BufferSize    int               `json:"bufferSize"`
	TargetFPS     float64           `json:"targetFPS"`
	Quality       string            `json:"quality"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	ShowStatusBar bool              `json:"showStatusBar"`
}

func getConfigPath() string {
	// try to save in same directory as binary
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		return filepath.Join(exeDir, "golizer-config.json")
	}
	// fallback to home directory
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".golizer-config.json")
}

func loadSavedConfig() *savedConfig {
	configPath := getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil // config file doesn't exist, that's ok
	}
	var config savedConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil // invalid config, ignore
	}
	return &config
}
