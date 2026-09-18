package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Test parseCountries function
func TestParseCountries(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{input: "ke,uk,us", expected: []string{"ke", "uk", "us"}},
		{input: " KE , UK , US ", expected: []string{"ke", "uk", "us"}},
		{input: "ke,,uk", expected: []string{"ke", "uk"}},
		{input: "", expected: []string{}},
		{input: "   ", expected: []string{}},
		{input: "ke", expected: []string{"ke"}},
	}

	for _, tt := range tests {
		result := parseCountries(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parseCountries(%q) = %v, expected %v", tt.input, result, tt.expected)
			continue
		}
		for i, v := range result {
			if v != tt.expected[i] {
				t.Errorf("parseCountries(%q)[%d] = %q, expected %q", tt.input, i, v, tt.expected[i])
			}
		}
	}
}

// Test fetchAndParseM3U function: parsing rules, https URLs, and non-stream lines
func TestFetchAndParseM3U(t *testing.T) {
	m3uContent := `#EXTM3U
#EXTINF:-1 tvg-id="bbc.one.uk" group-title="General" ,BBC One
http://example.com/bbc1.m3u8
redirect://not-a-http-stream
#EXTINF:-1 tvg-id="itv.one.uk" group-title="General" ,ITV One
https://example.com/itv1.m3u8
http://example.com/no-metadata.m3u8
#EXTINF:-1 tvg-id="channel4.uk" group-title="General" ,Channel 4
http://example.com/channel4.m3u8
#EXTINF:-1 tvg-id="orphan.uk" group-title="General" ,Orphan (no URL follows)

`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(m3uContent))
	}))
	defer ts.Close()

	client := &http.Client{}

	channels, err := fetchAndParseM3U(client, ts.URL)
	if err != nil {
		t.Fatalf("fetchAndParseM3U returned error: %v", err)
	}

	if len(channels) != 3 {
		t.Fatalf("Expected 3 channels, got %d", len(channels))
	}

	// Check first channel
	if channels[0].Metadata != `#EXTINF:-1 tvg-id="bbc.one.uk" group-title="General" ,BBC One` {
		t.Errorf("Unexpected metadata for first channel: %s", channels[0].Metadata)
	}
	if channels[0].URL != "http://example.com/bbc1.m3u8" {
		t.Errorf("Unexpected URL for first channel: %s", channels[0].URL)
	}

	// Check second channel (https URL)
	if channels[1].URL != "https://example.com/itv1.m3u8" {
		t.Errorf("Unexpected URL for second channel: %s", channels[1].URL)
	}

	// Check third channel
	if channels[2].Metadata != `#EXTINF:-1 tvg-id="channel4.uk" group-title="General" ,Channel 4` {
		t.Errorf("Unexpected metadata for third channel: %s", channels[2].Metadata)
	}
	if channels[2].URL != "http://example.com/channel4.m3u8" {
		t.Errorf("Unexpected URL for third channel: %s", channels[2].URL)
	}
}

// Test fetchAndParseM3U with error cases
func TestFetchAndParseM3U_Error(t *testing.T) {
	// Test with non-existent URL
	client := &http.Client{Timeout: 1 * time.Second}

	_, err := fetchAndParseM3U(client, "http://127.0.0.1:12345/nonexistent.m3u")
	if err == nil {
		t.Error("Expected error for non-existent URL, got nil")
	}

	// Test with non-200 status
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	if _, err := fetchAndParseM3U(client, ts.URL); err == nil {
		t.Error("Expected error for HTTP 500, got nil")
	}
}

// Test filterLowRes function: only explicit resolutions below the threshold are dropped
func TestFilterLowRes(t *testing.T) {
	channels := []Channel{
		{Metadata: "#EXTINF:-1 ,News (1080p)", URL: "http://a/1080"},
		{Metadata: "#EXTINF:-1 ,Old (360p) [SD]", URL: "http://a/360"},
		{Metadata: "#EXTINF:-1 ,Stadium (480p)", URL: "http://a/480"},
		{Metadata: "#EXTINF:-1 ,Classic (576p)", URL: "http://a/576"},
		{Metadata: "#EXTINF:-1 ,HD (720p)", URL: "http://a/720"},
		{Metadata: "#EXTINF:-1 ,Ultra (2160p)", URL: "http://a/2160"},
		{Metadata: "#EXTINF:-1 ,Casey (1080P)", URL: "http://a/1080cap"},
		{Metadata: "#EXTINF:-1 ,No Label", URL: "http://a/unknown"},
		{Metadata: `#EXTINF:-1 tvg-id="X.in@SD" ,Sports (2025)`, URL: "http://a/sports"},
	}

	result := filterLowRes(channels, 720)

	expected := []string{"http://a/1080", "http://a/720", "http://a/2160", "http://a/1080cap", "http://a/unknown", "http://a/sports"}
	if len(result) != len(expected) {
		t.Fatalf("Expected %d channels to remain, got %d", len(expected), len(result))
	}
	for i, ch := range result {
		if ch.URL != expected[i] {
			t.Errorf("result[%d] = %q, expected %q", i, ch.URL, expected[i])
		}
	}
}

