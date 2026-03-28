package jukebox

import (
	"context"
	"testing"
)

func TestChillhopCatalogParsed(t *testing.T) {
	tracks := parseChillhopCatalog()
	if len(tracks) < 100 {
		t.Errorf("expected at least 100 chillhop tracks, got %d", len(tracks))
	}
	if tracks[0].id == "" || tracks[0].title == "" {
		t.Errorf("first track has empty fields: %+v", tracks[0])
	}
}

func TestArchiveCatalogParsed(t *testing.T) {
	tracks := parseArchiveCatalog()
	if len(tracks) < 100 {
		t.Errorf("expected at least 100 archive tracks, got %d", len(tracks))
	}
	if tracks[0].id == "" || tracks[0].title == "" {
		t.Errorf("first track has empty fields: %+v", tracks[0])
	}
	// Archive titles should not end with .mp3
	for _, tr := range tracks[:10] {
		if tr.title[len(tr.title)-4:] == ".mp3" {
			t.Errorf("title should not have .mp3 extension: %q", tr.title)
		}
	}
}

func TestLofiEnabled(t *testing.T) {
	l := NewLofi()
	if !l.Enabled() {
		t.Error("lofi should be enabled with embedded catalogs")
	}
}

func TestLofiTrackCount(t *testing.T) {
	l := NewLofi()
	if l.TrackCount() < 3000 {
		t.Errorf("expected at least 3000 total tracks, got %d", l.TrackCount())
	}
}

func TestLofiName(t *testing.T) {
	l := NewLofi()
	if l.Name() != "lofi" {
		t.Errorf("name = %q, want lofi", l.Name())
	}
}

func TestLofiSearchPopular(t *testing.T) {
	l := NewLofi()
	tracks, err := l.Search(context.Background(), "popular", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(tracks) != 10 {
		t.Errorf("expected 10 tracks, got %d", len(tracks))
	}
	for _, tr := range tracks {
		if tr.Source != "lofi" {
			t.Errorf("source = %q, want lofi", tr.Source)
		}
		if tr.URL == "" {
			t.Error("track has empty URL")
		}
	}
}

func TestLofiSearchMixesSources(t *testing.T) {
	l := NewLofi()
	tracks, err := l.Search(context.Background(), "popular", 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	var chillhop, archive int
	for _, tr := range tracks {
		switch tr.Artist {
		case "Chillhop":
			chillhop++
		case "Lofi Girl":
			archive++
		}
	}
	if chillhop == 0 {
		t.Error("expected some chillhop tracks in random mix")
	}
	if archive == 0 {
		t.Error("expected some archive tracks in random mix")
	}
}

func TestLofiSearchByTitle(t *testing.T) {
	l := NewLofi()
	tracks, err := l.Search(context.Background(), "sun", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(tracks) == 0 {
		t.Error("expected at least one match for 'sun'")
	}
}

func TestLofiStreamURL(t *testing.T) {
	l := NewLofi()
	// Chillhop track
	url, err := l.StreamURL(context.Background(), l.chillhop[0].id)
	if err != nil {
		t.Fatalf("StreamURL chillhop: %v", err)
	}
	if url == "" {
		t.Error("empty chillhop URL")
	}
	// Archive track
	url, err = l.StreamURL(context.Background(), l.archive[0].id)
	if err != nil {
		t.Fatalf("StreamURL archive: %v", err)
	}
	if url == "" {
		t.Error("empty archive URL")
	}
}

func TestLofiStreamURLNotFound(t *testing.T) {
	l := NewLofi()
	_, err := l.StreamURL(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent track")
	}
}
