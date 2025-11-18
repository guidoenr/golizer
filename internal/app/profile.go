package app

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type profiler struct {
	mu      sync.Mutex
	file    *os.File
	logger  *log.Logger
	start   time.Time
	last    time.Time
	enabled bool
}

func newProfiler(path string, logger *log.Logger) *profiler {
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		if logger != nil {
			logger.Printf("profiler disabled: %v", err)
		}
		return nil
	}
	p := &profiler{
		file:    f,
		logger:  logger,
		enabled: true,
	}
	p.writeHeader()
	return p
}

func (p *profiler) writeHeader() {
	if p == nil || !p.enabled {
		return
	}
	fmt.Fprintln(p.file, "timestamp,section,delta_ms")
}

func (p *profiler) beginFrame() {
	if p == nil || !p.enabled {
		return
	}
	now := time.Now()
	p.start = now
	p.last = now
	p.log("frame_start", 0)
}

func (p *profiler) markSection(name string) {
	if p == nil || !p.enabled {
		return
	}
	now := time.Now()
	delta := now.Sub(p.last).Seconds() * 1000
	p.last = now
	p.log(name, delta)
}

func (p *profiler) endFrame() {
	if p == nil || !p.enabled {
		return
	}
	total := time.Since(p.start).Seconds() * 1000
	p.log("frame_total", total)
}

func (p *profiler) Close() error {
	if p == nil || !p.enabled {
		return nil
	}
	return p.file.Close()
}

func (p *profiler) log(section string, deltaMs float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.file == nil {
		return
	}
	timestamp := time.Now().Format(time.RFC3339Nano)
	fmt.Fprintf(p.file, "%s,%s,%.3f\n", timestamp, section, deltaMs)
}
