//go:build demo

// Demo build for UI work and screenshots: `wails3 task build EXTRA_TAGS=demo`.
// Two fake phones, a moving level meter, and settings kept in memory only.
package main

import (
	"math"
	"os"
	"time"

	"github.com/a27259954/btaudio/internal/config"
	"github.com/a27259954/btaudio/internal/core"
)

type demoBackend struct{ start time.Time }

func newBackend(cfg *config.Config) core.Backend {
	cfg.SetAuto("demo-iphone", true)
	return &demoBackend{start: time.Now()}
}

func configPath() string   { return "" }
func langOverride() string { return os.Getenv("BTAUDIO_LANG") }

func (b *demoBackend) Run(f func())   { f() }
func (b *demoBackend) Output() string { return "Speakers (Realtek(R) Audio)" }

func (b *demoBackend) Sources() ([]core.Source, error) {
	return []core.Source{{ID: "demo-iphone", Name: "iPhone 16 Pro"}, {ID: "demo-pixel", Name: "Pixel 9"}}, nil
}

func (b *demoBackend) Open(string) (core.Conn, error) {
	time.Sleep(600 * time.Millisecond)
	return demoConn{}, nil
}

func (b *demoBackend) Meter(string) core.Meter { return demoMeter{b.start} }

type demoConn struct{}

func (demoConn) Opened() bool { return true }
func (demoConn) Close()       {}

type demoMeter struct{ start time.Time }

func (m demoMeter) Peak() float32 {
	t := time.Since(m.start).Seconds()
	return float32(0.25 + 0.2*math.Abs(math.Sin(t*3.1)*math.Cos(t*1.3)))
}
func (demoMeter) Close() {}