// Test filterLowRes with a non-positive threshold uses the default
func TestFilterLowRes_Default(t *testing.T) {
	channels := []Channel{
		{Metadata: "#EXTINF:-1 ,HD (720p)", URL: "http://a/720"},
		{Metadata: "#EXTINF:-1 ,Old (360p)", URL: "http://a/360"},
	}
	result := filterLowRes(channels, 0)
	if len(result) != 1 || result[0].URL != "http://a/720" {
		t.Fatalf("filterLowRes(0) should use default 720p threshold: %+v", result)
	}
}

// Test validateChannels function with mocked HTTP responses
func TestValidateChannels(t *testing.T) {
	// Create test server that responds to specific URLs
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/good.m3u8" {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			// Write minimal valid HLS content
			w.Write([]byte("#EXTM3U\n"))
		} else if r.URL.Path == "/bad.m3u8" {
			w.WriteHeader(http.StatusNotFound)
		} else if r.URL.Path == "/server-error.m3u8" {
			w.WriteHeader(http.StatusInternalServerError)
		} else if r.URL.Path == "/redirect-good.m3u8" {
			http.Redirect(w, r, "/good.m3u8", http.StatusMovedPermanently)
		} else if r.URL.Path == "/redirect-dead.m3u8" {
			http.Redirect(w, r, "/bad.m3u8", http.StatusMovedPermanently)
		} else {
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

	validChannels := validateChannels(testChannels, 1*time.Second)

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

// Test validateChannels with a slow server that exceeds the client timeout
func TestValidateChannels_Timeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	channels := []Channel{
		{Metadata: "#EXTINF:-1 Slow Channel", URL: slow.URL + "/slow.m3u8"},
	}

	validChannels := validateChannels(channels, 200*time.Millisecond)
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
	valid := validateChannels(channels, time.Second, func(done, total, valid int) {
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

// Test validateChannels with a non-positive timeout uses the default
func TestValidateChannels_DefaultTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	validChannels := validateChannels([]Channel{{Metadata: "m", URL: ts.URL + "/ok"}}, 0)
	if len(validChannels) != 1 {
		t.Fatalf("Expected 1 valid channel with default timeout, got %d", len(validChannels))
	}
}

// Test the playlist writing function
func TestWritePlaylist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.m3u")

	channels := []Channel{
		{Metadata: "#EXTINF:-1 ,BBC One", URL: "http://example.com/bbc1.m3u8"},
		{Metadata: "#EXTINF:-1 ,Channel 4", URL: "http://example.com/channel4.m3u8"},
	}

	if err := writePlaylist(path, channels); err != nil {
		t.Fatalf("writePlaylist returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read written playlist: %v", err)
	}

	expected := "#EXTM3U\n" +
		"#EXTINF:-1 ,BBC One\nhttp://example.com/bbc1.m3u8\n" +
		"#EXTINF:-1 ,Channel 4\nhttp://example.com/channel4.m3u8\n"

	if string(data) != expected {
		t.Errorf("Written content mismatch.\nGot:\n%s\nExpected:\n%s", string(data), expected)
	}
}

// Test writePlaylist rewrites an existing file in place, preserving its inode.
// An initContainer runs alongside a pod that serves index.m3u, so the file must
// never be replaced via temp-file+rename (which would change the inode and
// leave a reader with an open file descriptor stuck on the old contents).
func TestWritePlaylist_PreservesInode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.m3u")

	if err := os.WriteFile(path, []byte("#EXTM3U\n#EXTINF:-1 ,Old\nhttp://example.com/old.m3u8\n"), 0644); err != nil {
		t.Fatalf("Failed to seed playlist: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat seeded playlist: %v", err)
	}
	before := info.Sys().(*syscall.Stat_t).Ino

	channels := []Channel{
		{Metadata: "#EXTINF:-1 ,BBC One", URL: "http://example.com/bbc1.m3u8"},
	}
	if err := writePlaylist(path, channels); err != nil {
		t.Fatalf("writePlaylist returned error: %v", err)
	}

	info, err = os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat rewritten playlist: %v", err)
	}
	after := info.Sys().(*syscall.Stat_t).Ino

	if after != before {
		t.Fatalf("writePlaylist replaced the file inode: before=%d after=%d", before, after)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rewritten playlist: %v", err)
	}
	expected := "#EXTM3U\n#EXTINF:-1 ,BBC One\nhttp://example.com/bbc1.m3u8\n"
	if string(data) != expected {
		t.Errorf("Content mismatch.\nGot:\n%s\nExpected:\n%s", string(data), expected)
	}
}

// Test writePlaylist creates missing parent directories
func TestWritePlaylist_CreatesDirs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deeper")
	path := filepath.Join(dir, "index.m3u")

	if err := writePlaylist(path, []Channel{{Metadata: "m", URL: "http://example.com/x"}}); err != nil {
		t.Fatalf("writePlaylist returned error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Expected playlist file to exist: %v", err)
	}
}

// Sanity check that the output uses standard M3U line structure
func TestWritePlaylist_M3UFormatting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.m3u")

	channels := []Channel{
		{Metadata: "#EXTINF:-1 ,A", URL: "http://example.com/a"},
		{Metadata: "#EXTINF:-1 ,B", URL: "http://example.com/b"},
	}
	if err := writePlaylist(path, channels); err != nil {
		t.Fatalf("writePlaylist returned error: %v", err)
	}

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if lines[0] != "#EXTM3U" {
		t.Errorf("Playlist must start with #EXTM3U, got %q", lines[0])
	}
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines (header + 2x2 channel lines), got %d", len(lines))
	}
}
