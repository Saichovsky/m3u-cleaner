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
playlist_urls = ["https://example.com/playlist1.m3u"]
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

	if len(cfg.PlaylistURLs) != 1 || cfg.PlaylistURLs[0] != "https://example.com/playlist1.m3u" {
		t.Errorf("Unexpected playlist URLs: %v", cfg.PlaylistURLs)
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

// Test LoadConfig uses defaults for unset fields
func TestLoadConfig_Defaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := `
playlist_urls = ["https://example.com/test.m3u"]
min_resolution_height = 480
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
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
	if len(cfg.PlaylistURLs) != 1 || cfg.PlaylistURLs[0] != "https://example.com/test.m3u" {
		t.Errorf("PlaylistURLs = %v, expected [https://example.com/test.m3u]", cfg.PlaylistURLs)
	}
}

// Test LoadConfig returns an error for an empty path (config is mandatory)
func TestLoadConfig_EmptyPathRequired(t *testing.T) {
	_, err := LoadConfig("")
	if err == nil {
		t.Error("Expected error for empty config path, got nil")
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

// Test Validate rejects empty playlist_urls
func TestValidate_MissingPlaylistURLs(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for missing playlist_urls, got nil")
	}
}

// Test Validate rejects non-positive values
func TestValidate_NonPositiveValues(t *testing.T) {
	tests := []struct {
		name      string
		modifyCfg func(Config) Config
		wantErr   bool
	}{
		{
			name: "min_resolution_height zero",
			modifyCfg: func(c Config) Config {
				c.PlaylistURLs = []string{"https://example.com/play.m3u"}
				c.MinResolutionHeight = 0
				return c
			},
			wantErr: true,
		},
		{
			name: "min_resolution_height negative",
			modifyCfg: func(c Config) Config {
				c.PlaylistURLs = []string{"https://example.com/play.m3u"}
				c.MinResolutionHeight = -1
				return c
			},
			wantErr: true,
		},
		{
			name: "timeout_seconds zero",
			modifyCfg: func(c Config) Config {
				c.PlaylistURLs = []string{"https://example.com/play.m3u"}
				c.TimeoutSeconds = 0
				return c
			},
			wantErr: true,
		},
		{
			name: "concurrency zero",
			modifyCfg: func(c Config) Config {
				c.PlaylistURLs = []string{"https://example.com/play.m3u"}
				c.Concurrency = 0
				return c
			},
			wantErr: true,
		},
		{
			name: "valid config with defaults",
			modifyCfg: func(c Config) Config {
				c.PlaylistURLs = []string{"https://example.com/play.m3u"}
				return c
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg = tt.modifyCfg(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
