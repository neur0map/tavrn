package jukebox

import (
	"strings"
	"testing"
)

func TestCatalogTrackCount(t *testing.T) {
	c := NewCatalog()
	if c.TrackCount(Genre(0)) < 3000 {
		t.Errorf("lofi tracks = %d, want >= 3000", c.TrackCount(Genre(0)))
	}
	if c.TrackCount(Genre(1)) < 200 {
		t.Errorf("jazz tracks = %d, want >= 200", c.TrackCount(Genre(1)))
	}
	if c.TrackCount(Genre(2)) < 200 {
		t.Errorf("electronic tracks = %d, want >= 200", c.TrackCount(Genre(2)))
	}
	if c.TrackCount(Genre(3)) < 100 {
		t.Errorf("cantina tracks = %d, want >= 100", c.TrackCount(Genre(3)))
	}
}

func TestCatalogRandomTracks(t *testing.T) {
	c := NewCatalog()
	for _, g := range AllGenres() {
		tracks := c.RandomTracks(g, 5)
		if len(tracks) != 5 {
			t.Errorf("%s: got %d tracks, want 5", g, len(tracks))
		}
		for _, tr := range tracks {
			if tr.URL == "" {
				t.Errorf("%s: track has empty URL", g)
			}
			if tr.Title == "" {
				t.Errorf("%s: track has empty title", g)
			}
		}
	}
}

func TestCatalogRandomTracksJazzURLFormat(t *testing.T) {
	c := NewCatalog()
	tracks := c.RandomTracks(Genre(1), 3)
	for _, tr := range tracks {
		if !strings.HasPrefix(tr.URL, "https://archive.org/download/") {
			t.Errorf("jazz URL wrong prefix: %q", tr.URL)
		}
	}
}
