package jukebox

import (
	"context"
	"io"
	"log"
	"net/http"
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
}

// NewStreamer creates a new audio streamer.
func NewStreamer() *Streamer {
	return &Streamer{
		conns:  make(map[io.WriteCloser]bool),
		client: &http.Client{},
	}
}

// SetOnDurationKnown sets a callback invoked when actual track duration
// is estimated from downloaded audio size (MP3 at ~128kbps).
func (s *Streamer) SetOnDurationKnown(fn func(int)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDurationKnown = fn
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

// StreamTrackWithAudio broadcasts pre-downloaded audio data for a track.
// Used for YouTube tracks where audio is downloaded via yt-dlp.
func (s *Streamer) StreamTrackWithAudio(track Track, audio []byte) {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = nil
	s.currentTrack = &track
	s.audioData = nil
	s.mu.Unlock()

	go s.broadcastAudio(track, audio)
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
		return
	}

	log.Printf("streamer: downloading %s", track.Title)

	// Download the full MP3
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, track.URL, nil)
	if err != nil {
		log.Printf("streamer: request error: %v", err)
		return
	}

	resp, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		log.Printf("streamer: fetch error: %v", err)
		return
	}
	defer resp.Body.Close()

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		log.Printf("streamer: read error: %v", err)
		return
	}

	// Estimate actual duration from file size (MP3 ~128kbps = 16000 bytes/sec)
	estimatedDuration := len(audio) / 16000
	if estimatedDuration < 10 {
		estimatedDuration = 10
	}
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

func (s *Streamer) broadcastAudio(track Track, audio []byte) {
	// Estimate actual duration from file size (MP3 ~128kbps = 16000 bytes/sec)
	estimatedDuration := len(audio) / 16000
	if estimatedDuration < 10 {
		estimatedDuration = 10
	}
	log.Printf("streamer: received %d bytes (~%ds), broadcasting to %d conns",
		len(audio), estimatedDuration, s.ConnCount())

	s.mu.Lock()
	if s.onDurationKnown != nil {
		go s.onDurationKnown(estimatedDuration)
	}
	s.audioData = audio
	s.playStart = time.Now()
	conns := make([]io.WriteCloser, 0, len(s.conns))
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	s.mu.Unlock()

	var failed []io.WriteCloser
	for _, conn := range conns {
		if err := s.sendTrack(conn, track, audio); err != nil {
			failed = append(failed, conn)
		}
	}

	if len(failed) > 0 {
		s.mu.Lock()
		for _, conn := range failed {
			delete(s.conns, conn)
			conn.Close()
		}
		s.mu.Unlock()
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
