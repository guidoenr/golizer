# GOLIZER

this directory is literally a copy of [Chroma](https://github.com/yuri-xyz/chroma), but **CPU-based**, but... why? everything surged when i tried to run the chroma gpu-based version on my setup and i faced this [issue](https://github.com/yuri-xyz/chroma/issues/14), so this repo contains a CPU-only re-implementation of Chroma’s audio reactive pipeline using Go.  
It keeps the same DSP ideas (PortAudio capture + FFT-based analyzer + beat detection) but renders ASCII frames on the CPU instead of dispatching WGSL shaders.

## Features

- PortAudio capture with device selection and mono conversion.
- FFT analyzer mirroring the Rust logic: Hann window, bass/mid/treble energy, beat pulse, and bass-drop detection.
- ASCII renderer with ANSI color output and palettes (`default`, `box`, `lines`, `spark`, `retro`, `minimal`, `block`, `bubble`) plus color modes (`chromatic`, `fire`, `aurora`, `mono`). Audio-reactive colour is enabled by default; pass `--color-on-audio=false` to keep full colour at all times.
- Multiple CPU patterns (`plasma`, `waves`, `ripples`, `nebula`, `noise`, `bands`, `strata`, `orbits`) ranging from shader-inspired looks to lightweight options for slower CPUs.
- Parallel row renderer that fans out across CPU cores using goroutines for faster ASCII conversion.
- Full-screen alternate-buffer rendering that restores the terminal state on exit.
- CLI flags for resolution, FPS, palette, audio device, buffer size, synthetic audio mode, and audio-triggered colour bursts.
- Automatic PortAudio device discovery with `--list-audio-devices` and smart default selection for “monitor/loopback” style inputs.
- Live visual randomiser (`R`) and keyboard quit (`Q` / `Esc`) bindings.
- One-shot bootstrap script (`scripts/install_rpi.sh`) for Debian 12 / Raspberry Pi 4 environments that installs Go, PortAudio headers, and builds the binary.

## Getting Started

1. **Bootstrap on Debian/Raspberry Pi (optional)**:
   ```bash
   ./scripts/install_rpi.sh
   ```
   The script installs Go (if missing or outdated), PortAudio headers, and builds the project in-place.
1. **Install PortAudio manually** (if you prefer doing it yourself):
   ```bash
   sudo apt install portaudio19-dev
   ```
1. **Build the native binary** (recommended for fastest startup):
   ```bash
   go build -o golizer ./cmd/visualizer
   ```
1. **Run with real audio** (auto sizes to the terminal and defaults to 500 FPS):
   ```bash
   ./golizer --fps 10000
   ```
1. **List audio devices** (from a tiny helper snippet):
   ```bash
   go run ./cmd/visualizer --list-audio-devices
   ```
   This prints all inputs, highlights the system default, and shows which one the auto-detector will choose. Add `--audio-device "<name>"` to override.

1. **Synthetic mode** (no PortAudio required):
   ```bash
   go run ./cmd/visualizer --no-audio --palette box --fps 15
   ```
1. **Colour burst tied to audio**:
   ```bash
   ./golizer --color-on-audio
   ```
   Colour-on-audio is on by default; add `--color-on-audio=false` if you want constant colour.

## Runtime Controls

- `R` randomises palette, pattern and colour mode.
- `Q` or `Esc` quits cleanly (Ctrl+C still works).
- Terminal is restored automatically thanks to the alternate buffer toggle.

## Development Notes

- Run formatting and dependency resolution:
  ```bash
  gofmt -w ./cmd ./internal
  go mod tidy
  ```
- Unit tests cover the DSP helper logic:
  ```bash
  go test ./internal/analyzer ./internal/params
  ```
  Full `go test ./...` requires `portaudio-2.0` headers/libraries. If unavailable, run with synthetic mode or install the dependency first.

## Roadmap Ideas

- Add JSON/TOML configuration loading plus live reload.
- Experiment with emoji/Unicode glyph packs for ultra-high density renders.
- Capture frame dumps for exporting GIF/MP4 clips.


