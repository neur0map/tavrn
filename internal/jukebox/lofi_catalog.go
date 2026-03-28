package jukebox

import (
	_ "embed"
	"net/url"
	"strings"
)

//go:embed chillhop_catalog.txt
var chillhopCatalog string

//go:embed archive_catalog.txt
var archiveCatalog string

// parseChillhopCatalog parses "ID!Title" lines.
func parseChillhopCatalog() []lofiTrack {
	var tracks []lofiTrack
	for _, line := range strings.Split(chillhopCatalog, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "!", 2)
		if len(parts) != 2 {
			continue
		}
		tracks = append(tracks, lofiTrack{
			id:    parts[0],
			title: parts[1],
		})
	}
	return tracks
}

// parseArchiveCatalog parses URL-encoded path lines.
// Extracts artist/title from the filename.
func parseArchiveCatalog() []lofiTrack {
	var tracks []lofiTrack
	for _, line := range strings.Split(archiveCatalog, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(strings.ToLower(line), ".mp3") {
			continue
		}
		// Decode URL-encoded path for display title
		decoded, err := url.PathUnescape(line)
		if err != nil {
			decoded = line
		}
		// Extract filename without extension for title
		parts := strings.Split(decoded, "/")
		name := parts[len(parts)-1]
		name = strings.TrimSuffix(name, ".mp3")
		// Remove track number prefix like "01 " or "01. "
		if len(name) > 3 && name[0] >= '0' && name[0] <= '9' {
			for i, c := range name {
				if c == ' ' || c == '-' {
					name = strings.TrimSpace(name[i+1:])
					break
				}
				if i > 3 {
					break
				}
			}
		}

		tracks = append(tracks, lofiTrack{
			id:    line, // URL-encoded path is the ID
			title: name,
		})
	}
	return tracks
}
