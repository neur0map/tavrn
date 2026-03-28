package jukebox

import (
	"encoding/json"
	"math/rand/v2"
	"net/url"
	"strings"

	"tavrn.sh/catalogs"
)

// catalogConfig is the top-level genres.json structure.
type catalogConfig struct {
	DefaultBaseURL string        `json:"default_base_url"`
	Genres         []genreConfig `json:"genres"`
}

type genreConfig struct {
	Name    string         `json:"name"`
	Sources []sourceConfig `json:"sources"`
}

type sourceConfig struct {
	File    string `json:"file"`
	BaseURL string `json:"base_url"`
	Format  string `json:"format"` // "id_title" or "path" (default)
	Artist  string `json:"artist"`
	Weight  int    `json:"weight"` // mixing weight for multi-source genres
}

type catalogTrack struct {
	id     string
	title  string
	artist string
	url    string
}

// sourceData holds parsed tracks and weight for one source within a genre.
type sourceData struct {
	tracks []catalogTrack
	weight int
}

// genreData holds all sources for one genre.
type genreData struct {
	name    string
	sources []sourceData
	total   int // sum of all source weights
}

// Catalog holds all genre track lists, loaded from genres.json.
type Catalog struct {
	genres []genreData
}

// NewCatalog reads genres.json and all referenced catalog files.
func NewCatalog() *Catalog {
	data, err := catalogs.FS.ReadFile("genres.json")
	if err != nil {
		panic("jukebox: read genres.json: " + err.Error())
	}
	var cfg catalogConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		panic("jukebox: parse genres.json: " + err.Error())
	}

	c := &Catalog{}
	genreNames = nil

	for _, gc := range cfg.Genres {
		gd := genreData{name: gc.Name}

		for _, src := range gc.Sources {
			baseURL := src.BaseURL
			if baseURL == "" {
				baseURL = cfg.DefaultBaseURL
			}
			format := src.Format
			if format == "" {
				format = "path"
			}
			weight := src.Weight
			if weight <= 0 {
				weight = 100
			}

			raw, err := catalogs.FS.ReadFile(src.File)
			if err != nil {
				panic("jukebox: read catalog " + src.File + ": " + err.Error())
			}

			tracks := parseCatalogFile(string(raw), format, baseURL, src.Artist)
			gd.sources = append(gd.sources, sourceData{tracks: tracks, weight: weight})
			gd.total += weight
		}

		c.genres = append(c.genres, gd)
		genreNames = append(genreNames, gc.Name)
	}

	return c
}

// TrackCount returns the number of tracks for a genre.
func (c *Catalog) TrackCount(g Genre) int {
	if int(g) < 0 || int(g) >= len(c.genres) {
		return 0
	}
	n := 0
	for _, s := range c.genres[g].sources {
		n += len(s.tracks)
	}
	return n
}

// RandomTracks picks n random tracks from the given genre.
// For multi-source genres, tracks are picked proportionally by weight.
func (c *Catalog) RandomTracks(g Genre, n int) []Track {
	if int(g) < 0 || int(g) >= len(c.genres) {
		return nil
	}
	gd := c.genres[g]
	if len(gd.sources) == 0 || gd.total == 0 {
		return nil
	}

	result := make([]Track, 0, n)
	for range n {
		// Pick a source by weight
		r := rand.IntN(gd.total)
		cum := 0
		for _, s := range gd.sources {
			cum += s.weight
			if r < cum && len(s.tracks) > 0 {
				t := s.tracks[rand.IntN(len(s.tracks))]
				result = append(result, Track{
					ID:     t.id,
					Title:  t.title,
					Artist: t.artist,
					URL:    t.url,
				})
				break
			}
		}
	}
	return result
}

// parseCatalogFile parses a catalog txt file into tracks.
func parseCatalogFile(raw, format, baseURL, artist string) []catalogTrack {
	switch format {
	case "id_title":
		return parseIDTitle(raw, baseURL, artist)
	default:
		return parsePath(raw, baseURL, artist)
	}
}

// parseIDTitle parses "ID!Title" lines (chillhop format).
func parseIDTitle(raw, baseURL, artist string) []catalogTrack {
	var tracks []catalogTrack
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "!", 2)
		if len(parts) != 2 {
			continue
		}
		tracks = append(tracks, catalogTrack{
			id:     parts[0],
			title:  parts[1],
			artist: artist,
			url:    baseURL + parts[0],
		})
	}
	return tracks
}

// parsePath parses path-based catalog lines.
// Lines can be URL-encoded or raw paths ending in .mp3.
func parsePath(raw, baseURL, artist string) []catalogTrack {
	var tracks []catalogTrack
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(strings.ToLower(line), ".mp3") {
			continue
		}

		// Build URL: encode path segments if needed
		trackURL := baseURL + encodePath(line)

		// Extract title from filename
		decoded, err := url.PathUnescape(line)
		if err != nil {
			decoded = line
		}
		parts := strings.Split(decoded, "/")
		name := parts[len(parts)-1]
		name = strings.TrimSuffix(name, ".mp3")
		name = stripTrackNumber(name)

		tracks = append(tracks, catalogTrack{
			id:     line,
			title:  name,
			artist: artist,
			url:    trackURL,
		})
	}
	return tracks
}

// encodePath URL-encodes path segments for archive.org URLs.
func encodePath(raw string) string {
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 {
		return url.PathEscape(raw)
	}
	return parts[0] + "/" + url.PathEscape(parts[1])
}

// stripTrackNumber removes leading track numbers like "01 ", "01. ", "01-".
func stripTrackNumber(name string) string {
	if len(name) > 3 && name[0] >= '0' && name[0] <= '9' {
		for i, c := range name {
			if c == ' ' || c == '-' {
				return strings.TrimSpace(name[i+1:])
			}
			if i > 3 {
				break
			}
		}
	}
	return name
}
