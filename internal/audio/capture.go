package audio

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/gordonklaus/portaudio"
)

// Capture wraps a PortAudio input stream and exposes thread-safe access to the latest samples.
type Capture struct {
	stream     *portaudio.Stream
	sampleRate float64
	channels   int
	device     *portaudio.DeviceInfo

	mu     sync.RWMutex
	buffer []float32
	index  int
}

// Config controls how a Capture instance is created.
type Config struct {
	DeviceName string
	BufferSize int
	Channels   int
}

const defaultBufferSize = 4096

// NewCapture opens a PortAudio stream using the provided configuration.
func NewCapture(cfg Config) (*Capture, error) {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = defaultBufferSize
	}
	if cfg.Channels <= 0 {
		cfg.Channels = 1
	}

	device, err := findDevice(cfg.DeviceName)
	if err != nil {
		return nil, err
	}

	inParams := portaudio.StreamDeviceParameters{
		Device:   device,
		Channels: cfg.Channels,
		Latency:  device.DefaultLowInputLatency,
	}

	sampleRate := device.DefaultSampleRate

	capture := &Capture{
		sampleRate: sampleRate,
		buffer:     make([]float32, cfg.BufferSize),
		channels:   cfg.Channels,
		device:     device,
	}

	framesPerBuffer := len(capture.buffer) / cfg.Channels
	if framesPerBuffer < 64 {
		framesPerBuffer = portaudio.FramesPerBufferUnspecified
	}

	stream, err := portaudio.OpenStream(portaudio.StreamParameters{
		Input:           inParams,
		Output:          portaudio.StreamDeviceParameters{},
		SampleRate:      sampleRate,
		FramesPerBuffer: framesPerBuffer,
	}, capture.process)
	if err != nil {
		return nil, fmt.Errorf("open stream: %w", err)
	}

	capture.stream = stream

	if err := capture.stream.Start(); err != nil {
		_ = capture.stream.Close()
		return nil, fmt.Errorf("start stream: %w", err)
	}

	return capture, nil
}

// Close stops and closes the underlying PortAudio stream.
func (c *Capture) Close() error {
	if c.stream == nil {
		return nil
	}
	if err := c.stream.Stop(); err != nil && !errorsIsInvalidStreamState(err) {
		return err
	}
	return c.stream.Close()
}

// SampleRate returns the stream sample rate.
func (c *Capture) SampleRate() float64 {
	return c.sampleRate
}

// Device returns the PortAudio device associated with the capture stream.
func (c *Capture) Device() *portaudio.DeviceInfo {
	return c.device
}

// Samples returns the most recent samples copied out of the internal ring buffer.
func (c *Capture) Samples() []float32 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.index == 0 {
		cp := make([]float32, len(c.buffer))
		copy(cp, c.buffer)
		return cp
	}

	cp := make([]float32, len(c.buffer))
	copy(cp, c.buffer[c.index:])
	copy(cp[len(c.buffer)-c.index:], c.buffer[:c.index])
	return cp
}

func (c *Capture) process(in []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channels > 1 {
		mono := make([]float32, len(in)/c.channels)
		for i := range mono {
			sum := float32(0)
			base := i * c.channels
			for ch := 0; ch < c.channels; ch++ {
				sum += in[base+ch]
			}
			mono[i] = sum / float32(c.channels)
		}
		c.mixIntoBuffer(mono)
		return
	}

	c.mixIntoBuffer(in)
}

func (c *Capture) mixIntoBuffer(in []float32) {
	if len(in) == 0 {
		return
	}

	if len(in) >= len(c.buffer) {
		copy(c.buffer, in[len(in)-len(c.buffer):])
		c.index = 0
		return
	}

	if c.index+len(in) <= len(c.buffer) {
		copy(c.buffer[c.index:], in)
		c.index += len(in)
		if c.index == len(c.buffer) {
			c.index = 0
		}
		return
	}

	remaining := len(c.buffer) - c.index
	copy(c.buffer[c.index:], in[:remaining])
	copy(c.buffer, in[remaining:])
	c.index = len(in) - remaining
}

func findDevice(name string) (*portaudio.DeviceInfo, error) {
	if name != "" {
		return findDeviceByName(name)
	}

	if dev, err := portaudio.DefaultInputDevice(); err == nil && dev != nil && dev.MaxInputChannels > 0 {
		return dev, nil
	}

	if host, err := portaudio.DefaultHostApi(); err == nil {
		if host != nil && host.DefaultInputDevice != nil && host.DefaultInputDevice.MaxInputChannels > 0 {
			return host.DefaultInputDevice, nil
		}
	}

	devices, err := portaudio.Devices()
	if err != nil {
		return nil, fmt.Errorf("list audio devices: %w", err)
	}

	candidate := pickBestDevice(devices)
	if candidate != nil {
		return candidate, nil
	}

	return nil, fmt.Errorf("no suitable audio input device found")
}

func findDeviceByName(name string) (*portaudio.DeviceInfo, error) {
	devices, err := portaudio.Devices()
	if err != nil {
		return nil, fmt.Errorf("list audio devices: %w", err)
	}

	name = strings.ToLower(name)
	for _, device := range devices {
		if device.MaxInputChannels == 0 {
			continue
		}
		deviceName := strings.ToLower(device.Name)
		if strings.Contains(deviceName, name) {
			return device, nil
		}
	}

	return nil, fmt.Errorf("audio device %q not found", name)
}

func pickBestDevice(devices []*portaudio.DeviceInfo) *portaudio.DeviceInfo {
	type scored struct {
		dev   *portaudio.DeviceInfo
		score int
	}

	var (
		results  []scored
		keywords = []string{"monitor", "loopback", "mix", "stereo mix", "what u hear"}
	)

	var defaultInputIndex = -1
	if def, err := portaudio.DefaultInputDevice(); err == nil && def != nil {
		defaultInputIndex = def.Index
	}

	var defaultHostIndex = -1
	if host, err := portaudio.DefaultHostApi(); err == nil && host != nil && host.DefaultInputDevice != nil {
		defaultHostIndex = host.DefaultInputDevice.Index
	}

	for _, d := range devices {
		if d == nil || d.MaxInputChannels <= 0 {
			continue
		}

		score := d.MaxInputChannels

		if d.Index == defaultInputIndex {
			score += 50
		}
		if d.Index == defaultHostIndex {
			score += 40
		}

		lower := strings.ToLower(d.Name)
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				score += 20
				break
			}
		}

		if strings.Contains(lower, "default") {
			score += 10
		}

		results = append(results, scored{dev: d, score: score})
	}

	if len(results) == 0 {
		return nil
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].score == results[j].score {
			return strings.ToLower(results[i].dev.Name) < strings.ToLower(results[j].dev.Name)
		}
		return results[i].score > results[j].score
	})

	return results[0].dev
}

// errorsIsInvalidStreamState checks if the provided error stems from stopping an already stopped stream.
func errorsIsInvalidStreamState(err error) bool {
	if err == nil {
		return false
	}
	const invalidStateMsg = "PaErrorCode -9986"
	return strings.Contains(err.Error(), invalidStateMsg)
}

// AutoDetectDevice returns the best available input device PortAudio can find.
func AutoDetectDevice() (*portaudio.DeviceInfo, error) {
	return findDevice("")
}
