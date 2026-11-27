package config

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"

	"m3u-cleaner/internal/iptv"
)

const (
	defaultRadioTimeoutSeconds = 15
	defaultRadioConcurrency    = 20
)

// Section holds the settings shared by the TV and radio pipelines.
type Section struct {
	// PlaylistURLs is the list of M3U playlist URLs to fetch and process.
	PlaylistURLs []string `toml:"playlist_urls"`
	// ExcludeChannels is a list of substrings (case-insensitive) matched
	// against channel EXTINF metadata; matching channels are dropped.
	ExcludeChannels []string `toml:"exclude_channels"`
	// TimeoutSeconds is the per-request timeout for stream validation.
	TimeoutSeconds int `toml:"timeout_seconds"`
	// Concurrency is the number of parallel stream checks.
	Concurrency int `toml:"concurrency"`
}

// TVSection is the [tv] table. Resolution filtering applies only to TV.
type TVSection struct {
	Section
	// MinResolutionHeight drops channels explicitly below this height.
	MinResolutionHeight int `toml:"min_resolution_height"`
}

// Config holds the two independently tuned playlist sections.
// Optional fields fall back to their default value via DefaultConfig.
type Config struct {
	TV    TVSection `toml:"tv"`
	Radio Section   `toml:"radio"`
}

// DefaultConfig returns a Config populated with the per-section defaults for
// optional fields. PlaylistURLs is not defaulted — it must be provided.
func DefaultConfig() Config {
	return Config{
		TV: TVSection{
			Section: Section{
				TimeoutSeconds: iptv.TimeoutSeconds,
				Concurrency:    iptv.ConcurrentChecks,
			},
			MinResolutionHeight: iptv.MinResolutionHeight,
		},
		Radio: Section{
			TimeoutSeconds: defaultRadioTimeoutSeconds,
			Concurrency:    defaultRadioConcurrency,
		},
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
	if len(c.TV.PlaylistURLs) == 0 && len(c.Radio.PlaylistURLs) == 0 {
		errs = append(errs, "at least one of tv.playlist_urls or radio.playlist_urls must contain a URL")
	}
	if len(c.TV.PlaylistURLs) > 0 {
		if c.TV.MinResolutionHeight <= 0 {
			errs = append(errs, "tv.min_resolution_height must be positive")
		}
		if c.TV.TimeoutSeconds <= 0 {
			errs = append(errs, "tv.timeout_seconds must be positive")
		}
		if c.TV.Concurrency <= 0 {
			errs = append(errs, "tv.concurrency must be positive")
		}
	}
	if len(c.Radio.PlaylistURLs) > 0 {
		if c.Radio.TimeoutSeconds <= 0 {
			errs = append(errs, "radio.timeout_seconds must be positive")
		}
		if c.Radio.Concurrency <= 0 {
			errs = append(errs, "radio.concurrency must be positive")
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
