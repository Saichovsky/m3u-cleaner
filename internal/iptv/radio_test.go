package iptv

import "testing"

// Test EnsureRadioAttr adds radio="true" only when absent and preserves the
// rest of the EXTINF metadata, including commas inside quoted attributes.
func TestEnsureRadioAttr(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "adds attribute after duration",
			in:   `#EXTINF:-1 tvg-id="kbc" group-title="Kenya",KBC`,
			want: `#EXTINF:-1 radio="true" tvg-id="kbc" group-title="Kenya",KBC`,
		},
		{
			name: "keeps existing attribute",
			in:   `#EXTINF:-1 radio="true" group-title="Kenya",KBC`,
			want: `#EXTINF:-1 radio="true" group-title="Kenya",KBC`,
		},
		{
			name: "keeps existing attribute case-insensitively",
			in:   `#EXTINF:-1 RADIO="TRUE" group-title="Kenya",KBC`,
			want: `#EXTINF:-1 RADIO="TRUE" group-title="Kenya",KBC`,
		},
		{
			name: "keeps any existing radio attribute",
			in:   `#EXTINF:-1 radio="false" group-title="Kenya",KBC`,
			want: `#EXTINF:-1 radio="false" group-title="Kenya",KBC`,
		},
		{
			name: "handles no attributes",
			in:   `#EXTINF:-1,KBC`,
			want: `#EXTINF:-1 radio="true",KBC`,
		},
		{
			name: "handles no name",
			in:   `#EXTINF:-1`,
			want: `#EXTINF:-1 radio="true"`,
		},
		{
			name: "ignores commas inside quoted attributes",
			in:   `#EXTINF:-1 group-title="News, Sports",KBC`,
			want: `#EXTINF:-1 radio="true" group-title="News, Sports",KBC`,
		},
		{
			name: "leaves non-EXTINF metadata alone",
			in:   `not metadata`,
			want: `not metadata`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EnsureRadioAttr(tt.in); got != tt.want {
				t.Errorf("EnsureRadioAttr(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
