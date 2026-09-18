package config

import (
	"os"
	"path/filepath"
	"testing"

	"m3u-cleaner/internal/iptv"
)

// Test LoadConfig with a valid TOML file
func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := `
exclude_channels = ["bbc pashto", "  cbeebies  "]
min_resolution_height = 480
timeout_seconds = 5
concurrency = 20
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if len(cfg.ExcludeChannels) != 2 || cfg.ExcludeChannels[0] != "bbc pashto" || cfg.ExcludeChannels[1] != "  cbeebies  " {
		t.Errorf("Unexpected exclude channels: %v", cfg.ExcludeChannels)
	}
	if cfg.MinResolutionHeight != 480 {
		t.Errorf("MinResolutionHeight = %d, expected 480", cfg.MinResolutionHeight)
	}
	if cfg.TimeoutSeconds != 5 {
		t.Errorf("TimeoutSeconds = %d, expected 5", cfg.TimeoutSeconds)
	}
	if cfg.Concurrency != 20 {
		t.Errorf("Concurrency = %d, expected 20", cfg.Concurrency)
	}
}

// Test LoadConfig uses defaults for unset fields and for an empty path
func TestLoadConfig_Defaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("min_resolution_height = 480\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.MinResolutionHeight != 480 {
		t.Errorf("MinResolutionHeight = %d, expected 480", cfg.MinResolutionHeight)
	}
	if cfg.TimeoutSeconds != iptv.TimeoutSeconds {
		t.Errorf("TimeoutSeconds = %d, expected default %d", cfg.TimeoutSeconds, iptv.TimeoutSeconds)
	}
	if cfg.Concurrency != iptv.ConcurrentChecks {
		t.Errorf("Concurrency = %d, expected default %d", cfg.Concurrency, iptv.ConcurrentChecks)
	}

	def, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig(\"\") returned error: %v", err)
	}
	if def.MinResolutionHeight != iptv.MinResolutionHeight || def.TimeoutSeconds != iptv.TimeoutSeconds || def.Concurrency != iptv.ConcurrentChecks {
		t.Errorf("Default config does not match domain constants: %+v", def)
	}
}

// Test LoadConfig returns an error for an invalid TOML file
func TestLoadConfig_Invalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("not: [valid toml"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Error("Expected error for invalid TOML, got nil")
	}
}
