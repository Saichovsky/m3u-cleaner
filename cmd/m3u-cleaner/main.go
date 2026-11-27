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
	forceFlag := flag.Bool("force", false, "Regenerate the playlist even if it already exists")
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

	// Skip all work when the destination already exists, unless --force.
	outputPath := filepath.Join(listDir, playlist.OutputFile)
	if !*forceFlag {
		exists, err := playlist.Exists(outputPath)
		if err != nil {
			log.Fatalf("Failed to check output playlist: %v", err)
		}
		if exists {
			log.Printf("Playlist already exists at %s; skipping (use --force to regenerate)", outputPath)
			return
		}
	}

	log.Printf("Starting IPTV Playlist Pipeline (TV: %d playlist(s), Radio: %d playlist(s))",
		len(cfg.TV.PlaylistURLs), len(cfg.Radio.PlaylistURLs))

	// 2. Fetch, filter, and validate each section with its own settings.
	client := &http.Client{Timeout: iptv.FetchTimeoutSeconds * time.Second}
	tvChannels, tvURLs, tvFetched := processSection("TV", client, cfg.TV.Section, cfg.TV.MinResolutionHeight, false)
	radioChannels, radioURLs, radioFetched := processSection("Radio", client, cfg.Radio, 0, true)

	validChannels := append(tvChannels, radioChannels...)
	processedURLs := append(tvURLs, radioURLs...)
	totalFetched := tvFetched + radioFetched

	if totalFetched == 0 {
		log.Fatal("No valid channels retrieved from configured playlists.")
	}

	// 3. Write a single combined output playlist.
	if err := playlist.WritePlaylist(outputPath, validChannels); err != nil {
		log.Fatalf("Failed to write output playlist: %v", err)
	}

	duration := time.Since(startTime)
	log.Println("--- SUMMARY ---")
	log.Printf("Playlists processed: %v", processedURLs)
	log.Printf("Total entries fetched: %d", totalFetched)
	log.Printf("Valid channels saved:  %d", len(validChannels))
	log.Printf("Clean playlist saved:  %s", outputPath)
	log.Printf("Total execution time:  %.2f seconds", duration.Seconds())
}

// processSection fetches, filters, and validates one section's playlists.
// minResolution is 0 when resolution filtering does not apply (radio).
// radio controls whether radio="true" is added to each entry.
func processSection(label string, client *http.Client, sec config.Section, minResolution int, radio bool) (valid []iptv.Channel, processedURLs []string, fetched int) {
	if len(sec.PlaylistURLs) == 0 {
		return nil, nil, 0
	}

	var channels []iptv.Channel
	for _, url := range sec.PlaylistURLs {
		log.Printf("[%s] Fetching: %s", label, url)

		parsed, err := iptv.FetchAndParseM3U(client, url)
		if err != nil {
			log.Printf("[%s] Warning: Error fetching %s: %v", label, url, err)
			continue
		}
		if radio {
			for i := range parsed {
				parsed[i].Metadata = iptv.EnsureRadioAttr(parsed[i].Metadata)
			}
		}
		channels = append(channels, parsed...)
		processedURLs = append(processedURLs, url)
		log.Printf("[%s] Fetched %s: %d channels (cumulative: %d)", label, url, len(parsed), len(channels))
	}

	fetched = len(channels)
	log.Printf("[%s] Total entries fetched: %d", label, fetched)
	if fetched == 0 {
		return nil, processedURLs, 0
	}

	if minResolution > 0 {
		channels = iptv.FilterLowRes(channels, minResolution)
		log.Printf("[%s] Total entries after low-res filter: %d", label, len(channels))
	}

	channels = iptv.FilterExcluded(channels, sec.ExcludeChannels)
	log.Printf("[%s] Total entries after exclude filter: %d", label, len(channels))

	progress := iptv.NewProgressReporter()
	valid = iptv.ValidateChannelsN(channels, time.Duration(sec.TimeoutSeconds)*time.Second, sec.Concurrency, func(done, total, validCount int) {
		progress.Report(label+" validation", done, total, validCount)
	})
	log.Printf("[%s] Validation complete: %d valid out of %d channels", label, len(valid), len(channels))
	return valid, processedURLs, fetched
}
