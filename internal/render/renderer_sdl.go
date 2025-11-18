//go:build sdl

package render

import (
	"fmt"

	"github.com/guidoenr/golizer/internal/analyzer"
	"github.com/guidoenr/golizer/internal/params"
	"github.com/veandco/go-sdl2/sdl"
)

type sdlState struct {
	initialized bool
	window      *sdl.Window
	renderer    *sdl.Renderer
	texture     *sdl.Texture
	pixelBuffer []byte
	width       int
	height      int
	pitch       int
	windowTitle string
}

func (r *Renderer) initSDL(width, height int) error {
	if r.sdl != nil {
		r.mode = backendSDL
		r.useANSI = false
		return nil
	}
	if err := sdl.InitSubSystem(sdl.INIT_VIDEO); err != nil {
		return err
	}
	r.sdl = &sdlState{
		initialized: true,
	}
	r.mode = backendSDL
	r.useANSI = false
	return nil
}

func (r *Renderer) ensureSDLResources() error {
	if r.sdl == nil {
		return fmt.Errorf("SDL backend not initialized")
	}
	state := r.sdl
	if !state.initialized {
		if err := sdl.InitSubSystem(sdl.INIT_VIDEO); err != nil {
			return err
		}
		state.initialized = true
	}
	if state.window == nil {
		window, err := sdl.CreateWindow(
			"golizer",
			sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED,
			int32(r.width), int32(r.height),
			sdl.WINDOW_SHOWN,
		)
		if err != nil {
			return err
		}
		state.window = window
	}
	if state.renderer == nil {
		renderer, err := sdl.CreateRenderer(state.window, -1, sdl.RENDERER_ACCELERATED|sdl.RENDERER_PRESENTVSYNC)
		if err != nil {
			return err
		}
		state.renderer = renderer
		_ = renderer.SetLogicalSize(int32(r.width), int32(r.height))
	}
	if state.texture == nil || state.width != r.width || state.height != r.height {
		if state.texture != nil {
			state.texture.Destroy()
			state.texture = nil
		}
		tex, err := state.renderer.CreateTexture(
			sdl.PIXELFORMAT_ABGR8888,
			sdl.TEXTUREACCESS_STREAMING,
			int32(r.width), int32(r.height),
		)
		if err != nil {
			return err
		}
		state.texture = tex
		state.width = r.width
		state.height = r.height
		state.pitch = r.width * 4
		state.pixelBuffer = make([]byte, state.pitch*r.height)
	} else if len(state.pixelBuffer) != state.pitch*r.height {
		state.pixelBuffer = make([]byte, state.pitch*r.height)
	}
	return nil
}

func (r *Renderer) renderSDL(p params.Parameters, feat analyzer.Features, fps float64, ctx frameParams, activation float64, xCoords, yCoords []float64, scale float64, noiseWarp, noiseDetail []float64) Frame {
	if err := r.ensureSDLResources(); err != nil {
		return Frame{
			Status: fmt.Sprintf("SDL init error: %v", err),
			Present: func(string) error {
				return err
			},
		}
	}
	state := r.sdl
	width := r.width
	height := r.height
	pitch := state.pitch

	for y := 0; y < height; y++ {
		vy := yCoords[y] * scale
		rowOffset := y * pitch
		for x := 0; x < width; x++ {
			vx := xCoords[x] * scale
			index := y*width + x
			res := r.evaluatePixel(vx, vy, p, ctx, feat, activation, noiseWarp, noiseDetail, index)
			rr, gg, bb := hsvToRGB(res.h, res.s, res.v)
			offset := rowOffset + x*4
			state.pixelBuffer[offset+0] = byte(clampFloat(rr*255, 0, 255))
			state.pixelBuffer[offset+1] = byte(clampFloat(gg*255, 0, 255))
			state.pixelBuffer[offset+2] = byte(clampFloat(bb*255, 0, 255))
			state.pixelBuffer[offset+3] = 255
		}
	}

	status := r.buildStatus(feat, fps)

	return Frame{
		Status: status,
		Present: func(status string) error {
			if status != "" && status != state.windowTitle && state.window != nil {
				_ = state.window.SetTitle(status)
				state.windowTitle = status
			}
			if err := state.texture.Update(nil, state.pixelBuffer, state.pitch); err != nil {
				return err
			}
			if err := state.renderer.Clear(); err != nil {
				return err
			}
			if err := state.renderer.Copy(state.texture, nil, nil); err != nil {
				return err
			}
			state.renderer.Present()
			for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
				switch event.(type) {
				case *sdl.QuitEvent:
					return ErrRendererQuit
				}
			}
			return nil
		},
	}
}

func (r *Renderer) resizeSDL() {
	if r.sdl == nil {
		return
	}
	r.sdl.width = 0
	r.sdl.height = 0
}

func (r *Renderer) closeSDL() error {
	if r.sdl == nil {
		return nil
	}
	if r.sdl.texture != nil {
		r.sdl.texture.Destroy()
		r.sdl.texture = nil
	}
	if r.sdl.renderer != nil {
		r.sdl.renderer.Destroy()
		r.sdl.renderer = nil
	}
	if r.sdl.window != nil {
		r.sdl.window.Destroy()
		r.sdl.window = nil
	}
	r.sdl.pixelBuffer = nil
	if r.sdl.initialized {
		sdl.QuitSubSystem(sdl.INIT_VIDEO)
		r.sdl.initialized = false
	}
	r.sdl = nil
	return nil
}

func (r *Renderer) windowedSDL() bool {
	return r.sdl != nil
}

func SupportsSDL() bool { return true }
