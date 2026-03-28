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
	defer e.mu.Unlock()

	if e.current == nil {
		return false
	}

	e.skipVoters[fingerprint] = true

	online := 0
	if e.onlineCount != nil {
		online = e.onlineCount()
	}

	if len(e.skipVoters) >= skipThreshold(online) {
		e.autoNext()
		return true
	}

	e.notifyChange()
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
	defer e.mu.Unlock()

	// No track playing — pick one
	if e.current == nil {
		e.autoNext()
		return
	}

	// Duration not yet known (waiting for ffprobe) — skip progress check
	duration := time.Duration(e.current.Duration) * time.Second
	if duration == 0 {
		return
	}

	// Check if track ended
	elapsed := time.Since(e.playStart)
	if elapsed >= duration {
		e.autoNext()
		return
	}

	// Check if threshold dropped (people disconnected) and skip should trigger
	online := 0
	if e.onlineCount != nil {
		online = e.onlineCount()
	}
	if len(e.skipVoters) > 0 && len(e.skipVoters) >= skipThreshold(online) {
		e.autoNext()
	}
}

func (e *Engine) autoNext() {
	tracks := e.lofi.randomTracks(1)
	if len(tracks) == 0 {
		return
	}
	pick := tracks[0]
	e.current = &pick
	e.playStart = time.Now()
	e.skipVoters = make(map[string]bool)
	e.notifyChange()
	e.notifyTrackChange(pick)
}

func (e *Engine) notifyChange() {
	if e.onStateChange != nil {
		e.onStateChange()
	}
}

func (e *Engine) notifyTrackChange(track Track) {
	if e.onTrackChange != nil {
		go e.onTrackChange(track)
	}
}

// skipThreshold returns how many votes are needed to skip.
func skipThreshold(online int) int {
	if online <= 5 {
		return 1
	}
	return 3 + (online-6)/10
}
