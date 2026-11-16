//go:build !demo

package main

import (
	"os"
	"time"

	"github.com/a27259954/btaudio/internal/audio"
	"github.com/a27259954/btaudio/internal/config"
	"github.com/a27259954/btaudio/internal/core"
)

// winBackend connects core to the Windows audio stack.
type winBackend struct{}

func newBackend(*config.Config) core.Backend { return winBackend{} }
func configPath() string                     { return config.DefaultPath() }

// langOverride lets BTAUDIO_LANG force the UI language (useful for testing).
func langOverride() string { return os.Getenv("BTAUDIO_LANG") }

func (winBackend) Run(f func())   { audio.Run(f) }
func (winBackend) Output() string { return audio.DefaultOutput() }

func (winBackend) Sources() ([]core.Source, error) {
	srcs, err := audio.Sources()
	out := make([]core.Source, len(srcs))
	for i, s := range srcs {
		out[i] = core.Source{ID: s.ID, Name: s.Name}
	}
	return out, err
}

func (winBackend) Open(id string) (core.Conn, error) {
	c, err := audio.Open(id, 20*time.Second)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (winBackend) Meter(phone string) core.Meter {
	if m := audio.FindMeter(phone); m != nil {
		return m
	}
	return nil
}
