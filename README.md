# golizer

real-time audio visualizer for the terminal. optimized for raspberry pi 4 and debian, runs smooth at 80+ fps with neon colors and minimal patterns.
this exists because of [this](https://github.com/yuri-xyz/chroma/issues/14)

## what is this

cpu-based audio reactive visualizer written in go. started as a port of [Chroma](https://github.com/yuri-xyz/chroma) but ended up being its own thing - faster, lighter, and way more trippy for electronic music.

captures audio via portaudio, does fft analysis for bass/mid/treble, detects kicks and drops, then renders sick ascii visuals that react to the music in real-time.

## features

- **audio reactive**: responds to kicks, snares, and low-end frequencies (not that crazy high-end stuff)
- **neon colors only**: red, cyan, blue, violet, pink. always saturated, never gray
- **14 minimal patterns**: dots, flash, grid, spark, pulse, scatter, beam, ripple, strobe, particle, laser, waves, orbit, explosion
- **optimized af**: 80-120 fps on raspberry pi 4, 300+ fps on desktop
- **auto-randomize**: patterns change every 10 seconds (configurable)
- **quality presets**: eco/balanced/high - auto-detects your platform
- **simple ascii**: fast characters (.,:;ox%#@) instead of slow unicode
- **black by default**: screen stays black until audio kicks in, then it explodes

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
--pattern auto                 # auto|dots|flash|grid|spark|pulse|scatter|beam|ripple|strobe|particle|laser|waves|orbit|explosion
--color-mode chromatic         # chromatic|fire|aurora|mono
--color-on-audio               # fade from black to neon based on audio

# randomization
--auto-randomize               # enable auto pattern switching
--randomize-interval 10s       # how often to randomize

# display
--status                       # show status bar
--no-color                     # disable ansi colors
--fullscreen                   # sdl fullscreen mode

# debug
--debug                        # verbose logging
--profile-log path.csv         # frame timing metrics
```

## keyboard controls

- `R` - randomize pattern/palette/colors
- `Q` or `Esc` - quit
- `Ctrl+C` - also quits

## patterns explained

all patterns react to bass/kicks, not treble (keeps it from going crazy):

- **dots**: random dots that pop with the beat
- **flash**: intense bursts from center
- **grid**: minimal grid lines
- **spark**: rays exploding outward
- **pulse**: concentric rings
- **scatter**: particles everywhere
- **beam**: vertical light beams
- **ripple**: water-like waves
- **strobe**: on/off strobing
- **particle**: moving particle system
- **laser**: scanning laser lines
- **waves**: diagonal waves
- **orbit**: circular orbits
- **explosion**: expanding from center

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
