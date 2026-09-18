package playlist

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"m3u-cleaner/internal/iptv"
)

// Test the playlist writing function
func TestWritePlaylist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.m3u")

	channels := []iptv.Channel{
		{Metadata: "#EXTINF:-1 ,BBC One", URL: "http://example.com/bbc1.m3u8"},
		{Metadata: "#EXTINF:-1 ,Channel 4", URL: "http://example.com/channel4.m3u8"},
	}

	if err := WritePlaylist(path, channels); err != nil {
		t.Fatalf("WritePlaylist returned error: %v", err)
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

// Test WritePlaylist rewrites an existing file in place, preserving its inode.
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

	channels := []iptv.Channel{
		{Metadata: "#EXTINF:-1 ,BBC One", URL: "http://example.com/bbc1.m3u8"},
	}
	if err := WritePlaylist(path, channels); err != nil {
		t.Fatalf("WritePlaylist returned error: %v", err)
	}

	info, err = os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat rewritten playlist: %v", err)
	}
	after := info.Sys().(*syscall.Stat_t).Ino

	if after != before {
		t.Fatalf("WritePlaylist replaced the file inode: before=%d after=%d", before, after)
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

// Test WritePlaylist creates missing parent directories
func TestWritePlaylist_CreatesDirs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deeper")
	path := filepath.Join(dir, "index.m3u")

	if err := WritePlaylist(path, []iptv.Channel{{Metadata: "m", URL: "http://example.com/x"}}); err != nil {
		t.Fatalf("WritePlaylist returned error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Expected playlist file to exist: %v", err)
	}
}

// Sanity check that the output uses standard M3U line structure
func TestWritePlaylist_M3UFormatting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.m3u")

	channels := []iptv.Channel{
		{Metadata: "#EXTINF:-1 ,A", URL: "http://example.com/a"},
		{Metadata: "#EXTINF:-1 ,B", URL: "http://example.com/b"},
	}
	if err := WritePlaylist(path, channels); err != nil {
		t.Fatalf("WritePlaylist returned error: %v", err)
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
