package ui

import (
	"testing"
	"time"
)

func TestFormatTimestamp_JustNow(t *testing.T) {
	now := time.Now()
	got := formatTimestamp(now, now)
	if got != "just now" {
		t.Errorf("got %q, want 'just now'", got)
	}
}

func TestFormatTimestamp_FewSecondsAgo(t *testing.T) {
	now := time.Now()
	got := formatTimestamp(now.Add(-5*time.Second), now)
	if got != "just now" {
		t.Errorf("got %q, want 'just now' (under 10s)", got)
	}
}

func TestFormatTimestamp_SecondsAgo(t *testing.T) {
	now := time.Now()
	got := formatTimestamp(now.Add(-30*time.Second), now)
	if got != "30s ago" {
		t.Errorf("got %q, want '30s ago'", got)
	}
}

func TestFormatTimestamp_OneMinAgo(t *testing.T) {
	now := time.Now()
	got := formatTimestamp(now.Add(-90*time.Second), now)
	if got != "1 min ago" {
		t.Errorf("got %q, want '1 min ago'", got)
	}
}

func TestFormatTimestamp_MinutesAgo(t *testing.T) {
	now := time.Now()
	got := formatTimestamp(now.Add(-10*time.Minute), now)
	if got != "10 mins ago" {
		t.Errorf("got %q, want '10 mins ago'", got)
	}
}

func TestFormatTimestamp_HoursAgo(t *testing.T) {
	now := time.Now()
	ts := now.Add(-3 * time.Hour)
	got := formatTimestamp(ts, now)
	expected := ts.Format("15:04")
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestFormatTimestamp_DaysAgo(t *testing.T) {
	now := time.Now()
	ts := now.Add(-48 * time.Hour)
	got := formatTimestamp(ts, now)
	expected := ts.Format("Jan 02 15:04")
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestFormatTimestamp_ExactBoundary10s(t *testing.T) {
	now := time.Now()
	got := formatTimestamp(now.Add(-10*time.Second), now)
	if got != "10s ago" {
		t.Errorf("at exactly 10s boundary got %q, want '10s ago'", got)
	}
}

func TestFormatTimestamp_ExactBoundary1Min(t *testing.T) {
	now := time.Now()
	got := formatTimestamp(now.Add(-60*time.Second), now)
	if got != "1 min ago" {
		t.Errorf("at exactly 60s boundary got %q, want '1 min ago'", got)
	}
}

func TestWordWrap_Short(t *testing.T) {
	lines := wordWrap("hello", 80)
	if len(lines) != 1 || lines[0] != "hello" {
		t.Errorf("got %v", lines)
	}
}

func TestWordWrap_Long(t *testing.T) {
	text := "the quick brown fox jumps over the lazy dog"
	lines := wordWrap(text, 20)
	if len(lines) < 2 {
		t.Errorf("expected wrapping, got %d lines", len(lines))
	}
	for _, line := range lines {
		if len(line) > 20 {
			t.Errorf("line %q exceeds width 20", line)
		}
	}
}

func TestWordWrap_ZeroWidth(t *testing.T) {
	lines := wordWrap("hello world", 0)
	if len(lines) != 1 {
		t.Errorf("zero width should return single line, got %d", len(lines))
	}
}

func TestWordWrap_ExactFit(t *testing.T) {
	lines := wordWrap("hello", 5)
	if len(lines) != 1 || lines[0] != "hello" {
		t.Errorf("got %v", lines)
	}
}

func TestWordWrap_SingleLongWord(t *testing.T) {
	// A single word longer than width can't be split
	lines := wordWrap("superlongword", 5)
	if len(lines) != 1 || lines[0] != "superlongword" {
		t.Errorf("got %v", lines)
	}
}
