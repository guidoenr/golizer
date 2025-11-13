# GOLIZER

This directory contains a CPU-only re-implementation of Chroma’s audio reactive pipeline using Go.  
It keeps the same DSP ideas (PortAudio capture + FFT-based analyzer + beat detection) but renders ASCII frames on the CPU instead of dispatching WGSL shaders.

## Features

- PortAudio capture with device selection and mono conversion.
- FFT analyzer mirroring the Rust logic: Hann window, bass/mid/treble energy, beat pulse, and bass-drop detection.
- ASCII renderer with ANSI color output and palettes (`default`, `box`, `lines`, `spark`) plus color modes (`chromatic`, `fire`, `aurora`, `mono`). When `--color-on-audio` is enabled the scene stays dark/monochrome until energy is detected.
- Multiple CPU patterns (`plasma`, `waves`, `ripples`, `nebula`, `noise`) that roughly mirror the GPU shader presets.
- Full-screen alternate-buffer rendering that restores the terminal state on exit.
- CLI flags for resolution, FPS, palette, audio device, buffer size, synthetic audio mode, and audio-triggered colour bursts.
- Automatic PortAudio device discovery with `--list-audio-devices` and smart default selection for “monitor/loopback” style inputs.
- Live visual randomiser (`R`) and keyboard quit (`Q` / `Esc`) bindings.

## Getting Started

1. **Install PortAudio** (required for real audio capture):
   ```bash
   sudo apt install portaudio19-dev
   ```
1. **Build the native binary** (recommended for fastest startup):
   ```bash
   go build -o golizer ./cmd/visualizer
   ```
1. **Run with real audio** (auto sizes to the terminal and defaults to 60 FPS):
   ```bash
   ./golizer --fps 60
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
   When the flag is enabled the scene stays monochrome until the analyser detects energy (great for “dark until I speak” setups).

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

- Implement more palettes and CPU-friendly pattern generators.
- Add JSON/TOML configuration loading plus live reload.
- Explore Go routine parallelism (rayon equivalent) for faster ASCII conversion.


