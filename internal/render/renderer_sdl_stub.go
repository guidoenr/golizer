//go:build !sdl

package render

import (
	"errors"

	"github.com/guidoenr/golizer/internal/analyzer"
	"github.com/guidoenr/golizer/internal/params"
)

type sdlState struct{}

func (r *Renderer) initSDL(width, height int) error {
	return errors.New("SDL backend not enabled; rebuild with -tags sdl")
}

func (r *Renderer) renderSDL(p params.Parameters, feat analyzer.Features, fps float64, ctx frameParams, activation float64, xCoords, yCoords []float64, scale float64, noiseWarp, noiseDetail []float64) Frame {
	return Frame{
		Status: "SDL backend unavailable (build without -tags sdl)",
		Present: func(string) error {
			return ErrRendererQuit
		},
	}
}

func (r *Renderer) resizeSDL() {}

func (r *Renderer) closeSDL() error { return nil }

func (r *Renderer) windowedSDL() bool { return false }

func SupportsSDL() bool { return false }
