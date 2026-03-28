package jukebox

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"
)

const (
	chillhopBaseURL = "https://stream.chillhop.com/mp3/"
	archiveBaseURL  = "https://ia601004.us.archive.org/31/items/lofigirl/"
)

// lofiTrack is a track from the embedded catalog.
type lofiTrack struct {
	id    string
	title string
}

// Lofi implements MusicBackend using two sources with automatic failover:
// 1. Chillhop (primary) — stream.chillhop.com
// 2. Archive.org Lofi Girl (fallback) — Internet Archive
// No API key required. Tracks are from embedded catalogs.
type Lofi struct {
	chillhop []lofiTrack
	archive  []lofiTrack
	client   *http.Client
}

// NewLofi creates a Lofi backend with both catalogs loaded.
func NewLofi() *Lofi {
	return &Lofi{
		chillhop: parseChillhopCatalog(),
		archive:  parseArchiveCatalog(),
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (l *Lofi) Name() string  { return "lofi" }
func (l *Lofi) Enabled() bool { return len(l.chillhop)+len(l.archive) > 0 }

// TrackCount returns the total number of tracks across both catalogs.
func (l *Lofi) TrackCount() int { return len(l.chillhop) + len(l.archive) }

func (l *Lofi) Search(_ context.Context, query string, limit int) ([]Track, error) {
	query = strings.ToLower(query)

	if query == "popular" {
		return l.randomTracks(limit), nil
	}

	var matches []Track
	// Search chillhop first
	for _, t := range l.chillhop {
		if strings.Contains(strings.ToLower(t.title), query) {
			matches = append(matches, l.chillhopTrack(t))
			if len(matches) >= limit {
				return matches, nil
			}
		}
	}
	// Then archive
	for _, t := range l.archive {
		if strings.Contains(strings.ToLower(t.title), query) {
			matches = append(matches, l.archiveTrack(t))
			if len(matches) >= limit {
				return matches, nil
			}
		}
	}
	return matches, nil
}

func (l *Lofi) StreamURL(_ context.Context, trackID string) (string, error) {
	for _, t := range l.chillhop {
		if t.id == trackID {
			return chillhopBaseURL + t.id, nil
		}
	}
	for _, t := range l.archive {
		if t.id == trackID {
			return archiveBaseURL + t.id, nil
		}
	}
	return "", fmt.Errorf("lofi: track %s not found", trackID)
}

func (l *Lofi) chillhopTrack(t lofiTrack) Track {
	return Track{
		ID:       t.id,
		Title:    t.title,
		Artist:   "Chillhop",
		Duration: 0, // determined by ffprobe after download
		URL:      chillhopBaseURL + t.id,
		Source:   "lofi",
	}
}

func (l *Lofi) archiveTrack(t lofiTrack) Track {
	return Track{
		ID:       t.id,
		Title:    t.title,
		Artist:   "Lofi Girl",
		Duration: 0,
		URL:      archiveBaseURL + t.id,
		Source:   "lofi",
	}
}

func (l *Lofi) randomTracks(n int) []Track {
	// Mix from both catalogs: 70% chillhop, 30% archive
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

	// Shuffle the mixed list
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
