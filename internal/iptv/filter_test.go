package iptv

import "testing"

// Test FilterLowRes function: only explicit resolutions below the threshold are dropped
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

	result := FilterLowRes(channels, 720)

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

// Test FilterLowRes with a non-positive threshold uses the default
func TestFilterLowRes_Default(t *testing.T) {
	channels := []Channel{
		{Metadata: "#EXTINF:-1 ,HD (720p)", URL: "http://a/720"},
		{Metadata: "#EXTINF:-1 ,Old (360p)", URL: "http://a/360"},
	}
	result := FilterLowRes(channels, 0)
	if len(result) != 1 || result[0].URL != "http://a/720" {
		t.Fatalf("FilterLowRes(0) should use default 720p threshold: %+v", result)
	}
}

// Test FilterExcluded with case-insensitive substring matching
func TestFilterExcluded(t *testing.T) {
	channels := []Channel{
		{Metadata: `#EXTINF:-1 tvg-id="bbc.pashto.uk" group-title="News" ,BBC Pashto`, URL: "http://a/pashto"},
		{Metadata: "#EXTINF:-1 ,BBC PASHTO HD", URL: "http://a/pashtohd"},
		{Metadata: "#EXTINF:-1 ,CBeebies", URL: "http://a/cbeebies"},
		{Metadata: "#EXTINF:-1 ,BBC One", URL: "http://a/bbc1"},
		{Metadata: `#EXTINF:-1 group-title="Sports" ,Football`, URL: "http://a/football"},
	}

	tests := []struct {
		name   string
		words  []string
		remain []string
	}{
		{
			name:   "substring case-insensitive",
			words:  []string{"pashto"},
			remain: []string{"http://a/cbeebies", "http://a/bbc1", "http://a/football"},
		},
		{
			name:   "matches group-title",
			words:  []string{"sports"},
			remain: []string{"http://a/pashto", "http://a/pashtohd", "http://a/cbeebies", "http://a/bbc1"},
		},
		{
			name:   "multiple words",
			words:  []string{"bbc pashto", "cbeebies"},
			remain: []string{"http://a/bbc1", "http://a/football"},
		},
		{
			name:   "empty words keep everything",
			words:  []string{"", "  "},
			remain: []string{"http://a/pashto", "http://a/pashtohd", "http://a/cbeebies", "http://a/bbc1", "http://a/football"},
		},
		{
			name:   "no match changes nothing",
			words:  []string{"sky cinema"},
			remain: []string{"http://a/pashto", "http://a/pashtohd", "http://a/cbeebies", "http://a/bbc1", "http://a/football"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterExcluded(channels, tt.words)
			if len(result) != len(tt.remain) {
				t.Fatalf("Expected %d channels to remain, got %d: %+v", len(tt.remain), len(result), result)
			}
			for i, ch := range result {
				if ch.URL != tt.remain[i] {
					t.Errorf("result[%d] = %q, expected %q", i, ch.URL, tt.remain[i])
				}
			}
		})
	}
}
