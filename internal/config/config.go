package config

import (
	"github.com/BurntSushi/toml"

	"m3u-cleaner/internal/iptv"
)

// Config holds tunable runtime settings loaded from a TOML file.
// A missing field falls back to its default value.
type Config struct {
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

// DefaultConfig returns a Config populated with the package defaults, keeping
// the config and domain packages in sync from a single source of truth.
func DefaultConfig() Config {
	return Config{
		MinResolutionHeight: iptv.MinResolutionHeight,
		TimeoutSeconds:      iptv.TimeoutSeconds,
		Concurrency:         iptv.ConcurrentChecks,
	}
}

// LoadConfig decodes a TOML file at path, falling back to defaults for
// unset fields. An empty path returns the default configuration.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
