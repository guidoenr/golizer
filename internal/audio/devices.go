package audio

import (
	"fmt"
	"sort"

	"github.com/gordonklaus/portaudio"
)

// Device describes a PortAudio device in a Go-friendly way.
type Device struct {
	Name            string
	MaxInput        int
	MaxOutput       int
	DefaultSampleHz float64
	HostAPI         string
	IsDefaultInput  bool
	IsDefaultOutput bool
}

// ListDevices returns all available devices across host APIs sorted by host and name.
func ListDevices() ([]Device, error) {
	hosts, err := portaudio.HostApis()
	if err != nil {
		return nil, fmt.Errorf("host apis: %w", err)
	}

	var defaultInputIndex = -1
	if def, err := portaudio.DefaultInputDevice(); err == nil && def != nil {
		defaultInputIndex = def.Index
	}

	devices := make([]Device, 0, len(hosts)*4)
	for _, host := range hosts {
		for _, d := range host.Devices {
			devices = append(devices, Device{
				Name:            d.Name,
				MaxInput:        d.MaxInputChannels,
				MaxOutput:       d.MaxOutputChannels,
				DefaultSampleHz: d.DefaultSampleRate,
				HostAPI:         host.Name,
				IsDefaultInput:  d.Index == defaultInputIndex,
				IsDefaultOutput: host.DefaultOutputDevice != nil && d.Index == host.DefaultOutputDevice.Index,
			})
		}
	}

	sort.Slice(devices, func(i, j int) bool {
		if devices[i].HostAPI == devices[j].HostAPI {
			return devices[i].Name < devices[j].Name
		}
		return devices[i].HostAPI < devices[j].HostAPI
	})

	return devices, nil
}
