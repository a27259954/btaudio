// Package config persists user settings as JSON.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config is safe for concurrent use. Every setter saves immediately.
type Config struct {
	mu   sync.Mutex
	path string
	data data
}

type data struct {
	AutoConnect map[string]bool `json:"autoConnect"`
	Language    string          `json:"language,omitempty"` // "" follows Windows
}

// DefaultPath is %APPDATA%\BtAudio\config.json.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "BtAudio", "config.json")
}

// Load reads path; a missing or broken file yields defaults. An empty path
// keeps everything in memory.
func Load(path string) *Config {
	c := &Config{path: path}
	if path != "" {
		if b, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(b, &c.data)
		}
	}
	if c.data.AutoConnect == nil {
		c.data.AutoConnect = map[string]bool{}
	}
	return c
}

func (c *Config) save() error {
	if c.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(c.path, b, 0o644)
}

// Auto reports whether the device should be connected automatically.
func (c *Config) Auto(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data.AutoConnect[id]
}

// SetAuto stores the auto-connect choice for a device.
func (c *Config) SetAuto(id string, on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data.AutoConnect[id] == on {
		return
	}
	c.data.AutoConnect[id] = on
	_ = c.save()
}

// Language returns the configured UI language ("" = follow Windows).
func (c *Config) Language() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data.Language
}

// SetLanguage stores the UI language ("" = follow Windows).
func (c *Config) SetLanguage(l string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data.Language = l
	_ = c.save()
}
