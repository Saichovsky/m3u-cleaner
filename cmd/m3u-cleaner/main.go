package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"m3u-cleaner/internal/config"
	"m3u-cleaner/internal/iptv"
	"m3u-cleaner/internal/playlist"
)

func main() {
	startTime := time.Now()

	// 1. Parse CLI flags and Environment Variables
	countriesFlag := flag.String("countries", "", "Comma-separated list of country codes (e.g. ke,uk,us)")
	outDirFlag := flag.String("outdir", "", "Target directory for the generated index.m3u")
	configFlag := flag.String("config", "", "Path to a TOML config file")
	flag.Parse()

	// Resolve config path: Flag -> ENV -> empty (defaults)
	configPath := *configFlag
	if configPath == "" {
		configPath = os.Getenv("CONFIG")
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if configPath != "" {
		log.Printf("Loaded config from %s", configPath)
	}

	// Resolve target directory: Flag -> ENV -> Default
	listDir := *outDirFlag
	if listDir == "" {
		listDir = os.Getenv("LISTDIR")
	}
	if listDir == "" {
		listDir = "/usr/share/nginx/html"
	}

	// Resolve countries list: Flag -> ENV -> Default
	countriesInput := *countriesFlag
	if countriesInput == "" {
		countriesInput = os.Getenv("COUNTRIES")
	}
	if countriesInput == "" {
		countriesInput = "ke,uk,us" // Default fallback
	}

	countries := parseCountries(countriesInput)

	log.SetFlags(log.Ldate | log.Ltime)
	log.Printf("Starting IPTV Playlist Pipeline for countries: %v", countries)

	// 2. Fetch Playlists
	client := &http.Client{Timeout: iptv.FetchTimeoutSeconds * time.Second}
	var channels []iptv.Channel
	var processedCountries []string

	for _, country := range countries {
		url := fmt.Sprintf("https://iptv-org.github.io/iptv/countries/%s.m3u", country)
		log.Printf("Fetching: %s", url)

		parsed, err := iptv.FetchAndParseM3U(client, url)
		if err != nil {
			log.Printf("Warning: Error processing country '%s': %v", country, err)
			continue
		}
		channels = append(channels, parsed...)
		processedCountries = append(processedCountries, country)
		log.Printf("Fetched country '%s': %d channels (cumulative: %d)", country, len(parsed), len(channels))
	}

	log.Printf("Total entries fetched: %d", len(channels))
	if len(channels) == 0 {
		log.Fatal("No valid channels retrieved from given countries.")
	}

	// 3. Filter Out Low-Resolution Channels (< configured height)
	channels = iptv.FilterLowRes(channels, cfg.MinResolutionHeight)
	log.Printf("Total entries after low-res filter: %d", len(channels))

	// 3.5 Exclude Channels by Config Patterns
	channels = iptv.FilterExcluded(channels, cfg.ExcludeChannels)
	log.Printf("Total entries after exclude filter:  %d", len(channels))

	// 4. Concurrently Validate Stream URLs
	progress := iptv.NewProgressReporter()
	validChannels := iptv.ValidateChannelsN(channels, time.Duration(cfg.TimeoutSeconds)*time.Second, cfg.Concurrency, func(done, total, valid int) {
		progress.Report("Validation", done, total, valid)
	})
	log.Printf("Validation complete: %d valid out of %d channels", len(validChannels), len(channels))

	// 5. Write Output Playlist
	outputPath := filepath.Join(listDir, playlist.OutputFile)
	if err := playlist.WritePlaylist(outputPath, validChannels); err != nil {
		log.Fatalf("Failed to write output playlist: %v", err)
	}

	duration := time.Since(startTime)
	log.Println("--- SUMMARY ---")
	log.Printf("Countries processed: %v", processedCountries)
	log.Printf("Total entries fetched: %d", len(channels))
	log.Printf("Valid channels saved:  %d", len(validChannels))
	log.Printf("Clean playlist saved:  %s", outputPath)
	log.Printf("Total execution time:  %.2f seconds", duration.Seconds())
}

// parseCountries splits a comma-separated string into cleaned country codes.
func parseCountries(input string) []string {
	raw := strings.Split(input, ",")
	var result []string
	for _, c := range raw {
		cleaned := strings.ToLower(strings.TrimSpace(c))
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}
