package config

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"

	"m3u-cleaner/internal/iptv"
)

// Config holds tunable runtime settings loaded from a TOML file.
// Optional fields fall back to their default value via DefaultConfig.
type Config struct {
	// PlaylistURLs is the list of M3U playlist URLs to fetch and process.
	PlaylistURLs []string `toml:"playlist_urls"`
	// ExcludeChannels is a list of substrings (case-insensitive) matched
	// against channel EXTINF metadata; matching channels are dropped.
	ExcludeChannels []string `toml:"exclude_channels"`
	// MinResolutionHeight drops channels explicitly below this height.
	MinResolutionHeight int `toml:"min_resolution_height"`
	// TimeoutSeconds is the per-request timeout for stream validation.
	TimeoutSeconds int `toml:"timeout_seconds"`
	// Concurrency is the number of parallel stream checks.
	Concurrency int `toml:"concurrency"`
}

// DefaultConfig returns a Config populated with the package defaults for
// optional fields. PlaylistURLs is not defaulted — it must be provided.
func DefaultConfig() Config {
	return Config{
		MinResolutionHeight: iptv.MinResolutionHeight,
		TimeoutSeconds:      iptv.TimeoutSeconds,
		Concurrency:         iptv.ConcurrentChecks,
	}
}

// LoadConfig decodes a TOML file at path and validates it.
// The config file is mandatory: an empty path returns an error.
func LoadConfig(path string) (Config, error) {
	if path == "" {
		return Config{}, fmt.Errorf("config file path is required (use --config or set CONFIG env var)")
	}
	cfg := DefaultConfig()
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Validate checks that all required config fields are present and valid.
// Returns an error listing all failures, or nil if the config is valid.
func (c Config) Validate() error {
	var errs []string
	if len(c.PlaylistURLs) == 0 {
		errs = append(errs, "playlist_urls must contain at least one URL")
	}
	if c.MinResolutionHeight <= 0 {
		errs = append(errs, "min_resolution_height must be positive")
	}
	if c.TimeoutSeconds <= 0 {
		errs = append(errs, "timeout_seconds must be positive")
	}
	if c.Concurrency <= 0 {
		errs = append(errs, "concurrency must be positive")
	}
	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
