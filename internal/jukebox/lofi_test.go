package jukebox

import "testing"

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
	for _, tr := range tracks[:10] {
		if tr.title[len(tr.title)-4:] == ".mp3" {
			t.Errorf("title should not have .mp3 extension: %q", tr.title)
		}
	}
}

func TestLofiTrackCount(t *testing.T) {
	l := NewLofi()
	if l.TrackCount() < 3000 {
		t.Errorf("expected at least 3000 total tracks, got %d", l.TrackCount())
	}
}

func TestLofiRandomTracks(t *testing.T) {
	l := NewLofi()
	tracks := l.randomTracks(10)
	if len(tracks) != 10 {
		t.Errorf("expected 10 tracks, got %d", len(tracks))
	}
	for _, tr := range tracks {
		if tr.URL == "" {
			t.Error("track has empty URL")
		}
		if tr.Title == "" {
			t.Error("track has empty title")
		}
	}
}

func TestLofiRandomMixesSources(t *testing.T) {
	l := NewLofi()
	tracks := l.randomTracks(20)
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
		t.Error("expected some chillhop tracks")
	}
	if archive == 0 {
		t.Error("expected some archive tracks")
	}
}
