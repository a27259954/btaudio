package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	c := Load(path)
	if c.Auto("phone") || c.Language() != "" {
		t.Fatal("fresh config should be empty")
	}
	c.SetAuto("phone", true)
	c.SetLanguage("ja")

	c2 := Load(path)
	if !c2.Auto("phone") || c2.Language() != "ja" {
		t.Fatalf("not persisted: auto=%v lang=%q", c2.Auto("phone"), c2.Language())
	}
}

func TestBrokenFileFallsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Load(path)
	c.SetAuto("x", true) // must not panic on a nil map
	if !c.Auto("x") {
		t.Fatal("SetAuto lost")
	}
}

func TestMemoryOnly(t *testing.T) {
	c := Load("")
	c.SetAuto("x", true)
	if !c.Auto("x") {
		t.Fatal("in-memory config should still work")
	}
}
