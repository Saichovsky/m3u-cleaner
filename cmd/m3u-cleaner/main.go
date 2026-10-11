package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"m3u-cleaner/internal/config"
	"m3u-cleaner/internal/iptv"
	"m3u-cleaner/internal/playlist"
)

func main() {
	startTime := time.Now()

	// 1. Parse CLI flags and Environment Variables
	outDirFlag := flag.String("outdir", "", "Target directory for the generated index.m3u")
	configFlag := flag.String("config", "", "Path to a TOML config file (required)")
	flag.Parse()

	// Config path: Flag -> ENV (mandatory — app won't run without it)
	configPath := *configFlag
	if configPath == "" {
		configPath = os.Getenv("CONFIG")
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Loaded config from %s", configPath)

	// Resolve target directory: Flag -> ENV -> Default
	listDir := *outDirFlag
	if listDir == "" {
		listDir = os.Getenv("LISTDIR")
	}
	if listDir == "" {
		listDir = "/usr/share/nginx/html"
	}

	log.SetFlags(log.Ldate | log.Ltime)
	log.Printf("Starting IPTV Playlist Pipeline for %d playlist(s)", len(cfg.PlaylistURLs))

	// 2. Fetch Playlists
	client := &http.Client{Timeout: iptv.FetchTimeoutSeconds * time.Second}
	var channels []iptv.Channel
	var processedURLs []string

	for _, url := range cfg.PlaylistURLs {
		log.Printf("Fetching: %s", url)

		parsed, err := iptv.FetchAndParseM3U(client, url)
		if err != nil {
			log.Printf("Warning: Error fetching %s: %v", url, err)
			continue
		}
		channels = append(channels, parsed...)
		processedURLs = append(processedURLs, url)
		log.Printf("Fetched %s: %d channels (cumulative: %d)", url, len(parsed), len(channels))
	}

	log.Printf("Total entries fetched: %d", len(channels))
	if len(channels) == 0 {
		log.Fatal("No valid channels retrieved from configured playlists.")
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
	log.Printf("Playlists processed: %v", processedURLs)
	log.Printf("Total entries fetched: %d", len(channels))
	log.Printf("Valid channels saved:  %d", len(validChannels))
	log.Printf("Clean playlist saved:  %s", outputPath)
	log.Printf("Total execution time:  %.2f seconds", duration.Seconds())
}
