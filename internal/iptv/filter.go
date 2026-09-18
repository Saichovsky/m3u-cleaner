package iptv

import (
	"regexp"
	"strconv"
	"strings"
)

// MinResolutionHeight is the default minimum channel resolution.
// Channels with an explicit resolution below this are filtered out.
const MinResolutionHeight = 720

var reResolution = regexp.MustCompile(`(?i)\((\d{3,4})\s*p\s*\)`)

// FilterLowRes removes channels with an explicit resolution below minHeight.
// Channels without a resolution marker (unknown quality) are kept.
// A non-positive minHeight selects MinResolutionHeight.
func FilterLowRes(channels []Channel, minHeight int) []Channel {
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

// FilterExcluded removes channels whose metadata contains any exclude word.
// Matching is a case-insensitive substring match, so "bbc pashto" also
// catches "BBC PASHTO" and "BBC Pashto HD".
func FilterExcluded(channels []Channel, excludeWords []string) []Channel {
	var words []string
	for _, w := range excludeWords {
		w = strings.ToLower(strings.TrimSpace(w))
		if w != "" {
			words = append(words, w)
		}
	}
	if len(words) == 0 {
		return channels
	}

	var result []Channel
	for _, ch := range channels {
		metadata := strings.ToLower(ch.Metadata)
		excluded := false
		for _, w := range words {
			if strings.Contains(metadata, w) {
				excluded = true
				break
			}
		}
		if !excluded {
			result = append(result, ch)
		}
	}
	return result
}
