package audio

import (
	"sync"

	"github.com/gordonklaus/portaudio"
)

var (
	initOnce sync.Once
	termOnce sync.Once
	initErr  error
)

// Initialize wraps portaudio.Initialize with sync.Once so multiple callers are safe.
func Initialize() error {
	initOnce.Do(func() {
		initErr = portaudio.Initialize()
	})
	return initErr
}

// Terminate wraps portaudio.Terminate with sync.Once to balance Initialize.
func Terminate() {
	if initErr != nil {
		return
	}
	termOnce.Do(func() {
		_ = portaudio.Terminate()
	})
}
