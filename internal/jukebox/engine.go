package jukebox

import (
	"context"
	"sync"
	"time"
)

// EngineState is a snapshot for the UI.
type EngineState struct {
	Current      *Track
	Position     time.Duration
	Listeners    int
	ActiveGenre  Genre
	PendingGenre Genre
}

type Engine struct {
	mu            sync.RWMutex
	catalog       *Catalog
	activeGenre   Genre
	pendingGenre  Genre
	current       *Track
	playStart     time.Time
	onlineCount   func() int
	onStateChange func()
	onTrackChange func(Track)
}

func NewEngineWithCatalog(c *Catalog) *Engine {
	return &Engine{catalog: c}
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

func (e *Engine) SetGenre(g Genre) {
	e.mu.Lock()
	e.pendingGenre = g
	fn := e.onStateChange
	e.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (e *Engine) State() EngineState {
	e.mu.RLock()
	defer e.mu.RUnlock()

	online := 0
	if e.onlineCount != nil {
		online = e.onlineCount()
	}

	state := EngineState{
		Current:      e.current,
		Listeners:    online,
		ActiveGenre:  e.activeGenre,
		PendingGenre: e.pendingGenre,
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

	if e.current == nil {
		e.playNext()
		return
	}

	// Duration not yet known (waiting for ffprobe)
	duration := time.Duration(e.current.Duration) * time.Second
	if duration == 0 {
		e.mu.Unlock()
		return
	}

	// Track ended — play next
	if time.Since(e.playStart) >= duration {
		e.playNext()
		return
	}

	e.mu.Unlock()
}

// playNext picks the next random track. Must be called with lock held.
// Releases the lock before calling onTrackChange.
func (e *Engine) playNext() {
	// Apply pending genre switch
	if e.pendingGenre != e.activeGenre {
		e.activeGenre = e.pendingGenre
	}

	tracks := e.catalog.RandomTracks(e.activeGenre, 1)
	if len(tracks) == 0 {
		e.mu.Unlock()
		return
	}
	pick := tracks[0]
	e.current = &pick
	e.playStart = time.Now()
	e.notifyChange()

	fn := e.onTrackChange
	e.mu.Unlock()

	if fn != nil {
		fn(pick)
	}
}

func (e *Engine) notifyChange() {
	if e.onStateChange != nil {
		e.onStateChange()
	}
}
