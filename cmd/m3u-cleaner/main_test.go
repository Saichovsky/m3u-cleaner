package main

import "testing"

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
