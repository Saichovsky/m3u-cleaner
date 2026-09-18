package iptv

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// FetchTimeoutSeconds bounds a single playlist download.
const FetchTimeoutSeconds = 15

// FetchAndParseM3U downloads url and parses it into channels. Every channel
// must consist of an #EXTINF metadata line followed immediately by a URL line
// starting with "http"; non-stream lines and orphan metadata are skipped.
func FetchAndParseM3U(client *http.Client, url string) ([]Channel, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error closing response body: %v", err)
		}
	}()

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
