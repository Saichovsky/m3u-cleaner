package iptv

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test ValidateChannels function with mocked HTTP responses
func TestValidateChannels(t *testing.T) {
	// Create test server that responds to specific URLs
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/good.m3u8":
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			// Write minimal valid HLS content
			if _, err := w.Write([]byte("#EXTM3U\n")); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		case "/bad.m3u8":
			w.WriteHeader(http.StatusNotFound)
		case "/server-error.m3u8":
			w.WriteHeader(http.StatusInternalServerError)
		case "/redirect-good.m3u8":
			http.Redirect(w, r, "/good.m3u8", http.StatusMovedPermanently)
		case "/redirect-dead.m3u8":
			http.Redirect(w, r, "/bad.m3u8", http.StatusMovedPermanently)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	testChannels := []Channel{
		{Metadata: "#EXTINF:-1 Good Channel", URL: ts.URL + "/good.m3u8"},
		{Metadata: "#EXTINF:-1 Bad Channel", URL: ts.URL + "/bad.m3u8"},
		{Metadata: "#EXTINF:-1 Server Error Channel", URL: ts.URL + "/server-error.m3u8"},
		{Metadata: "#EXTINF:-1 Redirect to Good", URL: ts.URL + "/redirect-good.m3u8"},
		{Metadata: "#EXTINF:-1 Redirect to Dead", URL: ts.URL + "/redirect-dead.m3u8"},
		{Metadata: "#EXTINF:-1 Invalid URL", URL: "not-a-valid-url"},
	}

	validChannels := ValidateChannels(testChannels, 1*time.Second)

	// Should only have 2 valid channels (good + redirect-to-good)
	if len(validChannels) != 2 {
		t.Fatalf("Expected 2 valid channels, got %d", len(validChannels))
	}

	validURLs := map[string]bool{}
	for _, ch := range validChannels {
		validURLs[ch.URL] = true
	}
	if ch, ok := validURLs[ts.URL+"/good.m3u8"]; !ok || !ch {
		t.Errorf("Expected %s to be valid", ts.URL+"/good.m3u8")
	}
	if ch, ok := validURLs[ts.URL+"/redirect-good.m3u8"]; !ok || !ch {
		t.Errorf("Expected redirect that lands on 200 to be valid")
	}
}

// Test ValidateChannels with a slow server that exceeds the client timeout
func TestValidateChannels_Timeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	channels := []Channel{
		{Metadata: "#EXTINF:-1 Slow Channel", URL: slow.URL + "/slow.m3u8"},
	}

	validChannels := ValidateChannels(channels, 200*time.Millisecond)
	if len(validChannels) != 0 {
		t.Fatalf("Expected timed-out channel to be dropped, got %d valid", len(validChannels))
	}
}

// Test the progress callback receives accurate final counts
func TestValidateChannels_Progress(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	channels := make([]Channel, 5)
	for i := range channels {
		channels[i] = Channel{
			Metadata: fmt.Sprintf("#EXTINF:-1 ch%d", i),
			URL:      fmt.Sprintf("%s/ch%d", ts.URL, i),
		}
	}

	var lastDone, lastTotal, lastValid int
	valid := ValidateChannels(channels, time.Second, func(done, total, valid int) {
		lastDone, lastTotal, lastValid = done, total, valid
	})

	if lastTotal != 5 || lastDone != 5 {
		t.Fatalf("Expected final progress of 5/5, got %d/%d", lastDone, lastTotal)
	}
	if lastValid != len(valid) {
		t.Errorf("Final valid count mismatch: progress=%d vs returned=%d", lastValid, len(valid))
	}

	pr := &ProgressReporter{lastPercent: -1}
	pr.Report("test", 1, 10, 1)
	pr.Report("test", 1, 10, 1) // duplicate percent must not log again
}

// Test ValidateChannels with a non-positive timeout uses the default
func TestValidateChannels_DefaultTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	validChannels := ValidateChannels([]Channel{{Metadata: "m", URL: ts.URL + "/ok"}}, 0)
	if len(validChannels) != 1 {
		t.Fatalf("Expected 1 valid channel with default timeout, got %d", len(validChannels))
	}
}
