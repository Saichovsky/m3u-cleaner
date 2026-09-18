package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Configuration
const (
	OutputFile          = "index.m3u"
	ConcurrentChecks    = 50
	TimeoutSeconds      = 10
	FetchTimeoutSeconds = 15
	MinResolutionHeight = 720
	UserAgent           = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
)

// DefaultTimeout is used when a caller passes a non-positive timeout.
const DefaultTimeout = TimeoutSeconds * time.Second

type Channel struct {
	Metadata string
	URL      string
}

func main() {
	startTime := time.Now()

	// 1. Parse CLI flags and Environment Variables
	countriesFlag := flag.String("countries", "", "Comma-separated list of country codes (e.g. ke,uk,us)")
	outDirFlag := flag.String("outdir", "", "Target directory for the generated index.m3u")
	flag.Parse()

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
	client := &http.Client{Timeout: FetchTimeoutSeconds * time.Second}
	var channels []Channel
	var processedCountries []string

	for _, country := range countries {
		url := fmt.Sprintf("https://iptv-org.github.io/iptv/countries/%s.m3u", country)
		log.Printf("Fetching: %s", url)

		parsed, err := fetchAndParseM3U(client, url)
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

	// 3. Filter Out Low-Resolution Channels (< 720p)
	channels = filterLowRes(channels, MinResolutionHeight)
	log.Printf("Total entries after low-res filter: %d", len(channels))

	// 4. Concurrently Validate Stream URLs
	progress := &ProgressReporter{lastPercent: -1}
	validChannels := validateChannels(channels, DefaultTimeout, func(done, total, valid int) {
		progress.Report("Validation", done, total, valid)
	})
	log.Printf("Validation complete: %d valid out of %d channels", len(validChannels), len(channels))

	// 5. Write Output Playlist
	outputPath := filepath.Join(listDir, OutputFile)
	if err := writePlaylist(outputPath, validChannels); err != nil {
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

// Parses comma-separated string into cleaned country slice
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

func fetchAndParseM3U(client *http.Client, url string) ([]Channel, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	var channels []Channel
	scanner := bufio.NewScanner(resp.Body)
	var lastMetadata string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#EXTINF:") {
			lastMetadata = line
		} else if strings.HasPrefix(line, "http") && lastMetadata != "" {
			channels = append(channels, Channel{
				Metadata: lastMetadata,
				URL:      line,
			})
			lastMetadata = ""
		}
	}
	return channels, scanner.Err()
}

var reResolution = regexp.MustCompile(`(?i)\((\d{3,4})\s*p\s*\)`)

// filterLowRes removes channels with an explicit resolution below minHeight.
// Channels without a resolution marker (unknown quality) are kept.
func filterLowRes(channels []Channel, minHeight int) []Channel {
	if minHeight <= 0 {
		minHeight = MinResolutionHeight
	}
	var result []Channel
	for _, ch := range channels {
		height := 0
		if m := reResolution.FindStringSubmatch(ch.Metadata); len(m) > 1 {
			height, _ = strconv.Atoi(m[1])
		}
		if height == 0 || height >= minHeight {
			result = append(result, ch)
		}
	}
	return result
}

// ProgressReporter logs periodic progress lines, throttled to integer
// percentage changes so long-running phases stay visible without spamming.
type ProgressReporter struct {
	lastPercent int
}

func (p *ProgressReporter) Report(phase string, done, total, valid int) {
	percent := 0
	if total > 0 {
		percent = done * 100 / total
	}
	if percent <= p.lastPercent {
		return
	}
	p.lastPercent = percent
	log.Printf("%s: %d/%d (%d%%), %d valid", phase, done, total, percent, valid)
}

func validateChannels(channels []Channel, timeout time.Duration, onProgress ...func(done, total, valid int)) []Channel {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	checkClient := &http.Client{Timeout: timeout}

	jobs := make(chan Channel, len(channels))
	results := make(chan *Channel, len(channels))
	var wg sync.WaitGroup

	for i := 0; i < ConcurrentChecks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ch := range jobs {
				req, err := http.NewRequest(http.MethodGet, ch.URL, nil)
				if err != nil {
					results <- nil
					continue
				}
				req.Header.Set("User-Agent", UserAgent)
				req.Header.Set("Accept", "application/vnd.apple.mpegurl, video/mp2t, */*")

				resp, err := checkClient.Do(req)
				if err == nil && resp.StatusCode == http.StatusOK {
					// Early cancellation: close body without reading to save bandwidth
					resp.Body.Close()
					chCopy := ch
					results <- &chCopy
				} else {
					if resp != nil {
						resp.Body.Close()
					}
					results <- nil
				}
			}
		}()
	}

	go func() {
		for _, ch := range channels {
			jobs <- ch
		}
		close(jobs)
	}()

	// Collect results as workers complete so progress is reported live
	// instead of only after the entire batch finishes.
	var valid []Channel
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		done := 0
		validCount := 0
		for res := range results {
			done++
			if res != nil {
				validCount++
				valid = append(valid, *res)
			}
			if len(onProgress) > 0 {
				onProgress[0](done, len(channels), validCount)
			}
		}
	}()

	wg.Wait()
	close(results)
	<-collectorDone

	return valid
}

func writePlaylist(outputPath string, channels []Channel) (err error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	writer := bufio.NewWriterSize(file, 64*1024)
	if _, err := writer.WriteString("#EXTM3U\n"); err != nil {
		return err
	}
	for _, ch := range channels {
		if _, err := writer.WriteString(ch.Metadata); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
		if _, err := writer.WriteString(ch.URL); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
	}
	return writer.Flush()
}
