package jukebox

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Streamer fetches MP3 data from track URLs and sends complete tracks
// to connected audio channels.
//
// Wire format per track: [4-byte header len][JSON header][4-byte audio len][MP3 bytes]
type Streamer struct {
	mu              sync.RWMutex
	conns           map[io.WriteCloser]bool
	cancel          context.CancelFunc
	currentTrack    *Track
	audioData       []byte    // complete MP3 for current track
	playStart       time.Time // when the current track started playing
	client          *http.Client
	onDurationKnown func(int) // callback to update engine with actual duration
	onError         func()    // callback when download fails (engine retries)
}

// NewStreamer creates a new audio streamer.
func NewStreamer() *Streamer {
	return &Streamer{
		conns:  make(map[io.WriteCloser]bool),
		client: &http.Client{},
	}
}

// SetOnDurationKnown sets a callback invoked when actual track duration is known.
func (s *Streamer) SetOnDurationKnown(fn func(int)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDurationKnown = fn
}

// SetOnError sets a callback invoked when a download fails.
func (s *Streamer) SetOnError(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onError = fn
}

// AddConn registers a new audio channel connection.
// If a track is playing, sends the remaining audio from the current position.
func (s *Streamer) AddConn(conn io.WriteCloser) {
	s.mu.Lock()
	track := s.currentTrack
	var audio []byte
	var skipBytes int
	if len(s.audioData) > 0 && track != nil {
		// Calculate how far into the track we are
		elapsed := time.Since(s.playStart).Seconds()
		duration := float64(track.Duration)
		if duration > 0 {
			progress := elapsed / duration
			if progress > 1.0 {
				progress = 1.0
			}
			skipBytes = int(progress * float64(len(s.audioData)))
		}
		remaining := s.audioData[skipBytes:]
		audio = make([]byte, len(remaining))
		copy(audio, remaining)
	}
	s.conns[conn] = true
	s.mu.Unlock()

	if track != nil && len(audio) > 0 {
		log.Printf("streamer: sending track to new conn: %s (skipped %d bytes, sending %d bytes)",
			track.Title, skipBytes, len(audio))
		s.sendTrack(conn, *track, audio)
	}
}

// RemoveConn removes an audio channel connection.
func (s *Streamer) RemoveConn(conn io.WriteCloser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, conn)
}

// ConnCount returns the number of connected audio channels.
func (s *Streamer) ConnCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.conns)
}

// StreamTrack downloads the track and sends it to all connected clients.
func (s *Streamer) StreamTrack(track Track) {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.currentTrack = &track
	s.audioData = nil
	s.mu.Unlock()

	go s.downloadAndBroadcast(ctx, track)
}

// Stop cancels the current download.
func (s *Streamer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.currentTrack = nil
	s.audioData = nil
}

func (s *Streamer) downloadAndBroadcast(ctx context.Context, track Track) {
	if track.URL == "" {
		s.signalError()
		return
	}

	log.Printf("streamer: downloading %s", track.Title)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, track.URL, nil)
	if err != nil {
		log.Printf("streamer: request error: %v", err)
		s.signalError()
		return
	}

	resp, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		log.Printf("streamer: fetch error: %v", err)
		s.signalError()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("streamer: HTTP %d for %s", resp.StatusCode, track.Title)
		s.signalError()
		return
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		log.Printf("streamer: read error: %v", err)
		s.signalError()
		return
	}

	if len(audio) < 1000 {
		log.Printf("streamer: audio too small (%d bytes), skipping %s", len(audio), track.Title)
		s.signalError()
		return
	}

	estimatedDuration := estimateMP3Duration(audio)
	log.Printf("streamer: downloaded %d bytes (~%ds), broadcasting to %d conns",
		len(audio), estimatedDuration, s.ConnCount())

	s.mu.Lock()
	if s.onDurationKnown != nil {
		go s.onDurationKnown(estimatedDuration)
	}

	// Store the audio data and mark playback start time
	s.audioData = audio
	s.playStart = time.Now()
	// Snapshot current conns
	conns := make([]io.WriteCloser, 0, len(s.conns))
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	s.mu.Unlock()

	// Send to all connected clients
	var failed []io.WriteCloser
	for _, conn := range conns {
		if err := s.sendTrack(conn, track, audio); err != nil {
			failed = append(failed, conn)
		}
	}

	// Remove failed connections
	if len(failed) > 0 {
		s.mu.Lock()
		for _, conn := range failed {
			delete(s.conns, conn)
			conn.Close()
		}
		s.mu.Unlock()
	}
}

func (s *Streamer) signalError() {
	s.mu.RLock()
	fn := s.onError
	s.mu.RUnlock()
	if fn != nil {
		fn()
	}
}

// sendTrack writes one complete track frame to a connection:
// [header len][JSON header][audio len][MP3 bytes]
func (s *Streamer) sendTrack(conn io.WriteCloser, track Track, audio []byte) error {
	if err := EncodeTrackHeader(conn, track); err != nil {
		log.Printf("streamer: header write error: %v", err)
		return err
	}
	if err := EncodeAudioLength(conn, uint32(len(audio))); err != nil {
		log.Printf("streamer: audio length write error: %v", err)
		return err
	}
	if _, err := conn.Write(audio); err != nil {
		log.Printf("streamer: audio write error: %v", err)
		return err
	}
	return nil
}

// estimateMP3Duration returns the duration in seconds.
// Uses ffprobe if available for accuracy, otherwise estimates from file size.
func estimateMP3Duration(data []byte) int {
	if _, err := exec.LookPath("ffprobe"); err == nil {
		if dur := ffprobeDuration(data); dur > 0 {
			return dur
		}
	}
	// Fallback: assume ~176kbps (typical for chillhop streams)
	dur := len(data) / 22000
	if dur < 10 {
		dur = 10
	}
	return dur
}

func ffprobeDuration(data []byte) int {
	tmp, err := os.CreateTemp("", "tavrn-probe-*.mp3")
	if err != nil {
		return 0
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := tmp.Write(data); err != nil {
		return 0
	}
	tmp.Close()

	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		tmp.Name(),
	).Output()
	if err != nil {
		return 0
	}

	f, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return int(f)
}
