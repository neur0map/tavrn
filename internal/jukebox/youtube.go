package jukebox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ytSearchResult is a single result from yt-dlp --flat-playlist -j.
type ytSearchResult struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Duration *float64 `json:"duration"`
	Channel  string   `json:"channel"`
}

// YouTube implements MusicBackend using yt-dlp for search and audio extraction.
// Audio is fetched through yt-dlp which can be configured with a proxy via
// the YT_PROXY environment variable to avoid exposing the server IP.
type YouTube struct {
	proxy string // optional SOCKS5/HTTP proxy for yt-dlp
}

// NewYouTube creates a YouTube backend.
// proxy is an optional proxy URL (e.g. "socks5://127.0.0.1:1080").
// Pass empty string for direct connection.
func NewYouTube(proxy string) *YouTube {
	return &YouTube{proxy: proxy}
}

func (y *YouTube) Name() string  { return "youtube" }
func (y *YouTube) Enabled() bool { _, err := exec.LookPath("yt-dlp"); return err == nil }

func (y *YouTube) Search(ctx context.Context, query string, limit int) ([]Track, error) {
	if limit > 10 {
		limit = 10
	}

	args := []string{
		"--flat-playlist",
		"-j",
		"--no-warnings",
		"--default-search", "ytsearch",
		fmt.Sprintf("ytsearch%d:%s", limit, query),
	}
	if y.proxy != "" {
		args = append([]string{"--proxy", y.proxy}, args...)
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("youtube: search: %w", err)
	}

	var tracks []Track
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var r ytSearchResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		dur := 0
		if r.Duration != nil {
			dur = int(*r.Duration)
		}
		// Skip livestreams (no duration) and very long videos (>15 min)
		if dur == 0 || dur > 900 {
			continue
		}
		tracks = append(tracks, Track{
			ID:       r.ID,
			Title:    r.Title,
			Artist:   r.Channel,
			Duration: dur,
			Source:   "youtube",
		})
	}
	return tracks, nil
}

func (y *YouTube) StreamURL(ctx context.Context, trackID string) (string, error) {
	args := []string{
		"-f", "bestaudio[ext=m4a]/bestaudio",
		"--get-url",
		"--no-warnings",
		fmt.Sprintf("https://www.youtube.com/watch?v=%s", trackID),
	}
	if y.proxy != "" {
		args = append([]string{"--proxy", y.proxy}, args...)
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("youtube: stream url: %w", err)
	}

	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", fmt.Errorf("youtube: no audio URL for %s", trackID)
	}
	return url, nil
}

// ResolveAndSetURL gets the direct audio URL for a track before streaming.
func (y *YouTube) ResolveAndSetURL(ctx context.Context, track *Track) error {
	if track.URL != "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	url, err := y.StreamURL(ctx, track.ID)
	if err != nil {
		return err
	}
	track.URL = url
	return nil
}

// DownloadMP3 downloads the audio as MP3 bytes using yt-dlp + ffmpeg.
// Downloads to a temp file since yt-dlp can't convert and pipe to stdout.
func (y *YouTube) DownloadMP3(ctx context.Context, trackID string) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "tavrn-yt-*")
	if err != nil {
		return nil, fmt.Errorf("youtube: tmpdir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	outPath := filepath.Join(tmpDir, "audio.%(ext)s")
	args := []string{
		"-f", "bestaudio",
		"-x", "--audio-format", "mp3",
		"--audio-quality", "128K",
		"-o", outPath,
		"--no-warnings",
		"--no-playlist",
		fmt.Sprintf("https://www.youtube.com/watch?v=%s", trackID),
	}
	if y.proxy != "" {
		args = append([]string{"--proxy", y.proxy}, args...)
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("youtube: download: %w", err)
	}

	mp3Path := filepath.Join(tmpDir, "audio.mp3")
	data, err := os.ReadFile(mp3Path)
	if err != nil {
		return nil, fmt.Errorf("youtube: read mp3: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("youtube: empty audio for %s", trackID)
	}
	return data, nil
}
