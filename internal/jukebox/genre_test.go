package jukebox

import "testing"

func TestGenreString(t *testing.T) {
	_ = NewCatalog() // populates genreNames from config
	tests := []struct {
		g    Genre
		want string
	}{
		{Genre(0), "Lofi"},
		{Genre(1), "Jazz"},
		{Genre(2), "Electronic"},
		{Genre(3), "Cantina"},
	}
	for _, tt := range tests {
		if got := tt.g.String(); got != tt.want {
			t.Errorf("Genre(%d).String() = %q, want %q", tt.g, got, tt.want)
		}
	}
}

func TestAllGenres(t *testing.T) {
	_ = NewCatalog()
	genres := AllGenres()
	if len(genres) < 4 {
		t.Errorf("expected at least 4 genres, got %d", len(genres))
	}
	if genres[0].String() != "Lofi" {
		t.Errorf("first genre should be Lofi, got %s", genres[0])
	}
}
