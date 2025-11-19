//go:build sdl

package render

import (
	"fmt"
	"math"
	"runtime"
	"unsafe"

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
	
	// Configurar hints de SDL para mejor rendimiento en plataformas embebidas
	if isEmbeddedPlatform() {
		// Usar rendering por software o hardware según disponibilidad
		sdl.SetHint(sdl.HINT_RENDER_DRIVER, "opengles2")
		// Mejorar el scaling en fullscreen
		sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "0")
		// Prevenir screen tearing
		sdl.SetHint(sdl.HINT_RENDER_VSYNC, "1")
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
		flags := uint32(sdl.WINDOW_SHOWN)
		
		// En Raspberry Pi, WINDOW_FULLSCREEN funciona mejor que WINDOW_FULLSCREEN_DESKTOP
		// para evitar problemas de scaling y aspect ratio
		var fullscreenMode uint32 = sdl.WINDOW_FULLSCREEN_DESKTOP
		if r.fullscreen {
			// Detectar si estamos en un entorno embebido (ARM)
			// En estos casos, usar WINDOW_FULLSCREEN es más confiable
			if isEmbeddedPlatform() {
				fullscreenMode = sdl.WINDOW_FULLSCREEN
				flags = sdl.WINDOW_FULLSCREEN
			} else {
				flags = sdl.WINDOW_FULLSCREEN_DESKTOP
			}
		}
		
		window, err := sdl.CreateWindow(
			"golizer",
			sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED,
			int32(r.width), int32(r.height),
			flags,
		)
		if err != nil {
			return err
		}
		state.window = window
		if r.fullscreen {
			_ = window.SetFullscreen(fullscreenMode)
		}
	}
	logicalW := int32(r.width)
	logicalH := int32(r.height)
	if state.renderer == nil {
		renderer, err := sdl.CreateRenderer(state.window, -1, sdl.RENDERER_ACCELERATED|sdl.RENDERER_PRESENTVSYNC)
		if err != nil {
			return err
		}
		state.renderer = renderer
		_ = renderer.SetLogicalSize(logicalW, logicalH)
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

	downsample := r.downsample
	if downsample < 1 {
		downsample = 1
	}

	for y := 0; y < height; y += downsample {
		sampleY := y + downsample/2
		if sampleY >= height {
			sampleY = height - 1
		}
		vy := yCoords[sampleY] * scale
		yEnd := y + downsample
		if yEnd > height {
			yEnd = height
		}
		for x := 0; x < width; x += downsample {
			sampleX := x + downsample/2
			if sampleX >= width {
				sampleX = width - 1
			}
			vx := xCoords[sampleX] * scale
			index := sampleY*width + sampleX
			res := r.evaluatePixel(vx, vy, p, ctx, feat, activation, noiseWarp, noiseDetail, index)
			rr, gg, bb := hsvToRGB(res.h, res.s, res.v)
			rByte := byte(clampFloat(rr*255, 0, 255))
			gByte := byte(clampFloat(gg*255, 0, 255))
			bByte := byte(clampFloat(bb*255, 0, 255))
			xEnd := x + downsample
			if xEnd > width {
				xEnd = width
			}
			for fy := y; fy < yEnd; fy++ {
				rowOffset := fy * pitch
				for fx := x; fx < xEnd; fx++ {
					offset := rowOffset + fx*4
					state.pixelBuffer[offset+0] = rByte
					state.pixelBuffer[offset+1] = gByte
					state.pixelBuffer[offset+2] = bByte
					state.pixelBuffer[offset+3] = 255
				}
			}
		}
	}

	status := r.buildStatus(feat, fps)

	return Frame{
		Status: status,
		Present: func(status string) error {
			if status != "" && status != state.windowTitle && state.window != nil {
				state.window.SetTitle(status)
				state.windowTitle = status
			}
			var pixels unsafe.Pointer
			if len(state.pixelBuffer) > 0 {
				pixels = unsafe.Pointer(&state.pixelBuffer[0])
			}
			if err := state.texture.Update(nil, pixels, state.pitch); err != nil {
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
	if !r.fullscreen {
		scale := r.scale
		if scale < 1 {
			scale = 1
		}
		width := int32(math.Max(1, float64(r.width)*scale))
		height := int32(math.Max(1, float64(r.height)*scale))
		r.sdl.window.SetSize(width, height)
	}
	_ = r.sdl.renderer.SetLogicalSize(int32(r.width), int32(r.height))
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

// isEmbeddedPlatform detecta si estamos corriendo en una plataforma embebida
// como Raspberry Pi, donde SDL_WINDOW_FULLSCREEN funciona mejor que SDL_WINDOW_FULLSCREEN_DESKTOP
func isEmbeddedPlatform() bool {
	arch := runtime.GOARCH
	// Detectar ARM/ARM64 que típicamente son Raspberry Pi u otros SBCs
	return arch == "arm" || arch == "arm64"
}

func SupportsSDL() bool { return true }
