# golizer

https://github.com/user-attachments/assets/3036c1f3-927f-493b-ad39-41aec2f9efa1

<img width="1008" height="598" alt="status_bar" src="https://github.com/user-attachments/assets/9de11cfc-8554-4de9-ac99-648d53d60185" />


> the setup showed in video is: Raspberry Pi4 Model B, with a Mic plugged in and a HDMI port to my TV

> the raspberry is running the `golizer` binary and listening to a analog sound system

real-time audio visualizer for the terminal. optimized for raspberry pi 4 and debian, runs smooth at 80+ fps with neon colors and minimal patterns.
this exists because of [this](https://github.com/yuri-xyz/chroma/issues/14)

## what is this

cpu-based audio reactive visualizer written in go. started as a port of [Chroma](https://github.com/yuri-xyz/chroma) but ended up being its own thing - faster, lighter, and way more trippy for electronic music.

captures audio via portaudio, does fft analysis for bass/mid/treble, detects kicks and drops, then renders sick ascii visuals that react to the music in real-time.

## features

- **audio reactive**: responds to kicks, snares, and low-end frequencies (not that crazy high-end stuff)
- **neon colors only**: red, cyan, blue, violet, pink. always saturated, never gray
- **16 sparse patterns**: flash, spark, scatter, beam, ripple, laser, orbit, explosion, rings, zigzag, cross, spiral, star, tunnel, neurons, fractal
- **optimized af**: 60-90 fps on raspberry pi 4, 200+ fps on desktop
- **auto-randomize**: patterns change every 10 seconds (configurable)
- **quality presets**: eco/balanced/high - auto-detects your platform
- **simple ascii**: fast characters (.,:;ox%#@) instead of slow unicode
- **black by default**: screen stays black until audio kicks in, then it explodes
- **web control panel**: full web interface to control everything from your phone (http://golizer.local:8080)

## quick start

### build it

```bash
./build.sh
```

the script auto-detects your architecture and builds the right binary:
- **x86_64/amd64** → `golizer-debian`
- **aarch64/arm64** → `golizer-pi`

### run it

```bash
# let it rip (auto-detects everything)
./golizer-debian

# raspberry pi
./golizer-pi

# custom settings
./golizer-debian --quality balanced --fps 90 --pattern pulse

# list audio devices
./golizer-debian --list-audio-devices
```

## all options

```bash
# audio
--audio-device "name"          # specific audio input
--buffer-size 2048             # fft buffer size (power of 2)
--noise-floor 0.20             # gate to ignore ambient noise
--no-audio                     # synthetic mode (for testing)

# visuals
--width 120                    # frame width (columns)
--height 40                    # frame height (rows)
--fps 90                       # target fps (0 = unlimited)
--quality balanced             # auto|high|balanced|eco
--backend ascii                # ascii|sdl
--palette auto                 # auto|default|box|lines|spark|retro|minimal|block|bubble
--pattern auto                 # auto|flash|spark|scatter|beam|ripple|laser|orbit|explosion|rings|zigzag|cross|spiral|star|tunnel|neurons|fractal
--color-mode chromatic         # chromatic|fire|aurora|mono

# randomization
--auto-randomize               # enable auto pattern switching
--randomize-interval 10s       # how often to randomize

# display
--status                       # show status bar
--no-color                     # disable ansi colors
--fullscreen                   # sdl fullscreen mode

# web server
--web-port 8080                # web control panel port (default: 8080, 0 = disabled)
--no-web                       # disable web server
--show-web-url                 # show web panel URL in status bar (default: true)

# debug
--debug                        # verbose logging
--profile-log path.csv         # frame timing metrics
```

## web control panel

golizer includes a full web interface to control everything from your phone or any device on your local network.

if you click on `status bar`, 

### web server (automatic)

the web server starts automatically on port 8080. just run:

```bash
./golizer-pi
```

then open in your browser:
- **http://localhost:8080** (on the pi itself)
- **http://<pi-ip>:8080** (from any device on your network)
- **http://golizer.local:8080** (if mDNS is configured - the binary tries to set this up automatically)

to disable the web server:
```bash
./golizer-pi --no-web
```

to change the port:
```bash
./golizer-pi --web-port 9000
```

### auto-start on boot (raspberry pi)

the web server starts automatically when you run the binary. to make it start on boot, create a systemd service:

```bash
# create service file
sudo tee /etc/systemd/system/golizer.service > /dev/null <<EOF
[Unit]
Description=golizer audio visualizer
After=network.target sound.target

[Service]
Type=simple
User=pi
WorkingDirectory=/home/pi/golizer
ExecStart=/home/pi/golizer/golizer-pi
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# enable and start
sudo systemctl daemon-reload
sudo systemctl enable golizer
sudo systemctl start golizer
```

the binary will automatically try to configure mDNS for `golizer.local` access.

### web panel features

- **visuals**: change pattern, palette, color mode in real-time
- **audio**: adjust noise floor, buffer size, see live audio stats
- **performance**: control fps, quality, resolution
- **parameters**: fine-tune frequency, amplitude, speed, brightness, contrast, saturation
- **beat response**: adjust sensitivity and influence of bass/mid/treble
- **randomization**: enable/disable auto-randomize, set interval, trigger manually
- **save config**: click "💾 SAVE" button to save all current settings as defaults

all changes apply instantly via websocket connection. saved config is loaded automatically on next startup.

## keyboard controls

- `R` - randomize pattern/palette/colors
- `Q` or `Esc` - quit
- `Ctrl+C` - also quits

## patterns explained

all patterns are **sparse** (only draw where there's action, rest is black) and react to bass/kicks with some mid/high response:

- **flash**: intense center burst on beat
- **spark**: rays exploding from center
- **scatter**: random particles popping
- **beam**: vertical scanning light beams
- **ripple**: expanding ring edges (like water)
- **laser**: thin scanning laser lines
- **orbit**: circular orbit paths
- **explosion**: expanding ring burst
- **rings**: concentric pulsing rings
- **zigzag**: lightning bolt effect
- **cross**: rotating cross beams
- **spiral**: rotating spiral arms
- **star**: star burst rays
- **tunnel**: 3d tunnel perspective
- **neurons**: neural network connections
- **fractal**: fractal branch patterns

## palettes

fast ascii characters optimized for terminal rendering:

- **default**: ` .,:;ox%#@`
- **box**: ` .o*O@`
- **lines**: ` .|/=#`
- **spark**: ` .'`:\*#`
- **retro**: ` .-=+*#@`
- **minimal**: ` .o*@`
- **block**: ` ░▒▓█`
- **bubble**: ` .oO@`

## performance

### raspberry pi 4
- **balanced quality**: 80-90 fps
- **eco quality**: 90-120 fps
- **settings**: `--quality balanced --fps 90`

### desktop (debian, ubuntu, etc)
- **balanced quality**: 300-500 fps
- **high quality**: 200-300 fps
- **settings**: `--quality high`

## optimizations

this thing is fast because:
- no rotation/zoom/swirl/warp effects
- no gamma/contrast calculations
- simplified hsv→rgb conversion
- patterns use basic math (no sin/cos/pow spam)
- fewer goroutines (less sync overhead)
- simple ascii chars (no unicode rendering cost)
- noise calculation disabled (was the bottleneck)

## installation

### debian/ubuntu/raspberry pi os

```bash
# install portaudio
sudo apt update
sudo apt install portaudio19-dev

# build
./build.sh

# run
./golizer-pi  # or ./golizer-debian
```

### dependencies script

if you're on a fresh system:

```bash
./dependencies.sh
```

installs go + portaudio if needed.

## saved versions

there's a gold version saved at `golizer-pi-gold-version` - this is the one that ran perfect on the pi before we made more changes. it's in the gitignore so it won't get committed.

to restore it:
```bash
git checkout gold-version
./build.sh
```

## development

```bash
# format code
gofmt -w ./cmd ./internal

# run tests
go test ./...

# tidy deps
go mod tidy

# build for specific arch
GOOS=linux GOARCH=arm64 go build -o golizer-pi ./cmd/visualizer
GOOS=linux GOARCH=amd64 go build -o golizer-debian ./cmd/visualizer
```

## why this exists

wanted audio visualization for my pi connected to the tv while playing music. chroma didn't work (gpu issues), and all the other terminal visualizers were either too slow or looked like crap.

so i made this. now it runs at 80+ fps with sick neon visuals that actually react to kicks and bass drops like they should.

## credits

- original inspiration: [Chroma](https://github.com/yuri-xyz/chroma) by yuri-xyz
- built with portaudio and go
- made for electronic music and trippy visuals

---

built for raspberry pi 4 and beyond. if it runs on your toaster, send me a pic.
