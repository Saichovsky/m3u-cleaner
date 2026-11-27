package config

import (
	"os"
	"path/filepath"
	"testing"

	"m3u-cleaner/internal/iptv"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	return path
}

// Test LoadConfig reads both sections with explicit values.
func TestLoadConfig(t *testing.T) {
	path := writeConfigFile(t, `
[tv]
playlist_urls = ["https://example.com/tv.m3u"]
exclude_channels = ["bbc pashto", "  cbeebies  "]
min_resolution_height = 480
timeout_seconds = 5
concurrency = 20

[radio]
playlist_urls = ["https://example.com/radio.m3u"]
exclude_channels = ["ghett"]
timeout_seconds = 15
concurrency = 10
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if len(cfg.TV.PlaylistURLs) != 1 || cfg.TV.PlaylistURLs[0] != "https://example.com/tv.m3u" {
		t.Errorf("Unexpected TV playlist URLs: %v", cfg.TV.PlaylistURLs)
	}
	if len(cfg.TV.ExcludeChannels) != 2 || cfg.TV.ExcludeChannels[0] != "bbc pashto" || cfg.TV.ExcludeChannels[1] != "  cbeebies  " {
		t.Errorf("Unexpected TV exclude channels: %v", cfg.TV.ExcludeChannels)
	}
	if cfg.TV.MinResolutionHeight != 480 {
		t.Errorf("TV MinResolutionHeight = %d, expected 480", cfg.TV.MinResolutionHeight)
	}
	if cfg.TV.TimeoutSeconds != 5 {
		t.Errorf("TV TimeoutSeconds = %d, expected 5", cfg.TV.TimeoutSeconds)
	}
	if cfg.TV.Concurrency != 20 {
		t.Errorf("TV Concurrency = %d, expected 20", cfg.TV.Concurrency)
	}

	if len(cfg.Radio.PlaylistURLs) != 1 || cfg.Radio.PlaylistURLs[0] != "https://example.com/radio.m3u" {
		t.Errorf("Unexpected radio playlist URLs: %v", cfg.Radio.PlaylistURLs)
	}
	if len(cfg.Radio.ExcludeChannels) != 1 || cfg.Radio.ExcludeChannels[0] != "ghett" {
		t.Errorf("Unexpected radio exclude channels: %v", cfg.Radio.ExcludeChannels)
	}
	if cfg.Radio.TimeoutSeconds != 15 {
		t.Errorf("Radio TimeoutSeconds = %d, expected 15", cfg.Radio.TimeoutSeconds)
	}
	if cfg.Radio.Concurrency != 10 {
		t.Errorf("Radio Concurrency = %d, expected 10", cfg.Radio.Concurrency)
	}
}

// Test LoadConfig uses per-section defaults for unset fields.
func TestLoadConfig_Defaults(t *testing.T) {
	path := writeConfigFile(t, `
[tv]
playlist_urls = ["https://example.com/tv.m3u"]

[radio]
playlist_urls = ["https://example.com/radio.m3u"]
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.TV.MinResolutionHeight != iptv.MinResolutionHeight {
		t.Errorf("TV MinResolutionHeight = %d, expected default %d", cfg.TV.MinResolutionHeight, iptv.MinResolutionHeight)
	}
	if cfg.TV.TimeoutSeconds != iptv.TimeoutSeconds {
		t.Errorf("TV TimeoutSeconds = %d, expected default %d", cfg.TV.TimeoutSeconds, iptv.TimeoutSeconds)
	}
	if cfg.TV.Concurrency != iptv.ConcurrentChecks {
		t.Errorf("TV Concurrency = %d, expected default %d", cfg.TV.Concurrency, iptv.ConcurrentChecks)
	}
	if cfg.Radio.TimeoutSeconds != defaultRadioTimeoutSeconds {
		t.Errorf("Radio TimeoutSeconds = %d, expected default %d", cfg.Radio.TimeoutSeconds, defaultRadioTimeoutSeconds)
	}
	if cfg.Radio.Concurrency != defaultRadioConcurrency {
		t.Errorf("Radio Concurrency = %d, expected default %d", cfg.Radio.Concurrency, defaultRadioConcurrency)
	}
}

// Test LoadConfig accepts a radio-only config.
func TestLoadConfig_RadioOnly(t *testing.T) {
	path := writeConfigFile(t, `
[radio]
playlist_urls = ["https://example.com/radio.m3u"]
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if len(cfg.TV.PlaylistURLs) != 0 {
		t.Errorf("TV PlaylistURLs = %v, expected none", cfg.TV.PlaylistURLs)
	}
	if len(cfg.Radio.PlaylistURLs) != 1 || cfg.Radio.PlaylistURLs[0] != "https://example.com/radio.m3u" {
		t.Errorf("Unexpected radio playlist URLs: %v", cfg.Radio.PlaylistURLs)
	}
}

// Test LoadConfig accepts a TV-only config.
func TestLoadConfig_TVOnly(t *testing.T) {
	path := writeConfigFile(t, `
[tv]
playlist_urls = ["https://example.com/tv.m3u"]
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if len(cfg.TV.PlaylistURLs) != 1 {
		t.Errorf("Unexpected TV playlist URLs: %v", cfg.TV.PlaylistURLs)
	}
	if len(cfg.Radio.PlaylistURLs) != 0 {
		t.Errorf("Radio PlaylistURLs = %v, expected none", cfg.Radio.PlaylistURLs)
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
	path := writeConfigFile(t, "not: [valid toml")
	if _, err := LoadConfig(path); err == nil {
		t.Error("Expected error for invalid TOML, got nil")
	}
}

// Test Validate rejects a config with no playlists in either section.
func TestValidate_MissingPlaylistURLs(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for missing playlist_urls, got nil")
	}
}

// Test Validate rejects non-positive values in each section.
func TestValidate_NonPositiveValues(t *testing.T) {
	tests := []struct {
		name      string
		modifyCfg func(Config) Config
		wantErr   bool
	}{
		{
			name: "tv min_resolution_height zero",
			modifyCfg: func(c Config) Config {
				c.TV.PlaylistURLs = []string{"https://example.com/tv.m3u"}
				c.TV.MinResolutionHeight = 0
				return c
			},
			wantErr: true,
		},
		{
			name: "tv min_resolution_height negative",
			modifyCfg: func(c Config) Config {
				c.TV.PlaylistURLs = []string{"https://example.com/tv.m3u"}
				c.TV.MinResolutionHeight = -1
				return c
			},
			wantErr: true,
		},
		{
			name: "tv timeout_seconds zero",
			modifyCfg: func(c Config) Config {
				c.TV.PlaylistURLs = []string{"https://example.com/tv.m3u"}
				c.TV.TimeoutSeconds = 0
				return c
			},
			wantErr: true,
		},
		{
			name: "tv concurrency zero",
			modifyCfg: func(c Config) Config {
				c.TV.PlaylistURLs = []string{"https://example.com/tv.m3u"}
				c.TV.Concurrency = 0
				return c
			},
			wantErr: true,
		},
		{
			name: "radio timeout_seconds zero",
			modifyCfg: func(c Config) Config {
				c.Radio.PlaylistURLs = []string{"https://example.com/radio.m3u"}
				c.Radio.TimeoutSeconds = 0
				return c
			},
			wantErr: true,
		},
		{
			name: "radio concurrency zero",
			modifyCfg: func(c Config) Config {
				c.Radio.PlaylistURLs = []string{"https://example.com/radio.m3u"}
				c.Radio.Concurrency = 0
				return c
			},
			wantErr: true,
		},
		{
			name: "tv only with defaults",
			modifyCfg: func(c Config) Config {
				c.TV.PlaylistURLs = []string{"https://example.com/tv.m3u"}
				return c
			},
			wantErr: false,
		},
		{
			name: "radio only with defaults",
			modifyCfg: func(c Config) Config {
				c.Radio.PlaylistURLs = []string{"https://example.com/radio.m3u"}
				return c
			},
			wantErr: false,
		},
		{
			name: "both sections",
			modifyCfg: func(c Config) Config {
				c.TV.PlaylistURLs = []string{"https://example.com/tv.m3u"}
				c.Radio.PlaylistURLs = []string{"https://example.com/radio.m3u"}
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
