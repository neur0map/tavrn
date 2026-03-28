package jukebox

// Genre is an index into the catalog's genre list.
type Genre int

// genreNames is populated by NewCatalog from genres.json.
var genreNames []string

func (g Genre) String() string {
	if int(g) >= 0 && int(g) < len(genreNames) {
		return genreNames[g]
	}
	return "Unknown"
}

// AllGenres returns all available genres in display order.
func AllGenres() []Genre {
	genres := make([]Genre, len(genreNames))
	for i := range genreNames {
		genres[i] = Genre(i)
	}
	return genres
}
