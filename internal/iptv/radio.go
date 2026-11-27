package iptv

import "strings"

// EnsureRadioAttr adds radio="true" to EXTINF metadata when no radio attribute
// is present. The attribute is inserted right after the duration token so
// commas inside attribute values or channel names are left untouched.
func EnsureRadioAttr(metadata string) string {
	const prefix = "#EXTINF:"
	if !strings.HasPrefix(metadata, prefix) || strings.Contains(strings.ToLower(metadata), "radio=") {
		return metadata
	}

	rest := metadata[len(prefix):]
	i := strings.IndexAny(rest, " ,")
	if i < 0 {
		return metadata + ` radio="true"`
	}
	pos := len(prefix) + i
	return metadata[:pos] + ` radio="true"` + metadata[pos:]
}
