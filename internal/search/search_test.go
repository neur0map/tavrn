package search

import "testing"

func TestNeedsSearch(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"@bartender what is a reverse shell", true},
		{"@bartender how does SSH work", true},
		{"@bartender explain buffer overflow", true},
		{"@bartender who is linus torvalds", true},
		{"@bartender tell me about nmap", true},
		{"@bartender search for SQL injection", true},
		{"@bartender weather in new york", true},
		{"@bartender current news", true},
		{"@bartender is rust faster than go?", true},
		{"@bartender hey", false},
		{"@bartender sup", false},
		{"@bartender lol", false},
		{"@bartender thanks", false},
		{"@bartender whats a CVE", true},
		{"@bartender look up XSS attacks", true},
	}
	for _, tt := range tests {
		got := NeedsSearch(tt.text)
		if got != tt.want {
			t.Errorf("NeedsSearch(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestFormatForLLM(t *testing.T) {
	results := []Result{
		{Title: "Test", URL: "https://example.com", Snippet: "This is a test result"},
	}
	formatted := FormatForLLM(results)
	if formatted == "" {
		t.Error("expected non-empty output")
	}
	if len(formatted) < 10 {
		t.Error("formatted output too short")
	}
}
