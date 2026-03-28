package jukebox

import (
	"math/rand/v2"
)

const (
	chillhopBaseURL = "https://stream.chillhop.com/mp3/"
	archiveBaseURL  = "https://ia601004.us.archive.org/31/items/lofigirl/"
)

type lofiTrack struct {
	id    string
	title string
}

// Lofi holds the embedded chillhop + archive.org catalogs.
type Lofi struct {
	chillhop []lofiTrack
	archive  []lofiTrack
}

func NewLofi() *Lofi {
	return &Lofi{
		chillhop: parseChillhopCatalog(),
		archive:  parseArchiveCatalog(),
	}
}

func (l *Lofi) TrackCount() int { return len(l.chillhop) + len(l.archive) }

func (l *Lofi) chillhopTrack(t lofiTrack) Track {
	return Track{
		ID:     t.id,
		Title:  t.title,
		Artist: "Chillhop",
		URL:    chillhopBaseURL + t.id,
	}
}

func (l *Lofi) archiveTrack(t lofiTrack) Track {
	return Track{
		ID:     t.id,
		Title:  t.title,
		Artist: "Lofi Girl",
		URL:    archiveBaseURL + t.id,
	}
}

func (l *Lofi) randomTracks(n int) []Track {
	// Mix: 70% chillhop, 30% archive
	chillN := n * 7 / 10
	archN := n - chillN
	if len(l.archive) == 0 {
		chillN = n
		archN = 0
	}
	if len(l.chillhop) == 0 {
		archN = n
		chillN = 0
	}

	var tracks []Track
	tracks = append(tracks, l.pickRandom(l.chillhop, chillN, true)...)
	tracks = append(tracks, l.pickRandom(l.archive, archN, false)...)

	rand.Shuffle(len(tracks), func(i, j int) {
		tracks[i], tracks[j] = tracks[j], tracks[i]
	})
	return tracks
}

func (l *Lofi) pickRandom(catalog []lofiTrack, n int, isChillhop bool) []Track {
	if n > len(catalog) {
		n = len(catalog)
	}
	if n == 0 {
		return nil
	}
	indices := make([]int, len(catalog))
	for i := range indices {
		indices[i] = i
	}
	for i := 0; i < n; i++ {
		j := i + rand.IntN(len(indices)-i)
		indices[i], indices[j] = indices[j], indices[i]
	}
	tracks := make([]Track, n)
	for i := 0; i < n; i++ {
		t := catalog[indices[i]]
		if isChillhop {
			tracks[i] = l.chillhopTrack(t)
		} else {
			tracks[i] = l.archiveTrack(t)
		}
	}
	return tracks
}
