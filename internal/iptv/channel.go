package iptv

// Channel is a single IPTV channel entry parsed from an M3U playlist.
type Channel struct {
	Metadata string
	URL      string
}
