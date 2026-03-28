package jukebox

import (
	"context"
	"sync"
	"time"
)

// EngineState is a snapshot of the engine's current state for the UI.
type EngineState struct {
	Current       *Track
	Position      time.Duration
	Listeners     int
	SkipVotes     int
	SkipThreshold int
}

type Engine struct {
	mu            sync.RWMutex
	trackMu       sync.Mutex // serializes track changes (skip, auto-next)
	lofi          *Lofi
	current       *Track
	playStart     time.Time
	skipVoters    map[string]bool
	onlineCount   func() int
	onStateChange func()
	onTrackChange func(Track)
}

func NewEngine(lofi *Lofi) *Engine {
	return &Engine{
		lofi:       lofi,
		skipVoters: make(map[string]bool),
	}
}

func (e *Engine) SetOnStateChange(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onStateChange = fn
}

func (e *Engine) SetOnTrackChange(fn func(Track)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onTrackChange = fn
}

func (e *Engine) SetOnlineCount(fn func() int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onlineCount = fn
}

func (e *Engine) State() EngineState {
	e.mu.RLock()
	defer e.mu.RUnlock()

	online := 0
	if e.onlineCount != nil {
		online = e.onlineCount()
	}

	state := EngineState{
		Current:       e.current,
		Listeners:     online,
		SkipVotes:     len(e.skipVoters),
		SkipThreshold: skipThreshold(online),
	}
	if e.current != nil {
		state.Position = time.Since(e.playStart)
	}
	return state
}

// UpdateDuration sets the current track's actual duration from ffprobe.
func (e *Engine) UpdateDuration(seconds int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current != nil && seconds > 0 {
		e.current.Duration = seconds
		e.notifyChange()
	}
}

// VoteSkip registers a skip vote. Returns true if the skip was triggered.
func (e *Engine) VoteSkip(fingerprint string) bool {
	e.mu.Lock()

	if e.current == nil {
		e.mu.Unlock()
		return false
	}

	// Debounce: ignore skips within 2 seconds of a track change
	if time.Since(e.playStart) < 2*time.Second {
		e.mu.Unlock()
		return false
	}

	e.skipVoters[fingerprint] = true

	online := 0
	if e.onlineCount != nil {
		online = e.onlineCount()
	}

	if len(e.skipVoters) >= skipThreshold(online) {
		track := e.doSkip()
		e.mu.Unlock()
		// Stream new track outside the lock to avoid deadlock
		if track != nil {
			e.fireTrackChange(*track)
		}
		return true
	}

	e.notifyChange()
	e.mu.Unlock()
	return false
}

// UserSkipped returns true if the user already voted to skip.
func (e *Engine) UserSkipped(fingerprint string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.skipVoters[fingerprint]
}

func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.tick()
		}
	}
}

func (e *Engine) tick() {
	e.mu.Lock()

	// No track playing — pick one
	if e.current == nil {
		track := e.doSkip()
		e.mu.Unlock()
		if track != nil {
			e.fireTrackChange(*track)
		}
		return
	}

	// Duration not yet known (waiting for ffprobe) — skip progress check
	duration := time.Duration(e.current.Duration) * time.Second
	if duration == 0 {
		e.mu.Unlock()
		return
	}

	// Check if track ended
	elapsed := time.Since(e.playStart)
	needsSkip := elapsed >= duration

	// Check if threshold dropped (people disconnected) and skip should trigger
	if !needsSkip {
		online := 0
		if e.onlineCount != nil {
			online = e.onlineCount()
		}
		if len(e.skipVoters) > 0 && len(e.skipVoters) >= skipThreshold(online) {
			needsSkip = true
		}
	}

	if needsSkip {
		track := e.doSkip()
		e.mu.Unlock()
		if track != nil {
			e.fireTrackChange(*track)
		}
		return
	}

	e.mu.Unlock()
}

// doSkip picks the next track and updates engine state. Must be called with lock held.
// Returns the new track, or nil if no tracks available.
func (e *Engine) doSkip() *Track {
	tracks := e.lofi.randomTracks(1)
	if len(tracks) == 0 {
		return nil
	}
	pick := tracks[0]
	e.current = &pick
	e.playStart = time.Now()
	e.skipVoters = make(map[string]bool)
	e.notifyChange()
	return &pick
}

func (e *Engine) notifyChange() {
	if e.onStateChange != nil {
		e.onStateChange()
	}
}

// fireTrackChange calls the track change callback synchronously.
// Serialized by trackMu to prevent concurrent streamer calls.
// Must be called WITHOUT the engine lock (e.mu) held.
func (e *Engine) fireTrackChange(track Track) {
	e.trackMu.Lock()
	defer e.trackMu.Unlock()
	e.mu.RLock()
	fn := e.onTrackChange
	e.mu.RUnlock()
	if fn != nil {
		fn(track)
	}
}

// skipThreshold returns how many votes are needed to skip.
func skipThreshold(online int) int {
	if online <= 5 {
		return 1
	}
	return 3 + (online-6)/10
}
