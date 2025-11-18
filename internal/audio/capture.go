package audio

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
const (
	activityProbeDuration   = 850 * time.Millisecond
	activitySilenceThresh   = 8e-5
	maxActivityProbeDevices = 5
)

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
	return c.SamplesInto(nil)
}

// SamplesInto copies the most recent samples into dst, reusing the slice when possible.
func (c *Capture) SamplesInto(dst []float32) []float32 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	size := len(c.buffer)
	if cap(dst) < size {
		dst = make([]float32, size)
	} else {
		dst = dst[:size]
	}

	if c.index == 0 {
		copy(dst, c.buffer)
		return dst
	}

	copy(dst, c.buffer[c.index:])
	copy(dst[size-c.index:], c.buffer[:c.index])
	return dst
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

	devices, err := portaudio.Devices()
	if err != nil {
		return nil, fmt.Errorf("list audio devices: %w", err)
	}

	candidates := make([]*portaudio.DeviceInfo, 0, len(devices))

	appendUnique := func(dev *portaudio.DeviceInfo) {
		if dev == nil || dev.MaxInputChannels <= 0 {
			return
		}
		for _, existing := range candidates {
			if existing == dev || existing.Index == dev.Index {
				return
			}
		}
		candidates = append(candidates, dev)
	}

	if dev, err := portaudio.DefaultInputDevice(); err == nil && dev != nil {
		appendUnique(dev)
	}

	if host, err := portaudio.DefaultHostApi(); err == nil && host != nil && host.DefaultInputDevice != nil {
		appendUnique(host.DefaultInputDevice)
	}

	for _, dev := range rankDevices(devices) {
		appendUnique(dev)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable audio input device found")
	}

	if active := selectActiveDevice(candidates); active != nil {
		return active, nil
	}

	return candidates[0], nil
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

func rankDevices(devices []*portaudio.DeviceInfo) []*portaudio.DeviceInfo {
	type scored struct {
		dev   *portaudio.DeviceInfo
		score int
	}

	results := make([]scored, 0, len(devices))
	keywords := []string{"monitor", "loopback", "mix", "stereo mix", "what u hear"}

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

	sort.Slice(results, func(i, j int) bool {
		if results[i].score == results[j].score {
			return strings.ToLower(results[i].dev.Name) < strings.ToLower(results[j].dev.Name)
		}
		return results[i].score > results[j].score
	})

	ranked := make([]*portaudio.DeviceInfo, 0, len(results))
	for _, r := range results {
		ranked = append(ranked, r.dev)
	}
	return ranked
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

func selectActiveDevice(candidates []*portaudio.DeviceInfo) *portaudio.DeviceInfo {
	probes := 0
	for _, dev := range candidates {
		if dev == nil || dev.MaxInputChannels <= 0 {
			continue
		}
		active, err := probeDeviceActivity(dev)
		if err != nil {
			continue
		}
		if active {
			return dev
		}
		probes++
		if probes >= maxActivityProbeDevices {
			break
		}
	}
	return nil
}

func probeDeviceActivity(dev *portaudio.DeviceInfo) (bool, error) {
	sampleRate := dev.DefaultSampleRate
	if sampleRate <= 0 {
		sampleRate = 44100
	}

	var hasSignal atomic.Bool
	callback := func(in []float32) {
		if hasSignal.Load() {
			return
		}
		for _, sample := range in {
			if math.Abs(float64(sample)) >= activitySilenceThresh {
				hasSignal.Store(true)
				return
			}
		}
	}

	stream, err := portaudio.OpenStream(portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   dev,
			Channels: 1,
			Latency:  dev.DefaultLowInputLatency,
		},
		Output:          portaudio.StreamDeviceParameters{},
		SampleRate:      sampleRate,
		FramesPerBuffer: 256,
	}, callback)
	if err != nil {
		return false, err
	}
	defer stream.Close()

	if err := stream.Start(); err != nil {
		return false, err
	}

	time.Sleep(activityProbeDuration)

	if err := stream.Stop(); err != nil && !errorsIsInvalidStreamState(err) {
		return false, err
	}

	return hasSignal.Load(), nil
}
