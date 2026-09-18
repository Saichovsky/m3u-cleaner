package iptv

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test FetchAndParseM3U function: parsing rules, https URLs, and non-stream lines
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
		if _, err := w.Write([]byte(m3uContent)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer ts.Close()

	client := &http.Client{}

	channels, err := FetchAndParseM3U(client, ts.URL)
	if err != nil {
		t.Fatalf("FetchAndParseM3U returned error: %v", err)
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

// Test FetchAndParseM3U with error cases
func TestFetchAndParseM3U_Error(t *testing.T) {
	// Test with non-existent URL
	client := &http.Client{Timeout: 1 * time.Second}

	_, err := FetchAndParseM3U(client, "http://127.0.0.1:12345/nonexistent.m3u")
	if err == nil {
		t.Error("Expected error for non-existent URL, got nil")
	}

	// Test with non-200 status
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	if _, err := FetchAndParseM3U(client, ts.URL); err == nil {
		t.Error("Expected error for HTTP 500, got nil")
	}
}
