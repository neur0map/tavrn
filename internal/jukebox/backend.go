package jukebox

import "time"

// Track represents a playable music track.
type Track struct {
	ID       string
	Title    string
	Artist   string
	Duration int    // seconds (0 until ffprobe detects it)
	URL      string // direct MP3 download URL
}

// DurationTime returns the track duration as time.Duration.
func (t Track) DurationTime() time.Duration {
	return time.Duration(t.Duration) * time.Second
}
